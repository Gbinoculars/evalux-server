package storage

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"path"
	"sync"
	"time"

	"evalux-server/internal/config"

	"github.com/google/uuid"
)

// CloudreveStorage Cloudreve 对象存储服务
type CloudreveStorage struct {
	baseURL  string
	user     string
	password string

	mu           sync.Mutex
	accessToken  string
	refreshToken string
	tokenExpiry  time.Time
	httpClient   *http.Client
}

// NewCloudreveStorage 创建 Cloudreve 存储实例并验证连接
func NewCloudreveStorage(cfg *config.Config) (*CloudreveStorage, error) {
	cs := &CloudreveStorage{
		baseURL:  cfg.CloudreveBaseURL,
		user:     cfg.CloudreveUser,
		password: cfg.CloudrevePassword,
		httpClient: &http.Client{
			// 不设置全局 Timeout，大文件分片上传可能耗时较长
			// 各请求通过 context 控制超时
		},
	}

	// 登录获取 token
	if err := cs.login(); err != nil {
		return nil, fmt.Errorf("Cloudreve 登录失败: %w", err)
	}
	log.Println("Cloudreve 对象存储登录成功")

	// 确保目录存在
	for _, dir := range []string{"/evalux/screenshots", "/evalux/recordings"} {
		_ = cs.ensureDirectory(dir)
	}

	return cs, nil
}

// =========== ObjectStorage 接口实现 ===========

// UploadFile 上传文件到 Cloudreve，返回 cloudreve URI 路径
// 支持任意大小文件：按 Cloudreve 返回的 ChunkSize 进行分片上传
func (cs *CloudreveStorage) UploadFile(ctx context.Context, folder, filename string, reader io.Reader, size int64, contentType string) (string, error) {
	// 读取全部数据
	data, err := io.ReadAll(reader)
	if err != nil {
		return "", fmt.Errorf("读取文件数据失败: %w", err)
	}
	if size <= 0 {
		size = int64(len(data))
	}

	// 构造存储路径: /evalux/{folder}/{date}/{uuid}{ext}
	ext := path.Ext(filename)
	datePath := time.Now().Format("2006/01/02")
	objectName := fmt.Sprintf("%s%s", uuid.New().String(), ext)
	storagePath := fmt.Sprintf("/evalux/%s/%s/%s", folder, datePath, objectName)

	// URL 编码路径
	encodedPath := encodeCloudrevePath(storagePath)
	uri := "cloudreve://my" + encodedPath

	// 确保日期目录存在
	dirPath := fmt.Sprintf("/evalux/%s/%s", folder, datePath)
	_ = cs.ensureDirectory(dirPath)

	// 步骤1: 创建上传会话
	sessionResp, err := cs.createUploadSession(uri, size, contentType)
	if err != nil {
		return "", fmt.Errorf("创建上传会话失败: %w", err)
	}

	// 步骤2: 按 ChunkSize 分片上传文件内容
	// Cloudreve V4 本机存储：最后一个分片上传完成后服务端自动合并
	chunkSize := sessionResp.ChunkSize
	if chunkSize <= 0 || chunkSize >= int64(len(data)) {
		// 文件小于等于一个分片大小，直接一次性上传（chunk index = 0）
		log.Printf("[Cloudreve] 文件 %d 字节 <= chunkSize %d，单次上传", len(data), chunkSize)
		if err := cs.uploadChunk(sessionResp.SessionID, 0, data); err != nil {
			return "", fmt.Errorf("上传文件失败: %w", err)
		}
	} else {
		// 需要分片上传
		totalSize := int64(len(data))
		chunkIndex := 0
		for offset := int64(0); offset < totalSize; offset += chunkSize {
			end := offset + chunkSize
			if end > totalSize {
				end = totalSize
			}
			chunk := data[offset:end]
			log.Printf("[Cloudreve] 上传分片 %d: offset=%d, size=%d, total=%d", chunkIndex, offset, len(chunk), totalSize)
			if err := cs.uploadChunk(sessionResp.SessionID, chunkIndex, chunk); err != nil {
				return "", fmt.Errorf("上传分片 %d 失败: %w", chunkIndex, err)
			}
			chunkIndex++
		}
		log.Printf("[Cloudreve] 分片上传完成: 共 %d 片", chunkIndex)
	}

	log.Printf("[Cloudreve] 文件上传完成: %s, 大小=%d", uri, len(data))

	// 返回 cloudreve URI 作为存储标识
	return uri, nil
}

// DeleteFile 从 Cloudreve 删除文件
func (cs *CloudreveStorage) DeleteFile(ctx context.Context, storagePath string) error {
	// Cloudreve 使用 URI 删除文件: DELETE /api/v4/object
	reqBody := map[string]interface{}{
		"uris": []string{storagePath},
	}
	bodyBytes, _ := json.Marshal(reqBody)

	req, err := http.NewRequestWithContext(ctx, "DELETE", cs.baseURL+"/api/v4/object", bytes.NewReader(bodyBytes))
	if err != nil {
		return err
	}
	cs.setAuthHeader(req)
	req.Header.Set("Content-Type", "application/json")

	resp, err := cs.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("Cloudreve 删除请求失败: %w", err)
	}
	defer resp.Body.Close()
	return nil
}

// GetFileURL 获取文件的临时匿名下载 URL
func (cs *CloudreveStorage) GetFileURL(ctx context.Context, storagePath string) (string, error) {
	reqBody := map[string]interface{}{
		"uris":    []string{storagePath},
		"archive": false,
	}
	bodyBytes, _ := json.Marshal(reqBody)

	req, err := http.NewRequestWithContext(ctx, "POST", cs.baseURL+"/api/v4/file/url", bytes.NewReader(bodyBytes))
	if err != nil {
		return "", err
	}
	cs.setAuthHeader(req)
	req.Header.Set("Content-Type", "application/json")

	resp, err := cs.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("Cloudreve 获取下载链接失败: %w", err)
	}
	defer resp.Body.Close()

	var result struct {
		Code int    `json:"code"`
		Msg  string `json:"msg"`
		Data struct {
			URLs []struct {
				URL string `json:"url"`
			} `json:"urls"`
			Expires string `json:"expires"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("解析下载链接响应失败: %w", err)
	}
	if result.Code != 0 {
		return "", fmt.Errorf("Cloudreve 获取下载链接错误(%d): %s", result.Code, result.Msg)
	}
	if len(result.Data.URLs) == 0 {
		return "", fmt.Errorf("Cloudreve 未返回下载链接")
	}

	return result.Data.URLs[0].URL, nil
}

// =========== 内部方法 ===========

// login 登录 Cloudreve 获取 JWT token
func (cs *CloudreveStorage) login() error {
	// 步骤1: 准备登录请求（V4 用 email 字段）
	loginBody := map[string]string{
		"email":    cs.user,
		"password": cs.password,
	}
	bodyBytes, _ := json.Marshal(loginBody)

	req, err := http.NewRequest("POST", cs.baseURL+"/api/v4/session/token", bytes.NewReader(bodyBytes))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := cs.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("登录请求失败: %w", err)
	}
	defer resp.Body.Close()

	// 先用 RawMessage 安全解析，因为错误时 data 可能不是对象
	var rawResult struct {
		Code int             `json:"code"`
		Msg  string          `json:"msg"`
		Data json.RawMessage `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&rawResult); err != nil {
		return fmt.Errorf("解析登录响应失败: %w", err)
	}
	if rawResult.Code != 0 {
		return fmt.Errorf("登录错误(%d): %s", rawResult.Code, rawResult.Msg)
	}

	// 成功时解析 data
	var loginData struct {
		User  interface{} `json:"user"`
		Token struct {
			AccessToken    string `json:"access_token"`
			RefreshToken   string `json:"refresh_token"`
			AccessExpires  string `json:"access_expires"`
			RefreshExpires string `json:"refresh_expires"`
		} `json:"token"`
	}
	if err := json.Unmarshal(rawResult.Data, &loginData); err != nil {
		return fmt.Errorf("解析登录数据失败: %w", err)
	}

	cs.mu.Lock()
	defer cs.mu.Unlock()
	cs.accessToken = loginData.Token.AccessToken
	cs.refreshToken = loginData.Token.RefreshToken

	// 解析过期时间
	if t, err := time.Parse(time.RFC3339Nano, loginData.Token.AccessExpires); err == nil {
		cs.tokenExpiry = t
	} else {
		// 默认1小时后过期
		cs.tokenExpiry = time.Now().Add(55 * time.Minute)
	}

	return nil
}

// refreshAccessToken 使用 refresh_token 刷新 access_token
func (cs *CloudreveStorage) refreshAccessToken() error {
	cs.mu.Lock()
	rt := cs.refreshToken
	cs.mu.Unlock()

	reqBody := map[string]string{
		"refresh_token": rt,
	}
	bodyBytes, _ := json.Marshal(reqBody)

	req, err := http.NewRequest("POST", cs.baseURL+"/api/v4/session/token/refresh", bytes.NewReader(bodyBytes))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := cs.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("刷新 token 请求失败: %w", err)
	}
	defer resp.Body.Close()

	var rawResult struct {
		Code int             `json:"code"`
		Msg  string          `json:"msg"`
		Data json.RawMessage `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&rawResult); err != nil {
		return fmt.Errorf("解析刷新响应失败: %w", err)
	}
	if rawResult.Code != 0 {
		// 刷新失败，尝试重新登录
		return cs.login()
	}

	var refreshData struct {
		AccessToken    string `json:"access_token"`
		RefreshToken   string `json:"refresh_token"`
		AccessExpires  string `json:"access_expires"`
		RefreshExpires string `json:"refresh_expires"`
	}
	if err := json.Unmarshal(rawResult.Data, &refreshData); err != nil {
		return cs.login()
	}

	cs.mu.Lock()
	defer cs.mu.Unlock()
	cs.accessToken = refreshData.AccessToken
	cs.refreshToken = refreshData.RefreshToken
	if t, err := time.Parse(time.RFC3339Nano, refreshData.AccessExpires); err == nil {
		cs.tokenExpiry = t
	} else {
		cs.tokenExpiry = time.Now().Add(55 * time.Minute)
	}
	return nil
}

// ensureToken 确保 token 有效，过期则刷新
func (cs *CloudreveStorage) ensureToken() error {
	cs.mu.Lock()
	expired := time.Now().After(cs.tokenExpiry.Add(-2 * time.Minute))
	cs.mu.Unlock()

	if expired {
		return cs.refreshAccessToken()
	}
	return nil
}

// setAuthHeader 设置请求的 Authorization 头
func (cs *CloudreveStorage) setAuthHeader(req *http.Request) {
	_ = cs.ensureToken()
	cs.mu.Lock()
	token := cs.accessToken
	cs.mu.Unlock()
	req.Header.Set("Authorization", "Bearer "+token)
}

// ensureDirectory 确保 Cloudreve 中目录存在
func (cs *CloudreveStorage) ensureDirectory(dirPath string) error {
	encodedPath := encodeCloudrevePath(dirPath)
	uri := "cloudreve://my" + encodedPath

	reqBody := map[string]interface{}{
		"uris": []string{uri},
	}
	bodyBytes, _ := json.Marshal(reqBody)

	req, err := http.NewRequest("PUT", cs.baseURL+"/api/v4/directory", bytes.NewReader(bodyBytes))
	if err != nil {
		return err
	}
	cs.setAuthHeader(req)
	req.Header.Set("Content-Type", "application/json")

	resp, err := cs.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return nil
}

// uploadSessionResponse 上传会话响应
type uploadSessionResponse struct {
	SessionID string `json:"session_id"`
	ChunkSize int64  `json:"chunk_size"`
	Expires   int64  `json:"expires"`
}

// createUploadSession 创建上传会话
func (cs *CloudreveStorage) createUploadSession(uri string, size int64, mimeType string) (*uploadSessionResponse, error) {
	reqBody := map[string]interface{}{
		"uri":       uri,
		"size":      size,
		"mime_type": mimeType,
	}
	bodyBytes, _ := json.Marshal(reqBody)
	log.Printf("[Cloudreve] 创建上传会话: uri=%s, size=%d, mime_type=%s", uri, size, mimeType)

	req, err := http.NewRequest("PUT", cs.baseURL+"/api/v4/file/upload", bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, err
	}
	cs.setAuthHeader(req)
	req.Header.Set("Content-Type", "application/json")

	resp, err := cs.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("创建上传会话请求失败: %w", err)
	}
	defer resp.Body.Close()

	// 先读取完整响应体用于日志
	respBody, _ := io.ReadAll(resp.Body)
	log.Printf("[Cloudreve] 创建上传会话响应: HTTP %d, body=%s", resp.StatusCode, string(respBody))

	var result struct {
		Code int                   `json:"code"`
		Msg  string                `json:"msg"`
		Data uploadSessionResponse `json:"data"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("解析上传会话响应失败: %w (body=%s)", err, string(respBody))
	}
	if result.Code != 0 {
		return nil, fmt.Errorf("创建上传会话错误(%d): %s", result.Code, result.Msg)
	}
	log.Printf("[Cloudreve] 上传会话已创建: session_id=%s, chunk_size=%d", result.Data.SessionID, result.Data.ChunkSize)
	if result.Data.SessionID == "" {
		return nil, fmt.Errorf("Cloudreve 返回的 session_id 为空，完整响应: %s", string(respBody))
	}
	return &result.Data, nil
}

// uploadChunk 上传文件分片（Cloudreve V4 本机存储使用 POST）
func (cs *CloudreveStorage) uploadChunk(sessionID string, index int, data []byte) error {
	uploadURL := fmt.Sprintf("%s/api/v4/file/upload/%s/%d", cs.baseURL, sessionID, index)
	log.Printf("[Cloudreve] 上传分片请求: POST %s, Content-Length=%d", uploadURL, len(data))

	req, err := http.NewRequest("POST", uploadURL, bytes.NewReader(data))
	if err != nil {
		return err
	}
	cs.setAuthHeader(req)
	req.Header.Set("Content-Type", "application/octet-stream")
	req.Header.Set("Content-Length", fmt.Sprintf("%d", len(data)))

	resp, err := cs.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("上传分片请求失败: %w", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	log.Printf("[Cloudreve] 上传分片 %d 响应: HTTP %d, body=%s", index, resp.StatusCode, string(respBody))

	var result struct {
		Code int    `json:"code"`
		Msg  string `json:"msg"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return fmt.Errorf("解析上传分片响应失败: %w (body=%s)", err, string(respBody))
	}
	if result.Code != 0 {
		return fmt.Errorf("上传分片错误(%d): %s", result.Code, result.Msg)
	}
	return nil
}

// encodeCloudrevePath 对路径中的各段进行 URL 编码
func encodeCloudrevePath(p string) string {
	// 分割路径，对每一段进行编码
	parts := splitPath(p)
	encoded := ""
	for _, part := range parts {
		encoded += "/" + url.PathEscape(part)
	}
	return encoded
}

// splitPath 将路径分割为各段（去除空段）
func splitPath(p string) []string {
	var parts []string
	for _, s := range splitBySlash(p) {
		if s != "" {
			parts = append(parts, s)
		}
	}
	return parts
}

func splitBySlash(s string) []string {
	result := []string{}
	current := ""
	for _, c := range s {
		if c == '/' {
			result = append(result, current)
			current = ""
		} else {
			current += string(c)
		}
	}
	result = append(result, current)
	return result
}
