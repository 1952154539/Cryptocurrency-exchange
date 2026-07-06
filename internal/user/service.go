package user

import (
	"context"
	"fmt"
	"time"

	"github.com/exchange/internal/common"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// StoredUser represents a user in the database.
type StoredUser struct {
	ID           string
	Email        string
	PasswordHash string
	KYCLevel     int
	TwoFAEnabled bool
	Status       string
	CreatedAt    time.Time
}

// Service handles user management.
type Service struct {
	pool *pgxpool.Pool
	auth *AuthService
}

// NewService creates a user service.
func NewService(pool *pgxpool.Pool, auth *AuthService) *Service {
	return &Service{pool: pool, auth: auth}
}

// RegisterUser creates a new user account.
func (s *Service) RegisterUser(ctx context.Context, email, password string) (*StoredUser, error) {
	// Check if email already exists
	var exists bool
	err := s.pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM users WHERE email = $1)`, email).Scan(&exists)
	if err != nil {
		return nil, fmt.Errorf("check email: %w", err)
	}
	if exists {
		return nil, fmt.Errorf("email already registered")
	}

	// Hash password
	hash, err := s.auth.HashPassword(password)
	if err != nil {
		return nil, err
	}

	// Create user
	userID := uuid.New().String()
	query := `INSERT INTO users (id, email, password_hash) VALUES ($1, $2, $3)`
	if _, err := s.pool.Exec(ctx, query, userID, email, hash); err != nil {
		return nil, fmt.Errorf("insert user: %w", err)
	}

	// Create default USDT balance for trading
	if _, err := s.pool.Exec(ctx,
		`INSERT INTO accounts (user_id, currency, balance) VALUES ($1, 'USDT', 1000000), ($1, 'ETH', 100)`,
		userID,
	); err != nil {
		return nil, fmt.Errorf("create accounts: %w", err)
	}

	return &StoredUser{
		ID:        userID,
		Email:     email,
		KYCLevel:  0,
		Status:    string(common.UserStatusActive),
		CreatedAt: time.Now(),
	}, nil
}

// Login authenticates a user and returns a JWT token.
func (s *Service) Login(ctx context.Context, email, password string) (string, error) {
	var id, hash, status string
	err := s.pool.QueryRow(ctx,
		`SELECT id, password_hash, status FROM users WHERE email = $1`, email,
	).Scan(&id, &hash, &status)
	if err != nil {
		return "", fmt.Errorf("user not found: %w", err)
	}

	if status != string(common.UserStatusActive) {
		return "", common.ErrUserSuspended
	}

	if !s.auth.VerifyPassword(hash, password) {
		return "", fmt.Errorf("invalid password")
	}

	return s.auth.GenerateJWT(id)
}

// GetUser retrieves a user by ID.
func (s *Service) GetUser(ctx context.Context, userID string) (*StoredUser, error) {
	u := &StoredUser{}
	err := s.pool.QueryRow(ctx,
		`SELECT id, email, password_hash, kyc_level, two_fa_enabled, status, created_at FROM users WHERE id = $1`,
		userID,
	).Scan(&u.ID, &u.Email, &u.PasswordHash, &u.KYCLevel, &u.TwoFAEnabled, &u.Status, &u.CreatedAt)
	if err != nil {
		return nil, common.ErrUserNotFound
	}
	return u, nil
}

// UpdateStatus changes a user's account status.
func (s *Service) UpdateStatus(ctx context.Context, userID string, status common.UserStatus) error {
	_, err := s.pool.Exec(ctx, `UPDATE users SET status = $1, updated_at = NOW() WHERE id = $2`, string(status), userID)
	return err
}
