package tools

import (
	"context"
	"strconv"
	"strings"

	"github.com/google/uuid"

	"github.com/smartkrishi/backend/internal/domain"
)

// FileReader is the narrow read surface the file tools need from the uploaded-
// file store. The file service / repository satisfies it. Scoped by user.
type FileReader interface {
	ListByChat(ctx context.Context, chatID uuid.UUID, userID int32) ([]domain.UploadedFile, error)
	GetByID(ctx context.Context, id uuid.UUID, userID int32) (*domain.UploadedFile, error)
}

// FileToolArgs are the parameters for the file tools. IDs arrive as strings
// from agent.State; userID is the integer user PK as a string.
type FileToolArgs struct {
	UserID   string
	ChatID   string
	FileID   string // specific file (get_pdf_content / get_image_analysis)
	Question string // query for Q&A / search
}

// ListUploadedFiles returns the chat's files (list_uploaded_files) from Postgres.
func (r *Registry) ListUploadedFiles(ctx context.Context, args FileToolArgs) map[string]any {
	if !r.filesEnabled() {
		return map[string]any{"error": "file tools unavailable"}
	}
	userID, chatID, errMap := r.resolveIDs(args.UserID, args.ChatID)
	if errMap != nil {
		return errMap
	}
	list, err := r.fileReader.ListByChat(ctx, chatID, userID)
	if err != nil {
		return map[string]any{"error": "failed to list files: " + err.Error()}
	}
	return map[string]any{
		"type":  "uploaded_files",
		"files": fileSummaries(list),
		"count": len(list),
	}
}

// SearchUserFiles keyword-searches the chat's files by filename/summary
// (search_user_files): case-insensitive substring matching over Postgres
// filename/summary metadata. There is no vector search.
func (r *Registry) SearchUserFiles(ctx context.Context, args FileToolArgs) map[string]any {
	if !r.filesEnabled() {
		return map[string]any{"error": "file tools unavailable"}
	}
	userID, chatID, errMap := r.resolveIDs(args.UserID, args.ChatID)
	if errMap != nil {
		return errMap
	}
	list, err := r.fileReader.ListByChat(ctx, chatID, userID)
	if err != nil {
		return map[string]any{"error": "failed to search files: " + err.Error()}
	}
	terms := strings.Fields(strings.ToLower(args.Question))
	matches := make([]map[string]any, 0)
	for _, f := range list {
		hay := strings.ToLower(f.OriginalFilename)
		if f.Summary != nil {
			hay += " " + strings.ToLower(*f.Summary)
		}
		if len(terms) == 0 || matchesAnyTerm(hay, terms) {
			matches = append(matches, fileSummary(f))
		}
	}
	return map[string]any{
		"type":    "file_search_results",
		"query":   args.Question,
		"matches": matches,
		"count":   len(matches),
	}
}

// AskAboutFile answers a question about a specific uploaded file via the Gemini
// File API (get_pdf_content / ask_question_about_files / get_image_analysis).
// If no FileID is given it uses the chat's most recent processed file.
func (r *Registry) AskAboutFile(ctx context.Context, args FileToolArgs) map[string]any {
	if !r.filesEnabled() {
		return map[string]any{"error": "file tools unavailable"}
	}
	userID, chatID, errMap := r.resolveIDs(args.UserID, args.ChatID)
	if errMap != nil {
		return errMap
	}

	var target *domain.UploadedFile
	if args.FileID != "" {
		fid, err := uuid.Parse(args.FileID)
		if err != nil {
			return map[string]any{"error": "invalid file_id"}
		}
		f, err := r.fileReader.GetByID(ctx, fid, userID)
		if err != nil {
			return map[string]any{"error": "file not found"}
		}
		target = f
	} else {
		list, err := r.fileReader.ListByChat(ctx, chatID, userID)
		if err != nil {
			return map[string]any{"error": "failed to load files: " + err.Error()}
		}
		target = latestProcessed(list)
		if target == nil {
			return map[string]any{"error": "no analyzable file found in this chat"}
		}
	}

	if target.AgentFileID == nil || *target.AgentFileID == "" {
		return map[string]any{
			"error":    "file is not available for analysis",
			"filename": target.OriginalFilename,
			"status":   target.ProcessingStatus,
		}
	}

	mime := ""
	if target.MimeType != nil {
		mime = *target.MimeType
	}
	answer, err := r.fileStore.Ask(ctx, chatID.String(), *target.AgentFileID, "", mime, r.agentModel, args.Question)
	if err != nil {
		return map[string]any{"error": "file analysis failed: " + err.Error(), "filename": target.OriginalFilename}
	}
	return map[string]any{
		"type":     "file_answer",
		"file_id":  target.ID.String(),
		"filename": target.OriginalFilename,
		"question": args.Question,
		"answer":   answer,
	}
}

// --- helpers ---------------------------------------------------------------

func (r *Registry) resolveIDs(userIDStr, chatIDStr string) (int32, uuid.UUID, map[string]any) {
	if strings.TrimSpace(userIDStr) == "" {
		return 0, uuid.Nil, map[string]any{"error": "user_id is required"}
	}
	uid, err := strconv.ParseInt(userIDStr, 10, 32)
	if err != nil {
		return 0, uuid.Nil, map[string]any{"error": "invalid user_id"}
	}
	cid, err := uuid.Parse(chatIDStr)
	if err != nil {
		return 0, uuid.Nil, map[string]any{"error": "invalid chat_id"}
	}
	return int32(uid), cid, nil
}

func matchesAnyTerm(haystack string, terms []string) bool {
	for _, t := range terms {
		if strings.Contains(haystack, t) {
			return true
		}
	}
	return false
}

func latestProcessed(list []domain.UploadedFile) *domain.UploadedFile {
	for i := len(list) - 1; i >= 0; i-- {
		if list[i].AgentFileID != nil && *list[i].AgentFileID != "" {
			f := list[i]
			return &f
		}
	}
	return nil
}

func fileSummaries(list []domain.UploadedFile) []map[string]any {
	out := make([]map[string]any, 0, len(list))
	for _, f := range list {
		out = append(out, fileSummary(f))
	}
	return out
}

func fileSummary(f domain.UploadedFile) map[string]any {
	m := map[string]any{
		"file_id":           f.ID.String(),
		"original_filename": f.OriginalFilename,
		"file_type":         f.FileType,
		"processing_status": f.ProcessingStatus,
	}
	if f.Summary != nil {
		m["summary"] = *f.Summary
	}
	return m
}
