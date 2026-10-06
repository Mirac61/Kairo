import { fileURLToPath, URL } from 'node:url'
import { mkdirSync, writeFileSync } from 'node:fs'
import type { ClientRequest, IncomingMessage } from 'node:http'
import { defineConfig, type Plugin, type ProxyOptions } from 'vite'
import vue from '@vitejs/plugin-vue'

const outDir = fileURLToPath(new URL('../backend/internal/web/dist', import.meta.url))
const backend = 'http://127.0.0.1:8742'
const devOrigins = ['http://127.0.0.1:5173', 'http://localhost:5173']

// Das Backend akzeptiert ändernde Requests nur vom eigenen Origin (siehe
// docs/02, "Lokale Sicherheit"). Der Dev-Proxy schreibt deshalb den Origin
// des Vite-Servers auf den des Backends um. Fremde Origins bleiben
// unverändert und werden vom Backend abgelehnt.
const backendProxy: ProxyOptions = {
  target: backend,
  changeOrigin: true,
  configure: (proxy) => {
    const rewriteOrigin = (proxyReq: ClientRequest, req: IncomingMessage) => {
      const origin = req.headers.origin
      if (origin && devOrigins.includes(origin)) proxyReq.setHeader('Origin', backend)
    }
    proxy.on('proxyReq', rewriteOrigin)
    proxy.on('proxyReqWs', rewriteOrigin)
  },
}

// emptyOutDir löscht auch .gitkeep; das Backend braucht es im Repo.
function restoreGitkeep(): Plugin {
  return {
    name: 'kairo-restore-gitkeep',
    apply: 'build',
    closeBundle() {
      mkdirSync(outDir, { recursive: true })
      writeFileSync(`${outDir}/.gitkeep`, '')
    },
  }
}

export default defineConfig({
  plugins: [vue(), restoreGitkeep()],
  resolve: { alias: { '@': fileURLToPath(new URL('./src', import.meta.url)) } },
  server: {
    host: '127.0.0.1',
    port: 5173,
    proxy: {
      '/api': backendProxy,
      '/ws': { ...backendProxy, ws: true },
    },
  },
  build: { outDir, emptyOutDir: true },
})
