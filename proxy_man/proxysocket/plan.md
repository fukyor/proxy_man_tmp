# 延迟删除（墓碑机制）实施计划

## 问题描述

连接断开后立即从 `Connections` Map 删除，导致短连接（生命周期 < 2秒推送间隔）从未被 WebSocket 推送到前端。

**根本原因**：WebSocket 推送器每 2 秒轮询一次，短连接在两次推送之间完成了"创建-传输-关闭-删除"的全过程。

## 解决方案

采用**延迟删除（墓碑机制）**：连接关闭时不立即删除，而是标记为 "Closed" 状态，保留 3 秒后再物理删除。

---

## 实施步骤

### 1. 修改 ConnectionInfo 结构体

**文件**: `mproxy/connections.go`

添加两个字段：

```go
type ConnectionInfo struct {
    // ... 现有字段 ...
    Status   string    `json:"status"`   // "Active" 或 "Closed"
    EndTime  time.Time `json:"endTime"`  // 连接关闭时间
}

// 添加便捷方法
func (c *ConnectionInfo) MarkClosed() {
    c.Status = "Closed"
    c.EndTime = time.Now()
}
```

### 2. 添加 MarkConnectionClosed 方法

**文件**: `mproxy/connections.go`

```go
func (proxy *CoreHttpServer) MarkConnectionClosed(session int64) {
    if value, ok := proxy.Connections.Load(session); ok {
        info := value.(*ConnectionInfo)
        info.MarkClosed()
    }
}
```

### 3. 修改所有 Store 调用（5处）

在创建连接时初始化 `Status: "Active"`：

| 文件              | 行号 | 协议类型     |
| ----------------- | ---- | ------------ |
| `mproxy/http.go`  | 33   | HTTP         |
| `mproxy/https.go` | 199  | TUNNEL       |
| `mproxy/https.go` | 272  | HTTPS-Tunnel |
| `mproxy/https.go` | 369  | HTTP-MITM    |
| `mproxy/https.go` | 544  | HTTPS-MITM   |

### 4. 修改所有 Delete 调用（8处）

将 `proxy.Connections.Delete(session)` 改为 `proxy.MarkConnectionClosed(session)`：

| 文件                | 行号 | 说明                        |
| ------------------- | ---- | --------------------------- |
| `mproxy/http.go`    | 74   | respBodyReader.onClose 回调 |
| `mproxy/http.go`    | 90   | 响应为空时                  |
| `mproxy/https.go`   | 328  | HTTP MITM 隧道退出          |
| `mproxy/https.go`   | 384  | HTTP MITM 子请求完成        |
| `mproxy/https.go`   | 498  | TLS MITM 隧道退出           |
| `mproxy/https.go`   | 559  | TLS MITM 子请求完成         |
| `mproxy/actions.go` | 115  | 隧道 onClose 回调           |
| `mproxy/actions.go` | 124  | 隧道 onClose 回调           |

### 5. 修改推送器

**文件**: `proxysocket/hub.go` 的 `StartConnectionPusher` 函数

添加垃圾回收逻辑：

```go
func (h *WebSocketHub) StartConnectionPusher() {
    const tombstoneRetention = 3 * time.Second

    go func() {
        ticker := time.NewTicker(2 * time.Second)
        defer ticker.Stop()

        for range ticker.C {
            connections := make([]map[string]any, 0)
            now := time.Now()

            h.proxy.Connections.Range(func(key, value any) bool {
                session := key.(int64)
                info := value.(*mproxy.ConnectionInfo)

                // 垃圾回收：已关闭且超过保留时间，物理删除
                if info.Status == "Closed" && now.Sub(info.EndTime) > tombstoneRetention {
                    h.proxy.Connections.Delete(session)
                    return true
                }

                // 收集连接数据（包含 status 字段）
                connData := map[string]any{
                    "id":        info.Session,
                    "parentId":  info.ParentSess,
                    "host":      info.Host,
                    "method":    info.Method,
                    "url":       info.URL,
                    "remote":    info.RemoteAddr,
                    "protocol":  info.Protocol,
                    "startTime": info.StartTime,
                    "status":    info.Status,
                }
                // ... 流量字段 ...
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
```

---

## 关键文件清单

| 文件                    | 修改内容                       |
| ----------------------- | ------------------------------ |
| `mproxy/connections.go` | 添加 Status/EndTime 字段和方法 |
| `mproxy/http.go`        | 1处 Store + 2处 Delete 修改    |
| `mproxy/https.go`       | 4处 Store + 4处 Delete 修改    |
| `mproxy/actions.go`     | 2处 Delete 修改                |
| `proxysocket/hub.go`    | 推送器 GC 逻辑和 status 字段   |

---

## 验证方法

1. 启动代理服务器：`go run main.go -v`
2. 发起短请求：`curl -x localhost:8080 http://example.com`
3. 观察 WebSocket 推送：
   - 短连接应以 `"status": "Closed"` 出现
   - 3 秒后从列表消失（物理删除）
4. 验证长连接流量统计仍然正确

## 前端兼容性

- JSON 新增 `status` 字段（"Active" 或 "Closed"）
- 可选：前端对 Closed 状态的连接显示灰色背景