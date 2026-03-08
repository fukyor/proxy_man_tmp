<template>
  <div class="connections history">
    <h1>历史记录</h1>

    <!-- 统计信息栏 -->
    <div class="stats-bar">
      <div class="stat-item">
        <span class="label">已归档历史连接:</span>
        <span class="value closed">{{ filteredHistoryConnections.length }}</span>
      </div>
    </div>

    <!-- 搜索和操作栏 -->
    <div class="actions-bar">
      <input
        v-model="searchQuery"
        type="text"
        placeholder="搜索 Host 或 URL..."
        class="search-input"
      />
      <button @click="handleClearHistory" class="btn-clear-history">清空历史</button>
    </div>

    <!-- 连接表格 -->
    <div class="table-section">
      <div class="conn-scroller" ref="historyScrollerRef">
        <!-- sticky 吸顶表头 -->
        <div class="conn-grid-row thead-row">
          <div class="th">ID</div>
          <div class="th">方法</div>
          <div class="th">Host</div>
          <div class="th">URL</div>
          <div class="th">协议</div>
          <div class="th">上传</div>
          <div class="th">下载</div>
        </div>

        <!-- 空数据提示 -->
        <div v-if="filteredHistoryConnections.length === 0" class="no-data">
          {{ searchQuery ? '未找到匹配的历史连接' : '暂无历史记录' }}
        </div>

        <!-- 虚拟高度容器 -->
        <div :style="{ position: 'relative', height: historyTotalSize + 'px' }">
          <div
            v-for="virtualRow in historyVirtualRows"
            :key="virtualRow.key"
            :data-index="virtualRow.index"
            :style="{
              position: 'absolute',
              top: 0,
              left: 0,
              width: '100%',
              transform: `translateY(${virtualRow.start}px)`
            }"
          >
            <div class="conn-grid-row data-row history-row">
              <div class="td">{{ filteredHistoryConnections[virtualRow.index].id }}</div>
              <div class="td">
                <span class="badge method-badge" :class="getMethodClass(filteredHistoryConnections[virtualRow.index].method)">
                  {{ filteredHistoryConnections[virtualRow.index].method }}
                </span>
              </div>
              <div class="td">{{ filteredHistoryConnections[virtualRow.index].host }}</div>
              <div class="td url-cell" :title="filteredHistoryConnections[virtualRow.index].url">
                {{ filteredHistoryConnections[virtualRow.index].url }}
              </div>
              <div class="td">
                <span class="badge protocol-badge" :class="getProtocolClass(filteredHistoryConnections[virtualRow.index].protocol)">
                  {{ filteredHistoryConnections[virtualRow.index].protocol }}
                </span>
              </div>
              <div class="td">{{ formatBytes(filteredHistoryConnections[virtualRow.index].up || 0) }}</div>
              <div class="td">{{ formatBytes(filteredHistoryConnections[virtualRow.index].down || 0) }}</div>
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
const flatHistoryConnections = ref([])
const searchQuery = ref('')
const historyScrollerRef = ref(null)

let unsubscribeHistory = null

// 历史记录虚拟滚动器（仅过滤不展开子节点）
const filteredHistoryConnections = computed(() => {
  let result = flatHistoryConnections.value
  if (searchQuery.value) {
    const query = searchQuery.value.toLowerCase()
    result = result.filter(conn =>
      conn.host?.toLowerCase().includes(query) ||
      (conn.url && conn.url.toLowerCase().includes(query))
    )
  }
  return result
})

const historyVirtualizer = useVirtualizer(
  computed(() => ({
    count: filteredHistoryConnections.value.length,
    getScrollElement: () => historyScrollerRef.value,
    estimateSize: () => 45,
    overscan: 20,
    getItemKey: (index) => 'h-' + filteredHistoryConnections.value[index].id,
  }))
)

const historyVirtualRows = computed(() => historyVirtualizer.value.getVirtualItems())
const historyTotalSize = computed(() => historyVirtualizer.value.getTotalSize())

// 更新连接列表
function updateHistoryConnections(data) {
  flatHistoryConnections.value = data
}

// 清除所有历史
function handleClearHistory() {
  wsStore.clearHistoryConnections()
}

// 格式化字节数
function formatBytes(bytes) {
  if (!bytes) return '0 B'
  const k = 1024
  const sizes = ['B', 'KB', 'MB', 'GB']
  const i = Math.floor(Math.log(bytes) / Math.log(k))
  return (bytes / Math.pow(k, i)).toFixed(2) + ' ' + sizes[i]
}

// 获取方法样式类
function getMethodClass(method) {
  const methodMap = {
    'GET': 'method-get',
    'POST': 'method-post',
    'PUT': 'method-put',
    'DELETE': 'method-delete',
    'PATCH': 'method-patch',
    'HEAD': 'method-head',
    'OPTIONS': 'method-options',
    'CONNECT': 'method-connect'
  }
  return methodMap[method] || 'method-default'
}

// 获取协议样式类
function getProtocolClass(protocol) {
  const protocolMap = {
    'HTTP': 'protocol-http',
    'HTTPS': 'protocol-https',
    'HTTPS-MITM': 'protocol-mitm',
    'HTTPS-Tunnel': 'protocol-tunnel',
    'HTTP-MITM': 'protocol-mitm'
  }
  return protocolMap[protocol] || 'protocol-default'
}

// 生命周期钩子
onMounted(() => {
  unsubscribeHistory = wsStore.subscribeHistoryConnections((data) => {
    updateHistoryConnections(data)
  })

  // 初始化数据（从 store 缓存读取）
  updateHistoryConnections(wsStore.historyConnections)
})

onUnmounted(() => {
  if (unsubscribeHistory) unsubscribeHistory()
})
</script>

<style scoped>
.connections {
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

/* 统计信息栏 */
.stats-bar {
  display: flex;
  gap: 30px;
  background: #2a2a2a;
  border-radius: 8px;
  padding: 15px 20px;
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

.stat-item .value.closed {
  color: #6c757d;
}

/* 操作栏 */
.actions-bar {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 20px;
  gap: 15px;
  flex-shrink: 0;
}

.search-input {
  flex: 1;
  max-width: 400px;
  padding: 10px 15px;
  background: #2a2a2a;
  border: 1px solid #3a3a3a;
  border-radius: 6px;
  color: #cba376;
  font-size: 0.95em;
  outline: none;
  transition: border-color 0.3s;
}

.search-input:focus {
  border-color: #cba376;
}

.btn-clear-history {
  background: #6c757d;
  color: white;
  border: none;
  padding: 10px 20px;
  border-radius: 6px;
  cursor: pointer;
  font-size: 0.95em;
  transition: background 0.3s;
  white-space: nowrap;
}

.btn-clear-history:hover {
  background: #5a6268;
}

/* 表格部分 */
.table-section {
  flex: 1;
  overflow: hidden;
  display: flex;
  flex-direction: column;
  min-height: 0;
  background: #2a2a2a;
  border-radius: 8px;
  padding: 0;
}

/* CSS Grid 行布局 */
.conn-grid-row {
  display: grid;
  grid-template-columns: minmax(80px, 0.8fr) minmax(60px, 0.8fr) minmax(100px, 1.5fr) minmax(150px, 3fr) minmax(80px, 1fr) minmax(60px, 0.8fr) minmax(60px, 0.8fr);
  align-items: flex-start;
  color: #cba376;
}

/* 虚拟滚动容器 */
.conn-scroller {
  flex: 1;
  overflow: auto;
  overscroll-behavior: contain;
  min-height: 0;
}

/* 表头行 */
.thead-row {
  background: #1a1a1a;
  border-bottom: 2px solid #cba376;
  position: sticky;
  top: 0;
  z-index: 10;
}

.th {
  padding: 12px;
  font-weight: 600;
  white-space: nowrap;
}

/* 数据单元格 */
.td {
  padding: 10px 12px;
  border-bottom: 1px solid #3a3a3a;
  overflow: hidden;
  white-space: normal;
  word-break: break-all;
}

.url-cell {
  overflow: hidden;
  white-space: normal;
  word-break: break-all;
}

.no-data {
  text-align: center;
  color: #999;
  padding: 40px;
}

.history-row {
  opacity: 0.85;
  transition: opacity 0.2s;
}

.history-row:hover {
  opacity: 1;
}

.data-row:hover .td {
  background: #333;
}

/* 徽章样式 */
.badge {
  display: inline-block;
  padding: 3px 8px;
  border-radius: 4px;
  font-size: 0.85em;
  font-weight: 600;
  text-transform: uppercase;
}

.method-badge {
  min-width: 60px;
  text-align: center;
}

.method-get { background: rgba(40, 167, 69, 0.2); color: #28a745; }
.method-post { background: rgba(0, 123, 255, 0.2); color: #007bff; }
.method-put { background: rgba(255, 193, 7, 0.2); color: #ffc107; }
.method-delete { background: rgba(220, 53, 69, 0.2); color: #dc3545; }
.method-patch { background: rgba(108, 117, 125, 0.2); color: #6c757d; }
.method-head { background: rgba(111, 66, 193, 0.2); color: #6f42c1; }
.method-options { background: rgba(23, 162, 184, 0.2); color: #17a2b8; }
.method-connect { background: rgba(203, 163, 118, 0.2); color: #cba376; }
.method-default { background: rgba(108, 117, 125, 0.2); color: #6c757d; }

.protocol-badge {
  min-width: 80px;
  text-align: center;
}

.protocol-http { background: rgba(0, 123, 255, 0.2); color: #007bff; }
.protocol-https { background: rgba(40, 167, 69, 0.2); color: #28a745; }
.protocol-mitm { background: rgba(255, 193, 7, 0.2); color: #ffc107; }
.protocol-tunnel { background: rgba(108, 117, 125, 0.2); color: #6c757d; }
.protocol-default { background: rgba(108, 117, 125, 0.2); color: #6c757d; }

/* 滚动条样式 */
.conn-scroller::-webkit-scrollbar {
  width: 8px;
  height: 8px;
}

.conn-scroller::-webkit-scrollbar-track {
  background: #1a1a1a;
}

.conn-scroller::-webkit-scrollbar-thumb {
  background: #444;
  border-radius: 4px;
}

.conn-scroller::-webkit-scrollbar-thumb:hover {
  background: #555;
}
</style>
