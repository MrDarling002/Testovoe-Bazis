package service

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/example/Testovoe-Bazis/internal/domain"
)

type memberKey struct {
	teamID int64
	userID int64
}

type stubTeams struct {
	roles     map[memberKey]domain.Role
	roleErr   error
	members   map[memberKey]bool
	memberErr error
}

func (s *stubTeams) GetMemberRole(_ context.Context, teamID, userID int64) (domain.Role, error) {
	if s.roleErr != nil {
		return "", s.roleErr
	}

	role, ok := s.roles[memberKey{teamID, userID}]
	if !ok {
		return "", domain.ErrNotFound
	}

	return role, nil
}

func (s *stubTeams) IsMember(_ context.Context, teamID, userID int64) (bool, error) {
	if s.memberErr != nil {
		return false, s.memberErr
	}

	return s.members[memberKey{teamID, userID}], nil
}

type stubTasks struct {
	task      domain.Task
	getErr    error
	createErr error

	listTasks  []domain.Task
	listTotal  int64
	listErr    error
	listCalled bool

	updateErr error

	history       []domain.TaskHistory
	historyLimit  int
	historyOffset int
}

func (s *stubTasks) Get(context.Context, int64) (domain.Task, error) {
	if s.getErr != nil {
		return domain.Task{}, s.getErr
	}

	return s.task, nil
}

func (s *stubTasks) List(context.Context, domain.TaskFilters) ([]domain.Task, int64, error) {
	s.listCalled = true

	if s.listErr != nil {
		return nil, 0, s.listErr
	}

	return s.listTasks, s.listTotal, nil
}

func (s *stubTasks) Create(_ context.Context, task domain.Task) (domain.Task, error) {
	if s.createErr != nil {
		return domain.Task{}, s.createErr
	}

	task.ID = 42

	return task, nil
}

func (s *stubTasks) Update(
	_ context.Context,
	_ int64,
	patch domain.TaskPatch,
	_ int64,
	authorize func(task domain.Task) error,
) (domain.Task, error) {
	if s.updateErr != nil {
		return domain.Task{}, s.updateErr
	}

	if err := authorize(s.task); err != nil {
		return domain.Task{}, err
	}

	updated := s.task
	if patch.Title != nil {
		updated.Title = *patch.Title
	}

	return updated, nil
}

func (s *stubTasks) History(_ context.Context, _ int64, limit, offset int) ([]domain.TaskHistory, error) {
	s.historyLimit = limit
	s.historyOffset = offset

	return s.history, nil
}

type stubCache struct {
	getData     []byte
	getOK       bool
	setCalled   bool
	invalidated []int64
}

func (s *stubCache) GetList(context.Context, int64, domain.TaskFilters) ([]byte, bool) {
	return s.getData, s.getOK
}

func (s *stubCache) SetList(context.Context, int64, domain.TaskFilters, []byte) {
	s.setCalled = true
}

func (s *stubCache) InvalidateTeam(_ context.Context, teamID int64) {
	s.invalidated = append(s.invalidated, teamID)
}

func newTaskService(tasks *stubTasks, teams *stubTeams, cache *stubCache) *TaskService {
	if teams.roles == nil {
		teams.roles = map[memberKey]domain.Role{}
	}

	if teams.members == nil {
		teams.members = map[memberKey]bool{}
	}

	return NewTaskService(tasks, teams, cache)
}

func TestTaskCreate_Validation(t *testing.T) {
	svc := newTaskService(&stubTasks{}, &stubTeams{}, &stubCache{})

	tests := []struct {
		name string
		in   CreateTaskInput
	}{
		{"empty title", CreateTaskInput{TeamID: 1, Title: "  "}},
		{"title too long", CreateTaskInput{TeamID: 1, Title: strings.Repeat("x", domain.MaxTaskTitleLength+1)}},
		{"invalid status", CreateTaskInput{TeamID: 1, Title: "ok", Status: "bogus"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := svc.Create(context.Background(), 1, tt.in)
			if !errors.Is(err, domain.ErrValidation) {
				t.Fatalf("Create() = %v, want ErrValidation", err)
			}
		})
	}
}

func TestTaskCreate_NotMember(t *testing.T) {
	svc := newTaskService(&stubTasks{}, &stubTeams{}, &stubCache{})

	_, err := svc.Create(context.Background(), 1, CreateTaskInput{TeamID: 5, Title: "task"})
	if !errors.Is(err, domain.ErrForbidden) {
		t.Fatalf("Create() = %v, want ErrForbidden", err)
	}
}

func TestTaskCreate_RoleLookupErrorIsNotForbidden(t *testing.T) {
	dbErr := errors.New("connection refused")
	svc := newTaskService(&stubTasks{}, &stubTeams{roleErr: dbErr}, &stubCache{})

	_, err := svc.Create(context.Background(), 1, CreateTaskInput{TeamID: 5, Title: "task"})
	if errors.Is(err, domain.ErrForbidden) {
		t.Fatalf("infrastructure error must not be reported as forbidden, got %v", err)
	}

	if !errors.Is(err, dbErr) {
		t.Fatalf("Create() = %v, want wrapped %v", err, dbErr)
	}
}

func TestTaskCreate_AssigneeNotMember(t *testing.T) {
	teams := &stubTeams{
		roles: map[memberKey]domain.Role{{5, 1}: domain.RoleMember},
	}
	svc := newTaskService(&stubTasks{}, teams, &stubCache{})

	assignee := int64(99)

	_, err := svc.Create(context.Background(), 1, CreateTaskInput{TeamID: 5, Title: "task", AssigneeID: &assignee})
	if !errors.Is(err, domain.ErrValidation) {
		t.Fatalf("Create() = %v, want ErrValidation", err)
	}
}

func TestTaskCreate_Success(t *testing.T) {
	teams := &stubTeams{
		roles: map[memberKey]domain.Role{{5, 1}: domain.RoleMember},
	}
	cache := &stubCache{}
	svc := newTaskService(&stubTasks{}, teams, cache)

	task, err := svc.Create(context.Background(), 1, CreateTaskInput{TeamID: 5, Title: "task"})
	if err != nil {
		t.Fatalf("Create() unexpected error: %v", err)
	}

	if task.Status != domain.TaskStatusTodo {
		t.Fatalf("status = %s, want default todo", task.Status)
	}

	if len(cache.invalidated) != 1 || cache.invalidated[0] != 5 {
		t.Fatalf("cache invalidation = %v, want [5]", cache.invalidated)
	}
}

func TestTaskList_Validation(t *testing.T) {
	svc := newTaskService(&stubTasks{}, &stubTeams{}, &stubCache{})

	if _, err := svc.List(context.Background(), 1, domain.TaskFilters{}); !errors.Is(err, domain.ErrValidation) {
		t.Fatalf("List() without team_id = %v, want ErrValidation", err)
	}

	if _, err := svc.List(context.Background(), 1, domain.TaskFilters{TeamID: 5, Status: "bogus"}); !errors.Is(err, domain.ErrValidation) {
		t.Fatalf("List() with bad status = %v, want ErrValidation", err)
	}
}

func TestTaskList_NotMember(t *testing.T) {
	svc := newTaskService(&stubTasks{}, &stubTeams{}, &stubCache{})

	_, err := svc.List(context.Background(), 1, domain.TaskFilters{TeamID: 5})
	if !errors.Is(err, domain.ErrForbidden) {
		t.Fatalf("List() = %v, want ErrForbidden", err)
	}
}

func TestTaskList_CacheHit(t *testing.T) {
	cachedPage := domain.TaskPage{
		Items:   []domain.Task{{ID: 1, Title: "cached"}},
		Total:   1,
		Page:    1,
		PerPage: domain.DefaultPageSize,
	}

	data, err := json.Marshal(cachedPage)
	if err != nil {
		t.Fatal(err)
	}

	teams := &stubTeams{roles: map[memberKey]domain.Role{{5, 1}: domain.RoleMember}}
	tasks := &stubTasks{}
	svc := newTaskService(tasks, teams, &stubCache{getData: data, getOK: true})

	page, err := svc.List(context.Background(), 1, domain.TaskFilters{TeamID: 5})
	if err != nil {
		t.Fatalf("List() unexpected error: %v", err)
	}

	if tasks.listCalled {
		t.Fatal("repository must not be queried on cache hit")
	}

	if len(page.Items) != 1 || page.Items[0].Title != "cached" {
		t.Fatalf("unexpected page from cache: %+v", page)
	}
}

func TestTaskList_CacheMiss(t *testing.T) {
	teams := &stubTeams{roles: map[memberKey]domain.Role{{5, 1}: domain.RoleMember}}
	tasks := &stubTasks{
		listTasks: []domain.Task{{ID: 1}, {ID: 2}},
		listTotal: 50,
	}
	cache := &stubCache{}
	svc := newTaskService(tasks, teams, cache)

	page, err := svc.List(context.Background(), 1, domain.TaskFilters{TeamID: 5, Page: 1, PerPage: 2})
	if err != nil {
		t.Fatalf("List() unexpected error: %v", err)
	}

	if !tasks.listCalled {
		t.Fatal("repository must be queried on cache miss")
	}

	if !cache.setCalled {
		t.Fatal("result must be written to cache")
	}

	if !page.HasNext {
		t.Fatalf("HasNext = false, want true (page 1 of 50 items, per_page 2)")
	}
}

func TestTaskUpdate_Validation(t *testing.T) {
	svc := newTaskService(&stubTasks{}, &stubTeams{}, &stubCache{})

	if _, err := svc.Update(context.Background(), 1, 1, domain.TaskPatch{}); !errors.Is(err, domain.ErrValidation) {
		t.Fatalf("Update() with empty patch = %v, want ErrValidation", err)
	}

	badStatus := domain.TaskStatus("bogus")
	if _, err := svc.Update(context.Background(), 1, 1, domain.TaskPatch{Status: &badStatus}); !errors.Is(err, domain.ErrValidation) {
		t.Fatalf("Update() with bad status = %v, want ErrValidation", err)
	}

	empty := "  "
	if _, err := svc.Update(context.Background(), 1, 1, domain.TaskPatch{Title: &empty}); !errors.Is(err, domain.ErrValidation) {
		t.Fatalf("Update() with empty title = %v, want ErrValidation", err)
	}
}

func TestTaskUpdate_NotFound(t *testing.T) {
	svc := newTaskService(&stubTasks{getErr: domain.ErrNotFound}, &stubTeams{}, &stubCache{})

	title := "x"

	_, err := svc.Update(context.Background(), 1, 1, domain.TaskPatch{Title: &title})
	if !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("Update() = %v, want ErrNotFound", err)
	}
}

func TestTaskUpdate_NotMember(t *testing.T) {
	tasks := &stubTasks{task: domain.Task{ID: 1, TeamID: 5}}
	svc := newTaskService(tasks, &stubTeams{}, &stubCache{})

	title := "x"

	_, err := svc.Update(context.Background(), 1, 1, domain.TaskPatch{Title: &title})
	if !errors.Is(err, domain.ErrForbidden) {
		t.Fatalf("Update() = %v, want ErrForbidden", err)
	}
}

func TestTaskUpdate_AssigneeNotMember(t *testing.T) {
	tasks := &stubTasks{task: domain.Task{ID: 1, TeamID: 5, CreatedBy: 1}}
	teams := &stubTeams{roles: map[memberKey]domain.Role{{5, 1}: domain.RoleMember}}
	svc := newTaskService(tasks, teams, &stubCache{})

	assignee := int64(99)
	patch := domain.TaskPatch{Assignee: domain.OptionalInt64{Set: true, Value: &assignee}}

	_, err := svc.Update(context.Background(), 1, 1, patch)
	if !errors.Is(err, domain.ErrValidation) {
		t.Fatalf("Update() = %v, want ErrValidation", err)
	}
}

func TestTaskUpdate_UnassignSkipsMembershipCheck(t *testing.T) {
	tasks := &stubTasks{task: domain.Task{ID: 1, TeamID: 5, CreatedBy: 1}}
	teams := &stubTeams{
		roles:     map[memberKey]domain.Role{{5, 1}: domain.RoleMember},
		memberErr: errors.New("IsMember must not be called for unassign"),
	}
	cache := &stubCache{}
	svc := newTaskService(tasks, teams, cache)

	patch := domain.TaskPatch{Assignee: domain.OptionalInt64{Set: true, Value: nil}}

	if _, err := svc.Update(context.Background(), 1, 1, patch); err != nil {
		t.Fatalf("Update() unexpected error: %v", err)
	}

	if len(cache.invalidated) != 1 {
		t.Fatalf("cache invalidation = %v, want one entry", cache.invalidated)
	}
}

func TestTaskUpdate_AssigneeRoleRestrictions(t *testing.T) {
	actorID := int64(2)
	tasks := &stubTasks{task: domain.Task{ID: 1, TeamID: 5, CreatedBy: 1, AssigneeID: &actorID}}
	teams := &stubTeams{roles: map[memberKey]domain.Role{{5, actorID}: domain.RoleMember}}
	svc := newTaskService(tasks, teams, &stubCache{})

	title := "renamed"

	_, err := svc.Update(context.Background(), actorID, 1, domain.TaskPatch{Title: &title})
	if !errors.Is(err, domain.ErrForbidden) {
		t.Fatalf("assignee renaming a task = %v, want ErrForbidden", err)
	}

	status := domain.TaskStatusDone

	if _, err := svc.Update(context.Background(), actorID, 1, domain.TaskPatch{Status: &status}); err != nil {
		t.Fatalf("assignee changing status = %v, want nil", err)
	}
}

func TestTaskHistory_Forbidden(t *testing.T) {
	tasks := &stubTasks{task: domain.Task{ID: 1, TeamID: 5}}
	svc := newTaskService(tasks, &stubTeams{}, &stubCache{})

	_, err := svc.History(context.Background(), 1, 1, 1, 20)
	if !errors.Is(err, domain.ErrForbidden) {
		t.Fatalf("History() = %v, want ErrForbidden", err)
	}
}

func TestTaskHistory_PaginationNormalized(t *testing.T) {
	tasks := &stubTasks{task: domain.Task{ID: 1, TeamID: 5}}
	teams := &stubTeams{roles: map[memberKey]domain.Role{{5, 1}: domain.RoleMember}}
	svc := newTaskService(tasks, teams, &stubCache{})

	if _, err := svc.History(context.Background(), 1, 1, 0, 0); err != nil {
		t.Fatalf("History() unexpected error: %v", err)
	}

	if tasks.historyLimit != domain.DefaultPageSize || tasks.historyOffset != 0 {
		t.Fatalf("limit/offset = %d/%d, want %d/0", tasks.historyLimit, tasks.historyOffset, domain.DefaultPageSize)
	}

	if _, err := svc.History(context.Background(), 1, 1, 3, 1000); err != nil {
		t.Fatalf("History() unexpected error: %v", err)
	}

	if tasks.historyLimit != domain.MaxPageSize || tasks.historyOffset != domain.MaxPageSize*2 {
		t.Fatalf("limit/offset = %d/%d, want %d/%d",
			tasks.historyLimit, tasks.historyOffset, domain.MaxPageSize, domain.MaxPageSize*2)
	}
}
