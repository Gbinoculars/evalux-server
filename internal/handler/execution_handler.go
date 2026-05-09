package handler

import (
	"context"
	"encoding/base64"
	"io"
	"log"
	"net/http"
	"time"

	"evalux-server/internal/model"
	"evalux-server/internal/response"
	"evalux-server/internal/service"

	"github.com/gin-gonic/gin"
)

type ExecutionHandler struct {
	execService *service.ExecutionService
}

func NewExecutionHandler(execService *service.ExecutionService) *ExecutionHandler {
	return &ExecutionHandler{execService: execService}
}

// StartSession POST /api/executions/start
func (h *ExecutionHandler) StartSession(c *gin.Context) {
	operatorID := getOperatorID(c)
	var req model.StartExecutionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "参数校验失败: "+err.Error())
		return
	}
	session, err := h.execService.StartSession(c.Request.Context(), operatorID, req)
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	response.OKMsg(c, "评估会话已创建", session)
}

// UploadStepScreenshot POST /api/executions/:id/screenshot
// 在截图后立即上传，与 ReportStep 解耦；返回存储路径供后续 ReportStep 引用
func (h *ExecutionHandler) UploadStepScreenshot(c *gin.Context) {
	sessionID := c.Param("id")
	if sessionID == "" {
		response.BadRequest(c, "session_id 不能为空")
		return
	}
	file, header, err := c.Request.FormFile("screenshot")
	if err != nil || file == nil {
		response.BadRequest(c, "未收到截图文件: "+err.Error())
		return
	}
	defer file.Close()
	data, readErr := io.ReadAll(file)
	if readErr != nil {
		response.BadRequest(c, "读取截图文件失败: "+readErr.Error())
		return
	}
	url, uploadErr := h.execService.UploadScreenshot(
		c.Request.Context(), sessionID, header.Filename, header.Header.Get("Content-Type"), data,
	)
	if uploadErr != nil {
		response.BadRequest(c, "截图上传失败: "+uploadErr.Error())
		return
	}
	// 同时返回 base64 供多模态模式下 AI 直接使用（避免二次读取存储）
	b64 := base64.StdEncoding.EncodeToString(data)
	response.OK(c, gin.H{"screenshot_url": url, "screenshot_base64": b64})
}

// ReportStep POST /api/executions/report-step
// 上报步骤执行结果，后端调用模型返回下一步指令
// 截图已在前一步独立上传，此处通过 screenshot_url 字段引用
func (h *ExecutionHandler) ReportStep(c *gin.Context) {
	sessionID := c.PostForm("session_id")
	if sessionID == "" {
		response.BadRequest(c, "session_id 不能为空")
		return
	}

	// 解析步骤信息
	var req model.ReportStepRequest
	req.SessionID = sessionID
	req.StepNo = parseInt(c.PostForm("step_no"))
	req.ScreenDesc = c.PostForm("screen_desc")
	req.ActionType = c.PostForm("action_type")
	req.ErrorMsg = c.PostForm("error_message")
	req.RetryCount = parseInt(c.PostForm("retry_count"))

	modelChannel := c.PostForm("model_channel")
	modelType := c.PostForm("model_type")
	// 截图存储路径（由前一步独立上传后传入）和多模态 base64
	screenshotURL := c.PostForm("screenshot_url")
	screenshotBase64 := c.PostForm("screenshot_base64")

	// 为 AI 推理设置独立超时（14 分钟），略短于 server WriteTimeout(15min)
	// 使用 ctx 而非 httpClient.Timeout，对流式响应友好
	ctx, cancel := context.WithTimeout(c.Request.Context(), 14*time.Minute)
	defer cancel()

	result, err := h.execService.ReportStep(ctx, req, screenshotURL, modelChannel, modelType, screenshotBase64)
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	response.OK(c, result)
}

// FinishSession POST /api/executions/finish
func (h *ExecutionHandler) FinishSession(c *gin.Context) {
	operatorID := getOperatorID(c)
	var req model.FinishSessionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "参数校验失败: "+err.Error())
		return
	}
	if err := h.execService.FinishSession(c.Request.Context(), operatorID, req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	response.OKMsg(c, "会话已结束", nil)
}

// GetSession GET /api/executions/:id
func (h *ExecutionHandler) GetSession(c *gin.Context) {
	operatorID := getOperatorID(c)
	session, err := h.execService.GetSession(c.Request.Context(), operatorID, c.Param("id"))
	if err != nil {
		response.NotFound(c, err.Error())
		return
	}
	response.OK(c, session)
}

// ListSessions GET /api/projects/:id/executions
func (h *ExecutionHandler) ListSessions(c *gin.Context) {
	operatorID := getOperatorID(c)
	projectID := c.Param("id")
	var query model.SessionListQuery
	if err := c.ShouldBindQuery(&query); err != nil {
		response.BadRequest(c, "参数校验失败: "+err.Error())
		return
	}
	result, err := h.execService.ListSessions(c.Request.Context(), operatorID, projectID, query)
	if err != nil {
		response.ServerError(c, err.Error())
		return
	}
	response.OK(c, result)
}

// ListSteps GET /api/executions/:id/steps
func (h *ExecutionHandler) ListSteps(c *gin.Context) {
	operatorID := getOperatorID(c)
	steps, err := h.execService.ListSteps(c.Request.Context(), operatorID, c.Param("id"))
	if err != nil {
		response.ServerError(c, err.Error())
		return
	}
	response.OK(c, steps)
}

// UploadRecording POST /api/executions/:id/recording
func (h *ExecutionHandler) UploadRecording(c *gin.Context) {
	sessionID := c.Param("id")
	file, header, err := c.Request.FormFile("recording")
	if err != nil {
		response.BadRequest(c, "请上传录屏文件")
		return
	}
	defer file.Close()
	data, _ := io.ReadAll(file)

	// 兜底 Content-Type：录屏文件一定是 video/mp4
	contentType := header.Header.Get("Content-Type")
	if contentType == "" || contentType == "application/octet-stream" {
		contentType = "video/mp4"
	}
	log.Printf("[UploadRecording] sessionID=%s, filename=%s, contentType=%s, dataSize=%d",
		sessionID, header.Filename, contentType, len(data))

	url, err := h.execService.UploadRecording(
		c.Request.Context(), sessionID, header.Filename, contentType, data,
	)
	if err != nil {
		response.ServerError(c, err.Error())
		return
	}
	response.OKMsg(c, "录屏上传成功", gin.H{"url": url})
}

func parseInt(s string) int {
	n := 0
	for _, c := range s {
		if c >= '0' && c <= '9' {
			n = n*10 + int(c-'0')
		}
	}
	return n
}

// GetFileURL POST /api/files/url - 获取文件临时访问URL（POST 方式，兼容保留）
func (h *ExecutionHandler) GetFileURL(c *gin.Context) {
	var req struct {
		StoragePath string `json:"storage_path" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "参数校验失败: "+err.Error())
		return
	}
	url, err := h.execService.GetFileURL(c.Request.Context(), req.StoragePath)
	if err != nil {
		response.ServerError(c, "获取文件访问链接失败: "+err.Error())
		return
	}
	response.OK(c, gin.H{"url": url})
}

// GetFileURLQuery GET /api/files/url?path=cloudreve://my/... - 获取文件临时访问URL（GET 方式）
// 前端直接用返回的临时 URL 请求 Cloudreve 下载，无需后端中转文件流
func (h *ExecutionHandler) GetFileURLQuery(c *gin.Context) {
	storagePath := c.Query("path")
	if storagePath == "" {
		response.BadRequest(c, "path 参数不能为空")
		return
	}
	url, err := h.execService.GetFileURL(c.Request.Context(), storagePath)
	if err != nil {
		response.ServerError(c, "获取文件访问链接失败: "+err.Error())
		return
	}
	response.OK(c, gin.H{"url": url})
}

// ProxyFile GET /api/files/proxy - 代理转发 Cloudreve 文件流给前端
// 支持 Range 请求，使 HTML5 <video> 标签能正常播放 MP4 视频
// 由于 Cloudreve 监听在 127.0.0.1，前端无法直接访问临时链接，需要后端代理
func (h *ExecutionHandler) ProxyFile(c *gin.Context) {
	storagePath := c.Query("path")
	if storagePath == "" {
		response.BadRequest(c, "path 参数不能为空")
		return
	}

	// 获取 Cloudreve 临时下载链接
	downloadURL, err := h.execService.GetFileURL(c.Request.Context(), storagePath)
	if err != nil {
		response.ServerError(c, "获取文件下载链接失败: "+err.Error())
		return
	}

	// 代理请求 Cloudreve 获取文件流
	proxyReq, err := http.NewRequestWithContext(c.Request.Context(), "GET", downloadURL, nil)
	if err != nil {
		response.ServerError(c, "创建代理请求失败")
		return
	}

	// 透传 Range 请求头（支持视频 seek 和 MP4 moov atom 读取）
	if rangeHeader := c.GetHeader("Range"); rangeHeader != "" {
		proxyReq.Header.Set("Range", rangeHeader)
	}

	proxyResp, err := http.DefaultClient.Do(proxyReq)
	if err != nil {
		response.ServerError(c, "代理请求文件失败: "+err.Error())
		return
	}
	defer proxyResp.Body.Close()

	// 转发关键响应头
	contentType := proxyResp.Header.Get("Content-Type")
	if contentType != "" {
		c.Header("Content-Type", contentType)
	}
	contentLength := proxyResp.Header.Get("Content-Length")
	if contentLength != "" {
		c.Header("Content-Length", contentLength)
	}
	contentDisposition := proxyResp.Header.Get("Content-Disposition")
	if contentDisposition != "" {
		c.Header("Content-Disposition", contentDisposition)
	}
	// 透传 Range 相关响应头（视频播放必需）
	if cr := proxyResp.Header.Get("Content-Range"); cr != "" {
		c.Header("Content-Range", cr)
	}
	if ar := proxyResp.Header.Get("Accept-Ranges"); ar != "" {
		c.Header("Accept-Ranges", ar)
	} else {
		c.Header("Accept-Ranges", "bytes")
	}

	c.Status(proxyResp.StatusCode)
	io.Copy(c.Writer, proxyResp.Body)
}
