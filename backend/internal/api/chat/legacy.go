package chat

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"

	"github.com/google/uuid"

	"github.com/smartkrishi/backend/internal/api"
	"github.com/smartkrishi/backend/internal/domain"
)

// Legacy non-streaming endpoints (Step 10) — parity with the Python
// backend/app/routers/chat.py. These are fallbacks the frontend still calls;
// the primary Dashboard path is send-stream. Text paths use a simple single
// Gemini call (ai_service.process_text_message), NOT the agent pipeline, and
// no checker — matching Python. Image paths are stateless vision calls.

// legacyImageMIME maps the image content types the legacy endpoints accept
// (Python allowed jpeg/png/jpg/webp). Value is the extension for storage.
var legacyImageExts = map[string]struct{}{
	"png": {}, "jpg": {}, "jpeg": {}, "webp": {},
}

const legacyImageMaxBytes = 10 << 20 // 10MB

// suggestions returns the static farmer suggestion list (Python /suggestions).
func (h *Handler) suggestions(w http.ResponseWriter, _ *http.Request) {
	api.WriteJSON(w, http.StatusOK, map[string]any{"suggestions": chatSuggestions})
}

var chatSuggestions = []map[string]string{
	{"id": "weather-advice", "text": "Weather Forecast Impact", "prompt": "How will the current weather conditions affect my farming activities?"},
	{"id": "crop-diseases", "text": "Disease Identification", "prompt": "Help me identify diseases affecting my crops and suggest treatments."},
	{"id": "soil-health", "text": "Soil Management", "prompt": "How can I improve my soil health for better crop yields?"},
	{"id": "market-prices", "text": "Market Insights", "prompt": "What are the current market trends for my crops?"},
	{"id": "pest-control", "text": "Pest Management", "prompt": "Suggest organic pest control methods for my crops."},
	{"id": "irrigation", "text": "Water Management", "prompt": "What's the best irrigation schedule for my current crops?"},
}

// askRoute / sendRoute / analyze* guards return a clear error when the AI agent
// isn't configured (no GEMINI_API_KEY), matching the send-stream guard.
func (h *Handler) askRoute(w http.ResponseWriter, r *http.Request) {
	if h.agent == nil {
		api.WriteError(w, http.StatusServiceUnavailable, "AI agent is not configured on this server")
		return
	}
	h.ask(w, r)
}

func (h *Handler) sendRoute(w http.ResponseWriter, r *http.Request) {
	if h.agent == nil {
		api.WriteError(w, http.StatusServiceUnavailable, "AI agent is not configured on this server")
		return
	}
	h.send(w, r)
}

func (h *Handler) analyzeImageRoute(w http.ResponseWriter, r *http.Request) {
	if h.agent == nil {
		api.WriteError(w, http.StatusServiceUnavailable, "AI agent is not configured on this server")
		return
	}
	h.analyzeImage(w, r)
}

func (h *Handler) analyzeImagePersistentRoute(w http.ResponseWriter, r *http.Request) {
	if h.agent == nil {
		api.WriteError(w, http.StatusServiceUnavailable, "AI agent is not configured on this server")
		return
	}
	h.analyzeImagePersistent(w, r)
}

// ask handles POST /chat/ask — stateless text Q&A. {message, chat_history?} →
// {response}. Ports the Python /ask wrapper over process_text_message.
func (h *Handler) ask(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.userID(w, r); !ok {
		return
	}
	var req domain.ChatMessageLegacy
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		api.WriteError(w, http.StatusBadRequest, "Invalid request body")
		return
	}
	if strings.TrimSpace(req.Message) == "" {
		api.WriteError(w, http.StatusBadRequest, "Message cannot be empty")
		return
	}

	answer, err := h.agent.AskText(r.Context(), req.Message, req.ChatHistory)
	if err != nil {
		api.WriteError(w, http.StatusInternalServerError, "Failed to process your question")
		return
	}
	api.WriteJSON(w, http.StatusOK, domain.ChatResponseLegacy{Response: answer})
}

// send handles POST /chat/send — non-streaming persistent message. {message,
// chat_id?} → {response, chat_id, message_id}. Gets/creates the chat, saves the
// user message, gets a simple AI response (no pipeline, no checker — matching
// Python), saves + returns the assistant message.
func (h *Handler) send(w http.ResponseWriter, r *http.Request) {
	userID, ok := h.userID(w, r)
	if !ok {
		return
	}
	var req domain.SendMessageRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		api.WriteError(w, http.StatusBadRequest, "Invalid request body")
		return
	}
	if strings.TrimSpace(req.Message) == "" {
		api.WriteError(w, http.StatusBadRequest, "Message cannot be empty")
		return
	}

	chat, err := h.chats.EnsureChatForStream(r.Context(), userID, req.ChatID, req.Message)
	if err != nil {
		api.WriteError(w, http.StatusNotFound, "Chat not found")
		return
	}

	// History for context (before saving the new user message, like Python).
	history := h.legacyHistory(r.Context(), chat.ID)

	if _, err := h.chats.AddMessage(r.Context(), chat.ID, userID, "user", req.Message); err != nil {
		api.WriteError(w, http.StatusInternalServerError, "Failed to save your message")
		return
	}

	answer, err := h.agent.AskText(r.Context(), req.Message, history)
	if err != nil {
		api.WriteError(w, http.StatusInternalServerError, "Failed to process your message")
		return
	}

	assistant, err := h.chats.AddMessage(r.Context(), chat.ID, userID, "assistant", answer)
	if err != nil {
		api.WriteError(w, http.StatusInternalServerError, "Failed to save the response")
		return
	}
	api.WriteJSON(w, http.StatusOK, domain.SendMessageResponse{
		Response:  answer,
		ChatID:    chat.ID,
		MessageID: assistant.ID,
	})
}

// analyzeImage handles POST /chat/analyze-image — stateless image analysis.
// multipart (file, message) → {response}.
func (h *Handler) analyzeImage(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.userID(w, r); !ok {
		return
	}
	message, data, _, ok := h.readLegacyImage(w, r)
	if !ok {
		return
	}
	answer, err := h.agent.AnalyzeImage(r.Context(), message, data, mimeForImage(data))
	if err != nil {
		api.WriteError(w, http.StatusInternalServerError, "Failed to analyze image")
		return
	}
	api.WriteJSON(w, http.StatusOK, domain.ChatResponseLegacy{Response: answer})
}

// analyzeImagePersistent handles POST /chat/analyze-image-persistent —
// multipart (file, message, chat_id?) → {response, chat_id, message_id}. Saves
// the file (disk + Gemini) linked to the user message, then returns a
// stateless image analysis as the assistant message.
func (h *Handler) analyzeImagePersistent(w http.ResponseWriter, r *http.Request) {
	userID, ok := h.userID(w, r)
	if !ok {
		return
	}
	message, data, filename, ok := h.readLegacyImage(w, r)
	if !ok {
		return
	}

	var chatID *uuid.UUID
	if raw := r.FormValue("chat_id"); raw != "" {
		parsed, perr := uuid.Parse(raw)
		if perr != nil {
			api.WriteError(w, http.StatusBadRequest, "Invalid chat ID format")
			return
		}
		chatID = &parsed
	}

	seed := message
	if chatID == nil {
		seed = "Image Analysis: " + filename
	}
	chat, err := h.chats.EnsureChatForStream(r.Context(), userID, chatID, seed)
	if err != nil {
		api.WriteError(w, http.StatusNotFound, "Chat not found")
		return
	}

	// Save the user message (with image marker, like Python).
	userMsg, err := h.chats.AddMessage(r.Context(), chat.ID, userID, "user",
		"📷 "+message+"\n[Uploaded image: "+filename+"]")
	if err != nil {
		api.WriteError(w, http.StatusInternalServerError, "Failed to save your message")
		return
	}

	// Persist the file (best-effort) linked to the user message, when the file
	// subsystem is available.
	if h.files != nil {
		if saved, ferr := h.files.Save(r.Context(), userID, chat.ID, &userMsg.ID, filename, data); ferr == nil {
			_ = saved // stored + linked; response shape doesn't include it
		}
	}

	answer, err := h.agent.AnalyzeImage(r.Context(), message, data, mimeForImage(data))
	if err != nil {
		api.WriteError(w, http.StatusInternalServerError, "Failed to analyze image")
		return
	}
	assistant, err := h.chats.AddMessage(r.Context(), chat.ID, userID, "assistant", answer)
	if err != nil {
		api.WriteError(w, http.StatusInternalServerError, "Failed to save the response")
		return
	}
	api.WriteJSON(w, http.StatusOK, domain.SendMessageResponse{
		Response:  answer,
		ChatID:    chat.ID,
		MessageID: assistant.ID,
	})
}

// readLegacyImage reads and validates the multipart image (10MB, image types
// only). Returns message, bytes, filename, ok.
func (h *Handler) readLegacyImage(w http.ResponseWriter, r *http.Request) (message string, data []byte, filename string, ok bool) {
	if err := r.ParseMultipartForm(legacyImageMaxBytes); err != nil {
		api.WriteError(w, http.StatusBadRequest, "Invalid multipart form")
		return
	}
	message = r.FormValue("message")
	if message == "" {
		message = "Analyze this crop image"
	}
	file, header, err := r.FormFile("file")
	if err != nil {
		api.WriteError(w, http.StatusBadRequest, "file is required")
		return
	}
	defer file.Close()

	b, err := io.ReadAll(io.LimitReader(file, legacyImageMaxBytes+1))
	if err != nil {
		api.WriteError(w, http.StatusInternalServerError, "Failed to read file")
		return
	}
	if int64(len(b)) > legacyImageMaxBytes {
		api.WriteError(w, http.StatusBadRequest, "File too large. Maximum size is 10MB.")
		return
	}
	ext := extLower(header.Filename)
	if _, allowed := legacyImageExts[ext]; !allowed {
		api.WriteError(w, http.StatusBadRequest, "Invalid file type. Please upload an image.")
		return
	}
	return message, b, header.Filename, true
}

// legacyHistory returns {role, content} history maps for a chat (for AskText).
func (h *Handler) legacyHistory(ctx context.Context, chatID uuid.UUID) []map[string]string {
	msgs, err := h.chats.RecentHistory(ctx, chatID, 20)
	if err != nil {
		return nil
	}
	out := make([]map[string]string, 0, len(msgs))
	for _, m := range msgs {
		out = append(out, map[string]string{"role": m.Role, "content": m.Content})
	}
	return out
}

func extLower(filename string) string {
	if i := strings.LastIndex(filename, "."); i >= 0 {
		return strings.ToLower(filename[i+1:])
	}
	return ""
}

// mimeForImage returns an image MIME type for the legacy vision call. It uses
// the file service's shared mapping via the extension when possible; here we
// sniff a minimal set from the bytes' magic, defaulting to jpeg.
func mimeForImage(data []byte) string {
	switch {
	case len(data) >= 8 && string(data[1:4]) == "PNG":
		return "image/png"
	case len(data) >= 12 && string(data[8:12]) == "WEBP":
		return "image/webp"
	default:
		return "image/jpeg"
	}
}
