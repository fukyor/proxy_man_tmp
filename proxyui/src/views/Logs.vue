<template>
  <div class="logs">
    <h1>日志</h1>

    <!-- 控制栏 -->
    <div class="controls">
      <div class="filter-group">
        <label for="logLevel">日志级别:</label>
        <select id="logLevel" v-model="selectedLevel" @change="handleLevelChange">
          <option value="DEBUG">DEBUG</option>
          <option value="INFO">INFO</option>
          <option value="WARN">WARN</option>
          <option value="ERROR">ERROR</option>
        </select>
      </div>

      <div class="button-group">
        <button @click="handleClearLogs" class="btn-clear">清除日志</button>
        <button @click="toggleAutoScroll" class="btn-scroll">
          {{ autoScroll ? '禁用自动滚动' : '启用自动滚动' }}
        </button>
      </div>
    </div>

    <!-- 日志列表 -->
    <div class="logs-container" ref="logsContainer">
      <div v-if="filteredLogs.length === 0" class="no-logs">
        暂无日志
      </div>
      <div
        v-for="log in filteredLogs"
        :key="log.id"
        :class="['log-entry', `log-${log.level.toLowerCase()}`]"
      >
        <span class="log-time">{{ formatTime(log.time) }}</span>
        <span class="log-level">{{ log.level }}</span>
        <span class="log-session">{{ log.session > 0 ? `[${log.session}]` : '' }}</span>
        <span class="log-message">{{ log.message }}</span>
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref, computed, onMounted, onUnmounted, nextTick } from 'vue'
import { useWebSocketStore } from '@/stores/websocket'

const wsStore = useWebSocketStore()

// 响应式数据
const selectedLevel = ref('INFO')
const autoScroll = ref(true)
const logsContainer = ref(null)

let unsubscribeLogs = null

// 日志级别权重（用于过滤）
const logLevels = { DEBUG: 0, INFO: 1, WARN: 2, ERROR: 3 }

// 计算过滤后的日志
const filteredLogs = computed(() => {
  const minLevel = logLevels[selectedLevel.value]
  return wsStore.logs.filter(log => {
    const logLevel = logLevels[log.level]
    return logLevel >= minLevel
  })
})

// 处理级别变化
function handleLevelChange() {
  // 更新 WebSocket 订阅的日志级别
  wsStore.updateSubscriptions({ logLevel: selectedLevel.value })
}

// 清除日志
function handleClearLogs() {
  wsStore.clearLogs()
}

// 切换自动滚动
function toggleAutoScroll() {
  autoScroll.value = !autoScroll.value
  if (autoScroll.value) {
    scrollToBottom()
  }
}

// 滚动到底部
function scrollToBottom() {
  if (!logsContainer.value || !autoScroll.value) return

  nextTick(() => {
    logsContainer.value.scrollTop = logsContainer.value.scrollHeight
  })
}

// 格式化时间
function formatTime(time) {
  if (!time) return ''
  const date = new Date(time)
  const hours = String(date.getHours()).padStart(2, '0')
  const minutes = String(date.getMinutes()).padStart(2, '0')
  const seconds = String(date.getSeconds()).padStart(2, '0')
  const ms = String(date.getMilliseconds()).padStart(3, '0')
  return `${hours}:${minutes}:${seconds}.${ms}`
}

// 生命周期钩子
onMounted(() => {
  // 订阅日志更新
  unsubscribeLogs = wsStore.subscribeLogs(() => {
    scrollToBottom()
  })

  // 初始化日志级别
  selectedLevel.value = wsStore.subscriptions.logLevel
})

onUnmounted(() => {
  // 取消订阅
  if (unsubscribeLogs) unsubscribeLogs()
})
</script>

<style scoped>
.logs {
  padding: 20px;
  height: calc(100vh - 40px);
  display: flex;
  flex-direction: column;
}

h1 {
  color: #cba376;
  margin-bottom: 20px;
}

/* 控制栏 */
.controls {
  display: flex;
  justify-content: space-between;
  align-items: center;
  background: #2a2a2a;
  padding: 15px 20px;
  border-radius: 8px;
  margin-bottom: 20px;
}

.filter-group {
  display: flex;
  align-items: center;
  gap: 10px;
}

.filter-group label {
  color: #cba376;
  font-weight: 500;
}

.filter-group select {
  padding: 6px 12px;
  background: #1a1a1a;
  color: #cba376;
  border: 1px solid #444;
  border-radius: 4px;
  cursor: pointer;
  font-size: 0.9em;
}

.filter-group select:focus {
  outline: none;
  border-color: #cba376;
}

.button-group {
  display: flex;
  gap: 10px;
}

.btn-clear,
.btn-scroll {
  padding: 8px 16px;
  border: none;
  border-radius: 4px;
  cursor: pointer;
  font-size: 0.9em;
  transition: background 0.3s;
}

.btn-clear {
  background: #d9534f;
  color: white;
}

.btn-clear:hover {
  background: #c9302c;
}

.btn-scroll {
  background: #5bc0de;
  color: white;
}

.btn-scroll:hover {
  background: #46b8da;
}

/* 日志容器 */
.logs-container {
  flex: 1;
  background: #1a1a1a;
  border-radius: 8px;
  padding: 15px;
  overflow-y: auto;
  font-family: 'Consolas', 'Monaco', monospace;
  font-size: 0.9em;
}

.no-logs {
  text-align: center;
  color: #999;
  padding: 40px;
}

/* 日志条目 */
.log-entry {
  padding: 6px 0;
  border-bottom: 1px solid #2a2a2a;
  display: flex;
  gap: 10px;
}

.log-entry:hover {
  background: #252525;
}

.log-time {
  color: #888;
  min-width: 100px;
}

.log-level {
  font-weight: bold;
  min-width: 60px;
}

.log-session {
  color: #5bc0de;
  min-width: 50px;
}

.log-message {
  color: #cba376;
  flex: 1;
  word-wrap: break-word;
}

/* 不同级别的颜色 */
.log-debug .log-level {
  color: #888;
}

.log-info .log-level {
  color: #5bc0de;
}

.log-warn .log-level {
  color: #f0ad4e;
}

.log-error .log-level {
  color: #d9534f;
}

/* 滚动条样式 */
.logs-container::-webkit-scrollbar {
  width: 8px;
}

.logs-container::-webkit-scrollbar-track {
  background: #1a1a1a;
}

.logs-container::-webkit-scrollbar-thumb {
  background: #444;
  border-radius: 4px;
}

.logs-container::-webkit-scrollbar-thumb:hover {
  background: #555;
}
</style>