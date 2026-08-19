package auth

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"golang.org/x/crypto/bcrypt"

	"github.com/smartkrishi/backend/internal/domain"
	"github.com/smartkrishi/backend/internal/firebase"
	"github.com/smartkrishi/backend/internal/repository/postgres"
)

var (
	ErrEmailAlreadyRegistered = errors.New("email already registered")
	ErrInvalidCredentials     = errors.New("invalid credentials")
	ErrInactiveUser           = errors.New("inactive user")

	// Mobile-auth errors.
	ErrInvalidPhoneNumber    = errors.New("invalid phone number")
	ErrUsernameRequired      = errors.New("username required")
	ErrFirebaseNotConfigured = errors.New("firebase not configured")
	ErrInvalidFirebaseToken  = errors.New("invalid firebase token")
	ErrPhoneMismatch         = errors.New("phone number mismatch")
	ErrUserNotFound          = errors.New("user not found")
	ErrUserAlreadyExists     = errors.New("user already exists")
	ErrPhoneNotInToken       = errors.New("phone number not found in token")
)

type Service struct {
	users    *postgres.UserRepository
	tokens   *TokenManager
	firebase firebase.Verifier
}

func NewService(users *postgres.UserRepository, tokens *TokenManager) *Service {
	return &Service{users: users, tokens: tokens}
}

// WithFirebase attaches a Firebase verifier, enabling mobile auth. Returns the
// same service for chaining.
func (s *Service) WithFirebase(v firebase.Verifier) *Service {
	s.firebase = v
	return s
}

func (s *Service) Signup(ctx context.Context, req domain.SignupRequest) (*domain.TokenResponse, error) {
	if req.Name == "" || req.Email == "" || req.Password == "" {
		return nil, fmt.Errorf("name, email, and password are required")
	}

	if _, err := s.users.GetByEmail(ctx, req.Email); err == nil {
		return nil, ErrEmailAlreadyRegistered
	} else if !errors.Is(err, postgres.ErrUserNotFound) {
		return nil, err
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		return nil, fmt.Errorf("hash password: %w", err)
	}

	hashStr := string(hash)
	email := req.Email
	user, err := s.users.Create(ctx, postgres.CreateUserParams{
		Name:           req.Name,
		Email:          &email,
		HashedPassword: &hashStr,
		AuthProvider:   domain.AuthProviderEmail,
	})
	if err != nil {
		return nil, err
	}

	token, err := s.tokens.CreateAccessToken(*user.Email, string(domain.AuthProviderEmail))
	if err != nil {
		return nil, err
	}

	return &domain.TokenResponse{AccessToken: token, TokenType: "bearer"}, nil
}

func (s *Service) Login(ctx context.Context, req domain.LoginRequest) (*domain.TokenResponse, error) {
	token, err := s.authenticateEmailPassword(ctx, req.Email, req.Password)
	if err != nil {
		return nil, err
	}
	return &domain.TokenResponse{AccessToken: token, TokenType: "bearer"}, nil
}

func (s *Service) LoginWithForm(ctx context.Context, username, password string) (*domain.TokenResponse, error) {
	token, err := s.authenticateEmailPassword(ctx, username, password)
	if err != nil {
		return nil, err
	}
	return &domain.TokenResponse{AccessToken: token, TokenType: "bearer"}, nil
}

func (s *Service) authenticateEmailPassword(ctx context.Context, email, password string) (string, error) {
	id, hashedPassword, isActive, storedEmail, err := s.users.GetHashedPasswordByEmail(ctx, email)
	if errors.Is(err, postgres.ErrUserNotFound) {
		return "", ErrInvalidCredentials
	}
	if err != nil {
		return "", err
	}
	if hashedPassword == "" {
		return "", ErrInvalidCredentials
	}
	if err := bcrypt.CompareHashAndPassword([]byte(hashedPassword), []byte(password)); err != nil {
		return "", ErrInvalidCredentials
	}
	if !isActive {
		return "", ErrInactiveUser
	}

	subject := email
	if storedEmail != nil {
		subject = *storedEmail
	}
	_ = id
	return s.tokens.CreateAccessToken(subject, string(domain.AuthProviderEmail))
}

func (s *Service) GetCurrentUser(ctx context.Context, claims *TokenClaims) (*domain.User, error) {
	if claims == nil {
		return nil, ErrInvalidCredentials
	}

	if claims.AuthProvider == string(domain.AuthProviderMobile) {
		return s.users.GetByPhone(ctx, claims.Subject)
	}

	// Legacy tokens without auth_provider default to email lookup.
	if claims.AuthProvider == "" || claims.AuthProvider == string(domain.AuthProviderEmail) {
		return s.users.GetByEmail(ctx, claims.Subject)
	}

	return s.users.GetByEmail(ctx, claims.Subject)
}

func (s *Service) VerifyToken(token string) (*TokenClaims, error) {
	return s.tokens.VerifyToken(token)
}

// ---- Mobile (Firebase phone) auth ----

// FirebaseEnabled reports whether a verifier is attached.
func (s *Service) FirebaseEnabled() bool {
	return s.firebase != nil
}

// validatePhoneNumber requires the number to start with '+' and be 10–15 chars
// long.
func validatePhoneNumber(phone string) error {
	if !strings.HasPrefix(phone, "+") {
		return ErrInvalidPhoneNumber
	}
	if len(phone) < 10 || len(phone) > 15 {
		return ErrInvalidPhoneNumber
	}
	return nil
}

// MobileInit validates the phone number and reports whether the user is new.
// For new users, a username of at least 2 non-space characters is required.
func (s *Service) MobileInit(ctx context.Context, req domain.MobileInitRequest) (*domain.MobileInitResponse, error) {
	if err := validatePhoneNumber(req.PhoneNumber); err != nil {
		return nil, err
	}

	_, err := s.users.GetByPhone(ctx, req.PhoneNumber)
	existing := err == nil
	if err != nil && !errors.Is(err, postgres.ErrUserNotFound) {
		return nil, err
	}

	if !existing {
		if len(strings.TrimSpace(req.Username)) < 2 {
			return nil, ErrUsernameRequired
		}
	}

	return &domain.MobileInitResponse{
		Message:     "Ready to send OTP",
		IsNewUser:   !existing,
		PhoneNumber: req.PhoneNumber,
		Status:      "ready",
	}, nil
}

// verifyFirebasePhone verifies the ID token and confirms it belongs to the
// expected phone number.
func (s *Service) verifyFirebasePhone(ctx context.Context, idToken, expectedPhone string) error {
	if s.firebase == nil {
		return ErrFirebaseNotConfigured
	}
	tok, err := s.firebase.VerifyIDToken(ctx, idToken)
	if err != nil {
		return ErrInvalidFirebaseToken
	}
	if tok.PhoneNumber == "" {
		return ErrPhoneNotInToken
	}
	if tok.PhoneNumber != expectedPhone {
		return ErrPhoneMismatch
	}
	return nil
}

// MobileVerify authenticates an existing mobile user after Firebase OTP.
func (s *Service) MobileVerify(ctx context.Context, req domain.MobileVerifyRequest) (*domain.TokenResponse, error) {
	if err := s.verifyFirebasePhone(ctx, req.OTP, req.PhoneNumber); err != nil {
		return nil, err
	}

	user, err := s.users.GetByPhone(ctx, req.PhoneNumber)
	if errors.Is(err, postgres.ErrUserNotFound) {
		return nil, ErrUserNotFound
	}
	if err != nil {
		return nil, err
	}
	if !user.IsActive {
		return nil, ErrInactiveUser
	}

	token, err := s.tokens.CreateAccessToken(*user.PhoneNumber, string(domain.AuthProviderMobile))
	if err != nil {
		return nil, err
	}
	return &domain.TokenResponse{AccessToken: token, TokenType: "bearer"}, nil
}

// MobileSignup creates a new mobile user after Firebase OTP verification.
func (s *Service) MobileSignup(ctx context.Context, req domain.MobileSignupRequest) (*domain.TokenResponse, error) {
	if req.PhoneNumber == "" || strings.TrimSpace(req.Username) == "" || req.Token() == "" {
		return nil, ErrInvalidPhoneNumber
	}
	if err := s.verifyFirebasePhone(ctx, req.Token(), req.PhoneNumber); err != nil {
		return nil, err
	}

	if _, err := s.users.GetByPhone(ctx, req.PhoneNumber); err == nil {
		return nil, ErrUserAlreadyExists
	} else if !errors.Is(err, postgres.ErrUserNotFound) {
		return nil, err
	}

	phone := req.PhoneNumber
	user, err := s.users.Create(ctx, postgres.CreateUserParams{
		Name:         req.Username,
		PhoneNumber:  &phone,
		AuthProvider: domain.AuthProviderMobile,
	})
	if err != nil {
		return nil, err
	}

	token, err := s.tokens.CreateAccessToken(*user.PhoneNumber, string(domain.AuthProviderMobile))
	if err != nil {
		return nil, err
	}
	return &domain.TokenResponse{AccessToken: token, TokenType: "bearer"}, nil
}
