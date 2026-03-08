import { createRouter, createWebHistory } from 'vue-router'
import LoginView from '../views/loginView.vue'
import DashboardView from '../views/dashboard.vue'
import Overview from '../views/Overview.vue'
import Connections from '../views/Connections.vue'
import HistoryConnections from '../views/HistoryConnections.vue'
import Logs from '../views/Logs.vue'
import MITM from '../views/MITM.vue'
import RouteConfig from '../views/RouteConfig.vue'
import AdvancedConfig from '../views/AdvancedConfig.vue'
import { useWebSocketStore } from '../stores/websocket'

const router = createRouter({
  history: createWebHistory(import.meta.env.BASE_URL),
  routes: [
    {
      path: '/',
      name: 'login',
      component: LoginView,
    },
    {
      path: '/dashboard',
      component: DashboardView,
      children: [
        {
          path: '',
          redirect: '/dashboard/overview',
        },
        {
          path: 'overview',
          name: 'overview',
          component: Overview,
        },
        {
          path: 'connections',
          name: 'connections',
          component: Connections,
        },
        {
          path: 'history-connections',
          name: 'history-connections',
          component: HistoryConnections,
        },
        {
          path: 'logs',
          name: 'logs',
          component: Logs,
        },
        {
          path: 'mitm',
          name: 'mitm',
          component: MITM,
        },
        {
          path: 'route-config',
          name: 'route-config',
          component: RouteConfig,
        },
        {
          path: 'advanced-config',
          name: 'advanced-config',
          component: AdvancedConfig,
        },
      ],
    },
  ],
})

router.beforeEach((to, from, next) => {
  // 必须在函数内部引入或调用 useWebSocketStore，确保 Pinia 已经挂载
  const wsStore = useWebSocketStore()
  
  // 判断目标路由是否是登录页，这里的登录页 path 是 '/'
  const isLoginPage = to.path === '/'
  
  if (!isLoginPage && !wsStore.isConnected) {
    console.log('[Route Guard] 未连接 WebSocket，拦截跳转并重定向至首页。想要访问的路径:', to.path)
    next({ path: '/' })
  } else {
    next()
  }
})

export default router
