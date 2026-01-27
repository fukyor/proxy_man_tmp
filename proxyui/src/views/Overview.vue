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
      </div>
      <div class="chart-container">
        <canvas ref="chartCanvas"></canvas>
      </div>
    </div>

    <!-- 连接列表区域 -->
    <div class="connections-section">
      <div class="section-header">
        <h2>活动连接</h2>
        <button @click="handleCloseAll" class="btn-close-all">关闭所有连接</button>
      </div>
      <div class="table-container">
        <table class="connections-table">
          <thead>
            <tr>
              <th>ID</th>
              <th>方法</th>
              <th>Host</th>
              <th>URL</th>
              <th>协议</th>
              <th>上传</th>
              <th>下载</th>
            </tr>
          </thead>
          <tbody>
            <tr v-if="connections.length === 0">
              <td colspan="7" class="no-data">暂无活动连接</td>
            </tr>
            <tr v-for="conn in connections" :key="conn.id" @click="handleRowClick(conn)" class="clickable-row">
              <td>{{ conn.id }}</td>
              <td>{{ conn.method }}</td>
              <td>{{ conn.host }}</td>
              <td class="url-cell">{{ conn.url }}</td>
              <td>{{ conn.protocol }}</td>
              <td>{{ formatBytes(conn.up || 0) }}</td>
              <td>{{ formatBytes(conn.down || 0) }}</td>
            </tr>
          </tbody>
        </table>
      </div>
    </div>

    <!-- 侧边栏遮罩层 -->
    <div v-if="sidebarVisible" class="sidebar-overlay" @click.self="closeSidebar">
      <div class="sidebar">
        <!-- 侧边栏头部 -->
        <div class="sidebar-header">
          <h3>隧道连接详情</h3>
          <button @click="closeSidebar" class="btn-close">&times;</button>
        </div>

        <!-- 侧边栏内容 -->
        <div class="sidebar-content">
          <!-- 隧道基本信息 -->
          <div class="tunnel-info">
            <div class="info-row">
              <span class="label">ID:</span>
              <span class="value">{{ selectedTunnel?.id }}</span>
            </div>
            <div class="info-row">
              <span class="label">Host:</span>
              <span class="value">{{ selectedTunnel?.host }}</span>
            </div>
            <div class="info-row">
              <span class="label">协议:</span>
              <span class="value">{{ selectedTunnel?.protocol }}</span>
            </div>
            <div class="info-row">
              <span class="label">远程地址:</span>
              <span class="value">{{ selectedTunnel?.remote }}</span>
            </div>
          </div>

          <!-- 流量统计 -->
          <div class="traffic-summary">
            <h4>流量统计</h4>
            <div class="traffic-row">
              <span class="label">总上传:</span>
              <span class="value">{{ formatBytes(sidebarTraffic.up) }}</span>
            </div>
            <div class="traffic-row">
              <span class="label">总下载:</span>
              <span class="value">{{ formatBytes(sidebarTraffic.down) }}</span>
            </div>
          </div>

          <!-- 子请求列表 -->
          <div class="children-list">
            <h4>子请求列表 ({{ sidebarChildren.length }})</h4>
            <div v-if="sidebarChildren.length === 0" class="no-children">
              暂无子请求
            </div>
            <div v-else class="children-table-container">
              <table class="children-table">
                <thead>
                  <tr>
                    <th>ID</th>
                    <th>方法</th>
                    <th>URL</th>
                    <th>上传</th>
                    <th>下载</th>
                  </tr>
                </thead>
                <tbody>
                  <tr v-for="child in sidebarChildren" :key="child.id">
                    <td>{{ child.id }}</td>
                    <td>{{ child.method }}</td>
                    <td class="url-cell">{{ child.url }}</td>
                    <td>{{ formatBytes(child.up || 0) }}</td>
                    <td>{{ formatBytes(child.down || 0) }}</td>
                  </tr>
                </tbody>
              </table>
            </div>
          </div>
        </div>
      </div>
    </div>
  </div>

</template>

<script setup>
import { ref, computed, onMounted, onUnmounted } from 'vue'
import { useWebSocketStore } from '@/stores/websocket'
import { Chart, registerables } from 'chart.js'

// 注册 Chart.js 组件
Chart.register(...registerables)

const wsStore = useWebSocketStore()

// 响应式数据
const chartCanvas = ref(null)
const currentUpload = ref(0)
const currentDownload = ref(0)
const connections = ref([])
const allConnections = ref([]) // 存储所有连接（包括隧道和子请求）
const sidebarVisible = ref(false) // 侧边栏显示状态
const selectedTunnel = ref(null) // 选中的隧道连接

let chart = null
let unsubscribeTraffic = null
let unsubscribeConnections = null

// 计算属性：侧边栏中显示的子请求列表
const sidebarChildren = computed(() => {
  if (!selectedTunnel.value) return []
  return allConnections.value.filter(conn => conn.parentId === selectedTunnel.value.id)
})

// 计算属性：聚合子请求流量计算隧道总流量
const sidebarTraffic = computed(() => {
  const children = sidebarChildren.value
  if (children.length === 0) return { up: 0, down: 0 }

  return children.reduce((acc, conn) => {
    acc.up += (conn.up || 0)
    acc.down += (conn.down || 0)
    return acc
  }, { up: 0, down: 0 })
})

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

  // 更新上传数据
  chart.data.datasets[0].data.push(data.up)
  if (chart.data.datasets[0].data.length > 60) {
    chart.data.datasets[0].data.shift()
  }

  // 更新下载数据
  chart.data.datasets[1].data.push(data.down)
  if (chart.data.datasets[1].data.length > 60) {
    chart.data.datasets[1].data.shift()
  }

  chart.update()
}

// 更新连接列表
function updateConnections(data) {
  allConnections.value = data
  // 主表格只显示顶层隧道连接（parentId === 0）
  connections.value = data.filter(conn => conn.parentId === 0)
}

// 处理行点击事件
function handleRowClick(conn) {
  selectedTunnel.value = conn
  sidebarVisible.value = true
}

// 关闭侧边栏
function closeSidebar() {
  sidebarVisible.value = false
  selectedTunnel.value = null
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

// 格式化时间
function formatTime(time) {
  if (!time) return ''
  const date = new Date(time)
  return date.toLocaleString('zh-CN', {
    year: 'numeric',
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
    second: '2-digit'
  })
}

// 生命周期钩子
onMounted(() => {
  initChart()

  // 订阅流量更新，注册流量更新回调，每次流量更新时执行
  unsubscribeTraffic = wsStore.subscribeTraffic((data) => {
    updateChart(data)
  })

  // 订阅连接更新，注册连接更新回调，每次连接更新时执行
  unsubscribeConnections = wsStore.subscribeConnections((data) => {
    updateConnections(data)
  })

  // 初始化数据
  connections.value = wsStore.connections
})

onUnmounted(() => {
  // 取消订阅
  if (unsubscribeTraffic) unsubscribeTraffic()
  if (unsubscribeConnections) unsubscribeConnections()

  // 销毁图表
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
}

.connections-table td {
  padding: 10px 12px;
  border-bottom: 1px solid #3a3a3a;
}

.connections-table tr:hover {
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

/* 表格行可点击样式 */
.clickable-row {
  cursor: pointer;
  transition: background-color 0.2s;
}

.clickable-row:hover {
  background-color: #3a3a3a !important;
}

/* 侧边栏遮罩层 */
.sidebar-overlay {
  position: fixed;
  top: 0;
  left: 0;
  right: 0;
  bottom: 0;
  background-color: rgba(0, 0, 0, 0.5);
  z-index: 1000;
  display: flex;
  justify-content: flex-end;
}

/* 侧边栏容器 */
.sidebar {
  width: 600px;
  max-width: 90vw;
  background: #2a2a2a;
  height: 100vh;
  overflow-y: auto;
  box-shadow: -2px 0 8px rgba(0, 0, 0, 0.3);
}

/* 侧边栏头部 */
.sidebar-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 20px;
  background: #1a1a1a;
  border-bottom: 2px solid #cba376;
  position: sticky;
  top: 0;
  z-index: 1;
}

.sidebar-header h3 {
  color: #cba376;
  margin: 0;
  font-size: 1.3em;
}

.btn-close {
  background: transparent;
  border: none;
  color: #cba376;
  font-size: 2em;
  cursor: pointer;
  line-height: 1;
  padding: 0;
  width: 30px;
  height: 30px;
  display: flex;
  align-items: center;
  justify-content: center;
  transition: color 0.3s;
}

.btn-close:hover {
  color: #fff;
}

/* 侧边栏内容 */
.sidebar-content {
  padding: 20px;
}

/* 隧道信息区 */
.tunnel-info {
  background: #1a1a1a;
  border-radius: 8px;
  padding: 15px;
  margin-bottom: 20px;
}

.info-row {
  display: flex;
  justify-content: space-between;
  padding: 8px 0;
  border-bottom: 1px solid #3a3a3a;
}

.info-row:last-child {
  border-bottom: none;
}

.info-row .label {
  color: #999;
  font-size: 0.9em;
}

.info-row .value {
  color: #cba376;
  font-weight: 500;
}

/* 流量统计区 */
.traffic-summary {
  background: #1a1a1a;
  border-radius: 8px;
  padding: 15px;
  margin-bottom: 20px;
}

.traffic-summary h4 {
  color: #cba376;
  margin: 0 0 15px 0;
  font-size: 1.1em;
}

.traffic-row {
  display: flex;
  justify-content: space-between;
  padding: 8px 0;
  border-bottom: 1px solid #3a3a3a;
}

.traffic-row:last-child {
  border-bottom: none;
}

.traffic-row .label {
  color: #999;
  font-size: 0.9em;
}

.traffic-row .value {
  color: #cba376;
  font-weight: 600;
}

/* 子请求列表区 */
.children-list {
  background: #1a1a1a;
  border-radius: 8px;
  padding: 15px;
}

.children-list h4 {
  color: #cba376;
  margin: 0 0 15px 0;
  font-size: 1.1em;
}

.no-children {
  text-align: center;
  color: #999;
  padding: 30px;
  font-size: 0.9em;
}

.children-table-container {
  overflow-x: auto;
  max-height: 400px;
  overflow-y: auto;
}

.children-table {
  width: 100%;
  border-collapse: collapse;
  color: #cba376;
  font-size: 0.9em;
}

.children-table th {
  background: #2a2a2a;
  padding: 10px;
  text-align: left;
  font-weight: 600;
  border-bottom: 1px solid #cba376;
  position: sticky;
  top: 0;
  z-index: 1;
}

.children-table td {
  padding: 8px 10px;
  border-bottom: 1px solid #3a3a3a;
}

.children-table tr:hover {
  background: #333;
}

.children-table .url-cell {
  max-width: 200px;
}
</style>
