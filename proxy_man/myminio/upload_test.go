package myminio

import (
	"crypto/rand"
    "io"
    "testing"
    "github.com/stretchr/testify/assert"
	"bytes"
)

func TestBodyCapture_Streaming(t *testing.T) {
    // 模拟请求体
    body := io.NopCloser(bytes.NewReader([]byte("test body data")))

    // 创建流式上传 Reader
    reader := BuildBodyReader(body, 1, "req", "text/plain")

    // 读取数据（会同时触发上传）
    data := make([]byte, 100)
    n, _ := reader.Read(data)

    assert.Equal(t, 14, n)  // "test body data" 长度

    // 关闭 Reader（等待上传完成）
    reader.Close()

    // 验证上传结果
    size := reader.Capture.Size
    assert.NoError(t, reader.Capture.Error)
    assert.Equal(t, int64(14), size)
}

func TestBodyCapture_EmptyBody(t *testing.T) {
    // 测试空 Body
    body := io.NopCloser(bytes.NewReader([]byte{}))
       reader := BuildBodyReader(body, 2, "req", "text/plain")

    reader.Close()

    size := reader.Capture.Size
    assert.NoError(t, reader.Capture.Error)
    assert.Equal(t, int64(0), size)
}

func TestBodyCapture_LargeFile(t *testing.T) {
    // 测试大文件流式上传（验证内存占用稳定）
    const streamSize = 100 * 1024 * 1024 // 100MB
    body := io.NopCloser(io.LimitReader(rand.Reader, streamSize))  
    reader := BuildBodyReader(body, 3, "resp", "application/octet-stream")

    _, err := io.Copy(io.Discard, reader)
    assert.NoError(t, err, "Copy should complete without error")
    assert.NoError(t, reader.Capture.Error, "上传错误")
    reader.Close()
    size := reader.Capture.Size

    assert.Equal(t, int64(100*1024*1024), size)
}

