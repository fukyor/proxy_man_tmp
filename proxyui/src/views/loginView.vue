<script setup>
import { ref, watch } from 'vue'
import { useRouter } from 'vue-router'
import { useWebSocketStore } from '@/stores/websocket'

const apiUrl = ref('http://127.0.0.1:8000')
const secret = ref('123')
const label = ref('')
const servers = ref([])

const wsStore = useWebSocketStore()
const router = useRouter()

// 监听连接状态，连接成功后跳转到仪表盘
watch(
  () => wsStore.isConnected,
  (connected) => {
    if (connected) {
      router.push('/dashboard')
    }
  }
)

const login = () => {
  wsStore.connect(apiUrl.value, secret.value)
  addServer()
}


const addServer = () => {
  // 检查是否已存在（可选）
  const exists = servers.value.some((s) => s.url === apiUrl.value && s.secret === secret.value)
  if (!exists) {
    servers.value.push({
      url: apiUrl.value,
      secret: secret.value,
    })
  }
}

</script>

<template>
  <div class="container">
    <div class="logo-container">
      <!-- Simple Cat SVG -->
      <svg
        width="80"
        height="80"
        viewBox="0 0 100 100"
        fill="none"
        stroke="#cba376"
        stroke-width="1.5"
        stroke-linecap="round"
        stroke-linejoin="round"
      >
        <path d="M25 75 L25 40 L15 20 L40 30 L60 30 L85 20 L75 40 L75 75 Q50 85 25 75 Z" />
        <circle cx="38" cy="48" r="3" />
        <circle cx="62" cy="48" r="3" />
        <path d="M45 60 Q50 63 55 60" />
      </svg>
    </div>

    <div class="form-container">
      <div class="row">
        <div class="col grow">
          <label>API Base URL</label>
          <input v-model="apiUrl" type="text" placeholder="http://127.0.0.1:8000" />
        </div>
        <div class="col shrink">
          <label>Secret(optional)</label>
          <input v-model="secret" type="text" placeholder="" />
        </div>
      </div>

      <div class="row">
        <div class="col grow">
          <label>Label(optional)</label>
          <input v-model="label" type="text" placeholder="" />
        </div>
      </div>

      <div class="actions">
        <button @click="login">Add</button>
      </div>
    </div>

    <div class="list-container">
      <div v-for="(server, index) in servers" :key="index" class="server-item">
        <div class="server-url">{{ server.url }}</div>
        <div class="server-secret">{{ server.secret }}</div>
      </div>
    </div>

    <div class="footer-icon">
      <!-- Moon Icon -->
      <svg
        width="20"
        height="20"
        viewBox="0 0 24 24"
        fill="none"
        stroke="#886e52"
        stroke-width="2"
        stroke-linecap="round"
        stroke-linejoin="round"
      >
        <path d="M21 12.79A9 9 0 1 1 11.21 3 7 7 0 0 0 21 12.79z"></path>
      </svg>
    </div>
  </div>
</template>

<style scoped>
.container {
  display: flex;
  flex-direction: column;
  gap: 3rem;
  position: relative;
  min-height: 80vh;
  width: 100%;
  align-items: center;
}

.logo-container {
  display: flex;
  justify-content: center;
  margin-top: 1rem;
  margin-bottom: 1rem;
  width: 100%;
}

.form-container {
  display: flex;
  flex-direction: column;
  gap: 2rem;
  width: 100%;
  max-width: 700px;
  padding: 0 1rem;
}

.row {
  display: flex;
  gap: 2rem;
  flex-wrap: wrap;
}

.col {
  display: flex;
  flex-direction: column;
}

.col.grow {
  flex-grow: 1;
  min-width: 200px;
}

.col.shrink {
  width: 150px;
  flex-shrink: 0;
}

.actions {
  display: flex;
  justify-content: flex-end;
}

label {
  font-size: 0.8rem;
  color: #886e52;
  margin-bottom: 0.5rem;
}

input {
  background: transparent;
  border: none;
  border-bottom: 1px solid #3a3a3a;
  color: #cba376;
  padding: 8px 0;
  font-size: 1rem;
  width: 100%;
  outline: none;
  transition: border-color 0.3s;
}

input:focus {
  border-bottom-color: #cba376;
}

input::placeholder {
  color: #4a4a4a;
}

button {
  background: transparent;
  border: 1px solid #3a3a3a;
  color: #cba376;
  padding: 8px 32px;
  border-radius: 20px;
  cursor: pointer;
  transition: all 0.3s;
  font-size: 0.9rem;
}

button:hover {
  border-color: #cba376;
  background: rgba(203, 163, 118, 0.05);
}

.list-container {
  display: flex;
  flex-direction: column;
  gap: 1rem;
  margin-top: 1rem;
  width: 100%;
  max-width: 700px;
  padding: 0 1rem;
}

.server-item {
  background: rgba(255, 255, 255, 0.02);
  padding: 1.5rem;
  border-radius: 8px;
  border: 1px solid #2a2a2a;
  text-align: left;
}

.server-url {
  font-size: 1rem;
  margin-bottom: 0.5rem;
  color: #cba376;
}

.server-secret {
  color: #cba376;
  font-weight: bold;
  letter-spacing: 2px;
}

.footer-icon {
  position: absolute;
  bottom: 0;
  left: 0;
  cursor: pointer;
  padding: 10px;
}
</style>
