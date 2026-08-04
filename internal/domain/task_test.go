package domain

import (
	"errors"
	"testing"
)

func TestTaskFiltersNormalize(t *testing.T) {
	tests := []struct {
		name        string
		in          TaskFilters
		wantPage    int
		wantPerPage int
	}{
		{"zero values", TaskFilters{}, 1, DefaultPageSize},
		{"negative page", TaskFilters{Page: -3, PerPage: 10}, 1, 10},
		{"per page capped", TaskFilters{Page: 2, PerPage: 100500}, 2, MaxPageSize},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.in.Normalize()

			if tt.in.Page != tt.wantPage || tt.in.PerPage != tt.wantPerPage {
				t.Fatalf("Normalize() = page %d per_page %d, want %d/%d",
					tt.in.Page, tt.in.PerPage, tt.wantPage, tt.wantPerPage)
			}
		})
	}
}

func TestTaskFiltersValidate(t *testing.T) {
	if err := (TaskFilters{}).Validate(); !errors.Is(err, ErrValidation) {
		t.Fatalf("Validate() without team = %v, want ErrValidation", err)
	}

	if err := (TaskFilters{TeamID: 1, Status: "bogus"}).Validate(); !errors.Is(err, ErrValidation) {
		t.Fatalf("Validate() with bad status = %v, want ErrValidation", err)
	}

	if err := (TaskFilters{TeamID: 1, Status: "done"}).Validate(); err != nil {
		t.Fatalf("Validate() = %v, want nil", err)
	}
}

func TestTaskStatusValid(t *testing.T) {
	for _, s := range []TaskStatus{TaskStatusTodo, TaskStatusInProgress, TaskStatusDone} {
		if !s.Valid() {
			t.Fatalf("%s must be valid", s)
		}
	}

	if TaskStatus("bogus").Valid() {
		t.Fatal("bogus status must be invalid")
	}
}

func TestRoleHelpers(t *testing.T) {
	if !RoleOwner.CanInvite() || !RoleAdmin.CanInvite() || RoleMember.CanInvite() {
		t.Fatal("CanInvite: owner/admin yes, member no")
	}

	if Role("bogus").Valid() {
		t.Fatal("bogus role must be invalid")
	}
}

func TestTaskPatchEmpty(t *testing.T) {
	if !(TaskPatch{}).Empty() {
		t.Fatal("zero patch must be empty")
	}

	title := "x"
	if (TaskPatch{Title: &title}).Empty() {
		t.Fatal("patch with title must not be empty")
	}

	if (TaskPatch{Assignee: OptionalInt64{Set: true}}).Empty() {
		t.Fatal("patch with explicit unassign must not be empty")
	}
}
