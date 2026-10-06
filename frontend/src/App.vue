<script setup lang="ts">
import { RouterLink, RouterView } from 'vue-router'
import ConfirmPopup from 'primevue/confirmpopup'
import { navItems } from '@/router'
import { useBackendStatus } from '@/composables/useBackendStatus'

const { online, version } = useBackendStatus()
</script>

<template>
  <div class="shell">
    <aside class="sidebar">
      <div class="brand">Kairo</div>
      <nav>
        <RouterLink v-for="item in navItems" :key="item.path" :to="item.path">{{ item.label }}</RouterLink>
      </nav>
      <div class="status muted"><i :class="['dot', { on: online }]" />{{ online ? `Backend online${version ? ` (${version})` : ''}` : 'Backend offline' }}</div>
    </aside>
    <main class="content"><RouterView /></main>
    <ConfirmPopup />
  </div>
</template>

<style scoped>
.shell { display: flex; height: 100%; }
.sidebar {
  width: 220px; flex-shrink: 0; display: flex; flex-direction: column; gap: 4px; padding: 16px 12px;
  background: var(--p-content-background); border-right: 1px solid var(--k-border-subtle);
}
.brand { font-size: 18px; font-weight: 600; letter-spacing: -0.03em; padding: 4px 10px 16px; }
nav { display: flex; flex-direction: column; gap: 2px; flex: 1; }
nav a { padding: 7px 10px; border-radius: 6px; color: var(--p-text-muted-color); border-left: 2px solid transparent; text-decoration: none; }
nav a:hover { background: var(--p-content-hover-background); color: var(--p-text-color); }
nav a.router-link-active { background: var(--p-content-hover-background); color: var(--p-text-color); font-weight: 500; border-left-color: var(--k-blue); }
.status { display: flex; align-items: center; gap: 8px; padding: 0 10px; }
.dot { width: 7px; height: 7px; border-radius: 50%; background: var(--k-red); }
.dot.on { background: var(--k-green); }
.content { flex: 1; min-width: 0; overflow-y: auto; padding: 32px 40px; }
.content > :not(.wide) { max-width: 960px; }
</style>
