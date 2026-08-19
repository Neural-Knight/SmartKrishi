package postgres

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/smartkrishi/backend/internal/agent/tools"
	"github.com/smartkrishi/backend/internal/domain"
)

var ErrChatNotFound = errors.New("chat not found")

// ChatRepository provides the read surface the agent chat_history tool needs.
var _ tools.MessageReader = (*ChatRepository)(nil)

type ChatRepository struct {
	pool *pgxpool.Pool
}

func NewChatRepository(pool *pgxpool.Pool) *ChatRepository {
	return &ChatRepository{pool: pool}
}

const chatColumns = `
	id, user_id, title, agent_chat_id, is_fallback_chat, fallback_phone_number,
	created_at, updated_at, is_deleted
`

// Create inserts a new chat. agent_chat_id is left NULL (populated later by the
// agent pipeline), matching the Python flow where the local chat is created first.
func (r *ChatRepository) Create(ctx context.Context, userID int32, req domain.CreateChatRequest) (*domain.Chat, error) {
	query := `
		INSERT INTO chats (user_id, title, is_fallback_chat, fallback_phone_number)
		VALUES ($1, $2, $3, $4)
		RETURNING ` + chatColumns

	row := r.pool.QueryRow(ctx, query, userID, req.Title, req.IsFallbackChat, req.FallbackPhoneNumber)
	return scanChat(row)
}

// GetByID returns a non-deleted chat scoped to the user, or ErrChatNotFound.
func (r *ChatRepository) GetByID(ctx context.Context, id uuid.UUID, userID int32) (*domain.Chat, error) {
	query := `SELECT ` + chatColumns + `
		FROM chats
		WHERE id = $1 AND user_id = $2 AND is_deleted = false`
	row := r.pool.QueryRow(ctx, query, id, userID)
	chat, err := scanChat(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrChatNotFound
	}
	return chat, err
}

// ListSummaries returns paginated chat summaries for a user, ordered by updated_at DESC.
// Mirrors ChatService.get_user_chats: outer join messages, count + latest content.
func (r *ChatRepository) ListSummaries(ctx context.Context, userID int32, skip, limit int32) ([]domain.ChatSummary, error) {
	query := `
		SELECT
			c.id, c.title, c.created_at, c.updated_at,
			c.is_fallback_chat, c.fallback_phone_number,
			COUNT(m.id) AS message_count,
			COALESCE(MAX(m.content), '') AS last_message
		FROM chats c
		LEFT JOIN chat_messages m ON m.chat_id = c.id
		WHERE c.user_id = $1 AND c.is_deleted = false
		GROUP BY c.id, c.title, c.created_at, c.updated_at, c.is_fallback_chat, c.fallback_phone_number
		ORDER BY c.updated_at DESC
		OFFSET $2 LIMIT $3`

	rows, err := r.pool.Query(ctx, query, userID, skip, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	summaries := make([]domain.ChatSummary, 0)
	for rows.Next() {
		var s domain.ChatSummary
		if err := rows.Scan(
			&s.ID, &s.Title, &s.CreatedAt, &s.UpdatedAt,
			&s.IsFallbackChat, &s.FallbackPhoneNumber,
			&s.MessageCount, &s.LastMessage,
		); err != nil {
			return nil, fmt.Errorf("scan chat summary: %w", err)
		}
		summaries = append(summaries, s)
	}
	return summaries, rows.Err()
}

// GetMessages returns all messages for a chat, ordered by created_at (matches the
// SQLAlchemy relationship order_by).
func (r *ChatRepository) GetMessages(ctx context.Context, chatID uuid.UUID) ([]domain.ChatMessage, error) {
	query := `
		SELECT id, chat_id, user_id, role, content, message_type, file_url,
		       is_edited, original_content, fallback_type, fallback_phone_number,
		       created_at, edited_at
		FROM chat_messages
		WHERE chat_id = $1
		ORDER BY created_at`

	rows, err := r.pool.Query(ctx, query, chatID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	messages := make([]domain.ChatMessage, 0)
	for rows.Next() {
		var m domain.ChatMessage
		if err := rows.Scan(
			&m.ID, &m.ChatID, &m.UserID, &m.Role, &m.Content, &m.MessageType, &m.FileURL,
			&m.IsEdited, &m.OriginalContent, &m.FallbackType, &m.FallbackPhoneNumber,
			&m.CreatedAt, &m.EditedAt,
		); err != nil {
			return nil, fmt.Errorf("scan chat message: %w", err)
		}
		// Always serialize as arrays (never null) for Python Pydantic parity.
		m.ReasoningSteps = []domain.ReasoningStep{}
		m.Files = []domain.UploadedFile{}
		messages = append(messages, m)
	}
	return messages, rows.Err()
}

// SearchMessages returns messages across a user's non-deleted chats whose
// content matches any of the given lowercased terms (case-insensitive), most
// recent first, capped at limit. When chatID is non-nil the search is scoped to
// that single chat (still user-scoped). Mirrors the Python search_messages,
// which OR-matches query terms; chatID=None searches all of the user's chats.
//
// terms must be non-empty; callers split the query. If terms is empty this
// returns no rows (an all-match would not be a "search").
func (r *ChatRepository) SearchMessages(ctx context.Context, userID int32, terms []string, chatID *uuid.UUID, limit int32) ([]domain.ChatMessage, error) {
	if len(terms) == 0 {
		return []domain.ChatMessage{}, nil
	}
	if limit <= 0 {
		limit = 10
	}

	// Build an OR of ILIKE '%term%' predicates. Args: $1 userID, $2 limit,
	// then one arg per term; chatID (if any) is the final arg.
	args := []any{userID, limit}
	var likes []string
	for _, t := range terms {
		args = append(args, "%"+t+"%")
		likes = append(likes, fmt.Sprintf("m.content ILIKE $%d", len(args)))
	}

	chatFilter := ""
	if chatID != nil {
		args = append(args, *chatID)
		chatFilter = fmt.Sprintf("AND m.chat_id = $%d", len(args))
	}

	query := fmt.Sprintf(`
		SELECT m.id, m.chat_id, m.user_id, m.role, m.content, m.message_type, m.file_url,
		       m.is_edited, m.original_content, m.fallback_type, m.fallback_phone_number,
		       m.created_at, m.edited_at
		FROM chat_messages m
		JOIN chats c ON c.id = m.chat_id
		WHERE c.user_id = $1 AND c.is_deleted = false %s AND (%s)
		ORDER BY m.created_at DESC
		LIMIT $2`, chatFilter, strings.Join(likes, " OR "))

	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	messages := make([]domain.ChatMessage, 0)
	for rows.Next() {
		var m domain.ChatMessage
		if err := rows.Scan(
			&m.ID, &m.ChatID, &m.UserID, &m.Role, &m.Content, &m.MessageType, &m.FileURL,
			&m.IsEdited, &m.OriginalContent, &m.FallbackType, &m.FallbackPhoneNumber,
			&m.CreatedAt, &m.EditedAt,
		); err != nil {
			return nil, fmt.Errorf("scan chat message: %w", err)
		}
		m.ReasoningSteps = []domain.ReasoningStep{}
		m.Files = []domain.UploadedFile{}
		messages = append(messages, m)
	}
	return messages, rows.Err()
}

// UpdateTitle updates a chat's title if owned by the user and not deleted.
func (r *ChatRepository) UpdateTitle(ctx context.Context, id uuid.UUID, userID int32, title string) (*domain.Chat, error) {
	query := `
		UPDATE chats
		SET title = $3, updated_at = now()
		WHERE id = $1 AND user_id = $2 AND is_deleted = false
		RETURNING ` + chatColumns
	row := r.pool.QueryRow(ctx, query, id, userID, title)
	chat, err := scanChat(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrChatNotFound
	}
	return chat, err
}

// SoftDelete marks a chat deleted; returns ErrChatNotFound if nothing matched.
func (r *ChatRepository) SoftDelete(ctx context.Context, id uuid.UUID, userID int32) error {
	tag, err := r.pool.Exec(ctx, `
		UPDATE chats
		SET is_deleted = true
		WHERE id = $1 AND user_id = $2 AND is_deleted = false`, id, userID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrChatNotFound
	}
	return nil
}

func scanChat(row scannable) (*domain.Chat, error) {
	var c domain.Chat
	err := row.Scan(
		&c.ID, &c.UserID, &c.Title, &c.AgentChatID, &c.IsFallbackChat,
		&c.FallbackPhoneNumber, &c.CreatedAt, &c.UpdatedAt, &c.IsDeleted,
	)
	if err != nil {
		return nil, fmt.Errorf("scan chat: %w", err)
	}
	c.Messages = []domain.ChatMessage{}
	return &c, nil
}
