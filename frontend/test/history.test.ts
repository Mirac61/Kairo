// Rückgängig/Wiederholen des Zeichen-Editors: node test/history.test.ts
import assert from 'node:assert/strict'
import { useHistory } from '../src/composables/useHistory.ts'

let state = [1]
const h = useHistory(() => state, (v) => (state = v))
h.reset()
assert.equal(h.changed.value, false)

state = [1, 2]
h.commit()
state = [1, 2, 3]
h.commit()
assert.deepEqual([h.at.value, h.len.value, h.changed.value], [2, 3, true])

assert.ok(h.go(1))
assert.deepEqual(state, [1, 2])
assert.ok(!h.go(5))

// Zurück auf den gespeicherten Stand: nichts geändert.
h.go(0)
assert.equal(h.changed.value, false)

// Neuer Zweig nach Rückgängig verwirft die Zukunft.
state = [9]
h.commit()
assert.deepEqual([h.at.value, h.len.value], [1, 2])
h.go(0)
assert.equal(h.changed.value, false)
