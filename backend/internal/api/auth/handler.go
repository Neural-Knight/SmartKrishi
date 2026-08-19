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

	// Mobile (Firebase phone) auth.
	r.Post("/mobile-init", h.mobileInit)
	r.Post("/mobile-verify", h.mobileVerify)
	r.Post("/mobile-signup", h.mobileSignup)
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

func (h *Handler) mobileInit(w http.ResponseWriter, r *http.Request) {
	var req domain.MobileInitRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		api.WriteError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	resp, err := h.service.MobileInit(r.Context(), req)
	if errors.Is(err, authservice.ErrInvalidPhoneNumber) {
		api.WriteError(w, http.StatusBadRequest, "Phone number must include country code (e.g. +91xxxxxxxxxx)")
		return
	}
	if errors.Is(err, authservice.ErrUsernameRequired) {
		api.WriteError(w, http.StatusBadRequest, "Username is required for new users and must be at least 2 characters")
		return
	}
	if err != nil {
		api.WriteError(w, http.StatusInternalServerError, "Failed to initialize mobile auth")
		return
	}
	api.WriteJSON(w, http.StatusOK, resp)
}

func (h *Handler) mobileVerify(w http.ResponseWriter, r *http.Request) {
	var req domain.MobileVerifyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		api.WriteError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	token, err := h.service.MobileVerify(r.Context(), req)
	if err != nil {
		writeMobileAuthError(w, err)
		return
	}
	api.WriteJSON(w, http.StatusOK, token)
}

func (h *Handler) mobileSignup(w http.ResponseWriter, r *http.Request) {
	var req domain.MobileSignupRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		api.WriteError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	token, err := h.service.MobileSignup(r.Context(), req)
	if err != nil {
		writeMobileAuthError(w, err)
		return
	}
	api.WriteJSON(w, http.StatusOK, token)
}

// writeMobileAuthError maps mobile-auth service errors to HTTP status codes and
// user-facing detail messages.
func writeMobileAuthError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, authservice.ErrFirebaseNotConfigured):
		api.WriteError(w, http.StatusServiceUnavailable, "Mobile authentication is not available")
	case errors.Is(err, authservice.ErrInvalidFirebaseToken):
		api.WriteError(w, http.StatusBadRequest, "Invalid Firebase token. Please try again with a fresh OTP.")
	case errors.Is(err, authservice.ErrPhoneNotInToken):
		api.WriteError(w, http.StatusBadRequest, "Phone number not found in Firebase token")
	case errors.Is(err, authservice.ErrPhoneMismatch):
		api.WriteError(w, http.StatusBadRequest, "Phone number mismatch")
	case errors.Is(err, authservice.ErrUserNotFound):
		api.WriteError(w, http.StatusNotFound, "User not found. Please complete signup first.")
	case errors.Is(err, authservice.ErrUserAlreadyExists):
		api.WriteError(w, http.StatusBadRequest, "User already exists with this phone number")
	case errors.Is(err, authservice.ErrInactiveUser):
		api.WriteError(w, http.StatusBadRequest, "User account is disabled")
	case errors.Is(err, authservice.ErrInvalidPhoneNumber):
		api.WriteError(w, http.StatusBadRequest, "Phone number, username, and Firebase token are required")
	default:
		api.WriteError(w, http.StatusInternalServerError, "Authentication failed")
	}
}
