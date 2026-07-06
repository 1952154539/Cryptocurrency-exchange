package user

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/hex"
	"encoding/pem"
	"fmt"
	"os"
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
	PrivateKey *rsa.PrivateKey
	PublicKey  *rsa.PublicKey
}

// AuthService handles authentication and authorization.
type AuthService struct {
	jwtCfg        JWTConfig
	useHMAC       bool
	accessSecret  string
	refreshSecret string
}

// NewAuthService creates an auth service with RSA keys.
func NewAuthService(jwtCfg JWTConfig) *AuthService {
	return &AuthService{jwtCfg: jwtCfg}
}

// NewAuthServiceWithSecrets creates an auth service with HMAC secrets (legacy dev mode).
func NewAuthServiceWithSecrets(accessSecret, refreshSecret string) *AuthService {
	svc := &AuthService{jwtCfg: JWTConfig{}}
	svc.accessSecret = accessSecret
	svc.refreshSecret = refreshSecret
	svc.useHMAC = true
	return svc
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

	if s.useHMAC {
		token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
		return token.SignedString([]byte(s.accessSecret))
	}

	if s.jwtCfg.PrivateKey == nil {
		return "", fmt.Errorf("no signing key configured")
	}
	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	return token.SignedString(s.jwtCfg.PrivateKey)
}

// VerifyJWT validates a JWT token and returns the user ID.
func (s *AuthService) VerifyJWT(tokenString string) (string, error) {
	var keyFunc jwt.Keyfunc

	if s.useHMAC {
		keyFunc = func(token *jwt.Token) (interface{}, error) {
			if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
			}
			return []byte(s.accessSecret), nil
		}
	} else {
		keyFunc = func(token *jwt.Token) (interface{}, error) {
			if _, ok := token.Method.(*jwt.SigningMethodRSA); !ok {
				return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
			}
			if s.jwtCfg.PublicKey == nil {
				return nil, fmt.Errorf("no verification key configured")
			}
			return s.jwtCfg.PublicKey, nil
		}
	}

	token, err := jwt.Parse(tokenString, keyFunc)
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

// LoadRSAKeyPair reads PEM-encoded RSA private and public keys from files.
func LoadRSAKeyPair(privateKeyPath, publicKeyPath string) (*rsa.PrivateKey, *rsa.PublicKey, error) {
	privKey, err := loadPrivateKey(privateKeyPath)
	if err != nil {
		return nil, nil, fmt.Errorf("load private key: %w", err)
	}
	pubKey, err := loadPublicKey(publicKeyPath)
	if err != nil {
		return nil, nil, fmt.Errorf("load public key: %w", err)
	}
	return privKey, pubKey, nil
}

// LoadRSAPublicKey reads a PEM-encoded RSA public key from a file.
func LoadRSAPublicKey(publicKeyPath string) (*rsa.PublicKey, error) {
	return loadPublicKey(publicKeyPath)
}

// GenerateRSAKeyPair generates a new 2048-bit RSA key pair. For development use only.
func GenerateRSAKeyPair() (*rsa.PrivateKey, *rsa.PublicKey, error) {
	privKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, nil, err
	}
	return privKey, &privKey.PublicKey, nil
}

func loadPrivateKey(path string) (*rsa.PrivateKey, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read file: %w", err)
	}
	block, _ := pem.Decode(data)
	if block == nil {
		return nil, fmt.Errorf("failed to decode PEM block")
	}
	key, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		// Try PKCS1
		key, err = x509.ParsePKCS1PrivateKey(block.Bytes)
		if err != nil {
			return nil, fmt.Errorf("parse private key: %w", err)
		}
	}
	rsaKey, ok := key.(*rsa.PrivateKey)
	if !ok {
		return nil, fmt.Errorf("not an RSA private key")
	}
	return rsaKey, nil
}

func loadPublicKey(path string) (*rsa.PublicKey, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read file: %w", err)
	}
	block, _ := pem.Decode(data)
	if block == nil {
		return nil, fmt.Errorf("failed to decode PEM block")
	}
	key, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("parse public key: %w", err)
	}
	rsaKey, ok := key.(*rsa.PublicKey)
	if !ok {
		return nil, fmt.Errorf("not an RSA public key")
	}
	return rsaKey, nil
}

// VerifyHMACSignature validates an HMAC-SHA256 signature for the API trading endpoint.
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
		var ms int64
		if _, err := fmt.Sscanf(timestamp, "%d", &ms); err != nil {
			return false
		}
		ts = time.UnixMilli(ms)
	}
	return time.Since(ts).Abs() <= window
}
