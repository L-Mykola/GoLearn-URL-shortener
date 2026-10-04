package httpapi

import (
	"math"
	"net"
	"net/http"
	"strconv"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

const (
	visitorIdleTTL = 10 * time.Minute
	sweepInterval  = time.Minute
)

type visitor struct {
	limiter  *rate.Limiter
	lastSeen time.Time
}

type ipLimiter struct {
	mu        sync.Mutex
	visitors  map[string]*visitor
	limit     rate.Limit
	burst     int
	lastSweep time.Time
}

func newIPLimiter(limit rate.Limit, burst int) *ipLimiter {
	return &ipLimiter{
		visitors:  make(map[string]*visitor),
		limit:     limit,
		burst:     burst,
		lastSweep: time.Now(),
	}
}

func (l *ipLimiter) allow(ip string) (bool, time.Duration) {
	l.mu.Lock()
	defer l.mu.Unlock()

	now := time.Now()
	l.sweep(now)

	v, ok := l.visitors[ip]
	if !ok {
		v = &visitor{limiter: rate.NewLimiter(l.limit, l.burst)}
		l.visitors[ip] = v
	}
	v.lastSeen = now

	res := v.limiter.ReserveN(now, 1)
	if delay := res.DelayFrom(now); delay > 0 {
		res.CancelAt(now)
		return false, delay
	}
	return true, 0
}

func (l *ipLimiter) sweep(now time.Time) {
	if now.Sub(l.lastSweep) < sweepInterval {
		return
	}
	l.lastSweep = now
	for ip, v := range l.visitors {
		if now.Sub(v.lastSeen) > visitorIdleTTL {
			delete(l.visitors, ip)
		}
	}
}

func (h *Handler) rateLimit(next http.Handler) http.Handler {
	if h.limiter == nil {
		return next
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ok, retryAfter := h.limiter.allow(clientIP(r))
		if !ok {
			secs := int(math.Ceil(retryAfter.Seconds()))
			w.Header().Set("Retry-After", strconv.Itoa(secs))
			h.writeError(w, http.StatusTooManyRequests, "rate limit exceeded")
			return
		}
		next.ServeHTTP(w, r)
	})
}

func clientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}
