package chat

import (
	"errors"
	"io"
	"net/http"

	"github.com/google/uuid"

	"github.com/smartkrishi/backend/internal/agent"
	"github.com/smartkrishi/backend/internal/api"
	"github.com/smartkrishi/backend/internal/domain"
	appmiddleware "github.com/smartkrishi/backend/internal/middleware"
	fileservice "github.com/smartkrishi/backend/internal/service/file"
)

// uploadFileRoute guards POST /chat/upload-file (JSON response). Returns a
// service-unavailable error when the file subsystem isn't configured.
func (h *Handler) uploadFileRoute(w http.ResponseWriter, r *http.Request) {
	if h.files == nil {
		api.WriteError(w, http.StatusServiceUnavailable, "File uploads are not configured on this server")
		return
	}
	h.uploadFile(w, r)
}

// uploadAndAnalyzeRoute guards POST /chat/upload-and-analyze-stream (SSE).
func (h *Handler) uploadAndAnalyzeRoute(w http.ResponseWriter, r *http.Request) {
	if h.files == nil || h.agent == nil {
		writeSSEError(w, "File analysis is not configured on this server")
		return
	}
	h.uploadAndAnalyze(w, r)
}

// listChatFilesRoute handles GET /chat/chats/{id}/files: returns the chat's
// uploaded files as {"files": [...]} (the Dashboard calls this on every chat
// reload). The chat is ownership-verified (same as getChat) before listing.
func (h *Handler) listChatFilesRoute(w http.ResponseWriter, r *http.Request) {
	if h.files == nil {
		// No file subsystem: return an empty list rather than an error so the
		// Dashboard's per-reload call degrades gracefully.
		api.WriteJSON(w, http.StatusOK, map[string]any{"files": []any{}})
		return
	}
	userID, ok := h.userID(w, r)
	if !ok {
		return
	}
	chatID, ok := parseChatID(w, r)
	if !ok {
		return
	}
	// Ownership check: GetChat returns ErrNotFound for another user's chat.
	if _, err := h.chats.GetChat(r.Context(), chatID, userID); err != nil {
		api.WriteError(w, http.StatusNotFound, "Chat not found")
		return
	}

	list, err := h.files.ListByChat(r.Context(), chatID, userID)
	if err != nil {
		api.WriteError(w, http.StatusInternalServerError, "Failed to list files")
		return
	}
	if list == nil {
		list = []domain.UploadedFile{}
	}
	api.WriteJSON(w, http.StatusOK, map[string]any{"files": list})
}

// uploadFile handles POST /chat/upload-file: multipart (file, chat_id,
// message_id?), stores the file (disk + Gemini) and returns FileUploadResponse
// JSON. 10MB limit. Mirrors the Python upload_file endpoint used by the
// Dashboard multi-file path.
func (h *Handler) uploadFile(w http.ResponseWriter, r *http.Request) {
	user, ok := appmiddleware.UserFromContext(r.Context())
	if !ok {
		api.WriteError(w, http.StatusUnauthorized, "Could not validate credentials")
		return
	}

	filename, data, chatID, messageID, ok := h.readMultipartFile(w, r, fileservice.MaxUploadFileBytes)
	if !ok {
		return
	}

	saved, err := h.files.Save(r.Context(), user.ID, chatID, messageID, filename, data)
	if err != nil {
		api.WriteError(w, http.StatusInternalServerError, "Failed to save file")
		return
	}

	api.WriteJSON(w, http.StatusOK, toUploadResponse(saved))
}

// uploadAndAnalyze handles POST /chat/upload-and-analyze-stream: multipart
// (file, message, chat_id?), stores the file, emits a file_uploaded SSE event,
// then runs the normal agent stream on the message (reusing the send-stream
// machinery). 20MB limit.
func (h *Handler) uploadAndAnalyze(w http.ResponseWriter, r *http.Request) {
	user, ok := appmiddleware.UserFromContext(r.Context())
	if !ok {
		writeSSEError(w, "Could not validate credentials")
		return
	}

	// Parse multipart (20MB). message comes as a form field.
	if err := r.ParseMultipartForm(fileservice.MaxAnalyzeBytes); err != nil {
		writeSSEError(w, "Invalid multipart form")
		return
	}
	message := r.FormValue("message")
	chatIDRaw := r.FormValue("chat_id")

	file, header, err := r.FormFile("file")
	if err != nil {
		writeSSEError(w, "file is required")
		return
	}
	defer file.Close()

	data, err := io.ReadAll(io.LimitReader(file, fileservice.MaxAnalyzeBytes+1))
	if err != nil {
		writeSSEError(w, "Failed to read file")
		return
	}
	if err := fileservice.Validate(header.Filename, int64(len(data)), fileservice.MaxAnalyzeBytes); err != nil {
		writeSSEError(w, validationMessage(err))
		return
	}

	var chatID *uuid.UUID
	if chatIDRaw != "" {
		parsed, perr := uuid.Parse(chatIDRaw)
		if perr != nil {
			writeSSEError(w, "Invalid chat ID format")
			return
		}
		chatID = &parsed
	}

	flusher, ok := beginSSE(w)
	if !ok {
		return
	}

	// Resolve/create the chat (auto-title from the message, or the filename when
	// there's no message).
	titleSeed := message
	if titleSeed == "" {
		titleSeed = header.Filename
	}
	chat, err := h.chats.EnsureChatForStream(r.Context(), user.ID, chatID, titleSeed)
	if err != nil {
		writeSSEEvent(w, flusher, agent.ErrorEventValue("Chat not found"))
		return
	}

	// Save the file to disk + Gemini + DB (not linked to a message yet).
	saved, err := h.files.Save(r.Context(), user.ID, chat.ID, nil, header.Filename, data)
	if err != nil {
		writeSSEEvent(w, flusher, agent.ErrorEventValue("Failed to save file"))
		return
	}

	// First event: file_uploaded. file_id is the LOCAL uploaded_files.id (UUID)
	// for frontend parity; agent_file_id lives on the DB row.
	fileEvent := agent.FileUploadedEventValue(saved.ID.String(), saved.OriginalFilename, saved.ProcessingStatus)

	// If there's no message, just emit file_uploaded + end (nothing to analyze).
	if message == "" {
		fileEvent.MessageID = ""
		fileEvent.ChatID = chat.ID.String()
		if writeSSEEvent(w, flusher, fileEvent) {
			end := agent.EndEventValue()
			end.ChatID = chat.ID.String()
			writeSSEEvent(w, flusher, end)
		}
		return
	}

	// Otherwise reuse the streaming core; the file_uploaded event is emitted as
	// a pre-event (stamped with the assistant message id + chat id). Once the
	// user message exists, link the uploaded file to it so the file reloads on
	// that message's files[] array.
	h.runStreamingTurn(r.Context(), w, flusher, user.ID, chat.ID, message,
		agent.RunOptions{}, []agent.Event{fileEvent}, func(userMsgID uuid.UUID) {
			_ = h.files.LinkToMessage(r.Context(), saved.ID, user.ID, userMsgID)
		})
}

// readMultipartFile parses a multipart request for the JSON upload-file path:
// reads the "file" part (bounded by maxBytes), the required "chat_id", and the
// optional "message_id". Writes a JSON error and returns ok=false on failure.
func (h *Handler) readMultipartFile(w http.ResponseWriter, r *http.Request, maxBytes int64) (filename string, data []byte, chatID uuid.UUID, messageID *uuid.UUID, ok bool) {
	if err := r.ParseMultipartForm(maxBytes); err != nil {
		api.WriteError(w, http.StatusBadRequest, "Invalid multipart form")
		return
	}
	chatIDRaw := r.FormValue("chat_id")
	cid, err := uuid.Parse(chatIDRaw)
	if err != nil {
		api.WriteError(w, http.StatusBadRequest, "Invalid or missing chat_id")
		return
	}
	if mid := r.FormValue("message_id"); mid != "" {
		parsed, perr := uuid.Parse(mid)
		if perr != nil {
			api.WriteError(w, http.StatusBadRequest, "Invalid message_id")
			return
		}
		messageID = &parsed
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		api.WriteError(w, http.StatusBadRequest, "file is required")
		return
	}
	defer file.Close()

	b, err := io.ReadAll(io.LimitReader(file, maxBytes+1))
	if err != nil {
		api.WriteError(w, http.StatusInternalServerError, "Failed to read file")
		return
	}
	if verr := fileservice.Validate(header.Filename, int64(len(b)), maxBytes); verr != nil {
		api.WriteError(w, http.StatusBadRequest, validationMessage(verr))
		return
	}

	return header.Filename, b, cid, messageID, true
}

// toUploadResponse maps a saved file to the JSON FileUploadResponse the
// frontend expects. The status message notes when analysis is unavailable.
func toUploadResponse(f *domain.UploadedFile) domain.FileUploadResponse {
	msg := "File uploaded successfully"
	if f.ProcessingStatus == "failed" {
		msg = "File uploaded but AI analysis is unavailable"
	}
	return domain.FileUploadResponse{
		FileID:           f.ID,
		OriginalFilename: f.OriginalFilename,
		FileType:         f.FileType,
		FileSize:         f.FileSize,
		ProcessingStatus: f.ProcessingStatus,
		AgentFileID:      f.AgentFileID,
		Message:          msg,
	}
}

// validationMessage renders a user-facing message for a validation sentinel.
func validationMessage(err error) string {
	switch {
	case errors.Is(err, fileservice.ErrTooLarge):
		return "File too large"
	case errors.Is(err, fileservice.ErrUnsupportedType):
		return "Unsupported file type. Allowed: PNG, JPG, JPEG, WebP, HEIC, HEIF, PDF, DOCX, XLSX, CSV"
	case errors.Is(err, fileservice.ErrEmpty):
		return "File is empty"
	default:
		return "Invalid file"
	}
}
