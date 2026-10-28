<script setup lang="ts">
import { ref } from 'vue'
import type { Project } from '@/api/client'
import SearchField from '@/components/SearchField.vue'

defineProps<{
  tabs: { id: string; label: string }[]
  extra: { id: string; label: string }[]
  filter: string
  counts: Record<string, number>
  search: string
  projects: Project[]
  projectId: string
  view: { group: string; sort: string }
  groups: readonly (readonly [string, string])[]
  sorts: readonly (readonly [string, string])[]
  selectionMode: boolean
}>()

const emit = defineEmits<{
  (e: 'update:filter', val: string): void
  (e: 'update:search', val: string): void
  (e: 'update:projectId', val: string): void
  (e: 'update:group', val: string): void
  (e: 'update:sort', val: string): void
  (e: 'openGithub'): void
  (e: 'toggleSelection'): void
}>()

const viewMenu = ref(false)
const moreMenu = ref(false)

function onOpenGithub() {
  emit('openGithub')
  moreMenu.value = false
}

function onToggleSelection() {
  emit('toggleSelection')
  moreMenu.value = false
}

function onTabKey(e: KeyboardEvent) {
  if (e.key !== 'ArrowRight' && e.key !== 'ArrowLeft') return
  const buttons = Array.from(
    (e.currentTarget as HTMLElement).querySelectorAll<HTMLButtonElement>('[role="tab"]'),
  )
  const i = buttons.indexOf(document.activeElement as HTMLButtonElement)
  if (i < 0) return
  const dir = e.key === 'ArrowRight' ? 1 : -1
  const next = buttons[(i + dir + buttons.length) % buttons.length]
  next?.focus()
  next?.click()
}
</script>

<template>
  <div class="tk-toolbar">
    <div class="filterbar tk-bar">
      <div class="seg tk-tabs" role="tablist" :aria-label="$t('Filter')" @keydown="onTabKey">
        <button
          v-for="f in tabs"
          :id="`tk-tab-${f.id}`"
          :key="f.id"
          type="button"
          role="tab"
          :aria-selected="filter === f.id"
          :tabindex="filter === f.id ? 0 : -1"
          @click="emit('update:filter', f.id)"
        >
          {{ f.label }}
          <span v-if="counts[f.id]" class="count" :class="{ od: f.id === 'ueber' }">
            {{ counts[f.id] }}
          </span>
        </button>
      </div>
      <span class="spacer"></span>
      <SearchField
        :model-value="search"
        :label="$t('Aufgaben durchsuchen')"
        @update:model-value="emit('update:search', $event)"
      />
    </div>

    <div class="tk-opts">
      <span class="spacer"></span>
      <select
        class="input"
        :value="extra.some((f) => f.id === filter) ? filter : ''"
        :aria-label="$t('Weitere Filter')"
        @change="(e) => emit('update:filter', (e.target as HTMLSelectElement).value || 'alle')"
      >
        <option value="">{{ $t('Weitere Filter') }}</option>
        <option v-for="f in extra" :key="f.id" :value="f.id">
          {{ f.label }} ({{ counts[f.id] }})
        </option>
      </select>

      <div class="tk-desktop-opts">
        <div class="menu-wrap" @keydown.esc="viewMenu = false">
          <button class="btn" type="button" :aria-expanded="viewMenu" @click="viewMenu = !viewMenu">
            {{ $t('Ansicht') }}
            <template v-if="projectId !== 'alle'">
              · {{ projects.find((p) => p.id === projectId)?.name }}
            </template>
            <svg class="ic tk-chev" aria-hidden="true"><use href="#i-chev" /></svg>
          </button>

          <template v-if="viewMenu">
            <div class="menu-scrim" @click="viewMenu = false"></div>
            <div class="menu">
              <label
                >{{ $t('Projekt') }}
                <select
                  :value="projectId"
                  class="input"
                  @change="emit('update:projectId', ($event.target as HTMLSelectElement).value)"
                >
                  <option value="alle">{{ $t('Alle') }}</option>
                  <option v-for="p in projects" :key="p.id" :value="p.id">{{ p.name }}</option>
                </select>
              </label>
              <label
                >{{ $t('Gruppe') }}
                <select
                  :value="view.group"
                  class="input"
                  @change="emit('update:group', ($event.target as HTMLSelectElement).value)"
                >
                  <option v-for="[val, lbl] in groups" :key="val" :value="val">{{ lbl }}</option>
                </select>
              </label>
              <label
                >{{ $t('Sortierung') }}
                <select
                  :value="view.sort"
                  class="input"
                  @change="emit('update:sort', ($event.target as HTMLSelectElement).value)"
                >
                  <option v-for="[val, lbl] in sorts" :key="val" :value="val">{{ lbl }}</option>
                </select>
              </label>
            </div>
          </template>
        </div>

        <button class="btn" type="button" @click="emit('openGithub')">
          {{ $t('GitHub-Issues') }}
        </button>

        <button
          class="btn tk-pick"
          type="button"
          :class="{ 'btn-primary': selectionMode }"
          :aria-pressed="selectionMode"
          @click="emit('toggleSelection')"
        >
          {{ $t('Auswählen') }}
        </button>
      </div>

      <!-- Mobil-Menü -->
      <div class="tk-mobile-more menu-wrap" @keydown.esc="moreMenu = false">
        <button
          class="btn tk-more-btn"
          type="button"
          :aria-expanded="moreMenu"
          :title="$t('Weitere Optionen')"
          :aria-label="$t('Weitere Optionen')"
          @click="moreMenu = !moreMenu"
        >
          <svg class="ic" viewBox="0 0 24 24" aria-hidden="true" width="16" height="16">
            <circle cx="12" cy="12" r="1.5" fill="currentColor" />
            <circle cx="6" cy="12" r="1.5" fill="currentColor" />
            <circle cx="18" cy="12" r="1.5" fill="currentColor" />
          </svg>
        </button>

        <template v-if="moreMenu">
          <div class="menu-scrim" @click="moreMenu = false"></div>
          <div class="menu tk-more-menu">
            <label
              >{{ $t('Projekt') }}
              <select
                :value="projectId"
                class="input"
                @change="emit('update:projectId', ($event.target as HTMLSelectElement).value)"
              >
                <option value="alle">{{ $t('Alle') }}</option>
                <option v-for="p in projects" :key="p.id" :value="p.id">{{ p.name }}</option>
              </select>
            </label>
            <label
              >{{ $t('Gruppe') }}
              <select
                :value="view.group"
                class="input"
                @change="emit('update:group', ($event.target as HTMLSelectElement).value)"
              >
                <option v-for="[val, lbl] in groups" :key="val" :value="val">{{ lbl }}</option>
              </select>
            </label>
            <label
              >{{ $t('Sortierung') }}
              <select
                :value="view.sort"
                class="input"
                @change="emit('update:sort', ($event.target as HTMLSelectElement).value)"
              >
                <option v-for="[val, lbl] in sorts" :key="val" :value="val">{{ lbl }}</option>
              </select>
            </label>
            <div class="tk-more-actions">
              <button class="btn btn-secondary" type="button" @click="onOpenGithub">
                {{ $t('GitHub-Issues') }}
              </button>
              <button
                class="btn tk-pick"
                type="button"
                :class="{ 'btn-primary': selectionMode }"
                :aria-pressed="selectionMode"
                @click="onToggleSelection"
              >
                {{ $t('Auswählen') }}
              </button>
            </div>
          </div>
        </template>
      </div>
    </div>
  </div>
</template>

<style scoped>
.tk-toolbar {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 4px 8px;
  padding-bottom: 14px;
  border-bottom: 1px solid var(--br-subtle);
}
.tk-toolbar .tk-bar {
  flex: 1 1 auto;
  margin-bottom: 0;
}
.tk-tabs {
  flex-wrap: wrap;
}
.tk-tabs .count.od {
  color: var(--a-red);
}
.tk-opts {
  display: flex;
  align-items: center;
  gap: 4px;
  flex-wrap: wrap;
  margin-left: auto;
}
.tk-opts select {
  height: 28px;
  width: auto;
  padding: 0 26px 0 10px;
  font-size: 12px;
  border-radius: 999px;
  border: 0;
  background-color: transparent;
  box-shadow: none;
  color: var(--tx-secondary);
  cursor: pointer;
}
.tk-opts select:hover {
  background-color: var(--bg-2);
  color: var(--tx-primary);
}
.tk-opts .menu select {
  width: 150px;
}
.tk-opts .btn {
  height: 28px;
  padding: 0 12px;
  font-size: 12px;
  border-radius: 999px;
  border: 0;
  background-color: transparent;
  box-shadow: none;
  color: var(--tx-secondary);
}
.tk-opts .btn:hover {
  background-color: var(--bg-2);
  color: var(--tx-primary);
}
.tk-pick {
  height: 28px;
  padding: 0 12px;
  border: 0;
  border-radius: 999px;
  background: none;
  box-shadow: none;
  color: var(--tx-secondary);
  font-size: 12px;
}
.tk-pick:hover,
.tk-pick[aria-pressed='true'] {
  background: var(--bg-2);
  color: var(--tx-primary);
}
.tk-chev {
  width: 12px;
  height: 12px;
  margin-left: 4px;
  transform: rotate(90deg);
}
.tk-desktop-opts {
  display: flex;
  align-items: center;
  gap: 4px;
}
.tk-mobile-more {
  display: none;
}
.spacer {
  flex: 1;
}

@media (max-width: 900px) {
  .tk-toolbar {
    display: block;
  }
  .tk-opts {
    margin-top: 8px;
  }
}

@media (max-width: 640px) {
  .tk-desktop-opts {
    display: none !important;
  }
  .tk-mobile-more {
    display: block;
  }
  .tk-more-btn {
    display: grid;
    place-items: center;
    width: 32px;
    height: 28px;
    padding: 0 !important;
    border-radius: 999px;
  }
  .tk-more-menu {
    width: 220px;
    right: 0;
    left: auto;
  }
  .tk-more-actions {
    display: flex;
    flex-direction: column;
    gap: 6px;
    margin-top: 8px;
    padding-top: 8px;
    border-top: 1px solid var(--br-subtle);
  }
  .tk-more-actions .btn {
    width: 100%;
    justify-content: center;
    font-size: 12px;
  }
}
</style>
