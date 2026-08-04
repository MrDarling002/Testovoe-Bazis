package service

import (
	"context"

	"github.com/example/Testovoe-Bazis/internal/domain"
)

// AnalyticsGateway queries are scoped by user ID at the SQL level, so every
// report only covers teams the requesting user belongs to.
type AnalyticsGateway interface {
	TeamSummary(ctx context.Context, userID int64) ([]domain.TeamSummary, error)
	TopCreators(ctx context.Context, userID int64) ([]domain.TopCreator, error)
	InvalidAssignees(ctx context.Context, userID int64) ([]domain.InvalidAssigneeTask, error)
}

type AnalyticsService struct {
	repo AnalyticsGateway
}

func NewAnalyticsService(repo AnalyticsGateway) *AnalyticsService {
	return &AnalyticsService{repo: repo}
}

func (s *AnalyticsService) TeamsSummary(ctx context.Context, actorID int64) ([]domain.TeamSummary, error) {
	return s.repo.TeamSummary(ctx, actorID)
}

func (s *AnalyticsService) TopCreators(ctx context.Context, actorID int64) ([]domain.TopCreator, error) {
	return s.repo.TopCreators(ctx, actorID)
}

func (s *AnalyticsService) InvalidAssignees(ctx context.Context, actorID int64) ([]domain.InvalidAssigneeTask, error) {
	return s.repo.InvalidAssignees(ctx, actorID)
}
