package client

import (
	"context"

	pb "github.com/exchange/api/proto/gen"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// OrderClient wraps the gRPC OrderService client.
type OrderClient struct {
	conn   *grpc.ClientConn
	client pb.OrderServiceClient
}

// NewOrderClient creates a gRPC order client connected to the given address.
func NewOrderClient(addr string) (*OrderClient, error) {
	conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, err
	}
	return &OrderClient{
		conn:   conn,
		client: pb.NewOrderServiceClient(conn),
	}, nil
}

// PlaceOrder forwards a place order request to the order service.
func (c *OrderClient) PlaceOrder(ctx context.Context, req *pb.PlaceOrderRequest) (*pb.PlaceOrderResponse, error) {
	return c.client.PlaceOrder(ctx, req)
}

// CancelOrder forwards a cancel order request.
func (c *OrderClient) CancelOrder(ctx context.Context, req *pb.CancelOrderRequest) (*pb.CancelOrderResponse, error) {
	return c.client.CancelOrder(ctx, req)
}

// GetOrder forwards a get order request.
func (c *OrderClient) GetOrder(ctx context.Context, req *pb.GetOrderRequest) (*pb.GetOrderResponse, error) {
	return c.client.GetOrder(ctx, req)
}

// GetOpenOrders forwards a get open orders request.
func (c *OrderClient) GetOpenOrders(ctx context.Context, req *pb.GetOpenOrdersRequest) (*pb.GetOpenOrdersResponse, error) {
	return c.client.GetOpenOrders(ctx, req)
}

// Close closes the gRPC connection.
func (c *OrderClient) Close() error {
	return c.conn.Close()
}
