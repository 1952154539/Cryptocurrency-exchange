package settlement

import (
	"context"

	"github.com/exchange/internal/common/decimal"
)

// FeeTier defines the fee rates for a volume tier.
type FeeTier struct {
	TierName  string
	MinVolume float64
	MakerFee  decimal.Decimal
	TakerFee  decimal.Decimal
}

// Default fee tiers (matching the DB schema).
var DefaultFeeTiers = []FeeTier{
	{MinVolume: 0, MakerFee: decimalMust("0.0010"), TakerFee: decimalMust("0.0010")},
	{MinVolume: 50000, MakerFee: decimalMust("0.0008"), TakerFee: decimalMust("0.0009")},
	{MinVolume: 500000, MakerFee: decimalMust("0.0005"), TakerFee: decimalMust("0.00075")},
	{MinVolume: 5000000, MakerFee: decimalMust("0.0002"), TakerFee: decimalMust("0.0006")},
	{MinVolume: 50000000, MakerFee: decimalMust("0.0000"), TakerFee: decimalMust("0.0005")},
}

func decimalMust(s string) decimal.Decimal {
	d, _ := decimal.NewFromString(s)
	return d
}

// FeeService calculates trading fees based on user volume tiers.
type FeeService struct {
	tiers []FeeTier
}

// NewFeeService creates a fee calculator.
func NewFeeService(tiers []FeeTier) *FeeService {
	if tiers == nil {
		tiers = DefaultFeeTiers
	}
	return &FeeService{tiers: tiers}
}

// CalculateFee computes the fee for a trade.
func (fs *FeeService) CalculateFee(ctx context.Context, userID string, isMaker bool, volume decimal.Decimal) decimal.Decimal {
	tier := fs.getTier(ctx, userID)
	rate := tier.TakerFee
	if isMaker {
		rate = tier.MakerFee
	}
	return volume.Mul(rate)
}

// getTier determines the user's fee tier based on 30-day volume.
func (fs *FeeService) getTier(ctx context.Context, userID string) FeeTier {
	// In production, look up the user's 30d volume from DB/Redis.
	// For MVP, return the default tier.
	return fs.tiers[0]
}

// GetMakerFee returns the maker fee rate for a user.
func (fs *FeeService) GetMakerFee(ctx context.Context, userID string) decimal.Decimal {
	return fs.getTier(ctx, userID).MakerFee
}

// GetTakerFee returns the taker fee rate for a user.
func (fs *FeeService) GetTakerFee(ctx context.Context, userID string) decimal.Decimal {
	return fs.getTier(ctx, userID).TakerFee
}
