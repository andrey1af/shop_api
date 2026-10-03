package middleware

import (
	"net"
	"net/http"
	"strconv"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

const (
	errCodeTooManyRequests    = "TOO_MANY_REQUESTS"
	errMessageTooManyRequests = "too many requests, try again later"

	idleVisitorTTL = 10 * time.Minute
)

type visitor struct {
	limiter  *rate.Limiter
	lastSeen time.Time
}

type RateLimiter struct {
	limit      rate.Limit
	burst      int
	retryAfter int
	mu         sync.Mutex
	visitors   map[string]*visitor
	lastSweep  time.Time
	now        func() time.Time
}

func NewRateLimiter(perMinute, burst int) *RateLimiter {
	return &RateLimiter{
		limit:      rate.Limit(float64(perMinute) / 60),
		burst:      burst,
		retryAfter: max(1, 60/max(perMinute, 1)),
		visitors:   make(map[string]*visitor),
		now:        time.Now,
	}
}

func (l *RateLimiter) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !l.allow(clientIP(r)) {
			w.Header().Set("Retry-After", strconv.Itoa(l.retryAfter))
			writeError(w, http.StatusTooManyRequests, errCodeTooManyRequests, errMessageTooManyRequests)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func (l *RateLimiter) allow(ip string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()

	now := l.now()
	l.sweep(now)

	v, ok := l.visitors[ip]
	if !ok {
		v = &visitor{limiter: rate.NewLimiter(l.limit, l.burst)}
		l.visitors[ip] = v
	}
	v.lastSeen = now

	return v.limiter.AllowN(now, 1)
}

func (l *RateLimiter) sweep(now time.Time) {
	if now.Sub(l.lastSweep) < idleVisitorTTL {
		return
	}
	l.lastSweep = now

	for ip, v := range l.visitors {
		if now.Sub(v.lastSeen) > idleVisitorTTL {
			delete(l.visitors, ip)
		}
	}
}

func clientIP(r *http.Request) string {
	if ip := r.Header.Get("X-Real-IP"); ip != "" {
		return ip
	}

	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}

	return host
}
