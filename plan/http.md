# HTTP 代理实现计划

## 目标

基于 goproxy 项目实现最基本的 HTTP 代理功能。

## 项目核心架构分析

通过阅读源码，goproxy 的核心设计如下：

### 核心组件

| 文件 | 职责 |
|------|------|
| `proxy.go` | `ProxyHttpServer` 主结构体，实现 `http.Handler` 接口 |
| `http.go` | HTTP 请求处理逻辑 |
| `ctx.go` | `ProxyCtx` 请求上下文 |
| `actions.go` | Handler 接口定义 |
| `dispatcher.go` | 条件匹配和处理器注册 |

### 请求处理流程

```
客户端请求 -> ServeHTTP() -> handleHttp()
                              |
                              v
                         filterRequest() -> 执行 reqHandlers
                              |
                              v
                         RoundTrip() -> 发送请求到目标服务器
                              |
                              v
                         filterResponse() -> 执行 respHandlers
                              |
                              v
                         复制响应到客户端
```

---

## 任务拆解

### 任务 1：创建代理服务器结构体

**目标**：创建基础的代理服务器结构体

**参考文件**：`proxy.go:13-54`

**需要实现**：
```go
type ProxyHttpServer struct {
    Verbose bool           // 是否输出详细日志
    Logger  Logger         // 日志接口
    Tr      *http.Transport // HTTP 传输层
}
```

**关键点**：
- 结构体需要实现 `http.Handler` 接口
- `Tr` 用于发送请求到目标服务器

---

### 任务 2：实现 NewProxyHttpServer 构造函数

**目标**：创建代理服务器实例

**参考文件**：`proxy.go:146-156`

**需要实现**：
```go
func NewProxyHttpServer() *ProxyHttpServer {
    // 初始化 Logger
    // 初始化 Transport
}
```

---

### 任务 3：实现 ServeHTTP 方法

**目标**：作为 HTTP 入口点，分发请求

**参考文件**：`proxy.go:137-143`

**需要实现**：
```go
func (proxy *ProxyHttpServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
    // 目前只处理 HTTP 请求
    proxy.handleHttp(w, r)
}
```

**关键点**：
- 检查 `r.URL.IsAbs()` 判断是否为代理请求
- 非代理请求可以返回错误或交给其他 handler

---

### 任务 4：实现 handleHttp 核心方法

**目标**：处理 HTTP 代理请求

**参考文件**：`http.go:10-97`

**需要实现的简化版本**：
```go
func (proxy *ProxyHttpServer) handleHttp(w http.ResponseWriter, r *http.Request) {
    // 1. 检查是否为代理请求 (URL 是否为绝对路径)
    // 2. 移除代理相关的 Header
    // 3. 使用 Transport 发送请求到目标服务器
    // 4. 复制响应 Header 到客户端
    // 5. 写入状态码
    // 6. 复制响应 Body 到客户端
}
```

**关键点**：
- 需要移除 `Proxy-Connection`、`Proxy-Authenticate`、`Proxy-Authorization` 等 Header
- 需要重置 `r.RequestURI = ""`（Go http client 要求）

---

### 任务 5：实现辅助函数

**目标**：实现必要的辅助函数

**参考文件**：`proxy.go:58-68`, `proxy.go:93-120`

**需要实现**：
```go
// 复制 HTTP Header
func copyHeaders(dst, src http.Header, keepDestHeaders bool)

// 移除代理相关 Header
func RemoveProxyHeaders(r *http.Request)
```

---

### 任务 6：实现日志接口

**目标**：提供简单的日志功能

**参考文件**：`logger.go`

**需要实现**：
```go
type Logger interface {
    Printf(format string, v ...any)
}
```

---

### 任务 7：创建 main.go 启动代理

**目标**：创建可运行的代理服务器

**参考文件**：`examples/base/main.go`

**需要实现**：
```go
func main() {
    proxy := NewProxyHttpServer()
    proxy.Verbose = true
    http.ListenAndServe(":8080", proxy)
}
```

---

## 实现顺序

1. **任务 6** - 日志接口（无依赖）
2. **任务 1** - 代理服务器结构体（依赖任务 6）
3. **任务 2** - 构造函数（依赖任务 1）
4. **任务 5** - 辅助函数（无依赖）
5. **任务 4** - handleHttp（依赖任务 1, 5）
6. **任务 3** - ServeHTTP（依赖任务 4）
7. **任务 7** - main.go（依赖所有）

---

## 测试方法

启动代理后，使用 curl 测试：

```bash
# 测试 HTTP 代理
curl -x http://localhost:8080 http://httpbin.org/get

# 测试带 Header 的请求
curl -x http://localhost:8080 -H "X-Custom: test" http://httpbin.org/headers
```

---

## 暂不实现的功能

以下功能在当前阶段不需要实现，后续按需添加：

- HTTPS/CONNECT 代理
- Handler 系统（reqHandlers, respHandlers）
- 条件匹配系统（ReqCondition, RespCondition）
- WebSocket 支持
- MITM 代理
- 证书管理
- 认证功能

---

## 文件结构建议

```
myproxy/
├── proxy.go      # ProxyHttpServer 结构体和 ServeHTTP
├── http.go       # handleHttp 实现
├── logger.go     # Logger 接口
├── main.go       # 入口文件
```
