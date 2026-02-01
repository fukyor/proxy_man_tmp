import { createRouter, createWebHistory } from 'vue-router'
import LoginView from '../views/loginView.vue'
import DashboardView from '../views/dashboard.vue'
import Overview from '../views/Overview.vue'
import Connections from '../views/Connections.vue'
import Logs from '../views/Logs.vue'
import MITM from '../views/MITM.vue'

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
          path: 'logs',
          name: 'logs',
          component: Logs,
        },
        {
          path: 'mitm',
          name: 'mitm',
          component: MITM,
        },
      ],
    },
  ],
})

export default router
