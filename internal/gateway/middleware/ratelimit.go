package middleware

import (
	"net/http"
	"sync"
	"time"
)

// TokenBucket implements a simple token bucket rate limiter.
type TokenBucket struct {
	rate     float64 // tokens per second
	capacity float64
	tokens   float64
	lastTime time.Time
	mu       sync.Mutex
}

// NewTokenBucket creates a token bucket with the given rate and burst capacity.
func NewTokenBucket(rate, capacity float64) *TokenBucket {
	return &TokenBucket{
		rate:     rate,
		capacity: capacity,
		tokens:   capacity,
		lastTime: time.Now(),
	}
}

// Allow checks if a request is allowed. Returns true if allowed.
func (tb *TokenBucket) Allow() bool {
	tb.mu.Lock()
	defer tb.mu.Unlock()

	now := time.Now()
	elapsed := now.Sub(tb.lastTime).Seconds()
	tb.tokens += elapsed * tb.rate
	if tb.tokens > tb.capacity {
		tb.tokens = tb.capacity
	}
	tb.lastTime = now

	if tb.tokens >= 1 {
		tb.tokens--
		return true
	}
	return false
}

// RateLimiter implements per-IP and per-user rate limiting.
type RateLimiter struct {
	mu       sync.Mutex
	buckets  map[string]*TokenBucket
	ipRate   float64
	userRate float64
}

// NewRateLimiter creates a rate limiter.
func NewRateLimiter(ipRate, userRate float64) *RateLimiter {
	return &RateLimiter{
		buckets:  make(map[string]*TokenBucket),
		ipRate:   ipRate,
		userRate: userRate,
	}
}

// AllowIP checks if an IP is within its rate limit.
func (rl *RateLimiter) AllowIP(ip string) bool {
	return rl.allow("ip:"+ip, rl.ipRate, rl.ipRate*2)
}

// AllowUser checks if a user is within their rate limit.
func (rl *RateLimiter) AllowUser(userID string) bool {
	return rl.allow("user:"+userID, rl.userRate, rl.userRate*2)
}

func (rl *RateLimiter) allow(key string, rate, capacity float64) bool {
	rl.mu.Lock()
	bucket, exists := rl.buckets[key]
	if !exists {
		bucket = NewTokenBucket(rate, capacity)
		rl.buckets[key] = bucket
	}
	rl.mu.Unlock()
	return bucket.Allow()
}

// Middleware returns an HTTP middleware that applies rate limiting.
func (rl *RateLimiter) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ip := r.RemoteAddr
		if forwarded := r.Header.Get("X-Forwarded-For"); forwarded != "" {
			ip = forwarded
		}

		if !rl.AllowIP(ip) {
			http.Error(w, `{"error":"rate limit exceeded"}`, http.StatusTooManyRequests)
			return
		}

		if userID := GetUserID(r); userID != "" {
			if !rl.AllowUser(userID) {
				http.Error(w, `{"error":"rate limit exceeded"}`, http.StatusTooManyRequests)
				return
			}
		}

		next.ServeHTTP(w, r)
	})
}
