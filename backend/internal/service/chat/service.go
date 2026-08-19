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

// Service holds chat business logic. It is intentionally thin for Milestone 1
// (CRUD only); agent-chat creation and message/AI flows arrive with the
// streaming step.
type Service struct {
	chats *postgres.ChatRepository
}

func NewService(chats *postgres.ChatRepository) *Service {
	return &Service{chats: chats}
}

// CreateChat creates a chat for the user. If no title is supplied a default is used.
//
// TODO(Step 7 — streaming): agent_chat_id is intentionally left NULL here.
// Before /chat/send-stream can work, the streaming flow must call an
// ensureAgentChat step (mirroring Python ChatService.ensure_agent_chat) that
// lazily creates the agent-side chat and persists its ID onto this row.
func (s *Service) CreateChat(ctx context.Context, userID int32, req domain.CreateChatRequest) (*domain.Chat, error) {
	if req.Title == "" {
		req.Title = "New Chat"
	}
	return s.chats.Create(ctx, userID, req)
}

// ListChats returns paginated summaries. Bounds mirror the Python router
// (skip >= 0, 1 <= limit <= 100) with the same defaults.
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

// GenerateChatTitle mirrors ChatService.generate_chat_title: first 50 chars of
// the first message, with an ellipsis when truncated. Exposed for the streaming
// step which auto-titles new chats.
func GenerateChatTitle(firstMessage string) string {
	const maxLen = 50
	runes := []rune(firstMessage)
	// trim leading/trailing spaces like Python's str.strip()
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
