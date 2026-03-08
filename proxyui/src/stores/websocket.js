import { defineStore } from 'pinia'
import { ref } from 'vue'
import { fetchConfig, updateConfig } from '@/api/api.js'


export const useWebSocketStore = defineStore('websocket', () => {
  // ==================== 状态 ====================
  const socket = ref(null)
  const isConnected = ref(false)
  const subscriptions = ref({
    traffic: true,
    connections: true,
    logs: true,
    logLevel: 'INFO',
    mitm: true
  })

  // ==================== 数据 ====================
  const trafficHistory = ref([])  // 最近 60 秒流量历史
  const connections = ref([])      // 当前连接列表
  const historyConnections = ref([]) // 历史闭合连接
  const historyConnectionsSet = new Set() // 去重闭合连接
  const logs = ref([])            // 日志列表
  const mitmExchanges = ref([])   // MITM 交换记录
  const apiUrl = ref('')           // API 基础地址（用于下载等 HTTP 请求）

  const MAX_HISTORY = 60  // 最近 60 秒流量历史
  const MAX_LOGS = 500
  const MAX_MITM_EXCHANGES = 2000
  const MAX_SNAPSHOT_CONNECTIONS = 3000  // 详细连接界面只展示前3000条连接
  const MAX_HISTORY_CONNECTIONS = 1000 // 历史连接最多展示1000条

  // 路由配置状态
  const config = ref(null)

  // ==================== 订阅者回调 ====================
  const trafficSubscribers = ref(new Set())
  const connectionsSubscribers = ref(new Set())
  const historyConnectionsSubscribers = ref(new Set())
  const logsSubscribers = ref(new Set())
  const mitmSubscribers = ref(new Set())

  // ==================== 连接管理 ====================

  /**
   * 连接到 WebSocket 服务器
   * @param {string} apiUrl - API 基础地址（如 http://127.0.0.1:8000）
   * @param {string} secret - 密钥
   */
  function connect(apiUrlParam, secret) {
    if (socket.value) disconnect()

    apiUrl.value = apiUrlParam

    const wsProtocol = apiUrlParam.startsWith('https') ? 'wss:' : 'ws:'
    const wsHost = apiUrlParam.replace(/^https?:\/\//, '')
    const wsUrl = `${wsProtocol}//${wsHost}/start?token=${secret || ''}`

    socket.value = new WebSocket(wsUrl)

    socket.value.onopen = () => {
      isConnected.value = true
      console.log('WebSocket 连接成功')
      subscribe()
    }

    socket.value.onmessage = (event) => {
      const msg = JSON.parse(event.data)
      handleMessage(msg)
    }

    socket.value.onclose = () => {
      isConnected.value = false
      console.log('WebSocket 连接已关闭')
      // 5 秒后自动重连
      setTimeout(() => {
        if (!isConnected.value) connect(apiUrlParam, secret)
      }, 5000)
    }

    socket.value.onerror = (error) => {
      isConnected.value = false
      console.error('WebSocket 错误:', error)
    }
  }

  /**
   * 断开连接
   */
  function disconnect() {
    if (socket.value) {
      socket.value.close()
      socket.value = null
      isConnected.value = false
    }
  }

  /**
   * 发送订阅消息到服务器
   */
  function subscribe() {
    if (!socket.value || socket.value.readyState !== WebSocket.OPEN) return

    socket.value.send(JSON.stringify({
      action: 'subscribe',
      topics: [
        subscriptions.value.traffic && 'traffic',
        subscriptions.value.connections && 'connections',
        subscriptions.value.logs && 'logs',
        subscriptions.value.mitm && 'mitm_detail'
      ].filter(Boolean),
      logLevel: subscriptions.value.logLevel
    }))
  }

  /**
   * 更新订阅配置
   * @param {Object} newSubs - 新的订阅配置
   */
  function updateSubscriptions(newSubs) {
    subscriptions.value = { ...subscriptions.value, ...newSubs }
    subscribe()
  }

  /**
   * 关闭所有连接
   */
  function closeAllConnections() {
    if (socket.value?.readyState === WebSocket.OPEN) {
      socket.value.send(JSON.stringify({ action: 'closeAllConnections' }))
    }
  }

  /**
   * 关闭单条连接
   * @param {number} id - 连接 Session ID
   */
  function closeConnection(id) {
    if (socket.value?.readyState === WebSocket.OPEN) {
      socket.value.send(JSON.stringify({ action: 'closeConnection', id }))
    }
  }

  // ==================== 消息处理 ====================

  /**
   * 处理接收到的消息
   * @param {Object} msg - 消息对象
   */
  function handleMessage(msg) {
    switch (msg.type) {
      case 'traffic':
        handleTraffic(msg.data)
        break
      case 'connections':
        handleConnections(msg.data)
        break
      case 'log_batch':
        handleLogBatch(msg.data)
        break
      case 'mitm_exchange_batch':
        handleMITMExchangeBatch(msg.data)
        break
    }
  }

  function handleMITMExchangeBatch(dataArray) {
    if (!dataArray || dataArray.length === 0) return
    const newExchanges = dataArray.map(buildExchangeObject)
    const updated = mitmExchanges.value.concat(newExchanges)
    mitmExchanges.value = updated.length > MAX_MITM_EXCHANGES
      ? updated.slice(-MAX_MITM_EXCHANGES)
      : updated
    mitmSubscribers.value.forEach(cb => cb())
  }

  /**
   * 处理全局流量数据
   * @param {Object} data - 流量数据 { up, down }
   */
  function handleTraffic(data) {
   // console.log(JSON.stringify(data, null, 2));
    const item = { ...data, timestamp: Date.now() }
    trafficHistory.value.push(item)
    if (trafficHistory.value.length > MAX_HISTORY) {
      trafficHistory.value.shift()
    }
    // 通知所有订阅者，订阅者就是不同的组件，它们把回调函数预先注册到trafficSubscribers
    trafficSubscribers.value.forEach(cb => cb(item))
  }

  /**
   * 处理连接数据
   * @param {Array} data - 连接列表
   */
  function handleConnections(data) {
    const currentMap = new Map()
    data.forEach(c => currentMap.set(c.id, c))

    const newHistory = []

    // 1. 之前在 active 列表中，现在消失了，或者状态变成 Closed 了
    connections.value.forEach(c => {
      const newC = currentMap.get(c.id)
      if (!newC || newC.status === 'Closed') {
        if (!historyConnectionsSet.has(c.id)) {
          // 补充最后一次的状态
          const historyItem = newC ? { ...newC, status: 'Closed' } : { ...c, status: 'Closed' }
          newHistory.push(historyItem)
          historyConnectionsSet.add(historyItem.id)
        }
      }
    })

    // 2. 新推送过来的直接就是 Closed 状态的
    data.forEach(c => {
      if (c.status === 'Closed' && !historyConnectionsSet.has(c.id)) {
        newHistory.push({ ...c })
        historyConnectionsSet.add(c.id)
      }
    })

    if (newHistory.length > 0) {
      historyConnections.value = [...newHistory, ...historyConnections.value]
      if (historyConnections.value.length > MAX_HISTORY_CONNECTIONS) {
        const removed = historyConnections.value.slice(MAX_HISTORY_CONNECTIONS)
        removed.forEach(c => historyConnectionsSet.delete(c.id))
        historyConnections.value = historyConnections.value.slice(0, MAX_HISTORY_CONNECTIONS)
      }
      historyConnectionsSubscribers.value.forEach(cb => cb(historyConnections.value))
    }

    const safeData = data.length > MAX_SNAPSHOT_CONNECTIONS
      ? data.slice(0, MAX_SNAPSHOT_CONNECTIONS)
      : data
    connections.value = safeData  // 写入 ref，供组件初始化时读取
    connectionsSubscribers.value.forEach(cb => cb(safeData))
  }

  /**
   * 批量处理日志数据，单次赋值触发一次 Vue 响应式更新
   * @param {Array} dataArray - 日志数组
   */
  function handleLogBatch(dataArray) {
    if (!dataArray || dataArray.length === 0) return

    const newLogs = dataArray.map(data => ({
      ...data,
      id: Math.random().toString(36).slice(2)
    }))

    const updatedLogs = logs.value.concat(newLogs)

    if (updatedLogs.length > MAX_LOGS) {
      logs.value = updatedLogs.slice(-MAX_LOGS)
    } else {
      logs.value = updatedLogs
    }

    logsSubscribers.value.forEach(cb => cb())
  }

  /**
   * 将原始数据转换为 exchange 对象（纯函数）
   * @param {Object} data - 原始 MITM 数据
   * @returns {Object} 扁平化的 exchange 对象
   */
  function buildExchangeObject(data) {
    return {
      // 元数据
      id: data.id,
      sessionId: data.sessionId,
      parentId: data.parentId,
      time: data.time,
      duration: data.duration,
      error: data.error || '',

      // 扁平化请求字段
      method: data.request?.method || '',
      url: data.request?.url || '',
      host: data.request?.host || '',
      requestHeaders: data.request?.header || {},
      requestSize: data.request?.sumSize || 0,

      // 请求体 MinIO 信息
      reqBodyKey: data.request?.bodyKey || '',
      reqBodySize: data.request?.bodySize || 0,
      reqBodyUploaded: data.request?.bodyUploaded || false,
      reqContentType: data.request?.contentType || '',
      reqBodyError: data.request?.bodyError || '',

      // 扁平化响应字段
      statusCode: data.response?.statusCode || 0,
      status: data.response?.status || '',
      responseHeaders: data.response?.header || {},
      responseSize: data.response?.sumSize || 0,

      // 响应体 MinIO 信息
      respBodyKey: data.response?.bodyKey || '',
      respBodySize: data.response?.bodySize || 0,
      respBodyUploaded: data.response?.bodyUploaded || false,
      respContentType: data.response?.contentType || '',
      respBodyError: data.response?.bodyError || '',

      // 衍生属性
      hasResponse: !!(data.response && data.response.statusCode),
      hasError: !!data.error
    }
  }

  // ==================== 订阅方法 ====================

  /**
   * 订阅流量更新
   * @param {Function} callback - 回调函数
   * @returns {Function} 取消订阅函数
   */
  function subscribeTraffic(callback) {
    trafficSubscribers.value.add(callback)
    return () => trafficSubscribers.value.delete(callback)
  }

  /**
   * 订阅连接更新
   * @param {Function} callback - 回调函数
   * @returns {Function} 取消订阅函数
   */
  function subscribeConnections(callback) {
    connectionsSubscribers.value.add(callback)
    return () => connectionsSubscribers.value.delete(callback)
  }

  /**
   * 订阅历史连接更新
   */
  function subscribeHistoryConnections(callback) {
    historyConnectionsSubscribers.value.add(callback)
    return () => historyConnectionsSubscribers.value.delete(callback)
  }

  function clearHistoryConnections() {
    historyConnections.value = []
    historyConnectionsSet.clear()
    historyConnectionsSubscribers.value.forEach(cb => cb([]))
  }

  /**
   * 订阅日志更新
   * @param {Function} callback - 回调函数
   * @returns {Function} 取消订阅函数
   */
  function subscribeLogs(callback) {
    logsSubscribers.value.add(callback)
    return () => logsSubscribers.value.delete(callback)
  }

  /**
   * 清除所有日志
   */
  function clearLogs() {
    logs.value = []
  }

  /**
   * 订阅 MITM 更新
   * @param {Function} callback - 回调函数
   * @returns {Function} 取消订阅函数
   */
  function subscribeMITM(callback) {
    mitmSubscribers.value.add(callback)
    return () => mitmSubscribers.value.delete(callback)
  }

  /**
   * 清除所有 MITM 交换记录
   */
  function clearMitmExchanges() {
    mitmExchanges.value = []
  }

  // ==================== 配置管理 ====================

  /**
   * 从服务器加载配置
   */
  async function loadConfig() {
    config.value = await fetchConfig(apiUrl.value)
    return config.value
  }

  /**
   * 保存配置到服务器
   * @param {Object} newConfig - 新配置
   */
  async function saveConfig(newConfig) {
    await updateConfig(apiUrl.value, newConfig)
    config.value = newConfig
  }

  // ==================== Return ====================

  return {
    socket,
    isConnected,
    subscriptions,
    trafficHistory,
    connections,
    historyConnections,
    logs,
    mitmExchanges,
    apiUrl,
    connect,
    disconnect,
    updateSubscriptions,
    closeAllConnections,
    closeConnection,
    subscribeTraffic,
    subscribeConnections,
    subscribeHistoryConnections,
    clearHistoryConnections,
    subscribeLogs,
    clearLogs,
    subscribeMITM,
    clearMitmExchanges,
    config,
    loadConfig,
    saveConfig
  }
})