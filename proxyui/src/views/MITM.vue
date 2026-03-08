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

    <!-- 表格区：唯一滚动容器 -->
    <div class="table-container">
      <!-- 虚拟滚动区域（同时处理水平+垂直滚动） -->
      <div class="mitm-scroller" ref="scrollerRef">
        <!-- 固定表头（sticky 吸顶） -->
        <div class="mitm-grid-row thead-row">
          <div @click="sortBy('id')" :class="['th', getSortClass('id')]">
            ID {{ getSortIcon('id') }}
          </div>
          <div @click="sortBy('sessionId')" :class="['th', getSortClass('sessionId')]">
            会话ID {{ getSortIcon('sessionId') }}
          </div>
          <div @click="sortBy('parentId')" :class="['th', getSortClass('parentId')]">
            父ID {{ getSortIcon('parentId') }}
          </div>
          <div @click="sortBy('time')" :class="['th', getSortClass('time')]">
            时间 {{ getSortIcon('time') }}
          </div>
          <div @click="sortBy('method')" :class="['th', getSortClass('method')]">
            方法 {{ getSortIcon('method') }}
          </div>
          <div @click="sortBy('host')" :class="['th', getSortClass('host')]">
            Host {{ getSortIcon('host') }}
          </div>
          <div class="th">URL</div>
          <div @click="sortBy('statusCode')" :class="['th', getSortClass('statusCode')]">
            状态码 {{ getSortIcon('statusCode') }}
          </div>
          <div @click="sortBy('duration')" :class="['th', getSortClass('duration')]">
            时长 {{ getSortIcon('duration') }}
          </div>
          <div @click="sortBy('requestSize')" :class="['th', getSortClass('requestSize')]">
            上传 {{ getSortIcon('requestSize') }}
          </div>
          <div @click="sortBy('responseSize')" :class="['th', getSortClass('responseSize')]">
            下载 {{ getSortIcon('responseSize') }}
          </div>
        </div>

        <!-- 空数据提示 -->
        <div v-if="filteredExchanges.length === 0" class="no-data">
          暂无记录
        </div>

        <div :style="{ position: 'relative', height: totalSize + 'px' }">
            <div
              v-for="virtualRow in virtualRows"
              :key="virtualRow.key"
              :ref="(el) => { if (el) virtualizer.measureElement(el) }"
              :data-index="virtualRow.index"
              :style="{
                position: 'absolute',
                top: 0,
                left: 0,
                width: '100%',
                transform: `translateY(${virtualRow.start}px)`
              }"
            >
              <!-- 数据行 -->
              <div
                class="mitm-grid-row data-row"
                @click="toggleExpand(filteredExchanges[virtualRow.index].id)"
              >
                <div class="td">
                  <span class="expand-toggle">
                    {{ isExpanded(filteredExchanges[virtualRow.index].id) ? '▼' : '▶' }}
                  </span>
                  {{ filteredExchanges[virtualRow.index].id }}
                </div>
                <div class="td">{{ filteredExchanges[virtualRow.index].sessionId }}</div>
                <div class="td">{{ filteredExchanges[virtualRow.index].parentId || '-' }}</div>
                <div class="td">{{ formatTime(filteredExchanges[virtualRow.index].time) }}</div>
                <div class="td">
                  <span :class="['method-badge', `method-${filteredExchanges[virtualRow.index].method.toLowerCase()}`]">
                    {{ filteredExchanges[virtualRow.index].method }}
                  </span>
                </div>
                <div class="td">{{ filteredExchanges[virtualRow.index].host }}</div>
                <div class="td url-cell" :title="filteredExchanges[virtualRow.index].url">
                  {{ filteredExchanges[virtualRow.index].url }}
                </div>
                <div class="td">
                  <span :class="['status-badge', getStatusClass(filteredExchanges[virtualRow.index].statusCode)]">
                    {{ filteredExchanges[virtualRow.index].statusCode || '-' }}
                  </span>
                </div>
                <div class="td">{{ formatDuration(filteredExchanges[virtualRow.index].duration) }}</div>
                <div class="td">{{ formatBytes(filteredExchanges[virtualRow.index].requestSize) }}</div>
                <div class="td">{{ formatBytes(filteredExchanges[virtualRow.index].responseSize) }}</div>
              </div>

              <!-- 详情面板 -->
              <div
                v-if="isExpanded(filteredExchanges[virtualRow.index].id)"
                class="detail-content"
              >
                <!-- 请求头 -->
                <div class="detail-section">
                  <h4 class="header-toggle" @click.stop="toggleHeader(filteredExchanges[virtualRow.index].id, 'request')">
                    <span class="toggle-icon">
                      {{ isHeaderExpanded(filteredExchanges[virtualRow.index].id, 'request') ? '▼' : '▶' }}
                    </span>
                    请求头 (Request Headers)
                  </h4>
                  <div v-show="isHeaderExpanded(filteredExchanges[virtualRow.index].id, 'request')" class="header-content">
                    <div
                      v-for="(value, key) in filteredExchanges[virtualRow.index].requestHeaders"
                      :key="key"
                      class="header-line"
                    >
                      <span class="header-key">{{ key }}:</span>
                      <span class="header-value">{{ value }}</span>
                    </div>
                    <div v-if="!filteredExchanges[virtualRow.index].requestHeaders || Object.keys(filteredExchanges[virtualRow.index].requestHeaders).length === 0" class="no-headers">
                      无请求头数据
                    </div>
                  </div>
                </div>

                <!-- 响应头 -->
                <div class="detail-section">
                  <h4 class="header-toggle" @click.stop="toggleHeader(filteredExchanges[virtualRow.index].id, 'response')">
                    <span class="toggle-icon">
                      {{ isHeaderExpanded(filteredExchanges[virtualRow.index].id, 'response') ? '▼' : '▶' }}
                    </span>
                    响应头 (Response Headers)
                  </h4>
                  <div v-show="isHeaderExpanded(filteredExchanges[virtualRow.index].id, 'response')" class="header-content">
                    <div
                      v-for="(value, key) in filteredExchanges[virtualRow.index].responseHeaders"
                      :key="key"
                      class="header-line"
                    >
                      <span class="header-key">{{ key }}:</span>
                      <span class="header-value">{{ value }}</span>
                    </div>
                    <div v-if="!filteredExchanges[virtualRow.index].responseHeaders || Object.keys(filteredExchanges[virtualRow.index].responseHeaders).length === 0" class="no-headers">
                      无响应头数据
                    </div>
                  </div>
                </div>

                <!-- Body 下载 -->
                <div class="detail-section body-download-section">
                  <h4>Body 下载</h4>
                  <div class="download-buttons">
                    <button
                      class="btn-download"
                      :disabled="!filteredExchanges[virtualRow.index].reqBodyUploaded"
                      @click.stop="downloadBody(filteredExchanges[virtualRow.index].reqBodyKey)"
                    >
                      Request Body
                      <span v-if="filteredExchanges[virtualRow.index].reqBodyUploaded" class="body-size">
                        ({{ formatBytes(filteredExchanges[virtualRow.index].reqBodySize) }})
                      </span>
                      <span v-else class="body-unavailable">不可用</span>
                    </button>
                    <button
                      class="btn-download"
                      :disabled="!filteredExchanges[virtualRow.index].respBodyUploaded"
                      @click.stop="downloadBody(filteredExchanges[virtualRow.index].respBodyKey)"
                    >
                      Response Body
                      <span v-if="filteredExchanges[virtualRow.index].respBodyUploaded" class="body-size">
                        ({{ formatBytes(filteredExchanges[virtualRow.index].respBodySize) }})
                      </span>
                      <span v-else class="body-unavailable">不可用</span>
                    </button>
                  </div>
                </div>

                <!-- 异常信息（仅在有任一错误时显示） -->
                <div v-if="hasAnyError(filteredExchanges[virtualRow.index])" class="detail-section error-section">
                  <h4>▼ 异常信息</h4>
                  <div class="error-list">
                    <div v-if="filteredExchanges[virtualRow.index].error" class="error-item">
                      <span class="error-label">[代理核心异常]</span>
                      <pre class="error-text">{{ filteredExchanges[virtualRow.index].error }}</pre>
                    </div>
                    <div v-if="filteredExchanges[virtualRow.index].reqBodyError" class="error-item">
                      <span class="error-label">[Request 上传异常]</span>
                      <pre class="error-text">{{ filteredExchanges[virtualRow.index].reqBodyError }}</pre>
                    </div>
                    <div v-if="filteredExchanges[virtualRow.index].respBodyError" class="error-item">
                      <span class="error-label">[Response 上传异常]</span>
                      <pre class="error-text">{{ filteredExchanges[virtualRow.index].respBodyError }}</pre>
                    </div>
                  </div>
                </div>
              </div>
            </div>
          </div>
        </div>
      </div>
    </div>
</template>

<script setup>
import { ref, computed, onMounted, onUnmounted } from 'vue'
import { useVirtualizer } from '@tanstack/vue-virtual'
import { useWebSocketStore } from '@/stores/websocket'

const wsStore = useWebSocketStore()

// 响应式数据
const searchQuery = ref('')
const sortField = ref('id')
const sortOrder = ref('desc')
const expandedRows = ref(new Set())
const expandedHeaders = ref(new Map())
const scrollerRef = ref(null)

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

    const numericFields = ['id', 'sessionId', 'parentId', 'time', 'statusCode', 'duration', 'requestSize', 'responseSize']
    if (numericFields.includes(sortField.value)) {
      aVal = Number(aVal) || 0
      bVal = Number(bVal) || 0
    } else {
      aVal = String(aVal || '').toLowerCase()
      bVal = String(bVal || '').toLowerCase()
    }

    if (aVal < bVal) return sortOrder.value === 'asc' ? -1 : 1
    if (aVal > bVal) return sortOrder.value === 'asc' ? 1 : -1
    return 0
  })

  return sorted
})

// 虚拟滚动器（动态高度）
const virtualizer = useVirtualizer(
  computed(() => ({
    count: filteredExchanges.value.length,
    getScrollElement: () => scrollerRef.value,
    estimateSize: () => 60,
    overscan: 10,
    getItemKey: (index) => filteredExchanges.value[index].id,
  }))
)

const virtualRows = computed(() => virtualizer.value.getVirtualItems())
const totalSize = computed(() => virtualizer.value.getTotalSize())

// 排序方法
function sortBy(field) {
  if (sortField.value === field) {
    sortOrder.value = sortOrder.value === 'asc' ? 'desc' : 'asc'
  } else {
    sortField.value = field
    sortOrder.value = 'desc'
  }
}

function getSortClass(field) {
  return sortField.value === field ? 'sortable active' : 'sortable'
}

function getSortIcon(field) {
  if (sortField.value !== field) return '⇅'
  return sortOrder.value === 'asc' ? '↑' : '↓'
}

function getStatusClass(statusCode) {
  if (!statusCode) return 'status-none'
  if (statusCode >= 200 && statusCode < 300) return 'status-2xx'
  if (statusCode >= 300 && statusCode < 400) return 'status-3xx'
  if (statusCode >= 400 && statusCode < 500) return 'status-4xx'
  if (statusCode >= 500) return 'status-5xx'
  return 'status-none'
}

function handleClearRecords() {
  wsStore.clearMitmExchanges()
}

function formatTime(time) {
  if (!time) return '-'
  const date = new Date(time)
  const hours = String(date.getHours()).padStart(2, '0')
  const minutes = String(date.getMinutes()).padStart(2, '0')
  const seconds = String(date.getSeconds()).padStart(2, '0')
  return `${hours}:${minutes}:${seconds}`
}

function formatDuration(duration) {
  if (!duration) return '-'
  if (duration < 1000) return `${duration}ms`
  return `${(duration / 1000).toFixed(2)}s`
}

function formatBytes(bytes) {
  if (!bytes || bytes <= 0) return '0 B'
  const k = 1024
  const sizes = ['B', 'KB', 'MB', 'GB', 'TB']
  const i = Math.min(Math.floor(Math.log(bytes) / Math.log(k)), sizes.length - 1)
  return (bytes / Math.pow(k, i)).toFixed(2) + ' ' + sizes[i]
}

function toggleExpand(id) {
  const set = expandedRows.value
  if (set.has(id)) {
    set.delete(id)
  } else {
    set.add(id)
  }
  // 触发响应式更新（ResizeObserver 自动处理高度变化）
  expandedRows.value = new Set(set)
}

function isExpanded(id) {
  return expandedRows.value.has(id)
}

function toggleHeader(exchangeId, headerType) {
  const map = expandedHeaders.value
  if (!map.has(exchangeId)) {
    map.set(exchangeId, new Set())
  }
  const headersSet = map.get(exchangeId)
  if (headersSet.has(headerType)) {
    headersSet.delete(headerType)
  } else {
    headersSet.add(headerType)
  }
  // 触发响应式更新（ResizeObserver 自动处理高度变化）
  expandedHeaders.value = new Map(map)
}

function isHeaderExpanded(exchangeId, headerType) {
  if (!expandedHeaders.value.has(exchangeId)) return false
  return expandedHeaders.value.get(exchangeId).has(headerType)
}

// 判断是否有任一错误
function hasAnyError(exchange) {
  return !!(exchange.error || exchange.reqBodyError || exchange.respBodyError)
}

async function downloadBody(bodyKey) {
  if (!bodyKey) return
  const base = wsStore.apiUrl
  const url = `${base}/api/storage/download?key=${encodeURIComponent(bodyKey)}`
  try {
    const resp = await fetch(url)
    const result = await resp.json()
    if (result.code !== 0) {
      alert(result.message)
      return
    }
    window.open(result.data.downloadUrl, '_blank')
  } catch (e) {
    alert('下载请求失败: ' + e.message)
  }
}

onMounted(() => {
  unsubscribeMITM = wsStore.subscribeMITM(() => {})
})

onUnmounted(() => {
  if (unsubscribeMITM) unsubscribeMITM()
})
</script>

<style scoped>
.mitm {
  padding: 20px;
  height: calc(100vh - 40px);
  display: flex;
  flex-direction: column;
}

h1 {
  color: #cba376;
  margin-bottom: 20px;
  flex-shrink: 0;
}

/* 统计栏 */
.stats-section {
  display: flex;
  gap: 30px;
  margin-bottom: 20px;
  flex-shrink: 0;
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
  flex-shrink: 0;
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
  box-sizing: border-box;
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

/* 表格区：不再自身滚动，由内部 .mitm-scroller 统一处理 */
.table-container {
  flex: 1;
  background: #2a2a2a;
  border-radius: 8px;
  overflow: hidden;
  display: flex;
  flex-direction: column;
  min-height: 0;
}

/* CSS Grid 行布局 */
.mitm-grid-row {
  display: grid;
  grid-template-columns: minmax(50px, 0.6fr) minmax(60px, 0.6fr) minmax(50px, 0.5fr) minmax(80px, 1fr) minmax(60px, 0.8fr) minmax(120px, 1.5fr) minmax(150px, 3fr) minmax(60px, 0.8fr) minmax(60px, 0.8fr) minmax(60px, 0.8fr) minmax(60px, 0.8fr);
  align-items: flex-start;
  color: #cba376;
}

/* 表头行 */
.thead-row {
  background: #1a1a1a;
  border-bottom: 2px solid #cba376;
  position: sticky;
  top: 0;
  z-index: 10;
  width: 100%;
}

.th {
  padding: 12px;
  text-align: left;
  font-weight: 600;
  white-space: nowrap;
}

.th.sortable {
  cursor: pointer;
  user-select: none;
  transition: background 0.2s;
}

.th.sortable:hover {
  background: #252525;
}

.th.sortable.active {
  color: #fff;
}

/* 数据单元格 */
.td {
  padding: 10px 12px;
  border-bottom: 1px solid #3a3a3a;
  overflow: hidden;
  white-space: normal;
  word-break: break-all;
}

/* 空数据 */
.no-data {
  text-align: center;
  color: #999;
  padding: 40px;
}

/* 数据行 */
.data-row {
  cursor: pointer;
}

.data-row:hover .td {
  background: #333;
}

/* URL 列截断 */
.url-cell {
  max-width: none;
  word-break: break-all;
}

/* 虚拟滚动容器（统一处理水平+垂直滚动） */
.mitm-scroller {
  flex: 1;
  overflow: auto;
  overscroll-behavior: contain;
  min-height: 0;
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

/* 详情内容面板 */
.detail-content {
  padding: 20px;
  background: #1e1e1e;
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

/* 异常信息区块 */
.error-section h4 {
  color: #d9534f;
}

.error-list {
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.error-item {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.error-label {
  color: #d9534f;
  font-weight: 600;
  font-size: 0.9em;
}

.error-text {
  background: rgba(217, 83, 79, 0.1);
  color: #dc3545;
  padding: 10px 12px;
  border-radius: 4px;
  margin: 0;
  font-family: 'Courier New', Courier, monospace;
  font-size: 0.85em;
  line-height: 1.5;
  white-space: pre-wrap;
  word-break: break-all;
  overflow-x: auto;
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
  overscroll-behavior: contain;
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

/* Body 下载按钮 */
.body-download-section h4 {
  margin-bottom: 12px;
}

.download-buttons {
  display: flex;
  gap: 12px;
}

.btn-download {
  padding: 6px 16px;
  background: rgba(203, 163, 118, 0.15);
  color: #cba376;
  border: 1px solid #cba376;
  border-radius: 4px;
  cursor: pointer;
  font-size: 0.85em;
  transition: all 0.2s;
}

.btn-download:hover:not(:disabled) {
  background: rgba(203, 163, 118, 0.3);
}

.btn-download:disabled {
  opacity: 0.4;
  cursor: not-allowed;
  border-color: #666;
  color: #666;
}

.body-size {
  color: #999;
  font-size: 0.9em;
}

.body-unavailable {
  color: #666;
  font-size: 0.85em;
}

/* 滚动条样式 */
.mitm-scroller::-webkit-scrollbar {
  width: 8px;
  height: 8px;
}

.mitm-scroller::-webkit-scrollbar-track {
  background: #1a1a1a;
}

.mitm-scroller::-webkit-scrollbar-thumb {
  background: #444;
  border-radius: 4px;
}

.mitm-scroller::-webkit-scrollbar-thumb:hover {
  background: #555;
}

@media screen and (max-width: 1200px) {
  .mitm-grid-row {
    grid-template-columns: minmax(40px, 0.5fr) minmax(80px, 1fr) minmax(50px, 0.8fr) minmax(100px, 1.5fr) minmax(120px, 3fr) minmax(50px, 0.8fr) minmax(50px, 0.8fr) minmax(50px, 0.8fr);
  }
  .mitm-grid-row > div:nth-child(2),
  .mitm-grid-row > div:nth-child(3),
  .mitm-grid-row > div:nth-child(9) {
    display: none;
  }
}
</style>
