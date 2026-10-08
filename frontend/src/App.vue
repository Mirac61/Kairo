<script setup lang="ts">
import { nextTick, onBeforeUnmount, onMounted, ref } from 'vue'
import { RouterLink, RouterView, useRouter } from 'vue-router'
import ConfirmPopup from 'primevue/confirmpopup'
import { navItems } from '@/router'
import UndoToast from '@/components/UndoToast.vue'
import { useBackendStatus } from '@/composables/useBackendStatus'
import { useLiveEvents } from '@/composables/useLiveEvents'
import { counts, loadSidebar, pins } from '@/composables/useSidebar'
import { hm } from '@/lib/dates'
import { store } from '@/lib/storage'
import { projectColor } from '@/lib/projectColor'

const { online, version } = useBackendStatus()
const router = useRouter()

useLiveEvents(loadSidebar)
const countOf: Record<string, () => string | number> = {
  tasks: () => counts.value.tasks, habits: () => counts.value.habits, proj: () => counts.value.projects, trash: () => counts.value.trash,
}

// „Neu …“ und ⌘K: Schnelleingabe der Aufgabenliste (die Ansicht fokussiert das Feld).
const newEntry = () => router.push({ path: '/tasks', query: { new: String(Date.now()) } })
function onKey(e: KeyboardEvent) {
  if (!(e.metaKey || e.ctrlKey) || e.key !== 'k') return
  e.preventDefault()
  void newEntry()
}
onMounted(() => { window.addEventListener('keydown', onKey); void loadSidebar() })
onBeforeUnmount(() => window.removeEventListener('keydown', onKey))

// Seitenleiste einklappen: nur Symbole, damit die Ansicht mehr Platz bekommt. Ohne gespeicherte Wahl startet sie im schmalen Fenster eingeklappt.
const mini = ref((store.get('kairo-sidebar') ?? (matchMedia('(max-width:960px)').matches ? 'mini' : 'full')) === 'mini')
function toggleSidebar() {
  mini.value = !mini.value
  store.set('kairo-sidebar', mini.value ? 'mini' : 'full')
  // Der Kalender misst sich nur bei Fenster-Resize neu.
  void nextTick(() => window.dispatchEvent(new Event('resize')))
}

// Theme: gespeicherte Wahl, sonst Systemeinstellung (siehe main.ts).
const dark = ref(document.documentElement.dataset.theme !== 'light')
function toggleTheme() {
  dark.value = !dark.value
  const t = dark.value ? 'dark' : 'light'
  document.documentElement.dataset.theme = t
  store.set('kairo-theme', t)
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
    <symbol id="i-bars" viewBox="0 0 24 24"><path d="M4.5 20.5V13M9.75 20.5V5M15 20.5v-7M20.25 20.5V9"/></symbol>
    <symbol id="i-grid" viewBox="0 0 24 24"><rect x="4" y="4" width="7" height="7" rx="1.5"/><rect x="13" y="4" width="7" height="7" rx="1.5"/><rect x="4" y="13" width="7" height="7" rx="1.5"/><rect x="13" y="13" width="7" height="7" rx="1.5"/></symbol>
    <symbol id="i-list" viewBox="0 0 24 24"><path d="M5 7h14M5 12h14M5 17h14"/></symbol>
    <symbol id="i-plus" viewBox="0 0 24 24"><path d="M12 5v14M5 12h14"/></symbol>
    <symbol id="i-left" viewBox="0 0 24 24"><path d="M14.5 5.5 8 12l6.5 6.5"/></symbol>
    <symbol id="i-right" viewBox="0 0 24 24"><path d="M9.5 5.5 16 12l-6.5 6.5"/></symbol>
    <symbol id="i-sun" viewBox="0 0 24 24"><circle cx="12" cy="12" r="4"/><path d="M12 2.5v2M12 19.5v2M2.5 12h2M19.5 12h2M5 5l1.4 1.4M17.6 17.6 19 19M19 5l-1.4 1.4M6.4 17.6 5 19"/></symbol>
    <symbol id="i-moon" viewBox="0 0 24 24"><path d="M20 14.5A8 8 0 1 1 9.5 4a6.5 6.5 0 0 0 10.5 10.5z"/></symbol>
    <symbol id="i-x" viewBox="0 0 24 24"><path d="M6 6l12 12M18 6 6 18"/></symbol>
    <symbol id="i-link" viewBox="0 0 24 24"><path d="M10 13a5 5 0 0 0 7.5.5l2-2a5 5 0 0 0-7-7l-1.2 1.2"/><path d="M14 11a5 5 0 0 0-7.5-.5l-2 2a5 5 0 0 0 7 7l1.2-1.2"/></symbol>
    <symbol id="i-file" viewBox="0 0 24 24"><path d="M13.5 2.5H7A1.5 1.5 0 0 0 5.5 4v16A1.5 1.5 0 0 0 7 21.5h10a1.5 1.5 0 0 0 1.5-1.5V7.5z"/><path d="M13.5 2.5V7.5h5"/></symbol>
    <symbol id="i-note" viewBox="0 0 24 24"><path d="M6.5 3.5h11a1.5 1.5 0 0 1 1.5 1.5v14a1.5 1.5 0 0 1-1.5 1.5h-11A1.5 1.5 0 0 1 5 19V5a1.5 1.5 0 0 1 1.5-1.5z"/><path d="M8.5 8h7M8.5 12h7M8.5 16h4"/></symbol>
    <symbol id="i-chev" viewBox="0 0 24 24"><path d="M9.5 6 15.5 12l-6 6"/></symbol>
    <symbol id="i-pen" viewBox="0 0 24 24"><path d="M15.5 4.5 19.5 8.5 8.5 19.5H4.5V15.5z"/><path d="M13 7l4 4"/></symbol>
    <symbol id="i-trash" viewBox="0 0 24 24"><path d="M4.5 6.5h15M9.5 6.5V4.5h5v2M6.5 6.5l.8 13a1.5 1.5 0 0 0 1.5 1.4h6.4a1.5 1.5 0 0 0 1.5-1.4l.8-13"/></symbol>
    <symbol id="i-alert" viewBox="0 0 24 24"><path d="M10.3 3.9 2.4 17.5a2 2 0 0 0 1.7 3h15.8a2 2 0 0 0 1.7-3L13.7 3.9a2 2 0 0 0-3.4 0z"/><path d="M12 9.5v4M12 17h.01"/></symbol>
    <symbol id="i-inbox" viewBox="0 0 24 24"><path d="M3.5 13.5 6 5a1.5 1.5 0 0 1 1.4-1h9.2A1.5 1.5 0 0 1 18 5l2.5 8.5V18a1.5 1.5 0 0 1-1.5 1.5H5A1.5 1.5 0 0 1 3.5 18z"/><path d="M3.5 13.5H9a3 3 0 0 0 6 0h5.5"/></symbol>
  </defs>
</svg>
  <div class="app">
    <aside class="sidebar" :class="{ mini }">
      <div class="logo" role="img" aria-label="Kairo"><span class="logo-word" aria-hidden="true">K<i class="lw-a"></i>IR<i class="lw-o"></i></span><span class="logo-mark" aria-hidden="true"><i></i></span></div>
      <button type="button" class="sb-new" aria-keyshortcuts="Meta+K Control+K" @click="newEntry">
        <svg class="ic"><use href="#i-plus" /></svg><span class="sb-new-l">Neu …</span><kbd class="key" aria-hidden="true">⌘K</kbd>
      </button>
      <nav aria-label="Hauptnavigation">
        <RouterLink v-for="item in navItems" :key="item.path" :to="item.path" class="nav-item" :aria-label="mini ? item.label : undefined" :data-tip="mini ? item.label : undefined">
          <svg class="ic"><use :href="`#i-${item.icon}`" /></svg><span class="nav-l">{{ item.label }}</span>
          <span v-if="countOf[item.icon]?.()" class="nav-n">{{ countOf[item.icon]!() }}</span>
        </RouterLink>
      </nav>
      <div v-if="pins.length" class="pins">
        <div class="pins-head"><span>Aktiv</span><span>diese Woche</span></div>
        <RouterLink v-for="p in pins" :key="p.id" to="/projects" class="nav-item pin">
          <span class="pin-dot"><i :style="{ background: projectColor(p.id) }"></i></span><span class="nav-l">{{ p.name }}</span>
          <span v-if="p.minutes" class="nav-n">{{ hm(p.minutes) }}</span>
        </RouterLink>
      </div>
      <div class="sidebar-footer">
        <div class="status" :class="{ off: !online }">
          <span class="status-dot" />
          <span>{{ online ? `Verbunden${version ? ` (${version})` : ''}` : 'Getrennt' }}</span>
        </div>
        <div class="sb-btns">
          <button class="icon-btn" aria-label="Theme wechseln" data-tip="Theme wechseln" @click="toggleTheme">
            <svg class="ic"><use :href="dark ? '#i-sun' : '#i-moon'" /></svg>
          </button>
          <button
            class="icon-btn sb-toggle" :aria-expanded="!mini" :aria-label="mini ? 'Seitenleiste ausklappen' : 'Seitenleiste einklappen'"
            :data-tip="mini ? 'Ausklappen' : 'Einklappen'" @click="toggleSidebar"
          ><svg class="ic"><use :href="mini ? '#i-right' : '#i-left'" /></svg></button>
        </div>
      </div>
    </aside>
    <main class="content"><RouterView /></main>
    <ConfirmPopup />
    <UndoToast />
  </div>
</template>
