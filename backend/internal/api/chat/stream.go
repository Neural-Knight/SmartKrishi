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

// AgentRunner builds and runs the agent pipeline for one streaming turn. The
// server wires a concrete implementation (planner + executor over the Gemini
// provider); tests supply a fake. Keeping this an interface lets the HTTP layer
// stay independent of the agent packages' construction details.
type AgentRunner interface {
	// Run drives the pipeline for state, invoking emit for each event in order.
	// Returning from Run means the stream is complete (end/error already
	// emitted).
	Run(ctx context.Context, state *agent.State, opts agent.RunOptions, emit func(agent.Event) bool)
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
// persists the user message, runs the agent pipeline, streams each event as SSE
// (`data: {json}\n\n`), and persists the assistant answer at the end. Mirrors
// the Python /send-stream router flow, with the agent running in-process.
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

	// Prepare SSE response headers before writing any event.
	flusher, ok := w.(http.Flusher)
	if !ok {
		// Cannot stream without flushing; fail loudly rather than buffering.
		http.Error(w, "streaming unsupported", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no") // disable proxy buffering (nginx)
	w.WriteHeader(http.StatusOK)

	ctx := r.Context()

	// Resolve/create chat + ensure agent chat id, then persist the user message.
	chat, err := h.chats.EnsureChatForStream(ctx, userID, chatID, req.Message)
	if err != nil {
		writeSSEEvent(w, flusher, agent.ErrorEventValue("Chat not found"))
		return
	}
	if _, err := h.chats.AddMessage(ctx, chat.ID, userID, "user", req.Message); err != nil {
		writeSSEEvent(w, flusher, agent.ErrorEventValue("Failed to save message"))
		return
	}

	// Create the assistant message up front with empty content (matching the
	// Python flow: a single assistant row that streams fill in). Its id is
	// stamped on every SSE event so the frontend can attach chunks to it; we
	// update its content after the stream instead of inserting a second row.
	assistant, err := h.chats.AddMessage(ctx, chat.ID, userID, "assistant", "")
	if err != nil {
		writeSSEEvent(w, flusher, agent.ErrorEventValue("Failed to create response"))
		return
	}
	messageID := assistant.ID.String()
	chatIDStr := chat.ID.String()

	// Load recent history for agent context (the just-added user + empty
	// assistant messages are included; the empty assistant adds no content).
	history := h.loadHistory(ctx, chat.ID)

	state := agent.NewState(intToString(userID), chatIDStr, req.Message)
	state.History = history

	// Stream the pipeline as SSE. Every forwarded event is stamped with the
	// assistant message_id and chat_id (the frontend drops events without a
	// message_id); the terminal end event also carries final_content. Each
	// reasoning-type event is also persisted to reasoning_steps (Step 8) so a
	// reloaded chat replays the agent's reasoning. Persistence is best-effort and
	// never blocks or fails the live stream. emit returns false when the client
	// is gone.
	stepOrder := int32(0)
	h.agent.Run(ctx, state, agent.RunOptions{
		Logs:  req.IncludeLogs,
		Model: req.Model,
		Tools: req.Tools,
	}, func(ev agent.Event) bool {
		ev.MessageID = messageID
		ev.ChatID = chatIDStr
		if ev.Type == agent.EventEnd {
			ev.FinalContent = state.Draft
		}
		if in, ok := reasoningStepFor(ev, assistant.ID, chat.ID, userID); ok {
			stepOrder++
			in.StepOrder = stepOrder
			_ = h.chats.SaveReasoningStep(ctx, in)
		}
		return writeSSEEvent(w, flusher, ev)
	})

	// Fill the assistant placeholder with the final answer (skip empty, matching
	// Python). Use a cancel-free context so a client disconnect after the final
	// event still persists the answer. No second assistant row is created.
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

// reasoningStepFor maps a streaming event to a reasoning-step insert (Step 8).
// It persists the reasoning-type events the frontend renders in its reasoning
// panel (plan, tool_call, thinking, code_execution, grounding_*, log, error)
// and returns ok=false for the answer events (response_chunk, response, end),
// which are the message content itself — not reasoning. step_metadata carries
// the full event for faithful replay, mirroring the Python step_metadata=event.
func reasoningStepFor(ev agent.Event, messageID, chatID uuid.UUID, userID int32) (domain.ReasoningStepInput, bool) {
	switch ev.Type {
	case agent.EventResponseChunk, agent.EventResponse, agent.EventEnd:
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
