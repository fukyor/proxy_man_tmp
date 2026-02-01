<template>
  <div class="mitm">
    <h1>MITM</h1>

    <!-- 统计栏 -->
    <div class="stats-section">
      <div class="stat-item">
        <span class="label">总记录:</span>
        <span class="value">{{ totalRecords }}</span>
      </div>
      <div class="stat-item">
        <span class="label">成功:</span>
        <span class="value success">{{ successRecords }}</span>
      </div>
      <div class="stat-item">
        <span class="label">错误:</span>
        <span class="value error">{{ errorRecords }}</span>
      </div>
    </div>

    <!-- 操作栏 -->
    <div class="controls">
      <div class="search-group">
        <input
          type="text"
          v-model="searchQuery"
          placeholder="搜索 URL、Host、方法、状态码..."
          class="search-input"
        />
      </div>
      <button @click="handleClearRecords" class="btn-clear">清除记录</button>
    </div>

    <!-- 表格区 -->
    <div class="table-container">
      <table class="mitm-table">
        <thead>
          <tr>
            <th @click="sortBy('id')" :class="getSortClass('id')">
              ID {{ getSortIcon('id') }}
            </th>
            <th @click="sortBy('sessionId')" :class="getSortClass('sessionId')">
              会话ID {{ getSortIcon('sessionId') }}
            </th>
            <th @click="sortBy('parentId')" :class="getSortClass('parentId')">
              父ID {{ getSortIcon('parentId') }}
            </th>
            <th @click="sortBy('time')" :class="getSortClass('time')">
              时间 {{ getSortIcon('time') }}
            </th>
            <th @click="sortBy('method')" :class="getSortClass('method')">
              方法 {{ getSortIcon('method') }}
            </th>
            <th @click="sortBy('host')" :class="getSortClass('host')">
              Host {{ getSortIcon('host') }}
            </th>
            <th>URL</th>
            <th @click="sortBy('statusCode')" :class="getSortClass('statusCode')">
              状态码 {{ getSortIcon('statusCode') }}
            </th>
            <th @click="sortBy('duration')" :class="getSortClass('duration')">
              时长 {{ getSortIcon('duration') }}
            </th>
            <th @click="sortBy('requestSize')" :class="getSortClass('requestSize')">
              上传 {{ getSortIcon('requestSize') }}
            </th>
            <th @click="sortBy('responseSize')" :class="getSortClass('responseSize')">
              下载 {{ getSortIcon('responseSize') }}
            </th>
          </tr>
        </thead>
        <tbody>
          <tr v-if="filteredExchanges.length === 0">
            <td colspan="11" class="no-data">暂无记录</td>
          </tr>
          <template v-for="exchange in filteredExchanges" :key="exchange.id">
            <!-- 数据行 -->
            <tr @click="toggleExpand(exchange.id)" class="data-row">
              <td>
                <span class="expand-toggle">{{ isExpanded(exchange.id) ? '▼' : '▶' }}</span>
                {{ exchange.id }}
              </td>
              <td>{{ exchange.sessionId }}</td>
              <td>{{ exchange.parentId || '-' }}</td>
              <td>{{ formatTime(exchange.time) }}</td>
              <td>
                <span :class="['method-badge', `method-${exchange.method.toLowerCase()}`]">
                  {{ exchange.method }}
                </span>
              </td>
              <td>{{ exchange.host }}</td>
              <td class="url-cell" :title="exchange.url">{{ exchange.url }}</td>
              <td>
                <span :class="['status-badge', getStatusClass(exchange.statusCode)]">
                  {{ exchange.statusCode || '-' }}
                </span>
              </td>
              <td>{{ formatDuration(exchange.duration) }}</td>
              <td>{{ formatBytes(exchange.requestSize) }}</td>
              <td>{{ formatBytes(exchange.responseSize) }}</td>
            </tr>
            <!-- 详情行 -->
            <tr v-show="isExpanded(exchange.id)" class="detail-row">
              <td colspan="11">
                <div class="detail-content">
                  <!-- 请求头 -->
                  <div class="detail-section">
                    <h4 class="header-toggle" @click.stop="toggleHeader(exchange.id, 'request')">
                      <span class="toggle-icon">
                        {{ isHeaderExpanded(exchange.id, 'request') ? '▼' : '▶' }}
                      </span>
                      请求头 (Request Headers)
                    </h4>
                    <div v-show="isHeaderExpanded(exchange.id, 'request')" class="header-content">
                      <div
                        v-for="(value, key) in exchange.requestHeaders"
                        :key="key"
                        class="header-line"
                      >
                        <span class="header-key">{{ key }}:</span>
                        <span class="header-value">{{ value }}</span>
                      </div>
                      <div v-if="!exchange.requestHeaders || Object.keys(exchange.requestHeaders).length === 0" class="no-headers">
                        无请求头数据
                      </div>
                    </div>
                  </div>

                  <!-- 响应头 -->
                  <div class="detail-section">
                    <h4 class="header-toggle" @click.stop="toggleHeader(exchange.id, 'response')">
                      <span class="toggle-icon">
                        {{ isHeaderExpanded(exchange.id, 'response') ? '▼' : '▶' }}
                      </span>
                      响应头 (Response Headers)
                    </h4>
                    <div v-show="isHeaderExpanded(exchange.id, 'response')" class="header-content">
                      <div
                        v-for="(value, key) in exchange.responseHeaders"
                        :key="key"
                        class="header-line"
                      >
                        <span class="header-key">{{ key }}:</span>
                        <span class="header-value">{{ value }}</span>
                      </div>
                      <div v-if="!exchange.responseHeaders || Object.keys(exchange.responseHeaders).length === 0" class="no-headers">
                        无响应头数据
                      </div>
                    </div>
                  </div>

                  <!-- 错误（仅在有错误时显示） -->
                  <div v-if="exchange.error" class="detail-section error-section">
                    <h4>▼ 错误</h4>
                    <pre class="error-content">{{ exchange.error }}</pre>
                  </div>
                </div>
              </td>
            </tr>
          </template>
        </tbody>
      </table>
    </div>
  </div>
</template>

<script setup>
import { ref, computed, onMounted, onUnmounted } from 'vue'
import { useWebSocketStore } from '@/stores/websocket'

const wsStore = useWebSocketStore()

// 响应式数据
const searchQuery = ref('')
const sortField = ref('id')
const sortOrder = ref('desc')
const expandedRows = ref(new Set())
const expandedHeaders = ref(new Map())

let unsubscribeMITM = null

// 统计数据
const totalRecords = computed(() => wsStore.mitmExchanges.length)
const successRecords = computed(() =>
  wsStore.mitmExchanges.filter(e => e.statusCode >= 200 && e.statusCode < 400 && !e.error).length
)
const errorRecords = computed(() =>
  wsStore.mitmExchanges.filter(e => e.error || e.statusCode >= 400).length
)

// 搜索过滤
const searchedExchanges = computed(() => {
  if (!searchQuery.value.trim()) return wsStore.mitmExchanges

  const query = searchQuery.value.toLowerCase()
  return wsStore.mitmExchanges.filter(exchange => {
    return (
      exchange.url.toLowerCase().includes(query) ||
      exchange.host.toLowerCase().includes(query) ||
      exchange.method.toLowerCase().includes(query) ||
      (exchange.error && exchange.error.toLowerCase().includes(query)) ||
      String(exchange.statusCode).includes(query) ||
      String(exchange.sessionId).includes(query)
    )
  })
})

// 排序后的数据
const filteredExchanges = computed(() => {
  const sorted = [...searchedExchanges.value]

  sorted.sort((a, b) => {
    let aVal = a[sortField.value]
    let bVal = b[sortField.value]

    // 数值字段
    const numericFields = ['id', 'sessionId', 'parentId', 'time', 'statusCode', 'duration', 'requestSize', 'responseSize']
    if (numericFields.includes(sortField.value)) {
      aVal = Number(aVal) || 0
      bVal = Number(bVal) || 0
    } else {
      // 字符串字段
      aVal = String(aVal || '').toLowerCase()
      bVal = String(bVal || '').toLowerCase()
    }

    if (aVal < bVal) return sortOrder.value === 'asc' ? -1 : 1
    if (aVal > bVal) return sortOrder.value === 'asc' ? 1 : -1
    return 0
  })

  return sorted
})

// 排序方法
function sortBy(field) {
  if (sortField.value === field) {
    sortOrder.value = sortOrder.value === 'asc' ? 'desc' : 'asc'
  } else {
    sortField.value = field
    sortOrder.value = 'desc'
  }
}

// 获取排序样式类
function getSortClass(field) {
  return sortField.value === field ? 'sortable active' : 'sortable'
}

// 获取排序图标
function getSortIcon(field) {
  if (sortField.value !== field) return '⇅'
  return sortOrder.value === 'asc' ? '↑' : '↓'
}

// 获取状态码样式类
function getStatusClass(statusCode) {
  if (!statusCode) return 'status-none'
  if (statusCode >= 200 && statusCode < 300) return 'status-2xx'
  if (statusCode >= 300 && statusCode < 400) return 'status-3xx'
  if (statusCode >= 400 && statusCode < 500) return 'status-4xx'
  if (statusCode >= 500) return 'status-5xx'
  return 'status-none'
}

// 清除记录
function handleClearRecords() {
  wsStore.clearMitmExchanges()
}

// 格式化时间
function formatTime(time) {
  if (!time) return '-'
  const date = new Date(time)
  const hours = String(date.getHours()).padStart(2, '0')
  const minutes = String(date.getMinutes()).padStart(2, '0')
  const seconds = String(date.getSeconds()).padStart(2, '0')
  const ms = String(date.getMilliseconds()).padStart(3, '0')
  return `${hours}:${minutes}:${seconds}.${ms}`
}

// 格式化时长
function formatDuration(duration) {
  if (!duration) return '-'
  if (duration < 1000) return `${duration}ms`
  return `${(duration / 1000).toFixed(2)}s`
}

// 格式化字节数
function formatBytes(bytes) {
  if (!bytes || bytes === 0) return '0 B'
  const k = 1024
  const sizes = ['B', 'KB', 'MB', 'GB']
  const i = Math.floor(Math.log(bytes) / Math.log(k))
  return (bytes / Math.pow(k, i)).toFixed(2) + ' ' + sizes[i]
}

// 展开/收起行
function toggleExpand(id) {
  if (expandedRows.value.has(id)) {
    expandedRows.value.delete(id)
  } else {
    expandedRows.value.add(id)
  }
}

// 判断是否展开
function isExpanded(id) {
  return expandedRows.value.has(id)
}

// 切换头信息展开/折叠
function toggleHeader(exchangeId, headerType) {
  if (!expandedHeaders.value.has(exchangeId)) {
    expandedHeaders.value.set(exchangeId, new Set())
  }
  const headersSet = expandedHeaders.value.get(exchangeId)
  if (headersSet.has(headerType)) {
    headersSet.delete(headerType)
  } else {
    headersSet.add(headerType)
  }
}

// 判断头信息是否展开
function isHeaderExpanded(exchangeId, headerType) {
  if (!expandedHeaders.value.has(exchangeId)) {
    return false  // 默认折叠
  }
  return expandedHeaders.value.get(exchangeId).has(headerType)
}

// 生命周期钩子
onMounted(() => {
  // 订阅 MITM 更新
  unsubscribeMITM = wsStore.subscribeMITM((exchange) => {
    // 新记录到达时的处理（如需要）
  })
})

onUnmounted(() => {
  // 取消订阅
  if (unsubscribeMITM) unsubscribeMITM()
})
</script>

<style scoped>
.mitm {
  padding: 20px;
}

h1 {
  color: #cba376;
  margin-bottom: 20px;
}

/* 统计栏 */
.stats-section {
  display: flex;
  gap: 30px;
  margin-bottom: 20px;
}

.stat-item {
  display: flex;
  align-items: center;
  gap: 10px;
}

.stat-item .label {
  color: #999;
  font-size: 0.9em;
}

.stat-item .value {
  color: #cba376;
  font-size: 1.1em;
  font-weight: bold;
}

.stat-item .value.success {
  color: #5cb85c;
}

.stat-item .value.error {
  color: #d9534f;
}

/* 操作栏 */
.controls {
  display: flex;
  justify-content: space-between;
  align-items: center;
  background: #2a2a2a;
  padding: 15px 20px;
  border-radius: 8px;
  margin-bottom: 20px;
}

.search-group {
  flex: 1;
  max-width: 500px;
}

.search-input {
  width: 100%;
  padding: 8px 12px;
  background: #1a1a1a;
  color: #cba376;
  border: 1px solid #444;
  border-radius: 4px;
  font-size: 0.9em;
}

.search-input:focus {
  outline: none;
  border-color: #cba376;
}

.btn-clear {
  padding: 8px 16px;
  background: #d9534f;
  color: white;
  border: none;
  border-radius: 4px;
  cursor: pointer;
  font-size: 0.9em;
  transition: background 0.3s;
}

.btn-clear:hover {
  background: #c9302c;
}

/* 表格区 */
.table-container {
  background: #2a2a2a;
  border-radius: 8px;
  padding: 20px;
  overflow-x: auto;
}

.mitm-table {
  width: 100%;
  border-collapse: collapse;
  color: #cba376;
  min-width: 1400px;
}

.mitm-table th {
  background: #1a1a1a;
  padding: 12px;
  text-align: left;
  font-weight: 600;
  border-bottom: 2px solid #cba376;
  white-space: nowrap;
}

.mitm-table th.sortable {
  cursor: pointer;
  user-select: none;
  transition: background 0.2s;
}

.mitm-table th.sortable:hover {
  background: #252525;
}

.mitm-table th.sortable.active {
  color: #fff;
}

.mitm-table td {
  padding: 10px 12px;
  border-bottom: 1px solid #3a3a3a;
}

.mitm-table tbody tr:hover {
  background: #333;
}

.url-cell {
  max-width: 300px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.no-data {
  text-align: center;
  color: #999;
  padding: 40px !important;
}

/* 方法徽章 */
.method-badge {
  display: inline-block;
  padding: 2px 8px;
  border-radius: 3px;
  font-size: 0.85em;
  font-weight: 600;
}

.method-get {
  background: rgba(92, 184, 92, 0.2);
  color: #5cb85c;
}

.method-post {
  background: rgba(91, 192, 222, 0.2);
  color: #5bc0de;
}

.method-put {
  background: rgba(240, 173, 78, 0.2);
  color: #f0ad4e;
}

.method-delete {
  background: rgba(217, 83, 79, 0.2);
  color: #d9534f;
}

.method-patch {
  background: rgba(138, 109, 195, 0.2);
  color: #8a6dc3;
}

.method-head,
.method-options,
.method-connect {
  background: rgba(153, 153, 153, 0.2);
  color: #999;
}

/* 状态码徽章 */
.status-badge {
  display: inline-block;
  padding: 2px 8px;
  border-radius: 3px;
  font-size: 0.85em;
  font-weight: 600;
}

.status-2xx {
  background: rgba(92, 184, 92, 0.2);
  color: #5cb85c;
}

.status-3xx {
  background: rgba(91, 192, 222, 0.2);
  color: #5bc0de;
}

.status-4xx {
  background: rgba(240, 173, 78, 0.2);
  color: #f0ad4e;
}

.status-5xx {
  background: rgba(217, 83, 79, 0.2);
  color: #d9534f;
}

.status-none {
  background: rgba(153, 153, 153, 0.2);
  color: #999;
}

/* 展开箭头 */
.expand-toggle {
  display: inline-block;
  margin-right: 8px;
  cursor: pointer;
  color: #cba376;
  user-select: none;
  font-size: 0.8em;
}

/* 数据行 */
.data-row {
  cursor: pointer;
}

.data-row:hover {
  background: #333 !important;
}

/* 详情行 */
.detail-row {
  background: #1e1e1e !important;
}

.detail-row:hover {
  background: #1e1e1e !important;
}

.detail-content {
  padding: 20px;
  color: #cba376;
}

.detail-section {
  margin-bottom: 20px;
}

.detail-section:last-child {
  margin-bottom: 0;
}

.detail-section h4 {
  color: #cba376;
  font-size: 1em;
  margin: 0 0 10px 0;
  font-weight: 600;
}

.detail-section pre {
  background: #1a1a1a;
  color: #a0a0a0;
  padding: 15px;
  border-radius: 4px;
  overflow-x: auto;
  margin: 0;
  font-family: 'Courier New', Courier, monospace;
  font-size: 0.85em;
  line-height: 1.5;
}

.error-section h4 {
  color: #d9534f;
}

.error-content {
  color: #dc3545 !important;
  background: rgba(217, 83, 79, 0.1) !important;
}

/* 滚动条样式 */
.table-container::-webkit-scrollbar {
  height: 8px;
}

.table-container::-webkit-scrollbar-track {
  background: #1a1a1a;
}

.table-container::-webkit-scrollbar-thumb {
  background: #444;
  border-radius: 4px;
}

.table-container::-webkit-scrollbar-thumb:hover {
  background: #555;
}

/* 头信息可折叠标题 */
.header-toggle {
  display: flex;
  align-items: center;
  gap: 8px;
  cursor: pointer;
  user-select: none;
  transition: color 0.2s;
}

.header-toggle:hover {
  color: #fff;
}

.toggle-icon {
  display: inline-block;
  font-size: 0.8em;
  color: #cba376;
}

/* 头信息内容容器 */
.header-content {
  background: #1a1a1a;
  padding: 12px 15px;
  border-radius: 4px;
  margin-top: 8px;
  max-height: 300px;
  overflow-y: auto;
}

/* 头信息行样式 */
.header-line {
  display: flex;
  padding: 4px 0;
  font-family: 'Consolas', 'Monaco', 'Courier New', monospace;
  font-size: 0.9em;
  line-height: 1.6;
  color: #d4d4d4;
  border-bottom: 1px solid #2a2a2a;
}

.header-line:last-child {
  border-bottom: none;
}

.header-key {
  color: #9cdcfe;
  font-weight: 500;
  min-width: 150px;
  flex-shrink: 0;
  word-break: break-word;
}

.header-value {
  color: #ce9178;
  font-weight: 400;
  word-break: break-all;
}

/* 无数据提示 */
.no-headers {
  color: #888;
  font-style: italic;
  padding: 8px 0;
  text-align: center;
}

/* 头信息滚动条 */
.header-content::-webkit-scrollbar {
  width: 6px;
}

.header-content::-webkit-scrollbar-track {
  background: #1a1a1a;
}

.header-content::-webkit-scrollbar-thumb {
  background: #444;
  border-radius: 3px;
}

.header-content::-webkit-scrollbar-thumb:hover {
  background: #555;
}
</style>
