package users

import "fmt"

type UserRole string

const (
	RoleUser  UserRole = "user"
	RoleAdmin UserRole = "admin"
)

func (r UserRole) Valid() bool {
	switch r {
	case RoleUser, RoleAdmin:
		return true
	default:
		return false
	}
}

func ParseUserRole(s string) (UserRole, error) {
	r := UserRole(s)
	if !r.Valid() {
		return "", fmt.Errorf("invalid role: %q", s)
	}
	return r, nil
}
