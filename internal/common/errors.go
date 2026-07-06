package common

import "errors"

// Domain errors - use errors.Is to match.
var (
	ErrInsufficientBalance   = errors.New("insufficient balance")
	ErrOrderNotFound         = errors.New("order not found")
	ErrMarketNotAvailable    = errors.New("market not available")
	ErrInvalidPricePrecision = errors.New("invalid price precision")
	ErrInvalidQtyPrecision   = errors.New("invalid quantity precision")
	ErrOrderSizeOutOfRange   = errors.New("order size out of range")
	ErrRateLimitExceeded     = errors.New("rate limit exceeded")
	ErrRiskBlocked           = errors.New("order blocked by risk control")
	ErrInvalidOrderType      = errors.New("invalid order type")
	ErrInvalidTimeInForce    = errors.New("invalid time in force")
	ErrUserNotFound          = errors.New("user not found")
	ErrUserSuspended         = errors.New("user suspended")
	ErrWithdrawalTooLarge    = errors.New("withdrawal exceeds daily limit")
	ErrInvalidAddress        = errors.New("invalid blockchain address")
	ErrDepositNotConfirmed   = errors.New("deposit not yet confirmed")
	ErrDuplicateDeposit      = errors.New("duplicate deposit transaction")
	ErrUnauthorized          = errors.New("unauthorized")
	ErrInvalidSignature      = errors.New("invalid API signature")
	ErrTimestampExpired      = errors.New("request timestamp expired")
	ErrInvalidAPIKey         = errors.New("invalid API key")
	ErrInternalServer        = errors.New("internal server error")
)
