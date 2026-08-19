package nodes_test

import (
	"context"
	"net/http"
	"testing"

	"github.com/smartkrishi/backend/internal/agent"
	"github.com/smartkrishi/backend/internal/agent/llm"
	"github.com/smartkrishi/backend/internal/agent/llm/mock"
	"github.com/smartkrishi/backend/internal/agent/nodes"
	"github.com/smartkrishi/backend/internal/agent/tools"
)

// buildPipeline wires real planner + executor nodes over one mock provider.
// The mock returns planJSON from Generate (planner) and streamChunks from
// GenerateStream (executor's main-agent call).
func buildPipeline(planJSON string, streamChunks []llm.StreamChunk, logs bool) (*agent.Pipeline, *mock.Provider) {
	m := &mock.Provider{GenerateText: planJSON, StreamChunks: streamChunks}
	reg := tools.NewRegistry(http.DefaultClient, tools.Config{}, nil)
	planner := nodes.NewPlanner(m, "planner-model")
	executor := nodes.NewExecutor(m, reg, "agent-model")
	return agent.NewPipeline(planner, executor, logs), m
}

// collect runs the pipeline and returns all emitted events.
func collect(p *agent.Pipeline, st *agent.State) []agent.Event {
	var evs []agent.Event
	p.Run(context.Background(), st, func(e agent.Event) bool {
		evs = append(evs, e)
		return true
	})
	return evs
}

func types(evs []agent.Event) []agent.EventType {
	out := make([]agent.EventType, len(evs))
	for i, e := range evs {
		out[i] = e.Type
	}
	return out
}

func TestPipeline_HappyPath_NoLogs(t *testing.T) {
	plan := `{"primary_intent":"weather_forecast","tools_needed":["weather_api"],"location":"Pune","crop":"general"}`
	stream := []llm.StreamChunk{
		{Text: "thinking about it", Thought: true},
		{Text: "It will "},
		{Text: "rain."},
	}
	p, _ := buildPipeline(plan, stream, false)
	st := agent.NewState("1", "", "weather in pune?")

	evs := collect(p, st)
	got := types(evs)
	want := []agent.EventType{
		agent.EventPlan,
		agent.EventToolCall, // weather_api
		agent.EventThinking,
		agent.EventResponseChunk,
		agent.EventResponseChunk,
		agent.EventResponse,
		agent.EventEnd,
	}
	if len(got) != len(want) {
		t.Fatalf("event types = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("event[%d] = %s, want %s (full: %v)", i, got[i], want[i], got)
		}
	}

	// plan event carries the parsed plan + raw response.
	if evs[0].Plan == nil || evs[0].Plan.PrimaryIntent != "weather_forecast" {
		t.Errorf("plan event bad: %+v", evs[0])
	}
	if evs[0].RawResponse == "" {
		t.Error("plan event should carry raw_response")
	}
	// tool_call is weather_api with the location as args.
	if evs[1].Tool != "weather_api" || evs[1].Args != "Pune" {
		t.Errorf("tool_call = %+v", evs[1])
	}
	// final response aggregates the answer chunks (not the thought).
	if evs[len(evs)-2].Response != "It will rain." {
		t.Errorf("response = %q", evs[len(evs)-2].Response)
	}
	// draft persisted on state for Step 7.
	if st.Draft != "It will rain." {
		t.Errorf("state.Draft = %q", st.Draft)
	}
}

func TestPipeline_WithLogs(t *testing.T) {
	plan := `{"primary_intent":"advise","tools_needed":[],"location":"unknown","crop":"general"}`
	stream := []llm.StreamChunk{{Text: "ok"}}
	p, _ := buildPipeline(plan, stream, true)
	st := agent.NewState("u1", "c1", "hi")

	evs := collect(p, st)
	got := types(evs)

	// Expect log events around each phase, no tool_call (empty tools).
	want := []agent.EventType{
		agent.EventLog, // initialization
		agent.EventLog, // planner_start
		agent.EventPlan,
		agent.EventLog, // planner_complete
		agent.EventLog, // tools_complete
		agent.EventLog, // agent_start
		agent.EventResponseChunk,
		agent.EventResponse,
		agent.EventLog, // complete
		agent.EventEnd,
	}
	if len(got) != len(want) {
		t.Fatalf("types = %v (len %d), want %v (len %d)", got, len(got), want, len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("event[%d] = %s, want %s", i, got[i], want[i])
		}
	}
	if evs[0].Stage != "initialization" {
		t.Errorf("first log stage = %q", evs[0].Stage)
	}
}

func TestPipeline_GroundingAndCode(t *testing.T) {
	plan := `{"primary_intent":"crop_advice","tools_needed":["soil_api"],"location":"Nashik","crop":"grape"}`
	stream := []llm.StreamChunk{
		{Grounding: &llm.Grounding{
			WebSearchQueries: []string{"grape soil"},
			Sources:          []llm.GroundingSource{{URI: "http://x", Title: "X"}},
		}},
		{Code: &llm.CodeExecution{Stage: "code", Code: "print(1)", Language: "python"}},
		{Code: &llm.CodeExecution{Stage: "result", Outcome: "OK", Result: "1"}},
		{Text: "Use gypsum."},
	}
	p, _ := buildPipeline(plan, stream, false)
	st := agent.NewState("1", "", "grape soil advice")

	evs := collect(p, st)
	got := types(evs)
	want := []agent.EventType{
		agent.EventPlan,
		agent.EventToolCall, // soil_api
		agent.EventGroundingWebSearchQuery,
		agent.EventGroundingChunks,
		agent.EventCodeExecution, // code
		agent.EventCodeExecution, // result
		agent.EventResponseChunk,
		agent.EventResponse,
		agent.EventEnd,
	}
	if len(got) != len(want) {
		t.Fatalf("types = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("event[%d] = %s, want %s", i, got[i], want[i])
		}
	}

	// grounding chunks carry the source; code events carry their stages.
	if len(evs[3].Sources) != 1 || evs[3].Sources[0].URI != "http://x" {
		t.Errorf("grounding_chunks = %+v", evs[3])
	}
	if evs[4].Stage != "code" || evs[4].Code != "print(1)" {
		t.Errorf("code event = %+v", evs[4])
	}
	if evs[5].Stage != "result" || evs[5].Outcome != "OK" || evs[5].Result != "1" {
		t.Errorf("code result event = %+v", evs[5])
	}
	// final response carries accumulated grounding_metadata.
	resp := evs[len(evs)-2]
	if resp.GroundingMetadata == nil || len(resp.GroundingMetadata.WebSearchQueries) != 1 {
		t.Errorf("response grounding_metadata missing: %+v", resp)
	}
}

func TestPipeline_StreamError(t *testing.T) {
	plan := `{"primary_intent":"advise","tools_needed":[],"location":"unknown","crop":"general"}`
	m := &mock.Provider{GenerateText: plan, StreamErr: context.DeadlineExceeded}
	reg := tools.NewRegistry(http.DefaultClient, tools.Config{}, nil)
	p := agent.NewPipeline(nodes.NewPlanner(m, "pm"), nodes.NewExecutor(m, reg, "am"), false)
	st := agent.NewState("1", "", "q")

	evs := collect(p, st)
	got := types(evs)
	// plan then error (no response/end after a stream failure).
	if len(got) != 2 || got[0] != agent.EventPlan || got[1] != agent.EventError {
		t.Fatalf("types = %v, want [plan error]", got)
	}
	if evs[1].Error == "" {
		t.Error("error event should carry a message")
	}
}

func TestPipeline_EmitStopsEarly(t *testing.T) {
	plan := `{"primary_intent":"advise","tools_needed":["weather_api"],"location":"X","crop":"general"}`
	stream := []llm.StreamChunk{{Text: "hello"}}
	p, _ := buildPipeline(plan, stream, false)
	st := agent.NewState("1", "", "q")

	// Stop after the very first event (plan).
	var count int
	p.Run(context.Background(), st, func(agent.Event) bool {
		count++
		return false
	})
	if count != 1 {
		t.Fatalf("emit called %d times, want 1 (should stop early)", count)
	}
}

func TestPipeline_ToolAllowList(t *testing.T) {
	// plan asks for weather + soil, but the allow-list only permits weather.
	plan := `{"primary_intent":"crop_advice","tools_needed":["weather_api","soil_api"],"location":"Pune","crop":"rice"}`
	stream := []llm.StreamChunk{{Text: "ok"}}
	m := &mock.Provider{GenerateText: plan, StreamChunks: stream}
	reg := tools.NewRegistry(http.DefaultClient, tools.Config{}, nil)
	p := agent.NewPipeline(nodes.NewPlanner(m, "pm"), nodes.NewExecutor(m, reg, "am"), false).
		WithToolAllowList([]string{"weather_api"})
	st := agent.NewState("1", "", "q")

	evs := collect(p, st)
	var toolCalls int
	for _, e := range evs {
		if e.Type == agent.EventToolCall {
			toolCalls++
			if e.Tool != "weather_api" {
				t.Errorf("allow-list should permit only weather_api, got %q", e.Tool)
			}
		}
	}
	if toolCalls != 1 {
		t.Fatalf("tool_call count = %d, want 1 (soil_api filtered by allow-list)", toolCalls)
	}
	if _, ran := st.ToolCalls["soil_api"]; ran {
		t.Error("soil_api should not have run (filtered by allow-list)")
	}
}

func TestPipeline_ToolCallSkipsUnavailable(t *testing.T) {
	// plan includes a deferred file tool + an unknown tool; neither should emit.
	plan := `{"primary_intent":"file_analysis","tools_needed":["get_pdf_content","weather_api","bogus"],"location":"Goa","crop":"general"}`
	stream := []llm.StreamChunk{{Text: "done"}}
	p, _ := buildPipeline(plan, stream, false)
	st := agent.NewState("1", "", "q")

	evs := collect(p, st)
	var toolCalls int
	for _, e := range evs {
		if e.Type == agent.EventToolCall {
			toolCalls++
			if e.Tool != "weather_api" {
				t.Errorf("unexpected tool_call for %q", e.Tool)
			}
		}
	}
	if toolCalls != 1 {
		t.Fatalf("tool_call count = %d, want 1 (only weather_api)", toolCalls)
	}
}
