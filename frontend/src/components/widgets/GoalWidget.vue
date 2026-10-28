<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { goalToday, type Goal } from '@/api/client'
import { goalProgressText } from '@/lib/goals'
import { tn } from '@/lib/i18n'
import { filesUrl } from '@/lib/noteFiles'
import type { Size } from '@/lib/homeLayout'

defineProps<{ size: Size }>()
const goal = ref<Goal | null>(null)
const loaded = ref(false)
// Ohne Ziel bleibt das Widget leer; ein Fehler hier soll die Startseite nicht stören.
onMounted(() =>
  goalToday()
    .then(
      (g) => (goal.value = g),
      () => {},
    )
    .finally(() => (loaded.value = true)),
)

const daysLeft = computed(() => {
  const due = goal.value?.due_date
  if (!due) return ''
  const n = Math.ceil((Date.parse(`${due}T23:59:59`) - Date.now()) / 864e5)
  return n >= 0 ? tn(n, 'noch {n} Tag', 'noch {n} Tage') : ''
})
</script>

<template>
  <section class="wg wg-goal" :aria-label="$t('Ziel')">
    <div class="wg-h">
      <router-link to="/goals"
        ><svg class="ic"><use href="#i-goal" /></svg>{{ $t('Ziel') }}</router-link
      ><span v-if="daysLeft">{{ daysLeft }}</span>
    </div>
    <router-link v-if="goal" to="/goals" class="wz-body">
      <img v-if="goal.image_path" :src="filesUrl(goal.image_path)" alt="" />
      <span class="wz-main">
        <b>{{ goal.title }}</b>
        <q v-if="goal.why">{{ goal.why }}</q>
        <span v-if="goal.progress" class="wz-prog">
          <span class="wz-pct">{{ goal.progress.percent }}<small>%</small></span>
          <span class="wz-txt">{{ goalProgressText(goal) }}</span>
        </span>
        <span v-if="goal.progress" class="wz-bar" aria-hidden="true"
          ><i :style="{ width: goal.progress.percent + '%' }"></i
        ></span>
      </span>
    </router-link>
    <div v-else-if="loaded" class="wg-empty">
      {{ $t('Noch kein Ziel.') }}
      <router-link to="/goals" class="wg-link">{{ $t('Ziel anlegen ›') }}</router-link>
    </div>
  </section>
</template>

<style scoped>
.wz-body {
  flex: 1;
  min-height: 0;
  display: flex;
  gap: 16px;
  color: inherit;
  text-decoration: none;
}
.wz-body img {
  flex: none;
  width: 38%;
  max-width: 180px;
  height: 100%;
  border-radius: 10px;
  object-fit: cover;
}
.wz-main {
  flex: 1;
  min-width: 0;
  display: flex;
  flex-direction: column;
  gap: 4px;
}
.wz-main b {
  flex: none;
  font: 400 26px/1.1 var(--font-serif);
  color: var(--tx-primary);
  display: -webkit-box;
  -webkit-line-clamp: 2;
  -webkit-box-orient: vertical;
  overflow: hidden;
}
.wz-main q {
  flex: none;
  font: italic 400 16px/1.3 var(--font-serif);
  color: var(--tx-secondary);
  display: -webkit-box;
  -webkit-line-clamp: 2;
  -webkit-box-orient: vertical;
  overflow: hidden;
}
.wz-prog {
  display: flex;
  align-items: baseline;
  gap: 10px;
  margin-top: auto;
}
.wz-pct {
  font: 400 36px/1 var(--font-serif);
  color: var(--tx-primary);
}
.wz-pct small {
  font: 400 14px/1 var(--font-ui);
  color: var(--tx-muted);
  margin-left: 2px;
}
.wz-txt {
  font: 400 12px/1.4 var(--font-ui);
  color: var(--tx-muted);
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.wz-bar {
  height: 4px;
  border-radius: 2px;
  background: var(--br-subtle);
  margin-top: 6px;
}
/* Niedrige Zeilen: Titel einzeilig, das Warum fällt weg, statt dass Schrift mittendurch abgeschnitten wird */
.wg-goal {
  container-type: size;
}
@container (max-height: 220px) {
  .wz-main q {
    display: none;
  }
  .wz-main b {
    display: block;
    font-size: 22px;
    white-space: nowrap;
    text-overflow: ellipsis;
  }
}
.wz-bar i {
  display: block;
  height: 4px;
  border-radius: 2px;
  background: var(--a-blue-hi);
}
</style>
