// Symbole für Projekte: Name → Symbol-ID im Sprite (App.vue). Ohne Wahl rät iconOf aus dem Namen.
export const PROJECT_ICONS = {
  folder: 'i-proj',
  code: 'i-code',
  terminal: 'i-terminal',
  briefcase: 'i-briefcase',
  book: 'hi-book',
  note: 'i-note',
  sparkles: 'hi-sparkles',
  palette: 'i-palette',
  database: 'i-database',
  bug: 'i-bug',
  cloud: 'i-cloud',
  cpu: 'i-cpu',
  shield: 'i-shield',
  zap: 'i-zap',
  rocket: 'i-rocket',
  wallet: 'hi-wallet',
  cards: 'i-cards',
  globe: 'hi-globe',
  music: 'hi-music',
  dumbbell: 'hi-dumbbell',
  goal: 'i-goal',
  bars: 'i-bars',
} as const

export type ProjectIcon = keyof typeof PROJECT_ICONS

export const PROJECT_ICON_OPTIONS: [ProjectIcon, string][] = [
  ['folder', 'Ordner'],
  ['code', 'Code & Dev'],
  ['terminal', 'Terminal & CLI'],
  ['briefcase', 'Arbeit & Business'],
  ['book', 'Uni & Lernen'],
  ['note', 'Dokumente & Notizen'],
  ['sparkles', 'Smart & Automation'],
  ['palette', 'Design & Kunst'],
  ['database', 'Datenbank & Backend'],
  ['bug', 'Debugging & Testing'],
  ['cloud', 'Cloud & Server'],
  ['cpu', 'Hardware & IoT'],
  ['shield', 'Sicherheit & Auth'],
  ['zap', 'Speed & Energie'],
  ['rocket', 'Startups & Launch'],
  ['wallet', 'Finanzen'],
  ['cards', 'Karten'],
  ['globe', 'Web & Reise'],
  ['music', 'Musik & Audio'],
  ['dumbbell', 'Sport'],
  ['goal', 'Ziel'],
  ['bars', 'Analyse'],
]

const GUESS: [RegExp, ProjectIcon][] = [
  [/terminal|cli|bash|zsh|shell|dotfile/i, 'terminal'],
  [/code|dev|track|soft|app|bot|prog|git|api|spring|react|vue|web/i, 'code'],
  [/bug|issue|qa|test/i, 'bug'],
  [/db|data|sql|postgres|mongo/i, 'database'],
  [/cloud|server|host|devops|aws|docker/i, 'cloud'],
  [/cpu|chip|hardw|esp32|arduino|raspi/i, 'cpu'],
  [/sec|auth|crypto|shield/i, 'shield'],
  [/design|kunst|\bart\b|draw|zeichn|ui|ux/i, 'palette'],
  [/rocket|launch|startup|pitch/i, 'rocket'],
  [/uni|stud|vorles|skript|abgab|lern|kurs|schul|book|buch/i, 'book'],
  [/smart|home|haus|wohn|iot|zigbee/i, 'sparkles'],
  [/bwerb|bewerb|job|cv|lebenslauf|typst|tex|doc|brief/i, 'note'],
  [/arbeit|business|kunden|mandant/i, 'briefcase'],
  [/finanz|geld|rechn|konto|steuer|budget|invest|money/i, 'wallet'],
  [/sport|fit|train|gym|work/i, 'dumbbell'],
  [/musik|audio|podcast|sound/i, 'music'],
  [/reise|travel|urlaub/i, 'globe'],
  [/ziel|goal|fokus/i, 'goal'],
]

export const projectIconKey = (p: { name: string; icon?: string }): ProjectIcon =>
  p.icon && p.icon in PROJECT_ICONS ? (p.icon as ProjectIcon) : GUESS.find(([re]) => re.test(p.name))?.[1] ?? 'folder'

export const projectIconOf = (p: { name: string; icon?: string }): string =>
  PROJECT_ICONS[projectIconKey(p)]
