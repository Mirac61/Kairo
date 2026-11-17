import { t } from './i18n.ts'

// Kürzel stehen im Code in Mac-Schreibweise (⌘ ⇧ ⌥). Windows und Linux zeigen Strg+/Shift+/Alt+;
// die Handler nehmen ohnehin metaKey oder ctrlKey, nur die Anzeige hängt an der Plattform.
export const isApple =
  typeof navigator !== 'undefined' && /Mac|iPhone|iPad/.test(navigator.userAgent)

// „⇧⌘Z“ → „Strg+Shift+Z“, „Rückgängig (⌘Z)“ → „Rückgängig (Strg+Z)“; Text drumherum bleibt.
export function keys(s: string, apple = isApple): string {
  if (apple) return s
  return s
    .replace('⌫', 'Backspace')
    .replace(
      /[⇧⌥⌘]+ ?/g,
      (m) =>
        [m.includes('⌘') && t('Strg'), m.includes('⌥') && 'Alt', m.includes('⇧') && 'Shift']
          .filter(Boolean)
          .join('+') + '+',
    )
}

// Für aria-keyshortcuts: „⌘D“ → „Meta+D“ (Apple) bzw. „Control+D“.
export const ariaKeys = (s: string, apple = isApple) =>
  s
    .replace('⌘', apple ? 'Meta+' : 'Control+')
    .replace('⇧', 'Shift+')
    .replace('⌥', 'Alt+')
    .replace('⌫', 'Backspace')
