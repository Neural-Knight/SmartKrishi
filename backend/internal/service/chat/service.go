package chat

import (
	"context"
	"errors"
	"strings"

	"github.com/google/uuid"

	"github.com/smartkrishi/backend/internal/domain"
	"github.com/smartkrishi/backend/internal/repository/postgres"
)

// Service-level sentinels so the API layer depends on the service package, not
// the repository package.
var (
	// ErrNotFound is returned when a chat does not exist or is not owned by the user.
	ErrNotFound = errors.New("chat not found")
	// ErrEmptyTitle is returned when an update supplies a blank/whitespace title.
	ErrEmptyTitle = errors.New("title cannot be empty")
)

// Service holds chat business logic: CRUD, agent-chat creation, and the
// message/AI flows used by streaming.
type Service struct {
	chats *postgres.ChatRepository
}

func NewService(chats *postgres.ChatRepository) *Service {
	return &Service{chats: chats}
}

// CreateChat creates a chat for the user. If no title is supplied a default is
// used. agent_chat_id is left NULL and populated lazily by EnsureAgentChat on
// the first streaming turn.
func (s *Service) CreateChat(ctx context.Context, userID int32, req domain.CreateChatRequest) (*domain.Chat, error) {
	if req.Title == "" {
		req.Title = "New Chat"
	}
	return s.chats.Create(ctx, userID, req)
}

// EnsureChatForStream resolves the chat for a streaming turn: it verifies an
// existing chat_id belongs to the user, or creates a new chat auto-titled from
// the first message. It then runs EnsureAgentChat so agent_chat_id is populated.
// Returns the resolved chat.
func (s *Service) EnsureChatForStream(ctx context.Context, userID int32, chatID *uuid.UUID, firstMessage string) (*domain.Chat, error) {
	var chat *domain.Chat
	if chatID != nil {
		c, err := s.chats.GetByID(ctx, *chatID, userID)
		if errors.Is(err, postgres.ErrChatNotFound) {
			return nil, ErrNotFound
		}
		if err != nil {
			return nil, err
		}
		chat = c
	} else {
		c, err := s.chats.Create(ctx, userID, domain.CreateChatRequest{Title: GenerateChatTitle(firstMessage)})
		if err != nil {
			return nil, err
		}
		chat = c
	}
	return s.EnsureAgentChat(ctx, userID, chat)
}

// EnsureAgentChat guarantees the chat has an agent_chat_id. The agent runs
// in-process, so there is no remote chat to create: agent_chat_id is set to the
// chat's own id, which satisfies the schema and any downstream that keys on it.
// Idempotent.
func (s *Service) EnsureAgentChat(ctx context.Context, userID int32, chat *domain.Chat) (*domain.Chat, error) {
	if chat.AgentChatID != nil && *chat.AgentChatID != "" {
		return chat, nil
	}
	agentChatID := chat.ID.String()
	if err := s.chats.SetAgentChatID(ctx, chat.ID, userID, agentChatID); err != nil {
		return nil, err
	}
	chat.AgentChatID = &agentChatID
	return chat, nil
}

// AddMessage persists a chat message (role "user" or "assistant").
func (s *Service) AddMessage(ctx context.Context, chatID uuid.UUID, userID int32, role, content string) (*domain.ChatMessage, error) {
	return s.chats.AddMessage(ctx, chatID, userID, role, content)
}

// UpdateMessage overwrites an existing message's content (used to fill the
// assistant placeholder with the final streamed answer).
func (s *Service) UpdateMessage(ctx context.Context, messageID uuid.UUID, content string) error {
	return s.chats.UpdateMessageContent(ctx, messageID, content)
}

// SaveReasoningStep persists one reasoning step. Best-effort at the call site:
// the streaming handler ignores the error so persistence never breaks the live
// stream.
func (s *Service) SaveReasoningStep(ctx context.Context, in domain.ReasoningStepInput) error {
	_, err := s.chats.InsertReasoningStep(ctx, in)
	return err
}

// RecentHistory returns up to limit recent messages for a chat, oldest-first,
// for use as agent context.
func (s *Service) RecentHistory(ctx context.Context, chatID uuid.UUID, limit int) ([]domain.ChatMessage, error) {
	msgs, err := s.chats.GetMessages(ctx, chatID)
	if err != nil {
		return nil, err
	}
	if limit > 0 && len(msgs) > limit {
		msgs = msgs[len(msgs)-limit:]
	}
	return msgs, nil
}

// ListChats returns paginated summaries. Bounds are clamped to skip >= 0 and
// 1 <= limit <= 100.
func (s *Service) ListChats(ctx context.Context, userID int32, skip, limit int32) ([]domain.ChatSummary, error) {
	if skip < 0 {
		skip = 0
	}
	if limit < 1 {
		limit = 50
	}
	if limit > 100 {
		limit = 100
	}
	return s.chats.ListSummaries(ctx, userID, skip, limit)
}

// GetChat returns a chat with its messages, scoped to the user.
func (s *Service) GetChat(ctx context.Context, id uuid.UUID, userID int32) (*domain.Chat, error) {
	chat, err := s.chats.GetByID(ctx, id, userID)
	if errors.Is(err, postgres.ErrChatNotFound) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	messages, err := s.chats.GetMessages(ctx, chat.ID)
	if err != nil {
		return nil, err
	}
	chat.Messages = messages
	return chat, nil
}

// UpdateChat updates a chat's title. Rejects blank/whitespace titles.
func (s *Service) UpdateChat(ctx context.Context, id uuid.UUID, userID int32, title string) (*domain.Chat, error) {
	if strings.TrimSpace(title) == "" {
		return nil, ErrEmptyTitle
	}
	chat, err := s.chats.UpdateTitle(ctx, id, userID, title)
	if errors.Is(err, postgres.ErrChatNotFound) {
		return nil, ErrNotFound
	}
	return chat, err
}

// DeleteChat soft-deletes a chat.
func (s *Service) DeleteChat(ctx context.Context, id uuid.UUID, userID int32) error {
	err := s.chats.SoftDelete(ctx, id, userID)
	if errors.Is(err, postgres.ErrChatNotFound) {
		return ErrNotFound
	}
	return err
}

// GenerateChatTitle builds a chat title from the first 50 chars of the first
// message, with an ellipsis when truncated. Used to auto-title new chats on the
// first streaming turn.
func GenerateChatTitle(firstMessage string) string {
	const maxLen = 50
	runes := []rune(firstMessage)
	// trim leading/trailing whitespace
	start, end := 0, len(runes)
	for start < end && isSpace(runes[start]) {
		start++
	}
	for end > start && isSpace(runes[end-1]) {
		end--
	}
	trimmed := runes[start:end]

	title := string(trimmed)
	if len(trimmed) > maxLen {
		title = string(trimmed[:maxLen]) + "..."
	}
	if title == "" {
		return "New Chat"
	}
	return title
}

func isSpace(r rune) bool {
	return r == ' ' || r == '\t' || r == '\n' || r == '\r' || r == '\v' || r == '\f'
}
