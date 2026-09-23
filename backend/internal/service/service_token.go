package service

import (
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

type TokenService interface {
	Sign(identityID uint, identityKey string) (string, error)
	Parse(tokenString string) (*Claims, error)
}

type Claims struct {
	IdentityID  uint   `json:"identityId"`
	IdentityKey string `json:"identityKey"`
	jwt.RegisteredClaims
}

type tokenService struct {
	secret []byte
	expire time.Duration
}

func NewTokenService(secret string, expireMin int) TokenService {
	return &tokenService{secret: []byte(secret), expire: time.Duration(expireMin) * time.Minute}
}

func (s *tokenService) Sign(identityID uint, identityKey string) (string, error) {
	now := time.Now()
	claims := Claims{
		IdentityID:  identityID,
		IdentityKey: identityKey,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    "gbtreehole",
			Subject:   identityKey,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(s.expire)),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString(s.secret)
	if err != nil {
		return "", fmt.Errorf("sign token: %w", err)
	}
	return signed, nil
}

func (s *tokenService) Parse(tokenString string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return s.secret, nil
	})
	if err != nil {
		return nil, fmt.Errorf("parse token: %w", err)
	}
	claims, ok := token.Claims.(*Claims)
	if !ok || !token.Valid {
		return nil, fmt.Errorf("invalid token")
	}
	return claims, nil
}
