<script setup lang="ts">
import { computed, nextTick, onMounted, ref, watch } from 'vue'
import '@/calendar.css'
import { useRoute, useRouter } from 'vue-router'
import FullCalendar from '@fullcalendar/vue3'
import dayGridPlugin from '@fullcalendar/daygrid'
import timeGridPlugin from '@fullcalendar/timegrid'
import interactionPlugin from '@fullcalendar/interaction'
import deLocale from '@fullcalendar/core/locales/de'
import enGbLocale from '@fullcalendar/core/locales/en-gb'
import type { CalendarOptions } from '@fullcalendar/core'
import Message from 'primevue/message'
import { errorMessage, taskAction } from '@/api/client'
import { useLiveEvents } from '@/composables/useLiveEvents'
import { useShortcuts } from '@/composables/useShortcuts'
import { useTaskSelection } from '@/composables/useTaskSelection'
import { useUndo } from '@/composables/useUndo'
import EventDialog from './components/EventDialog.vue'
import CalendarUnplannedAside from './components/CalendarUnplannedAside.vue'
import CalendarToolbar from './components/CalendarToolbar.vue'
import CalendarImportNotice from './components/CalendarImportNotice.vue'
import {
  isoWeek,
  mountEvent,
  paintDay,
  renderDayHeader,
  renderEventContent,
  scrollStart,
} from './components/calendarRenderers'
import { useCalendarEntries } from './composables/useCalendarEntries'
import { useCalendarOperations } from './composables/useCalendarOperations'
import { addDays, hhmm, ymd } from '@/lib/dates'
import { store } from '@/lib/storage'
import { prefs } from '@/lib/prefs'
import { lang, locale, t } from '@/lib/i18n'

const router = useRouter()
const route = useRoute()
const { offer, setDone } = useUndo()
const cal = ref<InstanceType<typeof FullCalendar>>()

const { stats, projects, unplannedTasks, dayStat, nextUp, nextId, loadEntries, loadProjects } =
  useCalendarEntries()

const {
  error,
  notice,
  form,
  guarded,
  moved,
  clicked,
  openForm,
  openNew,
  save,
  remove,
  postpone,
  importFile,
  handleDrop,
} = useCalendarOperations(cal, unplannedTasks, offer, router)

// Toolbar
const title = ref('')
const week = ref(0)
const SPAN = 'timeGridSpan'
const views = [
  ['dayGridMonth', t('Monat'), 'm'],
  [SPAN, t('5 Tage'), 'w'],
  ['timeGridDay', t('Tag'), 'd'],
] as const

const spanView = () => `timeGrid${prefs.calDays}`
const spanStart = () => addDays(new Date(), -Math.floor(prefs.calDays / 2))
const storedView = store.get('kairo-cal-view')
const narrow = matchMedia('(max-width:760px)').matches
const view = ref<string>(
  storedView === 'dayGridMonth'
    ? storedView
    : narrow || storedView === 'timeGridDay'
      ? 'timeGridDay'
      : spanView(),
)
const isWeek = computed(() => view.value === 'timeGrid5' || view.value === 'timeGrid7')
const isDay = computed(() => view.value === 'timeGridDay')

function goView(v: string) {
  const api = cal.value?.getApi()
  if (v === SPAN) api?.changeView(spanView(), spanStart())
  else api?.changeView(v)
}

function goToday() {
  const api = cal.value?.getApi()
  if (isWeek.value) api?.gotoDate(spanStart())
  else api?.today()
}

function setDays(n: 5 | 7) {
  prefs.calDays = n
  goView(SPAN)
}

// Ungeplant-Spalte: unter 1600 px standardmäßig zu, sonst bleiben für die Wochentage zu wenig Pixel.
const upOpen = ref((store.get('kairo-cal-unplanned') ?? (innerWidth < 1600 ? '0' : '1')) !== '0')
function toggleUp() {
  upOpen.value = !upOpen.value
  store.set('kairo-cal-unplanned', upOpen.value ? '1' : '0')
  void nextTick(() => cal.value?.getApi().updateSize())
}

const run = (fn: () => Promise<unknown>) => guarded(fn)
const selection = useTaskSelection(
  () => unplannedTasks.value,
  run,
  () => ymd(addDays(new Date(), 1)),
)
const { picked, onClick: pickRow, onPointerDown, onPointerUp } = selection

const projectOptions = computed(() =>
  projects.value.filter((p) => p.status !== 'ARCHIVED' || p.id === form.value?.projectId),
)

function gotoFromRoute() {
  const d = route.query.date
  if (typeof d !== 'string' || !/^\d{4}-\d{2}-\d{2}$/.test(d)) return
  const day = new Date(`${d}T12:00:00`)
  cal.value?.getApi().gotoDate(isWeek.value ? addDays(day, -Math.floor(prefs.calDays / 2)) : day)
}

watch(() => route.query.date, gotoFromRoute)
onMounted(() => {
  void loadProjects()
  void nextTick(gotoFromRoute)
})

useShortcuts((e) => {
  if (form.value) return
  const api = cal.value?.getApi()
  const v = views.find(([, , k]) => k === e.key)
  if (!api) return
  if (e.key === 't') goToday()
  else if (e.key === 'ArrowLeft') api.prev()
  else if (e.key === 'ArrowRight') api.next()
  else if (v) goView(v[0])
  else if (e.key === 'n') openNew()
  else return
  e.preventDefault()
})

const { dayStart: SLOT_MIN_HOUR, dayEnd: SLOT_MAX_HOUR } = prefs
let scrollFitted = false

const options: CalendarOptions = {
  plugins: [dayGridPlugin, timeGridPlugin, interactionPlugin],
  locales: [deLocale, enGbLocale],
  locale: lang.value === 'de' ? 'de' : 'en-gb',
  firstDay: prefs.weekStart,
  allDayText: t('Ganztag'),
  initialView: view.value,
  initialDate: isWeek.value ? spanStart() : undefined,
  views: {
    timeGrid5: { type: 'timeGrid', duration: { days: 5 }, dayMaxEvents: 2 },
    timeGrid7: { type: 'timeGrid', duration: { days: 7 }, dayMaxEvents: 2 },
    dayGridMonth: { eventDisplay: 'block', dayMaxEvents: 3 },
  },
  moreLinkContent: (a) => (lang.value === 'de' ? `+ ${a.num} weitere` : `+ ${a.num} more`),
  headerToolbar: false,
  height: '100%',
  nowIndicator: true,
  slotEventOverlap: false,
  slotMinTime: `${String(SLOT_MIN_HOUR).padStart(2, '0')}:00:00`,
  slotMaxTime: `${String(SLOT_MAX_HOUR).padStart(2, '0')}:00:00`,
  scrollTime: scrollStart(SLOT_MIN_HOUR),
  eventClassNames: (a) => (a.event.id === nextId.value ? ['kt-next'] : []),
  datesSet: (a) => {
    const day = a.view.type === 'timeGridDay'
    title.value = day
      ? a.view.currentStart.toLocaleDateString(locale.value, {
          weekday: 'long',
          day: 'numeric',
          month: 'long',
        })
      : a.view.title
    week.value = day ? isoWeek(a.view.currentStart) : 0
    view.value = a.view.type
    store.set('kairo-cal-view', a.view.type)
    a.view.calendar.scrollToTime(scrollStart(SLOT_MIN_HOUR))
    scrollFitted = false
  },
  dayHeaderContent: (a) => renderDayHeader(a, locale.value),
  dayHeaderDidMount: (a) => paintDay(a.el, stats),
  eventsSet: (evs) => {
    const root = cal.value?.$el as HTMLElement | undefined
    root?.querySelectorAll<HTMLElement>('.fc-col-header-cell').forEach((th) => paintDay(th, stats))
    if (scrollFitted || !evs.length) return
    scrollFitted = true
    const s = scrollStart(SLOT_MIN_HOUR, evs)
    if (s !== scrollStart(SLOT_MIN_HOUR)) cal.value?.getApi().scrollToTime(s)
  },
  slotLabelContent: (a) => String(a.date.getHours()).padStart(2, '0'),
  nowIndicatorContent: (a) => (a.isAxis ? hhmm(a.date) : ''),
  eventContent: renderEventContent,
  expandRows: true,
  snapDuration: '00:15:00',
  displayEventEnd: false,
  eventMinHeight: 26,
  eventShortHeight: 36,
  eventTimeFormat: { hour: '2-digit', minute: '2-digit', hour12: false },
  selectable: true,
  selectMirror: true,
  dayMaxEvents: 2,
  events: (info, ok, fail) => {
    loadEntries(info.start, info.end).then(ok, (e) => {
      error.value = errorMessage(e)
      fail(e)
    })
  },
  select: (info) => openForm(info.start, info.end, info.allDay),
  eventDrop: (info) => moved(info, false),
  eventResize: (info) => moved(info, true),
  eventClick: clicked,
  eventDidMount: mountEvent,
  droppable: true,
  drop: handleDrop,
}

useLiveEvents(() => {
  cal.value?.getApi().refetchEvents()
  void loadProjects()
})
</script>

<template>
  <div class="view-inner page">
    <Message v-if="error" severity="error">{{ error }}</Message>
    <CalendarImportNotice v-if="notice" :notice="notice" @close="notice = null" />

    <div class="cal-wrap">
      <div class="cal">
        <CalendarToolbar
          :title="title"
          :week="week"
          :view="view"
          :views="views"
          :span-key="SPAN"
          :is-week="isWeek"
          :is-day="isDay"
          :day-stat="dayStat"
          :cal-days="prefs.calDays"
          @prev="cal?.getApi().prev()"
          @next="cal?.getApi().next()"
          @today="goToday"
          @change-view="goView"
          @change-days="setDays"
          @import-file="importFile"
          @new-entry="openNew"
        />

        <div class="cal-fc">
          <FullCalendar ref="cal" :options="options" />
        </div>
      </div>

      <!-- Ungeplant-Seitenleiste -->
      <CalendarUnplannedAside
        :tasks="unplannedTasks"
        :projects="projects"
        :is-day="isDay"
        :is-open="upOpen"
        :next-up="nextUp"
        :picked-ids="picked"
        :selection-mode="selection.mode.value"
        @toggle-open="toggleUp"
        @pick-row="pickRow"
        @pointer-down="onPointerDown"
        @pointer-up="onPointerUp"
        @start-next-up="(task) => run(() => taskAction(task.id, 'start'))"
        @pause-next-up="(task) => run(() => taskAction(task.id, 'pause'))"
        @complete-next-up="(task) => setDone(task, 'COMPLETED', run)"
        @postpone-next-up="postpone"
        @to-tomorrow="selection.toTomorrow"
        @to-project="selection.toProject"
        @trash-selection="selection.trash"
        @clear-selection="selection.clear"
      />
    </div>

    <EventDialog
      v-if="form"
      :form="form"
      :projects="projectOptions"
      @save="save"
      @remove="remove"
      @close="form = null"
    />
  </div>
</template>

<style scoped>
.page {
  height: 100%;
  max-width: none;
  display: flex;
  flex-direction: column;
  gap: 12px;
}
.cal-wrap {
  flex: 1;
  min-height: 0;
  display: flex;
  gap: 12px;
}
.cal {
  flex: 1;
  min-width: 0;
  min-height: 0;
  display: flex;
  flex-direction: column;
  padding: 0;
  container-type: inline-size;
}
.cal-fc {
  flex: 1;
  min-height: 0;
  background: var(--bg-1);
  border: 1px solid var(--br-subtle);
  border-radius: var(--r-m);
  overflow: hidden;
}

@media (max-width: 900px) {
  .cal-wrap {
    flex-direction: column;
  }
}

@media (max-width: 760px) {
  .page {
    height: auto;
    padding-bottom: 16px;
  }
  .cal-fc {
    flex: none;
    height: 70dvh;
  }
}
</style>
