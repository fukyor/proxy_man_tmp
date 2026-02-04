package proxysocket

import (
	"encoding/json"
	"net/http"
	"proxy_man/minio"
	"strings"
	"time"
)

// APIResponse 通用 API 响应格式
type APIResponse struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data,omitempty"`
}

// DownloadData 下载信息
type DownloadData struct {
	DownloadURL string `json:"downloadUrl"` // 预签名下载链接
	ExpiresAt   string `json:"expiresAt"`   // 链接过期时间
	Filename    string `json:"filename"`    // 建议文件名
	Size        int64  `json:"size"`        // 文件大小（字节）
}

// HandleDownload 处理下载请求
// GET /api/storage/download?key=mitm-data/2026-02-04/10086/req
func HandleDownload(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	// 获取 ObjectKey 参数
	objectKey := r.URL.Query().Get("key")
	if objectKey == "" {
		json.NewEncoder(w).Encode(APIResponse{
			Code:    400,
			Message: "缺少参数: key",
		})
		return
	}

	// 检查 MinIO 是否启用
	if !minio.IsEnabled() {
		json.NewEncoder(w).Encode(APIResponse{
			Code:    503,
			Message: "MinIO 存储未启用",
		})
		return
	}

	// 检查对象是否存在
	info, err := minio.GlobalClient.StatObject(objectKey)
	if err != nil {
		json.NewEncoder(w).Encode(APIResponse{
			Code:    404,
			Message: "对象不存在",
		})
		return
	}

	// 生成预签名下载 URL（有效期 1 小时）
	expiry := 1 * time.Hour
	presignedURL, err := minio.GlobalClient.GetPresignedURL(objectKey, expiry)
	if err != nil {
		json.NewEncoder(w).Encode(APIResponse{
			Code:    500,
			Message: "生成下载链接失败",
		})
		return
	}

	// 提取文件名
	filename := extractFilename(objectKey)

	// 返回成功响应
	json.NewEncoder(w).Encode(APIResponse{
		Code:    0,
		Message: "success",
		Data: DownloadData{
			DownloadURL: presignedURL,
			ExpiresAt:   time.Now().Add(expiry).Format(time.RFC3339),
			Filename:    filename,
			Size:        info.Size,
		},
	})
}

// extractFilename 从 ObjectKey 提取文件名
// 格式: mitm-data/2026-02-04/10086/req -> 10086_req.bin
func extractFilename(key string) string {
	parts := strings.Split(key, "/")
	if len(parts) >= 2 {
		// 倒数第二个是 SessionID，倒数第一个是 bodyType
		sessionID := parts[len(parts)-2]
		bodyType := parts[len(parts)-1]
		return sessionID + "_" + bodyType + ".bin"
	}
	return "body.bin"
}
