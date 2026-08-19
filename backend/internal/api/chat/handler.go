package chat

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/smartkrishi/backend/internal/api"
	"github.com/smartkrishi/backend/internal/domain"
	appmiddleware "github.com/smartkrishi/backend/internal/middleware"
	authservice "github.com/smartkrishi/backend/internal/service/auth"
	chatservice "github.com/smartkrishi/backend/internal/service/chat"
	fileservice "github.com/smartkrishi/backend/internal/service/file"
)

type Handler struct {
	chats *chatservice.Service
	auth  *authservice.Service
	agent AgentRunner          // nil when the agent pipeline is not configured
	files *fileservice.Service // nil when the file subsystem is not configured
}

func NewHandler(chats *chatservice.Service, auth *authservice.Service) *Handler {
	return &Handler{chats: chats, auth: auth}
}

// WithAgent enables the streaming chat endpoint by attaching the agent runner.
// When unset, POST /chat/send-stream returns a service-unavailable error.
func (h *Handler) WithAgent(runner AgentRunner) *Handler {
	h.agent = runner
	return h
}

// WithFiles enables the file endpoints (upload-file, upload-and-analyze-stream)
// by attaching the file service. When unset, those endpoints return a
// service-unavailable error.
func (h *Handler) WithFiles(files *fileservice.Service) *Handler {
	h.files = files
	return h
}

// Routes mounts chat endpoints. All routes require a valid Bearer token; the
// AuthWithUser middleware resolves the user once and stores it in context.
func (h *Handler) Routes(r chi.Router) {
	r.Use(appmiddleware.AuthWithUser(h.auth))
	r.Get("/chats", h.listChats)
	r.Post("/chats", h.createChat)
	r.Get("/chats/{id}", h.getChat)
	r.Put("/chats/{id}", h.updateChat)
	r.Delete("/chats/{id}", h.deleteChat)
	r.Post("/send-stream", h.sendStreamRoute)
	r.Post("/upload-file", h.uploadFileRoute)
	r.Post("/upload-and-analyze-stream", h.uploadAndAnalyzeRoute)
	r.Get("/chats/{id}/files", h.listChatFilesRoute)

	// Legacy non-streaming endpoints (Step 10). suggestions is static (always on).
	r.Get("/suggestions", h.suggestions)
	r.Post("/ask", h.askRoute)
	r.Post("/send", h.sendRoute)
	r.Post("/analyze-image", h.analyzeImageRoute)
	r.Post("/analyze-image-persistent", h.analyzeImagePersistentRoute)
}

// sendStreamRoute guards the streaming endpoint: it returns a clear error when
// the agent pipeline isn't configured (e.g. no GEMINI_API_KEY), otherwise
// delegates to sendStream.
func (h *Handler) sendStreamRoute(w http.ResponseWriter, r *http.Request) {
	if h.agent == nil {
		writeSSEError(w, "AI agent is not configured on this server")
		return
	}
	h.sendStream(w, r)
}

// userID reads the user resolved by AuthWithUser middleware from context.
func (h *Handler) userID(w http.ResponseWriter, r *http.Request) (int32, bool) {
	user, ok := appmiddleware.UserFromContext(r.Context())
	if !ok {
		api.WriteError(w, http.StatusUnauthorized, "Could not validate credentials")
		return 0, false
	}
	return user.ID, true
}

func (h *Handler) listChats(w http.ResponseWriter, r *http.Request) {
	userID, ok := h.userID(w, r)
	if !ok {
		return
	}

	skip := parseIntQuery(r, "skip", 0)
	limit := parseIntQuery(r, "limit", 50)

	summaries, err := h.chats.ListChats(r.Context(), userID, skip, limit)
	if err != nil {
		api.WriteError(w, http.StatusInternalServerError, "Failed to get chats")
		return
	}
	api.WriteJSON(w, http.StatusOK, summaries)
}

func (h *Handler) createChat(w http.ResponseWriter, r *http.Request) {
	userID, ok := h.userID(w, r)
	if !ok {
		return
	}

	var req domain.CreateChatRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		api.WriteError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	chat, err := h.chats.CreateChat(r.Context(), userID, req)
	if err != nil {
		api.WriteError(w, http.StatusInternalServerError, "Failed to create chat")
		return
	}
	api.WriteJSON(w, http.StatusOK, chat)
}

func (h *Handler) getChat(w http.ResponseWriter, r *http.Request) {
	userID, ok := h.userID(w, r)
	if !ok {
		return
	}
	id, ok := parseChatID(w, r)
	if !ok {
		return
	}

	chat, err := h.chats.GetChat(r.Context(), id, userID)
	if errors.Is(err, chatservice.ErrNotFound) {
		api.WriteError(w, http.StatusNotFound, "Chat not found")
		return
	}
	if err != nil {
		api.WriteError(w, http.StatusInternalServerError, "Failed to get chat")
		return
	}
	api.WriteJSON(w, http.StatusOK, chat)
}

func (h *Handler) updateChat(w http.ResponseWriter, r *http.Request) {
	userID, ok := h.userID(w, r)
	if !ok {
		return
	}
	id, ok := parseChatID(w, r)
	if !ok {
		return
	}

	var req domain.UpdateChatRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		api.WriteError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	chat, err := h.chats.UpdateChat(r.Context(), id, userID, req.Title)
	if errors.Is(err, chatservice.ErrEmptyTitle) {
		api.WriteError(w, http.StatusBadRequest, "Title cannot be empty")
		return
	}
	if errors.Is(err, chatservice.ErrNotFound) {
		api.WriteError(w, http.StatusNotFound, "Chat not found")
		return
	}
	if err != nil {
		api.WriteError(w, http.StatusInternalServerError, "Failed to update chat")
		return
	}
	api.WriteJSON(w, http.StatusOK, chat)
}

func (h *Handler) deleteChat(w http.ResponseWriter, r *http.Request) {
	userID, ok := h.userID(w, r)
	if !ok {
		return
	}
	id, ok := parseChatID(w, r)
	if !ok {
		return
	}

	err := h.chats.DeleteChat(r.Context(), id, userID)
	if errors.Is(err, chatservice.ErrNotFound) {
		api.WriteError(w, http.StatusNotFound, "Chat not found")
		return
	}
	if err != nil {
		api.WriteError(w, http.StatusInternalServerError, "Failed to delete chat")
		return
	}
	api.WriteJSON(w, http.StatusOK, map[string]string{"message": "Chat deleted successfully"})
}

func parseChatID(w http.ResponseWriter, r *http.Request) (uuid.UUID, bool) {
	raw := chi.URLParam(r, "id")
	id, err := uuid.Parse(raw)
	if err != nil {
		api.WriteError(w, http.StatusBadRequest, "Invalid chat ID format")
		return uuid.Nil, false
	}
	return id, true
}

func parseIntQuery(r *http.Request, key string, def int32) int32 {
	raw := r.URL.Query().Get(key)
	if raw == "" {
		return def
	}
	v, err := strconv.Atoi(raw)
	if err != nil {
		return def
	}
	return int32(v)
}
