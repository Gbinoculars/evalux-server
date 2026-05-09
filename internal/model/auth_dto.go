package model

// RegisterRequest 注册请求参数
type RegisterRequest struct {
	Account  string `json:"account"  binding:"required,min=3,max=64"`
	Password string `json:"password" binding:"required,min=6,max=128"`
	Nickname string `json:"nickname" binding:"required,min=1,max=64"`
}

// LoginRequest 登录请求参数
type LoginRequest struct {
	Account  string `json:"account"  binding:"required"`
	Password string `json:"password" binding:"required"`
}

// AuthResponse 登录/注册统一响应
type AuthResponse struct {
	Token string        `json:"token"`
	User  UserWithRoles `json:"user"`
}
