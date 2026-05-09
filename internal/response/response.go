package response

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// R 统一响应结构
type R struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// OK 成功响应
func OK(c *gin.Context, data interface{}) {
	c.JSON(http.StatusOK, R{Code: 0, Message: "ok", Data: data})
}

// OKMsg 成功响应（带自定义消息）
func OKMsg(c *gin.Context, msg string, data interface{}) {
	c.JSON(http.StatusOK, R{Code: 0, Message: msg, Data: data})
}

// Fail 业务失败响应（HTTP 200，业务码非 0）
func Fail(c *gin.Context, bizCode int, msg string) {
	c.JSON(http.StatusOK, R{Code: bizCode, Message: msg})
}

// BadRequest 参数错误 400
func BadRequest(c *gin.Context, msg string) {
	c.JSON(http.StatusBadRequest, R{Code: 400, Message: msg})
}

// Unauthorized 未登录/登录态失效 401
func Unauthorized(c *gin.Context, msg string) {
	c.JSON(http.StatusUnauthorized, R{Code: 401, Message: msg})
}

// Forbidden 无权限 403
func Forbidden(c *gin.Context, msg string) {
	c.JSON(http.StatusForbidden, R{Code: 403, Message: msg})
}

// NotFound 资源不存在 404
func NotFound(c *gin.Context, msg string) {
	c.JSON(http.StatusNotFound, R{Code: 404, Message: msg})
}

// ServerError 服务端错误 500
func ServerError(c *gin.Context, msg string) {
	c.JSON(http.StatusInternalServerError, R{Code: 500, Message: msg})
}
