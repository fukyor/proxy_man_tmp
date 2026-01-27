# AGENTS.md

用途：快速了解 proxy_man_tmp 代码库结构与核心模块。

## 仓库结构
- proxy_man/：Go 代理服务主程序（重点）
- proxyui/：前端界面（本次未分析）

## 入口与主流程
- proxy_man/main.go：程序入口、参数、启用流量/日志钩子、启动 WebSocket 控制服务
- proxy_man/mproxy/：核心代理逻辑、Hook、流量统计、连接统计、日志
- proxy_man/proxysocket/：WebSocket Hub，推送流量/连接/日志数据

## 请求处理流程（高层）
- HTTP：ServeHTTP -> MyHttpHandle -> filterRequest -> RoundTrip -> filterResponse -> io.Copy
- HTTPS：ServeHTTP -> MyHttpsHandle -> CONNECT 分支
  - ConnectAccept（隧道）或 ConnectMitm（TLS MITM）或 ConnectHTTPMitm

## 流量统计模块
- 文件：proxy_man/mproxy/https_traffic.go，proxy_man/mproxy/tunnel_traffic.go，proxy_man/mproxy/actions.go
- 全局计数：GlobalTrafficUp / GlobalTrafficDown（原子计数）
- HTTP/MITM 路径：
  - AddTrafficMonitor 包装 req.Body 和 resp.Body
  - 请求头大小通过 httputil.DumpRequest 统计
  - 响应头大小通过 httputil.DumpResponse 统计
- 隧道路劲：
  - tunnelTrafficClient 统计 TCP Read/Write 字节数
- WebSocket 推送：
  - proxy_man/proxysocket/hub.go 的 StartTrafficPusher 每秒推送增量

## 连接统计模块
- 文件：proxy_man/mproxy/connections.go，proxy_man/mproxy/core_proxy.go
- 存储：CoreHttpServer.Connections（sync.Map，session -> ConnectionInfo）
- 注册位置：
  - HTTP：proxy_man/mproxy/http.go
  - HTTPS-Tunnel：proxy_man/mproxy/https.go（ConnectAccept）
  - HTTPS-MITM / HTTP-MITM：proxy_man/mproxy/https.go
- 清理时机：
  - HTTP：响应体 onClose
  - 隧道：onUpdate（连接关闭）
  - MITM：请求结束 defer 清理
- WebSocket 推送：
  - proxy_man/proxysocket/hub.go 的 StartConnectionPusher 每 2 秒推送

## 日志管理模块
- 文件：proxy_man/mproxy/logs.go，proxy_man/mproxy/ctxt.go
- LogCollector 包装 Logger，解析日志并写入 LogChan（非阻塞）
- Pcontext.Log_P / WarnP 统一带 Session 与级别前缀
- main.go 安装 LogCollector
- WebSocket 推送：proxysocket/hub.go 的 StartLogPusher，支持级别过滤

## 备注
- Session ID 通过 CoreHttpServer 的原子自增生成。
- MITM 动态证书：proxy_man/signer/，TLSConfigFromCA 在 https.go。
- WebSocket 主题：traffic / connections / logs。
