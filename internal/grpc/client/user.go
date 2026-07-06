package client

import (
	"context"

	pb "github.com/exchange/api/proto/gen"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// UserClient wraps the gRPC UserService client.
type UserClient struct {
	conn   *grpc.ClientConn
	client pb.UserServiceClient
}

// NewUserClient creates a gRPC user client.
func NewUserClient(addr string) (*UserClient, error) {
	conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, err
	}
	return &UserClient{
		conn:   conn,
		client: pb.NewUserServiceClient(conn),
	}, nil
}

// Register forwards a register request.
func (c *UserClient) Register(ctx context.Context, req *pb.RegisterRequest) (*pb.RegisterResponse, error) {
	return c.client.Register(ctx, req)
}

// Login forwards a login request.
func (c *UserClient) Login(ctx context.Context, req *pb.LoginRequest) (*pb.LoginResponse, error) {
	return c.client.Login(ctx, req)
}

// GetUser forwards a get user request.
func (c *UserClient) GetUser(ctx context.Context, req *pb.GetUserRequest) (*pb.GetUserResponse, error) {
	return c.client.GetUser(ctx, req)
}

// Close closes the gRPC connection.
func (c *UserClient) Close() error {
	return c.conn.Close()
}
