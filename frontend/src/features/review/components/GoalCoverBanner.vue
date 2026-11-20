<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { filesUrl } from '@/lib/noteFiles'
import { prefs } from '@/lib/prefs'
import { isVideoPath } from '../goalUtils'

const props = defineProps<{
  imagePath: string | null
  moodIdx: number
}>()

const emit = defineEmits<{
  upload: [file: File]
  clear: []
  nextMood: []
  hide: []
}>()

const fileInput = ref<HTMLInputElement | null>(null)
const dragDepth = ref(0)
const mediaError = ref(false)
const reducedMotion = computed(
  () =>
    prefs.motion ||
    (typeof matchMedia === 'function' && matchMedia('(prefers-reduced-motion: reduce)').matches),
)

watch(
  () => props.imagePath,
  () => {
    mediaError.value = false
  },
)

const hasMedia = computed(() => !!props.imagePath && !mediaError.value)

function triggerUpload() {
  fileInput.value?.click()
}

function applyFile(file: File | undefined) {
  if (!file) return
  if (!file.type.startsWith('image/') && !file.type.startsWith('video/')) return
  emit('upload', file)
}

function onFileChange(e: Event) {
  const inp = e.target as HTMLInputElement
  applyFile(inp.files?.[0])
  inp.value = ''
}

function onDragEnter(e: DragEvent) {
  e.preventDefault()
  dragDepth.value++
}

function onDragLeave() {
  dragDepth.value = Math.max(0, dragDepth.value - 1)
}

function onDrop(e: DragEvent) {
  e.preventDefault()
  dragDepth.value = 0
  applyFile(e.dataTransfer?.files?.[0])
}
</script>

<template>
  <div
    class="gdm-cover"
    :class="{ 'is-drag': dragDepth > 0 }"
    @dragenter="onDragEnter"
    @dragover.prevent
    @dragleave="onDragLeave"
    @drop="onDrop"
  >
    <input
      ref="fileInput"
      type="file"
      accept="image/*,video/mp4,video/webm,video/quicktime"
      class="sr-only"
      @change="onFileChange"
    />

    <video
      v-if="hasMedia && isVideoPath(imagePath)"
      :src="filesUrl(imagePath!)"
      :autoplay="!reducedMotion"
      loop
      muted
      playsinline
      class="gdm-media"
      @error="mediaError = true"
    ></video>
    <img
      v-else-if="hasMedia"
      :src="filesUrl(imagePath!)"
      alt=""
      class="gdm-media"
      :class="{ 'gdm-ken-burns': !reducedMotion }"
      @error="mediaError = true"
    />
    <svg
      v-else-if="moodIdx % 3 === 0"
      viewBox="0 0 1000 195"
      preserveAspectRatio="xMidYMid slice"
      class="gdm-media gdm-svg-mood"
      aria-hidden="true"
    >
      <rect width="1000" height="195" class="mood-bg" />
      <path
        class="gdm-drift-1 mood-l1"
        d="M-40 142 L150 78 L310 116 L475 50 L665 108 L840 68 L1040 124 L1040 200 L-40 200 Z"
      />
      <path
        class="gdm-drift-2 mood-l2"
        d="M-40 162 L220 110 L420 144 L640 88 L870 140 L1040 118 L1040 200 L-40 200 Z"
      />
    </svg>
    <svg
      v-else-if="moodIdx % 3 === 1"
      viewBox="0 0 1000 195"
      preserveAspectRatio="xMidYMid slice"
      class="gdm-media gdm-svg-mood"
      aria-hidden="true"
    >
      <rect width="1000" height="195" class="mood-bg" />
      <g class="gdm-orbit mood-stroke" stroke-width="1" fill="none">
        <circle cx="760" cy="95" r="46" />
        <circle cx="760" cy="95" r="94" />
        <circle cx="760" cy="95" r="144" />
        <circle cx="760" cy="95" r="196" />
      </g>
    </svg>
    <svg
      v-else
      viewBox="0 0 1000 195"
      preserveAspectRatio="xMidYMid slice"
      class="gdm-media gdm-svg-mood"
      aria-hidden="true"
    >
      <rect width="1000" height="195" class="mood-bg" />
      <path
        class="gdm-drift-1 mood-l1"
        d="M-40 128 Q 240 96 500 124 T 1040 112 L 1040 200 L -40 200 Z"
      />
      <path
        class="gdm-drift-2 mood-l2"
        d="M-40 150 Q 280 126 550 150 T 1040 138 L 1040 200 L -40 200 Z"
      />
    </svg>

    <div class="gdm-cover-fade" aria-hidden="true"></div>

    <div class="gdm-cover-actions">
      <button type="button" class="gdm-pill" @click="triggerUpload">
        {{ $t('Bild / Video hochladen') }}
      </button>
      <button v-if="imagePath" type="button" class="gdm-pill muted" @click="emit('clear')">
        {{ $t('Entfernen') }}
      </button>
      <button v-else type="button" class="gdm-pill muted" @click="emit('nextMood')">
        {{ $t('Stimmung wechseln') }}
      </button>
      <button type="button" class="gdm-pill muted" @click="emit('hide')">
        {{ $t('Ausblenden') }}
      </button>
    </div>

    <div v-if="dragDepth > 0" class="gdm-drop-hint">
      {{ $t('Bild oder Video hier ablegen') }}
    </div>
  </div>
</template>

<style scoped>
.gdm-cover {
  position: relative;
  height: 190px;
  flex-shrink: 0;
  overflow: hidden;
  background: var(--bg-2);
  border-bottom: 1px solid var(--br-subtle);
}
.gdm-cover.is-drag {
  outline: 2px dashed var(--a-blue-hi);
  outline-offset: -4px;
}
.gdm-media {
  position: absolute;
  inset: 0;
  width: 100%;
  height: 100%;
  object-fit: cover;
  display: block;
}
.mood-bg {
  fill: var(--bg-2);
}
.mood-l1 {
  fill: color-mix(in oklab, var(--tx-primary) 7%, var(--bg-1));
}
.mood-l2 {
  fill: color-mix(in oklab, var(--tx-primary) 3%, var(--bg-1));
}
.mood-stroke {
  stroke: color-mix(in oklab, var(--tx-primary) 12%, transparent);
}
.gdm-ken-burns {
  animation: gdm-kb 28s ease-in-out infinite alternate;
}
.gdm-drift-1 {
  animation: gdm-drift 20s ease-in-out infinite alternate;
}
.gdm-drift-2 {
  animation: gdm-drift 26s ease-in-out infinite alternate-reverse;
}
.gdm-orbit {
  transform-origin: 760px 95px;
  animation: gdm-rot 60s linear infinite;
}
@keyframes gdm-kb {
  from {
    transform: scale(1);
  }
  to {
    transform: scale(1.06);
  }
}
@keyframes gdm-drift {
  from {
    transform: translateX(-10px);
  }
  to {
    transform: translateX(10px);
  }
}
@keyframes gdm-rot {
  from {
    transform: rotate(0deg);
  }
  to {
    transform: rotate(360deg);
  }
}
@media (prefers-reduced-motion: reduce) {
  .gdm-ken-burns,
  .gdm-drift-1,
  .gdm-drift-2,
  .gdm-orbit {
    animation: none;
  }
}
.gdm-cover-fade {
  position: absolute;
  inset: 0;
  pointer-events: none;
  background: linear-gradient(180deg, transparent 48%, var(--bg-1) 100%);
}
.gdm-cover-actions {
  position: absolute;
  left: 16px;
  bottom: 12px;
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
  opacity: 0;
  transition: opacity 150ms ease;
}
.gdm-cover:hover .gdm-cover-actions,
.gdm-cover:focus-within .gdm-cover-actions {
  opacity: 1;
}
@media (hover: none) {
  .gdm-cover-actions {
    opacity: 1;
  }
}
.gdm-pill {
  height: 26px;
  padding: 0 10px;
  border-radius: 6px;
  border: 1px solid rgba(255, 255, 255, 0.16);
  background: rgba(14, 14, 18, 0.78);
  color: #f4f3f0;
  font: 500 11.5px/1 var(--font-ui);
  cursor: pointer;
  backdrop-filter: blur(6px);
}
.gdm-pill.muted {
  color: rgba(244, 243, 240, 0.78);
}
.gdm-pill:hover {
  background: rgba(14, 14, 18, 0.92);
  color: #ffffff;
}
.gdm-drop-hint {
  position: absolute;
  inset: 0;
  display: flex;
  align-items: center;
  justify-content: center;
  background: rgba(14, 14, 18, 0.78);
  color: #f4f3f0;
  font: 500 13px/1 var(--font-ui);
  pointer-events: none;
}
</style>
