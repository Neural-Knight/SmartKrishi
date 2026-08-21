package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/smartkrishi/backend/internal/domain"
)

// ErrFileNotFound is returned when an uploaded file does not exist or is not
// owned by the user.
var ErrFileNotFound = errors.New("file not found")

// FileRepository persists uploaded-file metadata. Raw bytes live on disk
// (uploads/) and in the Gemini File API; this table holds the metadata.
type FileRepository struct {
	pool *pgxpool.Pool
}

func NewFileRepository(pool *pgxpool.Pool) *FileRepository {
	return &FileRepository{pool: pool}
}

const fileColumns = `
	id, user_id, chat_id, message_id, original_filename, file_type, file_size,
	mime_type, agent_file_id, processing_status, summary, file_metadata,
	created_at, updated_at, is_deleted`

// Insert stores a new uploaded-file row and returns it.
func (r *FileRepository) Insert(ctx context.Context, in domain.UploadedFileInput) (*domain.UploadedFile, error) {
	const q = `
		INSERT INTO uploaded_files
			(user_id, chat_id, message_id, original_filename, file_type, file_size,
			 mime_type, agent_file_id, processing_status, summary, file_metadata)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)
		RETURNING ` + fileColumns

	row := r.pool.QueryRow(ctx, q,
		in.UserID, in.ChatID, in.MessageID, in.OriginalFilename, in.FileType, in.FileSize,
		in.MimeType, in.AgentFileID, in.ProcessingStatus, in.Summary, in.FileMetadata,
	)
	return scanFile(row)
}

// GetByID returns a non-deleted file scoped to the user, or ErrFileNotFound.
func (r *FileRepository) GetByID(ctx context.Context, id uuid.UUID, userID int32) (*domain.UploadedFile, error) {
	const q = `SELECT ` + fileColumns + ` FROM uploaded_files
		WHERE id = $1 AND user_id = $2 AND is_deleted = false`
	f, err := scanFile(r.pool.QueryRow(ctx, q, id, userID))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrFileNotFound
	}
	return f, err
}

// ListByChat returns the user's non-deleted files for a chat, oldest-first.
func (r *FileRepository) ListByChat(ctx context.Context, chatID uuid.UUID, userID int32) ([]domain.UploadedFile, error) {
	const q = `SELECT ` + fileColumns + ` FROM uploaded_files
		WHERE chat_id = $1 AND user_id = $2 AND is_deleted = false
		ORDER BY created_at`
	rows, err := r.pool.Query(ctx, q, chatID, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	files := make([]domain.UploadedFile, 0)
	for rows.Next() {
		f, err := scanFileRow(rows)
		if err != nil {
			return nil, err
		}
		files = append(files, *f)
	}
	return files, rows.Err()
}

// SetMessageID links a file to a message, scoped to the user and only when not
// already linked. Best-effort: returns nil even if no row matched.
func (r *FileRepository) SetMessageID(ctx context.Context, id uuid.UUID, userID int32, messageID uuid.UUID) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE uploaded_files SET message_id = $3, updated_at = now()
		WHERE id = $1 AND user_id = $2 AND message_id IS NULL`,
		id, userID, messageID)
	return err
}

// ExpireStaleGeminiFiles marks rows whose Gemini upload has passed the File API
// TTL: it clears agent_file_id and sets processing_status='expired' (with a
// user-visible summary) for any still-referencing row older than olderThan. The
// local disk copy and the DB row are kept — only the dead Gemini reference is
// cleared, so file tools won't try to use it. Returns the number of rows marked.
func (r *FileRepository) ExpireStaleGeminiFiles(ctx context.Context, olderThan time.Duration) (int64, error) {
	cutoff := time.Now().Add(-olderThan)
	tag, err := r.pool.Exec(ctx, `
		UPDATE uploaded_files
		SET agent_file_id = NULL,
		    processing_status = 'expired',
		    summary = COALESCE(summary, 'Uploaded file expired from the AI file store; re-upload to analyze again.'),
		    updated_at = now()
		WHERE agent_file_id IS NOT NULL
		  AND is_deleted = false
		  AND created_at < $1`, cutoff)
	if err != nil {
		return 0, err
	}
	return tag.RowsAffected(), nil
}

// filesByMessage batch-loads files grouped by message id, used to populate
// ChatMessage.Files in GetMessages.
func (r *FileRepository) filesByMessage(ctx context.Context, messageIDs []uuid.UUID) (map[uuid.UUID][]domain.UploadedFile, error) {
	out := make(map[uuid.UUID][]domain.UploadedFile)
	if len(messageIDs) == 0 {
		return out, nil
	}
	const q = `SELECT ` + fileColumns + ` FROM uploaded_files
		WHERE message_id = ANY($1) AND is_deleted = false
		ORDER BY message_id, created_at`
	rows, err := r.pool.Query(ctx, q, messageIDs)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		f, err := scanFileRow(rows)
		if err != nil {
			return nil, err
		}
		if f.MessageID != nil {
			out[*f.MessageID] = append(out[*f.MessageID], *f)
		}
	}
	return out, rows.Err()
}

type fileScanner interface {
	Scan(dest ...any) error
}

func scanFile(row fileScanner) (*domain.UploadedFile, error) {
	return scanFileRow(row)
}

func scanFileRow(row fileScanner) (*domain.UploadedFile, error) {
	var f domain.UploadedFile
	if err := row.Scan(
		&f.ID, &f.UserID, &f.ChatID, &f.MessageID, &f.OriginalFilename, &f.FileType,
		&f.FileSize, &f.MimeType, &f.AgentFileID, &f.ProcessingStatus, &f.Summary,
		&f.FileMetadata, &f.CreatedAt, &f.UpdatedAt, &f.IsDeleted,
	); err != nil {
		return nil, fmt.Errorf("scan uploaded file: %w", err)
	}
	return &f, nil
}
