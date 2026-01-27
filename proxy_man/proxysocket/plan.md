# 隧道流量统计修复实现计划

## 问题描述

当前隧道流量统计使用的 `var tunnelUp, tunnelDown *int64` 不能正确完成统计，主要问题：

1. **首次注册时指针为 nil**：`https.go:200-211` 行注册连接时，`tunnelUp` 和 `tunnelDown` 还未赋值（值为 nil）
2. **重复注册覆盖**：`https.go:271-282` 行用相同 Session ID 重新注册，覆盖了第一次注册
3. **并发安全问题**：`nread` 和 `nwrite` 是普通 `int64` 类型，在 WebSocket 推送时通过指针读取可能存在数据竞争

## 解决方案

扩展 `ConnectionInfo` 结构，新增 `TunnelUp` 和 `TunnelDown` 字段（类型为 `atomic.Int64`），专门用于隧道模式流量统计。

---

## 文件修改清单

| 文件                       | 修改内容                                                     |
| -------------------------- | ------------------------------------------------------------ |
| `mproxy/connections.go`    | 扩展 `ConnectionInfo`，新增 `TunnelUp`、`TunnelDown` 原子计数器 |
| `mproxy/tunnel_traffic.go` | 修改 `tunnelTrafficClient`，关联 `ConnectionInfo` 并更新原子计数器 |
| `mproxy/https.go`          | 重构隧道模式连接注册逻辑，消除外部变量依赖                   |
| `proxysocket/hub.go`       | 修改流量读取逻辑，支持隧道模式流量读取                       |

---

## 详细实现步骤

### 1. 修改 `mproxy/connections.go`

**修改位置**：`ConnectionInfo` 结构体定义

**新增字段**：

```go
type ConnectionInfo struct {
    // ... 现有字段 ...
    TunnelUp   atomic.Int64  // 隧道模式上行流量计数器
    TunnelDown atomic.Int64  // 隧道模式下行流量计数器
}
```

**说明**：

- 新字段位于 `DownloadRef` 之后
- 使用 `atomic.Int64` 类型保证并发安全
- 初始值为 0，自动初始化

---

### 2. 修改 `mproxy/tunnel_traffic.go`

**修改 1：`tunnelTrafficClient` 结构体**

新增 `connInfo` 字段，存储对 `ConnectionInfo` 的引用：

```go
type tunnelTrafficClient struct {
    halfClosable
    connInfo   *ConnectionInfo  // 新增：指向连接信息
    nread      int64           // 保留：用于日志输出
    nwrite     int64           // 保留：用于日志输出
    onUpdate   func()
}
```

**修改 2：`newTunnelTrafficClient` 构造函数**

修改签名，接收 `ConnectionInfo` 参数：

```go
func newTunnelTrafficClient(conn net.Conn, connInfo *ConnectionInfo) (*tunnelTrafficClient, bool)
```

**修改 3：`Read` 方法**

在累加 `nread` 的同时更新原子计数器：

```go
func (r *tunnelTrafficClient) Read(p []byte) (n int, err error) {
    n, err = r.halfClosable.Read(p)
    r.nread += int64(n)
    GlobalTrafficUp.Add(int64(n))
    r.connInfo.TunnelUp.Add(int64(n))  // 新增
    return n, err
}
```

**修改 4：`Write` 方法**

在累加 `nwrite` 的同时更新原子计数器：

```go
func (w *tunnelTrafficClient) Write(p []byte) (n int, err error) {
    n, err = w.halfClosable.Write(p)
    w.nwrite += int64(n)
    GlobalTrafficDown.Add(int64(n))
    r.connInfo.TunnelDown.Add(int64(n))  // 新增
    return n, err
}
```

**修改 5：`tunnelTrafficClientNoClosable`**

同样需要修改结构体、构造函数、`Read` 和 `Write` 方法。

---

### 3. 修改 `mproxy/https.go`

**步骤 1：删除外部变量声明（第 197 行）**

删除或注释掉：

```go
var tunnelUp, tunnelDown *int64
```

**步骤 2：删除首次注册（第 200-211 行）**

完全删除第一次注册连接的代码，避免重复注册。

**步骤 3：重构 `ConnectAccept` 分支（第 240-320 行）**

**修改前**：

```go
proxyClientTCP, clientOK := newTunnelTrafficClient(connFromClinet)

// ... 稍后注册 ...
proxy.Connections.Store(topctxt.Session, &ConnectionInfo{
    UploadRef:   &proxyClientTCP.nread,
    DownloadRef: &proxyClientTCP.nwrite,
})
tunnelUp = &proxyClientTCP.nread
tunnelDown = &proxyClientTCP.nwrite
```

**修改后**：

```go
// 1. 先创建 ConnectionInfo 对象
connInfo := &ConnectionInfo{
    Session:     topctxt.Session,
    Host:        host,
    Method:      "TUNNEL",
    URL:         host,
    RemoteAddr:  r.RemoteAddr,
    Protocol:    "HTTPS-Tunnel",
    StartTime:   time.Now(),
    OnClose:     func() { connFromClinet.Close() },
}

// 2. 存储 ConnectionInfo
proxy.Connections.Store(topctxt.Session, connInfo)

// 3. 创建包装器并关联 ConnectionInfo
proxyClientTCP, clientOK := newTunnelTrafficClient(connFromClinet, connInfo)
proxyClientTCPNo := newtunnelTrafficClientNoClosable(connFromClinet, connInfo)

// 4. 删除 tunnelUp 和 tunnelDown 的赋值（不再需要）
```

**步骤 4：清理 MITM 模式中的无效赋值**

删除或注释以下行：

- `https.go:390-391`（HTTP-MITM 模式）：`tunnelUp = &ctxt.TrafficCounter.req_sum`
- `https.go:560-561`（HTTPS-MITM 模式）：`tunnelDown = &ctxt.TrafficCounter.resp_sum`

**说明**：这些赋值没有实际作用，因为 HTTP/HTTPS-MITM 使用 `TrafficCounter`，不涉及隧道。

---

### 4. 修改 `proxysocket/hub.go`

**修改位置**：`StartConnectionPusher` 方法中的流量读取逻辑（第 110-116 行）

**修改前**：

```go
if info.UploadRef != nil {
    connData["up"] = *info.UploadRef
}
if info.DownloadRef != nil {
    connData["down"] = *info.DownloadRef
}
```

**修改后**：

```go
// 优先读取隧道模式流量
if info.Protocol == "HTTPS-Tunnel" {
    connData["up"] = info.TunnelUp.Load()
    connData["down"] = info.TunnelDown.Load()
} else {
    // HTTP/MITM 模式使用 UploadRef/DownloadRef
    if info.UploadRef != nil {
        connData["up"] = *info.UploadRef
    }
    if info.DownloadRef != nil {
        connData["down"] = *info.DownloadRef
    }
}
```

---

## 向后兼容性保证

| 模式         | 流量数据来源                                        | 说明               |
| ------------ | --------------------------------------------------- | ------------------ |
| HTTPS-Tunnel | `TunnelUp`、`TunnelDown`（原子计数器）              | 新增字段，线程安全 |
| HTTP         | `UploadRef`、`DownloadRef`（指向 `TrafficCounter`） | 保持不变           |
| HTTPS-MITM   | `UploadRef`、`DownloadRef`（指向 `TrafficCounter`） | 保持不变           |

---

## 验证方案

### 编译检查

```bash
go build -o proxy_man main.go
```

### 功能测试

1. **隧道模式流量统计**：
   - 启动代理，发起 HTTPS CONNECT 请求（非 MITM 模式）
   - 通过 WebSocket 订阅连接信息
   - 验证 `up` 和 `down` 实时更新
   - 验证日志输出正确的流量统计

2. **MITM 模式流量统计**：
   - 发起 HTTPS CONNECT 请求（MITM 模式）
   - 验证子请求流量统计正常
   - 验证不影响现有功能

3. **HTTP 模式流量统计**：
   - 发起 HTTP 请求
   - 验证流量统计功能不受影响

### 回归测试

| 功能点                | 预期结果               |
| --------------------- | ---------------------- |
| HTTP 代理流量统计     | 正常工作               |
| HTTPS-Tunnel 流量统计 | 正常工作，修复原有问题 |
| HTTPS-MITM 流量统计   | 正常工作，无副作用     |
| WebSocket 连接推送    | 正常显示所有模式流量   |
| 全局流量统计          | 累加正确               |

---

## 关键修改点总结

| 文件                | 行号范围                       | 修改内容                             |
| ------------------- | ------------------------------ | ------------------------------------ |
| `connections.go`    | ConnectionInfo 结构体          | 新增 `TunnelUp`、`TunnelDown` 字段   |
| `tunnel_traffic.go` | 结构体 + 构造函数 + Read/Write | 新增 `connInfo` 关联，更新原子计数器 |
| `https.go`          | 197, 200-211, 259-284          | 删除外部变量，重构注册逻辑           |
| `hub.go`            | 110-116                        | 修改流量读取逻辑                     |