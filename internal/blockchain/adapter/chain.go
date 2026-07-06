package adapter

import (
	"context"
	"math/big"
)

// ChainClient abstracts blockchain operations across different chains.
type ChainClient interface {
	ChainID() *big.Int
	ChainName() string
	BlockNumber(ctx context.Context) (uint64, error)
	GetBalance(ctx context.Context, address string) (*big.Int, error)
	GetERC20Balance(ctx context.Context, tokenAddr, holderAddr string) (*big.Int, error)
	SendTransaction(ctx context.Context, signedTx []byte) (string, error)
	SuggestGasPrice(ctx context.Context) (*big.Int, error)
	EstimateGas(ctx context.Context, from, to string, value *big.Int, data []byte) (uint64, error)
	Close()
}

// ChainConfig holds per-chain configuration.
type ChainConfig struct {
	Name               string
	ChainID            int64
	RPCURL             string
	NativeCurrency     string
	Confirmations      uint64
	PollIntervalSec    int
	GasLimit           uint64
	ERC20Tokens        map[string]string // symbol -> contract address
}

// TokenTransfer represents a detected token transfer on-chain.
type TokenTransfer struct {
	TxHash       string
	TokenAddress string
	TokenSymbol  string
	From         string
	To           string
	Amount       *big.Int
	BlockNumber  uint64
	LogIndex     uint
}
