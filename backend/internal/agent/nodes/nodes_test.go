package nodes_test

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"github.com/google/uuid"

	"github.com/smartkrishi/backend/internal/agent"
	"github.com/smartkrishi/backend/internal/agent/llm"
	"github.com/smartkrishi/backend/internal/agent/llm/mock"
	"github.com/smartkrishi/backend/internal/agent/nodes"
	"github.com/smartkrishi/backend/internal/agent/tools"
	"github.com/smartkrishi/backend/internal/domain"
)

// execReader is a minimal tools.MessageReader for executor tests.
type execReader struct {
	messages []domain.ChatMessage
}

func (e *execReader) GetByID(_ context.Context, chatID uuid.UUID, _ int32) (*domain.Chat, error) {
	return &domain.Chat{ID: chatID}, nil
}
func (e *execReader) GetMessages(_ context.Context, _ uuid.UUID) ([]domain.ChatMessage, error) {
	return e.messages, nil
}
func (e *execReader) SearchMessages(_ context.Context, _ int32, _ []string, _ *uuid.UUID, _ int32) ([]domain.ChatMessage, error) {
	return e.messages, nil
}
func (e *execReader) ListSummaries(_ context.Context, _ int32, _, _ int32) ([]domain.ChatSummary, error) {
	return nil, nil
}

func TestPlanner_ParsesJSON(t *testing.T) {
	m := &mock.Provider{GenerateText: `{"primary_intent":"market_analysis","tools_needed":["market_api"],"location":"Punjab","crop":"wheat","reasoning":"price query"}`}
	p := nodes.NewPlanner(m, "planner-model")

	st := agent.NewState("1", "", "wheat price in punjab")
	if err := p.Run(context.Background(), st); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if st.Plan.PrimaryIntent != "market_analysis" || st.Plan.Crop != "wheat" || st.Plan.Location != "Punjab" {
		t.Fatalf("bad plan: %+v", st.Plan)
	}
	if len(st.Plan.ToolsNeeded) != 1 || st.Plan.ToolsNeeded[0] != "market_api" {
		t.Fatalf("tools = %v", st.Plan.ToolsNeeded)
	}
	if m.LastOpts.Model != "planner-model" || !m.LastOpts.JSON {
		t.Fatalf("planner should call in JSON mode with its model; got %+v", m.LastOpts)
	}
}

func TestPlanner_StripsCodeFence(t *testing.T) {
	m := &mock.Provider{GenerateText: "```json\n{\"primary_intent\":\"advise\",\"tools_needed\":[\"weather_api\"]}\n```"}
	p := nodes.NewPlanner(m, "planner-model")
	st := agent.NewState("1", "", "q")
	_ = p.Run(context.Background(), st)
	if st.Plan.PrimaryIntent != "advise" {
		t.Fatalf("fenced JSON not parsed: %+v", st.Plan)
	}
}

func TestPlanner_FallbackOnGarbage(t *testing.T) {
	m := &mock.Provider{GenerateText: "not json at all"}
	p := nodes.NewPlanner(m, "planner-model")
	st := agent.NewState("1", "", "q")
	_ = p.Run(context.Background(), st)
	if st.Plan.PrimaryIntent != "advise" || len(st.Plan.ToolsNeeded) != 1 {
		t.Fatalf("expected fallback plan, got %+v", st.Plan)
	}
}

func TestPlanner_FallbackOnLLMError(t *testing.T) {
	m := &mock.Provider{GenerateFunc: func(context.Context, llm.Request, llm.Opts) (llm.Response, error) {
		return llm.Response{}, errors.New("boom")
	}}
	p := nodes.NewPlanner(m, "planner-model")
	st := agent.NewState("1", "", "q")
	if err := p.Run(context.Background(), st); err != nil {
		t.Fatalf("Run should not error on fallback: %v", err)
	}
	if st.Plan.Reasoning != "Default fallback plan" {
		t.Fatalf("expected fallback plan, got %+v", st.Plan)
	}
}

func TestExecutor_RoutesToolArgs(t *testing.T) {
	// registry with no creds: tools return fallbacks but still populate ToolCalls.
	reg := tools.NewRegistry(http.DefaultClient, tools.Config{}, nil)
	m := &mock.Provider{GenerateText: "Here is your advice."}
	ex := nodes.NewExecutor(m, reg, "agent-model")

	st := agent.NewState("1", "", "wheat prices")
	st.Plan = agent.Plan{
		ToolsNeeded: []string{"weather_api", "market_api", "soil_api", "get_pdf_content", "unknown_tool"},
		Location:    "Punjab",
		Crop:        "wheat",
	}
	if err := ex.Run(context.Background(), st); err != nil {
		t.Fatalf("Run: %v", err)
	}

	// Implemented tools ran; file tool + unknown were skipped.
	for _, name := range []string{"weather_api", "market_api", "soil_api"} {
		if _, ok := st.ToolCalls[name]; !ok {
			t.Errorf("expected %s in ToolCalls", name)
		}
	}
	if _, ok := st.ToolCalls["get_pdf_content"]; ok {
		t.Error("deferred file tool should be skipped")
	}
	if _, ok := st.ToolCalls["unknown_tool"]; ok {
		t.Error("unknown tool should be skipped")
	}

	// Market tool got the crop, not the location (bug fix).
	market := st.ToolCalls["market_api"].(map[string]any)
	if market["crop"] != "wheat" {
		t.Errorf("market crop = %v, want wheat", market["crop"])
	}

	if st.Draft != "Here is your advice." {
		t.Errorf("draft = %q", st.Draft)
	}
	if !m.LastOpts.Tools.GoogleSearch || !m.LastOpts.Tools.CodeExecution || !m.LastOpts.Thinking {
		t.Errorf("executor should enable native tools + thinking; got %+v", m.LastOpts)
	}
}

func TestExecutor_PopulatesChatHistory(t *testing.T) {
	chatID := uuid.New()
	reader := &execReader{messages: []domain.ChatMessage{
		{Role: "user", Content: "earlier question about maize"},
	}}
	reg := tools.NewRegistry(http.DefaultClient, tools.Config{}, reader)
	m := &mock.Provider{GenerateText: "answer"}
	ex := nodes.NewExecutor(m, reg, "agent-model")

	st := agent.NewState("1", chatID.String(), "what about maize now?")
	st.Plan = agent.Plan{ToolsNeeded: []string{"chat_history"}, Location: "unknown", Crop: "general"}
	if err := ex.Run(context.Background(), st); err != nil {
		t.Fatalf("Run: %v", err)
	}

	got, ok := st.ToolCalls["chat_history"]
	if !ok {
		t.Fatal("expected chat_history in ToolCalls")
	}
	result := got.(map[string]any)
	// No query text? There IS a query (the user message), so this is a search.
	if result["type"] != "search_results" {
		t.Fatalf("chat_history type = %v", result["type"])
	}
	if result["count"] != 1 {
		t.Fatalf("count = %v, want 1", result["count"])
	}
}

func TestExecutor_EmptyDraftOnLLMError(t *testing.T) {
	reg := tools.NewRegistry(http.DefaultClient, tools.Config{}, nil)
	m := &mock.Provider{GenerateFunc: func(context.Context, llm.Request, llm.Opts) (llm.Response, error) {
		return llm.Response{}, errors.New("model down")
	}}
	ex := nodes.NewExecutor(m, reg, "agent-model")
	st := agent.NewState("1", "", "q")
	st.Plan = agent.Plan{ToolsNeeded: []string{"soil_api"}, Location: "X"}
	if err := ex.Run(context.Background(), st); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if st.Draft != "" {
		t.Fatalf("expected empty draft on error, got %q", st.Draft)
	}
	// Tool still ran even though the model failed.
	if _, ok := st.ToolCalls["soil_api"]; !ok {
		t.Error("soil_api should have run before the model call")
	}
}

func TestChecker_ParsesVerdict(t *testing.T) {
	m := &mock.Provider{GenerateText: `{"approved":true,"issues":["minor wording"],"conf_delta":0.1}`}
	c := nodes.NewChecker(m, "checker-model")
	st := agent.NewState("1", "", "q")
	st.Draft = "some answer"
	if err := c.Run(context.Background(), st); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if !st.Approved {
		t.Error("expected approved")
	}
	if len(st.Issues) != 1 || st.Issues[0] != "minor wording" {
		t.Errorf("issues = %v", st.Issues)
	}
	if st.Confidence < 0.79 || st.Confidence > 0.81 { // 0.7 + 0.1
		t.Errorf("confidence = %v, want ~0.8", st.Confidence)
	}
}

func TestChecker_DefaultsApprovedOnBadJSON(t *testing.T) {
	m := &mock.Provider{GenerateText: "garbage"}
	c := nodes.NewChecker(m, "checker-model")
	st := agent.NewState("1", "", "q")
	st.Draft = "answer"
	_ = c.Run(context.Background(), st)
	if !st.Approved {
		t.Error("checker should default to approved on unparseable verdict")
	}
}

func TestChecker_DefaultsApprovedOnLLMError(t *testing.T) {
	m := &mock.Provider{GenerateFunc: func(context.Context, llm.Request, llm.Opts) (llm.Response, error) {
		return llm.Response{}, errors.New("boom")
	}}
	c := nodes.NewChecker(m, "checker-model")
	st := agent.NewState("1", "", "q")
	_ = c.Run(context.Background(), st)
	if !st.Approved {
		t.Error("checker should default to approved on LLM error")
	}
}
