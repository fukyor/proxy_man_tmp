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
- **Node 版本要求**: ^20.19.0 || >=22.12.0

## 项目架构

这是一个 WebSocket 客户端管理面板，用于连接和管理代理服务器（如 Clash/Mihomo）。

### 路由结构

- `/` - 登录页面 (`loginView.vue`)：配置 API 地址和密钥，连接到代理服务器
- `/dashboard` - 仪表盘布局 (`dashboard.vue`)：包含侧边栏导航
  - `/dashboard/overview` - 概览页面
  - `/dashboard/logs` - 日志页面

### 核心模块

**API 层** (`src/api/api.js`):
- `fetchAPI(config, endpoint, options)` - 统一的 HTTP 请求封装，支持 Bearer Token 认证
- `buildWebSocketURL(config, endpoint)` - 将 HTTP URL 转换为 WebSocket URL（http→ws, https→wss）
- `connectToLogs(config, onMessage, onError)` - 建立 WebSocket 连接获取实时日志流

**配置对象格式**:
```js
{
  baseURL: 'http://127.0.0.1:9090',  // API 地址
  secret: '密钥'                      // 可选，用于 Authorization header
}
```

### 路径别名

`@` 映射到 `./src` 目录

## 编码规范

- 使用 Context7 MCP 查询库文档以确保代码可靠性
- 使用 chrome-devtools 进行浏览器操作
- 所有对话和文件内容使用中文
