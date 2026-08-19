package agent

import (
	"encoding/json"
	"strings"
	"testing"
)

// TestMarshalNDJSON_WireFormat locks the on-the-wire JSON shape for each event
// type against the frontend contract (useStreamingChat.ts). A drift here would
// silently break the unchanged frontend.
func TestMarshalNDJSON_WireFormat(t *testing.T) {
	cases := []struct {
		name     string
		event    Event
		wantType string
		wantKeys []string // keys that MUST be present
		absent   []string // keys that must NOT be present
	}{
		{
			name:     "plan",
			event:    planEvent(Plan{PrimaryIntent: "weather_forecast", ToolsNeeded: []string{"weather_api"}}, "raw text"),
			wantType: "plan",
			wantKeys: []string{"plan", "raw_response"},
			absent:   []string{"content", "tool", "response"},
		},
		{
			name:     "tool_call",
			event:    toolCallEvent("weather_api", "Pune", map[string]any{"loc": "Pune"}),
			wantType: "tool_call",
			wantKeys: []string{"tool", "args", "result"},
			absent:   []string{"content", "plan"},
		},
		{
			name:     "thinking",
			event:    thinkingEvent("hmm"),
			wantType: "thinking",
			wantKeys: []string{"content"},
			absent:   []string{"response", "tool"},
		},
		{
			name:     "response_chunk",
			event:    responseChunkEvent("hello"),
			wantType: "response_chunk",
			wantKeys: []string{"content"},
		},
		{
			name:     "code_execution code",
			event:    codeEvent("print(1)", "python"),
			wantType: "code_execution",
			wantKeys: []string{"stage", "code", "language"},
			absent:   []string{"outcome"},
		},
		{
			name:     "code_execution result",
			event:    codeResultEvent("OK", "1"),
			wantType: "code_execution",
			wantKeys: []string{"stage", "outcome", "result"},
			absent:   []string{"code"},
		},
		{
			name:     "response",
			event:    responseEvent("final", &GroundingMetadata{WebSearchQueries: []string{"q"}}),
			wantType: "response",
			wantKeys: []string{"response", "grounding_metadata"},
		},
		{
			name:     "error",
			event:    errorEvent("boom"),
			wantType: "error",
			wantKeys: []string{"error"},
		},
		{
			name:     "end",
			event:    endEvent(),
			wantType: "end",
			wantKeys: []string{},
			absent:   []string{"content", "response", "stage"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			raw, err := tc.event.MarshalNDJSON()
			if err != nil {
				t.Fatalf("marshal: %v", err)
			}
			if !strings.HasSuffix(string(raw), "\n") {
				t.Fatalf("NDJSON line must end with newline: %q", raw)
			}
			var m map[string]any
			if err := json.Unmarshal(raw, &m); err != nil {
				t.Fatalf("unmarshal: %v", err)
			}
			if m["type"] != tc.wantType {
				t.Errorf("type = %v, want %s", m["type"], tc.wantType)
			}
			for _, k := range tc.wantKeys {
				if _, ok := m[k]; !ok {
					t.Errorf("missing key %q in %s", k, raw)
				}
			}
			for _, k := range tc.absent {
				if _, ok := m[k]; ok {
					t.Errorf("unexpected key %q in %s", k, raw)
				}
			}
		})
	}
}

func TestGroundingStreamEvents_SkipsEmpty(t *testing.T) {
	if evs := groundingStreamEvents(nil); evs != nil {
		t.Errorf("nil grounding should yield no events, got %v", evs)
	}
}
