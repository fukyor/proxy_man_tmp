# 动态流量统计、连接显示、日志显示 - 完整实施计划

## 一、功能概述

实现三个核心功能：
1. **动态流量统计**：实时显示上传/下载速率，支持图表展示
2. **动态连接显示**：实时显示所有活动代理连接，包括每个连接的流量
3. **动态日志显示**：实时推送日志，支持级别过滤

### 架构设计

```
                    ┌─────────────────────────────────────────┐
                    │              前端 (Vue 3)                │
                    │  ┌─────────┐  ┌──────────┐  ┌────────┐  │
                    │  │Overview │  │   Logs   │  │  其他  │  │
                    │  └────┬────┘  └────┬─────┘  └───┬────┘  │
                    │       │            │            │       │
                    │       └────────────┼────────────┘       │
                    │                    ▼                    │
                    │           ┌───────────────┐             │
                    │           │ WebSocket Store│             │
                    │           │ (发布-订阅)    │             │
                    │           └───────┬───────┘             │
                    └───────────────────┼─────────────────────┘
                                        │ 单一 WebSocket 连接
                    ┌───────────────────┼─────────────────────┐
                    │                   ▼                     │
                    │           ┌───────────────┐             │
                    │           │ WebSocket Hub │             │
                    │           │ (sync.Map)    │             │
                    │           └───────┬───────┘             │
                    │     ┌─────────────┼─────────────┐       │
                    │     ▼             ▼             ▼       │
                    │ ┌───────┐   ┌──────────┐   ┌───────┐   │
                    │ │Traffic│   │Connection│   │  Log  │   │
                    │ │Pusher │   │  Pusher  │   │Pusher │   │
                    │ └───┬───┘   └────┬─────┘   └───┬───┘   │
                    │     │            │             │        │
                    │     ▼            ▼             ▼        │
                    │ GlobalTraffic  Connections  LogCollector│
                    │ (atomic.Int64) (sync.Map)   (Channel)  │
                    │              后端 (Go)                  │
                    └─────────────────────────────────────────┘
```

---

## 二、后端实现计划

### 2.1 全局流量统计

**文件**：`mproxy/https_traffic.go`

添加全局流量计数器：
```go
var (
    GlobalTrafficUp   atomic.Int64  // 累计上行流量
    GlobalTrafficDown atomic.Int64  // 累计下行流量
)
```

修改 `TrafficCounter.Read()` 方法，在每次读取响应体时累加全局下行流量：
```go
func (c *TrafficCounter) Read(p []byte) (n int, err error) {
    n, err = c.ReadCloser.Read(p)
    c.resp_body += int64(n)
    GlobalTrafficDown.Add(int64(n))  // 实时累加全局下行
    return
}
```

**文件**：`mproxy/https_traffic.go`

修改 `reqBodyReader.Read()` 方法，在读取请求体时累加全局上行流量：
```go
func (r *reqBodyReader) Read(p []byte) (n int, err error) {
    n, err = r.ReadCloser.Read(p)
    r.counter.req_body += int64(n)
    GlobalTrafficUp.Add(int64(n))  // 实时累加全局上行
    return
}
```

**文件**：`mproxy/tunnel_traffic.go`

修改隧道流量统计，实时累加全局流量：
```go
type tunnelTrafficClient struct {
    halfClosable
    nread    int64
    nwrite   int64
    onUpdate func()
}

func (r *tunnelTrafficClient) Read(p []byte) (n int, err error) {
    n, err = r.halfClosable.Read(p)
    r.nread += int64(n)
    GlobalTrafficDown.Add(int64(n))  // 隧道下行 = 从服务端读取
    return
}

func (r *tunnelTrafficClient) Write(p []byte) (n int, err error) {
    n, err = r.halfClosable.Write(p)
    r.nwrite += int64(n)
    GlobalTrafficUp.Add(int64(n))  // 隧道上行 = 写入服务端
    return
}
```

同样修改 `tunnelTrafficClientNoClosable`。

---

### 2.2 连接管理

**文件**：`mproxy/core_proxy.go`

在 `CoreHttpServer` 结构体中添加连接存储：
```go
type CoreHttpServer struct {
    // ... 现有字段 ...
    Connections sync.Map  // int64 (Session) -> *ConnectionInfo
}
```

**文件**：`proxysocket/types.go`（新建）

定义连接信息结构：
```go
package proxysocket

import "time"

type ConnectionInfo struct {
    Session    int64     `json:"id"`
    Host       string    `json:"host"`
    Method     string    `json:"method"`
    URL        string    `json:"url"`
    RemoteAddr string    `json:"remote"`
    Protocol   string    `json:"protocol"`  // HTTP / HTTPS-Tunnel / HTTPS-MITM
    StartTime  time.Time `json:"startTime"`
    UploadRef  *int64    `json:"-"`         // 引用流量计数器（用于读取实时值）
    DownloadRef *int64   `json:"-"`
}
```

**文件**：`mproxy/http.go`

在 `MyHttpHandle()` 中注册/注销连接：
```go
func (proxy *CoreHttpServer) MyHttpHandle(w http.ResponseWriter, r *http.Request) {
    ctxt := &Pcontext{
        // ... 现有初始化 ...
    }

    // 注册连接
    proxy.Connections.Store(ctxt.Session, &proxysocket.ConnectionInfo{
        Session:    ctxt.Session,
        Host:       r.Host,
        Method:     r.Method,
        URL:        r.URL.String(),
        RemoteAddr: r.RemoteAddr,
        Protocol:   "HTTP",
        StartTime:  time.Now(),
        UploadRef:  &ctxt.TrafficCounter.req_body,
        DownloadRef: &ctxt.TrafficCounter.resp_body,
    })

    // 在流量统计关闭回调中注销（确保流量统计完成后再删除）
    originalOnClose := ctxt.TrafficCounter.onClose
    ctxt.TrafficCounter.onClose = func(bodyBytes int64) {
        if originalOnClose != nil {
            originalOnClose(bodyBytes)
        }
        proxy.Connections.Delete(ctxt.Session)
    }

    // ... 原有处理逻辑 ...
}
```

**文件**：`mproxy/https.go`

在 `MyHttpsHandle()` 的三种模式中分别处理：

**ConnectAccept（隧道模式）**：
```go
case ConnectAccept:
    // ... 现有隧道建立代码 ...

    // 注册连接（隧道模式作为整体长连接）
    proxy.Connections.Store(ctxt.Session, &proxysocket.ConnectionInfo{
        Session:    ctxt.Session,
        Host:       host,
        Method:     "CONNECT",
        URL:        host,
        RemoteAddr: r.RemoteAddr,
        Protocol:   "HTTPS-Tunnel",
        StartTime:  time.Now(),
        UploadRef:  &proxyClientTCP.nwrite,
        DownloadRef: &proxyClientTCP.nread,
    })

    // 在 onUpdate（连接关闭）时注销
    proxyClientTCP.onUpdate = func() {
        // ... 现有日志代码 ...
        proxy.Connections.Delete(ctxt.Session)
    }
```

**ConnectMitm（TLS MITM 模式）和 ConnectHTTPMitm（HTTP MITM 模式）**：

这两种模式不需要特殊处理。原因：
- `for !reqTlsReader.IsEOF()` / `for !reqReader.IsEOF()` 循环内
- 每次循环创建新的 `ctxt`（第 455-462 行），包含：
  - 新的 `Session`（`atomic.AddInt64(&proxy.sess, 1)`）
  - 新的 `TrafficCounter: &TrafficCounter{}`
- 然后调用 `filterRequest` 和 `filterResponse`
- **现有的 `AddTrafficMonitor` Hook 已经可以正确统计每个请求的流量**

需要在循环内添加连接注册/注销（类似 HTTP 模式）：
```go
// 在 for 循环内，创建 ctxt 之后
proxy.Connections.Store(ctxt.Session, &proxysocket.ConnectionInfo{
    Session:    ctxt.Session,
    Host:       req.Host,
    Method:     req.Method,
    URL:        req.URL.String(),
    RemoteAddr: r.RemoteAddr,
    Protocol:   "HTTPS-MITM",  // 或 "HTTP-MITM"
    StartTime:  time.Now(),
    UploadRef:  &ctxt.TrafficCounter.req_body,
    DownloadRef: &ctxt.TrafficCounter.resp_body,
})

// 在 continueLoop 函数的 defer 或响应完成后注销
defer proxy.Connections.Delete(ctxt.Session)
```

**关键理解**：
- `ConnectAccept`：透明隧道，整体作为一个长连接，使用 `tunnelTrafficClient` 统计
- `ConnectMitm`/`ConnectHTTPMitm`：解密后的 HTTP 请求，每个请求独立统计，复用现有 `TrafficCounter` 机制

---

### 2.3 日志收集

**文件**：`mproxy/logs.go`

扩展日志系统：
```go
package mproxy

import (
    "fmt"
    "regexp"
    "strconv"
    "strings"
    "time"
)

type Logger interface {
    Printf(format string, v ...any)
}

// 日志消息结构
type LogMessage struct {
    Level   string    `json:"level"`
    Session int64     `json:"session"`
    Message string    `json:"message"`
    Time    time.Time `json:"time"`
}

// 全局日志 Channel
var LogChan = make(chan LogMessage, 1000)

// 日志收集器，包装原有 Logger
type LogCollector struct {
    Underlying Logger
}

func NewLogCollector(underlying Logger) *LogCollector {
    return &LogCollector{Underlying: underlying}
}

func (l *LogCollector) Printf(format string, v ...any) {
    msg := fmt.Sprintf(format, v...)
    level, session, payload := ParseLogMessage(msg)

    // 非阻塞发送到 Channel
    select {
    case LogChan <- LogMessage{
        Level:   level,
        Session: session,
        Message: payload,
        Time:    time.Now(),
    }:
    default:
        // Channel 满时丢弃，避免阻塞主流程
    }

    // 同时输出到原始 Logger
    l.Underlying.Printf(format, v...)
}

// 解析日志消息，提取级别、Session、内容
func ParseLogMessage(msg string) (level string, session int64, payload string) {
    level = "INFO"
    session = 0
    payload = msg

    // 匹配 Session ID: [001] 格式
    sessionRe := regexp.MustCompile(`\[(\d+)\]`)
    if matches := sessionRe.FindStringSubmatch(msg); len(matches) > 1 {
        session, _ = strconv.ParseInt(matches[1], 10, 64)
    }

    // 判断日志级别
    if strings.Contains(msg, "WARN:") || strings.Contains(msg, "warn") {
        level = "WARN"
    } else if strings.Contains(msg, "ERROR:") || strings.Contains(msg, "error") {
        level = "ERROR"
    } else if strings.Contains(msg, "DEBUG:") {
        level = "DEBUG"
    }

    // 提取 payload（移除前缀）
    payloadRe := regexp.MustCompile(`\[\d+\]\s*(?:INFO|WARN|ERROR|DEBUG)?:?\s*(.*)`)
    if matches := payloadRe.FindStringSubmatch(msg); len(matches) > 1 {
        payload = strings.TrimSpace(matches[1])
    }

    return
}
```

---

### 2.4 WebSocket 服务

**文件**：`proxysocket/proxy.go`（重构）

```go
package proxysocket

import (
    "encoding/json"
    "errors"
    "fmt"
    "net/http"
    "sync"

    "github.com/gorilla/websocket"
    "github.com/rs/cors"
    "proxy_man/mproxy"
)

var upgrader = websocket.Upgrader{
    ReadBufferSize:  1024,
    WriteBufferSize: 1024,
    CheckOrigin:     func(r *http.Request) bool { return true },
    Error: func(w http.ResponseWriter, r *http.Request, status int, reason error) {
        w.Header().Set("Connection", "close")
        w.WriteHeader(status)
        w.Write([]byte("请使用websocket协议"))
        fmt.Println(reason)
    },
}

// 订阅信息
type Subscription struct {
    Traffic     bool
    Connections bool
    Logs        bool
    LogLevel    string
    writeMu     sync.Mutex  // 保护 WebSocket 写操作
}

// WebSocket Hub
type WebSocketHub struct {
    clients sync.Map  // *websocket.Conn -> *Subscription
    proxy   *mproxy.CoreHttpServer
}

var hub *WebSocketHub

// 启动控制服务器
func StartControlServer(proxy *mproxy.CoreHttpServer, addr string, secret string) {
    hub = &WebSocketHub{proxy: proxy}

    mux := http.NewServeMux()
    mux.HandleFunc("/start", loginHandler(secret, handleWebSocket))

    corsMiddleware := cors.New(cors.Options{
        AllowedOrigins:   []string{"*"},
        AllowedMethods:   []string{"GET", "POST", "OPTIONS"},
        AllowedHeaders:   []string{"*"},
        AllowCredentials: true,
    })

    // 启动推送服务
    hub.StartTrafficPusher()
    hub.StartConnectionPusher()
    hub.StartLogPusher()

    go http.ListenAndServe(addr, corsMiddleware.Handler(mux))
}

func loginHandler(secret string, next http.HandlerFunc) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        if secret != "" {
            token := r.URL.Query().Get("token")
            if token != secret {
                http.Error(w, "Unauthorized", http.StatusUnauthorized)
                return
            }
        }
        next(w, r)
    }
}

func handleWebSocket(w http.ResponseWriter, r *http.Request) {
    conn, err := upgrader.Upgrade(w, r, nil)
    if err != nil {
        var handshakeErr websocket.HandshakeError
        if errors.As(err, &handshakeErr) {
            fmt.Println("握手失败")
        }
        return
    }
    defer conn.Close()

    // 注册客户端
    sub := &Subscription{LogLevel: "INFO"}
    hub.clients.Store(conn, sub)
    defer hub.clients.Delete(conn)

    // 处理客户端消息
    for {
        _, message, err := conn.ReadMessage()
        if err != nil {
            break
        }

        var msg map[string]any
        if err := json.Unmarshal(message, &msg); err != nil {
            continue
        }

        action, _ := msg["action"].(string)
        switch action {
        case "subscribe":
            hub.updateSubscription(conn, sub, msg)
        case "closeAllConnections":
            hub.proxy.Connections.Range(func(key, value any) bool {
                hub.proxy.Connections.Delete(key)
                return true
            })
        }
    }
}
```

**文件**：`proxysocket/hub.go`（新建）

```go
package proxysocket

import (
    "encoding/json"
    "time"

    "github.com/gorilla/websocket"
    "proxy_man/mproxy"
)

// 更新订阅
func (h *WebSocketHub) updateSubscription(conn *websocket.Conn, sub *Subscription, msg map[string]any) {
    if topics, ok := msg["topics"].([]any); ok {
        sub.Traffic = contains(topics, "traffic")
        sub.Connections = contains(topics, "connections")
        sub.Logs = contains(topics, "logs")
    }
    if logLevel, ok := msg["logLevel"].(string); ok {
        sub.LogLevel = logLevel
    }
}

func contains(slice []any, item string) bool {
    for _, s := range slice {
        if str, ok := s.(string); ok && str == item {
            return true
        }
    }
    return false
}

// 向指定客户端发送消息（线程安全）
func (h *WebSocketHub) sendTo(conn *websocket.Conn, sub *Subscription, msg any) {
    sub.writeMu.Lock()
    defer sub.writeMu.Unlock()
    data, _ := json.Marshal(msg)
    conn.WriteMessage(websocket.TextMessage, data)
}

// 广播到订阅指定主题的客户端
func (h *WebSocketHub) broadcastToTopic(topic string, msg any) {
    h.clients.Range(func(key, value any) bool {
        conn := key.(*websocket.Conn)
        sub := value.(*Subscription)

        var shouldSend bool
        switch topic {
        case "traffic":
            shouldSend = sub.Traffic
        case "connections":
            shouldSend = sub.Connections
        }

        if shouldSend {
            h.sendTo(conn, sub, msg)
        }
        return true
    })
}

// 流量推送器（每秒推送一次）
func (h *WebSocketHub) StartTrafficPusher() {
    var lastUp, lastDown int64

    go func() {
        ticker := time.NewTicker(time.Second)
        defer ticker.Stop()

        for range ticker.C {
            currentUp := mproxy.GlobalTrafficUp.Load()
            currentDown := mproxy.GlobalTrafficDown.Load()

            deltaUp := currentUp - lastUp
            deltaDown := currentDown - lastDown

            lastUp, lastDown = currentUp, currentDown

            h.broadcastToTopic("traffic", map[string]any{
                "type": "traffic",
                "data": map[string]int64{"up": deltaUp, "down": deltaDown},
            })
        }
    }()
}

// 连接推送器（每 2 秒推送一次）
func (h *WebSocketHub) StartConnectionPusher() {
    go func() {
        ticker := time.NewTicker(2 * time.Second)
        defer ticker.Stop()

        for range ticker.C {
            connections := make([]map[string]any, 0)

            h.proxy.Connections.Range(func(key, value any) bool {
                info := value.(*ConnectionInfo)
                connData := map[string]any{
                    "id":       info.Session,
                    "host":     info.Host,
                    "method":   info.Method,
                    "url":      info.URL,
                    "remote":   info.RemoteAddr,
                    "protocol": info.Protocol,
                }

                // 读取实时流量（如果有引用）
                if info.UploadRef != nil {
                    connData["up"] = *info.UploadRef
                }
                if info.DownloadRef != nil {
                    connData["down"] = *info.DownloadRef
                }

                connections = append(connections, connData)
                return true
            })

            h.broadcastToTopic("connections", map[string]any{
                "type": "connections",
                "data": connections,
            })
        }
    }()
}

// 日志推送器（实时推送）
func (h *WebSocketHub) StartLogPusher() {
    go func() {
        for msg := range mproxy.LogChan {
            h.broadcastLog(msg)
        }
    }()
}

func (h *WebSocketHub) broadcastLog(msg mproxy.LogMessage) {
    data := map[string]any{
        "type": "log",
        "data": map[string]any{
            "level":   msg.Level,
            "session": msg.Session,
            "message": msg.Message,
            "time":    msg.Time,
        },
    }

    h.clients.Range(func(key, value any) bool {
        conn := key.(*websocket.Conn)
        sub := value.(*Subscription)

        if sub.Logs && shouldSendLog(msg.Level, sub.LogLevel) {
            h.sendTo(conn, sub, data)
        }
        return true
    })
}

var logLevels = map[string]int{"DEBUG": 0, "INFO": 1, "WARN": 2, "ERROR": 3}

func shouldSendLog(msgLevel, clientLevel string) bool {
    return logLevels[msgLevel] >= logLevels[clientLevel]
}
```

---

### 2.5 主程序修改

**文件**：`main.go`

```go
func main() {
    proxy := mproxy.NewCoreHttpSever()
    proxy.Verbose = true
    proxy.AllowHTTP2 = false

    // 使用 LogCollector 包装原有 Logger
    proxy.Logger = mproxy.NewLogCollector(proxy.Logger)

    mproxy.AddTrafficMonitor(proxy)
    mproxy.HttpMitmMode(proxy)

    // 启动 WebSocket 控制服务
    proxysocket.StartControlServer(proxy, ":8000", "123")

    // 启动代理服务
    s := &http.Server{Addr: ":8080", Handler: proxy}
    log.Fatal(s.ListenAndServe())
}
```

---

## 三、前端实现计划

### 3.1 WebSocket Store

**文件**：`proxyui/src/stores/websocket.js`

```javascript
import { defineStore } from 'pinia'
import { ref } from 'vue'

export const useWebSocketStore = defineStore('websocket', () => {
  // 状态
  const socket = ref(null)
  const isConnected = ref(false)
  const subscriptions = ref({
    traffic: true,
    connections: true,
    logs: true,
    logLevel: 'INFO'
  })

  // 数据
  const trafficHistory = ref([])  // 最近 60 秒流量历史
  const connections = ref([])      // 当前连接列表
  const logs = ref([])            // 日志列表

  const MAX_HISTORY = 60
  const MAX_LOGS = 500

  // 订阅者回调
  const trafficSubscribers = ref(new Set())
  const connectionsSubscribers = ref(new Set())
  const logsSubscribers = ref(new Set())

  // 连接
  function connect(apiUrl, secret) {
    if (socket.value) disconnect()

    const wsProtocol = apiUrl.startsWith('https') ? 'wss:' : 'ws:'
    const wsHost = apiUrl.replace(/^https?:\/\//, '')
    const wsUrl = `${wsProtocol}//${wsHost}/start?token=${secret || ''}`

    socket.value = new WebSocket(wsUrl)

    socket.value.onopen = () => {
      isConnected.value = true
      subscribe()
    }

    socket.value.onmessage = (event) => {
      const msg = JSON.parse(event.data)
      handleMessage(msg)
    }

    socket.value.onclose = () => {
      isConnected.value = false
      // 5 秒后自动重连
      setTimeout(() => {
        if (!isConnected.value) connect(apiUrl, secret)
      }, 5000)
    }

    socket.value.onerror = () => {
      isConnected.value = false
    }
  }

  function disconnect() {
    if (socket.value) {
      socket.value.close()
      socket.value = null
      isConnected.value = false
    }
  }

  function subscribe() {
    if (!socket.value || socket.value.readyState !== WebSocket.OPEN) return

    socket.value.send(JSON.stringify({
      action: 'subscribe',
      topics: [
        subscriptions.value.traffic && 'traffic',
        subscriptions.value.connections && 'connections',
        subscriptions.value.logs && 'logs'
      ].filter(Boolean),
      logLevel: subscriptions.value.logLevel
    }))
  }

  function updateSubscriptions(newSubs) {
    subscriptions.value = { ...subscriptions.value, ...newSubs }
    subscribe()
  }

  function closeAllConnections() {
    if (socket.value?.readyState === WebSocket.OPEN) {
      socket.value.send(JSON.stringify({ action: 'closeAllConnections' }))
    }
  }

  function handleMessage(msg) {
    switch (msg.type) {
      case 'traffic':
        handleTraffic(msg.data)
        break
      case 'connections':
        handleConnections(msg.data)
        break
      case 'log':
        handleLog(msg.data)
        break
    }
  }

  function handleTraffic(data) {
    const item = { ...data, timestamp: Date.now() }
    trafficHistory.value.push(item)
    if (trafficHistory.value.length > MAX_HISTORY) {
      trafficHistory.value.shift()
    }
    trafficSubscribers.value.forEach(cb => cb(item))
  }

  function handleConnections(data) {
    connections.value = data
    connectionsSubscribers.value.forEach(cb => cb(data))
  }

  function handleLog(data) {
    const item = { ...data, id: Math.random().toString(36).slice(2) }
    logs.value.push(item)
    if (logs.value.length > MAX_LOGS) {
      logs.value.shift()
    }
    logsSubscribers.value.forEach(cb => cb(item))
  }

  // 订阅方法
  function subscribeTraffic(callback) {
    trafficSubscribers.value.add(callback)
    return () => trafficSubscribers.value.delete(callback)
  }

  function subscribeConnections(callback) {
    connectionsSubscribers.value.add(callback)
    return () => connectionsSubscribers.value.delete(callback)
  }

  function subscribeLogs(callback) {
    logsSubscribers.value.add(callback)
    return () => logsSubscribers.value.delete(callback)
  }

  function clearLogs() {
    logs.value = []
  }

  return {
    socket,
    isConnected,
    subscriptions,
    trafficHistory,
    connections,
    logs,
    connect,
    disconnect,
    updateSubscriptions,
    closeAllConnections,
    subscribeTraffic,
    subscribeConnections,
    subscribeLogs,
    clearLogs
  }
})
```

### 3.2 Overview.vue

**文件**：`proxyui/src/views/Overview.vue`

实现功能：
- 流量图表（Chart.js 折线图，最近 60 秒）
- 实时上传/下载速率显示
- 连接列表表格（ID、方法、Host、URL、流量）
- "关闭所有连接"按钮

### 3.3 Logs.vue

**文件**：`proxyui/src/views/Logs.vue`

实现功能：
- 日志列表（时间、级别、Session、消息）
- 级别过滤下拉框（DEBUG/INFO/WARN/ERROR）
- "清除日志"按钮
- 自动滚动到底部

### 3.4 依赖安装

```bash
cd proxyui
npm install chart.js
```

---

## 四、协议定义

### 客户端 → 服务端

```json
// 订阅
{"action": "subscribe", "topics": ["traffic", "connections", "logs"], "logLevel": "INFO"}

// 关闭所有连接
{"action": "closeAllConnections"}
```

### 服务端 → 客户端

```json
// 流量（每秒）
{"type": "traffic", "data": {"up": 12345, "down": 67890}}

// 连接（每 2 秒）
{"type": "connections", "data": [
  {"id": 1, "method": "GET", "host": "example.com", "url": "/api", "remote": "127.0.0.1:12345", "protocol": "HTTPS-MITM", "up": 1234, "down": 5678}
]}

// 日志（实时）
{"type": "log", "data": {"level": "INFO", "session": 1, "message": "...", "time": "2024-01-23T10:30:45Z"}}
```

---

## 五、文件修改清单

### 后端

| 文件 | 操作 | 说明 |
|------|------|------|
| `mproxy/https_traffic.go` | 修改 | 添加全局计数器，修改 Read 方法实时累加 |
| `mproxy/tunnel_traffic.go` | 修改 | 修改 Read/Write 实时累加全局流量 |
| `mproxy/core_proxy.go` | 修改 | 添加 Connections sync.Map |
| `mproxy/http.go` | 修改 | 添加连接注册/注销 |
| `mproxy/https.go` | 修改 | 三种模式添加连接注册/注销 |
| `mproxy/logs.go` | 修改 | 添加 LogCollector、LogChan、ParseLogMessage |
| `proxysocket/proxy.go` | 重构 | WebSocket Hub、订阅处理 |
| `proxysocket/hub.go` | 新建 | 三个 Pusher 实现 |
| `proxysocket/types.go` | 新建 | ConnectionInfo 结构定义 |
| `main.go` | 修改 | 替换 Logger，启动 WebSocket 服务 |

### 前端

| 文件 | 操作 | 说明 |
|------|------|------|
| `proxyui/src/stores/websocket.js` | 重写 | 完整的发布-订阅 Store |
| `proxyui/src/api/api.js` | 修改 | 简化 WebSocket URL 构建 |
| `proxyui/src/views/Overview.vue` | 实现 | 流量图表 + 连接列表 |
| `proxyui/src/views/Logs.vue` | 实现 | 日志展示 + 级别过滤 |
| `proxyui/package.json` | 修改 | 添加 chart.js |

---

## 六、实施顺序

1. **后端阶段 1**：全局流量统计（https_traffic.go, tunnel_traffic.go）
2. **后端阶段 2**：连接管理（core_proxy.go, http.go, https.go, types.go）
3. **后端阶段 3**：日志收集（logs.go）
4. **后端阶段 4**：WebSocket 服务（proxy.go, hub.go, main.go）
5. **前端阶段 1**：WebSocket Store（websocket.js）
6. **前端阶段 2**：Overview 页面（Overview.vue）
7. **前端阶段 3**：Logs 页面（Logs.vue）

---

## 七、验证方法

1. **流量统计**：启动代理，访问网页，观察 WebSocket 推送的流量数据
2. **连接显示**：发起多个请求，验证连接列表实时更新和自动清理
3. **日志显示**：设置不同日志级别，验证过滤功能正常
4. **长连接**：通过代理访问 WebSocket 服务，验证长连接流量实时显示
5. **并发安全**：多个浏览器同时连接，验证推送无冲突