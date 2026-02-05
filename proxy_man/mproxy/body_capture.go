package mproxy

import (
	"context"
	"io"
	"proxy_man/myminio"
	"strings"
	"time"
)

// BodyCapture Body 捕获状态
type BodyCapture struct {
	ObjectKey   string // MinIO 对象 Key
	Size        int64  // 上传后的实际大小
	Uploaded    bool   // 是否成功上传
	ContentType string // 内容类型
	Error       error  // 上传过程中的错误
}

// bodyCaptReader 流式上传包装器
type bodyCaptReader struct {
	inner       io.ReadCloser  // 内层 Reader（通常是流量统计层）
	pipeWriter  *io.PipeWriter // 用于向上传协程传输数据
	capture     *BodyCapture   // 捕获状态
	doneCh      chan struct{}  // 上传完成信号
	skipUpload bool           // 是否跳过捕获
}

// shouldSkipCapture 判断是否应该跳过捕获
func shouldSkipCapture(contentType string) bool {
	skipTypes := []string{
		"text/event-stream", // SSE
		"websocket",         // WebSocket
		"multipart/x-mixed-replace", // 流式响应
	}
	lowerType := strings.ToLower(contentType)
	for _, t := range skipTypes {
		if strings.Contains(lowerType, t) {
			return true
		}
	}
	return false
}

// WrapBodyForCapture 包装 Body 进行 MinIO 捕获
// 参数:
//   - inner: 内层 io.ReadCloser（通常是流量统计层）
//   - sessionID: 会话 ID
//   - bodyType: "req" 或 "resp"
//   - contentType: Content-Type 头
// 返回:
//   - *bodyCaptReader: 包装后的 Reader
//   - *BodyCapture: 捕获状态（nil 表示跳过捕获）
func WrapBodyForCapture(inner io.ReadCloser, sessionID int64, bodyType, contentType string) (*bodyCaptReader, *BodyCapture) {
	// 如果 MinIO 未启用或内容类型需要跳过，直接返回透传 Reader
	if !myminio.IsEnabled() || shouldSkipCapture(contentType) {
		return &bodyCaptReader{inner: inner, skipUpload: true}, nil
	}

	// 初始化捕获状态
	capture := &BodyCapture{
		ObjectKey:   myminio.GetObjectKey(sessionID, bodyType),
		ContentType: contentType,
	}

	// 创建 Pipe 用于数据流转
	pr, pw := io.Pipe()

	reader := &bodyCaptReader{
		inner:      inner,
		pipeWriter: pw,
		capture:    capture,
		doneCh:     make(chan struct{}),
	}

	// 启动上传协程
	go reader.uploadToMinIO(pr)

	return reader, capture
}

// uploadToMinIO 上传到 MinIO（在独立协程中运行）
func (r *bodyCaptReader) uploadToMinIO(pr *io.PipeReader) {
	defer close(r.doneCh)
	defer pr.Close()

	// 设置上传超时（30分钟）
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()

	// 执行上传
	info, err := myminio.GlobalClient.PutObject(ctx, r.capture.ObjectKey, pr, r.capture.ContentType)
	if err != nil {
		r.capture.Error = err
		return
	}

	// 记录上传成功信息
	r.capture.Size = info.Size
	r.capture.Uploaded = true
}

// Read 实现 io.Reader 接口
func (r *bodyCaptReader) Read(p []byte) (n int, err error) {
	// 从内层 Reader 读取数据
	n, err = r.inner.Read(p)

	// 如果没有跳过捕获且读取到数据，写入 Pipe
	if !r.skipUpload && n > 0 && r.pipeWriter != nil {
		if _, writeErr := r.pipeWriter.Write(p[:n]); writeErr != nil {
			// 记录写入错误但不影响主流程
			r.capture.Error = writeErr
		}
	}

	return n, err
}

// Close 实现 io.Closer 接口
func (r *bodyCaptReader) Close() error {
	// 关闭 PipeWriter，通知上传协程数据已结束
	if r.pipeWriter != nil {
		r.pipeWriter.Close()
	}

	// 等待上传完成
	if r.doneCh != nil {
		<-r.doneCh
	}

	// 关闭内层 Reader
	return r.inner.Close()
}
