package httpapi

import (
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/example/Testovoe-Bazis/internal/auth"
	"github.com/example/Testovoe-Bazis/internal/metrics"
	"github.com/example/Testovoe-Bazis/internal/ratelimit"
	"github.com/example/Testovoe-Bazis/internal/service"
)

type Handler struct {
	auth      *service.AuthService
	teams     *service.TeamService
	tasks     *service.TaskService
	analytics *service.AnalyticsService
	metrics   *metrics.Metrics
	limiter   ratelimit.Limiter
	jwt       *auth.JWTManager
	logger    *slog.Logger
}

func NewHandler(
	authService *service.AuthService,
	teamService *service.TeamService,
	taskService *service.TaskService,
	analyticsService *service.AnalyticsService,
	m *metrics.Metrics,
	limiter ratelimit.Limiter,
	jwtManager *auth.JWTManager,
	logger *slog.Logger,
) *Handler {
	return &Handler{
		auth:      authService,
		teams:     teamService,
		tasks:     taskService,
		analytics: analyticsService,
		metrics:   m,
		limiter:   limiter,
		jwt:       jwtManager,
		logger:    logger,
	}
}

func (h *Handler) Router() http.Handler {
	r := chi.NewRouter()

	r.Use(chimiddleware.RequestID)
	r.Use(chimiddleware.RealIP)
	r.Use(chimiddleware.Recoverer)
	r.Use(h.metrics.Middleware)

	r.Get("/health", func(w http.ResponseWriter, _ *http.Request) {
		respondJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})

	r.Handle("/metrics", promhttp.Handler())

	r.Route("/api/v1", func(r chi.Router) {
		r.Group(func(r chi.Router) {
			r.Use(h.RateLimit)

			r.Post("/register", h.Register)
			r.Post("/login", h.Login)
		})

		r.Group(func(r chi.Router) {
			r.Use(auth.AuthMiddleware(h.jwt))
			r.Use(h.RateLimit)

			r.Post("/teams", h.CreateTeam)
			r.Get("/teams", h.ListTeams)
			r.Post("/teams/{id}/invite", h.InviteToTeam)

			r.Post("/tasks", h.CreateTask)
			r.Get("/tasks", h.ListTasks)
			r.Put("/tasks/{id}", h.UpdateTask)
			r.Get("/tasks/{id}/history", h.TaskHistory)

			r.Get("/analytics/teams-summary", h.TeamsSummary)
			r.Get("/analytics/top-creators", h.TopCreators)
			r.Get("/analytics/invalid-assignees", h.InvalidAssignees)
		})
	})

	return r
}
