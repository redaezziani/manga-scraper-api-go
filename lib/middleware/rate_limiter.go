package middleware

import (
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
)

type ClientInfo struct {
	requestCount int
	lastRequest  time.Time
}

type RateLimiter struct {
	clients  map[string]*ClientInfo
	mu       sync.Mutex
	limit    int
	interval time.Duration
}

func NewRateLimiter(limit int, interval time.Duration) *RateLimiter {
	return &RateLimiter{
		clients:  make(map[string]*ClientInfo),
		limit:    limit,
		interval: interval,
	}
}

func extractClientIP(r *http.Request) string {
	xff := r.Header.Get("X-Forwarded-For")
	if xff != "" {
		parts := strings.Split(xff, ",")
		return strings.TrimSpace(parts[0])
	}
	ip, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return ip
}

func (rl *RateLimiter) Limit(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		clientIP := extractClientIP(r)

		rl.mu.Lock()
		defer rl.mu.Unlock()

		clientInfo, exists := rl.clients[clientIP]
		if !exists {
			rl.clients[clientIP] = &ClientInfo{
				requestCount: 1,
				lastRequest:  time.Now(),
			}
			next.ServeHTTP(w, r)
			return
		}

		if time.Since(clientInfo.lastRequest) > rl.interval {
			clientInfo.requestCount = 1
			clientInfo.lastRequest = time.Now()
			next.ServeHTTP(w, r)
			return
		}

		if clientInfo.requestCount < rl.limit {
			clientInfo.requestCount++
			clientInfo.lastRequest = time.Now()
			next.ServeHTTP(w, r)
			return
		}

		http.Error(w, "Too many requests", http.StatusTooManyRequests)
	})
}
