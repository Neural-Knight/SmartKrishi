package auth

import (
	"context"
	"errors"
	"testing"

	"github.com/smartkrishi/backend/internal/domain"
	"github.com/smartkrishi/backend/internal/firebase"
)

func TestValidatePhoneNumber(t *testing.T) {
	cases := []struct {
		in      string
		wantErr bool
	}{
		{"+919876543210", false},
		{"+14155552671", false},
		{"9876543210", true},   // no +
		{"+123", true},         // too short
		{"+1234567890123456", true}, // too long
		{"", true},
	}
	for _, c := range cases {
		err := validatePhoneNumber(c.in)
		if (err != nil) != c.wantErr {
			t.Errorf("validatePhoneNumber(%q) err=%v, wantErr=%v", c.in, err, c.wantErr)
		}
	}
}

func TestMobileSignupRequestToken(t *testing.T) {
	if got := (domain.MobileSignupRequest{FirebaseToken: "a", OTP: "b"}).Token(); got != "a" {
		t.Errorf("prefer firebase_token, got %q", got)
	}
	if got := (domain.MobileSignupRequest{OTP: "b"}).Token(); got != "b" {
		t.Errorf("fallback to otp, got %q", got)
	}
	if got := (domain.MobileSignupRequest{}).Token(); got != "" {
		t.Errorf("empty when neither, got %q", got)
	}
}

// stubVerifier lets us exercise verify paths without real Firebase.
type stubVerifier struct {
	tok *firebase.Token
	err error
}

func (s stubVerifier) VerifyIDToken(_ context.Context, _ string) (*firebase.Token, error) {
	return s.tok, s.err
}

func TestVerifyFirebasePhone(t *testing.T) {
	ctx := context.Background()

	// Not configured.
	s := &Service{}
	if err := s.verifyFirebasePhone(ctx, "tok", "+911111111111"); !errors.Is(err, ErrFirebaseNotConfigured) {
		t.Errorf("want ErrFirebaseNotConfigured, got %v", err)
	}

	// Invalid token.
	s = (&Service{}).WithFirebase(stubVerifier{err: errors.New("bad")})
	if err := s.verifyFirebasePhone(ctx, "tok", "+911111111111"); !errors.Is(err, ErrInvalidFirebaseToken) {
		t.Errorf("want ErrInvalidFirebaseToken, got %v", err)
	}

	// No phone in token.
	s = (&Service{}).WithFirebase(stubVerifier{tok: &firebase.Token{UID: "u"}})
	if err := s.verifyFirebasePhone(ctx, "tok", "+911111111111"); !errors.Is(err, ErrPhoneNotInToken) {
		t.Errorf("want ErrPhoneNotInToken, got %v", err)
	}

	// Phone mismatch.
	s = (&Service{}).WithFirebase(stubVerifier{tok: &firebase.Token{PhoneNumber: "+919999999999"}})
	if err := s.verifyFirebasePhone(ctx, "tok", "+911111111111"); !errors.Is(err, ErrPhoneMismatch) {
		t.Errorf("want ErrPhoneMismatch, got %v", err)
	}

	// Success.
	s = (&Service{}).WithFirebase(stubVerifier{tok: &firebase.Token{PhoneNumber: "+911111111111"}})
	if err := s.verifyFirebasePhone(ctx, "tok", "+911111111111"); err != nil {
		t.Errorf("want nil, got %v", err)
	}
}
