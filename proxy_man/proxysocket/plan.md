# MinIO 上传内存优化方案

## 问题分析

**根本原因**：`minioUtils.go:17` 中 `PutObject` 的 size 参数硬编码为 `-1`，导致 MinIO SDK 在处理未知大小流时将数据读入内存缓冲，引发 OOM。

**问题链条**：

```
actions.go:45/118 → BuildBodyReader(未传ContentLength)
                     ↓
minioUpload.go:72 → uploadToMinIO(PipeReader)
                     ↓
minioUpload.go:87 → PutObject(ctx, key, pr, contentType)
                     ↓
minioUtils.go:17 → client.PutObject(..., -1, ...)
                                    ^^^
                                    触发 MinIO SDK 内存缓冲
```

## 解决方案：混合策略

### 策略 A：优先使用 Content-Length（Happy Path）

当 `ContentLength > 0` 时，直接透传给 MinIO，零内存占用。

### 策略 B：临时文件中转（保底方案）

当 `ContentLength == -1`（chunked 编码）时，先写入临时文件获得确切 size，再上传。

## 修改文件清单

| 文件                     | 修改内容                                        |
| ------------------------ | ----------------------------------------------- |
| `myminio/minioUtils.go`  | 添加 `PutObjectWithSize` 方法                   |
| `myminio/minioUpload.go` | 添加 `contentLength` 字段，实现条件分支上传逻辑 |
| `mproxy/actions.go`      | 传递 `ContentLength` 给 `BuildBodyReader`       |

## 详细实现步骤

### 步骤 1：修改 `myminio/minioUtils.go`

添加新方法 `PutObjectWithSize`，允许传入确切的大小：

```go
// PutObjectWithSize 上传对象到 MinIO（指定大小，推荐使用）
func (c *Client) PutObjectWithSize(ctx context.Context, key string, reader io.Reader, size int64, contentType string) (minio.UploadInfo, error) {
    opts := minio.PutObjectOptions{
        ContentType: contentType,
    }
    // 传入真实的 size，避免 MinIO SDK 使用内存缓冲
    return c.client.PutObject(ctx, c.config.Bucket, key, reader, size, opts)
}
```

### 步骤 2：修改 `myminio/minioUpload.go`

#### 2.1 扩展 `bodyCaptReader` 结构体

```go
type bodyCaptReader struct {
    inner         io.ReadCloser  // 内层 Reader（通常是流量统计层）
    pipeWriter    *io.PipeWriter // 用于向上传协程传输数据
    Capture       *BodyCapture   // 捕获状态
    doneCh        chan struct{}  // 上传完成信号
    skipUpload    bool           // 是否跳过捕获
    contentLength int64          // HTTP Content-Length（-1 表示未知）
}
```

#### 2.2 修改 `BuildBodyReader` 函数签名

```go
func BuildBodyReader(inner io.ReadCloser, sessionID int64, bodyType, contentType string, contentLength int64) (*bodyCaptReader)
```

#### 2.3 重写 `uploadToMinIO` 方法

```go
func (r *bodyCaptReader) uploadToMinIO(pr *io.PipeReader) {
    defer close(r.doneCh)
    defer pr.Close()

    ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
    defer cancel()

    var info minio.UploadInfo
    var err error

    if r.contentLength > 0 {
        // 策略 A：已知长度，直接流式上传（零内存占用）
        info, err = GlobalClient.PutObjectWithSize(ctx, r.Capture.ObjectKey, pr, r.contentLength, r.Capture.ContentType)
    } else {
        // 策略 B：未知长度，使用临时文件中转
        info, err = r.uploadViaTempFile(ctx, pr)
    }

    if err != nil {
        r.Capture.Error = err
        return
    }

    r.Capture.Size = info.Size
    r.Capture.Uploaded = true
}

// uploadViaTempFile 通过临时文件上传（辅助方法）
func (r *bodyCaptReader) uploadViaTempFile(ctx context.Context, pr *io.PipeReader) (minio.UploadInfo, error) {
    // 1. 创建临时文件
    if err := os.MkdirAll("myminio/tmp", 0755); err != nil {
        return minio.UploadInfo{}, err
    }

    tempFile, err := os.CreateTemp("myminio/tmp", "upload-*.tmp")
    if err != nil {
        return minio.UploadInfo{}, err
    }
    tempPath := tempFile.Name()

    defer func() {
        tempFile.Close()
        os.Remove(tempPath)
    }()

    // 2. 写入临时文件
    size, err := io.Copy(tempFile, pr)
    if err != nil {
        return minio.UploadInfo{}, err
    }

    // 3. 重置文件指针
    if _, err := tempFile.Seek(0, 0); err != nil {
        return minio.UploadInfo{}, err
    }

    // 4. 使用已知 size 上传
    contentType := r.Capture.ContentType
    if contentType == "" {
        contentType = "application/octet-stream"
    }

    return GlobalClient.PutObjectWithSize(ctx, r.Capture.ObjectKey, tempFile, size, contentType)
}
```

### 步骤 3：修改 `mproxy/actions.go`

在两处调用 `BuildBodyReader` 的地方传递 `ContentLength`：

#### 3.1 请求阶段（约第 45 行）

```go
contentType := req.Header.Get("Content-Type")
captReader := myminio.BuildBodyReader(trafficReader, ctx.Session, "req", contentType, req.ContentLength)
```

#### 3.2 响应阶段（约第 118 行）

```go
contentType := resp.Header.Get("Content-Type")
captReader := myminio.BuildBodyReader(trafficReader, ctx.Session, "resp", contentType, resp.ContentLength)
```

### 步骤 4：创建临时文件目录

在项目根目录或启动脚本中确保 `myminio/tmp` 目录存在：

```bash
mkdir -p myminio/tmp
```

或在代码中 `init()` 时自动创建。

## 验证测试

### 1. 单元测试

```bash
cd myminio
go test -v -run TestBodyCapture
```

### 2. 内存测试

上传 100MB 文件，监控内存占用：

```bash
# ContentLength > 0 场景（应该保持低内存）
curl -X POST -p -x http://127.0.0.1:8080 --data-binary @test/data/huge_100m.bin http://localhost:9001/test/upload

# Chunked 编码场景（应该使用磁盘，内存仍低）
curl -X POST  -p -x http://127.0.0.1:8080 -H "Transfer-Encoding: chunked"  --data-binary @test/data/huge_100m.bin http://localhost:9001/test/upload
```

### 3. 监控内存使用

```bash
# Windows
tasklist /FI "IMAGENAME eq proxy_man.exe"

# 或在代码中添加 runtime.MemStats 打印
```

## 预期效果

| 场景              | 修改前             | 修改后             |
| ----------------- | ------------------ | ------------------ |
| ContentLength > 0 | 高内存（SDK 缓冲） | 零额外内存         |
| Chunked 编码      | 高内存 + 可能 OOM  | 磁盘 I/O，内存稳定 |
| 并发大文件        | 内存爆炸风险       | 磁盘代替内存，稳定 |

## 注意事项

1. **临时文件目录**：确保 `myminio/tmp` 目录存在且有写入权限
2. **清理机制**：代码已包含 `defer os.Remove(tempPath)` 确保清理
3. **并发安全**：每个上传使用独立的临时文件，无竞争问题
4. **性能权衡**：临时文件方案增加磁盘 I/O，但避免 OOM 风险

## 后续优化（可选）

如需进一步优化，可考虑：

- 添加磁盘空间监控
- 添加文件大小/并发数量限制
- 使用内存映射文件（mmap）优化性能