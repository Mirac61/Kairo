<script lang="ts">
const svgCache = new Map<string, string>()

function escapeHtml(s: string) {
  return s
    .replaceAll('&', '&amp;')
    .replaceAll('<', '&lt;')
    .replaceAll('>', '&gt;')
    .replaceAll('"', '&quot;')
    .replaceAll("'", '&#39;')
}
</script>

<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { hasMath as detectMath, splitMath } from '@/lib/math'

// Einzeilige Vorschau für die Kartentabelle: ohne $ nur Text, mit $…$/$$…$$ wird MathJax erst
// gerendert, wenn die Zeile in den sichtbaren Bereich kommt (bleibt bei hunderten Karten flüssig).
const props = defineProps<{ text: string }>()

const el = ref<HTMLElement>()
const visible = ref(false)
const rendered = ref<string | null>(null)

const plain = computed(() => props.text.replace(/\s+/g, ' ').trim())
const hasMath = computed(() => detectMath(plain.value))

let observer: IntersectionObserver | null = null

async function renderMath() {
  const src = plain.value
  if (!hasMath.value || !visible.value) {
    rendered.value = null
    return
  }
  const { texToSvg } = await import('@/lib/tex')
  if (src !== plain.value) return
  let out = ''
  for (const seg of splitMath(src)) {
    if (seg.kind === 'text') {
      out += escapeHtml(seg.value)
    } else {
      let svg = svgCache.get(seg.expr)
      if (!svg) {
        svg = texToSvg(seg.expr, false)
        if (svgCache.size >= 500) {
          const oldest = svgCache.keys().next().value
          if (oldest !== undefined) svgCache.delete(oldest)
        }
        svgCache.set(seg.expr, svg)
      }
      out += `<span class="ci-math">${svg}</span>`
    }
  }
  rendered.value = out
}

function observe() {
  if (visible.value || observer) return
  if (typeof IntersectionObserver === 'undefined') {
    visible.value = true
    return
  }
  observer = new IntersectionObserver(
    (entries) => {
      if (entries.some((e) => e.isIntersecting)) {
        visible.value = true
        observer?.disconnect()
        observer = null
      }
    },
    { rootMargin: '200px' },
  )
  if (el.value) observer.observe(el.value)
}

onMounted(() => {
  if (hasMath.value) observe()
})

onBeforeUnmount(() => {
  observer?.disconnect()
})

watch(hasMath, (m) => {
  if (m) observe()
})

watch([plain, visible], () => {
  void renderMath()
})
</script>

<template>
  <!-- eslint-disable-next-line vue/no-v-html -- Textteile sind escaped, Formeln kommen aus MathJax-SVG (ohne html/action-Packages) -->
  <span v-if="rendered !== null" ref="el" class="ci" v-html="rendered"></span>
  <span v-else ref="el" class="ci">{{ plain }}</span>
</template>

<style scoped>
.ci {
  display: inline;
}
.ci :deep(.ci-math) {
  display: inline;
}
.ci :deep(.ci-math svg) {
  display: inline-block;
  vertical-align: -0.22em;
  max-height: 1.45em;
  width: auto;
}
</style>
