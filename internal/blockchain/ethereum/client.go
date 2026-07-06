package ethereum

import (
	"context"
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/rs/zerolog/log"
)

// Client wraps an Ethereum JSON-RPC client.
type Client struct {
	client  *ethclient.Client
	rpcURL  string
	chainID *big.Int
}

// NewClient creates a new Ethereum client connected to the given RPC URL.
func NewClient(rpcURL string) (*Client, error) {
	c, err := ethclient.Dial(rpcURL)
	if err != nil {
		return nil, fmt.Errorf("dial ethereum rpc: %w", err)
	}

	chainID, err := c.ChainID(context.Background())
	if err != nil {
		c.Close()
		return nil, fmt.Errorf("get chain id: %w", err)
	}

	log.Info().
		Str("rpc_url", rpcURL).
		Str("chain_id", chainID.String()).
		Msg("ethereum client connected")

	return &Client{
		client:  c,
		rpcURL:  rpcURL,
		chainID: chainID,
	}, nil
}

// BlockNumber returns the current block number.
func (c *Client) BlockNumber(ctx context.Context) (uint64, error) {
	return c.client.BlockNumber(ctx)
}

// Close shuts down the client.
func (c *Client) Close() {
	c.client.Close()
}
