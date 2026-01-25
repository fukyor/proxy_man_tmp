// config.js - 存储 API 配置
// const apiConfig = {
//   baseURL: 'http://127.0.0.1:9090',  // API 地址
//   secret: '你的密钥'                   // 密钥（可选）
// };


/**
 * 生成请求头
 * @param {string} secret - API 密钥（可选）
 * @returns {Object} 请求头对象
 */
export function generateHeaders(secret) {
  // 基础请求头
  const headers = {
    'Content-Type': 'application/json'
  };

  // 如果有密钥，添加 Authorization
  if (secret) {
    headers['Authorization'] = `Bearer ${secret}`;
  }

  return headers;
}


/**
 * 发送 API 请求
 * @param {Object} config - API 配置 {baseURL, secret}
 * @param {string} endpoint - API 端点，如 '/configs'
 * @param {Object} options - fetch 额外选项
 * @returns {Promise} 返回 JSON 数据
 */
export async function fetchAPI(config, endpoint, options = {}) {
  const headers = generateHeaders(config.secret);
  const url = config.baseURL + endpoint;
  // 合并请求选项
  const fetchOptions = {
    ...options,           // 用户自定义选项（如 method, body）
    headers: headers      // 加上我们的 headers
  };

  const response = await fetch(url, fetchOptions);
  if (response.status === 401) {
    throw new Error('认证失败：密钥错误或缺失');
  }

  if (!response.ok) {
    throw new Error(`请求失败：${response.status} ${response.statusText}`);
  }

  return await response.json();
}


/**
 * 构建 WebSocket URL
 * @param {Object} config - API 配置 {baseURL, secret}
 * @param {string} endpoint - WebSocket 端点，如 '/logs'
 * @returns {string} 完整的 WebSocket URL
 */
export function buildWebSocketURL(config, endpoint) {
  // 第一步：解析 baseURL
  const url = new URL(config.baseURL);

  // 第二步：转换协议（http → ws, https → wss）
  if (url.protocol === 'https:') {
    url.protocol = 'wss:';
  } else {
    url.protocol = 'ws:';
  }

  // 第三步：构建查询参数
  const params = new URLSearchParams();
  if (config.secret) {
    params.set('token', config.secret);
  }

  // 第四步：拼接完整 URL
  const wsURL = `${url.origin}${endpoint}?${params.toString()}`;

  return wsURL;
}

/**
 * 连接到日志流
 * @param {Object} config - API 配置
 * @param {Function} onMessage - 收到消息时的回调
 * @param {Function} onError - 发生错误时的回调
 */
export function connectToLogs(config, onMessage, onError) {
  // 第一步：生成 WebSocket URL
  const wsURL = buildWebSocketURL(config, '/logs');
  console.log('连接到：', wsURL);

  // 第二步：创建 WebSocket 连接
  const ws = new WebSocket(wsURL);

  // 第三步：设置事件监听
  ws.onopen = () => {
    console.log('✓ WebSocket 已连接');
  };

  ws.onmessage = (event) => {
    const data = JSON.parse(event.data);
    onMessage(data);  // 调用用户提供的回调
  };

  ws.onerror = (error) => {
    console.error('✗ WebSocket 错误：', error);
    if (onError) onError(error);
  };

  ws.onclose = (event) => {
    if (event.code === 1008) {  // 1008 = Policy Violation (认证失败)
      console.error('✗ 认证失败：密钥错误');
    } else {
      console.log('WebSocket 已关闭');
    }
  };

  return ws;  // 返回 WebSocket 对象，方便外部控制
}