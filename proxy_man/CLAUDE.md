# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## 项目概述

**proxy_man** 是一个基于 Go 语言实现的 HTTP/HTTPS 代理服务器，基于 `github.com/elazarl/goproxy` v1.7.2 扩展，支持 MITM（中间人）模式、WebSocket、流量统计等功能。

## 常用命令

### 构建和运行
```bash
# 构建主程序
go build -o proxy_man main.go

# 运行代理服务器（默认端口 8080，开启详细日志）
./proxy_man -v

# 自定义端口
./proxy_man -addr :8888 -v

# 关闭详细日志
./proxy_man
```

### 测试
```bash
# 运行所有测试
go test ./...

# 运行特定包的测试
go test ./http1parser/

# 运行特定测试函数（详细输出）
go test -v ./http1parser/ -run TestHeader

# 测试 HTTP 代理
curl -x http://localhost:8080 http://httpbin.org/get

# 测试 HTTPS 代理（MITM 模式）
curl -x http://localhost:8080 https://httpbin.org/get -k
```

### 调试构建
```bash
# 使用调试符号构建（用于 VS Code 调试）
go build -gcflags="all=-N -l" -o use_demo/filter/test_bin ./use_demo/filter/main.go
```

### 依赖管理
```bash
# 下载依赖
go mod download

# 整理依赖
go mod tidy
```

## 核心架构

### 责任链模式

项目使用责任链模式实现请求/响应过滤：

```
ServeHTTP (判断 HTTP/HTTPS)
    ↓
HTTP 请求 → MyHttpHandle → filterRequest (reqHandlers) → RoundTrip → filterResponse (respHandlers) → io.Copy
HTTPS 请求 → MyHttpsHandle → Connect 处理 → MITM 或隧道模式
```

### 关键组件

| 文件 | 职责 |
|------|------|
| `mproxy/core_proxy.go` | `CoreHttpServer` 主结构体，实现 `http.Handler` 接口 |
| `mproxy/http.go` | HTTP 请求处理逻辑 |
| `mproxy/https.go` | HTTPS/CONNECT 处理逻辑（约 620 行，核心文件） |
| `mproxy/hooks.go` | Hook 机制和条件过滤器（`ReqCondition`, `RespCondition`） |
| `mproxy/actions.go` | Handler 接口定义和便捷函数，包含流量监控、MITM 模式设置 |
| `mproxy/ctxt.go` | `Pcontext` 请求上下文，存储请求/响应状态 |
| `mproxy/websocket.go` | WebSocket 支持和连接劫持 |
| `mproxy/https_traffic.go` | HTTPS/MITM 流量统计 |
| `mproxy/tunnel_traffic.go` | 隧道模式流量统计 |
| `signer/` | 动态证书生成（MITM 模式） |
| `http1parser/` | HTTP/1 协议解析器 |

### Hook 机制

**请求 Hook**：修改请求或提前返回响应
```go
proxy.HookOnReq().DoFunc(func(req *http.Request, ctx *mproxy.Pcontext) (*http.Request, *http.Response) {
    // 返回 nil, nil 表示继续转发
    // 返回 resp != nil 表示直接返回该响应
    return req, nil
})
```

**CONNECT Hook**：控制 HTTPS 连接行为（MITM/隧道）
```go
proxy.HookOnReq().DoConnectFunc(func(host string, ctx *mproxy.Pcontext) (*ConnectAction, string) {
    return MitmConnect, host    // 使用 MITM 模式
    // return HTTPMitmConnect, host  // 强制 HTTP 模式处理
    // return OkConnect, host  // 普通隧道模式
})
```

### 条件过滤器

```go
// URL 精确匹配
proxy.HookOnReq(UrlHook("/api/user")).DoFunc(...)

// URL 正则匹配
proxy.HookOnReq(UrlRegHook(`^https://.*\.com/.*`)).DoFunc(...)

// 响应 Content-Type 匹配
proxy.HookOnResp(ContentTypeHook("application/json")).DoFunc(...)

// 双层条件：请求 + 响应
proxy.HookOnResp().OnRespByReq(UrlHook("/api")).DoFunc(...)
```

## 流量统计

流量统计通过包装 `req.Body` 和 `resp.Body` 实现，在 `actions.go:11` 的 `AddTrafficMonitor()` 中注册。

每个请求的 `Pcontext` 中都有一个 `TrafficCounter`，统计：
- 上行流量：`req_header` + `req_body`
- 下行流量：`resp_header` + `resp_body`
- 总计：`total`

流量统计会在请求完成后通过 `ctx.Log_P()` 输出日志。

## MITM 模式

MITM 模式通过动态生成证书实现 HTTPS 内容拦截。相关代码：
- `signer/signer.go`：动态证书生成
- `mproxy/https.go`：CONNECT 请求处理
- `mproxy/tls_cert.go`：内置 CA 证书（`Proxy_ManCa`）

### 连接模式选择
通过 `ConnectAction` 常量控制连接行为：
- `OkConnect`：普通 HTTPS 隧道模式
- `MitmConnect`：TLS MITM 解密模式（需要客户端信任 CA 证书）
- `HTTPMitmConnect`：强制按 HTTP 处理（用于纯 HTTP 站点）
- `ConnectHijack`：连接劫持
- `ConnectProxyAuthHijack`：代理认证劫持


## WebSocket 支持

WebSocket 支持通过 `websocket.go` 实现：
- 检测 WebSocket 握手：`isWebSocketHandshake()` 检查 `Connection: upgrade` 和 `Upgrade: websocket` 头
- 连接劫持：`hijackConnection()` 获取原始 TCP 连接
- 双向转发：`proxyWebsocket()` 实现客户端和服务端之间的数据双向转发


## VS Code 调试配置

使用 `.vscode/launch.json` 配置调试：
- 按 `F5` 启动调试
- 断点设置在 Go 代码中
- 预启动任务：`go: build for debug`（由 `.vscode/tasks.json` 定义）
- 调试目标：`use_demo/filter/test_bin`

## 开发注意事项

1. **修改核心转发逻辑**时需注意：`filterRequest` 在 `RoundTrip` 之前，`filterResponse` 在 `RoundTrip` 之后、`io.Copy` 之前

2. **流量统计**依赖 Body 包装器（`reqBodyReader` 和 `TrafficCounter`），直接修改 `req.Body` 或 `resp.Body` 会影响统计。包装逻辑在 `actions.go:11-68`

3. **WebSocket 握手**需要保留 `Connection: upgrade` 头部，检测逻辑在 `websocket.go:21-24`

4. **HTTP/2 支持**：`proxy.AllowHTTP2 = false` 可禁用 HTTP/2（当前默认禁用）

5. **调试模式**：`proxy.Verbose = true` 输出详细日志；`proxy.KeepHeader = false` 不保留代理头部

6. **证书缓存**：MITM 模式下使用 `ctx.certStore` 缓存生成的证书，避免重复签名（`https.go:100-110`）

7. **IPv6 地址处理**：`stripPort()` 函数同时支持 IPv4 和 IPv6 地址格式（`https.go:66-86`）

8. **连接复用**：通过 `httptrace.GotConn` 可以监控连接是否被复用，有助于调试性能问题

