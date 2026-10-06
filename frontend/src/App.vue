<script setup lang="ts">
import { navItems } from '@/router'
import { useBackendStatus } from '@/composables/useBackendStatus'

const { online, version } = useBackendStatus()
</script>

<template>
  <div class="shell">
    <aside class="sidebar">
      <div class="brand">Kairo</div>
      <nav>
        <RouterLink v-for="item in navItems" :key="item.path" :to="item.path">
          {{ item.label }}
        </RouterLink>
      </nav>
      <div class="status" :class="online ? 'online' : 'offline'">
        <span class="dot" />
        <span v-if="online">Backend online<template v-if="version"> ({{ version }})</template></span>
        <span v-else>Backend offline</span>
      </div>
    </aside>
    <main class="content">
      <RouterView />
    </main>
  </div>
</template>

<style scoped>
.shell {
  display: flex;
  height: 100%;
}
.sidebar {
  width: var(--sidebar-width);
  flex-shrink: 0;
  display: flex;
  flex-direction: column;
  padding: 16px 12px;
  background: var(--surface);
  border-right: 1px solid var(--border);
}
.brand {
  font-size: 20px;
  font-weight: 700;
  padding: 4px 10px 16px;
  color: var(--accent);
}
nav {
  display: flex;
  flex-direction: column;
  gap: 2px;
  flex: 1;
}
nav a {
  padding: 8px 10px;
  border-radius: 6px;
  color: var(--text);
  text-decoration: none;
}
nav a:hover {
  background: var(--bg);
}
nav a.router-link-active {
  background: var(--accent-soft);
  color: var(--accent);
  font-weight: 600;
}
.status {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 8px 10px;
  font-size: 13px;
  color: var(--text-muted);
}
.dot {
  width: 8px;
  height: 8px;
  border-radius: 50%;
  background: var(--err);
}
.online .dot {
  background: var(--ok);
}
.content {
  flex: 1;
  min-width: 0;
  overflow-y: auto;
  padding: 32px 40px;
}
</style>
