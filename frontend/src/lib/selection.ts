// Mehrfachauswahl in Listen: Shift erweitert vom Anker bis zur Zeile, sonst wird die Zeile umgeschaltet.
export function pick(
  order: string[],
  picked: string[],
  anchor: string | null,
  id: string,
  shift: boolean,
): string[] {
  const a = anchor === null ? -1 : order.indexOf(anchor)
  const b = order.indexOf(id)
  if (shift && a >= 0 && b >= 0)
    return [...new Set([...picked, ...order.slice(Math.min(a, b), Math.max(a, b) + 1)])]
  return picked.includes(id) ? picked.filter((x) => x !== id) : [...picked, id]
}
