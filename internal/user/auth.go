package user

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

const (
	bcryptCost    = 12
	jwtExpiry     = 15 * time.Minute
	refreshExpiry = 7 * 24 * time.Hour
)

// JWTConfig holds JWT signing configuration.
type JWTConfig struct {
	AccessSecret  string
	RefreshSecret string
}

// AuthService handles authentication and authorization.
type AuthService struct {
	jwtCfg JWTConfig
}

// NewAuthService creates an auth service.
func NewAuthService(jwtCfg JWTConfig) *AuthService {
	return &AuthService{jwtCfg: jwtCfg}
}

// HashPassword creates a bcrypt hash of the password.
func (s *AuthService) HashPassword(password string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcryptCost)
	if err != nil {
		return "", fmt.Errorf("hash password: %w", err)
	}
	return string(hash), nil
}

// VerifyPassword checks a password against its hash.
func (s *AuthService) VerifyPassword(hash, password string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	return err == nil
}

// GenerateJWT creates a new JWT access token.
func (s *AuthService) GenerateJWT(userID string) (string, error) {
	claims := jwt.MapClaims{
		"sub": userID,
		"iat": time.Now().Unix(),
		"exp": time.Now().Add(jwtExpiry).Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(s.jwtCfg.AccessSecret))
}

// VerifyJWT validates a JWT token and returns the user ID.
func (s *AuthService) VerifyJWT(tokenString string) (string, error) {
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(s.jwtCfg.AccessSecret), nil
	})
	if err != nil {
		return "", fmt.Errorf("parse jwt: %w", err)
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok || !token.Valid {
		return "", fmt.Errorf("invalid token")
	}

	sub, _ := claims["sub"].(string)
	return sub, nil
}

// VerifyHMACSignature validates an HMAC-SHA256 signature for the API trading endpoint.
// Signature format: HMAC-SHA256(secret, timestamp + method + path + query + body)
func VerifyHMACSignature(secret, timestamp, method, path, query, body, signature string) bool {
	message := timestamp + method + path + query + body
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(message))
	expected := hex.EncodeToString(mac.Sum(nil))
	return hmac.Equal([]byte(expected), []byte(signature))
}

// VerifyTimestamp checks that the request timestamp is within the allowed window.
func VerifyTimestamp(timestamp string, window time.Duration) bool {
	ts, err := time.Parse(time.RFC3339, timestamp)
	if err != nil {
		// Also accept Unix milliseconds
		var ms int64
		if _, err := fmt.Sscanf(timestamp, "%d", &ms); err != nil {
			return false
		}
		ts = time.UnixMilli(ms)
	}
	return time.Since(ts).Abs() <= window
}
