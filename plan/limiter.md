# 代理流量限制方案设计

## 需求背景

在 HTTP/HTTPS 代理中实现流量检测和拦截功能，当总下载流量达到设定限制时进行拦截。

---

## 架构分析

### 现有架构
```
CoreHttpServer
├── reqHandlers  []ReqHandler   // 请求过滤器链
├── respHandlers []RespHandler  // 响应过滤器链
├── HookOnReq()                 // 请求 Hook 注册
├── OnResponse()                // 响应 Hook 注册
└── 处理流程：
    filterRequest → RoundTrip → filterResponse → io.Copy
```

### 关键代码位置
- `mproxy/core_proxy.go` - 代理核心结构
- `mproxy/http.go:77` - `io.Copy(bodyWriter, resp.Body)` 数据传输
- `mproxy/hooks.go` - Hook 机制实现

---

## 方案1：流量计数器（统计模块）

### 设计思路
实现一个独立的流量计数器模块，提供流量的累加和查询功能。**注意：本方案只负责统计流量，不负责拦截。**

### 架构设计

#### 1.1 流量计数器模块
```go
// mproxy/traffic_counter.go
package mproxy

import "sync"

// TrafficCounter 流量计数器
type TrafficCounter struct {
    mu           sync.RWMutex
    totalBytes   int64           // 累计总流量（字节）
}

// NewTrafficCounter 创建流量计数器
func NewTrafficCounter() *TrafficCounter

// AddBytes 增加流量计数（线程安全）
func (c *TrafficCounter) AddBytes(n int64)

// GetBytes 获取当前累计流量
func (c *TrafficCounter) GetBytes() int64

// Reset 重置流量计数
func (c *TrafficCounter) Reset()

// GetMB 获取兆字节单位的流量
func (c *TrafficCounter) GetMB() float64

// GetGB 获取吉字节单位的流量
func (c *TrafficCounter) GetGB() float64
```

#### 1.2 在 CoreHttpServer 中集成
```go
// core_proxy.go 中添加
type CoreHttpServer struct {
    // ... 现有字段
    TrafficCounter *TrafficCounter  // 流量计数器
}
```

#### 1.3 无条件 Hook 注册方式

**方式A：直接追加到 handler 链（推荐）**
```go
// 在 NewCoreHttpSever() 或初始化后
proxy.respHandlers = append(proxy.respHandlers,
    FuncRespHandler(trafficCounterHandler))
```

**方式B：创建 AlwaysTrue 条件**
```go
// hooks.go 中添加
func AlwaysTrue() ReqConditionFunc {
    return func(req *http.Request, ctx *Pcontext) bool {
        return true
    }
}

// 使用
proxy.OnResponse().DoFunc(trafficCounterHandler)
```

#### 1.4 Handler 实现

**问题：在 filterResponse 阶段无法准确统计**

由于 `filterResponse` 在 `io.Copy` **之前**执行，此时还未发生实际数据传输，所以：
- 无法通过 `resp.Body` 获取已传输字节数
- `Content-Length` 可能不存在（chunked 编码）
- 即使有 `Content-Length` 也只是预估，实际传输可能因压缩、断开等原因不同

**解决方案：延迟统计**

```go
// trafficCounterHandler 流量计数处理器
func trafficCounterHandler(resp *http.Response, ctx *Pcontext) *http.Response {
    // 方案1：预估流量（不准确，但简单）
    if contentLength := resp.ContentLength; contentLength > 0 {
        ctx.core_proxy.TrafficCounter.AddBytes(contentLength)
        ctx.Log_P("预估流量: %d bytes", contentLength)
        return resp
    }

    // 方案2：包装 resp.Body，在实际传输时统计（准确）
    // 但这需要在 http.go 中配合修改
    resp.Body = &countingReader{
        src:     resp.Body,
        counter: ctx.core_proxy.TrafficCounter,
        ctx:     ctx,
    }
    return resp
}

// countingReader 包装 Body 进行流量统计
type countingReader struct {
    src     io.ReadCloser
    counter *TrafficCounter
    ctx     *Pcontext
}

func (cr *countingReader) Read(p []byte) (n int, err error) {
    n, err = cr.src.Read(p)
    if n > 0 {
        cr.counter.AddBytes(int64(n))
    }
    return n, err
}

func (cr *countingReader) Close() error {
    return cr.src.Close()
}
```

### 优缺点分析

#### 优点
- ✅ 与现有架构完美契合，使用已有的 Hook 机制
- ✅ 实现简单，代码侵入性低
- ✅ 提供流量查询接口，可配合外部监控系统

#### 缺点
- ❌ 在 `filterResponse` 阶段无法获取准确传输字节数
- ❌ 需要包装 `resp.Body` 才能准确统计（稍微增加复杂度）
- ❌ 无法中断 TCP 连接（这不是计数器的问题，是架构限制）

### 适用场景
- 只需要统计流量，不需要拦截
- 流量监控、日志记录、计费系统
- 配合方案2的拦截功能使用（方案2负责拦截，本方案负责查询）

---

## 方案2：基于 io.Reader 包装的实时拦截

### 设计思路
包装 `resp.Body` 的 `io.Reader`，在每次读取时累加字节数，超限则返回错误中断 `io.Copy`。

### 架构设计

#### 2.1 流量统计 Reader
```go
// mproxy/traffic_reader.go
package mproxy

// trafficReader 包装 io.Reader，实时统计流量并在超限时中断
type trafficReader struct {
    src     io.Reader          // 原始响应 Body
    limiter *TrafficLimiter    // 流量限制器
    ctx     *Pcontext          // 请求上下文（用于日志）
}

// newTrafficReader 创建流量统计 Reader
func newTrafficReader(src io.Reader, limiter *TrafficLimiter, ctx *Pcontext) io.ReadCloser

// Read 实现 io.Reader 接口
func (tr *trafficReader) Read(p []byte) (n int, err error)

// Close 实现 io.Closer 接口
func (tr *trafficReader) Close() error
```

#### 2.2 Read 方法核心逻辑
```go
func (tr *trafficReader) Read(p []byte) (n int, err error) {
    // 检查全局流量是否已超限
    if tr.limiter.IsExceeded() {
        tr.ctx.Log_P("流量已达上限，中断传输")
        return 0, errors.New("流量已达上限")
    }

    // 从原始 Body 读取数据
    n, err = tr.src.Read(p)

    // 累加流量计数
    if n > 0 {
        exceeded := tr.limiter.AddBytes(int64(n))
        if exceeded {
            tr.ctx.Log_P("本次读取后流量超限，下次将拦截")
        }
    }

    return n, err
}
```

#### 2.3 在 http.go 中集成
```go
// http.go 的 MyHttpHandler 方法中修改

// 原代码（第 42 行）
resp = proxy.filterResponse(resp, ctxt)

// 修改为：
resp = proxy.filterResponse(resp, ctxt)
if resp != nil && proxy.TrafficLimiter != nil {
    // 包装 resp.Body 进行流量统计
    resp.Body = newTrafficReader(resp.Body, proxy.TrafficLimiter, ctxt)
}
```

#### 2.4 配合请求前拦截（双层防护）
```go
// 在 filterRequest 阶段也检查
func requestTrafficLimiter(req *http.Request, ctx *Pcontext) (*http.Request, *http.Response) {
    if ctx.core_proxy.TrafficLimiter.IsExceeded() {
        // 直接返回拦截响应，不发起实际请求
        return req, &http.Response{
            StatusCode: http.StatusForbidden,
            Header: http.Header{
                "Connection":   []string{"close"},
                "Content-Type": []string{"text/plain; charset=utf-8"},
            },
            Body: io.NopCloser(strings.NewReader("流量已达上限，请求被拦截")),
        }
    }
    return req, nil
}

// 注册到请求过滤器链
proxy.reqHandlers = append(proxy.reqHandlers,
    FuncReqHandler(requestTrafficLimiter))
```

### 优缺点分析

#### 优点
- ✅ 流量统计最准确（基于实际传输的字节）
- ✅ 可以中断正在进行的下载
- ✅ 对 HTTPS CONNECT 隧道也有效（在隧道层面统计）
- ✅ 实时性最好，达到上限立即中断

#### 缺点
- ❌ 需要修改 http.go 的核心转发逻辑
- ❌ 实现复杂度较高
- ❌ 中断时客户端可能只收到部分数据+错误

### 适用场景
- 需要精确控制流量上限
- 必须能实时中断大文件下载
- 对流量控制的准确性要求高

---

## 综合对比

| 维度 | 方案1 (计数器) | 方案2 (拦截器) |
|------|---------------|---------------|
| 主要功能 | 流量统计 | 拦截 + 统计 |
| 实现难度 | ⭐ 简单 | ⭐⭐⭐ 中等 |
| 代码侵入性 | 低 | 中等（需修改 http.go） |
| 流量准确性 | 高（包装 Body 后） | 高 |
| 实时性 | N/A（不涉及拦截） | 高（立即中断传输） |
| HTTPS 支持 | ✅ 完全支持 | ✅ 完全支持 |
| 性能影响 | 低 | 低 |
| 是否可独立使用 | ✅ 是 | ✅ 是（但需要计数器支持） |

**关系说明**：方案1提供流量统计能力，方案2在方案1的基础上增加拦截能力。两者可以独立实现，也可以配合使用。

---

## 推荐实施路径

### 阶段1：实现方案1（流量计数器）
1. 创建 `TrafficCounter` 结构和基础方法
2. 实现包装 `resp.Body` 的 `countingReader`
3. 在响应过滤器链中注册计数 Handler
4. 提供流量查询 API（`GetBytes()`, `GetMB()`, `GetGB()`）

**产出**：可实时查询当前累计流量

### 阶段2：实现方案2（拦截器）
1. 基于 `TrafficCounter` 创建 `TrafficLimiter`
2. 实现请求前拦截（`filterRequest` 阶段）
3. 实现传输中中断（`trafficReader` 超限返回错误）
4. 提供配置接口（设置上限、启用/禁用）

**产出**：达到流量上限时自动拦截

### 阶段3：功能增强（可选）
- 支持按 IP/用户维度限流
- 支持时间窗口限流（每小时/每天）
- 持久化流量统计数据（Redis/数据库）
- 提供管理 API（查询、重置、配置）

---

## 待确认问题

1. **流量统计维度**：
   - [ ] 全局总流量
   - [ ] 单 IP 流量
   - [ ] 单用户流量（需要认证）

2. **流量重置策略**：
   - [ ] 手动重置
   - [ ] 定时重置（每天/每小时）
   - [ ] 永不重置

3. **拦截行为**：
   - [ ] 返回友好提示页面
   - [ ] 直接关闭连接
   - [ ] 返回 HTTP 429 状态码

4. **HTTPS 隧道处理**：
   - CONNECT 方法的流量如何统计？
   - 是否需要在隧道层面单独处理？