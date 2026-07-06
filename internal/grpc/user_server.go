package grpc

import (
	"context"

	pb "github.com/exchange/api/proto/gen"
	"github.com/exchange/internal/common"
	"github.com/exchange/internal/user"
)

// UserServer implements the UserService gRPC server.
type UserServer struct {
	pb.UnimplementedUserServiceServer
	userSvc *user.Service
	authSvc *user.AuthService
}

// NewUserServer creates a gRPC user server.
func NewUserServer(userSvc *user.Service, authSvc *user.AuthService) *UserServer {
	return &UserServer{userSvc: userSvc, authSvc: authSvc}
}

// Register implements the Register RPC.
func (s *UserServer) Register(ctx context.Context, req *pb.RegisterRequest) (*pb.RegisterResponse, error) {
	u, err := s.userSvc.RegisterUser(ctx, req.Email, req.Password)
	if err != nil {
		return nil, err
	}
	return &pb.RegisterResponse{
		UserId: u.ID,
		Email:  u.Email,
	}, nil
}

// Login implements the Login RPC.
func (s *UserServer) Login(ctx context.Context, req *pb.LoginRequest) (*pb.LoginResponse, error) {
	token, err := s.userSvc.Login(ctx, req.Email, req.Password)
	if err != nil {
		return nil, err
	}
	// Get user by email to return ID
	// For simplicity, parse the token to get the user ID
	userID, _ := s.authSvc.VerifyJWT(token)
	return &pb.LoginResponse{
		Token:  token,
		UserId: userID,
	}, nil
}

// GetUser implements the GetUser RPC.
func (s *UserServer) GetUser(ctx context.Context, req *pb.GetUserRequest) (*pb.GetUserResponse, error) {
	u, err := s.userSvc.GetUser(ctx, req.UserId)
	if err != nil {
		return nil, err
	}
	return &pb.GetUserResponse{
		UserId:       u.ID,
		Email:        u.Email,
		KycLevel:     int32(u.KYCLevel),
		TwoFaEnabled: u.TwoFAEnabled,
		Status:       u.Status,
	}, nil
}

// UpdateKYC implements the UpdateKYC RPC.
func (s *UserServer) UpdateKYC(ctx context.Context, req *pb.UpdateKYCRequest) (*pb.UpdateKYCResponse, error) {
	if err := s.userSvc.UpdateStatus(ctx, req.UserId, common.UserStatusActive); err != nil {
		return nil, err
	}
	return &pb.UpdateKYCResponse{Status: "updated"}, nil
}
