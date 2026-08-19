package postgres

import (
	"context"
	"encoding/json"
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
	pool  *pgxpool.Pool
	files *FileRepository // optional; when set, GetMessages populates message.Files
}

func NewChatRepository(pool *pgxpool.Pool) *ChatRepository {
	return &ChatRepository{pool: pool}
}

// WithFiles attaches a FileRepository so GetMessages populates each message's
// files array via the same batch-load used for reasoning steps. Kept optional
// so chat CRUD works without the file subsystem. Returns the repo for chaining.
func (r *ChatRepository) WithFiles(files *FileRepository) *ChatRepository {
	r.files = files
	return r
}

const chatColumns = `
	id, user_id, title, agent_chat_id, is_fallback_chat, fallback_phone_number,
	created_at, updated_at, is_deleted
`

// Create inserts a new chat. agent_chat_id is left NULL and populated later by
// the agent pipeline; the local chat is created first.
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
// Outer-joins messages to include the message count and latest content.
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

// GetMessages returns all messages for a chat, ordered by created_at. It
// populates ReasoningSteps and Files via best-effort batch loads.
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
	ids := make([]uuid.UUID, 0)
	for rows.Next() {
		var m domain.ChatMessage
		if err := rows.Scan(
			&m.ID, &m.ChatID, &m.UserID, &m.Role, &m.Content, &m.MessageType, &m.FileURL,
			&m.IsEdited, &m.OriginalContent, &m.FallbackType, &m.FallbackPhoneNumber,
			&m.CreatedAt, &m.EditedAt,
		); err != nil {
			return nil, fmt.Errorf("scan chat message: %w", err)
		}
		// Always serialize as arrays (never null); the frontend depends on this.
		m.ReasoningSteps = []domain.ReasoningStep{}
		m.Files = []domain.UploadedFile{}
		messages = append(messages, m)
		ids = append(ids, m.ID)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	// Attach persisted reasoning steps so a reloaded chat replays the agent's
	// reasoning. Best-effort: on error, messages keep empty arrays.
	if steps, err := r.reasoningStepsByMessage(ctx, ids); err == nil {
		for i := range messages {
			if s, ok := steps[messages[i].ID]; ok {
				messages[i].ReasoningSteps = s
			}
		}
	}

	// Attach uploaded files the same way, when a FileRepository is wired.
	// Best-effort: on error, messages keep empty file arrays.
	if r.files != nil {
		if fs, err := r.files.filesByMessage(ctx, ids); err == nil {
			for i := range messages {
				if f, ok := fs[messages[i].ID]; ok {
					messages[i].Files = f
				}
			}
		}
	}
	return messages, nil
}

// toJSONB marshals v for a JSONB column, returning nil (SQL NULL) for a nil
// value so empty payloads don't store the literal "null".
func toJSONB(v any) ([]byte, error) {
	if v == nil {
		return nil, nil
	}
	return json.Marshal(v)
}

// fromJSONB unmarshals a JSONB column into a generic value, returning nil for
// empty/NULL data.
func fromJSONB(b []byte) any {
	if len(b) == 0 {
		return nil
	}
	var v any
	if err := json.Unmarshal(b, &v); err != nil {
		return nil
	}
	return v
}

// SearchMessages returns messages across a user's non-deleted chats whose
// content matches any of the given lowercased terms (case-insensitive), most
// recent first, capped at limit. When chatID is non-nil the search is scoped to
// that single chat (still user-scoped); a nil chatID searches all of the user's
// chats. Terms are OR-matched.
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

// InsertReasoningStep persists one reasoning step. tool_result and step_metadata
// are stored as JSONB. Best-effort JSON marshaling: a nil value stores SQL NULL.
// Returns the inserted step id.
func (r *ChatRepository) InsertReasoningStep(ctx context.Context, in domain.ReasoningStepInput) (uuid.UUID, error) {
	toolResult, err := toJSONB(in.ToolResult)
	if err != nil {
		return uuid.Nil, fmt.Errorf("marshal tool_result: %w", err)
	}
	stepMeta, err := toJSONB(in.StepMetadata)
	if err != nil {
		return uuid.Nil, fmt.Errorf("marshal step_metadata: %w", err)
	}

	const insert = `
		INSERT INTO reasoning_steps
			(message_id, chat_id, user_id, step_type, step_order, stage, content,
			 tool_name, tool_args, tool_result, step_metadata)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)
		RETURNING id`

	var id uuid.UUID
	err = r.pool.QueryRow(ctx, insert,
		in.MessageID, in.ChatID, in.UserID, in.StepType, in.StepOrder, in.Stage, in.Content,
		in.ToolName, in.ToolArgs, toolResult, stepMeta,
	).Scan(&id)
	if err != nil {
		return uuid.Nil, fmt.Errorf("insert reasoning step: %w", err)
	}
	return id, nil
}

// reasoningStepsByMessage loads all reasoning steps for the given message ids,
// grouped by message id and ordered by step_order. Used to populate
// ChatMessage.ReasoningSteps in GetMessages.
func (r *ChatRepository) reasoningStepsByMessage(ctx context.Context, messageIDs []uuid.UUID) (map[uuid.UUID][]domain.ReasoningStep, error) {
	out := make(map[uuid.UUID][]domain.ReasoningStep)
	if len(messageIDs) == 0 {
		return out, nil
	}
	const q = `
		SELECT id, message_id, step_type, step_order, stage, content,
		       tool_name, tool_args, tool_result, step_metadata, created_at
		FROM reasoning_steps
		WHERE message_id = ANY($1)
		ORDER BY message_id, step_order`

	rows, err := r.pool.Query(ctx, q, messageIDs)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var (
			s          domain.ReasoningStep
			messageID  uuid.UUID
			toolResult []byte
			stepMeta   []byte
		)
		if err := rows.Scan(
			&s.ID, &messageID, &s.StepType, &s.StepOrder, &s.Stage, &s.Content,
			&s.ToolName, &s.ToolArgs, &toolResult, &stepMeta, &s.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan reasoning step: %w", err)
		}
		s.ToolResult = fromJSONB(toolResult)
		s.StepMetadata = fromJSONB(stepMeta)
		out[messageID] = append(out[messageID], s)
	}
	return out, rows.Err()
}

// AddMessage inserts a chat message and bumps the chat's updated_at. role is
// "user" or "assistant"; content is required. Returns the inserted message. It
// does not verify ownership — callers resolve and verify the chat (via GetByID)
// before writing.
func (r *ChatRepository) AddMessage(ctx context.Context, chatID uuid.UUID, userID int32, role, content string) (*domain.ChatMessage, error) {
	const insert = `
		INSERT INTO chat_messages (chat_id, user_id, role, content)
		VALUES ($1, $2, $3, $4)
		RETURNING id, chat_id, user_id, role, content, message_type, file_url,
		          is_edited, original_content, fallback_type, fallback_phone_number,
		          created_at, edited_at`

	row := r.pool.QueryRow(ctx, insert, chatID, userID, role, content)
	var m domain.ChatMessage
	if err := row.Scan(
		&m.ID, &m.ChatID, &m.UserID, &m.Role, &m.Content, &m.MessageType, &m.FileURL,
		&m.IsEdited, &m.OriginalContent, &m.FallbackType, &m.FallbackPhoneNumber,
		&m.CreatedAt, &m.EditedAt,
	); err != nil {
		return nil, fmt.Errorf("insert chat message: %w", err)
	}
	m.ReasoningSteps = []domain.ReasoningStep{}
	m.Files = []domain.UploadedFile{}

	// Bump the parent chat's updated_at (best-effort; ignore if the chat is gone).
	_, _ = r.pool.Exec(ctx, `UPDATE chats SET updated_at = now() WHERE id = $1`, chatID)
	return &m, nil
}

// UpdateMessageContent overwrites a message's content (used to fill the
// assistant placeholder with the final streamed answer). Best-effort: returns
// nil even if no row matched.
func (r *ChatRepository) UpdateMessageContent(ctx context.Context, messageID uuid.UUID, content string) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE chat_messages SET content = $2 WHERE id = $1`, messageID, content)
	return err
}

// SetAgentChatID stores the agent-side chat id on a chat if not already set,
// scoped to the user. Used by EnsureAgentChat. Returns ErrChatNotFound if no
// matching non-deleted chat exists for the user.
func (r *ChatRepository) SetAgentChatID(ctx context.Context, id uuid.UUID, userID int32, agentChatID string) error {
	tag, err := r.pool.Exec(ctx, `
		UPDATE chats
		SET agent_chat_id = $3, updated_at = now()
		WHERE id = $1 AND user_id = $2 AND is_deleted = false AND agent_chat_id IS NULL`,
		id, userID, agentChatID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		// Either the chat is gone, or agent_chat_id was already set — both are
		// non-fatal for the caller, which re-reads the chat. Distinguish a
		// missing chat only when needed; here treat 0 rows as "nothing to do".
		return nil
	}
	return nil
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
