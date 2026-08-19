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

// MobileInitRequest is the body for POST /auth/mobile-init.
type MobileInitRequest struct {
	PhoneNumber string `json:"phone_number"`
	Username    string `json:"username"`
}

// MobileInitResponse is the response body for POST /auth/mobile-init.
type MobileInitResponse struct {
	Message     string `json:"message"`
	IsNewUser   bool   `json:"is_new_user"`
	PhoneNumber string `json:"phone_number"`
	Status      string `json:"status"`
}

// MobileVerifyRequest is the body for POST /auth/mobile-verify (existing users).
// The frontend sends the Firebase ID token in the `otp` field.
type MobileVerifyRequest struct {
	PhoneNumber string `json:"phone_number"`
	OTP         string `json:"otp"`
}

// MobileSignupRequest is the body for POST /auth/mobile-signup (new users).
// The frontend sends the Firebase ID token in `firebase_token`; `otp` is also
// accepted, so both are honored.
type MobileSignupRequest struct {
	PhoneNumber   string `json:"phone_number"`
	Username      string `json:"username"`
	FirebaseToken string `json:"firebase_token"`
	OTP           string `json:"otp"`
}

// Token returns the ID token from whichever field the client populated.
func (r MobileSignupRequest) Token() string {
	if r.FirebaseToken != "" {
		return r.FirebaseToken
	}
	return r.OTP
}
