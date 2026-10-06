<script setup lang="ts">
import { ref } from 'vue'
import { RouterLink, RouterView } from 'vue-router'
import ConfirmPopup from 'primevue/confirmpopup'
import { navItems } from '@/router'
import { useBackendStatus } from '@/composables/useBackendStatus'

const { online, version } = useBackendStatus()

// Theme: gespeicherte Wahl, sonst Systemeinstellung (siehe main.ts).
const dark = ref(document.documentElement.dataset.theme !== 'light')
function toggleTheme() {
  dark.value = !dark.value
  const t = dark.value ? 'dark' : 'light'
  document.documentElement.dataset.theme = t
  try { localStorage.setItem('kairo-theme', t) } catch { /* privater Modus */ }
}
</script>

<template>
  <svg width="0" height="0" style="position:absolute" aria-hidden="true">
  <defs>
    <symbol id="i-cal" viewBox="0 0 24 24"><rect x="3.5" y="4.5" width="17" height="16" rx="2"/><path d="M3.5 9.5h17M8 2.5v4M16 2.5v4"/></symbol>
    <symbol id="i-today" viewBox="0 0 24 24"><circle cx="12" cy="12" r="8.5"/><circle cx="12" cy="12" r="2.4" fill="currentColor" stroke="none"/></symbol>
    <symbol id="i-tasks" viewBox="0 0 24 24"><rect x="4" y="4" width="16" height="16" rx="3"/><path d="m8.5 12.3 2.4 2.4 4.8-5.4"/></symbol>
    <symbol id="i-habits" viewBox="0 0 24 24"><path d="M21 3v5h-5M3 21v-5h5"/><path d="M4.6 9.5a8 8 0 0 1 13.4-3.3L21 8M3 16l3 1.8a8 8 0 0 0 13.4-3.3"/></symbol>
    <symbol id="i-proj" viewBox="0 0 24 24"><path d="M3.5 7a2 2 0 0 1 2-2H9l2 2.5h7.5a2 2 0 0 1 2 2V17a2 2 0 0 1-2 2h-13a2 2 0 0 1-2-2z"/></symbol>
    <symbol id="i-grid" viewBox="0 0 24 24"><rect x="4" y="4" width="7" height="7" rx="1.5"/><rect x="13" y="4" width="7" height="7" rx="1.5"/><rect x="4" y="13" width="7" height="7" rx="1.5"/><rect x="13" y="13" width="7" height="7" rx="1.5"/></symbol>
    <symbol id="i-plus" viewBox="0 0 24 24"><path d="M12 5v14M5 12h14"/></symbol>
    <symbol id="i-left" viewBox="0 0 24 24"><path d="M14.5 5.5 8 12l6.5 6.5"/></symbol>
    <symbol id="i-right" viewBox="0 0 24 24"><path d="M9.5 5.5 16 12l-6.5 6.5"/></symbol>
    <symbol id="i-sun" viewBox="0 0 24 24"><circle cx="12" cy="12" r="4"/><path d="M12 2.5v2M12 19.5v2M2.5 12h2M19.5 12h2M5 5l1.4 1.4M17.6 17.6 19 19M19 5l-1.4 1.4M6.4 17.6 5 19"/></symbol>
    <symbol id="i-moon" viewBox="0 0 24 24"><path d="M20 14.5A8 8 0 1 1 9.5 4a6.5 6.5 0 0 0 10.5 10.5z"/></symbol>
    <symbol id="i-x" viewBox="0 0 24 24"><path d="M6 6l12 12M18 6 6 18"/></symbol>
    <symbol id="i-link" viewBox="0 0 24 24"><path d="M10 13a5 5 0 0 0 7.5.5l2-2a5 5 0 0 0-7-7l-1.2 1.2"/><path d="M14 11a5 5 0 0 0-7.5-.5l-2 2a5 5 0 0 0 7 7l1.2-1.2"/></symbol>
    <symbol id="i-file" viewBox="0 0 24 24"><path d="M13.5 2.5H7A1.5 1.5 0 0 0 5.5 4v16A1.5 1.5 0 0 0 7 21.5h10a1.5 1.5 0 0 0 1.5-1.5V7.5z"/><path d="M13.5 2.5V7.5h5"/></symbol>
    <symbol id="i-trash" viewBox="0 0 24 24"><path d="M4.5 6.5h15M9.5 6.5V4.5h5v2M6.5 6.5l.8 13a1.5 1.5 0 0 0 1.5 1.4h6.4a1.5 1.5 0 0 0 1.5-1.4l.8-13"/></symbol>
    <symbol id="i-inbox" viewBox="0 0 24 24"><path d="M3.5 13.5 6 5a1.5 1.5 0 0 1 1.4-1h9.2A1.5 1.5 0 0 1 18 5l2.5 8.5V18a1.5 1.5 0 0 1-1.5 1.5H5A1.5 1.5 0 0 1 3.5 18z"/><path d="M3.5 13.5H9a3 3 0 0 0 6 0h5.5"/></symbol>
  </defs>
</svg>
  <div class="app">
    <aside class="sidebar">
      <div class="logo">Kairo</div>
      <div class="lbl nav-label">Ansichten</div>
      <nav aria-label="Hauptnavigation">
        <RouterLink v-for="item in navItems" :key="item.path" :to="item.path" class="nav-item">
          <svg class="ic"><use :href="`#i-${item.icon}`" /></svg><span>{{ item.label }}</span>
        </RouterLink>
      </nav>
      <div class="sidebar-footer">
        <div class="status">
          <span class="status-dot" :style="{ background: online ? 'var(--a-green)' : 'var(--a-red)' }" />
          <span>{{ online ? `Verbunden${version ? ` (${version})` : ''}` : 'Backend offline' }}</span>
        </div>
        <button class="icon-btn" aria-label="Theme wechseln" data-tip="Theme wechseln" @click="toggleTheme">
          <svg class="ic"><use :href="dark ? '#i-sun' : '#i-moon'" /></svg>
        </button>
      </div>
    </aside>
    <main class="content"><RouterView /></main>
    <ConfirmPopup />
  </div>
</template>
