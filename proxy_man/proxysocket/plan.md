# MinIO 对象存储集成方案

## 一、方案概述

### 核心策略

采用**纯流式上传**方案：

1. **请求/响应阶段**：使用 `io.Pipe` + `io.TeeReader` 双流写入（客户端/服务端 + MinIO）
2. **实时上传**：数据流经时直接上传到 MinIO，无需内存缓冲
3. **下载阶段**：使用 Presigned URL 实现直链下载

### 流式上传优势

- **零内存占用**：不需要内存缓冲，大文件不会导致内存溢出
- **实时传输**：数据边读边上传，无需等待整个 Body 读取完成
- **简单可靠**：单一策略，无需判断文件大小和切换模式

---

## 二、数据结构设计

### 2.1 扩展 HttpExchange 相关结构

**文件**: `mproxy/mitm_exchange.go`

```go
// RequestSnapshot 请求快照
type RequestSnapshot struct {
    Method  string              `json:"method"`
    URL     string              `json:"url"`
    Host    string              `json:"host"`
    Header  map[string][]string `json:"header"`
    SumSize int64               `json:"sumSize"`

    // 新增：MinIO 对象存储引用
    BodyKey      string `json:"bodyKey,omitempty"`      // MinIO 对象 Key（格式：mitm-data/{YYYY-MM-DD}/{SessionID}/req）
    BodySize     int64  `json:"bodySize,omitempty"`     // Body 实际大小
    BodyUploaded bool   `json:"bodyUploaded,omitempty"` // 是否已上传到 MinIO
    ContentType  string `json:"contentType,omitempty"`  // Content-Type（用于下载时设置）
}

// ResponseSnapshot 响应快照
type ResponseSnapshot struct {
    StatusCode int                 `json:"statusCode"`
    Status     string              `json:"status"`
    Header     map[string][]string `json:"header"`
    SumSize    int64               `json:"sumSize"`

    // 新增：MinIO 对象存储引用
    BodyKey      string `json:"bodyKey,omitempty"`      // MinIO 对象 Key（格式：mitm-data/{YYYY-MM-DD}/{SessionID}/resp）
    BodySize     int64  `json:"bodySize,omitempty"`     // Body 实际大小
    BodyUploaded bool   `json:"bodyUploaded,omitempty"` // 是否已上传到 MinIO
    ContentType  string `json:"contentType,omitempty"`  // Content-Type
}

// HttpExchange 完整交换记录
type HttpExchange struct {
    ID        int64            `json:"id"`
    SessionID int64            `json:"sessionId"`
    ParentID  int64            `json:"parentId"`
    Time      int64            `json:"time"`
    Request   RequestSnapshot  `json:"request"`
    Response  ResponseSnapshot `json:"response"`
    Duration  int64            `json:"duration"`
    Error     string           `json:"error,omitempty"`
}
```

### 2.2 MinIO 配置结构

**新增文件**: `minio/config.go`

```go
// Config MinIO 客户端配置
type Config struct {
    Endpoint        string // MinIO 服务器地址（如：play.min.io:9000）
    AccessKeyID     string // 访问密钥 ID
    SecretAccessKey string // 密钥
    UseSSL          bool   // 是否使用 HTTPS
    Bucket          string // 存储桶名称（如：proxy-traffic）
}

// Client MinIO 客户端封装
type Client struct {
    client *minio.Client
    config Config
}

// 全局单例
var GlobalMinIOClient *Client
```

### 2.3 Body 捕获器

**新增文件**: `mproxy/body_capture.go`

```go
// bodyCaptReader 流式上传 Body Reader
type bodyCaptReader struct {
    io.ReadCloser
    pipeReader  *io.PipeReader  // Pipe Reader（连接到 MinIO 上传）
    pipeWriter  *io.PipeWriter  // Pipe Writer（接收数据流）
    size        int64           // 已读取字节数（单线程写入，无需 atomic）
    contentType string          // Content-Type
    sessionID   int64           // SessionID
    bodyType    string          // "req" 或 "resp"
    uploadErr   error           // 上传错误（通过 channel 传递）
    doneCh      chan struct{}   // 上传完成信号
}
```

---

## 三、对象命名规范

### 3.1 Object Key 格式

```
格式：mitm-data/{YYYY-MM-DD}/{SessionID}/{Type}
示例：mitm-data/2026-02-02/10086/req
     mitm-data/2026-02-02/10086/resp

参数说明：
- mitm-data：固定的路径前缀，用于分类
- YYYY-MM-DD：日期分区，便于按日期管理和清理
- SessionID：代理生成的唯一会话 ID（int64）
- Type：req（请求体）或 resp（响应体）
```

### 3.2 Bucket 配置

```go
Bucket: "proxy-traffic"

建议策略：
- 版本控制：禁用（不需要历史版本）
- 生命周期：设置 30 天过期规则
- 访问策略：私有（通过 Presigned URL 访问）
```

### 3.3 SessionID → ObjectKey 映射

**确定性映射**（无需数据库）：

```go
func GetObjectKey(sessionID int64, bodyType string) string {
    date := time.Now().Format("2006-01-02")
    return fmt.Sprintf("mitm-data/%s/%d/%s", date, sessionID, bodyType)
}

// 示例
sessionID := 10086
reqKey := GetObjectKey(sessionID, "req")  // "mitm-data/2026-02-02/10086/req"
respKey := GetObjectKey(sessionID, "resp") // "mitm-data/2026-02-02/10086/resp"
```

---

## 四、实现流程设计

### 4.1 请求处理流程（流式上传）

```
1. 客户端发起请求
   ↓
2. HookOnReq 触发（actions.go）
   ↓
3. 创建 bodyCaptReader 包装 req.Body
   ├─ 创建 io.Pipe
   ├─ 启动 MinIO 上传协程（从 PipeReader 读取）
   ├─ 替换 req.Body 为 TeeReader（原始 → PipeWriter）
   └─ sessionID: ctx.Session
   ↓
4. RoundTrip 发送请求到目标服务器
   ├─ req.Body.Read() 触发
   ├─ TeeReader 同时写入 PipeWriter
   └─ MinIO 协程实时接收并上传
   ↓
5. 响应返回，HookOnResp 触发
   ↓
6. 创建 bodyCaptReader 包装 resp.Body
   ├─ 创建新的 io.Pipe
   ├─ 启动 MinIO 上传协程
   └─ 替换 resp.Body
   ↓
7. io.Copy 将响应写回客户端
   ├─ resp.Body.Read() 触发
   ├─ TeeReader 同时写入 PipeWriter
   └─ MinIO 协程实时上传
   ↓
8. resp.Body.Close() 触发
   ↓
9. bodyCaptReader.Close()
   ├─ 关闭 PipeWriter
   ├─ 等待 MinIO 上传完成
   └─ 记录上传结果（size、error）
   ↓
10. SendExchange()
   ├─ 设置 exchange.Request.BodyKey
   ├─ 设置 exchange.Request.BodySize
   ├─ 设置 exchange.Response.BodyKey
   └─ 推送到 GlobalExchangeChan
```

### 4.2 流式上传实现

```go
// 创建流式上传 Reader
func newBodyCaptReader(body io.ReadCloser, sessionID int64, bodyType, contentType string) *bodyCaptReader {
    // 创建 Pipe
    pr, pw := io.Pipe()

    reader := &bodyCaptReader{
        ReadCloser: io.TeeReader(body, pw), // 双流：原始消费者 + Pipe
        pipeReader: pr,
        pipeWriter: pw,
        contentType: contentType,
        sessionID:  sessionID,
        bodyType:   bodyType,
        doneCh:     make(chan struct{}),
    }

    // 启动 MinIO 上传协程
    go reader.uploadToMinIO()

    return reader
}

// 上传协程：从 PipeReader 读取并上传到 MinIO
func (r *bodyCaptReader) uploadToMinIO() {
    defer close(r.doneCh)
    defer r.pipeReader.Close()

    ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
    defer cancel()

    objectKey := GetObjectKey(r.sessionID, r.bodyType)
    info, err := GlobalMinIOClient.client.PutObject(ctx, bucket, objectKey, r.pipeReader, -1, minio.PutObjectOptions{
        ContentType: r.contentType,
    })

    if err != nil {
        r.uploadErr = fmt.Errorf("MinIO 上传失败: %w", err)
        return
    }

    // 记录上传大小
    r.size = info.Size
}

// Read 操作：TeeReader 自动写入 PipeWriter
func (r *bodyCaptReader) Read(p []byte) (n int, err error) {
    n, err = r.ReadCloser.Read(p)
    r.size += int64(n)  // Read() 只在单个 goroutine 中调用，无需 atomic
    return n, err
}

// Close 操作：关闭 PipeWriter，等待上传完成
func (r *bodyCaptReader) Close() error {
    // 1. 关闭 PipeWriter（触发上传结束）
    r.pipeWriter.Close()

    // 2. 关闭原始 Body
    err := r.ReadCloser.Close()

    // 3. 等待minio处理数据完成，虽然此时pw已经上传完毕，而且因为pw和pr是同步的
    // 说明minio端的pr肯定也是读取完毕了。但是minio处理数据和返回200OK还需要时间
    // 我们这里就是等待PutObject函数返回
    <-r.doneCh

    return err
}

// 获取上传结果
func (r *bodyCaptReader) GetUploadResult() (int64, error) {
    <-r.doneCh
    return r.size, r.uploadErr  // doneCh 关闭后才读取，无需 atomic
}
```

---

## 五、关键实现细节

### 5.1 Context 生命周期隔离

**问题**：如果使用 `req.Context()` 启动上传协程，客户端断开时 Context 会被 Cancel，导致上传中断。

**解决方案**：上传协程必须使用独立的 Context

```go
// ❌ 错误：使用请求 Context
go minioClient.PutObject(req.Context(), bucket, key, ...)

// ✅ 正确：使用独立 Context
go func() {
    ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
    defer cancel()
    minioClient.PutObject(ctx, bucket, key, ...)
}()
```

### 5.2 并发安全保护

**上传状态同步**：

```go
type bodyCaptReader struct {
    // ...
    size      int64             // Read() 单线程写入，Close() 后写入最终值
    uploadErr error             // 通过 doneCh 同步
    doneCh    chan struct{}     // 完成信号
}

// 分析：
// 1. Read() 只在 HTTP 请求处理的单个 goroutine 中调用
// 2. uploadToMinIO() 协程在 PipeWriter.Close() 后才写入最终 size
// 3. GetUploadResult() 只在 doneCh 关闭后读取 size
// 结论：不存在并发竞争，无需 atomic 操作
```

### 5.3 错误处理与重试

```go
func (c *Client) UploadWithRetry(ctx context.Context, bucket, key string, reader io.Reader, size int64) error {
    maxRetries := 3
    for i := 0; i < maxRetries; i++ {
        _, err := c.client.PutObject(ctx, bucket, key, reader, size, minio.PutObjectOptions{})
        if err == nil {
            return nil
        }

        // 网络错误重试
        if isNetworkError(err) && i < maxRetries-1 {
            time.Sleep(time.Second * time.Duration(i+1))
            continue
        }

        return err
    }
    return fmt.Errorf("上传失败，已重试 %d 次", maxRetries)
}
```

### 5.4 WebSocket/SSE 跳过处理

```go
func (r *bodyCaptReader) Read(p []byte) (n int, err error) {
    n, err = r.ReadCloser.Read(p)

    // 检测流式响应特征
    contentType := r.contentType
    if contentType == "text/event-stream" ||
       strings.Contains(contentType, "websocket") {
        // 跳过捕获，只转发数据
        return n, err
    }

    // 正常捕获逻辑
    // ...
}
```

---

## 六、后端 API 设计

### 6.1 下载接口

**路径**: `GET /api/storage/download`

**查询参数**:

| 参数         | 类型   | 必填 | 说明                                       |
| ------------ | ------ | ---- | ------------------------------------------ |
| session_id   | int64  | 是   | 会话 ID                                    |
| type         | string | 是   | "req" 或 "resp"                            |
| filename     | string | 否   | 下载文件名（默认：{sessionID}_{type}.bin） |
| content_type | string | 否   | 强制 Content-Type（默认：自动检测）        |

**响应**:

```json
{
  "code": 0,
  "message": "success",
  "data": {
    "downloadUrl": "https://minio.example.com/proxy-traffic/...",
    "expiresAt": "2026-02-02T13:00:00Z",
    "filename": "10086_req.json",
    "size": 1024
  }
}
```

**错误响应**:

```json
// 对象不存在
{
  "code": 404,
  "message": "对象不存在或正在上传中"
}

// SessionID 无效
{
  "code": 400,
  "message": "无效的 session_id"
}
```

### 6.2 状态检查接口

**路径**: `GET /api/storage/status`

**查询参数**: `session_id`

**响应**:

```json
{
  "code": 0,
  "data": {
    "sessionId": 10086,
    "reqBody": {
      "uploaded": true,
      "size": 1024,
      "key": "mitm-data/2026-02-02/10086/req"
    },
    "respBody": {
      "uploaded": false,
      "status": "uploading"  // "uploaded" | "uploading" | "failed"
    }
  }
}
```

---

## 七、后端 API 文档

### 7.1 下载接口

**路径**: `GET /api/storage/download`

**查询参数**:

| 参数         | 类型   | 必填 | 说明                                                |
| ------------ | ------ | ---- | --------------------------------------------------- |
| session_id   | int64  | 是   | 会话 ID                                             |
| type         | string | 是   | "req" 或 "resp"                                     |
| filename     | string | 否   | 下载文件名（默认：{sessionID}_{type}.bin）          |
| content_type | string | 否   | 强制 Content-Type（默认：application/octet-stream） |

**响应**:

```json
{
  "code": 0,
  "message": "success",
  "data": {
    "downloadUrl": "https://minio.example.com/proxy-traffic/...",
    "expiresAt": "2026-02-02T13:00:00Z",
    "filename": "10086_req.json",
    "size": 1024
  }
}
```

**错误响应**:

```json
// 对象不存在
{
  "code": 404,
  "message": "对象不存在或正在上传中"
}

// SessionID 无效
{
  "code": 400,
  "message": "无效的 session_id"
}

// MinIO 服务异常
{
  "code": 500,
  "message": "MinIO 服务不可用"
}
```

### 7.2 状态检查接口

**路径**: `GET /api/storage/status`

**查询参数**:

| 参数       | 类型  | 必填 | 说明    |
| ---------- | ----- | ---- | ------- |
| session_id | int64 | 是   | 会话 ID |

**响应**:

```json
{
  "code": 0,
  "message": "success",
  "data": {
    "sessionId": 10086,
    "reqBody": {
      "uploaded": true,
      "size": 1024,
      "key": "mitm-data/2026-02-02/10086/req",
      "contentType": "application/json"
    },
    "respBody": {
      "uploaded": true,
      "size": 2048,
      "key": "mitm-data/2026-02-02/10086/resp",
      "contentType": "application/json"
    }
  }
}
```

**上传状态枚举**:

| 状态        | 说明       |
| ----------- | ---------- |
| uploaded    | 上传完成   |
| uploading   | 正在上传中 |
| failed      | 上传失败   |
| not_started | 未开始上传 |

**错误响应**:

```json
// SessionID 不存在
{
  "code": 404,
  "message": "会话不存在"
}
```

## 十、关键文件清单

### 需要修改的文件

| 文件                      | 修改内容            | 优先级 |
| ------------------------- | ------------------- | ------ |
| `mproxy/mitm_exchange.go` | 扩展数据结构        | **高** |
| `mproxy/actions.go`       | 集成 Body 捕获      | **高** |
| `main.go`                 | 初始化 MinIO 客户端 | **高** |

### 需要新增的文件

| 文件                     | 职责               | 优先级 |
| ------------------------ | ------------------ | ------ |
| `minio/config.go`        | MinIO 配置和客户端 | **高** |
| `minio/uploader.go`      | 上传逻辑和重试     | **高** |
| `mproxy/body_capture.go` | Body 捕获器        | **高** |
| `api/storage.go`         | 下载接口           | 中     |
| `minio/lifecycle.go`     | 定期清理任务       | 低     |

### 只读参考文件

| 文件                      | 用途                   |
| ------------------------- | ---------------------- |
| `mproxy/https_traffic.go` | 参考现有 Body 包装逻辑 |
| `mproxy/ctxt.go`          | 了解 Pcontext 结构     |
| `mproxy/https.go`         | 了解 MITM 流程         |

---

## 十一、风险与应对

| 风险         | 影响 | 应对措施                               |
| ------------ | ---- | -------------------------------------- |
| 上传失败     | 高   | 实现重试机制和错误日志记录             |
| Pipe 阻塞    | 中   | 设置合理的上传超时时间（30分钟）       |
| 网络中断     | 中   | 使用独立 Context，避免请求取消影响上传 |
| MinIO 不可用 | 低   | 降级为不存储，记录错误日志             |

---

## 十二、总结

### 核心优势

1. **零内存占用**：纯流式上传，无需内存缓冲
2. **确定性映射**：SessionID → ObjectKey 无需数据库查询
3. **直链下载**：Presigned URL 支持动态 Content-Type
4. **简单可靠**：单一策略，无需判断文件大小

### 技术亮点

1. **实时上传**：数据流经时直接上传，无需等待 Body 读取完成
2. **错误恢复**：自动重试和状态追踪
3. **安全隔离**：独立 Context 避免生命周期冲突

### 预期效果

- **性能影响**：< 3% 延迟增加（流式上传）
- **内存占用**：固定开销（Pipe 缓冲区约 32KB）
- **存储成本**：按实际 Body 大小计算
- **可靠性**：99.9% 上传成功率（3 次重试）