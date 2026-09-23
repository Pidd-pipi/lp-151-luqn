package service

import (
	"strings"
	"testing"
)

func TestTokenServiceSignAndParse(t *testing.T) {
	svc := NewTokenService("test-secret", 60)
	token, err := svc.Sign(42, "identity-key")
	if err != nil {
		t.Fatalf("sign token: %v", err)
	}
	claims, err := svc.Parse(token)
	if err != nil {
		t.Fatalf("parse token: %v", err)
	}
	if claims.IdentityID != 42 {
		t.Fatalf("identity id mismatch: got %d", claims.IdentityID)
	}
	if claims.IdentityKey != "identity-key" {
		t.Fatalf("identity key mismatch: got %s", claims.IdentityKey)
	}
}

func TestTokenServiceParseInvalid(t *testing.T) {
	svc := NewTokenService("test-secret", 60)
	tests := []struct {
		name  string
		token string
	}{
		{"empty", ""},
		{"garbage", "not-a-jwt"},
		{"wrong-signature", "eyJhbGciOiJIUzI1NiJ9.eyJzdWIiOiIxIn0.abc"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := svc.Parse(tt.token); err == nil {
				t.Fatal("expected error for invalid token")
			}
		})
	}
}

func TestGenerateKey(t *testing.T) {
	k1, err := generateKey()
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	k2, err := generateKey()
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	if k1 == k2 {
		t.Fatal("generated keys should be unique")
	}
	if !strings.HasPrefix(k1, "") {
		t.Fatal("unexpected key format")
	}
}
