package handler

import (
	"evalux-server/internal/response"
	"evalux-server/internal/service"

	"github.com/gin-gonic/gin"
)

type DeviceHandler struct {
	deviceService *service.DeviceService
}

func NewDeviceHandler(deviceService *service.DeviceService) *DeviceHandler {
	return &DeviceHandler{deviceService: deviceService}
}

// ListDevices GET /api/devices - 获取当前连接的 Android 设备列表
func (h *DeviceHandler) ListDevices(c *gin.Context) {
	result, err := h.deviceService.ListDevices(c.Request.Context())
	if err != nil {
		response.ServerError(c, "检测设备失败: "+err.Error())
		return
	}
	response.OK(c, result)
}
