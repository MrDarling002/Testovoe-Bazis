package metrics

import (
	"database/sql"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
)

type Metrics struct {
	RequestsTotal   *prometheus.CounterVec
	RequestDuration *prometheus.HistogramVec
	EmailErrors     prometheus.Counter
	RateLimited     prometheus.Counter
	CacheHits       prometheus.Counter
	CacheMisses     prometheus.Counter
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
				Name:    "http_request_duration_seconds",
				Help:    "HTTP request duration in seconds",
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

func RegisterDBStats(db *sql.DB, dbName string) {
	prometheus.MustRegister(collectors.NewDBStatsCollector(db, dbName))
}

func (m *Metrics) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		ww := chimiddleware.NewWrapResponseWriter(w, r.ProtoMajor)

		next.ServeHTTP(ww, r)

		routePattern := chi.RouteContext(r.Context()).RoutePattern()
		if routePattern == "" {
			routePattern = "unknown"
		}

		status := ww.Status()
		if status == 0 {
			status = http.StatusOK
		}

		m.RequestsTotal.WithLabelValues(r.Method, routePattern, strconv.Itoa(status)).Inc()
		m.RequestDuration.WithLabelValues(r.Method, routePattern).Observe(time.Since(start).Seconds())
	})
}
