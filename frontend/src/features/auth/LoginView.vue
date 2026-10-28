<script setup lang="ts">
import { ref } from 'vue'
import { useRouter } from 'vue-router'
import { ApiError } from '@/api/client'
import { login } from '@/composables/useAuth'
import { t } from '@/lib/i18n'

const router = useRouter()
const email = ref('')
const password = ref('')
const error = ref('')
const submitting = ref(false)

async function onSubmit() {
  error.value = ''
  submitting.value = true
  try {
    await login(email.value, password.value)
    router.replace('/')
  } catch (err: unknown) {
    const status = err instanceof ApiError ? err.status : undefined
    error.value =
      status === 401
        ? t('E-Mail oder Passwort ist falsch.')
        : status === 429
          ? t('Zu viele Fehlversuche. Bitte eine Minute warten.')
          : t('Anmeldung fehlgeschlagen.')
  } finally {
    submitting.value = false
  }
}
</script>

<template>
  <div class="auth-page">
    <form class="auth-card" @submit.prevent="onSubmit">
      <div class="auth-brand" aria-hidden="true">
        <span class="auth-logo">K<i></i>IR<i class="o"></i></span>
      </div>
      <h1>{{ t('Bei Kairo anmelden') }}</h1>
      <label>
        <span class="lbl">{{ t('E-Mail') }}</span>
        <input
          v-model="email"
          type="email"
          autocomplete="email"
          required
          :placeholder="t('name@beispiel.de')"
        />
      </label>
      <label>
        <span class="lbl">{{ t('Passwort') }}</span>
        <input
          v-model="password"
          type="password"
          autocomplete="current-password"
          required
          :placeholder="t('Passwort')"
        />
      </label>
      <p v-if="error" class="auth-error">{{ error }}</p>
      <button type="submit" class="btn-primary" :disabled="submitting">
        {{ submitting ? t('Wird angemeldet …') : t('Anmelden') }}
      </button>
    </form>
  </div>
</template>

<style scoped>
.auth-page {
  min-height: 100vh;
  display: grid;
  place-items: center;
  padding: 1.5rem;
  background: var(--bg-0);
}
.auth-card {
  width: min(100%, 24rem);
  display: grid;
  gap: 1rem;
  padding: 2rem;
  background: var(--bg-1);
  border: 1px solid var(--br-default);
  border-radius: var(--r-m);
  box-shadow: var(--shadow-pop);
}
.auth-brand {
  display: flex;
  justify-content: center;
  margin-bottom: 0.25rem;
}
.auth-logo {
  font:
    600 28px/1 'Cormorant Garamond',
    serif;
  letter-spacing: 0.1em;
  color: var(--tx-primary);
}
.auth-logo i {
  display: inline-block;
  position: relative;
  width: 0.74em;
  height: 0.64em;
  margin-right: 0.1em;
  vertical-align: middle;
}
.auth-logo i::before,
.auth-logo i::after {
  content: '';
  position: absolute;
  inset: 0;
}
.auth-logo i::before {
  background: var(--tx-primary);
  clip-path: polygon(50% 0, 0 100%, 58% 100%);
}
.auth-logo i::after {
  background: #7a7a7e;
  clip-path: polygon(50% 0, 58% 100%, 100% 100%);
}
.auth-logo i.o {
  width: 0.64em;
  height: 0.64em;
  border-radius: 50%;
  margin-left: 0.02em;
  background:
    radial-gradient(circle at 62% 60%, oklch(0.4 0.11 32 / 0.6) 0 9%, transparent 11%),
    radial-gradient(
      circle at 34% 30%,
      oklch(0.82 0.12 52),
      oklch(0.64 0.17 38) 42%,
      oklch(0.28 0.08 30) 100%
    );
  box-shadow: 0 0 0.18em 0.01em oklch(0.65 0.17 38 / 0.35);
}
.auth-logo i.o::after {
  content: '';
  position: absolute;
  top: 7%;
  left: 30%;
  width: 30%;
  height: 11%;
  border-radius: 50%;
  background: oklch(0.96 0.02 60 / 0.9);
}
.auth-card h1 {
  font: 600 22px/1.2 var(--font-ui);
  letter-spacing: -0.02em;
  margin: 0;
  text-align: center;
  color: var(--tx-primary);
}
.auth-card label {
  display: grid;
  gap: 0.4rem;
}
.auth-card input {
  padding: 0.65rem 0.8rem;
  border-radius: var(--r-s);
  border: 1px solid var(--br-default);
  background: var(--bg-0);
  color: var(--tx-primary);
  font: 400 14px/1.4 var(--font-ui);
  transition:
    border-color var(--dur) ease-out,
    box-shadow var(--dur) ease-out;
}
.auth-card input::placeholder {
  color: var(--tx-muted);
}
.auth-card input:hover {
  border-color: var(--br-strong);
}
.auth-card input:focus {
  border-color: var(--a-blue-hi);
  box-shadow: 0 0 0 3px var(--ring);
}
.auth-error {
  margin: 0;
  padding: 0.5rem 0.6rem;
  border-radius: var(--r-s);
  background: color-mix(in oklab, var(--a-red) 12%, transparent);
  color: var(--a-red);
  font-size: 13px;
}
.btn-primary {
  padding: 0.7rem 1rem;
  border-radius: var(--r-s);
  border: none;
  background: var(--a-blue);
  color: white;
  font: 500 14px/1 var(--font-ui);
  cursor: pointer;
  transition: filter var(--dur) ease-out;
}
.btn-primary:hover {
  filter: brightness(1.08);
}
.btn-primary:disabled {
  opacity: 0.6;
  cursor: not-allowed;
}
</style>
