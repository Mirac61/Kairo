import type { Component } from 'vue'
import type { WidgetId } from '@/lib/homeLayout'
import AgendaWidget from './AgendaWidget.vue'
import CardsWidget from './CardsWidget.vue'
import DayplanWidget from './DayplanWidget.vue'
import GithubWidget from './GithubWidget.vue'
import GoalWidget from './GoalWidget.vue'
import HabitsWidget from './HabitsWidget.vue'
import JournalWidget from './JournalWidget.vue'
import NoteWidget from './NoteWidget.vue'
import NowWidget from './NowWidget.vue'
import WeekWidget from './WeekWidget.vue'

// Komponente je Widget; Titel und erlaubte Größen stehen in lib/homeLayout.ts.
export const WIDGET_COMPONENTS: Record<WidgetId, Component> = {
  dayplan: DayplanWidget,
  agenda: AgendaWidget,
  now: NowWidget,
  habits: HabitsWidget,
  goal: GoalWidget,
  cards: CardsWidget,
  journal: JournalWidget,
  week: WeekWidget,
  note: NoteWidget,
  github: GithubWidget,
}
