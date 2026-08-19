package auth

import (
	"context"
	"errors"
	"fmt"

	"golang.org/x/crypto/bcrypt"

	"github.com/smartkrishi/backend/internal/domain"
	"github.com/smartkrishi/backend/internal/repository/postgres"
)

var (
	ErrEmailAlreadyRegistered = errors.New("email already registered")
	ErrInvalidCredentials     = errors.New("invalid credentials")
	ErrInactiveUser           = errors.New("inactive user")
)

type Service struct {
	users  *postgres.UserRepository
	tokens *TokenManager
}

func NewService(users *postgres.UserRepository, tokens *TokenManager) *Service {
	return &Service{users: users, tokens: tokens}
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
