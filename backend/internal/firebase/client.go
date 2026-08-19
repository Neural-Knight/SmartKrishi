// Package firebase wraps the Firebase Admin SDK for verifying phone-auth ID
// tokens. It mirrors the Python firebase_service.py: credentials may be supplied
// as inline JSON or a file path, initialization is lazy, and token verification
// retries once with clock-skew tolerance on "used too early"/timestamp errors.
package firebase

import (
	"context"
	"errors"
	"strings"
	"sync"

	firebase "firebase.google.com/go/v4"
	"firebase.google.com/go/v4/auth"
	"google.golang.org/api/option"
)

// Token is the subset of Firebase token data the app needs.
type Token struct {
	UID         string
	PhoneNumber string
}

// Verifier verifies a Firebase ID token. Defined as an interface so the auth
// service can be unit-tested with a stub.
type Verifier interface {
	VerifyIDToken(ctx context.Context, idToken string) (*Token, error)
}

var (
	// ErrNotConfigured means no credentials were provided.
	ErrNotConfigured = errors.New("firebase not configured")
	// ErrInvalidToken means the ID token could not be verified.
	ErrInvalidToken = errors.New("invalid firebase token")
)

// Client is a lazily-initialized Firebase Admin auth client.
type Client struct {
	credentials string // inline JSON or file path
	projectID   string

	once   sync.Once
	auth   *auth.Client
	initErr error
}

// NewClient builds a client from config values. Initialization is deferred to
// the first VerifyIDToken call (matching the Python lazy init).
func NewClient(credentials, projectID string) *Client {
	return &Client{credentials: strings.TrimSpace(credentials), projectID: projectID}
}

// Configured reports whether credentials were supplied.
func (c *Client) Configured() bool {
	return c != nil && c.credentials != ""
}

func (c *Client) init() {
	c.once.Do(func() {
		if c.credentials == "" {
			c.initErr = ErrNotConfigured
			return
		}

		var opt option.ClientOption
		if strings.HasPrefix(c.credentials, "{") {
			// Inline service-account JSON (prod/Render style).
			opt = option.WithCredentialsJSON([]byte(c.credentials))
		} else {
			// Absolute path to a service-account key file (local dev).
			opt = option.WithCredentialsFile(c.credentials)
		}

		cfg := &firebase.Config{}
		if c.projectID != "" {
			cfg.ProjectID = c.projectID
		}

		app, err := firebase.NewApp(context.Background(), cfg, opt)
		if err != nil {
			c.initErr = err
			return
		}
		authClient, err := app.Auth(context.Background())
		if err != nil {
			c.initErr = err
			return
		}
		c.auth = authClient
	})
}

// VerifyIDToken verifies a Firebase ID token, returning the UID and phone number.
// It retries once with clock-skew tolerance on timestamp-related errors, matching
// the Python service's retry behavior.
func (c *Client) VerifyIDToken(ctx context.Context, idToken string) (*Token, error) {
	c.init()
	if c.initErr != nil {
		return nil, c.initErr
	}
	if idToken == "" {
		return nil, ErrInvalidToken
	}

	tok, err := c.auth.VerifyIDTokenAndCheckRevoked(ctx, idToken)
	if err != nil {
		if isClockSkewError(err) {
			// Retry tolerating small clock skew, as the Python service does.
			tok, err = c.auth.VerifyIDToken(ctx, idToken)
		}
		if err != nil {
			return nil, ErrInvalidToken
		}
	}

	phone, _ := tok.Claims["phone_number"].(string)
	return &Token{UID: tok.UID, PhoneNumber: phone}, nil
}

func isClockSkewError(err error) bool {
	msg := strings.ToLower(err.Error())
	for _, phrase := range []string{"used too early", "clock", "issued at", "timestamp", "future"} {
		if strings.Contains(msg, phrase) {
			return true
		}
	}
	return false
}
