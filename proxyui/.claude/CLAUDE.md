# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## 开发命令

```bash
npm install          # 安装依赖
npm run dev          # 启动开发服务器（Vite）
npm run build        # 构建生产版本
npm run preview      # 预览生产构建
npm run lint         # ESLint 检查并自动修复
npm run format       # Prettier 格式化 src/ 目录
```

## 技术栈

- **框架**: Vue 3 (Composition API + `<script setup>`)
- **构建工具**: Vite 7
- **状态管理**: Pinia
- **路由**: Vue Router 4
- **图表库**: Chart.js 4
- **Node 版本要求**: ^20.19.0 || >=22.12.0

## 项目架构

这是一个 WebSocket 客户端管理面板，用于连接和管理代理服务器（如 Clash/Mihomo）。

### 路由结构

- `/` - 登录页面 (`loginView.vue`)：配置 API 地址和密钥，连接到代理服务器
- `/dashboard` - 仪表盘布局 (`dashboard.vue`)：包含侧边栏导航
  - `/dashboard/overview` - 概览页面（流量统计 + 连接统计）
  - `/dashboard/logs` - 日志页面（日志统计）

### 路径别名

`@` 映射到 `./src` 目录

---

## 核心数据流架构（重点）

项目采用 **发布-订阅模式** 实现 WebSocket 实时数据流，核心在 `src/stores/websocket.js`。

### WebSocket Store 数据结构

```js
// 状态
isConnected      // 连接状态
subscriptions    // 订阅配置 { traffic, connections, logs, logLevel }

// 数据存储
trafficHistory   // 最近 60 秒流量历史，格式: { up, down, timestamp }
connections      // 当前连接列表，格式: Array<{ id, method, host, url, protocol, up, down }>
logs             // 日志列表，格式: Array<{ id, level, session, message, time }>

// 订阅者集合（Set 存储回调函数）
trafficSubscribers
connectionsSubscribers
logsSubscribers
```

### WebSocket 消息协议

连接建立后，客户端发送订阅消息：
```json
{
  "action": "subscribe",
  "topics": ["traffic", "connections", "logs"],
  "logLevel": "INFO"
}
```

服务器推送的消息格式：
```json
// 流量更新
{ "type": "traffic", "data": { "up": 1024, "down": 2048 } }

// 连接列表
{ "type": "connections", "data": [...] }

// 日志条目
{ "type": "log", "data": { "level": "INFO", "session": 1, "message": "...", "time": "..." } }
```

---

## 三大统计模块详解

### 1. 流量统计模块

**数据流**: WebSocket → Store.handleTraffic → trafficHistory → 订阅者回调

**关键文件**:
- `src/stores/websocket.js:141-149` - `handleTraffic()` 处理流量数据
- `src/views/Overview.vue:83-164` - Chart.js 实时图表渲染

**流量数据格式**:
```js
{ up: 1024, down: 2048, timestamp: 1737792000000 }
```

**组件使用方式**:
```js
// 订阅流量更新
const unsubscribe = wsStore.subscribeTraffic((data) => {
  // data: { up, down }
  updateChart(data)
})
```

**图表配置** (`Overview.vue:88-141`):
- Chart.js 折线图
- 两个数据集：上传（青色）和下载（红色）
- 固定 60 个数据点，超出自动左移
- Y 轴自动格式化字节单位 (B/KB/MB/GB)

### 2. 连接统计模块

**数据流**: WebSocket → Store.handleConnections → connections → 订阅者回调

**关键文件**:
- `src/stores/websocket.js:156-159` - `handleConnections()` 处理连接列表
- `src/views/Overview.vue:166-174` - 连接表格渲染

**连接数据格式**:
```js
{
  id: "conn-001",
  method: "GET",
  host: "example.com",
  url: "https://example.com/path",
  protocol: "HTTP",
  up: 1024,
  down: 2048
}
```

**组件使用方式**:
```js
// 订阅连接更新
const unsubscribe = wsStore.subscribeConnections((data) => {
  connections.value = data
})

// 关闭所有连接
wsStore.closeAllConnections()  // 发送 { action: 'closeAllConnections' }
```

### 3. 日志统计模块

**数据流**: WebSocket → Store.handleLog → logs → 订阅者回调

**关键文件**:
- `src/stores/websocket.js:166-173` - `handleLog()` 处理日志数据
- `src/views/Logs.vue:44-122` - 日志列表渲染和过滤

**日志数据格式**:
```js
{
  id: "random-id",
  level: "INFO",      // DEBUG | INFO | WARN | ERROR
  session: 1,
  message: "连接成功",
  time: "2024-01-25T10:30:00.000Z"
}
```

**日志级别过滤** (`Logs.vue:58-67`):
```js
const logLevels = { DEBUG: 0, INFO: 1, WARN: 2, ERROR: 3 }
// 只显示级别 >= selectedLevel 的日志
const filteredLogs = computed(() => {
  const minLevel = logLevels[selectedLevel.value]
  return wsStore.logs.filter(log => logLevels[log.level] >= minLevel)
})
```

**组件使用方式**:
```js
// 订阅日志更新
const unsubscribe = wsStore.subscribeLogs(() => {
  scrollToBottom()  // 新日志时自动滚动
})

// 更新订阅的日志级别（同步到服务器）
wsStore.updateSubscriptions({ logLevel: 'ERROR' })

// 清除所有日志
wsStore.clearLogs()
```

---

## API 层说明

`src/api/api.js` 提供基础 HTTP 和 WebSocket 工具，但核心数据流通过 WebSocket Store 管理。

### 配置对象格式
```js
{
  baseURL: 'http://127.0.0.1:8000',
  secret: '密钥'  // 可选，用于 Bearer Token 认证
}
```

## 编码规范

- 使用 Vue 3 Composition API + `<script setup>` 语法
- 使用 Context7 MCP 查询 Vue 3 文档以确保代码可靠性
- 使用 chrome-devtools MCP 进行浏览器操作测试
