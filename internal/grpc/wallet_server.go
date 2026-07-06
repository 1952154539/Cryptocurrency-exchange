package grpc

import (
	"context"

	pb "github.com/exchange/api/proto/gen"
	"github.com/exchange/internal/common"
	"github.com/exchange/internal/common/decimal"
	"github.com/exchange/internal/wallet"
)

// WalletServer implements the WalletService gRPC server.
type WalletServer struct {
	pb.UnimplementedWalletServiceServer
	walletSvc *wallet.Service
}

// NewWalletServer creates a gRPC wallet server.
func NewWalletServer(walletSvc *wallet.Service) *WalletServer {
	return &WalletServer{walletSvc: walletSvc}
}

// GetDepositAddress implements the GetDepositAddress RPC.
func (s *WalletServer) GetDepositAddress(ctx context.Context, req *pb.GetDepositAddressRequest) (*pb.GetDepositAddressResponse, error) {
	addr, err := s.walletSvc.GenerateDepositAddress(ctx, req.UserId, common.Currency(req.Currency), common.Chain(req.Chain))
	if err != nil {
		return nil, err
	}
	return &pb.GetDepositAddressResponse{
		Address: addr,
		Chain:   req.Chain,
	}, nil
}

// RequestWithdrawal implements the RequestWithdrawal RPC.
func (s *WalletServer) RequestWithdrawal(ctx context.Context, req *pb.WithdrawalRequest) (*pb.WithdrawalResponse, error) {
	amount, err := decimal.NewFromString(req.Amount)
	if err != nil {
		return nil, err
	}
	fee, err := decimal.NewFromString(req.Fee)
	if err != nil {
		return nil, err
	}

	result, err := s.walletSvc.RequestWithdrawal(ctx, &wallet.WithdrawalRequest{
		UserID:    req.UserId,
		Currency:  req.Currency,
		Chain:     req.Chain,
		ToAddress: req.ToAddress,
		Amount:    amount,
		Fee:       fee,
	})
	if err != nil {
		return nil, err
	}

	return &pb.WithdrawalResponse{
		WithdrawalId: result.WithdrawalID,
		Status:       result.Status,
	}, nil
}

// GetBalances implements the GetBalances RPC.
func (s *WalletServer) GetBalances(ctx context.Context, req *pb.GetBalancesRequest) (*pb.GetBalancesResponse, error) {
	// Query account balances from DB via wallet service
	// For now return available currencies from the hot wallet caps
	return &pb.GetBalancesResponse{
		Balances: []*pb.Balance{
			{Currency: "ETH", Available: "0", Frozen: "0"},
			{Currency: "USDT", Available: "0", Frozen: "0"},
		},
	}, nil
}
