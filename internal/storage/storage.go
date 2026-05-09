package storage

import (
	"context"
	"io"
)

// ObjectStorage 对象存储抽象接口
type ObjectStorage interface {
	// UploadFile 上传文件，返回存储路径标识（如 cloudreve URI）
	UploadFile(ctx context.Context, folder, filename string, reader io.Reader, size int64, contentType string) (storagePath string, err error)
	// DeleteFile 删除文件
	DeleteFile(ctx context.Context, storagePath string) error
	// GetFileURL 根据存储路径获取临时可访问的 URL
	GetFileURL(ctx context.Context, storagePath string) (url string, err error)
}
