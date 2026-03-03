<template>
  <div class="connections">
    <h1>详细连接</h1>

    <!-- 统计信息栏 -->
    <div class="stats-bar">
      <div class="stat-item">
        <span class="label">隧道数:</span>
        <span class="value">{{ tunnelCount }}</span>
      </div>
      <div class="stat-item">
        <span class="label">总连接:</span>
        <span class="value">{{ totalConnections }}</span>
      </div>
      <div class="stat-item">
        <span class="label">活跃中:</span>
        <span class="value active">{{ activeConnections }}</span>
      </div>
      <div class="stat-item">
        <span class="label">已关闭:</span>
        <span class="value closed">{{ closedConnections }}</span>
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
      <div class="conn-scroller" ref="scrollerRef">
        <!-- sticky 吸顶表头 -->
        <div class="conn-grid-row thead-row">
          <div @click="handleSort('id')" class="th sortable">
            <span class="expand-header-placeholder"></span>
            ID
            <span class="sort-icon" v-if="sortBy === 'id'">{{ sortOrder === 'asc' ? '▲' : '▼' }}</span>
          </div>
          <div @click="handleSort('method')" class="th sortable">
            方法
            <span class="sort-icon" v-if="sortBy === 'method'">{{ sortOrder === 'asc' ? '▲' : '▼' }}</span>
          </div>
          <div @click="handleSort('host')" class="th sortable">
            Host
            <span class="sort-icon" v-if="sortBy === 'host'">{{ sortOrder === 'asc' ? '▲' : '▼' }}</span>
          </div>
          <div @click="handleSort('url')" class="th sortable">
            URL
            <span class="sort-icon" v-if="sortBy === 'url'">{{ sortOrder === 'asc' ? '▲' : '▼' }}</span>
          </div>
          <div @click="handleSort('protocol')" class="th sortable">
            协议
            <span class="sort-icon" v-if="sortBy === 'protocol'">{{ sortOrder === 'asc' ? '▲' : '▼' }}</span>
          </div>
          <div @click="handleSort('up')" class="th sortable">
            上传
            <span class="sort-icon" v-if="sortBy === 'up'">{{ sortOrder === 'asc' ? '▲' : '▼' }}</span>
          </div>
          <div @click="handleSort('down')" class="th sortable">
            下载
            <span class="sort-icon" v-if="sortBy === 'down'">{{ sortOrder === 'asc' ? '▲' : '▼' }}</span>
          </div>
        </div>

        <!-- 空数据提示 -->
        <div v-if="sortedConnections.length === 0" class="no-data">
          {{ searchQuery ? '未找到匹配的连接' : '暂无活动连接' }}
        </div>

        <!-- 虚拟高度容器 -->
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
            <div
              class="conn-grid-row data-row"
              :class="{
                'parent-row': sortedConnections[virtualRow.index].isParent,
                'child-row': !sortedConnections[virtualRow.index].isParent
              }"
            >
              <div class="td">
                <span
                  v-if="sortedConnections[virtualRow.index].isParent && hasChildren(sortedConnections[virtualRow.index].id)"
                  @click.stop="toggleExpand(sortedConnections[virtualRow.index].id)"
                  class="expand-icon"
                >
                  {{ expandedIds.has(sortedConnections[virtualRow.index].id) ? '▼' : '▶' }}
                </span>
                <span v-else class="expand-placeholder"></span>
                {{ sortedConnections[virtualRow.index].id }}
              </div>
              <div class="td">
                <span class="badge method-badge" :class="getMethodClass(sortedConnections[virtualRow.index].method)">
                  {{ sortedConnections[virtualRow.index].method }}
                </span>
              </div>
              <div class="td">{{ sortedConnections[virtualRow.index].host }}</div>
              <div class="td url-cell" :title="sortedConnections[virtualRow.index].url">
                {{ sortedConnections[virtualRow.index].url }}
              </div>
              <div class="td">
                <span class="badge protocol-badge" :class="getProtocolClass(sortedConnections[virtualRow.index].protocol)">
                  {{ sortedConnections[virtualRow.index].protocol }}
                </span>
              </div>
              <div class="td">{{ formatBytes(sortedConnections[virtualRow.index].up || 0) }}</div>
              <div class="td">{{ formatBytes(sortedConnections[virtualRow.index].down || 0) }}</div>
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
const flatConnections = ref([])
const searchQuery = ref('')
const sortBy = ref('id')
const sortOrder = ref('desc')
const expandedIds = ref(new Set())
const scrollerRef = ref(null)

let unsubscribeConnections = null

// 计算属性：子节点映射表
const childrenMap = computed(() => {
  const map = {}
  flatConnections.value.forEach(conn => {
    if (conn.parentId !== 0) {
      if (!map[conn.parentId]) map[conn.parentId] = []
      map[conn.parentId].push(conn)
    }
  })
  return map
})

// 检查是否有子节点
function hasChildren(id) {
  return childrenMap.value[id] && childrenMap.value[id].length > 0
}

// 切换展开状态（new Set() 强制触发响应式更新）
function toggleExpand(id) {
  const set = expandedIds.value
  if (set.has(id)) { set.delete(id) } else { set.add(id) }
  expandedIds.value = new Set(set)
}

// 搜索时自动展开匹配的父节点
function expandSearchResults() {
  if (!searchQuery.value) return
  const query = searchQuery.value.toLowerCase()
  const set = expandedIds.value
  flatConnections.value.forEach(conn => {
    if ((conn.host?.toLowerCase().includes(query) ||
         conn.url?.toLowerCase().includes(query)) &&
        conn.parentId !== 0) {
      set.add(conn.parentId)
    }
  })
  expandedIds.value = new Set(set)
}

// 计算属性：过滤后的连接
const filteredConnections = computed(() => {
  let roots = flatConnections.value.filter(c => c.parentId === 0)

  if (searchQuery.value) {
    const query = searchQuery.value.toLowerCase()
    roots = roots.filter(conn =>
      conn.host?.toLowerCase().includes(query) ||
      conn.url?.toLowerCase().includes(query) ||
      (childrenMap.value[conn.id] || []).some(child =>
        child.host?.toLowerCase().includes(query) ||
        child.url?.toLowerCase().includes(query)
      )
    )
  }
  return roots
})

// 计算属性：排序后的连接（构建扁平化列表包含展开的子节点）
const sortedConnections = computed(() => {
  const sorted = [...filteredConnections.value]

  sorted.sort((a, b) => {
    let aVal = a[sortBy.value]
    let bVal = b[sortBy.value]

    if (sortBy.value === 'id' || sortBy.value === 'up' || sortBy.value === 'down') {
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

  const result = []
  sorted.forEach(parent => {
    result.push({ ...parent, isParent: true })
    if (expandedIds.value.has(parent.id) && childrenMap.value[parent.id]) {
      childrenMap.value[parent.id].forEach(child => {
        result.push({ ...child, isParent: false })
      })
    }
  })
  return result
})

// 虚拟滚动器（动态高度，measureElement 感知 count 变化）
const virtualizer = useVirtualizer(
  computed(() => ({
    count: sortedConnections.value.length,
    getScrollElement: () => scrollerRef.value,
    estimateSize: () => 42,
    overscan: 10,
    getItemKey: (index) => {
      const conn = sortedConnections.value[index]
      return `${conn.isParent ? 'p' : 'c'}-${conn.id}`
    },
  }))
)

const virtualRows = computed(() => virtualizer.value.getVirtualItems())
const totalSize = computed(() => virtualizer.value.getTotalSize())

// 计算属性：隧道数
const tunnelCount = computed(() => {
  return flatConnections.value.filter(c => c.parentId === 0 && c.protocol !== 'HTTP').length
})

// 计算属性：所有连接数
const totalConnections = computed(() => {
  return flatConnections.value.filter(c => c.parentId !== 0 || c.protocol === 'HTTP').length
})

// 计算属性：活跃连接数
const activeConnections = computed(() => {
  return flatConnections.value.filter(c => (c.parentId !== 0 || c.protocol === 'HTTP') && c.status === 'Active').length
})

// 计算属性：已关闭连接数
const closedConnections = computed(() => {
  return flatConnections.value.filter(c => (c.parentId !== 0 || c.protocol === 'HTTP') && c.status === 'Closed').length
})

// 更新连接列表（后端全量快照替换）
function updateConnections(data) {
  flatConnections.value = data
  expandSearchResults()
}

// 处理排序
function handleSort(field) {
  if (sortBy.value === field) {
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
  unsubscribeConnections = wsStore.subscribeConnections((data) => {
    updateConnections(data)
  })

  // 初始化数据（从 store 缓存读取）
  updateConnections(wsStore.connections)
})

onUnmounted(() => {
  if (unsubscribeConnections) unsubscribeConnections()
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

.stat-item .value.active {
  color: #28a745;
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
  grid-template-columns: 100px 80px 200px 1fr 120px 80px 80px;
  align-items: center;
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

.th.sortable {
  cursor: pointer;
  user-select: none;
  transition: background 0.2s;
}

.th.sortable:hover {
  background: #252525;
}

.sort-icon {
  display: inline-block;
  margin-left: 5px;
  font-size: 0.8em;
  opacity: 0.8;
}

/* 数据单元格 */
.td {
  padding: 10px 12px;
  border-bottom: 1px solid #3a3a3a;
  overflow: hidden;
  white-space: nowrap;
  text-overflow: ellipsis;
}

.url-cell {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.no-data {
  text-align: center;
  color: #999;
  padding: 40px;
}

/* 父行样式 */
.parent-row {
  background: #2a2a2a;
}

/* 子行样式 */
.child-row .td:first-child {
  padding-left: 40px;
}

.child-row .td {
  background: #222;
}

.child-row:hover .td {
  background: #2d2d2d !important;
}

/* 数据行 hover */
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

/* 展开图标 */
.expand-icon {
  cursor: pointer;
  display: inline-block;
  width: 16px;
  margin-right: 8px;
  color: #cba376;
  user-select: none;
}

.expand-icon:hover {
  color: #fff;
}

.expand-placeholder {
  display: inline-block;
  width: 24px;
}

.expand-header-placeholder {
  display: inline-block;
  width: 24px;
}

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
