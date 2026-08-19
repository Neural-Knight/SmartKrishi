package chat

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"github.com/google/uuid"

	"github.com/smartkrishi/backend/internal/agent"
	"github.com/smartkrishi/backend/internal/domain"
	appmiddleware "github.com/smartkrishi/backend/internal/middleware"
)

// AgentRunner builds and runs the agent pipeline for one streaming turn, and
// also serves the non-streaming AI paths. The server wires a concrete
// implementation (planner + executor over the Gemini provider); tests supply a
// fake. Keeping this an interface lets the HTTP layer stay independent of how
// the agent packages are constructed.
type AgentRunner interface {
	// Run drives the pipeline for state, invoking emit for each event in order.
	// Returning from Run means the stream is complete (end/error already
	// emitted).
	Run(ctx context.Context, state *agent.State, opts agent.RunOptions, emit func(agent.Event) bool)

	// AskText answers a question with a single LLM call (not the agent
	// pipeline) — the /ask and /send path.
	AskText(ctx context.Context, message string, history []map[string]string) (string, error)

	// AnalyzeImage runs a stateless vision call over the image — the
	// /analyze-image and /analyze-image-persistent path.
	AnalyzeImage(ctx context.Context, message string, image []byte, mime string) (string, error)
}

// sendStreamRequest is the POST /chat/send-stream body. Matches the frontend
// chatService.sendMessageStream payload.
type sendStreamRequest struct {
	Message     string   `json:"message"`
	ChatID      *string  `json:"chat_id"`
	Model       string   `json:"model"`
	Tools       []string `json:"tools"`
	IncludeLogs bool     `json:"include_logs"`
}

// sendStream handles POST /chat/send-stream: it resolves/creates the chat,
// persists the user message, runs the agent pipeline in-process, streams each
// event as SSE (`data: {json}\n\n`), and persists the assistant answer at the
// end.
func (h *Handler) sendStream(w http.ResponseWriter, r *http.Request) {
	userCtx, ok := appmiddleware.UserFromContext(r.Context())
	if !ok {
		writeSSEError(w, "Could not validate credentials")
		return
	}
	userID := userCtx.ID

	var req sendStreamRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeSSEError(w, "Invalid request body")
		return
	}
	if strings.TrimSpace(req.Message) == "" {
		writeSSEError(w, "message is required")
		return
	}

	// Resolve an optional chat id.
	var chatID *uuid.UUID
	if req.ChatID != nil && *req.ChatID != "" {
		parsed, err := uuid.Parse(*req.ChatID)
		if err != nil {
			writeSSEError(w, "Invalid chat ID format")
			return
		}
		chatID = &parsed
	}

	flusher, ok := beginSSE(w)
	if !ok {
		return
	}

	// Resolve/create chat + ensure agent chat id.
	chat, err := h.chats.EnsureChatForStream(r.Context(), userID, chatID, req.Message)
	if err != nil {
		writeSSEEvent(w, flusher, agent.ErrorEventValue("Chat not found"))
		return
	}

	h.runStreamingTurn(r.Context(), w, flusher, userID, chat.ID, req.Message,
		agent.RunOptions{Logs: req.IncludeLogs, Model: req.Model, Tools: req.Tools}, nil, nil)
}

// beginSSE verifies the writer supports flushing and writes SSE headers. It
// returns the flusher and true on success; on failure it writes an error and
// returns false.
func beginSSE(w http.ResponseWriter) (http.Flusher, bool) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming unsupported", http.StatusInternalServerError)
		return nil, false
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no") // disable proxy buffering (nginx)
	w.WriteHeader(http.StatusOK)
	return flusher, true
}

// runStreamingTurn is the shared streaming core for send-stream and
// upload-and-analyze-stream: it persists the user message + assistant
// placeholder, emits any preEvents (e.g. file_uploaded) stamped with the
// message/chat ids, runs the pipeline forwarding + persisting events, and fills
// the assistant placeholder at the end. The chat must already be resolved
// (EnsureChatForStream) by the caller.
// onUserMessage, when non-nil, is invoked with the persisted user message id
// right after it is created — used by upload-and-analyze to link the uploaded
// file to that message. It runs before streaming and must not block.
func (h *Handler) runStreamingTurn(ctx context.Context, w http.ResponseWriter, flusher http.Flusher, userID int32, chatID uuid.UUID, message string, opts agent.RunOptions, preEvents []agent.Event, onUserMessage func(userMsgID uuid.UUID)) {
	chatIDStr := chatID.String()

	userMsg, err := h.chats.AddMessage(ctx, chatID, userID, "user", message)
	if err != nil {
		writeSSEEvent(w, flusher, agent.ErrorEventValue("Failed to save message"))
		return
	}
	if onUserMessage != nil {
		onUserMessage(userMsg.ID)
	}
	// Assistant placeholder (single row; filled after streaming).
	assistant, err := h.chats.AddMessage(ctx, chatID, userID, "assistant", "")
	if err != nil {
		writeSSEEvent(w, flusher, agent.ErrorEventValue("Failed to create response"))
		return
	}
	messageID := assistant.ID.String()

	// Emit any pre-stream events (file_uploaded), stamped like pipeline events.
	for _, ev := range preEvents {
		ev.MessageID = messageID
		ev.ChatID = chatIDStr
		if !writeSSEEvent(w, flusher, ev) {
			return
		}
	}

	history := h.loadHistory(ctx, chatID)
	state := agent.NewState(intToString(userID), chatIDStr, message)
	state.History = history

	stepOrder := int32(0)
	h.agent.Run(ctx, state, opts, func(ev agent.Event) bool {
		ev.MessageID = messageID
		ev.ChatID = chatIDStr
		if ev.Type == agent.EventEnd {
			ev.FinalContent = state.Draft
		}
		if in, ok := reasoningStepFor(ev, assistant.ID, chatID, userID); ok {
			stepOrder++
			in.StepOrder = stepOrder
			_ = h.chats.SaveReasoningStep(ctx, in)
		}
		return writeSSEEvent(w, flusher, ev)
	})

	// Fill the assistant placeholder with the final answer (skip if empty). Use a
	// cancel-free context so a client disconnect after the final event still
	// persists the answer. No second assistant row is created.
	if strings.TrimSpace(state.Draft) != "" {
		saveCtx := context.WithoutCancel(ctx)
		_ = h.chats.UpdateMessage(saveCtx, assistant.ID, state.Draft)
	}
}

// loadHistory returns recent messages as agent history, best-effort (an error
// just yields empty history rather than aborting the turn).
func (h *Handler) loadHistory(ctx context.Context, chatID uuid.UUID) []agent.HistoryMessage {
	msgs, err := h.chats.RecentHistory(ctx, chatID, 20)
	if err != nil {
		return nil
	}
	out := make([]agent.HistoryMessage, 0, len(msgs))
	for _, m := range msgs {
		out = append(out, agent.HistoryMessage{Role: m.Role, Msg: m.Content})
	}
	return out
}

// writeSSEEvent marshals ev and writes it as one SSE frame: `data: {json}\n\n`.
// Returns false if the write fails (client disconnected), signaling the pipeline
// to stop.
func writeSSEEvent(w http.ResponseWriter, flusher http.Flusher, ev agent.Event) bool {
	payload, err := json.Marshal(ev)
	if err != nil {
		return true // skip a bad event, keep streaming
	}
	if _, err := w.Write([]byte("data: ")); err != nil {
		return false
	}
	if _, err := w.Write(payload); err != nil {
		return false
	}
	if _, err := w.Write([]byte("\n\n")); err != nil {
		return false
	}
	flusher.Flush()
	return true
}

// writeSSEError writes a single SSE error frame with proper headers. Used for
// pre-stream failures (bad request, auth) where headers aren't set yet.
func writeSSEError(w http.ResponseWriter, message string) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.WriteHeader(http.StatusOK)
	payload, _ := json.Marshal(agent.ErrorEventValue(message))
	_, _ = w.Write([]byte("data: "))
	_, _ = w.Write(payload)
	_, _ = w.Write([]byte("\n\n"))
	if f, ok := w.(http.Flusher); ok {
		f.Flush()
	}
}

func intToString(i int32) string {
	return strconv.Itoa(int(i))
}

// reasoningStepFor maps a streaming event to a reasoning-step insert. It
// persists the reasoning-type events the frontend renders in its reasoning
// panel (plan, tool_call, thinking, code_execution, grounding_*, log, error)
// and returns ok=false for the answer events (response_chunk, response, end),
// which are the message content itself — not reasoning. step_metadata carries
// the full event so the reasoning panel can be replayed faithfully on reload.
func reasoningStepFor(ev agent.Event, messageID, chatID uuid.UUID, userID int32) (domain.ReasoningStepInput, bool) {
	switch ev.Type {
	case agent.EventResponseChunk, agent.EventResponse, agent.EventEnd, agent.EventFileUploaded:
		// Answer content (response_chunk/response/end) and the file_uploaded
		// notification are not reasoning — don't persist them as reasoning steps.
		return domain.ReasoningStepInput{}, false
	}

	in := domain.ReasoningStepInput{
		MessageID:    messageID,
		ChatID:       chatID,
		UserID:       userID,
		StepType:     string(ev.Type),
		StepMetadata: ev,
	}
	if ev.Stage != "" {
		in.Stage = strPtr(ev.Stage)
	}
	if ev.Tool != "" {
		in.ToolName = strPtr(ev.Tool)
		in.ToolArgs = toolArgsString(ev.Args)
		in.ToolResult = ev.Result
	}

	// content MUST be non-empty: the frontend reasoning panel filters out any
	// step whose content.trim() is empty (DashboardPage.tsx), so a NULL content
	// would hide plan/tool_call steps after reload. Synthesize a label matching
	// useStreamingChat.ts for each type; fall back to the raw content/message.
	in.Content = strPtr(reasoningContent(ev))
	return in, true
}

// reasoningContent returns the human-readable content string for a reasoning
// step, matching the labels useStreamingChat.ts renders live so a reloaded chat
// shows the same text. Never returns "" for a persisted step.
func reasoningContent(ev agent.Event) string {
	switch ev.Type {
	case agent.EventPlan:
		intent := ""
		if ev.Plan != nil {
			intent = ev.Plan.PrimaryIntent
		}
		if intent == "" {
			return "Planning: Creating strategy"
		}
		return "Planning: " + intent
	case agent.EventToolCall:
		tool := ev.Tool
		if tool == "" {
			tool = "tool"
		}
		return "Using " + tool
	case agent.EventCodeExecution:
		if ev.Stage == "result" {
			outcome := ev.Outcome
			if outcome == "" {
				outcome = "success"
			}
			return "Result: " + outcome
		}
		lang := ev.Language
		if lang == "" {
			lang = "code"
		}
		return "Executing: " + lang
	case agent.EventGroundingWebSearchQuery:
		if len(ev.Queries) > 0 {
			return "Web searches: " + strings.Join(ev.Queries, ", ")
		}
		return "Web searches: web queries"
	case agent.EventGroundingChunks:
		return "Sources: " + strconv.Itoa(len(ev.Sources)) + " references"
	case agent.EventGroundingSupports:
		return "Citations linked"
	case agent.EventThinking:
		if ev.Content != "" {
			return ev.Content
		}
		return "Thinking..."
	}
	// log / error and any other reasoning type: prefer content, then message.
	if ev.Content != "" {
		return ev.Content
	}
	if ev.Message != "" {
		return ev.Message
	}
	return string(ev.Type)
}

// toolArgsString renders tool args for the tool_args TEXT column: a plain
// string is stored as-is; anything else is JSON-marshaled (so structured args
// aren't silently dropped). Returns nil for empty/nil args.
func toolArgsString(args any) *string {
	if args == nil {
		return nil
	}
	if s, ok := args.(string); ok {
		if s == "" {
			return nil
		}
		return strPtr(s)
	}
	b, err := json.Marshal(args)
	if err != nil {
		return nil
	}
	return strPtr(string(b))
}

func strPtr(s string) *string { return &s }
