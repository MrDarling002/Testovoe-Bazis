package service

import "github.com/example/Testovoe-Bazis/internal/domain"

type AccessInfo struct {
	TeamRole   domain.Role
	IsCreator  bool
	IsAssignee bool
}

// CanInviteWithRole checks that the actor may invite a user with targetRole:
// only owner/admin can invite, nobody can invite a second owner, and only the
// owner can grant the admin role.
func CanInviteWithRole(actorRole, targetRole domain.Role) error {
	if !actorRole.CanInvite() {
		return domain.ErrForbidden
	}

	if targetRole == domain.RoleOwner {
		return domain.ErrValidation
	}

	if targetRole == domain.RoleAdmin && actorRole != domain.RoleOwner {
		return domain.ErrForbidden
	}

	return nil
}

// CanUpdateTask enforces update permissions: owner/admin and the task creator
// may change anything; the assignee may only change status and description.
func CanUpdateTask(patch domain.TaskPatch, info AccessInfo) error {
	if info.TeamRole.IsOwnerOrAdmin() {
		return nil
	}

	if info.IsCreator {
		return nil
	}

	if info.IsAssignee {
		if patch.Title != nil || patch.Assignee.Set {
			return domain.ErrForbidden
		}

		return nil
	}

	return domain.ErrForbidden
}
