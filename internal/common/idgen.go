package common

import (
	"crypto/rand"
	"fmt"
	"strings"
	"time"

	"github.com/oklog/ulid/v2"
)

// NewOrderID generates a user-visible order ID like "ord_4H7XK2M9P1".
func NewOrderID() string {
	id := ulid.MustNew(ulid.Timestamp(time.Now()), rand.Reader)
	return "ord_" + strings.ToUpper(id.String()[:11])
}

// NewTradeID generates a trade ID like "trd_8F3K9M2X7P5".
func NewTradeID() string {
	id := ulid.MustNew(ulid.Timestamp(time.Now()), rand.Reader)
	return "trd_" + strings.ToUpper(id.String()[:11])
}

// NewWithdrawalID generates a withdrawal ID like "wdr_A3B7C9D1E5F".
func NewWithdrawalID() string {
	id := ulid.MustNew(ulid.Timestamp(time.Now()), rand.Reader)
	return "wdr_" + strings.ToUpper(id.String()[:11])
}

// NewEventID generates a unique event ID.
func NewEventID() string {
	return ulid.MustNew(ulid.Timestamp(time.Now()), rand.Reader).String()
}

// NewAPIKey generates an API key with prefix.
func NewAPIKey() (key, prefix string) {
	raw := make([]byte, 32)
	rand.Read(raw)
	key = fmt.Sprintf("ak_%x", raw)
	prefix = "ak_"
	return
}

// NewAPISecret generates an API secret.
func NewAPISecret() string {
	raw := make([]byte, 32)
	rand.Read(raw)
	return fmt.Sprintf("%x", raw)
}
