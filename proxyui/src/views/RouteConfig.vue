<script setup>
import { ref, onMounted, computed, watch } from 'vue'
import { useWebSocketStore } from '@/stores/websocket.js'

const wsStore = useWebSocketStore()

const loading = ref(false)
const saving = ref(false)
const errorMsg = ref('')
const successMsg = ref('')

// 本地配置副本（用于编辑）
const localConfig = ref({
  RouteEnable: false,
  ProxyNodes: [],
  Routes: []
})

// 原始配置深拷贝用于对比，以检测是否有更改
const originalConfigStr = ref('')

const hasChanges = computed(() => {
  if (!originalConfigStr.value) return false
  return JSON.stringify(localConfig.value) !== originalConfigStr.value
})

// 新节点输入
const newNode = ref({ Name: '', URL: '' })
// 新规则输入
const newRule = ref({ Type: 'DomainSuffix', Value: '', Action: 'Direct', Enable: true, Remarks: '' })

const ruleValuePlaceholder = computed(() => {
  switch (newRule.value.Type) {
    case 'DomainSuffix':
      return 'eg. example.com, www.example.com'
    case 'DomainKeyword':
      return 'eg. ^baidu\\.com$, \\.cn$'
    case 'IP':
      return 'eg. 1.1.1.1, 8.8.8.8'
    default:
      return '规则值'
  }
})

// 所有可选目标节点（Direct + 已配置节点）
const availableTargets = computed(() => {
  const names = localConfig.value.ProxyNodes.map(n => n.Name).filter(Boolean)
  return ['Direct', ...names]
})

// 规则类型选项
const ruleTypes = [
  { value: 'DomainSuffix', label: '域名后缀' },
  { value: 'DomainKeyword', label: '域名关键词' },
  { value: 'IP', label: 'IP 精确' }
]

// 下一个规则 ID（前端自增）
const nextRuleId = computed(() => {
  const ids = localConfig.value.Routes.map(r => r.Id)
  return ids.length > 0 ? Math.max(...ids) + 1 : 1
})

// 监听并提示中文逗号
watch(() => newRule.value.Value, (newVal) => {
  if (newVal && newVal.includes('，')) {
    errorMsg.value = '检测到中文逗号 "，" ，已自动替换为英文逗号'
    newRule.value.Value = newVal.replace(/，/g, ',')
    setTimeout(() => {
      if (errorMsg.value === '检测到中文逗号 "，" ，已自动替换为英文逗号') {
        errorMsg.value = ''
      }
    }, 3000)
  }
})

onMounted(async () => {
  await loadConfig()
})

async function loadConfig() {
  loading.value = true
  errorMsg.value = ''
  try {
    const cfg = await wsStore.loadConfig()
    localConfig.value = {
      RouteEnable: cfg.RouteEnable ?? false,
      ProxyNodes: cfg.ProxyNodes ?? [],
      Routes: cfg.Routes ?? []
    }
    originalConfigStr.value = JSON.stringify(localConfig.value)
  } catch (e) {
    errorMsg.value = `加载配置失败: ${e.message}`
  } finally {
    loading.value = false
  }
}

async function saveConfig() {
  saving.value = true
  errorMsg.value = ''
  successMsg.value = ''
  try {
    // 合并完整配置（保留其他字段）
    const current = wsStore.config
    const merged = {
      ...current,
      RouteEnable: localConfig.value.RouteEnable,
      ProxyNodes: localConfig.value.ProxyNodes,
      Routes: localConfig.value.Routes
    }
    await wsStore.saveConfig(merged)
    originalConfigStr.value = JSON.stringify(localConfig.value) // 保存成功后更新原始数据对照模板
    successMsg.value = '配置已保存'
    setTimeout(() => { successMsg.value = '' }, 3000)
  } catch (e) {
    errorMsg.value = `保存失败: ${e.message}`
  } finally {
    saving.value = false
  }
}

// 节点操作
function addNode() {
  if (!newNode.value.Name || !newNode.value.URL) return

  // 验证代理地址格式，要求以协议开头（如http://），且包含端口号（如:7892）
  const urlPattern = /^[a-zA-Z]+:\/\/.+:\d+$/
  if (!urlPattern.test(newNode.value.URL)) {
    errorMsg.value = '代理地址格式不合法，请输入完整的协议、地址和端口（如 http://127.0.0.1:7892）'
    setTimeout(() => {
      if (errorMsg.value === '代理地址格式不合法，请输入完整的协议、地址和端口（如 http://127.0.0.1:7892）') {
        errorMsg.value = ''
      }
    }, 3000)
    return
  }

  localConfig.value.ProxyNodes.push({ ...newNode.value })
  newNode.value = { Name: '', URL: '' }
}

function removeNode(index) {
  localConfig.value.ProxyNodes.splice(index, 1)
}

// 规则操作
function addRule() {
  if (!newRule.value.Value || !newRule.value.Action) return
  
  // 去除逗号前后的空格，确保全部清除
  const cleanValue = newRule.value.Value.split(',').map(v => v.trim()).filter(Boolean).join(',')

  localConfig.value.Routes.push({
    Id: nextRuleId.value,
    ...newRule.value,
    Value: cleanValue
  })
  newRule.value = { Type: 'DomainSuffix', Value: '', Action: 'Direct', Enable: true, Remarks: '' }
}

function removeRule(index) {
  localConfig.value.Routes.splice(index, 1)
}

function getRuleTypeLabel(type) {
  return ruleTypes.find(t => t.value === type)?.label || type
}
</script>

<template>
  <div class="route-config">
    <div class="page-header">
      <h2>路由配置</h2>
      <div class="header-actions">
        <button class="btn btn-secondary" @click="loadConfig" :disabled="loading">
          {{ loading ? '加载中...' : '刷新' }}
        </button>
        <div class="save-btn-container">
          <button class="btn btn-primary" @click="saveConfig" :disabled="saving">
            {{ saving ? '保存中...' : '保存配置' }}
          </button>
          <span v-if="hasChanges" class="badge-dot"></span>
        </div>
      </div>
    </div>

    <div v-if="errorMsg" class="alert alert-error">{{ errorMsg }}</div>
    <div v-if="successMsg" class="alert alert-success">{{ successMsg }}</div>

    <!-- 全局设置 -->
    <div class="section">
      <h3>全局设置</h3>
      <label class="toggle-label">
        <span>启用路由规则</span>
        <div class="switch">
          <input type="checkbox" v-model="localConfig.RouteEnable" />
          <span class="slider"></span>
        </div>
        <span class="toggle-text">{{ localConfig.RouteEnable ? '已启用' : '已禁用' }}</span>
      </label>
      <p class="hint">启用后，流量将按规则分发到不同出站节点；不匹配任何规则时默认直连。</p>
    </div>

    <!-- 代理节点 -->
    <div class="section">
      <h3>代理节点</h3>
      <table class="data-table" v-if="localConfig.ProxyNodes.length > 0">
        <thead>
          <tr>
            <th>节点名称</th>
            <th>代理地址</th>
            <th>操作</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="(node, i) in localConfig.ProxyNodes" :key="i">
            <td>{{ node.Name }}</td>
            <td>{{ node.URL }}</td>
            <td><button class="btn btn-danger-sm" @click="removeNode(i)">删除</button></td>
          </tr>
        </tbody>
      </table>
      <p v-else class="empty-hint">暂无代理节点</p>

      <!-- 添加节点 -->
      <div class="add-row">
        <input v-model="newNode.Name" placeholder="节点名称（如 clash）" class="input" />
        <input v-model="newNode.URL" placeholder="代理地址（如 http://127.0.0.1:7892）" class="input input-wide" />
        <button class="btn btn-add" @click="addNode">添加节点</button>
      </div>
    </div>

    <!-- 路由规则 -->
    <div class="section">
      <h3>路由规则</h3>
      <p class="hint">不匹配任何规则的流量默认直连（Direct）。</p>
      <table class="data-table" v-if="localConfig.Routes.length > 0">
        <thead>
          <tr>
            <th>启用</th>
            <th>类型</th>
            <th>值</th>
            <th>目标节点</th>
            <th>备注</th>
            <th>操作</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="(rule, i) in localConfig.Routes" :key="rule.Id">
            <td><input type="checkbox" v-model="rule.Enable" /></td>
            <td>{{ getRuleTypeLabel(rule.Type) }}</td>
            <td>{{ rule.Value }}</td>
            <td>{{ rule.Action }}</td>
            <td>{{ rule.Remarks }}</td>
            <td><button class="btn btn-danger-sm" @click="removeRule(i)">删除</button></td>
          </tr>
        </tbody>
      </table>
      <p v-else class="empty-hint">暂无路由规则</p>

      <!-- 添加规则 -->
      <div class="add-row add-rule-row">
        <select v-model="newRule.Type" class="input input-select">
          <option v-for="t in ruleTypes" :key="t.value" :value="t.value">{{ t.label }}</option>
        </select>
        <input v-model="newRule.Value" :placeholder="ruleValuePlaceholder" class="input flex-fill" spellcheck="false" />
        <select v-model="newRule.Action" class="input input-select">
          <option v-for="t in availableTargets" :key="t" :value="t">{{ t }}</option>
        </select>
        <input v-model="newRule.Remarks" placeholder="备注（可选）" class="input" />
        <label class="checkbox-label">
          <input type="checkbox" v-model="newRule.Enable" />
          <span>启用</span>
        </label>
        <button class="btn btn-add" @click="addRule">添加规则</button>
      </div>
    </div>
  </div>
</template>

<style scoped>
.route-config {
  color: #cba376;
  max-width: 1000px;
}

.page-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 24px;
}

.page-header h2 {
  font-size: 1.4rem;
  font-weight: 600;
  margin: 0;
}

.header-actions {
  display: flex;
  gap: 10px;
}

.section {
  background: #2a2a2a;
  border: 1px solid #333;
  border-radius: 8px;
  padding: 20px;
  margin-bottom: 20px;
}

.section h3 {
  margin: 0 0 16px;
  font-size: 1rem;
  font-weight: 600;
  color: #cba376;
}

.toggle-label {
  display: flex;
  align-items: center;
  gap: 12px;
  cursor: pointer;
}

/* Switch 开关样式 */
.switch {
  position: relative;
  display: inline-block;
  width: 44px;
  height: 24px;
}

.switch input {
  opacity: 0;
  width: 0;
  height: 0;
}

.slider {
  position: absolute;
  cursor: pointer;
  top: 0;
  left: 0;
  right: 0;
  bottom: 0;
  background-color: #333; /* 关状态背景色 */
  transition: .3s;
  border-radius: 24px;
}

.slider:before {
  position: absolute;
  content: "";
  height: 18px;
  width: 18px;
  left: 3px;
  bottom: 3px;
  background-color: #888; /* 关状态滑块色 */
  transition: .3s;
  border-radius: 50%;
}

input:checked + .slider {
  background-color: #0d4a65; /* 开状态背景色（参考图片配色） */
}

input:checked + .slider:before {
  transform: translateX(20px);
  background-color: #cba376; /* 开状态滑块色（参考图片配色） */
}

.toggle-text {
  font-size: 0.85rem;
  color: #888;
}

.hint {
  margin: 8px 0 0;
  font-size: 0.8rem;
  color: #666;
}

.empty-hint {
  color: #555;
  font-size: 0.85rem;
  margin: 8px 0;
}

.data-table {
  width: 100%;
  border-collapse: collapse;
  margin-bottom: 16px;
  font-size: 0.85rem;
}

.data-table th,
.data-table td {
  padding: 8px 12px;
  border-bottom: 1px solid #333;
  text-align: left;
}

.data-table th {
  color: #888;
  font-weight: 500;
}

.add-row {
  display: flex;
  gap: 8px;
  flex-wrap: wrap;
  align-items: center;
  margin-top: 12px;
}

.add-rule-row {
  flex-wrap: wrap;
}

.flex-fill {
  flex: 1 1 300px; /* 占据剩余空间，最小宽度 300px */
}

.input {
  background: #1a1a1a;
  border: 1px solid #444;
  border-radius: 4px;
  color: #cba376;
  padding: 6px 10px;
  font-size: 0.85rem;
  min-width: 140px;
}

.input-wide {
  min-width: 260px;
}

.input-select {
  min-width: 120px;
  cursor: pointer;
}

.input:focus {
  outline: none;
  border-color: #cba376;
}

.checkbox-label {
  display: flex;
  align-items: center;
  gap: 6px;
  font-size: 0.85rem;
}

.btn {
  padding: 7px 16px;
  border: none;
  border-radius: 4px;
  cursor: pointer;
  font-size: 0.85rem;
  transition: opacity 0.2s;
}

.btn:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}

.btn-primary {
  background: #cba376;
  color: #1a1a1a;
  font-weight: 600;
}

.btn-secondary {
  background: #333;
  color: #cba376;
  border: 1px solid #444;
}

.save-btn-container {
  position: relative;
  display: inline-block;
}

.badge-dot {
  position: absolute;
  top: -4px;
  right: -4px;
  width: 8px;
  height: 8px;
  background-color: #ff4d4f; /* 醒目的红色 */
  border-radius: 50%;
  box-shadow: 0 0 0 2px #2a2a2a; /* 添加一点背景色边缘作为呼吸感/分隔感 */
}

.btn-add {
  background: #2d4a3e;
  color: #5fad8a;
  border: 1px solid #3a6b55;
}

.btn-danger-sm {
  background: transparent;
  color: #e06c75;
  border: 1px solid #e06c75;
  padding: 3px 10px;
  font-size: 0.8rem;
}

.btn-danger-sm:hover {
  background: rgba(224, 108, 117, 0.1);
}

.alert {
  padding: 10px 16px;
  border-radius: 4px;
  margin-bottom: 16px;
  font-size: 0.85rem;
}

.alert-error {
  background: rgba(224, 108, 117, 0.15);
  color: #e06c75;
  border: 1px solid #e06c75;
}

.alert-success {
  background: rgba(95, 173, 138, 0.15);
  color: #5fad8a;
  border: 1px solid #5fad8a;
}
</style>
