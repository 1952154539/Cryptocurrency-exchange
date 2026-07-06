package kyc

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog/log"
)

// Status represents a KYC verification status.
type Status string

const (
	StatusNone     Status = "none"
	StatusPending  Status = "pending"
	StatusApproved Status = "approved"
	StatusRejected Status = "rejected"
)

// Verification stores KYC verification data.
type Verification struct {
	ID          string    `json:"id"`
	UserID      string    `json:"user_id"`
	Status      Status    `json:"status"`
	Level       int       `json:"level"`
	FullName    string    `json:"full_name"`
	DocType     string    `json:"doc_type"`
	DocNumber   string    `json:"doc_number"`
	DocImageURL string    `json:"doc_image_url"`
	ReviewNotes string    `json:"review_notes,omitempty"`
	ReviewedBy  string    `json:"reviewed_by,omitempty"`
	SubmittedAt time.Time `json:"submitted_at"`
	ReviewedAt  *time.Time `json:"reviewed_at,omitempty"`
}

// Service handles KYC operations.
type Service struct {
	pool *pgxpool.Pool
}

// NewService creates a KYC service.
func NewService(pool *pgxpool.Pool) *Service {
	return &Service{pool: pool}
}

// Submit initiates or updates a KYC verification.
func (s *Service) Submit(ctx context.Context, v *Verification) error {
	v.Status = StatusPending
	v.SubmittedAt = time.Now()

	_, err := s.pool.Exec(ctx,
		`INSERT INTO kyc_verifications (user_id, status, level, full_name, doc_type, doc_number, doc_image_url, submitted_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		 ON CONFLICT (user_id) DO UPDATE SET
		   status = $2, level = $3, full_name = $4, doc_type = $5, doc_number = $6,
		   doc_image_url = $7, submitted_at = $8, reviewed_at = NULL, review_notes = NULL`,
		v.UserID, string(v.Status), v.Level, v.FullName, v.DocType, v.DocNumber, v.DocImageURL, v.SubmittedAt,
	)
	if err != nil {
		return fmt.Errorf("submit kyc: %w", err)
	}

	log.Info().Str("user_id", v.UserID).Msg("KYC verification submitted")
	return nil
}

// GetStatus returns the current KYC status for a user.
func (s *Service) GetStatus(ctx context.Context, userID string) (*Verification, error) {
	v := &Verification{}
	var reviewedAt *time.Time
	err := s.pool.QueryRow(ctx,
		`SELECT id, user_id, status, level, full_name, doc_type, doc_number, doc_image_url, review_notes, reviewed_by, submitted_at, reviewed_at
		 FROM kyc_verifications WHERE user_id = $1`, userID,
	).Scan(&v.ID, &v.UserID, &v.Status, &v.Level, &v.FullName, &v.DocType, &v.DocNumber, &v.DocImageURL, &v.ReviewNotes, &v.ReviewedBy, &v.SubmittedAt, &reviewedAt)
	if err != nil {
		return nil, err
	}
	if reviewedAt != nil {
		v.ReviewedAt = reviewedAt
	}
	return v, nil
}

// Approve approves a KYC verification.
func (s *Service) Approve(ctx context.Context, userID, reviewerID, notes string) error {
	now := time.Now()
	_, err := s.pool.Exec(ctx,
		`UPDATE kyc_verifications SET status = $1, review_notes = $2, reviewed_by = $3, reviewed_at = $4
		 WHERE user_id = $5`,
		string(StatusApproved), notes, reviewerID, now, userID,
	)
	if err != nil {
		return fmt.Errorf("approve kyc: %w", err)
	}

	// Update user's KYC level
	_, _ = s.pool.Exec(ctx, `UPDATE users SET kyc_level = 1, updated_at = NOW() WHERE id = $1`, userID)

	log.Info().Str("user_id", userID).Str("reviewer", reviewerID).Msg("KYC approved")
	return nil
}

// Reject rejects a KYC verification.
func (s *Service) Reject(ctx context.Context, userID, reviewerID, notes string) error {
	now := time.Now()
	_, err := s.pool.Exec(ctx,
		`UPDATE kyc_verifications SET status = $1, review_notes = $2, reviewed_by = $3, reviewed_at = $4
		 WHERE user_id = $5`,
		string(StatusRejected), notes, reviewerID, now, userID,
	)
	if err != nil {
		return fmt.Errorf("reject kyc: %w", err)
	}

	log.Info().Str("user_id", userID).Str("reviewer", reviewerID).Msg("KYC rejected")
	return nil
}
