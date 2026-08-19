package chat_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	chathandler "github.com/smartkrishi/backend/internal/api/chat"
	"github.com/smartkrishi/backend/internal/domain"
	"github.com/smartkrishi/backend/internal/repository/postgres"
	authservice "github.com/smartkrishi/backend/internal/service/auth"
	chatservice "github.com/smartkrishi/backend/internal/service/chat"
)

// testEnv holds the router plus helpers for a live-DB API test.
type testEnv struct {
	router *chi.Mux
	users  *postgres.UserRepository
	tokens *authservice.TokenManager
	pool   *pgxpool.Pool
}

// setup builds the chat router against a real Postgres. It skips the test when
// no database URL is configured so `go test ./...` stays green in CI without a DB.
func setup(t *testing.T) *testEnv {
	t.Helper()
	dbURL := os.Getenv("TEST_DATABASE_URL")
	if dbURL == "" {
		dbURL = os.Getenv("DATABASE_URL")
	}
	if dbURL == "" {
		t.Skip("no TEST_DATABASE_URL/DATABASE_URL set; skipping chat API integration test")
	}

	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dbURL)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	if err := pool.Ping(ctx); err != nil {
		t.Skipf("cannot reach database (%v); skipping", err)
	}
	t.Cleanup(pool.Close)

	tokens, err := authservice.NewTokenManager("test-secret-key-at-least-32-characters-long", "HS256", 60)
	if err != nil {
		t.Fatalf("token manager: %v", err)
	}

	userRepo := postgres.NewUserRepository(pool)
	authSvc := authservice.NewService(userRepo, tokens)
	chatSvc := chatservice.NewService(postgres.NewChatRepository(pool))
	handler := chathandler.NewHandler(chatSvc, authSvc)

	r := chi.NewRouter()
	r.Route("/api/v1/chat", handler.Routes)

	return &testEnv{router: r, users: userRepo, tokens: tokens, pool: pool}
}

// createUser inserts a fresh email user and returns its ID and a Bearer token.
func (e *testEnv) createUser(t *testing.T) (int32, string) {
	t.Helper()
	email := fmt.Sprintf("chatapitest_%d_%s@example.com", time.Now().UnixNano(), uuid.NewString()[:8])
	hash := "x" // password not exercised in these tests
	user, err := e.users.Create(context.Background(), postgres.CreateUserParams{
		Name:           "Chat API Test",
		Email:          &email,
		HashedPassword: &hash,
		AuthProvider:   domain.AuthProviderEmail,
	})
	if err != nil {
		t.Fatalf("create user: %v", err)
	}
	t.Cleanup(func() {
		_, _ = e.pool.Exec(context.Background(), `DELETE FROM chat_messages WHERE user_id=$1`, user.ID)
		_, _ = e.pool.Exec(context.Background(), `DELETE FROM chats WHERE user_id=$1`, user.ID)
		_, _ = e.pool.Exec(context.Background(), `DELETE FROM users WHERE id=$1`, user.ID)
	})

	token, err := e.tokens.CreateAccessToken(email, string(domain.AuthProviderEmail))
	if err != nil {
		t.Fatalf("token: %v", err)
	}
	return user.ID, token
}

func (e *testEnv) do(t *testing.T, method, path, token, body string) *httptest.ResponseRecorder {
	t.Helper()
	var reader *strings.Reader
	if body != "" {
		reader = strings.NewReader(body)
	} else {
		reader = strings.NewReader("")
	}
	req := httptest.NewRequest(method, path, reader)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	e.router.ServeHTTP(rec, req)
	return rec
}

func TestChatCRUDFlow(t *testing.T) {
	e := setup(t)
	_, token := e.createUser(t)

	// Create with explicit title.
	rec := e.do(t, http.MethodPost, "/api/v1/chat/chats", token, `{"title":"How to grow rice"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("create: want 200, got %d (%s)", rec.Code, rec.Body.String())
	}
	var created domain.Chat
	if err := json.Unmarshal(rec.Body.Bytes(), &created); err != nil {
		t.Fatalf("decode create: %v", err)
	}
	if created.Title != "How to grow rice" || created.ID == uuid.Nil {
		t.Fatalf("unexpected created chat: %+v", created)
	}
	if created.AgentChatID != nil {
		t.Errorf("agent_chat_id should be nil on create, got %v", *created.AgentChatID)
	}

	// List includes the new chat.
	rec = e.do(t, http.MethodGet, "/api/v1/chat/chats?skip=0&limit=50", token, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("list: want 200, got %d", rec.Code)
	}
	var list []domain.ChatSummary
	if err := json.Unmarshal(rec.Body.Bytes(), &list); err != nil {
		t.Fatalf("decode list: %v", err)
	}
	if len(list) != 1 || list[0].ID != created.ID {
		t.Fatalf("list mismatch: %+v", list)
	}

	// Get returns messages as an array (never null).
	rec = e.do(t, http.MethodGet, "/api/v1/chat/chats/"+created.ID.String(), token, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("get: want 200, got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), `"messages":[]`) {
		t.Errorf("get should serialize empty messages array, got %s", rec.Body.String())
	}

	// Update title.
	rec = e.do(t, http.MethodPut, "/api/v1/chat/chats/"+created.ID.String(), token, `{"title":"Rice farming basics"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("update: want 200, got %d", rec.Code)
	}
	var updated domain.Chat
	_ = json.Unmarshal(rec.Body.Bytes(), &updated)
	if updated.Title != "Rice farming basics" {
		t.Errorf("update title mismatch: %q", updated.Title)
	}

	// Delete.
	rec = e.do(t, http.MethodDelete, "/api/v1/chat/chats/"+created.ID.String(), token, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("delete: want 200, got %d", rec.Code)
	}

	// Get after delete → 404.
	rec = e.do(t, http.MethodGet, "/api/v1/chat/chats/"+created.ID.String(), token, "")
	if rec.Code != http.StatusNotFound {
		t.Errorf("get after delete: want 404, got %d", rec.Code)
	}
}

func TestChatCrossUserIsolation(t *testing.T) {
	e := setup(t)
	_, tokenA := e.createUser(t)
	_, tokenB := e.createUser(t)

	// User A creates a chat.
	rec := e.do(t, http.MethodPost, "/api/v1/chat/chats", tokenA, `{"title":"A's chat"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("create: %d", rec.Code)
	}
	var chat domain.Chat
	_ = json.Unmarshal(rec.Body.Bytes(), &chat)

	// User B cannot see it (list empty) or fetch it (404).
	rec = e.do(t, http.MethodGet, "/api/v1/chat/chats", tokenB, "")
	var listB []domain.ChatSummary
	_ = json.Unmarshal(rec.Body.Bytes(), &listB)
	if len(listB) != 0 {
		t.Errorf("user B should see no chats, got %d", len(listB))
	}

	rec = e.do(t, http.MethodGet, "/api/v1/chat/chats/"+chat.ID.String(), tokenB, "")
	if rec.Code != http.StatusNotFound {
		t.Errorf("user B get A's chat: want 404, got %d", rec.Code)
	}

	// User B cannot update or delete it either.
	rec = e.do(t, http.MethodPut, "/api/v1/chat/chats/"+chat.ID.String(), tokenB, `{"title":"hijack"}`)
	if rec.Code != http.StatusNotFound {
		t.Errorf("user B update A's chat: want 404, got %d", rec.Code)
	}
	rec = e.do(t, http.MethodDelete, "/api/v1/chat/chats/"+chat.ID.String(), tokenB, "")
	if rec.Code != http.StatusNotFound {
		t.Errorf("user B delete A's chat: want 404, got %d", rec.Code)
	}
}

func TestChatValidationAndAuth(t *testing.T) {
	e := setup(t)
	_, token := e.createUser(t)

	// No auth → 401.
	rec := e.do(t, http.MethodGet, "/api/v1/chat/chats", "", "")
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("no auth: want 401, got %d", rec.Code)
	}

	// Bad UUID → 400.
	rec = e.do(t, http.MethodGet, "/api/v1/chat/chats/not-a-uuid", token, "")
	if rec.Code != http.StatusBadRequest {
		t.Errorf("bad uuid: want 400, got %d", rec.Code)
	}

	// Create a chat, then attempt empty-title update → 400.
	rec = e.do(t, http.MethodPost, "/api/v1/chat/chats", token, `{"title":"seed"}`)
	var chat domain.Chat
	_ = json.Unmarshal(rec.Body.Bytes(), &chat)

	rec = e.do(t, http.MethodPut, "/api/v1/chat/chats/"+chat.ID.String(), token, `{"title":"   "}`)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("empty title update: want 400, got %d (%s)", rec.Code, rec.Body.String())
	}

	// Missing-uuid path (empty) hits the collection route, not the item route,
	// so create with default title works.
	rec = e.do(t, http.MethodPost, "/api/v1/chat/chats", token, `{}`)
	if rec.Code != http.StatusOK {
		t.Errorf("create default title: want 200, got %d", rec.Code)
	}
	var def domain.Chat
	_ = json.Unmarshal(rec.Body.Bytes(), &def)
	if def.Title != "New Chat" {
		t.Errorf("default title: want 'New Chat', got %q", def.Title)
	}
}
