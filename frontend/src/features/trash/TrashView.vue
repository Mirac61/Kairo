<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import {
  deleteEvent,
  deleteHabit,
  deleteTask,
  getTrash,
  listProjects,
  restoreEvent,
  restoreHabit,
  restoreTask,
  type Project,
  type Trash,
} from '@/api/client'
import { projectColor } from '@/lib/projectColor'
import { locale, t } from '@/lib/i18n'
import { useLiveEvents } from '@/composables/useLiveEvents'
import { useLoader } from '@/composables/useLoader'
import DeleteButton from '@/components/DeleteButton.vue'
import SearchField from '@/components/SearchField.vue'

const trash = ref<Trash>({ tasks: [], events: [], habits: [] })
const projects = ref<Project[]>([])

const { error, load, run } = useLoader(async () => {
  ;[trash.value, projects.value] = await Promise.all([getTrash(), listProjects()])
})

const time = (iso: string) =>
  new Intl.DateTimeFormat(locale.value, { hour: '2-digit', minute: '2-digit' }).format(
    new Date(iso),
  )
const dayKey = (d: Date) => d.toDateString()
const dayLabel = (iso: string) => {
  const d = new Date(iso),
    today = new Date(),
    yest = new Date(Date.now() - 864e5)
  if (dayKey(d) === dayKey(today)) return t('Heute')
  if (dayKey(d) === dayKey(yest)) return t('Gestern')
  return new Intl.DateTimeFormat(locale.value, {
    weekday: 'long',
    day: 'numeric',
    month: 'long',
  }).format(d)
}

// Eine Liste, neueste zuerst; der Typ steht als Icon und Wort in der Zeile.
const rows = computed(() =>
  [
    ...trash.value.tasks.map((t) => ({
      id: t.id,
      kind: 'Aufgabe',
      icon: 'tasks',
      title: t.title,
      project: t.project_id,
      at: t.deleted_at,
      restore: () => restoreTask(t.id),
      purge: () => deleteTask(t.id, true),
    })),
    ...trash.value.events.map((e) => ({
      id: e.id,
      kind: 'Termin',
      icon: 'cal',
      title: e.title,
      project: null,
      at: e.deleted_at,
      restore: () => restoreEvent(e.id),
      purge: () => deleteEvent(e.id, true),
    })),
    ...trash.value.habits.map((h) => ({
      id: h.id,
      kind: 'Gewohnheit',
      icon: 'habits',
      title: h.name,
      project: null,
      at: h.deleted_at,
      restore: () => restoreHabit(h.id),
      purge: () => deleteHabit(h.id, true),
    })),
  ].sort((x, y) => y.at.localeCompare(x.at)),
)
const projectName = computed(() => new Map(projects.value.map((p) => [p.id, p.name])))
const KINDS = ['Aufgabe', 'Termin', 'Gewohnheit'] as const
const PLURAL = { Aufgabe: 'Aufgaben', Termin: 'Termine', Gewohnheit: 'Gewohnheiten' } as const
const kind = ref<'' | (typeof KINDS)[number]>('')
const count = (k: string) => rows.value.filter((r) => !k || r.kind === k).length
const search = ref('')
const shown = computed(() => {
  const q = search.value.trim().toLowerCase()
  return rows.value.filter(
    (r) =>
      (!kind.value || r.kind === kind.value) &&
      (!q || `${r.title} ${t(r.kind)}`.toLowerCase().includes(q)),
  )
})
// Nach Löschtag gruppiert; rows ist schon neueste zuerst sortiert.
const groups = computed(() => {
  const out: { label: string; rows: typeof shown.value }[] = []
  for (const r of shown.value) {
    const label = dayLabel(r.at)
    if (out.at(-1)?.label === label) out.at(-1)!.rows.push(r)
    else out.push({ label, rows: [r] })
  }
  return out
})

onMounted(load)
useLiveEvents(load)
</script>

<template>
  <div class="view-inner">
    <div class="v-head">
      <h1 class="v-title">{{ $t('Papierkorb') }}</h1>
      <div class="v-sub">
        {{ $tn(rows.length, '{n} Eintrag', '{n} Einträge') }} ·
        {{ $t('Einträge werden hier aufbewahrt und können wiederhergestellt werden.') }}
      </div>
    </div>
    <div v-if="error" class="badge" role="alert">{{ error }}</div>
    <div v-if="!rows.length" class="card tr-empty">
      <svg class="ic" aria-hidden="true"><use href="#i-trash" /></svg>
      <p>{{ $t('Der Papierkorb ist leer.') }}</p>
    </div>
    <template v-else>
      <div class="filterbar">
        <div class="seg" role="group" :aria-label="$t('Filter')">
          <button type="button" :aria-pressed="kind === ''" @click="kind = ''">
            {{ $t('Alle') }} <span class="count">{{ count('') }}</span>
          </button>
          <button
            v-for="k in KINDS"
            :key="k"
            type="button"
            :aria-pressed="kind === k"
            :disabled="!count(k)"
            @click="kind = k"
          >
            {{ $t(PLURAL[k]) }} <span class="count">{{ count(k) }}</span>
          </button>
        </div>
        <SearchField v-model="search" :label="$t('Papierkorb durchsuchen')" />
      </div>
      <div v-if="!shown.length" class="v-sub">
        {{ $t('Keine Treffer für „{q}“.', { q: search }) }}
      </div>
      <template v-else>
        <section v-for="g in groups" :key="g.label" class="tr-group">
          <h2 class="tr-day">
            {{ g.label }} <span class="count">{{ g.rows.length }}</span>
          </h2>
          <div class="card tasklist">
            <div v-for="r in g.rows" :key="r.kind + r.id" class="task-row">
              <span class="tr-ic" :data-kind="r.icon"
                ><svg class="ic" aria-hidden="true"><use :href="`#i-${r.icon}`" /></svg
              ></span>
              <span class="t tr-title"
                >{{ r.title }}
                <span class="tr-from"
                  >{{ $t(r.kind)
                  }}<template v-if="r.project && projectName.get(r.project)">
                    · <i class="pdot" :style="{ background: projectColor(r.project) }"></i>aus
                    {{ projectName.get(r.project) }}</template
                  ></span
                >
              </span>
              <span class="due">{{ time(r.at) }}</span>
              <span class="row-act"
                ><button
                  type="button"
                  class="btn btn-secondary"
                  :aria-label="`Wiederherstellen: ${r.title}`"
                  @click="run(r.restore)"
                >
                  {{ $t('Wiederherstellen') }}
                </button>
                <DeleteButton
                  ghost
                  :label="$t('Endgültig löschen')"
                  :text="
                    $t('„{title}“ endgültig löschen? Das lässt sich nicht rückgängig machen.', {
                      title: r.title,
                    })
                  "
                  @confirm="run(r.purge)"
              /></span>
            </div>
          </div>
        </section>
      </template>
    </template>
  </div>
</template>

<style scoped>
.view-inner {
  max-width: 960px;
  margin: 0 auto;
}
.card.tasklist {
  border-radius: var(--r-m);
  background: var(--bg-1);
  border: 1px solid var(--br-default);
}
.tr-group + .tr-group {
  margin-top: 20px;
}
.tr-day {
  display: flex;
  align-items: baseline;
  gap: 8px;
  margin: 0 0 8px 2px;
  font: 600 13px/1.2 var(--font-ui);
  color: var(--tx-secondary);
}
.tr-day .count {
  font-weight: 400;
  color: var(--tx-muted);
}
.tr-ic {
  --c: var(--a-blue);
  flex: none;
  display: grid;
  place-items: center;
  width: 28px;
  height: 28px;
  border-radius: 8px;
  color: var(--c);
  background: color-mix(in oklab, var(--c) 14%, transparent);
}
.tr-ic[data-kind='cal'] {
  --c: var(--a-event);
}
.tr-ic[data-kind='habits'] {
  --c: var(--a-run);
}
.tr-empty {
  display: grid;
  justify-items: center;
  gap: 10px;
  padding: 48px 16px;
  border: 1px dashed var(--br-default);
  border-radius: var(--r-m);
  background: none;
  color: var(--tx-muted);
}
.tr-empty .ic {
  width: 32px;
  height: 32px;
  opacity: 0.6;
}
.tr-empty p {
  margin: 0;
  font: 400 14px/1.4 var(--font-ui);
}
.tr-title {
  display: grid;
  gap: 2px;
  white-space: normal;
}
.tr-from {
  display: flex;
  align-items: center;
  gap: 6px;
  font: 400 12px/1.3 var(--font-ui);
  color: var(--tx-muted);
}
.tr-from .pdot {
  display: inline-block;
  width: 6px;
  height: 6px;
  border-radius: 50%;
}
</style>
