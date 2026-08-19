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

	chathandler "github.com/smartkrishi/backend/internal/api/chat"
	"github.com/smartkrishi/backend/internal/domain"
	"github.com/smartkrishi/backend/internal/repository/postgres"
	authservice "github.com/smartkrishi/backend/internal/service/auth"
	chatservice "github.com/smartkrishi/backend/internal/service/chat"
)

func TestSuggestions(t *testing.T) {
	e := setup(t)
	_, token := e.createUser(t)
	router := e.routerWithAgent(&fakeRunner{})

	rec := e.doReq(t, router, http.MethodGet, "/api/v1/chat/suggestions", token, "", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	var resp struct {
		Suggestions []map[string]string `json:"suggestions"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(resp.Suggestions) != 6 {
		t.Fatalf("suggestions = %d, want 6", len(resp.Suggestions))
	}
}

func TestAsk(t *testing.T) {
	e := setup(t)
	_, token := e.createUser(t)
	runner := &fakeRunner{askAnswer: "Grow paddy in June."}
	router := e.routerWithAgent(runner)

	rec := e.doReq(t, router, http.MethodPost, "/api/v1/chat/ask", token,
		`{"message":"when to grow paddy?","chat_history":[{"role":"user","content":"hi"}]}`, "application/json")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d (%s)", rec.Code, rec.Body.String())
	}
	var resp domain.ChatResponseLegacy
	_ = json.Unmarshal(rec.Body.Bytes(), &resp)
	if resp.Response != "Grow paddy in June." {
		t.Fatalf("response = %q", resp.Response)
	}
	if runner.lastAskMessage != "when to grow paddy?" {
		t.Errorf("ask message = %q", runner.lastAskMessage)
	}
	if len(runner.lastAskHistory) != 1 {
		t.Errorf("history len = %d, want 1", len(runner.lastAskHistory))
	}
}

func TestAsk_EmptyMessage(t *testing.T) {
	e := setup(t)
	_, token := e.createUser(t)
	router := e.routerWithAgent(&fakeRunner{})
	rec := e.doReq(t, router, http.MethodPost, "/api/v1/chat/ask", token, `{"message":"  "}`, "application/json")
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
}

func TestSend_PersistsBothMessages(t *testing.T) {
	e := setup(t)
	userID, token := e.createUser(t)
	runner := &fakeRunner{askAnswer: "Use drip irrigation."}
	router := e.routerWithAgent(runner)

	rec := e.doReq(t, router, http.MethodPost, "/api/v1/chat/send", token, `{"message":"how to irrigate?"}`, "application/json")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d (%s)", rec.Code, rec.Body.String())
	}
	var resp domain.SendMessageResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.Response != "Use drip irrigation." || resp.ChatID == uuid.Nil || resp.MessageID == uuid.Nil {
		t.Fatalf("bad response: %+v", resp)
	}

	// Both user + assistant messages saved.
	var count int
	if err := e.pool.QueryRow(context.Background(), `SELECT count(*) FROM chat_messages WHERE chat_id=$1 AND user_id=$2`, resp.ChatID, userID).Scan(&count); err != nil {
		t.Fatalf("count: %v", err)
	}
	if count != 2 {
		t.Fatalf("messages = %d, want 2 (user + assistant)", count)
	}
	// The returned message_id is the assistant message.
	var role string
	_ = e.pool.QueryRow(context.Background(), `SELECT role FROM chat_messages WHERE id=$1`, resp.MessageID).Scan(&role)
	if role != "assistant" {
		t.Errorf("returned message_id role = %q, want assistant", role)
	}
}

func TestSend_ExistingChat(t *testing.T) {
	e := setup(t)
	userID, token := e.createUser(t)
	router := e.routerWithAgent(&fakeRunner{askAnswer: "ok"})

	chat, _ := chatservice.NewService(postgres.NewChatRepository(e.pool)).CreateChat(context.Background(), userID, domain.CreateChatRequest{Title: "Existing"})
	rec := e.doReq(t, router, http.MethodPost, "/api/v1/chat/send", token, `{"message":"hi","chat_id":"`+chat.ID.String()+`"}`, "application/json")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d (%s)", rec.Code, rec.Body.String())
	}
	var resp domain.SendMessageResponse
	_ = json.Unmarshal(rec.Body.Bytes(), &resp)
	if resp.ChatID != chat.ID {
		t.Errorf("chat_id = %s, want existing %s", resp.ChatID, chat.ID)
	}
}

func TestAnalyzeImage(t *testing.T) {
	e := setup(t)
	_, token := e.createUser(t)
	runner := &fakeRunner{imageAnswer: "Leaf blight detected."}
	router := e.routerWithAgent(runner)

	// PNG magic bytes so mimeForImage returns image/png.
	png := append([]byte{0x89}, []byte("PNG\r\n\x1a\n")...)
	body, ct := buildMultipart(t, "file", "leaf.png", png, map[string]string{"message": "what's wrong?"})
	rec := e.doMultipart(t, router, "/api/v1/chat/analyze-image", token, body, ct)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d (%s)", rec.Code, rec.Body.String())
	}
	var resp domain.ChatResponseLegacy
	_ = json.Unmarshal(rec.Body.Bytes(), &resp)
	if resp.Response != "Leaf blight detected." {
		t.Fatalf("response = %q", resp.Response)
	}
	if runner.lastImageMIME != "image/png" {
		t.Errorf("mime = %q, want image/png", runner.lastImageMIME)
	}
}

func TestAnalyzeImage_RejectsNonImage(t *testing.T) {
	e := setup(t)
	_, token := e.createUser(t)
	router := e.routerWithAgent(&fakeRunner{})
	body, ct := buildMultipart(t, "file", "notes.pdf", []byte("%PDF"), map[string]string{"message": "x"})
	rec := e.doMultipart(t, router, "/api/v1/chat/analyze-image", token, body, ct)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400 for non-image", rec.Code)
	}
}

func TestAnalyzeImagePersistent(t *testing.T) {
	e := setup(t)
	_, token := e.createUser(t)
	runner := &fakeRunner{imageAnswer: "Healthy crop."}
	// Use the files-enabled router so the file is saved + linked.
	router, _ := e.routerWithAgentAndFiles(t, runner)

	jpg := []byte("\xff\xd8\xff\xe0 jpeg")
	body, ct := buildMultipart(t, "file", "field.jpg", jpg, map[string]string{"message": "assess"})
	rec := e.doMultipart(t, router, "/api/v1/chat/analyze-image-persistent", token, body, ct)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d (%s)", rec.Code, rec.Body.String())
	}
	var resp domain.SendMessageResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.Response != "Healthy crop." || resp.ChatID == uuid.Nil {
		t.Fatalf("bad response: %+v", resp)
	}

	ctx := context.Background()
	// user + assistant messages saved.
	var msgCount int
	_ = e.pool.QueryRow(ctx, `SELECT count(*) FROM chat_messages WHERE chat_id=$1`, resp.ChatID).Scan(&msgCount)
	if msgCount != 2 {
		t.Fatalf("messages = %d, want 2", msgCount)
	}
	// file saved + linked to the user message.
	var fileCount int
	_ = e.pool.QueryRow(ctx, `SELECT count(*) FROM uploaded_files WHERE chat_id=$1 AND message_id IS NOT NULL`, resp.ChatID).Scan(&fileCount)
	if fileCount != 1 {
		t.Fatalf("linked files = %d, want 1", fileCount)
	}
}

func TestLegacy_AgentNotConfigured(t *testing.T) {
	e := setup(t)
	_, token := e.createUser(t)
	// Router WITHOUT an agent.
	authSvc := authservice.NewService(e.users, e.tokens)
	chatSvc := chatservice.NewService(postgres.NewChatRepository(e.pool))
	handler := chathandler.NewHandler(chatSvc, authSvc)
	router := chi.NewRouter()
	router.Route("/api/v1/chat", handler.Routes)

	rec := e.doReq(t, router, http.MethodPost, "/api/v1/chat/ask", token, `{"message":"hi"}`, "application/json")
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503 when agent not configured", rec.Code)
	}
	// suggestions is always available (static).
	rec = e.doReq(t, router, http.MethodGet, "/api/v1/chat/suggestions", token, "", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("suggestions status = %d, want 200 (static)", rec.Code)
	}
}

// --- small test helpers ---

func (e *testEnv) doReq(t *testing.T, router http.Handler, method, path, token, body, contentType string) *httptest.ResponseRecorder {
	t.Helper()
	var r *http.Request
	if body != "" {
		r = httptest.NewRequest(method, path, strings.NewReader(body))
	} else {
		r = httptest.NewRequest(method, path, nil)
	}
	if token != "" {
		r.Header.Set("Authorization", "Bearer "+token)
	}
	if contentType != "" {
		r.Header.Set("Content-Type", contentType)
	}
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, r)
	return rec
}

func (e *testEnv) doMultipart(t *testing.T, router http.Handler, path, token string, body interface{ Bytes() []byte }, contentType string) *httptest.ResponseRecorder {
	t.Helper()
	r := httptest.NewRequest(http.MethodPost, path, strings.NewReader(string(body.Bytes())))
	r.Header.Set("Authorization", "Bearer "+token)
	r.Header.Set("Content-Type", contentType)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, r)
	return rec
}
