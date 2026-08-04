package domain

import "time"

type Role string

const (
	RoleOwner Role = "owner"
	RoleAdmin Role = "admin"
	RoleMember Role = "member"
)

func (r Role) CanInvite() bool {
	return r == RoleOwner || r == RoleAdmin
}

func (r Role) IsOwnerOrAdmin() bool {
	return r == RoleOwner || r == RoleAdmin
}

type User struct {
	ID int64 `db:"id" json:"id"`
	Email string `db:"email" json:"email"`
	Username string `db:"username" json:"username"`
	PasswordHash string `db:"-" json:"-"`
	CreatedAt time.Time `db:"created_at" json:"created_at"`
}

type Team struct {
	ID int64 `db:"id" json:"id"`
	Name string `db:"name" json:"name"`
	Description string `db:"description" json:"description,omitempty"`
	CreatedBy int64 `db:"created_by" json:"created_by"`
	CreatedAt time.Time `db:"created_at" json:"created_at"`
	UpdatedAt time.Time `db:"updated_at" json:"updated_at"`
}

type TeamMember struct {
	TeamID int64 `db:"team_id" json:"team_id"`
	UserID int64 `db:"user_id" json:"user_id"`
	Role Role `db:"role" json:"role"`
	InvitedBy *int64 `db:"invited_by" json:"invited_by,omitempty"`
	CreatedAt time.Time `db:"created_at" json:"created_at"`
}