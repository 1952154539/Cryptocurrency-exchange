package ethereum

import (
	"context"
	"crypto/ecdsa"
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
)

// WithdrawalProcessor signs and broadcasts withdrawal transactions.
type WithdrawalProcessor struct {
	client   *ethclient.Client
	chainID  *big.Int
	gasLimit uint64
}

// NewWithdrawalProcessor creates a withdrawal processor.
func NewWithdrawalProcessor(client *ethclient.Client, chainID *big.Int) *WithdrawalProcessor {
	return &WithdrawalProcessor{
		client:   client,
		chainID:  chainID,
		gasLimit: 21000, // standard ETH transfer
	}
}

// SendETH sends ETH from a hot wallet to a destination address.
func (p *WithdrawalProcessor) SendETH(ctx context.Context, privateKey *ecdsa.PrivateKey, toAddress string, amountWei *big.Int) (string, error) {
	fromAddr := crypto.PubkeyToAddress(privateKey.PublicKey)

	nonce, err := p.client.PendingNonceAt(ctx, fromAddr)
	if err != nil {
		return "", fmt.Errorf("get nonce: %w", err)
	}

	gasPrice, err := p.client.SuggestGasPrice(ctx)
	if err != nil {
		return "", fmt.Errorf("get gas price: %w", err)
	}

	to := common.HexToAddress(toAddress)
	tx := types.NewTransaction(nonce, to, amountWei, p.gasLimit, gasPrice, nil)

	signedTx, err := types.SignTx(tx, types.NewEIP155Signer(p.chainID), privateKey)
	if err != nil {
		return "", fmt.Errorf("sign tx: %w", err)
	}

	if err := p.client.SendTransaction(ctx, signedTx); err != nil {
		return "", fmt.Errorf("send tx: %w", err)
	}

	return signedTx.Hash().Hex(), nil
}
