# MITM 中间人代理界面设计方案

## 核心理念

**保证能够完全清晰地显示中间人解密的数据** - 即后端发送的完整 HTTP 交换记录。

---

## 一、界面布局

```
┌─────────────────────────────────────────────────────────┐
│  侧边栏              主内容区                           │
│  ┌────────┐        ┌───────────────────────────────┐   │
│  │ 概览   │        │  MITM 中间人代理              │   │
│  │详细连接│        │                               │   │
│  │ 日志   │        │  ┌─────────────────────────┐  │   │
│  │ MITM ✓ │        │  │ 搜索栏                 │  │   │
│  └────────┘        │  │ [搜索框]               │  │   │
│                    │  └─────────────────────────┘  │   │
│                    │  ┌─────────────────────────┐  │   │
│                    │  │ HTTP交换记录表格        │  │   │
│                    │  │ (可排序、可滚动)        │  │   │
│                    │  └─────────────────────────┘  │   │
│                    └───────────────────────────────┘   │
└─────────────────────────────────────────────────────────┘
```

### 两大区域

| 区域       | 说明                       | 样式参考                |
| ---------- | -------------------------- | ----------------------- |
| **搜索栏** | 搜索框，支持多字段过滤     | Connections.vue:385-400 |
| **表格区** | HTTP交换记录列表（可排序） | Connections.vue:423-630 |

---

## 二、表格列设计

| 列名   | 数据源                | 可排序 | 展示方式                          |
| ------ | --------------------- | ------ | --------------------------------- |
| ID     | `data.id`             | ✓      | 数值                              |
| 会话ID | `data.sessionId`      | ✓      | 数值                              |
| 父ID   | `data.parentId`       | ✓      | 数值（0或null显示"-"）            |
| 时间   | `data.time`           | ✓      | 格式化 `YYYY-MM-DD HH:mm:ss`      |
| 方法   | `request.method`      | ✓      | 徽章样式（GET/POST等）            |
| URL    | `request.url`         | ✗      | 文本（可截断，hover显示完整）     |
| Host   | `request.host`        | ✓      | 文本                              |
| 状态码 | `response.statusCode` | ✓      | 徽章样式（无响应显示"-"）         |
| 时长   | `data.duration`       | ✓      | `150ms` / `1.5s`                  |
| 上传   | `request.sumSize`    | ✓      | 格式化字节 `1.23 KB`              |
| 下载   | `response.sumSize`   | ✓      | 格式化字节 `5.67 MB`              |
| 错误   | `data.error`          | ✓      | 文本（空显示"-"，有错误红色高亮） |

### 状态码徽章配色

| 状态码范围     | 背景色                     | 文字色    |
| -------------- | -------------------------- | --------- |
| 2xx 成功       | `rgba(40, 167, 69, 0.2)`   | `#28a745` |
| 3xx 重定向     | `rgba(0, 123, 255, 0.2)`   | `#007bff` |
| 4xx 客户端错误 | `rgba(255, 193, 7, 0.2)`   | `#ffc107` |
| 5xx 服务器错误 | `rgba(220, 53, 69, 0.2)`   | `#dc3545` |
| 无响应         | `rgba(108, 117, 125, 0.2)` | `#6c757d` |

### 错误列样式

| 条件               | 样式                                 |
| ------------------ | ------------------------------------ |
| 无错误（空字符串） | 灰色文本 `"-"`                       |
| 有错误             | 红色高亮 `#dc3545`，显示完整错误信息 |

---

## 三、数据结构设计

### 后端接口格式

```json
{
  "type": "mitm_exchange",
  "data": {
    "id": 42,
    "sessionId": 156,
    "parentId": 150,
    "time": 1706500000000,
    "duration": 150,
    "error": "",
    "request": {
      "method": "GET",
      "url": "https://example.com/api/users",
      "host": "example.com",
      "header": {
        "User-Agent": ["Mozilla/5.0..."],
        "Accept": ["application/json"]
      },
      "sumSize": 0
    },
    "response": {
      "statusCode": 200,
      "status": "200 OK",
      "header": {
        "Content-Type": ["application/json"]
      },
      "sumSize": 1234
    }
  }
}
```

### 订阅主题

```javascript
// WebSocket 订阅请求
{
  "action": "subscribe",
  "topics": ["mitm_detail"]  // 注意：主题名称是 mitm_detail
}
```

### 前端存储结构

```javascript
// websocket.js 新增状态
const mitmExchanges = ref([])          // MITM 交换记录列表
const mitmSubscribers = ref(new Set()) // MITM 订阅者回调
const MAX_MITM_EXCHANGES = 1000        // 最大存储条数
```

### 数据标准化（推荐）

后端数据扁平化后便于快速访问和显示：

```javascript
// 标准化后的单条记录
{
  id: 42,
  sessionId: 156,
  parentId: 150,
  time: 1706500000000,
  duration: 150,
  error: "",

  // 请求相关
  method: "GET",
  url: "https://example.com/api/users",
  host: "example.com",
  requestHeaders: {"User-Agent": ["Mozilla/5.0..."], "Accept": ["application/json"]},
  requestSize: 0,

  // 响应相关
  statusCode: 200,
  status: "200 OK",
  responseHeaders: {"Content-Type": ["application/json"]},
  responseSize: 1234,

  // 衍生属性
  hasResponse: true,
  hasError: false
}
```

---

## 四、WebSocket 订阅机制扩展

### 需要修改的文件

**`src/stores/websocket.js`**

| 修改点              | 位置        | 内容                                                     |
| ------------------- | ----------- | -------------------------------------------------------- |
| 新增状态            | 第19行后    | `mitmExchanges`, `mitmSubscribers`, `MAX_MITM_EXCHANGES` |
| 更新订阅            | 第9-14行    | subscriptions 添加 `mitm: true`                          |
| 更新subscribe()     | 第85-96行   | topics 添加 `'mitm_detail'`                              |
| 更新handleMessage() | 第123-135行 | 添加 `case 'mitm_exchange'` 分支                         |
| 新增函数            | 第175行后   | `handleMITMExchange(data)` 处理数据                      |
| 新增函数            | 第207行后   | `subscribeMITM(callback)` 订阅方法                       |
| 更新导出            | 第217-232行 | 添加 `mitmExchanges`, `subscribeMITM`                    |

### WebSocket 消息处理流程

```
后端推送 { type: "mitm_exchange", data: {...} }
    ↓
handleMessage(msg) 识别类型
    ↓
handleMITMExchange(data) 标准化并存储
    ↓
mitmExchanges.push(exchange) 添加到列表
    ↓
mitmSubscribers.forEach(cb) 通知订阅者
    ↓
组件重新渲染
```

### 关键实现点

```javascript
// handleMITMExchange 函数核心逻辑
function handleMITMExchange(data) {
  const exchange = {
    ...data,
    // 扁平化 request
    method: data.request?.method,
    url: data.request?.url,
    host: data.request?.host,
    requestHeaders: data.request?.header || {},
    requestSize: data.request?.sumSize || 0,

    // 扁平化 response
    statusCode: data.response?.statusCode,
    status: data.response?.status,
    responseHeaders: data.response?.header || {},
    responseSize: data.response?.sumSize || 0,

    // 衍生属性
    hasResponse: !!(data.response && data.response.statusCode),
    hasError: !!data.error
  }

  mitmExchanges.value.push(exchange)

  // 限制最大数量
  if (mitmExchanges.value.length > MAX_MITM_EXCHANGES) {
    mitmExchanges.value.shift()
  }

  // 通知订阅者
  mitmSubscribers.value.forEach(cb => cb(exchange))
}
```

---

## 五、搜索功能设计

### 搜索字段

| 字段     | 数据源                | 说明            |
| -------- | --------------------- | --------------- |
| URL      | `request.url`         | 完整URL模糊匹配 |
| Host     | `request.host`        | 主机名模糊匹配  |
| 方法     | `request.method`      | GET/POST等      |
| 状态码   | `response.statusCode` | 数字搜索        |
| 会话ID   | `sessionId`           | 数字搜索        |
| 错误信息 | `error`               | 错误文本匹配    |

### 搜索逻辑

```
搜索输入 → 转小写
    ↓
遍历 mitmExchanges
    ↓
匹配任一字段 → 包含在结果
    ↓
返回过滤后列表
```

---

## 六、需要修改的文件清单

| 文件                      | 修改内容                                            | 优先级 |
| ------------------------- | --------------------------------------------------- | ------ |
| `src/stores/websocket.js` | 添加 MITM 状态、订阅机制（`mitm_detail`）、处理函数 | 🔴 高   |
| `src/views/MITM.vue`      | 实现完整 MITM 界面组件（搜索+表格）                 | 🔴 高   |

### 参考文件（无需修改）

- `src/views/Connections.vue` - 样式和布局参考
- `src/views/Logs.vue` - 过滤器设计参考

---

## 七、实现验证

### 测试步骤

1. 启动开发服务器：`npm run dev`
2. 登录后导航到 MITM 页面
3. 验证表格正确显示 HTTP 交换记录（包含所有字段）
4. 输入搜索关键词，验证过滤功能（URL、Host、方法、状态码、会话ID、错误）
5. 点击表头，验证排序功能
6. 观察 WebSocket 实时更新，验证新记录自动添加

### 预期效果

- 界面风格与 Connections.vue 保持一致（暗色主题 + 金棕色主题色）
- 完整清晰显示后端推送的 MITM 数据（请求+响应+元数据）
- 有错误的请求红色高亮显示
- 搜索和排序功能正常工作
- 无控制台错误