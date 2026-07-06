package cold

import (
	"crypto/ecdsa"
	"fmt"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/crypto"
	"github.com/rs/zerolog/log"
)

// SignatureShare represents one signer's contribution to a multi-sig transaction.
type SignatureShare struct {
	SignerIndex int
	R, S        []byte
	V           uint8
}

// SigningRequest represents a transaction awaiting multi-sig approval.
type SigningRequest struct {
	ID           string
	FromAddress  string
	ToAddress    string
	Amount       string
	Currency     string
	Nonce        uint64
	Data         []byte
	Threshold    int              // M (required signatures)
	TotalSigners int              // N (total signers)
	Shares       []SignatureShare
	CreatedAt    time.Time
	ExpiresAt    time.Time
	Status       string // "pending", "ready", "broadcast", "expired"
	TxHash       string
}

// MultiSigWallet manages M-of-N multi-signature cold wallet operations.
type MultiSigWallet struct {
	mu       sync.RWMutex
	signers  []*ecdsa.PrivateKey // In production, these are in HSMs
	requests map[string]*SigningRequest
}

// NewMultiSigWallet creates a multi-sig wallet with N signers.
func NewMultiSigWallet(n int) *MultiSigWallet {
	signers := make([]*ecdsa.PrivateKey, n)
	for i := 0; i < n; i++ {
		key, _ := crypto.GenerateKey()
		signers[i] = key
	}
	return &MultiSigWallet{
		signers:  signers,
		requests: make(map[string]*SigningRequest),
	}
}

// InitiateSigning creates a new M-of-N signing request.
func (w *MultiSigWallet) InitiateSigning(to string, amount, currency string, nonce uint64, data []byte, threshold int) *SigningRequest {
	w.mu.Lock()
	defer w.mu.Unlock()

	req := &SigningRequest{
		ID:           fmt.Sprintf("ms_%d", time.Now().UnixNano()),
		ToAddress:    to,
		Amount:       amount,
		Currency:     currency,
		Nonce:        nonce,
		Data:         data,
		Threshold:    threshold,
		TotalSigners: len(w.signers),
		CreatedAt:    time.Now(),
		ExpiresAt:    time.Now().Add(24 * time.Hour),
		Status:       "pending",
	}

	w.requests[req.ID] = req
	log.Info().
		Str("request_id", req.ID).
		Int("threshold", threshold).
		Int("total_signers", len(w.signers)).
		Msg("multi-sig signing initiated")
	return req
}

// Sign adds a signature share from signer at index i.
func (w *MultiSigWallet) Sign(requestID string, signerIndex int) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	req, ok := w.requests[requestID]
	if !ok {
		return fmt.Errorf("request %s not found", requestID)
	}
	if req.Status != "pending" {
		return fmt.Errorf("request %s is in status %s", requestID, req.Status)
	}
	if signerIndex < 0 || signerIndex >= len(w.signers) {
		return fmt.Errorf("invalid signer index %d", signerIndex)
	}
	if time.Now().After(req.ExpiresAt) {
		req.Status = "expired"
		return fmt.Errorf("request %s has expired", requestID)
	}

	// In production, the HSM would produce this share
	hash := crypto.Keccak256Hash([]byte(fmt.Sprintf("%s:%s:%d", req.ToAddress, req.Amount, req.Nonce)))
	sig, err := crypto.Sign(hash.Bytes(), w.signers[signerIndex])
	if err != nil {
		return fmt.Errorf("sign: %w", err)
	}

	req.Shares = append(req.Shares, SignatureShare{
		SignerIndex: signerIndex,
		R: sig[:32],
		S: sig[32:64],
		V: sig[64] + 27,
	})

	log.Info().
		Str("request_id", requestID).
		Int("signer", signerIndex).
		Int("collected", len(req.Shares)).
		Int("threshold", req.Threshold).
		Msg("multi-sig share collected")

	if len(req.Shares) >= req.Threshold {
		req.Status = "ready"
		log.Info().Str("request_id", requestID).Msg("multi-sig threshold reached, ready to broadcast")
	}

	return nil
}

// GetReadyRequests returns all requests that have reached the threshold.
func (w *MultiSigWallet) GetReadyRequests() []*SigningRequest {
	w.mu.RLock()
	defer w.mu.RUnlock()
	var ready []*SigningRequest
	for _, req := range w.requests {
		if req.Status == "ready" {
			ready = append(ready, req)
		}
	}
	return ready
}

// MarkBroadcast updates a request status after the transaction is broadcast.
func (w *MultiSigWallet) MarkBroadcast(requestID, txHash string) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if req, ok := w.requests[requestID]; ok {
		req.Status = "broadcast"
		req.TxHash = txHash
		log.Info().Str("request_id", requestID).Str("tx_hash", txHash).Msg("multi-sig transaction broadcast")
	}
}

// SweepRequest generates a transaction to sweep hot wallet funds to cold storage.
func (w *MultiSigWallet) SweepRequest(fromHotWallet, toColdAddress string, amount, currency string, nonce uint64, threshold int) *SigningRequest {
	log.Info().
		Str("from", fromHotWallet).Str("to", toColdAddress).
		Str("amount", amount).Str("currency", currency).
		Msg("hot-to-cold sweep initiated")
	return w.InitiateSigning(toColdAddress, amount, currency, nonce, nil, threshold)
}
