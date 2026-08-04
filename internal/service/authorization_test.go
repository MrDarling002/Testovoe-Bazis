package service

import (
	"errors"
	"testing"

	"github.com/example/Testovoe-Bazis/internal/domain"
)

func TestCanInviteWithRole(t *testing.T) {
	tests := []struct {
		name       string
		actorRole  domain.Role
		targetRole domain.Role
		wantErr    error
	}{
		{"member cannot invite", domain.RoleMember, domain.RoleMember, domain.ErrForbidden},
		{"admin invites member", domain.RoleAdmin, domain.RoleMember, nil},
		{"owner invites member", domain.RoleOwner, domain.RoleMember, nil},
		{"nobody invites owner", domain.RoleOwner, domain.RoleOwner, domain.ErrValidation},
		{"admin cannot invite admin", domain.RoleAdmin, domain.RoleAdmin, domain.ErrForbidden},
		{"owner invites admin", domain.RoleOwner, domain.RoleAdmin, nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := CanInviteWithRole(tt.actorRole, tt.targetRole)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("CanInviteWithRole(%s, %s) = %v, want %v", tt.actorRole, tt.targetRole, err, tt.wantErr)
			}
		})
	}
}

func TestCanUpdateTask(t *testing.T) {
	title := "new title"
	assignee := int64(7)

	titlePatch := domain.TaskPatch{Title: &title}
	assigneePatch := domain.TaskPatch{Assignee: domain.OptionalInt64{Set: true, Value: &assignee}}

	status := domain.TaskStatusDone
	statusPatch := domain.TaskPatch{Status: &status}

	tests := []struct {
		name    string
		patch   domain.TaskPatch
		info    AccessInfo
		wantErr error
	}{
		{"owner updates anything", titlePatch, AccessInfo{TeamRole: domain.RoleOwner}, nil},
		{"admin updates anything", assigneePatch, AccessInfo{TeamRole: domain.RoleAdmin}, nil},
		{"creator updates anything", titlePatch, AccessInfo{TeamRole: domain.RoleMember, IsCreator: true}, nil},
		{"assignee updates status", statusPatch, AccessInfo{TeamRole: domain.RoleMember, IsAssignee: true}, nil},
		{"assignee cannot change title", titlePatch, AccessInfo{TeamRole: domain.RoleMember, IsAssignee: true}, domain.ErrForbidden},
		{"assignee cannot reassign", assigneePatch, AccessInfo{TeamRole: domain.RoleMember, IsAssignee: true}, domain.ErrForbidden},
		{"unrelated member forbidden", statusPatch, AccessInfo{TeamRole: domain.RoleMember}, domain.ErrForbidden},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := CanUpdateTask(tt.patch, tt.info)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("CanUpdateTask() = %v, want %v", err, tt.wantErr)
			}
		})
	}
}
