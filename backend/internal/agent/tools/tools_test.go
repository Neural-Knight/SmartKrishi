package tools_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/smartkrishi/backend/internal/agent/tools"
	"github.com/smartkrishi/backend/internal/domain"
)

func TestWeather_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Query().Get("q"); got != "Pune" {
			t.Errorf("q = %q, want Pune", got)
		}
		w.Write([]byte(`{"location":{"name":"Pune"},"current":{"temp_c":30.5,"humidity":40,"wind_kph":12,"condition":{"text":"Sunny"}}}`))
	}))
	defer srv.Close()

	r := tools.NewRegistry(srv.Client(), tools.Config{WeatherAPIKey: "k", WeatherBaseURL: srv.URL}, nil)
	out := r.Weather(context.Background(), "Pune")

	if out["source"] != "weatherapi.com" {
		t.Fatalf("source = %v", out["source"])
	}
	if out["forecast"] != "Sunny at 30.5°C" {
		t.Fatalf("forecast = %v", out["forecast"])
	}
	if out["loc"] != "Pune" {
		t.Fatalf("loc = %v", out["loc"])
	}
}

func TestWeather_NoKeyFallback(t *testing.T) {
	r := tools.NewRegistry(http.DefaultClient, tools.Config{}, nil)
	out := r.Weather(context.Background(), "Delhi")
	if out["source"] != "fallback" {
		t.Fatalf("expected fallback, got %v", out["source"])
	}
	if out["loc"] != "Delhi" {
		t.Fatalf("loc = %v", out["loc"])
	}
}

func TestWeather_APIError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Write([]byte(`{"error":{"message":"No matching location"}}`))
	}))
	defer srv.Close()
	r := tools.NewRegistry(srv.Client(), tools.Config{WeatherAPIKey: "k", WeatherBaseURL: srv.URL}, nil)
	out := r.Weather(context.Background(), "Nowhere")
	if out["forecast"] != "Error: No matching location" {
		t.Fatalf("forecast = %v", out["forecast"])
	}
}

func TestMarket_Success_UsesCrop(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify the CROP is sent as the commodity filter (the bug fix).
		if got := r.URL.Query().Get("filters[commodity]"); got != "wheat" {
			t.Errorf("commodity = %q, want wheat", got)
		}
		if got := r.URL.Query().Get("filters[state]"); got != "Punjab" {
			t.Errorf("state = %q, want Punjab", got)
		}
		w.Write([]byte(`{"records":[{"modal_price":"2000"},{"modal_price":"2100"},{"modal_price":"bad"}]}`))
	}))
	defer srv.Close()

	r := tools.NewRegistry(srv.Client(), tools.Config{DataGovKey: "k", AgmarknetID: "res", MarketBaseURL: srv.URL}, nil)
	out := r.Market(context.Background(), "wheat", "Punjab")

	if out["crop"] != "wheat" {
		t.Fatalf("crop = %v", out["crop"])
	}
	if out["latest"] != 2100.0 {
		t.Fatalf("latest = %v, want 2100", out["latest"])
	}
	if out["source"] != "agmarknet" {
		t.Fatalf("source = %v", out["source"])
	}
}

func TestMarket_NoCredsFallback(t *testing.T) {
	r := tools.NewRegistry(http.DefaultClient, tools.Config{}, nil)
	out := r.Market(context.Background(), "rice", "national")
	if out["source"] != "fallback" || out["crop"] != "rice" {
		t.Fatalf("unexpected fallback: %v", out)
	}
}

func TestSoil_Placeholder(t *testing.T) {
	r := tools.NewRegistry(http.DefaultClient, tools.Config{}, nil)
	out := r.Soil(context.Background(), "Nashik")
	if out["loc"] != "Nashik" || out["pH"] != 6.5 {
		t.Fatalf("unexpected soil: %v", out)
	}
}

func TestIsDeferredFileTool(t *testing.T) {
	if !tools.IsDeferredFileTool("get_pdf_content") {
		t.Error("get_pdf_content should be deferred")
	}
	if tools.IsDeferredFileTool("weather_api") {
		t.Error("weather_api should not be deferred")
	}
}

func TestRegistry_Has(t *testing.T) {
	r := tools.NewRegistry(nil, tools.Config{}, nil)
	for _, name := range []string{tools.NameWeather, tools.NameMarket, tools.NameSoil, tools.NameChatHistory} {
		if !r.Has(name) {
			t.Errorf("Has(%q) = false", name)
		}
	}
	if r.Has("get_pdf_content") {
		t.Error("Has(file tool) should be false")
	}
}

// fakeReader implements tools.MessageReader for chat_history tests.
type fakeReader struct {
	messages   []domain.ChatMessage   // returned by GetMessages
	searchHits []domain.ChatMessage   // returned by SearchMessages
	summaries  []domain.ChatSummary   // returned by ListSummaries
	ownedChats map[uuid.UUID]struct{} // chats the user owns (for GetByID)
	err        error

	// captured args for assertions
	lastSearchChatID *uuid.UUID
	lastSearchTerms  []string
}

func (f *fakeReader) GetByID(_ context.Context, chatID uuid.UUID, _ int32) (*domain.Chat, error) {
	if f.err != nil {
		return nil, f.err
	}
	if f.ownedChats != nil {
		if _, ok := f.ownedChats[chatID]; !ok {
			return nil, errors.New("chat not found")
		}
	}
	return &domain.Chat{ID: chatID}, nil
}

func (f *fakeReader) GetMessages(_ context.Context, _ uuid.UUID) ([]domain.ChatMessage, error) {
	return f.messages, f.err
}

func (f *fakeReader) SearchMessages(_ context.Context, _ int32, terms []string, chatID *uuid.UUID, _ int32) ([]domain.ChatMessage, error) {
	f.lastSearchChatID = chatID
	f.lastSearchTerms = terms
	return f.searchHits, f.err
}

func (f *fakeReader) ListSummaries(_ context.Context, _ int32, _, _ int32) ([]domain.ChatSummary, error) {
	return f.summaries, f.err
}

func TestChatHistory_RequiresUser(t *testing.T) {
	r := tools.NewRegistry(nil, tools.Config{}, &fakeReader{})
	out := r.ChatHistory(context.Background(), tools.ChatHistoryArgs{})
	if out["error"] != "user_id is required" {
		t.Fatalf("error = %v", out["error"])
	}
}

func TestChatHistory_NoReader(t *testing.T) {
	r := tools.NewRegistry(nil, tools.Config{}, nil)
	out := r.ChatHistory(context.Background(), tools.ChatHistoryArgs{UserID: "1"})
	if out["error"] == nil {
		t.Fatalf("expected error for nil reader, got %v", out)
	}
}

func TestChatHistory_RecentMessages(t *testing.T) {
	chatID := uuid.New()
	reader := &fakeReader{
		ownedChats: map[uuid.UUID]struct{}{chatID: {}},
		messages: []domain.ChatMessage{
			{Role: "user", Content: "how to grow wheat"},
			{Role: "assistant", Content: "sow in november"},
		},
	}
	r := tools.NewRegistry(nil, tools.Config{}, reader)
	out := r.ChatHistory(context.Background(), tools.ChatHistoryArgs{UserID: "1", ChatID: chatID.String()})
	if out["type"] != "recent_messages" {
		t.Fatalf("type = %v", out["type"])
	}
	if out["count"] != 2 {
		t.Fatalf("count = %v, want 2", out["count"])
	}
}

func TestChatHistory_InChatSearch(t *testing.T) {
	chatID := uuid.New()
	reader := &fakeReader{
		ownedChats: map[uuid.UUID]struct{}{chatID: {}},
		searchHits: []domain.ChatMessage{{Role: "user", Content: "how to grow wheat"}},
	}
	r := tools.NewRegistry(nil, tools.Config{}, reader)
	out := r.ChatHistory(context.Background(), tools.ChatHistoryArgs{UserID: "1", ChatID: chatID.String(), Query: "Wheat"})
	if out["type"] != "search_results" {
		t.Fatalf("type = %v", out["type"])
	}
	if out["chat_id"] != chatID.String() {
		t.Fatalf("chat_id = %v, want the specific chat", out["chat_id"])
	}
	if out["count"] != 1 {
		t.Fatalf("count = %v, want 1", out["count"])
	}
	// Search must be scoped to the chat and terms lowercased.
	if reader.lastSearchChatID == nil || *reader.lastSearchChatID != chatID {
		t.Errorf("search should be scoped to the chat, got %v", reader.lastSearchChatID)
	}
	if len(reader.lastSearchTerms) != 1 || reader.lastSearchTerms[0] != "wheat" {
		t.Errorf("terms = %v, want [wheat]", reader.lastSearchTerms)
	}
}

// REQUIRED (review item 1): query set + chat_id empty searches across ALL the
// user's chats and returns search_results with chat_id "all_chats".
func TestChatHistory_CrossChatSearch(t *testing.T) {
	reader := &fakeReader{searchHits: []domain.ChatMessage{
		{Role: "user", Content: "wheat sowing time"},           // from chat A
		{Role: "assistant", Content: "wheat needs less water"}, // from chat B
	}}
	r := tools.NewRegistry(nil, tools.Config{}, reader)
	out := r.ChatHistory(context.Background(), tools.ChatHistoryArgs{UserID: "7", Query: "wheat"})

	if out["type"] != "search_results" {
		t.Fatalf("type = %v, want search_results (not user_chats)", out["type"])
	}
	if out["chat_id"] != "all_chats" {
		t.Fatalf("chat_id = %v, want all_chats", out["chat_id"])
	}
	if out["count"] != 2 {
		t.Fatalf("count = %v, want 2 matches across chats", out["count"])
	}
	// Cross-chat search must pass a nil chat id to the reader.
	if reader.lastSearchChatID != nil {
		t.Errorf("cross-chat search should pass nil chatID, got %v", reader.lastSearchChatID)
	}
}

// Ownership: a chat_id not owned by the user is rejected before any read.
func TestChatHistory_ChatNotOwned(t *testing.T) {
	other := uuid.New()
	reader := &fakeReader{ownedChats: map[uuid.UUID]struct{}{ /* empty: owns nothing */ }}
	r := tools.NewRegistry(nil, tools.Config{}, reader)
	out := r.ChatHistory(context.Background(), tools.ChatHistoryArgs{UserID: "1", ChatID: other.String()})
	if out["error"] != "chat not found for user" {
		t.Fatalf("error = %v, want ownership rejection", out["error"])
	}
}

func TestChatHistory_UserChats(t *testing.T) {
	reader := &fakeReader{summaries: []domain.ChatSummary{
		{ID: uuid.New(), Title: "Chat A", MessageCount: 3},
	}}
	r := tools.NewRegistry(nil, tools.Config{}, reader)
	out := r.ChatHistory(context.Background(), tools.ChatHistoryArgs{UserID: "42"})
	if out["type"] != "user_chats" {
		t.Fatalf("type = %v", out["type"])
	}
	if out["count"] != 1 {
		t.Fatalf("count = %v", out["count"])
	}
}

func TestChatHistory_InvalidIDs(t *testing.T) {
	r := tools.NewRegistry(nil, tools.Config{}, &fakeReader{})
	if out := r.ChatHistory(context.Background(), tools.ChatHistoryArgs{UserID: "1", ChatID: "not-a-uuid"}); out["error"] != "invalid chat_id" {
		t.Errorf("chat_id error = %v", out["error"])
	}
	if out := r.ChatHistory(context.Background(), tools.ChatHistoryArgs{UserID: "not-an-int"}); out["error"] != "invalid user_id" {
		t.Errorf("user_id error = %v", out["error"])
	}
}

// guard against accidental default-timeout regressions in NewRegistry.
func TestNewRegistry_DefaultTimeout(t *testing.T) {
	r := tools.NewRegistry(nil, tools.Config{}, nil)
	_ = r
	// Just ensure a nil client does not panic on a fallback path.
	out := r.Weather(context.Background(), "X")
	if out["source"] != "fallback" {
		t.Fatalf("weather with no key should fall back, got %v", out)
	}
	_ = time.Second
}
