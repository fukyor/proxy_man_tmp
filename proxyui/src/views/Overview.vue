<template>
  <div class="overview">
    <h1>概览</h1>

    <!-- 流量图表区域 -->
    <div class="traffic-section">
      <h2>实时流量</h2>
      <div class="traffic-stats">
        <div class="stat-item">
          <span class="label">上传速率:</span>
          <span class="value">{{ formatBytes(currentUpload) }}/s</span>
        </div>
        <div class="stat-item">
          <span class="label">下载速率:</span>
          <span class="value">{{ formatBytes(currentDownload) }}/s</span>
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
      <div class="chart-container">
        <canvas ref="chartCanvas"></canvas>
      </div>
    </div>

    <!-- 连接列表区域 -->
    <div class="connections-section">
      <div class="section-header">
        <h2>活动连接 ({{ totalActiveCount }})</h2>
        <button @click="handleCloseAll" class="btn-close-all">关闭所有连接</button>
      </div>

      <!-- CSS Grid + 虚拟滚动 -->
      <div class="conn-scroller" ref="scrollerRef">
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
        <div v-if="connections.length === 0" class="no-data">暂无活跃连接</div>

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
            <div class="conn-grid-row data-row">
              <div class="td">{{ connections[virtualRow.index].id }}</div>
              <div class="td">{{ connections[virtualRow.index].method }}</div>
              <div class="td">{{ connections[virtualRow.index].host }}</div>
              <div class="td url-cell" :title="connections[virtualRow.index].url">{{ connections[virtualRow.index].url }}</div>
              <div class="td">{{ connections[virtualRow.index].protocol }}</div>
              <div class="td">{{ formatBytes(connections[virtualRow.index].up || 0) }}</div>
              <div class="td">{{ formatBytes(connections[virtualRow.index].down || 0) }}</div>
            </div>
          </div>
        </div>
      </div>

      <!-- 查看更多 -->
      <div v-if="totalActiveCount > OVERVIEW_MAX_DISPLAY" class="view-more">
        <span>显示前 {{ OVERVIEW_MAX_DISPLAY }} 条，共 {{ totalActiveCount }} 条活跃连接</span>
        <router-link to="/dashboard/connections" class="btn-view-all">查看全部连接</router-link>
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref, computed, onMounted, onUnmounted } from 'vue'
import { useVirtualizer } from '@tanstack/vue-virtual'
import { useWebSocketStore } from '@/stores/websocket'
import { Chart, registerables } from 'chart.js'

// 注册 Chart.js 组件
Chart.register(...registerables)

const wsStore = useWebSocketStore()

const OVERVIEW_MAX_DISPLAY = 50

// 响应式数据
const chartCanvas = ref(null)
const currentUpload = ref(0)
const currentDownload = ref(0)
const connections = ref([])
const totalUpload = ref(0)
const totalDownload = ref(0)
const totalActiveCount = ref(0)
const scrollerRef = ref(null)

let chart = null
let unsubscribeTraffic = null
let unsubscribeConnections = null

// 虚拟滚动器
const virtualizer = useVirtualizer(
  computed(() => ({
    count: connections.value.length,
    getScrollElement: () => scrollerRef.value,
    estimateSize: () => 60,
    overscan: 10,
    getItemKey: (index) => connections.value[index].id,
  }))
)

const virtualRows = computed(() => virtualizer.value.getVirtualItems())
const totalSize = computed(() => virtualizer.value.getTotalSize())

// 初始化图表
function initChart() {
  if (!chartCanvas.value) return

  const ctx = chartCanvas.value.getContext('2d')

  chart = new Chart(ctx, {
    type: 'line',
    data: {
      labels: Array(60).fill(''),
      datasets: [
        {
          label: '上传 (B/s)',
          data: Array(60).fill(0),
          borderColor: 'rgb(75, 192, 192)',
          backgroundColor: 'rgba(75, 192, 192, 0.2)',
          tension: 0.4,
          fill: true
        },
        {
          label: '下载 (B/s)',
          data: Array(60).fill(0),
          borderColor: 'rgb(255, 99, 132)',
          backgroundColor: 'rgba(255, 99, 132, 0.2)',
          tension: 0.4,
          fill: true
        }
      ]
    },
    options: {
      responsive: true,
      maintainAspectRatio: false,
      animation: {
        duration: 0
      },
      scales: {
        y: {
          beginAtZero: true,
          ticks: {
            callback: (value) => formatBytes(value)
          }
        },
        x: {
          display: false
        }
      },
      plugins: {
        legend: {
          position: 'top'
        },
        tooltip: {
          callbacks: {
            label: (context) => {
              return `${context.dataset.label}: ${formatBytes(context.parsed.y)}`
            }
          }
        }
      }
    }
  })
}

// 更新图表数据
function updateChart(data) {
  if (!chart) return

  currentUpload.value = data.up
  currentDownload.value = data.down
  totalUpload.value = data.totalUp || 0
  totalDownload.value = data.totalDown || 0

  chart.data.datasets[0].data.push(data.up)
  if (chart.data.datasets[0].data.length > 60) {
    chart.data.datasets[0].data.shift()
  }

  chart.data.datasets[1].data.push(data.down)
  if (chart.data.datasets[1].data.length > 60) {
    chart.data.datasets[1].data.shift()
  }

  chart.update()
}

// 更新连接列表（只显示活跃状态的子节点连接，最多 50 条）
function updateConnections(data) {
  const activeChildren = data.filter(conn =>
    conn.parentId !== 0 && conn.status === 'Active'
  )
  totalActiveCount.value = activeChildren.length
  connections.value = activeChildren.slice(0, OVERVIEW_MAX_DISPLAY)
}

// 关闭所有连接
function handleCloseAll() {
  wsStore.closeAllConnections()
}

// 格式化字节数
function formatBytes(bytes) {
  if (!bytes || bytes <= 0) return '0 B'
  const k = 1024
  const sizes = ['B', 'KB', 'MB', 'GB', 'TB']
  const i = Math.min(Math.floor(Math.log(bytes) / Math.log(k)), sizes.length - 1)
  return (bytes / Math.pow(k, i)).toFixed(2) + ' ' + sizes[i]
}

// 生命周期钩子
onMounted(() => {
  initChart()

  unsubscribeTraffic = wsStore.subscribeTraffic((data) => {
    updateChart(data)
  })

  unsubscribeConnections = wsStore.subscribeConnections((data) => {
    updateConnections(data)
  })

  // 初始化数据（从 store 缓存读取）
  if (wsStore.connections.length > 0) {
    updateConnections(wsStore.connections)
  }
})

onUnmounted(() => {
  if (unsubscribeTraffic) unsubscribeTraffic()
  if (unsubscribeConnections) unsubscribeConnections()
  if (chart) {
    chart.destroy()
  }
})
</script>

<style scoped>
.overview {
  padding: 20px;
}

h1 {
  color: #cba376;
  margin-bottom: 20px;
}

h2 {
  color: #cba376;
  font-size: 1.2em;
  margin-bottom: 15px;
}

/* 流量部分 */
.traffic-section {
  background: #2a2a2a;
  border-radius: 8px;
  padding: 20px;
  margin-bottom: 30px;
}

.traffic-stats {
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

.chart-container {
  height: 300px;
  position: relative;
}

/* 连接列表部分 */
.connections-section {
  background: #2a2a2a;
  border-radius: 8px;
  padding: 20px;
}

.section-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 15px;
}

.btn-close-all {
  background: #d9534f;
  color: white;
  border: none;
  padding: 8px 16px;
  border-radius: 4px;
  cursor: pointer;
  font-size: 0.9em;
  transition: background 0.3s;
}

.btn-close-all:hover {
  background: #c9302c;
}

/* CSS Grid 行布局 */
.conn-grid-row {
  display: grid;
  grid-template-columns: minmax(40px, 0.5fr) minmax(60px, 0.8fr) minmax(100px, 1.5fr) minmax(120px, 3fr) minmax(60px, 0.8fr) minmax(60px, 0.8fr) minmax(60px, 0.8fr);
  align-items: flex-start;
  color: #cba376;
}

/* 虚拟滚动容器 */
.conn-scroller {
  max-height: 400px;
  overflow: auto;
  overscroll-behavior: contain;
  border-radius: 4px;
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

/* 数据行 */
.data-row {
  border-bottom: 1px solid #3a3a3a;
}

.data-row:hover .td {
  background: #333;
}

.td {
  padding: 10px 12px;
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

/* 查看更多 */
.view-more {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-top: 12px;
  padding: 10px 12px;
  background: #1a1a1a;
  border-radius: 4px;
  font-size: 0.9em;
  color: #999;
}

.btn-view-all {
  color: #cba376;
  text-decoration: none;
  padding: 6px 14px;
  border: 1px solid #cba376;
  border-radius: 4px;
  font-size: 0.9em;
  transition: all 0.2s;
  white-space: nowrap;
}

.btn-view-all:hover {
  background: rgba(203, 163, 118, 0.15);
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
