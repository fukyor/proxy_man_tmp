import { defineStore } from 'pinia'
import { ref } from 'vue'


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
  const logs = ref([])            // 日志列表
  const mitmExchanges = ref([])   // MITM 交换记录

  const MAX_HISTORY = 60
  const MAX_LOGS = 500
  const MAX_MITM_EXCHANGES = 1000

  // ==================== 订阅者回调 ====================
  const trafficSubscribers = ref(new Set())
  const connectionsSubscribers = ref(new Set())
  const logsSubscribers = ref(new Set())
  const mitmSubscribers = ref(new Set())

  // ==================== 连接管理 ====================

  /**
   * 连接到 WebSocket 服务器
   * @param {string} apiUrl - API 基础地址（如 http://127.0.0.1:8000）
   * @param {string} secret - 密钥
   */
  function connect(apiUrl, secret) {
    if (socket.value) disconnect()

    const wsProtocol = apiUrl.startsWith('https') ? 'wss:' : 'ws:'
    const wsHost = apiUrl.replace(/^https?:\/\//, '')
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
        if (!isConnected.value) connect(apiUrl, secret)
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
      case 'log':
        handleLog(msg.data)
        break
      case 'mitm_exchange':
        handleMITMExchange(msg.data)
        break
    }
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
    //console.log(JSON.stringify(data, null, 2));
    // 通知所有订阅者，订阅者就是不同的组件，它们把回调函数预先注册到trafficSubscribers
    connectionsSubscribers.value.forEach(cb => cb(data))
  }

  /**
   * 处理日志数据
   * @param {Object} data - 日志数据 { level, session, message, time }
   */
  function handleLog(data) {
    const item = { ...data, id: Math.random().toString(36).slice(2) }
    logs.value.push(item)
    if (logs.value.length > MAX_LOGS) {
      logs.value.shift()
    }
    // 通知所有订阅者
    logsSubscribers.value.forEach(cb => cb(item))
  }

  /**
   * 处理 MITM 交换数据
   * @param {Object} data - MITM 交换数据
   */
  function handleMITMExchange(data) {
    console.log(JSON.stringify(data, null, 2));
    const exchange = {
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

      // 扁平化响应字段
      statusCode: data.response?.statusCode || 0,
      status: data.response?.status || '',
      responseHeaders: data.response?.header || {},
      responseSize: data.response?.sumSize || 0,

      // 衍生属性
      hasResponse: !!(data.response && data.response.statusCode),
      hasError: !!data.error
    }

    mitmExchanges.value.push(exchange)
    if (mitmExchanges.value.length > MAX_MITM_EXCHANGES) {
      mitmExchanges.value.shift()
    }
    mitmSubscribers.value.forEach(cb => cb(exchange))
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

  // ==================== Return ====================

  return {
    socket,
    isConnected,
    subscriptions,
    trafficHistory,
    connections,
    logs,
    mitmExchanges,
    connect,
    disconnect,
    updateSubscriptions,
    closeAllConnections,
    subscribeTraffic,
    subscribeConnections,
    subscribeLogs,
    clearLogs,
    subscribeMITM,
    clearMitmExchanges
  }
})