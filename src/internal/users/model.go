package users

import "time"

type User struct {
	ID           string    `json:"id"`
	Email        string    `json:"email"`
	PasswordHash string    `json:"-"`
	Role         UserRole  `json:"role"`
	CreatedAt    time.Time `json:"created_at"`
}

type CreateUserRequest struct {
	Email    string
	Password string
}

type UpdateUserRequest struct {
	ID           string
	Email        string
	PasswordHash string
	Role         UserRole // só admin pode seta
}

type UserResponse struct {
	ID        string    `json:"id"`
	Email     string    `json:"email"`
	Role      UserRole  `json:"role,omitempty"`
	CreatedAt time.Time `json:"created_at,omitempty"`
}

type UserFilter struct {
	Query  string
	Limit  int
	Offset int
}
