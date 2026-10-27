// Symbole für Gewohnheiten: Name → Symbol-ID im Sprite (App.vue). Ohne Wahl rät iconOf aus dem Namen.
export const HABIT_ICONS = {
  book: 'hi-book', dumbbell: 'hi-dumbbell', run: 'hi-run', bike: 'hi-bike', droplet: 'hi-droplet', leaf: 'hi-leaf',
  bed: 'hi-bed', sun: 'i-sun', moon: 'i-moon', food: 'hi-food', coffee: 'hi-coffee', pill: 'hi-pill', heart: 'hi-heart',
  pen: 'i-pen', journal: 'i-journal', code: 'i-code', cards: 'i-cards', globe: 'hi-globe', music: 'hi-music',
  phone: 'hi-phone', sparkles: 'hi-sparkles', wallet: 'hi-wallet', target: 'i-goal', repeat: 'i-habits',
} as const
export type HabitIcon = keyof typeof HABIT_ICONS

const GUESS: [RegExp, HabitIcon][] = [
  [/les|buch|book|read/i, 'book'], [/lern|stud|vokab|anki|karte/i, 'cards'], [/sport|gym|kraft|workout|train/i, 'dumbbell'],
  [/lauf|jogg|run|spazier|schritt|walk/i, 'run'], [/rad|bike/i, 'bike'], [/wasser|trink|water/i, 'droplet'],
  [/medit|atem|yoga|achtsam/i, 'leaf'], [/schlaf|sleep|bett/i, 'bed'], [/aufsteh|morgen/i, 'sun'], [/abend/i, 'moon'],
  [/ess|koch|obst|gemüse|food|cook/i, 'food'], [/kaffee|tee\b|coffee/i, 'coffee'], [/vitamin|tablett|medik|pill/i, 'pill'],
  [/journal|tagebuch|rückblick|review/i, 'journal'], [/schreib|write/i, 'pen'], [/code|program|leetcode/i, 'code'],
  [/sprach|englisch|spanisch|türk|language/i, 'globe'], [/musik|gitarre|klavier|piano|guitar/i, 'music'],
  [/handy|bildschirm|screen|phone/i, 'phone'], [/putz|aufräum|clean/i, 'sparkles'], [/geld|spar|budget|money/i, 'wallet'],
  [/dank|grat|liebe|family|familie/i, 'heart'],
]

export const iconKey = (h: { name: string; icon?: string }): HabitIcon =>
  h.icon && h.icon in HABIT_ICONS ? (h.icon as HabitIcon) : GUESS.find(([re]) => re.test(h.name))?.[1] ?? 'repeat'
export const iconOf = (h: { name: string; icon?: string }) => HABIT_ICONS[iconKey(h)]
