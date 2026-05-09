package handler

import (
	"evalux-server/internal/config"
	"evalux-server/internal/middleware"
	"evalux-server/internal/model"
	"evalux-server/internal/response"
	"evalux-server/internal/service"

	"github.com/gin-gonic/gin"
)

type AuthHandler struct {
	authService *service.AuthService
	cfg         *config.Config
}

func NewAuthHandler(authService *service.AuthService, cfg *config.Config) *AuthHandler {
	return &AuthHandler{authService: authService, cfg: cfg}
}

func (h *AuthHandler) Register(c *gin.Context) {
	var req model.RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "参数校验失败: "+err.Error())
		return
	}

	userWithRoles, err := h.authService.Register(c.Request.Context(), req.Account, req.Password, req.Nickname)
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	token, err := middleware.GenerateToken(userWithRoles.UserID, h.cfg)
	if err != nil {
		response.ServerError(c, "令牌生成失败")
		return
	}

	response.OKMsg(c, "注册成功", model.AuthResponse{
		Token: token,
		User:  *userWithRoles,
	})
}

func (h *AuthHandler) Login(c *gin.Context) {
	var req model.LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "参数校验失败")
		return
	}

	userWithRoles, err := h.authService.Login(c.Request.Context(), req.Account, req.Password)
	if err != nil {
		response.Unauthorized(c, err.Error())
		return
	}

	token, err := middleware.GenerateToken(userWithRoles.UserID, h.cfg)
	if err != nil {
		response.ServerError(c, "令牌生成失败")
		return
	}

	response.OKMsg(c, "登录成功", model.AuthResponse{
		Token: token,
		User:  *userWithRoles,
	})
}

func (h *AuthHandler) GetMe(c *gin.Context) {
	userID, _ := c.Get("user_id")

	userWithRoles, err := h.authService.GetCurrentUser(c.Request.Context(), userID.(string))
	if err != nil {
		response.Unauthorized(c, err.Error())
		return
	}

	response.OK(c, userWithRoles)
}
