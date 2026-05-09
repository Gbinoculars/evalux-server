package model

import "time"

type User struct {
	UserID       string     `json:"user_id"`
	Account      string     `json:"account"`
	PasswordHash string     `json:"-"`
	Nickname     string     `json:"nickname"`
	Status       string     `json:"status"`
	CreatedAt    time.Time  `json:"created_at"`
	LastLoginAt  *time.Time `json:"last_login_at"`
}

type Role struct {
	RoleID   string `json:"role_id"`
	RoleCode string `json:"role_code"`
	RoleName string `json:"role_name"`
}

type UserWithRoles struct {
	User
	Roles []Role `json:"roles"`
}
