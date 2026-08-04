//go:build integration

package mysql_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jmoiron/sqlx"
	tcmysql "github.com/testcontainers/testcontainers-go/modules/mysql"

	"github.com/example/Testovoe-Bazis/internal/domain"
	mysqlstore "github.com/example/Testovoe-Bazis/internal/store/mysql"
)

func setupDB(t *testing.T) *sqlx.DB {
	t.Helper()

	ctx := context.Background()

	container, err := tcmysql.Run(ctx, "mysql:8.0",
		tcmysql.WithDatabase("task_service"),
		tcmysql.WithUsername("app"),
		tcmysql.WithPassword("app"),
	)
	if err != nil {
		t.Fatalf("start mysql container: %v", err)
	}

	t.Cleanup(func() {
		_ = container.Terminate(context.Background())
	})

	dsn, err := container.ConnectionString(ctx, "parseTime=true", "loc=UTC")
	if err != nil {
		t.Fatalf("connection string: %v", err)
	}

	if err := mysqlstore.Migrate(dsn); err != nil {
		t.Fatalf("apply migrations: %v", err)
	}

	db, err := mysqlstore.NewDB(ctx, mysqlstore.DBConfig{
		DSN:             dsn,
		MaxOpenConns:    5,
		MaxIdleConns:    5,
		ConnMaxLifetime: time.Minute,
	})
	if err != nil {
		t.Fatalf("connect: %v", err)
	}

	t.Cleanup(func() { db.Close() })

	return db
}

func TestRepositoriesIntegration(t *testing.T) {
	db := setupDB(t)
	ctx := context.Background()

	users := mysqlstore.NewUserRepository(db)
	teams := mysqlstore.NewTeamRepository(db)
	tasks := mysqlstore.NewTaskRepository(db)
	analytics := mysqlstore.NewAnalyticsRepository(db)

	alice, err := users.Create(ctx, "alice@example.com", "alice", "hash-alice")
	if err != nil {
		t.Fatalf("create alice: %v", err)
	}

	bob, err := users.Create(ctx, "bob@example.com", "bob", "hash-bob")
	if err != nil {
		t.Fatalf("create bob: %v", err)
	}

	t.Run("duplicate user email conflicts", func(t *testing.T) {
		_, err := users.Create(ctx, "alice@example.com", "alice2", "hash")
		if !errors.Is(err, domain.ErrConflict) {
			t.Fatalf("duplicate email = %v, want ErrConflict", err)
		}
	})

	t.Run("get user by email includes password hash", func(t *testing.T) {
		u, err := users.GetByEmail(ctx, "alice@example.com")
		if err != nil {
			t.Fatal(err)
		}

		if u.PasswordHash != "hash-alice" {
			t.Fatalf("password hash = %q, want stored value", u.PasswordHash)
		}
	})

	team, err := teams.Create(ctx, "dream team", "", alice.ID)
	if err != nil {
		t.Fatalf("create team: %v", err)
	}

	t.Run("team creator becomes owner", func(t *testing.T) {
		role, err := teams.GetMemberRole(ctx, team.ID, alice.ID)
		if err != nil {
			t.Fatal(err)
		}

		if role != domain.RoleOwner {
			t.Fatalf("role = %s, want owner", role)
		}
	})

	t.Run("non-member role is not found", func(t *testing.T) {
		_, err := teams.GetMemberRole(ctx, team.ID, bob.ID)
		if !errors.Is(err, domain.ErrNotFound) {
			t.Fatalf("role for non-member = %v, want ErrNotFound", err)
		}
	})

	if err := teams.AddMember(ctx, team.ID, bob.ID, alice.ID, domain.RoleMember); err != nil {
		t.Fatalf("add bob: %v", err)
	}

	t.Run("duplicate membership conflicts", func(t *testing.T) {
		err := teams.AddMember(ctx, team.ID, bob.ID, alice.ID, domain.RoleMember)
		if !errors.Is(err, domain.ErrConflict) {
			t.Fatalf("duplicate member = %v, want ErrConflict", err)
		}
	})

	task, err := tasks.Create(ctx, domain.Task{
		TeamID:     team.ID,
		Title:      "first task",
		Status:     domain.TaskStatusTodo,
		AssigneeID: &bob.ID,
		CreatedBy:  alice.ID,
	})
	if err != nil {
		t.Fatalf("create task: %v", err)
	}

	t.Run("history scans created row with NULL fields", func(t *testing.T) {
		history, err := tasks.History(ctx, task.ID, 10, 0)
		if err != nil {
			t.Fatalf("history after create: %v", err)
		}

		if len(history) != 1 || history[0].Action != "created" {
			t.Fatalf("history = %+v, want single created row", history)
		}

		if history[0].Field != nil || history[0].OldValue != nil || history[0].NewValue != nil {
			t.Fatalf("created row must have NULL field/old/new, got %+v", history[0])
		}
	})

	t.Run("update writes audit rows and completed_at", func(t *testing.T) {
		status := domain.TaskStatusDone
		title := "renamed task"

		updated, err := tasks.Update(ctx, task.ID, domain.TaskPatch{
			Title:    &title,
			Status:   &status,
			Assignee: domain.OptionalInt64{Set: true, Value: nil},
		}, alice.ID, func(domain.Task) error { return nil })
		if err != nil {
			t.Fatalf("update: %v", err)
		}

		if updated.CompletedAt == nil {
			t.Fatal("completed_at must be set when status becomes done")
		}

		if updated.AssigneeID != nil {
			t.Fatal("assignee must be removed by explicit unassign")
		}

		history, err := tasks.History(ctx, task.ID, 10, 0)
		if err != nil {
			t.Fatal(err)
		}

		if len(history) != 4 {
			t.Fatalf("history rows = %d, want 4", len(history))
		}
	})

	t.Run("unauthorized update is rejected before write", func(t *testing.T) {
		title := "hacked"

		_, err := tasks.Update(ctx, task.ID, domain.TaskPatch{Title: &title}, bob.ID, func(domain.Task) error {
			return domain.ErrForbidden
		})
		if !errors.Is(err, domain.ErrForbidden) {
			t.Fatalf("update = %v, want ErrForbidden", err)
		}
	})

	t.Run("list filters and paginates", func(t *testing.T) {
		for i := 0; i < 3; i++ {
			if _, err := tasks.Create(ctx, domain.Task{
				TeamID:    team.ID,
				Title:     "todo task",
				Status:    domain.TaskStatusTodo,
				CreatedBy: bob.ID,
			}); err != nil {
				t.Fatal(err)
			}
		}

		page, total, err := tasks.List(ctx, domain.TaskFilters{
			TeamID:  team.ID,
			Status:  string(domain.TaskStatusTodo),
			Page:    1,
			PerPage: 2,
		})
		if err != nil {
			t.Fatal(err)
		}

		if total != 3 || len(page) != 2 {
			t.Fatalf("total = %d, page len = %d, want 3/2", total, len(page))
		}
	})

	t.Run("analytics team summary", func(t *testing.T) {
		summary, err := analytics.TeamSummary(ctx, alice.ID)
		if err != nil {
			t.Fatalf("team summary: %v", err)
		}

		if len(summary) != 1 {
			t.Fatalf("summary rows = %d, want 1", len(summary))
		}

		if summary[0].MembersCount != 2 {
			t.Fatalf("members = %d, want 2", summary[0].MembersCount)
		}

		if summary[0].DoneLast7Days != 1 {
			t.Fatalf("done last 7 days = %d, want 1", summary[0].DoneLast7Days)
		}
	})

	t.Run("analytics top creators executes window query", func(t *testing.T) {
		top, err := analytics.TopCreators(ctx, alice.ID)
		if err != nil {
			t.Fatalf("top creators (rank alias regression): %v", err)
		}

		if len(top) != 2 {
			t.Fatalf("top rows = %d, want 2 (alice and bob)", len(top))
		}

		if top[0].UserID != bob.ID || top[0].RankPosition != 1 {
			t.Fatalf("top[0] = %+v, want bob at rank 1 (3 tasks)", top[0])
		}
	})

	t.Run("analytics finds assignees outside the team", func(t *testing.T) {
		orphan, err := tasks.Create(ctx, domain.Task{
			TeamID:     team.ID,
			Title:      "orphan assignee",
			Status:     domain.TaskStatusTodo,
			AssigneeID: &bob.ID,
			CreatedBy:  alice.ID,
		})
		if err != nil {
			t.Fatal(err)
		}

		if _, err := db.ExecContext(ctx,
			`DELETE FROM team_members WHERE team_id = ? AND user_id = ?`, team.ID, bob.ID,
		); err != nil {
			t.Fatal(err)
		}

		invalid, err := analytics.InvalidAssignees(ctx, alice.ID)
		if err != nil {
			t.Fatal(err)
		}

		found := false
		for _, row := range invalid {
			if row.TaskID == orphan.ID {
				found = true
			}
		}

		if !found {
			t.Fatalf("invalid assignees = %+v, want task %d present", invalid, orphan.ID)
		}
	})

	t.Run("analytics is scoped to the caller's teams", func(t *testing.T) {
		summary, err := analytics.TeamSummary(ctx, bob.ID)
		if err != nil {
			t.Fatal(err)
		}

		if len(summary) != 0 {
			t.Fatalf("bob must not see foreign teams, got %+v", summary)
		}
	})
}
