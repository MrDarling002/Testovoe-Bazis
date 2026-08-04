package email

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/example/task-team-service/internal/config"
	"github.com/example/task-team-service/internal/metrics"
	"github.com/sony/gobreaker"
)

type HTTPSender struct {
	client *http.Client
	baseURL string
	breaker *gobreaker.CircuitBreaker
	metrics *metrics.Metrics
}

func NewHTTPSender(cfg struct {
	BaseURL string
	Timeout time.Duration
}, m *metrics.Metrics) *HTTPSender {
	s := &HTTPSender{
		client: &http.Client{
			Timeout: cfg.Timeout,
		},
		baseURL: cfg.BaseURL,
		metrics: m,
	}

	settings := gobreaker.Settings{
		Name: "email-service",
		MaxRequests: 3,
		Interval: 10 * time.Second,
		Timeout: 30 * time.Second,
		ReadyToTrip: func(counts gobreaker.Counts) bool {
			return counts.ConsecutiveFailures >= 5
		},
	}

	s.breaker = gobreaker.NewCircuitBreaker(settings)

	return s
}

type invitePayload struct {
	Email string `json:"email"`
	TeamName string `json:"team_name"`
	InviterName string `json:"inviter_name"`
}

func (s *HTTPSender) SendInvitation(ctx context.Context, email string, teamName string, inviterName string) error {
	payload := invitePayload{
		Email: email,
		TeamName: teamName,
		InviterName: inviterName,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal email payload: %w", err)
	}

	_, err = s.breaker.Execute(func() (interface{}, error) {
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.baseURL+"/invite", bytes.NewReader(body))
		if err != nil {
			return nil, err
		}

		req.Header.Set("Content-Type", "application/json")

		resp, err := s.client.Do(req)
		if err != nil {
			return nil, err
		}
		defer resp.Body.Close()

		if resp.StatusCode >= 500 {
			return nil, fmt.Errorf("email service returned status %d", resp.StatusCode)
		}

		if resp.StatusCode >= 400 {
			return nil, fmt.Errorf("email service client error status %d", resp.StatusCode)
		}

		return nil, nil
	})

	if err != nil {
		s.metrics.EmailErrors.Inc()
		return err
	}

	return nil
}
