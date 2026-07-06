package telemetry

import (
	"encoding/json"
	"net/http"
	"sync/atomic"
	"time"
)

// Metrics holds all application metrics.
type Metrics struct {
	// HTTP
	RequestsTotal   atomic.Int64
	RequestDuration atomic.Int64 // cumulative microseconds

	// Matching
	OrdersMatched    atomic.Int64
	MatchingDuration atomic.Int64 // cumulative microseconds
	OrdersRejected   atomic.Int64

	// Settlement
	TradesSettled     atomic.Int64
	SettlementErrors  atomic.Int64

	// Wallet
	DepositsConfirmed   atomic.Int64
	WithdrawalsRequested atomic.Int64
}

var globalMetrics = &Metrics{}

// GetMetrics returns the global metrics instance.
func GetMetrics() *Metrics {
	return globalMetrics
}

// RecordHTTP records an HTTP request.
func (m *Metrics) RecordHTTP(duration time.Duration) {
	m.RequestsTotal.Add(1)
	m.RequestDuration.Add(duration.Microseconds())
}

// RecordMatch records a matching operation.
func (m *Metrics) RecordMatch(duration time.Duration, rejected bool) {
	if rejected {
		m.OrdersRejected.Add(1)
	} else {
		m.OrdersMatched.Add(1)
	}
	m.MatchingDuration.Add(duration.Microseconds())
}

// RecordSettlement records a settlement operation.
func (m *Metrics) RecordSettlement(success bool) {
	if success {
		m.TradesSettled.Add(1)
	} else {
		m.SettlementErrors.Add(1)
	}
}

// RecordDeposit records a confirmed deposit.
func (m *Metrics) RecordDeposit() {
	m.DepositsConfirmed.Add(1)
}

// RecordWithdrawal records a withdrawal request.
func (m *Metrics) RecordWithdrawal() {
	m.WithdrawalsRequested.Add(1)
}

// MetricsHandler returns an HTTP handler that serves Prometheus-format metrics.
func MetricsHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		m := globalMetrics
		w.Header().Set("Content-Type", "text/plain; version=0.0.4")

		writeMetric := func(name, help, typ string, value int64) {
			w.Write([]byte("# HELP " + name + " " + help + "\n"))
			w.Write([]byte("# TYPE " + name + " " + typ + "\n"))
			w.Write([]byte(name + " " + itoa(value) + "\n"))
		}

		writeMetric("http_requests_total", "Total HTTP requests", "counter", m.RequestsTotal.Load())
		writeMetric("http_request_duration_us", "Cumulative HTTP request duration in microseconds", "counter", m.RequestDuration.Load())
		writeMetric("orders_matched_total", "Total matched orders", "counter", m.OrdersMatched.Load())
		writeMetric("orders_rejected_total", "Total rejected orders", "counter", m.OrdersRejected.Load())
		writeMetric("matching_duration_us", "Cumulative matching duration in microseconds", "counter", m.MatchingDuration.Load())
		writeMetric("trades_settled_total", "Total settled trades", "counter", m.TradesSettled.Load())
		writeMetric("settlement_errors_total", "Total settlement errors", "counter", m.SettlementErrors.Load())
		writeMetric("deposits_confirmed_total", "Total confirmed deposits", "counter", m.DepositsConfirmed.Load())
		writeMetric("withdrawals_requested_total", "Total withdrawal requests", "counter", m.WithdrawalsRequested.Load())
	})
}

func itoa(n int64) string {
	if n == 0 {
		return "0"
	}
	b := make([]byte, 0, 20)
	neg := n < 0
	if neg {
		n = -n
	}
	for n > 0 {
		b = append([]byte{byte('0' + n%10)}, b...)
		n /= 10
	}
	if neg {
		b = append([]byte{'-'}, b...)
	}
	return string(b)
}

// HealthResponse is returned by the health check endpoint.
type HealthResponse struct {
	Status    string            `json:"status"`
	Timestamp string            `json:"timestamp"`
	Services  map[string]string `json:"services,omitempty"`
}

// HealthHandler returns an HTTP handler that reports service health.
func HealthHandler(checks map[string]func() error) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := HealthResponse{
			Status:    "ok",
			Timestamp: time.Now().UTC().Format(time.RFC3339),
			Services:  make(map[string]string),
		}

		for name, check := range checks {
			if err := check(); err != nil {
				resp.Services[name] = "unhealthy: " + err.Error()
				resp.Status = "degraded"
			} else {
				resp.Services[name] = "healthy"
			}
		}

		if resp.Status != "ok" {
			w.WriteHeader(http.StatusServiceUnavailable)
		}
		json.NewEncoder(w).Encode(resp)
	})
}

// ReadyHandler returns an HTTP handler that reports readiness.
func ReadyHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"status":"ready"}`))
	})
}
