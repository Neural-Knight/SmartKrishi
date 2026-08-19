package domain

import "time"

type AuthProvider string

const (
	AuthProviderEmail  AuthProvider = "email"
	AuthProviderMobile AuthProvider = "mobile"
)

type User struct {
	ID                    int32        `json:"id"`
	Name                  string       `json:"name"`
	Email                 *string      `json:"email"`
	PhoneNumber           *string      `json:"phone_number"`
	AuthProvider          AuthProvider `json:"auth_provider"`
	IsActive              bool         `json:"is_active"`
	CreatedAt             time.Time    `json:"created_at"`
	UpdatedAt             *time.Time   `json:"updated_at"`
	AutoFallbackEnabled   bool         `json:"auto_fallback_enabled"`
	FallbackMode          string       `json:"fallback_mode"`
	FallbackActive        bool         `json:"fallback_active"`
	FallbackPhone         *string      `json:"fallback_phone"`
	FallbackPhoneVerified bool         `json:"fallback_phone_verified"`
	WhatsappUserID        *string      `json:"whatsapp_user_id"`
}

type TokenResponse struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
}

type SignupRequest struct {
	Name     string `json:"name"`
	Email    string `json:"email"`
	Password string `json:"password"`
}

type LoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}
