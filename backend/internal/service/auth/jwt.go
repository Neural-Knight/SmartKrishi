package auth

import (
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

var ErrInvalidToken = errors.New("invalid token")

type TokenClaims struct {
	Subject      string
	AuthProvider string
}

type TokenManager struct {
	secret     []byte
	algorithm  string
	expireMins int
}

func NewTokenManager(secret, algorithm string, expireMinutes int) (*TokenManager, error) {
	if secret == "" {
		return nil, fmt.Errorf("SECRET_KEY is required")
	}
	if algorithm == "" {
		algorithm = "HS256"
	}
	if expireMinutes <= 0 {
		expireMinutes = 60
	}
	return &TokenManager{
		secret:     []byte(secret),
		algorithm:  algorithm,
		expireMins: expireMinutes,
	}, nil
}

func (m *TokenManager) CreateAccessToken(subject, authProvider string) (string, error) {
	now := time.Now().UTC()
	claims := jwt.MapClaims{
		"sub":            subject,
		"auth_provider":  authProvider,
		"exp":            now.Add(time.Duration(m.expireMins) * time.Minute).Unix(),
		"iat":            now.Unix(),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(m.secret)
}

func (m *TokenManager) VerifyToken(tokenString string) (*TokenClaims, error) {
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (any, error) {
		if token.Method.Alg() != jwt.SigningMethodHS256.Alg() {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return m.secret, nil
	})
	if err != nil || !token.Valid {
		return nil, ErrInvalidToken
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return nil, ErrInvalidToken
	}

	subject, ok := claims["sub"].(string)
	if !ok || subject == "" {
		return nil, ErrInvalidToken
	}

	authProvider, _ := claims["auth_provider"].(string)
	return &TokenClaims{
		Subject:      subject,
		AuthProvider: authProvider,
	}, nil
}
