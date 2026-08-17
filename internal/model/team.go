package model

import (
	"fmt"
	"time"
)

type TeamRole string

const (
	TeamRoleOwner  TeamRole = "owner"
	TeamRoleAdmin  TeamRole = "admin"
	TeamRoleMember TeamRole = "member"
)

func (r TeamRole) Valid() bool {
	switch r {
	case TeamRoleOwner, TeamRoleAdmin, TeamRoleMember:
		return true
	default:
		return false
	}
}

func (r TeamRole) String() string {
	return string(r)
}

func ValidateTeamRole(role TeamRole) error {
	if !role.Valid() {
		return fmt.Errorf("invalid team role '%s': must be one of 'owner', 'admin', 'member'", role)
	}
	return nil
}

type TeamMember struct {
	TeamID int64    `db:"team_id"`
	UserID int64    `db:"user_id"`
	Role   TeamRole `db:"role"`
}

type Team struct {
	ID        int64     `db:"id"`
	Name      string    `db:"name"`
	CreatedBy int64     `db:"created_by"`
	CreatedAt time.Time `db:"created_at"`
}
