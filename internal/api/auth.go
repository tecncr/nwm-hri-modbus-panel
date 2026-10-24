package api

import (
	"crypto/sha256"
	"crypto/subtle"
	"strings"
	"sync"
	"time"

	"github.com/gofiber/fiber/v3"
)

// auth requires the master API key in "Authorization: Bearer <key>" or
// "X-API-Key: <key>". Clients that fail too often are temporarily blocked.
func (s *Server) auth(c fiber.Ctx) error {
	if c.Method() == fiber.MethodOptions {
		return c.Next()
	}
	ip := c.IP()
	if s.authFail.blocked(ip) {
		c.Set(fiber.HeaderRetryAfter, "60")
		return fiber.NewError(fiber.StatusTooManyRequests, "too many failed authentication attempts; try again later")
	}
	key := c.Get("X-API-Key")
	if key == "" {
		if h := c.Get(fiber.HeaderAuthorization); len(h) > 7 && strings.EqualFold(h[:7], "bearer ") {
			key = strings.TrimSpace(h[7:])
		}
	}
	if key == "" || !keyEqual(key, s.APIKey) {
		s.authFail.fail(ip)
		c.Set(fiber.HeaderWWWAuthenticate, `Bearer realm="nwm-panel"`)
		return fiber.NewError(fiber.StatusUnauthorized, "missing or invalid API key")
	}
	return c.Next()
}

// keyEqual compares keys in constant time regardless of their lengths.
func keyEqual(a, b string) bool {
	ha, hb := sha256.Sum256([]byte(a)), sha256.Sum256([]byte(b))
	return subtle.ConstantTimeCompare(ha[:], hb[:]) == 1
}

func (s *Server) authCheck(c fiber.Ctx) error {
	return c.JSON(fiber.Map{"ok": true})
}

// failureLimiter counts authentication failures per client IP over a fixed
// window.
type failureLimiter struct {
	mu     sync.Mutex
	max    int
	window time.Duration
	hits   map[string]*failWindow
	sweep  time.Time
}

type failWindow struct {
	count int
	reset time.Time
}

func newFailureLimiter(max int, window time.Duration) *failureLimiter {
	return &failureLimiter{max: max, window: window, hits: map[string]*failWindow{}}
}

func (l *failureLimiter) blocked(ip string) bool {
	if l.max <= 0 {
		return false
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	w, ok := l.hits[ip]
	return ok && time.Now().Before(w.reset) && w.count >= l.max
}

func (l *failureLimiter) fail(ip string) {
	if l.max <= 0 {
		return
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	now := time.Now()
	if now.After(l.sweep) {
		for k, w := range l.hits {
			if now.After(w.reset) {
				delete(l.hits, k)
			}
		}
		l.sweep = now.Add(l.window)
	}
	w, ok := l.hits[ip]
	if !ok || now.After(w.reset) {
		w = &failWindow{reset: now.Add(l.window)}
		l.hits[ip] = w
	}
	w.count++
}
