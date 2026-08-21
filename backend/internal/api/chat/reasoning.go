package chat

import (
	"errors"
	"net/http"

	"github.com/smartkrishi/backend/internal/api"
	"github.com/smartkrishi/backend/internal/domain"
	chatservice "github.com/smartkrishi/backend/internal/service/chat"
)

// chatReasoning handles GET /chat/chats/{id}/reasoning: returns all persisted
// reasoning steps for a chat, so the UI can replay the agent's reasoning on
// chat reload. Ownership-scoped (another user's chat → 404).
func (h *Handler) chatReasoning(w http.ResponseWriter, r *http.Request) {
	userID, ok := h.userID(w, r)
	if !ok {
		return
	}
	chatID, ok := parseChatID(w, r)
	if !ok {
		return
	}
	steps, err := h.chats.ChatReasoning(r.Context(), chatID, userID)
	if errors.Is(err, chatservice.ErrNotFound) {
		api.WriteError(w, http.StatusNotFound, "Chat not found")
		return
	}
	if err != nil {
		api.WriteError(w, http.StatusInternalServerError, "Failed to get chat reasoning")
		return
	}
	api.WriteJSON(w, http.StatusOK, map[string]any{
		"chat_id":         chatID.String(),
		"reasoning_steps": nonNilSteps(steps),
	})
}

// messageReasoning handles GET /chat/messages/{id}/reasoning: returns the
// reasoning steps for a single message. Ownership-scoped (another user's
// message → 404).
func (h *Handler) messageReasoning(w http.ResponseWriter, r *http.Request) {
	userID, ok := h.userID(w, r)
	if !ok {
		return
	}
	msgID, ok := parseChatID(w, r) // {id} URL param; parses any UUID
	if !ok {
		return
	}
	steps, err := h.chats.MessageReasoning(r.Context(), msgID, userID)
	if errors.Is(err, chatservice.ErrMessageNotFound) {
		api.WriteError(w, http.StatusNotFound, "Message not found")
		return
	}
	if err != nil {
		api.WriteError(w, http.StatusInternalServerError, "Failed to get message reasoning")
		return
	}
	api.WriteJSON(w, http.StatusOK, map[string]any{
		"message_id":      msgID.String(),
		"reasoning_steps": nonNilSteps(steps),
	})
}

// nonNilSteps ensures the JSON array is [] not null.
func nonNilSteps(steps []domain.ReasoningStep) []domain.ReasoningStep {
	if steps == nil {
		return []domain.ReasoningStep{}
	}
	return steps
}
