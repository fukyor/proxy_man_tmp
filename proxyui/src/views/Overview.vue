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
            <tr v-for="conn in connections" :key="conn.id">
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
  </div>
</template>

<script setup>
import { ref, onMounted, onUnmounted } from 'vue'
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

let chart = null
let unsubscribeTraffic = null
let unsubscribeConnections = null

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
  connections.value = data
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
</style>
