package service

import (
	"context"
	"fmt"
	"os/exec"
	"runtime"
	"strings"
	"time"
)

// DeviceInfo 单个 Android 设备信息
type DeviceInfo struct {
	Serial      string `json:"serial"`
	Status      string `json:"status"`
	Model       string `json:"model"`
	Product     string `json:"product"`
	DeviceName  string `json:"device_name"`
	TransportID string `json:"transport_id"`
}

// DeviceListResult 设备检测结果
type DeviceListResult struct {
	Success   bool         `json:"success"`
	Message   string       `json:"message"`
	ScannedAt string       `json:"scanned_at"`
	AdbPath   string       `json:"adb_path"`
	Devices   []DeviceInfo `json:"devices"`
}

type DeviceService struct{}

func NewDeviceService() *DeviceService {
	return &DeviceService{}
}

// ListDevices 通过 adb devices -l 获取当前连接的 Android 设备
func (s *DeviceService) ListDevices(ctx context.Context) (*DeviceListResult, error) {
	scannedAt := time.Now().Format(time.RFC3339)

	adbPath := resolveAdb()

	// 设置超时 10 秒
	cmdCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	cmd := exec.CommandContext(cmdCtx, adbPath, "devices", "-l")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return &DeviceListResult{
			Success:   false,
			Message:   normalizeAdbError(err, string(output)),
			ScannedAt: scannedAt,
			AdbPath:   adbPath,
			Devices:   []DeviceInfo{},
		}, nil
	}

	devices := parseAdbDevices(string(output))
	return &DeviceListResult{
		Success:   true,
		Message:   "",
		ScannedAt: scannedAt,
		AdbPath:   adbPath,
		Devices:   devices,
	}, nil
}

// resolveAdb 查找 adb 可执行文件
func resolveAdb() string {
	if runtime.GOOS == "windows" {
		return "adb.exe"
	}
	return "adb"
}

// parseAdbDevices 解析 adb devices -l 的输出
func parseAdbDevices(stdout string) []DeviceInfo {
	var devices []DeviceInfo
	lines := strings.Split(stdout, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "List of devices attached") || strings.HasPrefix(line, "* daemon") {
			continue
		}
		parts := strings.Fields(line)
		if len(parts) < 2 {
			continue
		}

		serial := parts[0]
		status := parts[1]
		metadata := make(map[string]string)
		for _, seg := range parts[2:] {
			idx := strings.Index(seg, ":")
			if idx > 0 {
				metadata[seg[:idx]] = seg[idx+1:]
			}
		}

		devices = append(devices, DeviceInfo{
			Serial:      serial,
			Status:      status,
			Model:       normalizeLabel(metadata["model"]),
			Product:     normalizeLabel(metadata["product"]),
			DeviceName:  normalizeLabel(metadata["device"]),
			TransportID: metadata["transport_id"],
		})
	}
	return devices
}

func normalizeLabel(s string) string {
	return strings.TrimSpace(strings.ReplaceAll(s, "_", " "))
}

func normalizeAdbError(err error, output string) string {
	msg := err.Error()
	if strings.Contains(msg, "not found") || strings.Contains(msg, "ENOENT") || strings.Contains(msg, "executable file not found") {
		return "未找到可用的 adb，请安装 Android platform-tools 并确保 adb 在 PATH 中。"
	}
	if output != "" {
		firstLine := strings.SplitN(output, "\n", 2)[0]
		return fmt.Sprintf("读取连接设备失败：%s", strings.TrimSpace(firstLine))
	}
	return fmt.Sprintf("读取连接设备失败：%s", msg)
}
