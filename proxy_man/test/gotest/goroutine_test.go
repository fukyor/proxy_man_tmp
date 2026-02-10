package main_test

import (
	"context"
	"io"
	"net/http"
	"proxy_man/myminio"
	"strings"
	"testing"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"github.com/stretchr/testify/assert"
	"go.uber.org/goleak"
)

// TestMain 用于在所有测试结束后验证 Main 级别的泄漏（可选）
func TestMain(m *testing.M) {
	goleak.VerifyTestMain(m)
}


// 专门测试 BuildBodyReader 是否正确释放了协程
func TestBodyReader_Leak(t *testing.T) {
	// 初始化 MinIO
	endpoint := "127.0.0.1:9000"
	accessKeyID := "root"
	secretAccessKey := "12345678"
	useSSL := false
    bucketName := "bodydata"
	tr := &http.Transport{}

	client, err := minio.New(endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(accessKeyID, secretAccessKey, ""),
		Secure: useSSL,
		Transport: tr, // <--- 关键点：在这里注入
	})
	// 检查 Bucket 是否存在（可选，确保服务是通的）
    exists, err := client.BucketExists(context.Background(), bucketName)
    if err != nil || !exists {
        // 如果连不上 MinIO，测试也没法跑，可以选择跳过或报错
        t.Logf("警告: 无法连接 MinIO (%v)，测试可能不准确", err)
    }
	myminio.GlobalClient = &myminio.Client{
		Client: client,
	}

	defer goleak.VerifyNone(t)
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Minute)
    defer cancel()
	defer tr.CloseIdleConnections() // <--- 测试结束强制关闭空闲连接
	// 2. 构造数据
	
	fakeBody := io.NopCloser(strings.NewReader("test data"))
	
	// 3. 调用被测函数（这会启动一个 goroutine）
	reader := myminio.BuildBodyReader(fakeBody, 123, "req", "text/plain", 9)

	rn, err := io.Copy(io.Discard, reader)
	if err != nil {
		t.Fatal("❌ iocopy读取失败")
	}

	// 5. 关键：调用 Close。如果 Close 实现有问题，uploadToMinIO 协程就会挂起
	done := make(chan struct{})
	go func() {
        // 这里的 Close 如果卡死，只会卡死这个协程，不会卡死主测试线程
        reader.Close()
        close(done)
    }()

    select {
    case <-done:
        // A面：Close 在 1 分钟内成功返回
        // 在这里进行正常的 Assert 验证，例如验证 Size 或 Error
        assert.Equal(t, rn, reader.Capture.Size)
        
    case <-ctx.Done():
        // B面：超时时间到了，Close 还没返回
        // 直接判定测试失败，打印堆栈信息，强制结束当前测试用例
        t.Fatal("❌ 测试超时：reader.Close() 发生死锁或耗时过长，已强制终止")
    }
    
    // 函数返回时，defer goleak.VerifyNone 会自动执行检查
}
