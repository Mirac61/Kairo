<script setup lang="ts">
import { ref, watch } from 'vue'
import { vDialog } from '@/lib/dialog'
import { PROJECT_COLORS } from '@/lib/projectColor'
import { PROJECT_ICON_OPTIONS } from '@/lib/projectIcons'

const props = defineProps<{
  open: boolean
  error?: string
}>()

const emit = defineEmits<{
  (e: 'close'): void
  (
    e: 'save',
    payload: {
      name: string
      description: string
      local_path: string | null
      github_repo: string | null
      color?: string
      icon?: string
    },
  ): void
}>()

const emptyForm = () => ({
  name: '',
  description: '',
  local_path: '',
  github_repo: '',
  color: '',
  icon: '',
})

const form = ref(emptyForm())

watch(
  () => props.open,
  (isOpen) => {
    if (isOpen) form.value = emptyForm()
  },
)

function submit() {
  const name = form.value.name.trim()
  if (!name) return

  emit('save', {
    name,
    description: form.value.description.trim(),
    local_path: form.value.local_path.trim() || null,
    github_repo: form.value.github_repo.trim() || null,
    color: form.value.color || undefined,
    icon: form.value.icon || undefined,
  })
}
</script>

<template>
  <div
    v-if="open"
    v-dialog="() => emit('close')"
    class="sheet-wrap"
    @mousedown.self="emit('close')"
  >
    <form class="sheet p-sheet" :aria-label="$t('Neues Projekt')" @submit.prevent="submit">
      <div class="sheet-head">
        <span class="lbl">{{ $t('Neues Projekt') }}</span>
        <button class="icon-btn" type="button" :aria-label="$t('Schließen')" @click="emit('close')">
          <svg class="ic"><use href="#i-x" /></svg>
        </button>
      </div>

      <div class="sheet-body">
        <input
          id="p-name"
          v-model="form.name"
          class="sheet-title"
          :placeholder="$t('Projektname')"
          :aria-label="$t('Name')"
          required
          autofocus
        />

        <input
          id="p-desc"
          v-model="form.description"
          class="sh-sub-in"
          :placeholder="$t('Beschreibung')"
          :aria-label="$t('Beschreibung')"
        />

        <div v-if="error" class="badge sh-err" role="alert">{{ error }}</div>

        <div class="sheet-props">
          <label class="sheet-row">
            <span>{{ $t('Ordner') }}</span>
            <input
              id="p-path"
              v-model="form.local_path"
              class="input"
              :placeholder="$t('Absoluter Pfad oder ~/… auf deinem Rechner (optional)')"
            />
          </label>

          <label class="sheet-row">
            <span>{{ $t('GitHub-Repo') }}</span>
            <input
              id="p-repo"
              v-model="form.github_repo"
              class="input"
              :placeholder="$t('owner/name, sonst aus dem Ordner (optional)')"
            />
          </label>

          <label class="sheet-row">
            <span>{{ $t('Symbol') }}</span>
            <select id="p-icon" v-model="form.icon" class="input">
              <option value="">{{ $t('Automatisch') }}</option>
              <option v-for="[k, label] in PROJECT_ICON_OPTIONS" :key="k" :value="k">
                {{ $t(label) }}
              </option>
            </select>
          </label>

          <div class="sheet-row">
            <span id="p-color-l">{{ $t('Farbe') }}</span>
            <div class="swatches" role="radiogroup" aria-labelledby="p-color-l">
              <label
                v-for="[c, label] in PROJECT_COLORS"
                :key="c"
                class="swatch"
                :title="$t(label)"
              >
                <input
                  v-model="form.color"
                  type="radio"
                  name="p-color"
                  :value="c"
                  :aria-label="$t(label)"
                />
                <span :style="{ background: `var(--p-${c})` }"></span>
              </label>
            </div>
          </div>
        </div>
      </div>

      <div class="sheet-foot">
        <span class="spacer" />
        <button class="btn btn-ghost" type="button" @click="emit('close')">
          {{ $t('Abbrechen') }}
        </button>
        <button class="btn btn-primary" type="submit">{{ $t('Anlegen') }}</button>
      </div>
    </form>
  </div>
</template>

<style scoped>
.p-sheet {
  --sheet-lbl-w: 104px;
}
.sh-sub-in {
  width: 100%;
  padding: 6px 0;
  margin-top: 2px;
  border: 0;
  border-bottom: 1.5px solid transparent;
  border-radius: 0;
  background: none;
  color: var(--tx-secondary);
  font: 400 13.5px/1.4 var(--font-ui);
}
.sh-sub-in::placeholder {
  color: var(--tx-muted);
}
.sh-sub-in:focus-visible {
  outline: none;
  border-radius: 0;
  box-shadow: none;
  border-bottom-color: var(--a-focus);
}
.sh-err {
  margin-top: 12px;
}
.swatches {
  display: flex;
  gap: 8px;
  flex-wrap: wrap;
  padding: 4px 6px;
}
.swatch {
  cursor: pointer;
}
.swatch input {
  position: absolute;
  opacity: 0;
  width: 0;
  height: 0;
}
.swatch span {
  display: block;
  width: 20px;
  height: 20px;
  border-radius: 50%;
  border: 2px solid transparent;
  transition: transform 0.1s;
}
.swatch input:checked + span {
  border-color: var(--tx-primary);
  transform: scale(1.1);
}
.swatch input:focus-visible + span {
  outline: 2px solid var(--ring);
  outline-offset: 2px;
}
.spacer {
  flex: 1;
}
</style>
