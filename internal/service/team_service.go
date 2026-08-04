package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"github.com/example/Testovoe-Bazis/internal/domain"
)

const maxTeamNameLength = 120

type UserGateway interface {
	GetByID(ctx context.Context, id int64) (domain.User, error)
	GetByEmail(ctx context.Context, email string) (domain.User, error)
}

type EmailGateway interface {
	SendInvitation(ctx context.Context, email string, teamName string, inviterName string) error
}

type TeamStore interface {
	Create(ctx context.Context, name string, description string, creatorID int64) (domain.Team, error)
	Get(ctx context.Context, id int64) (domain.Team, error)
	ListByUser(ctx context.Context, userID int64) ([]domain.Team, error)
	GetMemberRole(ctx context.Context, teamID int64, userID int64) (domain.Role, error)
	IsMember(ctx context.Context, teamID int64, userID int64) (bool, error)
	AddMember(ctx context.Context, teamID int64, userID int64, invitedBy int64, role domain.Role) error
}

// InviteInput identifies the invitee either by user ID or by email
// (exactly one must be set) and carries the target role.
type InviteInput struct {
	UserID int64
	Email  string
	Role   domain.Role
}

func (r *InviteInput) Normalize() {
	r.Email = strings.ToLower(strings.TrimSpace(r.Email))

	if r.Role == "" {
		r.Role = domain.RoleMember
	}
}

func (r InviteInput) Validate() error {
	hasEmail := r.Email != ""
	hasUserID := r.UserID != 0

	switch {
	case hasEmail && hasUserID:
		return fmt.Errorf("%w: provide either user_id or email, not both", domain.ErrValidation)
	case !hasEmail && !hasUserID:
		return fmt.Errorf("%w: user_id or email is required", domain.ErrValidation)
	case !r.Role.Valid():
		return fmt.Errorf("%w: unknown role %q", domain.ErrValidation, r.Role)
	}

	return nil
}

type InviteResult struct {
	TeamMember       domain.TeamMember
	NotificationSent bool
}

type TeamService struct {
	teams  TeamStore
	users  UserGateway
	email  EmailGateway
	logger *slog.Logger
}

func NewTeamService(teams TeamStore, users UserGateway, email EmailGateway, logger *slog.Logger) *TeamService {
	return &TeamService{
		teams:  teams,
		users:  users,
		email:  email,
		logger: logger,
	}
}

func (s *TeamService) CreateTeam(ctx context.Context, actorID int64, name string, description string) (domain.Team, error) {
	name = strings.TrimSpace(name)

	if name == "" {
		return domain.Team{}, fmt.Errorf("%w: team name is required", domain.ErrValidation)
	}

	if len([]rune(name)) > maxTeamNameLength {
		return domain.Team{}, fmt.Errorf(
			"%w: team name must be at most %d characters",
			domain.ErrValidation, maxTeamNameLength,
		)
	}

	return s.teams.Create(ctx, name, description, actorID)
}

func (s *TeamService) ListTeams(ctx context.Context, actorID int64) ([]domain.Team, error) {
	return s.teams.ListByUser(ctx, actorID)
}

func (s *TeamService) Invite(ctx context.Context, actorID int64, teamID int64, req InviteInput) (InviteResult, error) {
	actorRole, err := s.teams.GetMemberRole(ctx, teamID, actorID)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return InviteResult{}, domain.ErrForbidden
		}

		return InviteResult{}, fmt.Errorf("get member role: %w", err)
	}

	req.Normalize()

	if err := req.Validate(); err != nil {
		return InviteResult{}, err
	}

	if err := CanInviteWithRole(actorRole, req.Role); err != nil {
		return InviteResult{}, err
	}

	invitee, err := s.resolveInvitee(ctx, req)
	if err != nil {
		return InviteResult{}, err
	}

	if invitee.ID == actorID {
		return InviteResult{}, fmt.Errorf("%w: cannot invite yourself", domain.ErrValidation)
	}

	alreadyMember, err := s.teams.IsMember(ctx, teamID, invitee.ID)
	if err != nil {
		return InviteResult{}, fmt.Errorf("check membership: %w", err)
	}

	if alreadyMember {
		return InviteResult{}, fmt.Errorf("%w: user is already a member of the team", domain.ErrConflict)
	}

	team, actor, err := s.loadInviteContext(ctx, teamID, actorID)
	if err != nil {
		return InviteResult{}, err
	}

	// AddMember relies on the composite primary key to resolve the race
	// between the IsMember check above and this insert: a concurrent invite
	// surfaces as domain.ErrConflict.
	if err := s.teams.AddMember(ctx, teamID, invitee.ID, actorID, req.Role); err != nil {
		return InviteResult{}, err
	}

	invitedBy := actorID

	return InviteResult{
		TeamMember: domain.TeamMember{
			TeamID:    teamID,
			UserID:    invitee.ID,
			Role:      req.Role,
			InvitedBy: &invitedBy,
		},
		NotificationSent: s.notifyInvitation(ctx, teamID, invitee.ID, invitee.Email, team.Name, actor.Username),
	}, nil
}

func (s *TeamService) resolveInvitee(ctx context.Context, req InviteInput) (domain.User, error) {
	if req.Email != "" {
		return s.users.GetByEmail(ctx, req.Email)
	}

	return s.users.GetByID(ctx, req.UserID)
}

func (s *TeamService) loadInviteContext(ctx context.Context, teamID, actorID int64) (domain.Team, domain.User, error) {
	team, err := s.teams.Get(ctx, teamID)
	if err != nil {
		return domain.Team{}, domain.User{}, fmt.Errorf("get team: %w", err)
	}

	actor, err := s.users.GetByID(ctx, actorID)
	if err != nil {
		return domain.Team{}, domain.User{}, fmt.Errorf("get actor: %w", err)
	}

	return team, actor, nil
}

func (s *TeamService) notifyInvitation(
	ctx context.Context,
	teamID, userID int64,
	email, teamName, inviterName string,
) bool {
	if err := s.email.SendInvitation(ctx, email, teamName, inviterName); err != nil {
		s.logger.Warn("email invitation failed",
			"team_id", teamID,
			"user_id", userID,
			"error", err,
		)

		return false
	}

	return true
}
