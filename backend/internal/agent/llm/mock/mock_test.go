package mock_test

import (
	"context"
	"errors"
	"testing"

	"github.com/smartkrishi/backend/internal/agent/llm"
	"github.com/smartkrishi/backend/internal/agent/llm/mock"
)

func TestGenerate_DefaultText(t *testing.T) {
	p := &mock.Provider{GenerateText: "hello"}
	resp, err := p.Generate(context.Background(), llm.Request{Prompt: "hi"}, llm.Opts{Model: "m"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Text != "hello" {
		t.Fatalf("got %q, want %q", resp.Text, "hello")
	}
	if p.Calls != 1 {
		t.Fatalf("Calls = %d, want 1", p.Calls)
	}
	if p.LastOpts.Model != "m" || p.LastRequest.Prompt != "hi" {
		t.Fatalf("recorded request/opts wrong: %+v %+v", p.LastRequest, p.LastOpts)
	}
}

func TestGenerate_Func(t *testing.T) {
	p := &mock.Provider{
		GenerateFunc: func(_ context.Context, req llm.Request, _ llm.Opts) (llm.Response, error) {
			return llm.Response{Text: "echo:" + req.Prompt}, nil
		},
	}
	resp, err := p.Generate(context.Background(), llm.Request{Prompt: "x"}, llm.Opts{Model: "m"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Text != "echo:x" {
		t.Fatalf("got %q", resp.Text)
	}
}

func TestGenerateStream_ScriptedChunks(t *testing.T) {
	want := []llm.StreamChunk{
		{Text: "thinking...", Thought: true},
		{Text: "answer ", Thought: false},
		{Text: "text", Thought: false},
		{Grounding: &llm.Grounding{WebSearchQueries: []string{"wheat price"}}},
		{Code: &llm.CodeExecution{Stage: "code", Code: "print(1)", Language: "python"}},
	}
	p := &mock.Provider{StreamChunks: want}

	var got []llm.StreamChunk
	for c, err := range p.GenerateStream(context.Background(), llm.Request{Prompt: "q"}, llm.Opts{Model: "m"}) {
		if err != nil {
			t.Fatalf("unexpected stream error: %v", err)
		}
		got = append(got, c)
	}

	if len(got) != len(want) {
		t.Fatalf("got %d chunks, want %d", len(got), len(want))
	}
	if !got[0].Thought || got[0].Text != "thinking..." {
		t.Errorf("chunk0 = %+v, want thought", got[0])
	}
	if got[3].Grounding == nil || len(got[3].Grounding.WebSearchQueries) != 1 {
		t.Errorf("chunk3 grounding wrong: %+v", got[3])
	}
	if got[4].Code == nil || got[4].Code.Stage != "code" {
		t.Errorf("chunk4 code wrong: %+v", got[4])
	}
}

func TestGenerateStream_DefaultSingleChunk(t *testing.T) {
	p := &mock.Provider{GenerateText: "only"}
	var got []llm.StreamChunk
	for c, err := range p.GenerateStream(context.Background(), llm.Request{}, llm.Opts{Model: "m"}) {
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		got = append(got, c)
	}
	if len(got) != 1 || got[0].Text != "only" {
		t.Fatalf("got %+v, want single 'only' chunk", got)
	}
}

func TestGenerateStream_Error(t *testing.T) {
	sentinel := errors.New("boom")
	p := &mock.Provider{StreamErr: sentinel}
	var gotErr error
	for _, err := range p.GenerateStream(context.Background(), llm.Request{}, llm.Opts{Model: "m"}) {
		if err != nil {
			gotErr = err
		}
	}
	if !errors.Is(gotErr, sentinel) {
		t.Fatalf("got %v, want sentinel", gotErr)
	}
}

func TestGenerateStream_ContextCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	p := &mock.Provider{StreamChunks: []llm.StreamChunk{{Text: "a"}, {Text: "b"}}}
	var gotErr error
	for _, err := range p.GenerateStream(ctx, llm.Request{}, llm.Opts{Model: "m"}) {
		if err != nil {
			gotErr = err
			break
		}
	}
	if !errors.Is(gotErr, context.Canceled) {
		t.Fatalf("got %v, want context.Canceled", gotErr)
	}
}
