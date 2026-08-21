// Package file holds the uploaded-file business logic: validation, local-disk
// storage, Gemini File API upload, and metadata persistence.
//
// The Gemini File API is the only document/image analysis path: there is no
// vector store, and no doc/xlsx conversion — bytes are uploaded to Gemini
// directly. Unsupported types are recorded with processing_status="failed" and
// a user-visible message rather than silently dropped.
package file

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/smartkrishi/backend/internal/agent/files"
	"github.com/smartkrishi/backend/internal/domain"
	"github.com/smartkrishi/backend/internal/repository/postgres"
)

// Service-level sentinels so the API layer depends on this package, not the repo.
var (
	// ErrNotFound is returned when a file does not exist or is not owned by the user.
	ErrNotFound = errors.New("file not found")
	// ErrTooLarge is returned when a file exceeds the size limit.
	ErrTooLarge = errors.New("file too large")
	// ErrUnsupportedType is returned when a file's extension is not allowed.
	ErrUnsupportedType = errors.New("unsupported file type")
	// ErrEmpty is returned for an empty file.
	ErrEmpty = errors.New("empty file")
)

// Size limits (bytes) matching the frontend.
const (
	MaxUploadFileBytes = 10 << 20 // 10MB for POST /upload-file
	MaxAnalyzeBytes    = 20 << 20 // 20MB for POST /upload-and-analyze-stream
)

// allowedExts matches the frontend's accepted types.
var allowedExts = map[string]struct{}{
	"png": {}, "jpg": {}, "jpeg": {}, "webp": {}, "heic": {}, "heif": {},
	"pdf": {}, "docx": {}, "xlsx": {}, "csv": {},
}

// geminiSupportedExts are the types we upload to the Gemini File API directly.
// docx/xlsx are accepted for storage but NOT sent to Gemini (no conversion) —
// they are recorded processing_status="failed" with a clear message.
var geminiSupportedExts = map[string]struct{}{
	"png": {}, "jpg": {}, "jpeg": {}, "webp": {}, "heic": {}, "heif": {},
	"pdf": {}, "csv": {},
}

// Service stores uploaded files (disk + Gemini) and persists their metadata.
type Service struct {
	files      *postgres.FileRepository
	store      files.Store // nil ⇒ Gemini upload skipped (no GEMINI_API_KEY)
	uploadsDir string
}

// NewService builds the file service. store may be nil (files are still saved
// to disk + DB, but not to Gemini and cannot be analyzed).
func NewService(files *postgres.FileRepository, store files.Store, uploadsDir string) *Service {
	if uploadsDir == "" {
		uploadsDir = "uploads"
	}
	return &Service{files: files, store: store, uploadsDir: uploadsDir}
}

// Validate checks extension and size, returning a sentinel error on failure.
func Validate(filename string, size int64, maxBytes int64) error {
	if size <= 0 {
		return ErrEmpty
	}
	if size > maxBytes {
		return fmt.Errorf("%w: %d bytes (max %d)", ErrTooLarge, size, maxBytes)
	}
	if _, ok := allowedExts[extOf(filename)]; !ok {
		return fmt.Errorf("%w: %s", ErrUnsupportedType, extOf(filename))
	}
	return nil
}

// Save stores the file bytes on disk, uploads to Gemini when supported, and
// inserts the metadata row. It returns the persisted file. A Gemini upload
// failure (or an unsupported-for-Gemini type) does not fail the call: the row
// is saved with processing_status="failed" and a summary message, so the file
// is still listed and the user sees why analysis is unavailable.
func (s *Service) Save(ctx context.Context, userID int32, chatID uuid.UUID, messageID *uuid.UUID, filename string, data []byte) (*domain.UploadedFile, error) {
	ext := extOf(filename)
	mime := mimeForExt(ext)

	// 1. Local disk: uploads/{user_id}/{uuid}_{filename}
	localPath, err := s.saveToDisk(userID, filename, data)
	if err != nil {
		return nil, fmt.Errorf("save to disk: %w", err)
	}

	in := domain.UploadedFileInput{
		UserID:           userID,
		ChatID:           chatID,
		MessageID:        messageID,
		OriginalFilename: filename,
		FileType:         ext,
		FileSize:         int64(len(data)),
		MimeType:         strPtr(mime),
		ProcessingStatus: "uploaded",
		FileMetadata:     strPtr(fmt.Sprintf(`{"local_path":%q}`, localPath)),
	}

	// 2. Gemini File API upload (supported types only).
	if _, ok := geminiSupportedExts[ext]; !ok {
		in.ProcessingStatus = "failed"
		in.Summary = strPtr(fmt.Sprintf("Analysis unavailable: %s files are not supported for AI analysis in this build.", strings.ToUpper(ext)))
	} else if s.store != nil {
		up, err := s.store.Upload(ctx, chatID.String(), data, mime, filename)
		if err != nil {
			in.ProcessingStatus = "failed"
			in.Summary = strPtr("File saved but AI upload failed; analysis unavailable.")
		} else {
			in.AgentFileID = strPtr(up.FileID)
			in.ProcessingStatus = "processed"
		}
	} else {
		// No Gemini configured: stored but not analyzable.
		in.ProcessingStatus = "uploaded"
	}

	// 3. Persist metadata.
	return s.files.Insert(ctx, in)
}

// GeminiFileTTL is the lifetime of a file in the Gemini File API. After this,
// an uploaded file's agent reference is dead and analysis must re-upload.
const GeminiFileTTL = 48 * time.Hour

// SweepExpired clears dead Gemini file references for uploads older than the
// File API TTL, marking them processing_status="expired". The local disk copy
// and DB row are kept. Best-effort; returns the number of rows marked.
func (s *Service) SweepExpired(ctx context.Context) (int64, error) {
	return s.files.ExpireStaleGeminiFiles(ctx, GeminiFileTTL)
}

// ListByChat returns the user's files for a chat.
func (s *Service) ListByChat(ctx context.Context, chatID uuid.UUID, userID int32) ([]domain.UploadedFile, error) {
	return s.files.ListByChat(ctx, chatID, userID)
}

// LinkToMessage sets a file's message_id, associating an already-saved file
// with the message it belongs to (used by upload-and-analyze once the user
// message exists). Best-effort; scoped to the user.
func (s *Service) LinkToMessage(ctx context.Context, fileID uuid.UUID, userID int32, messageID uuid.UUID) error {
	return s.files.SetMessageID(ctx, fileID, userID, messageID)
}

// GetByID returns a file scoped to the user.
func (s *Service) GetByID(ctx context.Context, id uuid.UUID, userID int32) (*domain.UploadedFile, error) {
	f, err := s.files.GetByID(ctx, id, userID)
	if errors.Is(err, postgres.ErrFileNotFound) {
		return nil, ErrNotFound
	}
	return f, err
}

func (s *Service) saveToDisk(userID int32, filename string, data []byte) (string, error) {
	dir := filepath.Join(s.uploadsDir, fmt.Sprintf("%d", userID))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	// Prefix with a UUID to avoid collisions; keep the original base name.
	safe := filepath.Base(filename)
	path := filepath.Join(dir, uuid.NewString()+"_"+safe)
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return "", err
	}
	return path, nil
}

func extOf(filename string) string {
	e := strings.ToLower(filepath.Ext(filename))
	return strings.TrimPrefix(e, ".")
}

func mimeForExt(ext string) string {
	switch ext {
	case "png":
		return "image/png"
	case "jpg", "jpeg":
		return "image/jpeg"
	case "webp":
		return "image/webp"
	case "heic":
		return "image/heic"
	case "heif":
		return "image/heif"
	case "pdf":
		return "application/pdf"
	case "csv":
		return "text/csv"
	case "docx":
		return "application/vnd.openxmlformats-officedocument.wordprocessingml.document"
	case "xlsx":
		return "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"
	default:
		return "application/octet-stream"
	}
}

func strPtr(s string) *string { return &s }
