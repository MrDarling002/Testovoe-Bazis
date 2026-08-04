package service

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"strings"
	"testing"

	"github.com/example/Testovoe-Bazis/internal/domain"
)

type stubTeamStore struct {
	roles        map[memberKey]domain.Role
	members      map[memberKey]bool
	team         domain.Team
	created      *domain.Team
	addMemberErr error
	addedMember  *memberKey
}

func (s *stubTeamStore) Create(_ context.Context, name, description string, creatorID int64) (domain.Team, error) {
	team := domain.Team{ID: 1, Name: name, Description: description, CreatedBy: creatorID}
	s.created = &team

	return team, nil
}

func (s *stubTeamStore) Get(context.Context, int64) (domain.Team, error) {
	return s.team, nil
}

func (s *stubTeamStore) ListByUser(context.Context, int64) ([]domain.Team, error) {
	return []domain.Team{s.team}, nil
}

func (s *stubTeamStore) GetMemberRole(_ context.Context, teamID, userID int64) (domain.Role, error) {
	role, ok := s.roles[memberKey{teamID, userID}]
	if !ok {
		return "", domain.ErrNotFound
	}

	return role, nil
}

func (s *stubTeamStore) IsMember(_ context.Context, teamID, userID int64) (bool, error) {
	return s.members[memberKey{teamID, userID}], nil
}

func (s *stubTeamStore) AddMember(_ context.Context, teamID, userID int64, _ int64, _ domain.Role) error {
	if s.addMemberErr != nil {
		return s.addMemberErr
	}

	s.addedMember = &memberKey{teamID, userID}

	return nil
}

type stubUsers struct {
	usersByID    map[int64]domain.User
	usersByEmail map[string]domain.User
}

func (s *stubUsers) GetByID(_ context.Context, id int64) (domain.User, error) {
	u, ok := s.usersByID[id]
	if !ok {
		return domain.User{}, domain.ErrNotFound
	}

	return u, nil
}

func (s *stubUsers) GetByEmail(_ context.Context, email string) (domain.User, error) {
	u, ok := s.usersByEmail[email]
	if !ok {
		return domain.User{}, domain.ErrNotFound
	}

	return u, nil
}

type stubEmail struct {
	err    error
	called bool
}

func (s *stubEmail) SendInvitation(context.Context, string, string, string) error {
	s.called = true
	return s.err
}

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func TestCreateTeam_Validation(t *testing.T) {
	svc := NewTeamService(&stubTeamStore{}, &stubUsers{}, &stubEmail{}, testLogger())

	if _, err := svc.CreateTeam(context.Background(), 1, "   ", ""); !errors.Is(err, domain.ErrValidation) {
		t.Fatalf("CreateTeam(empty) = %v, want ErrValidation", err)
	}

	longName := strings.Repeat("x", 121)
	if _, err := svc.CreateTeam(context.Background(), 1, longName, ""); !errors.Is(err, domain.ErrValidation) {
		t.Fatalf("CreateTeam(too long) = %v, want ErrValidation", err)
	}
}

func TestCreateTeam_TrimsName(t *testing.T) {
	store := &stubTeamStore{}
	svc := NewTeamService(store, &stubUsers{}, &stubEmail{}, testLogger())

	team, err := svc.CreateTeam(context.Background(), 1, "  my team  ", "desc")
	if err != nil {
		t.Fatalf("CreateTeam() unexpected error: %v", err)
	}

	if team.Name != "my team" {
		t.Fatalf("name = %q, want trimmed", team.Name)
	}
}

func inviteFixture() (*stubTeamStore, *stubUsers, *stubEmail, *TeamService) {
	store := &stubTeamStore{
		roles: map[memberKey]domain.Role{
			{1, 10}: domain.RoleOwner,
			{1, 20}: domain.RoleAdmin,
			{1, 30}: domain.RoleMember,
		},
		members: map[memberKey]bool{
			{1, 10}: true,
			{1, 20}: true,
			{1, 30}: true,
		},
		team: domain.Team{ID: 1, Name: "team"},
	}

	users := &stubUsers{
		usersByID: map[int64]domain.User{
			10: {ID: 10, Username: "owner", Email: "owner@example.com"},
			30: {ID: 30, Username: "member", Email: "member@example.com"},
			40: {ID: 40, Username: "newbie", Email: "newbie@example.com"},
		},
		usersByEmail: map[string]domain.User{
			"newbie@example.com": {ID: 40, Username: "newbie", Email: "newbie@example.com"},
		},
	}

	email := &stubEmail{}

	return store, users, email, NewTeamService(store, users, email, testLogger())
}

func TestInvite_Permissions(t *testing.T) {
	_, _, _, svc := inviteFixture()

	tests := []struct {
		name    string
		actorID int64
		req     InviteInput
		wantErr error
	}{
		{"stranger is forbidden", 99, InviteInput{UserID: 40}, domain.ErrForbidden},
		{"member cannot invite", 30, InviteInput{UserID: 40}, domain.ErrForbidden},
		{"admin cannot grant admin", 20, InviteInput{UserID: 40, Role: domain.RoleAdmin}, domain.ErrForbidden},
		{"nobody grants owner", 10, InviteInput{UserID: 40, Role: domain.RoleOwner}, domain.ErrValidation},
		{"both identifiers rejected", 10, InviteInput{UserID: 40, Email: "newbie@example.com"}, domain.ErrValidation},
		{"no identifiers rejected", 10, InviteInput{}, domain.ErrValidation},
		{"self invite rejected", 10, InviteInput{UserID: 10}, domain.ErrValidation},
		{"existing member conflicts", 10, InviteInput{UserID: 30}, domain.ErrConflict},
		{"unknown user not found", 10, InviteInput{UserID: 777}, domain.ErrNotFound},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := svc.Invite(context.Background(), tt.actorID, 1, tt.req)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("Invite() = %v, want %v", err, tt.wantErr)
			}
		})
	}
}

func TestInvite_SuccessByEmail(t *testing.T) {
	store, _, email, svc := inviteFixture()

	result, err := svc.Invite(context.Background(), 10, 1, InviteInput{Email: "  NEWBIE@example.com "})
	if err != nil {
		t.Fatalf("Invite() unexpected error: %v", err)
	}

	if store.addedMember == nil || store.addedMember.userID != 40 {
		t.Fatalf("added member = %+v, want user 40", store.addedMember)
	}

	if result.TeamMember.Role != domain.RoleMember {
		t.Fatalf("role = %s, want default member", result.TeamMember.Role)
	}

	if !email.called || !result.NotificationSent {
		t.Fatalf("email called = %v, notification sent = %v, want both true", email.called, result.NotificationSent)
	}
}

func TestInvite_EmailFailureDoesNotFailInvite(t *testing.T) {
	store, _, email, svc := inviteFixture()
	email.err = errors.New("email service down")

	result, err := svc.Invite(context.Background(), 10, 1, InviteInput{UserID: 40})
	if err != nil {
		t.Fatalf("Invite() unexpected error: %v", err)
	}

	if store.addedMember == nil {
		t.Fatal("member must be added even when email fails")
	}

	if result.NotificationSent {
		t.Fatal("NotificationSent must be false when email fails")
	}
}

func TestInvite_ConcurrentDuplicateSurfacesConflict(t *testing.T) {
	store, _, _, svc := inviteFixture()
	store.addMemberErr = domain.ErrConflict

	_, err := svc.Invite(context.Background(), 10, 1, InviteInput{UserID: 40})
	if !errors.Is(err, domain.ErrConflict) {
		t.Fatalf("Invite() = %v, want ErrConflict", err)
	}
}
