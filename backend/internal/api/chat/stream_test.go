package chat_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/smartkrishi/backend/internal/agent"
	chathandler "github.com/smartkrishi/backend/internal/api/chat"
	"github.com/smartkrishi/backend/internal/domain"
	"github.com/smartkrishi/backend/internal/repository/postgres"
	authservice "github.com/smartkrishi/backend/internal/service/auth"
	chatservice "github.com/smartkrishi/backend/internal/service/chat"
)

// fakeRunner is a scripted AgentRunner: it emits the given events and sets the
// draft on state so the handler can persist the assistant message.
type fakeRunner struct {
	events []agent.Event
	draft  string
	// captured for assertions
	lastState *agent.State
	lastOpts  agent.RunOptions

	// legacy AI (Step 10) scripting + captures
	askAnswer        string
	askErr           error
	imageAnswer      string
	imageErr         error
	lastAskMessage   string
	lastAskHistory   []map[string]string
	lastImageMessage string
	lastImageMIME    string
	lastImageBytes   int
}

func (f *fakeRunner) Run(_ context.Context, state *agent.State, opts agent.RunOptions, emit func(agent.Event) bool) {
	f.lastState = state
	f.lastOpts = opts
	// Set the draft before emitting so the end event's final_content (stamped by
	// the handler from state.Draft) is populated, matching real behavior where
	// the draft accumulates during streaming and is complete by the end event.
	state.Draft = f.draft
	for _, e := range f.events {
		if !emit(e) {
			return
		}
	}
}

// Legacy AI methods (Step 10). Scripted for tests.
func (f *fakeRunner) AskText(_ context.Context, message string, history []map[string]string) (string, error) {
	f.lastAskMessage = message
	f.lastAskHistory = history
	if f.askErr != nil {
		return "", f.askErr
	}
	if f.askAnswer != "" {
		return f.askAnswer, nil
	}
	return "ANSWER: " + message, nil
}

func (f *fakeRunner) AnalyzeImage(_ context.Context, message string, image []byte, mime string) (string, error) {
	f.lastImageMessage = message
	f.lastImageMIME = mime
	f.lastImageBytes = len(image)
	if f.imageErr != nil {
		return "", f.imageErr
	}
	if f.imageAnswer != "" {
		return f.imageAnswer, nil
	}
	return "IMAGE: " + message, nil
}

// routerWithAgent builds a chat router (against the same live pool) with a fake
// agent runner attached to the streaming endpoint.
func (e *testEnv) routerWithAgent(runner chathandler.AgentRunner) *chi.Mux {
	authSvc := authservice.NewService(e.users, e.tokens)
	chatSvc := chatservice.NewService(postgres.NewChatRepository(e.pool))
	handler := chathandler.NewHandler(chatSvc, authSvc).WithAgent(runner)
	r := chi.NewRouter()
	r.Route("/api/v1/chat", handler.Routes)
	return r
}

// parseSSEEvents parses each `data: {json}` frame into an agent.Event.
func parseSSEEvents(t *testing.T, body string) []agent.Event {
	t.Helper()
	var events []agent.Event
	for _, line := range strings.Split(body, "\n") {
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		payload := strings.TrimSpace(strings.TrimPrefix(line, "data: "))
		if payload == "" {
			continue
		}
		var ev agent.Event
		if err := json.Unmarshal([]byte(payload), &ev); err != nil {
			t.Fatalf("bad SSE JSON %q: %v", payload, err)
		}
		events = append(events, ev)
	}
	return events
}

func eventTypes(events []agent.Event) []string {
	out := make([]string, len(events))
	for i, e := range events {
		out[i] = string(e.Type)
	}
	return out
}

func TestSendStream_FullFlow(t *testing.T) {
	e := setup(t)
	userID, token := e.createUser(t)

	runner := &fakeRunner{
		events: []agent.Event{
			agent.PlanEventValue(agent.Plan{PrimaryIntent: "advise", ToolsNeeded: []string{}}, "raw"),
			agent.ResponseChunkEventValue("Grow "),
			agent.ResponseChunkEventValue("rice."),
			agent.ResponseEventValue("Grow rice.", nil),
			agent.EndEventValue(),
		},
		draft: "Grow rice.",
	}
	router := e.routerWithAgent(runner)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/chat/send-stream", strings.NewReader(`{"message":"how to grow rice?","include_logs":false}`))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if ct := rec.Header().Get("Content-Type"); ct != "text/event-stream" {
		t.Fatalf("content-type = %q", ct)
	}

	events := parseSSEEvents(t, rec.Body.String())
	types := eventTypes(events)
	want := []string{"plan", "response_chunk", "response_chunk", "response", "end"}
	if strings.Join(types, ",") != strings.Join(want, ",") {
		t.Fatalf("event types = %v, want %v", types, want)
	}

	// The runner saw the new chat id and the user query.
	if runner.lastState == nil || runner.lastState.UserQuery != "how to grow rice?" {
		t.Fatalf("runner state = %+v", runner.lastState)
	}

	// BLOCKING REQUIREMENT: every forwarded event must carry message_id + chat_id.
	chatIDStr := runner.lastState.ChatID
	var messageID string
	for _, ev := range events {
		if ev.MessageID == "" {
			t.Fatalf("event %s missing message_id", ev.Type)
		}
		if ev.ChatID != chatIDStr {
			t.Fatalf("event %s chat_id = %q, want %q", ev.Type, ev.ChatID, chatIDStr)
		}
		if messageID == "" {
			messageID = ev.MessageID
		} else if ev.MessageID != messageID {
			t.Fatalf("message_id changed mid-stream: %q vs %q", ev.MessageID, messageID)
		}
	}
	// The end event carries final_content = the draft.
	end := events[len(events)-1]
	if end.Type != "end" || end.FinalContent != "Grow rice." {
		t.Fatalf("end event = %+v, want final_content 'Grow rice.'", end)
	}

	// A new chat was created, agent_chat_id populated, and both messages saved.
	chatID := runner.lastState.ChatID
	ctx := context.Background()
	var (
		count       int
		agentChatID *string
	)
	if err := e.pool.QueryRow(ctx, `SELECT count(*) FROM chat_messages WHERE chat_id=$1`, chatID).Scan(&count); err != nil {
		t.Fatalf("count messages: %v", err)
	}
	if count != 2 {
		t.Fatalf("message count = %d, want 2 (user + assistant, no duplicate row)", count)
	}
	if err := e.pool.QueryRow(ctx, `SELECT agent_chat_id FROM chats WHERE id=$1`, chatID).Scan(&agentChatID); err != nil {
		t.Fatalf("select agent_chat_id: %v", err)
	}
	if agentChatID == nil || *agentChatID != chatID {
		t.Fatalf("agent_chat_id = %v, want %q (in-process = chat id)", agentChatID, chatID)
	}

	// The assistant placeholder (id == the streamed message_id) was updated with
	// the final draft, not duplicated.
	var assistantContent string
	if err := e.pool.QueryRow(ctx, `SELECT content FROM chat_messages WHERE id=$1 AND role='assistant'`, messageID).Scan(&assistantContent); err != nil {
		t.Fatalf("select assistant message by streamed id: %v", err)
	}
	if assistantContent != "Grow rice." {
		t.Fatalf("assistant content = %q, want %q", assistantContent, "Grow rice.")
	}
	_ = userID
}

func TestSendStream_PersistsReasoningSteps(t *testing.T) {
	e := setup(t)
	_, token := e.createUser(t)

	runner := &fakeRunner{
		events: []agent.Event{
			agent.PlanEventValue(agent.Plan{PrimaryIntent: "crop_advice", ToolsNeeded: []string{"weather_api"}}, "raw"),
			agent.ToolCallEventValue("weather_api", "Pune", map[string]any{"loc": "Pune", "forecast": "Sunny"}),
			agent.ThinkingEventValue("considering the weather"),
			agent.ResponseChunkEventValue("Water in the morning."),
			agent.ResponseEventValue("Water in the morning.", nil),
			agent.EndEventValue(),
		},
		draft: "Water in the morning.",
	}
	router := e.routerWithAgent(runner)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/chat/send-stream", strings.NewReader(`{"message":"crop advice for pune"}`))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}

	chatID := runner.lastState.ChatID
	ctx := context.Background()

	// Only the reasoning-type events are persisted (plan, tool_call, thinking) —
	// NOT response_chunk / response / end.
	var stepCount int
	if err := e.pool.QueryRow(ctx, `SELECT count(*) FROM reasoning_steps WHERE chat_id=$1`, chatID).Scan(&stepCount); err != nil {
		t.Fatalf("count reasoning steps: %v", err)
	}
	if stepCount != 3 {
		t.Fatalf("reasoning step count = %d, want 3 (plan, tool_call, thinking)", stepCount)
	}

	// Reload messages via the repo and assert reasoning steps hang off the
	// assistant message, ordered, with the tool step populated.
	repo := postgres.NewChatRepository(e.pool)
	cid, _ := uuid.Parse(chatID)
	msgs, err := repo.GetMessages(ctx, cid)
	if err != nil {
		t.Fatalf("GetMessages: %v", err)
	}
	var assistant *domain.ChatMessage
	for i := range msgs {
		if msgs[i].Role == "assistant" {
			m := msgs[i]
			assistant = &m
			break
		}
	}
	if assistant == nil {
		t.Fatal("no assistant message found")
	}
	if len(assistant.ReasoningSteps) != 3 {
		t.Fatalf("assistant reasoning steps = %d, want 3", len(assistant.ReasoningSteps))
	}
	// Ordered by step_order: plan (1), tool_call (2), thinking (3).
	if assistant.ReasoningSteps[0].StepType != "plan" ||
		assistant.ReasoningSteps[1].StepType != "tool_call" ||
		assistant.ReasoningSteps[2].StepType != "thinking" {
		t.Fatalf("reasoning step order/types wrong: %+v", assistant.ReasoningSteps)
	}
	if assistant.ReasoningSteps[0].StepOrder != 1 || assistant.ReasoningSteps[2].StepOrder != 3 {
		t.Errorf("step_order not sequential: %+v", assistant.ReasoningSteps)
	}
	tc := assistant.ReasoningSteps[1]
	if tc.ToolName == nil || *tc.ToolName != "weather_api" {
		t.Errorf("tool step tool_name = %v", tc.ToolName)
	}
	if tc.ToolResult == nil {
		t.Error("tool step tool_result should be persisted (JSONB)")
	}

	// BLOCKING: every reloaded step must have non-empty content, else the
	// frontend reasoning panel filters it out. Verify plan + tool_call in
	// particular (previously NULL).
	for _, s := range assistant.ReasoningSteps {
		if s.Content == nil || strings.TrimSpace(*s.Content) == "" {
			t.Fatalf("reloaded %s step has empty content (would be filtered by the UI)", s.StepType)
		}
	}
	if *assistant.ReasoningSteps[0].Content != "Planning: crop_advice" {
		t.Errorf("plan content = %q", *assistant.ReasoningSteps[0].Content)
	}
	if *assistant.ReasoningSteps[1].Content != "Using weather_api" {
		t.Errorf("tool_call content = %q", *assistant.ReasoningSteps[1].Content)
	}
}

func TestSendStream_ModelOverrideAndLogs(t *testing.T) {
	e := setup(t)
	_, token := e.createUser(t)

	runner := &fakeRunner{events: []agent.Event{agent.EndEventValue()}, draft: ""}
	router := e.routerWithAgent(runner)

	body := `{"message":"hi","model":"gemini-2.5-pro","tools":["weather_api","soil_api"],"include_logs":true}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/chat/send-stream", strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	if runner.lastOpts.Model != "gemini-2.5-pro" {
		t.Errorf("model override = %q, want gemini-2.5-pro", runner.lastOpts.Model)
	}
	if !runner.lastOpts.Logs {
		t.Error("include_logs should propagate as Logs=true")
	}
	if len(runner.lastOpts.Tools) != 2 || runner.lastOpts.Tools[0] != "weather_api" {
		t.Errorf("tools = %v, want [weather_api soil_api]", runner.lastOpts.Tools)
	}
}

func TestSendStream_EmptyMessage(t *testing.T) {
	e := setup(t)
	_, token := e.createUser(t)
	router := e.routerWithAgent(&fakeRunner{})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/chat/send-stream", strings.NewReader(`{"message":"   "}`))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if !strings.Contains(rec.Body.String(), `"type":"error"`) {
		t.Fatalf("expected error event for empty message, got %s", rec.Body.String())
	}
}

func TestSendStream_AgentNotConfigured(t *testing.T) {
	e := setup(t)
	_, token := e.createUser(t)

	// Build a router WITHOUT an agent.
	authSvc := authservice.NewService(e.users, e.tokens)
	chatSvc := chatservice.NewService(postgres.NewChatRepository(e.pool))
	handler := chathandler.NewHandler(chatSvc, authSvc)
	r := chi.NewRouter()
	r.Route("/api/v1/chat", handler.Routes)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/chat/send-stream", strings.NewReader(`{"message":"hi"}`))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if !strings.Contains(rec.Body.String(), "not configured") {
		t.Fatalf("expected not-configured error, got %s", rec.Body.String())
	}
}
