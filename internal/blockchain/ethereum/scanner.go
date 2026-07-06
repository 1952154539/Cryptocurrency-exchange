package ethereum

import (
	"context"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/rs/zerolog/log"
)

// DepositEvent represents a detected on-chain deposit.
type DepositEvent struct {
	TxHash               string
	Currency             string
	Chain                string
	FromAddress          string
	ToAddress            string
	Amount               string // decimal string
	BlockNumber          uint64
	Confirmations        uint64
	RequiredConfirmations uint64
}

// Scanner scans Ethereum blocks for deposit transactions.
type Scanner struct {
	client        *Client
	lastBlock     uint64
	confirmations uint64
	watchedAddrs  map[string]string // address -> userID
}

// NewScanner creates a new block scanner.
func NewScanner(client *Client, confirmations uint64) *Scanner {
	return &Scanner{
		client:        client,
		confirmations: confirmations,
		watchedAddrs:  make(map[string]string),
	}
}

// WatchAddress adds an address to monitor for deposits.
func (s *Scanner) WatchAddress(address, userID string) {
	s.watchedAddrs[address] = userID
}

// ScanNewBlocks scans for new blocks and detects deposits to watched addresses.
func (s *Scanner) ScanNewBlocks(ctx context.Context) ([]DepositEvent, error) {
	currentBlock, err := s.client.BlockNumber(ctx)
	if err != nil {
		return nil, err
	}

	if s.lastBlock == 0 {
		s.lastBlock = currentBlock
		return nil, nil
	}

	var deposits []DepositEvent

	for blockNum := s.lastBlock + 1; blockNum <= currentBlock; blockNum++ {
		block, err := s.client.client.BlockByNumber(ctx, big.NewInt(int64(blockNum)))
		if err != nil {
			log.Warn().Err(err).Uint64("block", blockNum).Msg("failed to get block")
			continue
		}

		for _, tx := range block.Transactions() {
			if tx.To() == nil {
				continue // contract creation
			}

			toAddr := tx.To().Hex()
			if userID, watched := s.watchedAddrs[toAddr]; watched {
				confirmations := currentBlock - blockNum + 1
				deposits = append(deposits, s.buildDepositEvent(tx, block, toAddr, userID, confirmations))
			}

			// Also check if it's a plain ETH transfer (value > 0)
			if tx.To() != nil && tx.Value().Sign() > 0 {
				toAddr := tx.To().Hex()
				if userID, watched := s.watchedAddrs[toAddr]; watched {
					confirmations := currentBlock - blockNum + 1
					deposits = append(deposits, s.buildDepositEvent(tx, block, toAddr, userID, confirmations))
				}
			}
		}
	}

	s.lastBlock = currentBlock
	return deposits, nil
}

func (s *Scanner) buildDepositEvent(tx *types.Transaction, block *types.Block, toAddr, userID string, confirmations uint64) DepositEvent {
	fromAddr := ""
	if from, err := types.Sender(types.LatestSignerForChainID(tx.ChainId()), tx); err == nil {
		fromAddr = from.Hex()
	}

	return DepositEvent{
		TxHash:               tx.Hash().Hex(),
		Currency:             "ETH",
		Chain:                "ethereum",
		FromAddress:          fromAddr,
		ToAddress:            toAddr,
		Amount:               weiToEther(tx.Value()).String(),
		BlockNumber:          block.NumberU64(),
		Confirmations:        confirmations,
		RequiredConfirmations: s.confirmations,
	}
}

func weiToEther(wei *big.Int) *big.Float {
	return new(big.Float).Quo(new(big.Float).SetInt(wei), new(big.Float).SetInt(big.NewInt(1e18)))
}

func stringToCommon(s string) common.Address {
	return common.HexToAddress(s)
}

var _ = stringToCommon
