package auth

import (
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// Sentinel errors for JWT operations.
var (
	ErrInvalidToken = errors.New("invalid token")
	ErrExpiredToken = errors.New("token has expired")
)

// Claims represents the custom JWT claims used for authentication.
type Claims struct {
	UserID             int    `json:"user_id"`
	SessionID          string `json:"session_id"`
	TokenType          string `json:"token_type"`
	ImpersonatorUserID *int   `json:"impersonator_user_id,omitempty"`
	APIKeyID           int64  `json:"api_key_id,omitempty"`
	RateTier           string `json:"rate_tier,omitempty"`
	jwt.RegisteredClaims
}

const (
	TokenTypeAccess       = "access"
	TokenTypeRefresh      = "refresh"
	TokenTypeAPIKey       = "api_key"
	TokenTypePluginAccess = "plugin_access"
)

const PluginAccessCookieName = "silo_plugin_access"

// JWTService handles JWT token generation and validation using HMAC-SHA256.
type JWTService struct {
	secret        []byte
	accessExpiry  time.Duration
	refreshExpiry time.Duration
}

// NewJWTService creates a new JWTService with the given secret and expiry durations.
func NewJWTService(secret string, accessExpiry, refreshExpiry time.Duration) *JWTService {
	return &JWTService{
		secret:        []byte(secret),
		accessExpiry:  accessExpiry,
		refreshExpiry: refreshExpiry,
	}
}

// AccessExpiry returns the configured access token expiry duration.
func (j *JWTService) AccessExpiry() time.Duration {
	return j.accessExpiry
}

// RefreshExpiry returns the configured refresh token expiry duration.
func (j *JWTService) RefreshExpiry() time.Duration {
	return j.refreshExpiry
}

// GenerateAccessToken creates a signed JWT access token with the configured
// access token expiry duration.
func (j *JWTService) GenerateAccessToken(userID int, sessionID string) (string, error) {
	return j.generateAccessToken(Claims{
		UserID:    userID,
		SessionID: sessionID,
	})
}

// GenerateRefreshToken creates a signed JWT refresh token with the configured
// refresh token expiry duration.
func (j *JWTService) GenerateRefreshToken(userID int, sessionID string) (string, error) {
	return j.generateRefreshToken(Claims{
		UserID:    userID,
		SessionID: sessionID,
	})
}

func (j *JWTService) GeneratePluginAccessToken(userID int, sessionID string, ttl time.Duration) (string, error) {
	if ttl <= 0 {
		ttl = 5 * time.Minute
	}
	return j.generateToken(Claims{
		UserID:    userID,
		SessionID: sessionID,
	}, TokenTypePluginAccess, ttl)
}

func (j *JWTService) generateAccessToken(claims Claims) (string, error) {
	return j.generateToken(claims, TokenTypeAccess, j.accessExpiry)
}

func (j *JWTService) generateRefreshToken(claims Claims) (string, error) {
	return j.generateToken(claims, TokenTypeRefresh, j.refreshExpiry)
}

// generateToken creates a signed JWT with the given claims and expiry duration.
func (j *JWTService) generateToken(claims Claims, tokenType string, expiry time.Duration) (string, error) {
	now := time.Now()
	claims.TokenType = tokenType
	claims.RegisteredClaims = jwt.RegisteredClaims{
		ExpiresAt: jwt.NewNumericDate(now.Add(expiry)),
		IssuedAt:  jwt.NewNumericDate(now),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, &claims)
	signedToken, err := token.SignedString(j.secret)
	if err != nil {
		return "", fmt.Errorf("signing token: %w", err)
	}

	return signedToken, nil
}

// ValidateToken parses and validates a JWT token string. It verifies the
// signature, expiry, and signing method (HMAC-SHA256). Returns the parsed
// claims on success.
func (j *JWTService) ValidateToken(tokenStr string) (*Claims, error) {
	if tokenStr == "" {
		return nil, ErrInvalidToken
	}

	claims := &Claims{}
	token, err := jwt.ParseWithClaims(tokenStr, claims, func(token *jwt.Token) (any, error) {
		// Reject any signing method other than HMAC.
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return j.secret, nil
	})
	if err != nil {
		if errors.Is(err, jwt.ErrTokenExpired) {
			return nil, fmt.Errorf("%w: %w", ErrExpiredToken, err)
		}
		return nil, fmt.Errorf("%w: %w", ErrInvalidToken, err)
	}

	if !token.Valid {
		return nil, ErrInvalidToken
	}

	return claims, nil
}
