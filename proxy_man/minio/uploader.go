package minio

import (
	"context"
	"io"
	"time"

	"github.com/minio/minio-go/v7"
)

// PutObject 上传对象到 MinIO
func (c *Client) PutObject(ctx context.Context, key string, reader io.Reader, contentType string) (minio.UploadInfo, error) {
	opts := minio.PutObjectOptions{
		ContentType: contentType,
	}
	return c.client.PutObject(ctx, c.config.Bucket, key, reader, -1, opts)
}

// StatObject 获取对象信息
func (c *Client) StatObject(key string) (minio.ObjectInfo, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	return c.client.StatObject(ctx, c.config.Bucket, key, minio.StatObjectOptions{})
}

// GetPresignedURL 生成预签名下载 URL
func (c *Client) GetPresignedURL(key string, expiry time.Duration) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	url, err := c.client.PresignedGetObject(ctx, c.config.Bucket, key, expiry, nil)
	if err != nil {
		return "", err
	}
	return url.String(), nil
}

// Bucket 返回当前使用的 Bucket 名称
func (c *Client) Bucket() string {
	return c.config.Bucket
}
