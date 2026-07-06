package grpc

import (
	"context"

	pb "github.com/exchange/api/proto/gen"
	"github.com/exchange/internal/common"
	"github.com/exchange/internal/common/decimal"
	"github.com/exchange/internal/order"
)

// OrderServer implements the OrderService gRPC server.
type OrderServer struct {
	pb.UnimplementedOrderServiceServer
	orderSvc *order.Service
}

// NewOrderServer creates a gRPC order server.
func NewOrderServer(orderSvc *order.Service) *OrderServer {
	return &OrderServer{orderSvc: orderSvc}
}

// PlaceOrder implements the PlaceOrder RPC.
func (s *OrderServer) PlaceOrder(ctx context.Context, req *pb.PlaceOrderRequest) (*pb.PlaceOrderResponse, error) {
	price, err := decimal.NewFromString(req.Price)
	if err != nil {
		return nil, err
	}
	stopPrice, _ := decimal.NewFromString(req.StopPrice)
	quantity, err := decimal.NewFromString(req.Quantity)
	if err != nil {
		return nil, err
	}

	resp, err := s.orderSvc.PlaceOrder(ctx, &order.PlaceOrderRequest{
		UserID:        req.UserId,
		Symbol:        common.Symbol(req.Symbol),
		Side:          common.Side(req.Side),
		Type:          common.OrderType(req.Type),
		TimeInForce:   common.TimeInForce(req.TimeInForce),
		Price:         price,
		StopPrice:     stopPrice,
		Quantity:      quantity,
		ClientOrderID: req.ClientOrderId,
	})
	if err != nil {
		return nil, err
	}

	return &pb.PlaceOrderResponse{
		OrderId:       resp.OrderID,
		ClientOrderId: resp.ClientOrderID,
		Status:        string(resp.Status),
		FilledQty:     resp.FilledQty.String(),
	}, nil
}

// CancelOrder implements the CancelOrder RPC.
func (s *OrderServer) CancelOrder(ctx context.Context, req *pb.CancelOrderRequest) (*pb.CancelOrderResponse, error) {
	if err := s.orderSvc.CancelOrder(ctx, req.UserId, common.Symbol(req.Symbol), req.OrderId); err != nil {
		return nil, err
	}
	return &pb.CancelOrderResponse{Status: "cancelled"}, nil
}

// GetOrder implements the GetOrder RPC.
func (s *OrderServer) GetOrder(ctx context.Context, req *pb.GetOrderRequest) (*pb.GetOrderResponse, error) {
	o, err := s.orderSvc.GetOrder(ctx, req.OrderId)
	if err != nil {
		return nil, err
	}
	return &pb.GetOrderResponse{
		OrderId:   o.OrderID,
		UserId:    o.UserID,
		Symbol:    o.Symbol,
		Side:      o.Side,
		Type:      o.Type,
		Price:     o.Price,
		Quantity:  o.Quantity,
		FilledQty: o.FilledQty,
		Status:    o.Status,
		CreatedAt: o.CreatedAt.Unix(),
	}, nil
}

// GetOpenOrders implements the GetOpenOrders RPC.
func (s *OrderServer) GetOpenOrders(ctx context.Context, req *pb.GetOpenOrdersRequest) (*pb.GetOpenOrdersResponse, error) {
	orders, err := s.orderSvc.GetOpenOrders(ctx, req.UserId, req.Symbol)
	if err != nil {
		return nil, err
	}

	resp := &pb.GetOpenOrdersResponse{}
	for _, o := range orders {
		resp.Orders = append(resp.Orders, &pb.GetOrderResponse{
			OrderId:   o.OrderID,
			UserId:    o.UserID,
			Symbol:    o.Symbol,
			Side:      o.Side,
			Type:      o.Type,
			Price:     o.Price,
			Quantity:  o.Quantity,
			FilledQty: o.FilledQty,
			Status:    o.Status,
			CreatedAt: o.CreatedAt.Unix(),
		})
	}
	return resp, nil
}
