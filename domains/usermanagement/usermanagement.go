package usermanagement

import (
	"time"
)

type User struct {
	ID        int       `json:"id" db:"id"`
	Username  string    `json:"username" db:"username"`
	Password  string    `json:"password" db:"password"`
	IsActive  bool      `json:"is_active" db:"is_active"`
	CreatedAt time.Time `json:"created_at" db:"created_at"`
	UpdatedAt time.Time `json:"updated_at" db:"updated_at"`
}

type CreateUserRequest struct {
	Username string `json:"username" validate:"required,min=3,max=50"`
	Password string `json:"password" validate:"required,min=6"`
}

type UpdateUserRequest struct {
	Username string `json:"username" validate:"omitempty,min=3,max=50"`
	Password string `json:"password" validate:"omitempty,min=6"`
	IsActive *bool  `json:"is_active"`
}

type UserResponse struct {
	ID          int       `json:"id"`
	Username    string    `json:"username"`
	IsActive    bool      `json:"is_active"`
	IsConnected bool      `json:"is_connected"`
	IsLoggedIn  bool      `json:"is_logged_in"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}
