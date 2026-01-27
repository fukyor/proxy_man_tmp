<template>
  <div class="connections">
    <h1>详细连接</h1>

    <!-- 统计信息栏 -->
    <div class="stats-bar">
      <div class="stat-item">
        <span class="label">连接数:</span>
        <span class="value">{{ connections.length }}</span>
      </div>
      <div class="stat-item">
        <span class="label">总上传:</span>
        <span class="value">{{ formatBytes(totalUpload) }}</span>
      </div>
      <div class="stat-item">
        <span class="label">总下载:</span>
        <span class="value">{{ formatBytes(totalDownload) }}</span>
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
      <button @click="handleCloseAll" class="btn-close-all">关闭所有连接</button>
    </div>

    <!-- 连接表格 -->
    <div class="table-section">
      <div class="table-container">
        <table class="connections-table">
          <thead>
            <tr>
              <th @click="handleSort('id')" class="sortable">
                ID
                <span class="sort-icon" v-if="sortBy === 'id'">
                  {{ sortOrder === 'asc' ? '▲' : '▼' }}
                </span>
              </th>
              <th @click="handleSort('method')" class="sortable">
                方法
                <span class="sort-icon" v-if="sortBy === 'method'">
                  {{ sortOrder === 'asc' ? '▲' : '▼' }}
                </span>
              </th>
              <th @click="handleSort('host')" class="sortable">
                Host
                <span class="sort-icon" v-if="sortBy === 'host'">
                  {{ sortOrder === 'asc' ? '▲' : '▼' }}
                </span>
              </th>
              <th @click="handleSort('url')" class="sortable">
                URL
                <span class="sort-icon" v-if="sortBy === 'url'">
                  {{ sortOrder === 'asc' ? '▲' : '▼' }}
                </span>
              </th>
              <th @click="handleSort('protocol')" class="sortable">
                协议
                <span class="sort-icon" v-if="sortBy === 'protocol'">
                  {{ sortOrder === 'asc' ? '▲' : '▼' }}
                </span>
              </th>
              <th @click="handleSort('up')" class="sortable">
                上传
                <span class="sort-icon" v-if="sortBy === 'up'">
                  {{ sortOrder === 'asc' ? '▲' : '▼' }}
                </span>
              </th>
              <th @click="handleSort('down')" class="sortable">
                下载
                <span class="sort-icon" v-if="sortBy === 'down'">
                  {{ sortOrder === 'asc' ? '▲' : '▼' }}
                </span>
              </th>
            </tr>
          </thead>
          <tbody>
            <tr v-if="filteredConnections.length === 0">
              <td colspan="7" class="no-data">
                {{ searchQuery ? '未找到匹配的连接' : '暂无活动连接' }}
              </td>
            </tr>
            <tr v-for="conn in sortedConnections" :key="conn.id">
              <td>{{ conn.id }}</td>
              <td>
                <span class="badge method-badge" :class="getMethodClass(conn.method)">
                  {{ conn.method }}
                </span>
              </td>
              <td>{{ conn.host }}</td>
              <td class="url-cell">{{ conn.url }}</td>
              <td>
                <span class="badge protocol-badge" :class="getProtocolClass(conn.protocol)">
                  {{ conn.protocol }}
                </span>
              </td>
              <td>{{ formatBytes(conn.up || 0) }}</td>
              <td>{{ formatBytes(conn.down || 0) }}</td>
            </tr>
          </tbody>
        </table>
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref, computed, onMounted, onUnmounted } from 'vue'
import { useWebSocketStore } from '@/stores/websocket'

const wsStore = useWebSocketStore()

// 响应式数据
const connections = ref([])
const searchQuery = ref('')
const sortBy = ref('id')
const sortOrder = ref('desc')

let unsubscribeConnections = null

// 计算属性：过滤后的连接
const filteredConnections = computed(() => {
  if (!searchQuery.value) {
    return connections.value
  }

  const query = searchQuery.value.toLowerCase()
  return connections.value.filter(conn =>
    conn.host?.toLowerCase().includes(query) ||
    conn.url?.toLowerCase().includes(query)
  )
})

// 计算属性：排序后的连接
const sortedConnections = computed(() => {
  const sorted = [...filteredConnections.value]

  sorted.sort((a, b) => {
    let aVal = a[sortBy.value]
    let bVal = b[sortBy.value]

    // 处理数值类型
    if (sortBy.value === 'id' || sortBy.value === 'up' || sortBy.value === 'down') {
      aVal = Number(aVal) || 0
      bVal = Number(bVal) || 0
    } else {
      // 字符串类型
      aVal = String(aVal || '').toLowerCase()
      bVal = String(bVal || '').toLowerCase()
    }

    if (aVal < bVal) return sortOrder.value === 'asc' ? -1 : 1
    if (aVal > bVal) return sortOrder.value === 'asc' ? 1 : -1
    return 0
  })

  return sorted
})

// 计算属性：总上传流量
const totalUpload = computed(() => {
  return connections.value.reduce((sum, conn) => sum + (conn.up || 0), 0)
})

// 计算属性：总下载流量
const totalDownload = computed(() => {
  return connections.value.reduce((sum, conn) => sum + (conn.down || 0), 0)
})

// 更新连接列表（只显示顶层隧道连接）
function updateConnections(data) {
  connections.value = data.filter(conn => conn.parentId === 0)
}

// 处理排序
function handleSort(field) {
  if (sortBy.value === field) {
    // 切换排序方向
    sortOrder.value = sortOrder.value === 'asc' ? 'desc' : 'asc'
  } else {
    sortBy.value = field
    sortOrder.value = 'desc'
  }
}

// 关闭所有连接
function handleCloseAll() {
  wsStore.closeAllConnections()
}

// 格式化字节数
function formatBytes(bytes) {
  if (bytes === 0) return '0 B'
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
  // 订阅连接更新
  unsubscribeConnections = wsStore.subscribeConnections((data) => {
    updateConnections(data)
  })

  // 初始化数据
  updateConnections(wsStore.connections)
})

onUnmounted(() => {
  // 取消订阅
  if (unsubscribeConnections) unsubscribeConnections()
})
</script>

<style scoped>
.connections {
  padding: 20px;
}

h1 {
  color: #cba376;
  margin-bottom: 20px;
}

/* 统计信息栏 */
.stats-bar {
  display: flex;
  gap: 30px;
  background: #2a2a2a;
  border-radius: 8px;
  padding: 15px 20px;
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

/* 操作栏 */
.actions-bar {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 20px;
  gap: 15px;
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

.search-input::placeholder {
  color: #666;
}

.btn-close-all {
  background: #d9534f;
  color: white;
  border: none;
  padding: 10px 20px;
  border-radius: 6px;
  cursor: pointer;
  font-size: 0.95em;
  transition: background 0.3s;
  white-space: nowrap;
}

.btn-close-all:hover {
  background: #c9302c;
}

/* 表格部分 */
.table-section {
  background: #2a2a2a;
  border-radius: 8px;
  padding: 20px;
}

.table-container {
  overflow-x: auto;
}

.connections-table {
  width: 100%;
  border-collapse: collapse;
  color: #cba376;
}

.connections-table th {
  background: #1a1a1a;
  padding: 12px;
  text-align: left;
  font-weight: 600;
  border-bottom: 2px solid #cba376;
  white-space: nowrap;
}

.connections-table th.sortable {
  cursor: pointer;
  user-select: none;
  position: relative;
  transition: background 0.2s;
}

.connections-table th.sortable:hover {
  background: #252525;
}

.sort-icon {
  display: inline-block;
  margin-left: 5px;
  font-size: 0.8em;
  opacity: 0.8;
}

.connections-table td {
  padding: 10px 12px;
  border-bottom: 1px solid #3a3a3a;
}

.connections-table tr:hover {
  background: #333;
}

.url-cell {
  max-width: 400px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.no-data {
  text-align: center;
  color: #999;
  padding: 40px !important;
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

/* 方法徽章 */
.method-badge {
  min-width: 60px;
  text-align: center;
}

.method-get {
  background: rgba(40, 167, 69, 0.2);
  color: #28a745;
}

.method-post {
  background: rgba(0, 123, 255, 0.2);
  color: #007bff;
}

.method-put {
  background: rgba(255, 193, 7, 0.2);
  color: #ffc107;
}

.method-delete {
  background: rgba(220, 53, 69, 0.2);
  color: #dc3545;
}

.method-patch {
  background: rgba(108, 117, 125, 0.2);
  color: #6c757d;
}

.method-head {
  background: rgba(111, 66, 193, 0.2);
  color: #6f42c1;
}

.method-options {
  background: rgba(23, 162, 184, 0.2);
  color: #17a2b8;
}

.method-connect {
  background: rgba(203, 163, 118, 0.2);
  color: #cba376;
}

.method-default {
  background: rgba(108, 117, 125, 0.2);
  color: #6c757d;
}

/* 协议徽章 */
.protocol-badge {
  min-width: 80px;
  text-align: center;
}

.protocol-http {
  background: rgba(0, 123, 255, 0.2);
  color: #007bff;
}

.protocol-https {
  background: rgba(40, 167, 69, 0.2);
  color: #28a745;
}

.protocol-mitm {
  background: rgba(255, 193, 7, 0.2);
  color: #ffc107;
}

.protocol-tunnel {
  background: rgba(108, 117, 125, 0.2);
  color: #6c757d;
}

.protocol-default {
  background: rgba(108, 117, 125, 0.2);
  color: #6c757d;
}
</style>
