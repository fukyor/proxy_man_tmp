<script setup>
import { RouterLink, RouterView } from 'vue-router'
</script>

<template>
  <div class="dashboard-container">
    <aside class="sidebar">
      <!-- Logo or Header -->
      <div class="sidebar-header">
        <svg
          width="40"
          height="40"
          viewBox="0 0 100 100"
          fill="none"
          stroke="#cba376"
          stroke-width="2"
          stroke-linecap="round"
          stroke-linejoin="round"
        >
          <path d="M25 75 L25 40 L15 20 L40 30 L60 30 L85 20 L75 40 L75 75 Q50 85 25 75 Z" />
          <circle cx="38" cy="48" r="3" />
          <circle cx="62" cy="48" r="3" />
          <path d="M45 60 Q50 63 55 60" />
        </svg>
      </div>

      <nav class="nav-menu">
        <RouterLink to="/dashboard/overview" class="nav-item" active-class="active">
          <!-- Icon for Overview -->
          <svg
            width="18"
            height="18"
            viewBox="0 0 24 24"
            fill="none"
            stroke="currentColor"
            stroke-width="2"
            stroke-linecap="round"
            stroke-linejoin="round"
          >
            <rect x="3" y="3" width="7" height="7"></rect>
            <rect x="14" y="3" width="7" height="7"></rect>
            <rect x="14" y="14" width="7" height="7"></rect>
            <rect x="3" y="14" width="7" height="7"></rect>
          </svg>
          <span>概览</span>
        </RouterLink>

        <RouterLink to="/dashboard/logs" class="nav-item" active-class="active">
          <!-- Icon for Logs -->
          <svg
            width="18"
            height="18"
            viewBox="0 0 24 24"
            fill="none"
            stroke="currentColor"
            stroke-width="2"
            stroke-linecap="round"
            stroke-linejoin="round"
          >
            <path d="M14 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8z"></path>
            <polyline points="14 2 14 8 20 8"></polyline>
            <line x1="16" y1="13" x2="8" y2="13"></line>
            <line x1="16" y1="17" x2="8" y2="17"></line>
            <polyline points="10 9 9 9 8 9"></polyline>
          </svg>
          <span>日志</span>
        </RouterLink>
      </nav>
    </aside>

    <main class="main-content">
      <RouterView />
    </main>
  </div>
</template>

<style scoped>
.dashboard-container {
  display: flex;
  height: 100vh;
  width: 100%;
  background-color: #1a1a1a;
  color: #cba376;
}

.sidebar {
  width: 200px;
  background-color: #222;
  display: flex;
  flex-direction: column;
  padding: 20px 10px;
  border-right: 1px solid #333;
}

.sidebar-header {
  display: flex;
  justify-content: center;
  margin-bottom: 40px;
  margin-top: 20px;
}

.nav-menu {
  display: flex;
  flex-direction: column;
  gap: 10px;
}

.nav-item {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 10px 15px;
  color: #888;
  text-decoration: none;
  border-radius: 8px;
  transition: all 0.3s;
}

.nav-item:hover {
  background-color: rgba(203, 163, 118, 0.1);
  color: #cba376;
}

.nav-item.active {
  background-color: rgba(203, 163, 118, 0.15);
  color: #cba376;
  font-weight: 500;
}

.main-content {
  flex-grow: 1;
  padding: 20px;
  overflow-y: auto;
}
</style>
