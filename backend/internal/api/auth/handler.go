package auth

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/smartkrishi/backend/internal/api"
	"github.com/smartkrishi/backend/internal/domain"
	appmiddleware "github.com/smartkrishi/backend/internal/middleware"
	authservice "github.com/smartkrishi/backend/internal/service/auth"
)

type Handler struct {
	service *authservice.Service
}

func NewHandler(service *authservice.Service) *Handler {
	return &Handler{service: service}
}

func (h *Handler) Routes(r chi.Router) {
	r.Post("/signup", h.signup)
	r.Post("/login", h.login)
	r.Post("/token", h.token)
	r.With(appmiddleware.Auth(h.service)).Get("/me", h.me)
}

func (h *Handler) signup(w http.ResponseWriter, r *http.Request) {
	var req domain.SignupRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		api.WriteError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	token, err := h.service.Signup(r.Context(), req)
	if errors.Is(err, authservice.ErrEmailAlreadyRegistered) {
		api.WriteError(w, http.StatusBadRequest, "Email already registered")
		return
	}
	if err != nil {
		api.WriteError(w, http.StatusInternalServerError, "Failed to create account")
		return
	}

	api.WriteJSON(w, http.StatusOK, token)
}

func (h *Handler) login(w http.ResponseWriter, r *http.Request) {
	var req domain.LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		api.WriteError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	token, err := h.service.Login(r.Context(), req)
	if errors.Is(err, authservice.ErrInvalidCredentials) {
		w.Header().Set("WWW-Authenticate", "Bearer")
		api.WriteError(w, http.StatusUnauthorized, "Incorrect email or password")
		return
	}
	if errors.Is(err, authservice.ErrInactiveUser) {
		api.WriteError(w, http.StatusBadRequest, "Inactive user")
		return
	}
	if err != nil {
		api.WriteError(w, http.StatusInternalServerError, "Failed to login")
		return
	}

	api.WriteJSON(w, http.StatusOK, token)
}

func (h *Handler) token(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		api.WriteError(w, http.StatusBadRequest, "Invalid form data")
		return
	}

	username := r.FormValue("username")
	password := r.FormValue("password")

	token, err := h.service.LoginWithForm(r.Context(), username, password)
	if errors.Is(err, authservice.ErrInvalidCredentials) {
		w.Header().Set("WWW-Authenticate", "Bearer")
		api.WriteError(w, http.StatusUnauthorized, "Incorrect username or password")
		return
	}
	if err != nil {
		api.WriteError(w, http.StatusInternalServerError, "Failed to login")
		return
	}

	api.WriteJSON(w, http.StatusOK, token)
}

func (h *Handler) me(w http.ResponseWriter, r *http.Request) {
	claims, ok := appmiddleware.ClaimsFromContext(r.Context())
	if !ok {
		api.WriteError(w, http.StatusUnauthorized, "Could not validate credentials")
		return
	}

	user, err := h.service.GetCurrentUser(r.Context(), claims)
	if err != nil {
		api.WriteError(w, http.StatusUnauthorized, "Could not validate credentials")
		return
	}
	if !user.IsActive {
		api.WriteError(w, http.StatusBadRequest, "Inactive user")
		return
	}

	api.WriteJSON(w, http.StatusOK, user)
}
