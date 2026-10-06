<script setup lang="ts">
import { RouterLink, RouterView } from 'vue-router'
import ConfirmPopup from 'primevue/confirmpopup'
import Tag from 'primevue/tag'
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
      <Tag :severity="online ? 'success' : 'danger'" rounded class="status">
        {{ online ? `Backend online${version ? ` (${version})` : ''}` : 'Backend offline' }}
      </Tag>
    </aside>
    <main class="content"><RouterView /></main>
    <ConfirmPopup />
  </div>
</template>

<style scoped>
.shell { display: flex; height: 100%; }
.sidebar {
  width: 220px; flex-shrink: 0; display: flex; flex-direction: column; gap: 4px; padding: 16px 12px;
  background: var(--p-content-background); border-right: 1px solid var(--p-content-border-color);
}
.brand { font-size: 20px; font-weight: 700; padding: 4px 10px 16px; color: var(--p-primary-color); }
nav { display: flex; flex-direction: column; gap: 2px; flex: 1; }
nav a { padding: 8px 10px; border-radius: 8px; color: var(--p-text-color); text-decoration: none; }
nav a:hover { background: var(--p-content-hover-background); }
nav a.router-link-active { background: var(--p-highlight-background); color: var(--p-highlight-color); font-weight: 600; }
.status { align-self: flex-start; }
.content { flex: 1; min-width: 0; overflow-y: auto; padding: 32px 40px; }
.content > * { max-width: 960px; }
</style>
