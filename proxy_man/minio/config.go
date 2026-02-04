package minio

import (
	"context"
	"fmt"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

// Config MinIO 配置结构
type Config struct {
	Endpoint        string
	AccessKeyID     string
	SecretAccessKey string
	UseSSL          bool
	Bucket          string
	Enabled         bool
}

// Client MinIO 客户端封装
type Client struct {
	client *minio.Client
	config Config
}

// GlobalClient 全局 MinIO 客户端实例
var GlobalClient *Client

// NewClient 创建新的 MinIO 客户端
func NewClient(cfg Config) (*Client, error) {
	if !cfg.Enabled {
		return nil, nil
	}

	client, err := minio.New(cfg.Endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(cfg.AccessKeyID, cfg.SecretAccessKey, ""),
		Secure: cfg.UseSSL,
	})
	if err != nil {
		return nil, fmt.Errorf("创建 MinIO 客户端失败: %w", err)
	}

	// 检查 Bucket 是否存在
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	exists, err := client.BucketExists(ctx, cfg.Bucket)
	if err != nil {
		return nil, fmt.Errorf("检查 Bucket 失败: %w", err)
	}
	if !exists {
		return nil, fmt.Errorf("Bucket '%s' 不存在", cfg.Bucket)
	}

	return &Client{client: client, config: cfg}, nil
}

// IsEnabled 检查 MinIO 是否已启用
func IsEnabled() bool {
	return GlobalClient != nil && GlobalClient.config.Enabled
}

// GetObjectKey 生成对象存储的 Key
// 格式: mitm-data/YYYY-MM-DD/SessionID/bodyType
// 示例: mitm-data/2026-02-04/10086/req
func GetObjectKey(sessionID int64, bodyType string) string {
	date := time.Now().Format("2006-01-02")
	return fmt.Sprintf("mitm-data/%s/%d/%s", date, sessionID, bodyType)
}
