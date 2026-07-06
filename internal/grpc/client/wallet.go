package client

import (
	"context"

	pb "github.com/exchange/api/proto/gen"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// WalletClient wraps the gRPC WalletService client.
type WalletClient struct {
	conn   *grpc.ClientConn
	client pb.WalletServiceClient
}

// NewWalletClient creates a gRPC wallet client.
func NewWalletClient(addr string) (*WalletClient, error) {
	conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, err
	}
	return &WalletClient{
		conn:   conn,
		client: pb.NewWalletServiceClient(conn),
	}, nil
}

// GetDepositAddress forwards a deposit address request.
func (c *WalletClient) GetDepositAddress(ctx context.Context, req *pb.GetDepositAddressRequest) (*pb.GetDepositAddressResponse, error) {
	return c.client.GetDepositAddress(ctx, req)
}

// RequestWithdrawal forwards a withdrawal request.
func (c *WalletClient) RequestWithdrawal(ctx context.Context, req *pb.WithdrawalRequest) (*pb.WithdrawalResponse, error) {
	return c.client.RequestWithdrawal(ctx, req)
}

// GetBalances forwards a balances request.
func (c *WalletClient) GetBalances(ctx context.Context, req *pb.GetBalancesRequest) (*pb.GetBalancesResponse, error) {
	return c.client.GetBalances(ctx, req)
}

// Close closes the gRPC connection.
func (c *WalletClient) Close() error {
	return c.conn.Close()
}
