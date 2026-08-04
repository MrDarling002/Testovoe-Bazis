package metrics

import (
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/prometheus/client_golang/prometheus"
)

type Metrics struct {
	RequestsTotal *prometheus.CounterVec
	RequestDuration *prometheus.HistogramVec
	EmailErrors prometheus.Counter
	RateLimited prometheus.Counter
	CacheHits prometheus.Counter
	CacheMisses prometheus.Counter
}

func New() *Metrics {
	m := &Metrics{
		RequestsTotal: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "http_requests_total",
				Help: "Total HTTP requests",
			},
			[]string{"method", "path", "status"},
		),
		RequestDuration: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Name: "http_request_duration_seconds",
				Help: "HTTP request duration in seconds",
				Buckets: prometheus.DefBuckets,
			},
			[]string{"method", "path"},
		),
		EmailErrors: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "email_errors_total",
			Help: "Total email service errors",
		}),
		RateLimited: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "rate_limited_requests_total",
			Help: "Total rate limited requests",
		}),
		CacheHits: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "cache_hits_total",
			Help: "Total cache hits",
		}),
		CacheMisses: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "cache_misses_total",
			Help: "Total cache misses",
		}),
	}

	prometheus.MustRegister(
		m.RequestsTotal,
		m.RequestDuration,
		m.EmailErrors,
		m.RateLimited,
		m.CacheHits,
		m.CacheMisses,
	)

	return m
}

func (m *Metrics) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		ww := &statusWriter{ResponseWriter: w, status: http.StatusOK}

		next.ServeHTTP(ww, r)

		routePattern := chi.RouteContext(r.Context()).RoutePattern()
		if routePattern == "" {
			routePattern = "unknown"
		}

		status := strconv.Itoa(ww.status)

		m.RequestsTotal.WithLabelValues(r.Method, routePattern, status).Inc()
		m.RequestDuration.WithLabelValues(r.Method, routePattern).Observe(time.Since(start).Seconds())
	})
}

type statusWriter struct {
	http.ResponseWriter
	status int
}

func (w *statusWriter) WriteHeader(code int) {
	w.status = code
	w.ResponseWriter.WriteHeader(code)
}