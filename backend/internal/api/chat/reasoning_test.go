package chat_test

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/google/uuid"

	"github.com/smartkrishi/backend/internal/domain"
	"github.com/smartkrishi/backend/internal/repository/postgres"
	chatservice "github.com/smartkrishi/backend/internal/service/chat"
)

func TestChatReasoning_Endpoint(t *testing.T) {
	e := setup(t)
	userID, token := e.createUser(t)
	router := e.routerWithAgent(&fakeRunner{})
	ctx := context.Background()

	chatRepo := postgres.NewChatRepository(e.pool)
	chat, _ := chatservice.NewService(chatRepo).CreateChat(ctx, userID, domain.CreateChatRequest{Title: "R"})
	msg, err := chatRepo.AddMessage(ctx, chat.ID, userID, "assistant", "answer")
	if err != nil {
		t.Fatalf("add message: %v", err)
	}
	// Insert two reasoning steps for the message.
	for i, st := range []string{"plan", "tool_call"} {
		if _, err := chatRepo.InsertReasoningStep(ctx, domain.ReasoningStepInput{
			MessageID: msg.ID, ChatID: chat.ID, UserID: userID,
			StepType: st, StepOrder: int32(i + 1), Content: strPtr2("step " + st),
		}); err != nil {
			t.Fatalf("insert step: %v", err)
		}
	}

	rec := e.doReq(t, router, http.MethodGet, "/api/v1/chat/chats/"+chat.ID.String()+"/reasoning", token, "", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d (%s)", rec.Code, rec.Body.String())
	}
	var resp struct {
		ChatID         string                 `json:"chat_id"`
		ReasoningSteps []domain.ReasoningStep `json:"reasoning_steps"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.ChatID != chat.ID.String() || len(resp.ReasoningSteps) != 2 {
		t.Fatalf("reasoning = %+v", resp)
	}
}

func TestChatReasoning_OtherUser404(t *testing.T) {
	e := setup(t)
	ownerID, _ := e.createUser(t)
	_, otherToken := e.createUser(t)
	router := e.routerWithAgent(&fakeRunner{})

	chat, _ := chatservice.NewService(postgres.NewChatRepository(e.pool)).CreateChat(context.Background(), ownerID, domain.CreateChatRequest{Title: "Owner"})
	rec := e.doReq(t, router, http.MethodGet, "/api/v1/chat/chats/"+chat.ID.String()+"/reasoning", otherToken, "", "")
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404 for another user's chat", rec.Code)
	}
}

func TestMessageReasoning_Endpoint(t *testing.T) {
	e := setup(t)
	userID, token := e.createUser(t)
	router := e.routerWithAgent(&fakeRunner{})
	ctx := context.Background()

	chatRepo := postgres.NewChatRepository(e.pool)
	chat, _ := chatservice.NewService(chatRepo).CreateChat(ctx, userID, domain.CreateChatRequest{Title: "M"})
	msg, _ := chatRepo.AddMessage(ctx, chat.ID, userID, "assistant", "answer")
	if _, err := chatRepo.InsertReasoningStep(ctx, domain.ReasoningStepInput{
		MessageID: msg.ID, ChatID: chat.ID, UserID: userID,
		StepType: "thinking", StepOrder: 1, Content: strPtr2("pondering"),
	}); err != nil {
		t.Fatalf("insert step: %v", err)
	}

	rec := e.doReq(t, router, http.MethodGet, "/api/v1/chat/messages/"+msg.ID.String()+"/reasoning", token, "", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d (%s)", rec.Code, rec.Body.String())
	}
	var resp struct {
		MessageID      string                 `json:"message_id"`
		ReasoningSteps []domain.ReasoningStep `json:"reasoning_steps"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &resp)
	if resp.MessageID != msg.ID.String() || len(resp.ReasoningSteps) != 1 {
		t.Fatalf("reasoning = %+v", resp)
	}
}

func TestMessageReasoning_UnknownMessage404(t *testing.T) {
	e := setup(t)
	_, token := e.createUser(t)
	router := e.routerWithAgent(&fakeRunner{})
	rec := e.doReq(t, router, http.MethodGet, "/api/v1/chat/messages/"+uuid.NewString()+"/reasoning", token, "", "")
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404 for unknown message", rec.Code)
	}
}

func TestAgentTools_Endpoint(t *testing.T) {
	e := setup(t)
	_, token := e.createUser(t)
	router := e.routerWithAgent(&fakeRunner{})

	rec := e.doReq(t, router, http.MethodGet, "/api/v1/chat/agent-tools", token, "", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	var resp struct {
		Tools []map[string]any `json:"tools"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &resp)
	if len(resp.Tools) == 0 {
		t.Fatalf("expected non-empty tools list")
	}
}

func TestAgentConfig_Endpoint(t *testing.T) {
	e := setup(t)
	_, token := e.createUser(t)
	router := e.routerWithAgent(&fakeRunner{})

	rec := e.doReq(t, router, http.MethodGet, "/api/v1/chat/agent-config", token, "", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	var cfg map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &cfg)
	if cfg["agent_model"] == nil {
		t.Fatalf("expected agent_model in config, got %v", cfg)
	}
}

func strPtr2(s string) *string { return &s }
