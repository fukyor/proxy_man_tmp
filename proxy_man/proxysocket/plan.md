# 隧道连接分级显示功能 - 最终实现计划

## 目标

在 Overview.vue 页面实现隧道与子请求的父子关系显示，点击隧道行打开侧边栏显示子请求列表。

---

## 后端修改方案

### 核心思路

- 在 `MyHttpsHandle` 函数开头创建隧道记录
- 子请求设置 `ParentSess` 指向隧道 Session
- **隧道流量由前端聚合子请求实时计算**（不在后端累加）
- ConnectAccept 模式覆盖隧道记录（用自己的流量计数器）

### 1. 修改 `MyHttpsHandle` 入口 (`https.go:189-222`)

```go
func (proxy *CoreHttpServer) MyHttpsHandle(w http.ResponseWriter, r *http.Request) {
    ctxt := &Pcontext{
        core_proxy:     proxy,
        Req:            r,
        TrafficCounter: &TrafficCounter{},
        Session:        atomic.AddInt64(&proxy.sess, 1),
    }

    // ===== 新增：创建顶层隧道连接记录 =====
    tunnelSession := ctxt.Session
    proxy.Connections.Store(tunnelSession, &ConnectionInfo{
        Session:     tunnelSession,
        ParentSess:  0,
        Host:        r.URL.Host,
        Method:      "TCP",
        URL:         r.URL.Host,
        RemoteAddr:  r.RemoteAddr,
        Protocol:    "TUNNEL",
        StartTime:   time.Now(),
        // 注意：MITM 模式下隧道流量由前端聚合子请求计算，此处不设置 UploadRef/DownloadRef
    })
    // ===== 新增结束 =====

    // 创建 hijack
    hijk, ok := w.(http.Hijacker)
    // ... 后续代码 ...
```

### 2. ConnectAccept 模式 (`https.go:225-310`)

**保留现有逻辑**，覆盖入口记录（用 tunnelTrafficClient 的流量引用）：

```go
// 现有代码不变，第 256-267 行的 Store 会覆盖入口记录
// 并设置 UploadRef/DownloadRef 指向 proxyClientTCP.nread/nwrite
```

### 3. ConnectHTTPMitm 模式 (`https.go:315-465`)

#### 3.1 在 case 开头添加 defer 清理隧道记录

```go
case ConnectHTTPMitm:
    _, _ = connFromClinet.Write([]byte("HTTP/1.0 200 OK\r\n\r\n"))
    ctxt.Log_P("HTTP Tunnel established, http MITM")

    defer proxy.Connections.Delete(tunnelSession)  // ===== 新增 =====
```

#### 3.2 修改子请求 ConnectionInfo (第 357-368 行)

添加 `ParentSess` 字段：

```go
proxy.Connections.Store(ctxt.Session, &ConnectionInfo{
    Session:     ctxt.Session,
    ParentSess:  tunnelSession,  // ===== 新增：指向顶层隧道 =====
    Host:        r.Host,
    Method:      req.Method,
    URL:         req.URL.String(),
    RemoteAddr:  r.RemoteAddr,
    Protocol:    "HTTP-MITM",
    StartTime:   time.Now(),
    UploadRef:   &ctxt.TrafficCounter.req_sum,
    DownloadRef: &ctxt.TrafficCounter.resp_sum,
    OnClose:     func() { finishRequest() },
})
```

#### 3.3 子请求 defer (第 369 行)

保持现有逻辑不变（不累加流量到隧道，由前端聚合）：

```go
defer proxy.Connections.Delete(ctxt.Session) // 现有代码保持不变
```

### 4. ConnectMitm 模式 (`https.go:466-671`)

#### 4.1 在 go func() 内添加 defer 清理隧道记录

```go
go func() {
    defer proxy.Connections.Delete(tunnelSession)  // ===== 新增 =====

    tlsConn := tls.Server(connFromClinet, tlsConfig)
    defer tlsConn.Close()
    // ...
```

#### 4.2 修改子请求 ConnectionInfo (第 522-533 行)

添加 `ParentSess` 字段：

```go
proxy.Connections.Store(ctxt.Session, &ConnectionInfo{
    Session:     ctxt.Session,
    ParentSess:  tunnelSession,  // ===== 新增：指向顶层隧道 =====
    Host:        r.Host,
    Method:      req.Method,
    URL:         req.URL.String(),
    RemoteAddr:  r.RemoteAddr,
    Protocol:    "HTTPS-MITM",
    StartTime:   time.Now(),
    UploadRef:   &ctxt.TrafficCounter.req_sum,
    DownloadRef: &ctxt.TrafficCounter.resp_sum,
    OnClose:     func() { finishRequest() },
})
```

#### 4.3 子请求 defer (第 534 行)

保持现有逻辑不变（不累加流量到隧道，由前端聚合）：

```go
defer proxy.Connections.Delete(ctxt.Session) // 现有代码保持不变
```

---

## 前端修改方案

### 关键文件

- `proxyui/src/views/Overview.vue`

### 修改原则

- **不修改活动连接表格组件**
- 点击连接行后在侧边栏显示子请求列表

### 修改内容

1. 新增响应式数据：`sidebarVisible`, `selectedTunnel`
2. 新增计算属性：
   - `sidebarChildren`：选中隧道的子请求列表
   - `sidebarTraffic`：聚合子请求流量计算隧道总流量
3. 新增交互方法：`handleRowClick`, `closeSidebar`, `formatTime`
4. 新增侧边栏组件（详见原 plan.md）
5. 新增样式（详见原 plan.md）

---

## 关键文件清单

| 文件                             | 修改内容                                                     |
| -------------------------------- | ------------------------------------------------------------ |
| `mproxy/https.go`                | 入口创建隧道记录 + 各模式覆盖/更新 + 子请求 ParentSess + 流量累加 |
| `proxyui/src/views/Overview.vue` | 计算属性 + 侧边栏组件 + 交互逻辑 + 样式                      |

---

## 验证方案

```bash
# 后端
go build -o proxy_man && ./proxy_man -v
curl -x localhost:8080 --insecure https://httpbin.org/get

# 前端
cd proxyui && npm run dev
```

测试点：

1. 主表格只显示 parentId=0 的连接
2. 点击隧道行打开侧边栏
3. MITM 模式显示真实子请求
4. 纯隧道模式显示虚拟节点
5. 流量统计正确聚合