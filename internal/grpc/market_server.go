package grpc

import (
	"context"

	pb "github.com/exchange/api/proto/gen"
	"github.com/exchange/internal/common"
	"github.com/exchange/internal/marketdata"
)

// MarketServer implements the MarketDataService gRPC server.
type MarketServer struct {
	pb.UnimplementedMarketDataServiceServer
	marketSvc *marketdata.Service
}

// NewMarketServer creates a gRPC market data server.
func NewMarketServer(marketSvc *marketdata.Service) *MarketServer {
	return &MarketServer{marketSvc: marketSvc}
}

// GetDepth implements the GetDepth RPC.
func (s *MarketServer) GetDepth(ctx context.Context, req *pb.DepthRequest) (*pb.DepthResponse, error) {
	snap, err := s.marketSvc.GetOrderBook(ctx, common.Symbol(req.Symbol), int(req.Limit))
	if err != nil {
		return nil, err
	}

	resp := &pb.DepthResponse{Symbol: string(snap.Symbol)}
	for _, b := range snap.Bids {
		resp.Bids = append(resp.Bids, &pb.PriceLevel{Price: b.Price, Volume: b.Volume, Orders: int32(b.Orders)})
	}
	for _, a := range snap.Asks {
		resp.Asks = append(resp.Asks, &pb.PriceLevel{Price: a.Price, Volume: a.Volume, Orders: int32(a.Orders)})
	}
	return resp, nil
}

// GetTrades implements the GetTrades RPC.
func (s *MarketServer) GetTrades(ctx context.Context, req *pb.TradesRequest) (*pb.TradesResponse, error) {
	trades, err := s.marketSvc.GetRecentTrades(ctx, common.Symbol(req.Symbol), int(req.Limit))
	if err != nil {
		return nil, err
	}

	resp := &pb.TradesResponse{}
	for _, t := range trades {
		resp.Trades = append(resp.Trades, &pb.Trade{
			Id:           t.ID,
			Price:        t.Price,
			Qty:          t.Quantity,
			QuoteQty:     t.QuoteQty,
			Time:         t.Time.UnixMilli(),
			IsBuyerMaker: t.IsBuyerMaker,
		})
	}
	return resp, nil
}

// GetTicker implements the GetTicker RPC.
func (s *MarketServer) GetTicker(ctx context.Context, req *pb.TickerRequest) (*pb.TickerResponse, error) {
	t, err := s.marketSvc.GetTicker(ctx, common.Symbol(req.Symbol))
	if err != nil {
		return nil, err
	}
	return &pb.TickerResponse{
		Symbol:    t.Symbol,
		Open:      t.Open,
		High:      t.High,
		Low:       t.Low,
		Last:      t.Last,
		Volume:    t.Volume,
		Change:    t.Change,
		ChangePct: t.ChangePct,
	}, nil
}

// GetKlines implements the GetKlines RPC.
func (s *MarketServer) GetKlines(ctx context.Context, req *pb.KlinesRequest) (*pb.KlinesResponse, error) {
	kl, err := s.marketSvc.GetKlines(ctx, common.Symbol(req.Symbol), req.Interval, int(req.Limit))
	if err != nil {
		return nil, err
	}

	resp := &pb.KlinesResponse{}
	for _, k := range kl {
		resp.Klines = append(resp.Klines, &pb.Candlestick{
			OpenTime:  k.OpenTime,
			Open:      k.Open,
			High:      k.High,
			Low:       k.Low,
			Close:     k.Close,
			Volume:    k.Volume,
			CloseTime: k.CloseTime,
		})
	}
	return resp, nil
}
