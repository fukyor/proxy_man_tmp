# MITM Exchange 捕获方案 - 实施计划（简洁版）

## 设计理念

**最小侵入原则**：利用现有的 `respBodyReader.onClose` 回调机制 + 给 Pcontext 新增少量字段，将捕获逻辑完全封装在 `mitm_exchange.go` 中，`https.go` 主逻辑几乎无需改动。

## 对原始 plan.md 的关键修正

| #    | 原始方案问题                      | 修正方案                                                     |
| ---- | --------------------------------- | ------------------------------------------------------------ |
| 1    | 使用 UUID                         | 改用 `int64` 原子递增计数器，不引入新依赖                    |
| 2    | 用 `ctx.UserData` 存储元数据      | 给 Pcontext 新增 `ExchangeCapture` 专用字段（结构化，不会与用户 UserData 冲突） |
| 3    | 在 https.go 添加大量局部变量      | 所有变量封装在 `ExchangeCapture` 结构中，https.go 只需调用 `StartCapture()` 和 `SetError()` |
| 4    | filterRequest 后立即读取 BodySize | 在 `respBodyReader.onClose` 回调中触发发送，此时流量统计已完成 |
| 5    | 每个 return 点手动插入代码        | 通过 onClose 回调自动触发，错误路径只需 `SetError()`         |
| 6    | WebSocket 跳过逻辑分散            | `SkipCapture()` 一行代码                                     |

---

## 步骤 1：新建 `mproxy/mitm_exchange.go`

将所有捕获逻辑封装在此文件中：

```go
package mproxy

import (
    "net/http"
    "sync/atomic"
    "time"
)

var exchangeIDCounter int64

// HttpExchange 代表 MITM 模式下一次完整的 HTTP 请求-响应交互
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

type RequestSnapshot struct {
    Method   string              `json:"method"`
    URL      string              `json:"url"`
    Host     string              `json:"host"`
    Header   map[string][]string `json:"header"`
    BodySize int64               `json:"bodySize"`
}

type ResponseSnapshot struct {
    StatusCode int                 `json:"statusCode"`
    Status     string              `json:"status"`
    Header     map[string][]string `json:"header"`
    BodySize   int64               `json:"bodySize"`
}

var GlobalExchangeChan = make(chan *HttpExchange, 1000)

// ========== ExchangeCapture：封装捕获状态 ==========

// ExchangeCapture 封装单次请求的捕获状态
type ExchangeCapture struct {
    startTime  time.Time
    reqSnap    RequestSnapshot
    parentID   int64
    skip       bool
    err        error
    sent       bool  // 防止重复发送
}

// StartCapture 初始化捕获，在 Pcontext 创建后调用
func (ctx *Pcontext) StartCapture(parentSession int64) {
    ctx.exchangeCapture = &ExchangeCapture{
        startTime: time.Now(),
        parentID:  parentSession,
    }
}

// CaptureRequest 捕获请求快照，在 filterRequest 之后调用
func (ctx *Pcontext) CaptureRequest(req *http.Request) {
    if ctx.exchangeCapture == nil {
        return
    }
    ctx.exchangeCapture.reqSnap = RequestSnapshot{
        Method: req.Method,
        URL:    req.URL.String(),
        Host:   req.Host,
        Header: cloneHeader(req.Header),
    }
}

// SkipCapture 标记跳过捕获（用于 WebSocket）
func (ctx *Pcontext) SkipCapture() {
    if ctx.exchangeCapture != nil {
        ctx.exchangeCapture.skip = true
    }
}

// SetCaptureError 记录错误
func (ctx *Pcontext) SetCaptureError(err error) {
    if ctx.exchangeCapture != nil && err != nil {
        ctx.exchangeCapture.err = err
    }
}

// SendExchange 发送 Exchange 到全局通道，在响应完成后自动调用
// 这个方法会被 respBodyReader.onClose 触发
func (ctx *Pcontext) SendExchange() {
    cap := ctx.exchangeCapture
    if cap == nil || cap.skip || cap.sent {
        return
    }
    cap.sent = true

    exchange := &HttpExchange{
        ID:        atomic.AddInt64(&exchangeIDCounter, 1),
        SessionID: ctx.Session,
        ParentID:  cap.parentID,
        Time:      cap.startTime.UnixMilli(),
        Request:   cap.reqSnap,
        Duration:  time.Since(cap.startTime).Milliseconds(),
    }

    // 从 TrafficCounter 读取 body 大小
    if ctx.TrafficCounter != nil {
        exchange.Request.BodySize = ctx.TrafficCounter.req_body
    }

    // 从 ctx.Resp 读取响应信息
    if ctx.Resp != nil {
        exchange.Response = ResponseSnapshot{
            StatusCode: ctx.Resp.StatusCode,
            Status:     ctx.Resp.Status,
            Header:     cloneHeader(ctx.Resp.Header),
        }
        if ctx.TrafficCounter != nil {
            exchange.Response.BodySize = ctx.TrafficCounter.resp_body
        }
    }

    if cap.err != nil {
        exchange.Error = cap.err.Error()
    }

    // 非阻塞发送
    select {
    case GlobalExchangeChan <- exchange:
    default:
    }
}

func cloneHeader(h http.Header) map[string][]string {
    if h == nil {
        return nil
    }
    result := make(map[string][]string, len(h))
    for k, v := range h {
        result[k] = append([]string{}, v...)
    }
    return result
}
```

---

## 步骤 2：修改 `mproxy/ctxt.go` — Pcontext 新增字段

在 Pcontext 结构体末尾新增一个字段（第 24 行后）：

```go
type Pcontext struct {
    // ... 现有字段 ...
    tunnelTrafficClientNoClosable *tunnelTrafficClientNoClosable

    exchangeCapture *ExchangeCapture  // 新增：MITM Exchange 捕获状态
}
```

**说明**：只新增一个指针字段，不污染结构体，非 MITM 请求该字段为 nil。

---

## 步骤 3：修改 `mproxy/actions.go` — 在 onClose 中触发发送

在 `AddTrafficMonitor` 的 `respBodyReader.onClose` 回调末尾添加一行：

**位置**：第 64-72 行的 onClose 闭包内

```go
onClose: func() {
    ctx.TrafficCounter.UpdateTotal()
    ctx.parCtx.TrafficCounter.UpdateTotal()
    ctx.Log_P("[流量统计] ...")

    ctx.SendExchange()  // === 新增：触发 Exchange 发送 ===
},
```

**说明**：onClose 在响应体读取完成后触发，此时 `req_body` 和 `resp_body` 都已统计完毕，是最佳发送时机。

---

## 步骤 4：修改 `mproxy/https.go` — 极简插入

### 4.1 requestOk（HTTP-MITM）— 共 4 处改动

**位置**：第 361-486 行

| 行号         | 改动           | 代码                                    |
| ------------ | -------------- | --------------------------------------- |
| ~L368 后     | 启动捕获       | `ctxt.StartCapture(tunnelSession)`      |
| ~L399 后     | 捕获请求       | `ctxt.CaptureRequest(req)`              |
| ~L434        | WebSocket 跳过 | `if isWebsocket { ctxt.SkipCapture() }` |
| 各错误返回前 | 记录错误       | `ctxt.SetCaptureError(err)`             |

**完整改动示例**：

```go
requestOk := func(req *http.Request) bool {
    ctxt := &Pcontext{...}
    ctxt.StartCapture(tunnelSession)  // === 新增 ===

    // 注册连接（原有代码）
    proxy.Connections.Store(ctxt.Session, &ConnectionInfo{...})
    defer proxy.MarkConnectionClosed(ctxt.Session)

    // ... 原有代码 ...

    req, resp := proxy.filterRequest(req, ctxt)
    ctxt.CaptureRequest(req)  // === 新增 ===

    if resp == nil {
        // ... 建立连接 ...
        if err != nil {
            ctxt.SetCaptureError(err)  // === 新增 ===
            ctxt.WarnP("...")
            return false
        }
        // ... 发送请求 ...
    }

    resp = proxy.filterResponse(resp, ctxt)
    defer resp.Body.Close()

    isWebsocket := isWebSocketHandshake(resp.Header)
    if isWebsocket {
        ctxt.SkipCapture()  // === 新增 ===
    }

    // ... 写回响应（原有代码，各错误返回前添加 ctxt.SetCaptureError(err)） ...

    return true
}
```

### 4.2 continueLoop（HTTPS-MITM）— 同样 4 处改动

**位置**：第 550-699 行

改动位置与 requestOk 对称：

1. `ctxt` 创建后（L534 后）：`ctxt.StartCapture(tunnelSession)`
2. `filterRequest` 后（L576 后）：`ctxt.CaptureRequest(req)`
3. WebSocket 检查处（L608）：`if isWebsocket { ctxt.SkipCapture() }`
4. 各错误返回前：`ctxt.SetCaptureError(err)`

---

## 步骤 5：修改 `proxysocket/proxy.go`

### 5.1 Subscription 新增字段（L28-34）

```go
type Subscription struct {
    Traffic     bool
    Connections bool
    Logs        bool
    LogLevel    string
    MitmDetail  bool       // 新增
    writeMu     sync.Mutex
}
```

### 5.2 StartControlServer 中启动推送器（~L67 后）

```go
hub.StartMitmDetailPusher()  // 新增
```

---

## 步骤 6：修改 `proxysocket/hub.go`

### 6.1 updateSubscription 新增解析（L16 后）

```go
sub.MitmDetail = contains(topics, "mitm_detail")
```

### 6.2 broadcastToTopic 新增 case（L54 后）

```go
case "mitm_detail":
    shouldSend = sub.MitmDetail
```

### 6.3 新增 StartMitmDetailPusher 函数（文件末尾）

```go
func (h *WebSocketHub) StartMitmDetailPusher() {
    go func() {
        for exchange := range mproxy.GlobalExchangeChan {
            h.broadcastToTopic("mitm_detail", map[string]any{
                "type": "mitm_exchange",
                "data": exchange,
            })
        }
    }()
}
```

---

## 文件修改清单

| 文件                      | 操作     | 改动量                               |
| ------------------------- | -------- | ------------------------------------ |
| `mproxy/mitm_exchange.go` | **新建** | ~100 行（数据结构 + 捕获方法）       |
| `mproxy/ctxt.go`          | 修改     | +1 行（新增 exchangeCapture 字段）   |
| `mproxy/actions.go`       | 修改     | +1 行（onClose 中调用 SendExchange） |
| `mproxy/https.go`         | 修改     | ~10 行（4 处简洁调用 × 2 个函数）    |
| `proxysocket/proxy.go`    | 修改     | +2 行                                |
| `proxysocket/hub.go`      | 修改     | +12 行                               |

**不需要修改**：`https_traffic.go`、`connections.go`、`main.go`、`go.mod`

---

## 方案对比

| 维度          | 原始方案                   | 简洁方案                    |
| ------------- | -------------------------- | --------------------------- |
| https.go 改动 | 大量局部变量 + 长 defer 块 | 4 个简洁的方法调用          |
| 错误处理      | 每个 return 前手动插入     | `SetCaptureError(err)` 一行 |
| 发送时机      | defer 块中手动构建         | 复用 onClose 回调自动触发   |
| 代码可读性    | 主逻辑被捕获代码割裂       | 主逻辑几乎不变              |

---

## 前端订阅与推送格式

**订阅**：

```json
{"action": "subscribe", "topics": ["connections", "mitm_detail"]}
```

**推送**：

```json
{
  "type": "mitm_exchange",
  "data": {
    "id": 42,
    "sessionId": 156,
    "parentId": 150,
    "time": 1706500000000,
    "request": {
      "method": "GET",
      "url": "https://example.com/api/users",
      "host": "example.com",
      "header": {"User-Agent": ["Mozilla/5.0..."]},
      "bodySize": 0
    },
    "response": {
      "statusCode": 200,
      "status": "200 OK",
      "header": {"Content-Type": ["application/json"]},
      "bodySize": 1234
    },
    "duration": 150
  }
}
```

---

## 验证方案

1. `go build` 确认编译通过
2. 启动代理 `go run main.go -v`，前端订阅 `mitm_detail`
3. 通过代理访问：`curl -x http://localhost:8080 https://httpbin.org/get`
4. 验证收到 `mitm_exchange` 消息，bodySize > 0，duration > 0
5. 测试错误场景（不存在的域名）：验证 exchange 包含 error 字段
6. 测试 WebSocket：验证不产生 mitm_exchange 消息