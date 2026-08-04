package httpapi

import (
	"net"
	"net/http"
	"strconv"

	"github.com/example/Testovoe-Bazis/internal/auth"
	"github.com/example/Testovoe-Bazis/internal/domain"
)

// currentUser extracts the authenticated user ID from the request context and
// writes a 401 response when it is missing.
func (h *Handler) currentUser(w http.ResponseWriter, r *http.Request) (int64, bool) {
	userID, ok := auth.UserIDFromContext(r.Context())
	if !ok {
		h.respondError(w, r, domain.ErrUnauthorized)
		return 0, false
	}

	return userID, true
}

// RateLimit throttles requests per authenticated user, falling back to the
// client IP for unauthenticated endpoints (register/login). On limiter
// infrastructure errors it fails open so Redis downtime does not take the
// API down.
func (h *Handler) RateLimit(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var key string

		if userID, ok := auth.UserIDFromContext(r.Context()); ok {
			key = "ratelimit:user:" + strconv.FormatInt(userID, 10)
		} else {
			host, _, err := net.SplitHostPort(r.RemoteAddr)
			if err != nil {
				host = r.RemoteAddr
			}

			key = "ratelimit:ip:" + host
		}

		allowed, err := h.limiter.Allow(r.Context(), key)
		if err != nil {
			h.logger.Warn("rate limiter error, failing open", "error", err)
			next.ServeHTTP(w, r)

			return
		}

		if !allowed {
			h.metrics.RateLimited.Inc()
			w.Header().Set("Retry-After", "60")
			respondJSON(w, http.StatusTooManyRequests, map[string]string{"error": "rate limit exceeded"})

			return
		}

		next.ServeHTTP(w, r)
	})
}
