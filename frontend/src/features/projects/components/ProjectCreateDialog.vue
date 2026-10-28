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

const form = ref({
  name: '',
  description: '',
  local_path: '',
  github_repo: '',
  status: 'ACTIVE',
  color: '',
  icon: '',
})

watch(
  () => props.open,
  (isOpen) => {
    if (isOpen) {
      form.value = {
        name: '',
        description: '',
        local_path: '',
        github_repo: '',
        status: 'ACTIVE',
        color: '',
        icon: '',
      }
    }
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
    class="overlay open"
    @mousedown.self="emit('close')"
  >
    <div class="dialog" :aria-label="$t('Neues Projekt')">
      <form @submit.prevent="submit">
        <div class="dlg-head">
          <h3>{{ $t('Neues Projekt') }}</h3>
          <button
            class="icon-btn"
            type="button"
            :aria-label="$t('Schließen')"
            @click="emit('close')"
          >
            <svg class="ic"><use href="#i-x" /></svg>
          </button>
        </div>

        <div class="dlg-body">
          <div class="field">
            <label for="p-name">{{ $t('Name') }}</label>
            <input
              id="p-name"
              v-model="form.name"
              class="input"
              :placeholder="$t('Projektname')"
              autofocus
            />
          </div>

          <div class="field">
            <label for="p-desc">{{ $t('Beschreibung') }}</label>
            <input id="p-desc" v-model="form.description" class="input" />
          </div>

          <div v-if="error" class="badge" role="alert">{{ error }}</div>

          <div class="field">
            <label for="p-path">{{ $t('Ordner') }}</label>
            <input
              id="p-path"
              v-model="form.local_path"
              class="input"
              :placeholder="$t('Absoluter Pfad oder ~/… auf deinem Rechner (optional)')"
            />
          </div>

          <div class="field">
            <label for="p-repo">{{ $t('GitHub-Repo') }}</label>
            <input
              id="p-repo"
              v-model="form.github_repo"
              class="input"
              :placeholder="$t('owner/name, sonst aus dem Ordner (optional)')"
            />
          </div>

          <div class="field">
            <label for="p-icon">{{ $t('Symbol') }}</label>
            <select id="p-icon" v-model="form.icon" class="input">
              <option value="">{{ $t('Automatisch') }}</option>
              <option v-for="[k, label] in PROJECT_ICON_OPTIONS" :key="k" :value="k">
                {{ label }}
              </option>
            </select>
          </div>

          <div class="field">
            <label id="p-color-l">{{ $t('Farbe') }}</label>
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
                  :aria-label="label"
                />
                <span :style="{ background: `var(--p-${c})` }"></span>
              </label>
            </div>
          </div>
        </div>

        <div class="dlg-foot">
          <span class="spacer"></span>
          <button class="btn btn-ghost" type="button" @click="emit('close')">
            {{ $t('Abbrechen') }}
          </button>
          <button class="btn btn-primary" type="submit">{{ $t('Anlegen') }}</button>
        </div>
      </form>
    </div>
  </div>
</template>

<style scoped>
.spacer {
  flex: 1;
}
.swatches {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
}
.swatch {
  position: relative;
  display: inline-flex;
  cursor: pointer;
}
.swatch input {
  position: absolute;
  inset: 0;
  opacity: 0;
  margin: 0;
  cursor: pointer;
}
.swatch span {
  width: 22px;
  height: 22px;
  border-radius: 50%;
  border: 2px solid transparent;
  box-shadow: inset 0 0 0 2px var(--bg-1);
}
.swatch input:checked + span {
  border-color: var(--tx-primary);
}
.swatch input:focus-visible + span {
  outline: 2px solid var(--tx-primary);
  outline-offset: 2px;
}
</style>
