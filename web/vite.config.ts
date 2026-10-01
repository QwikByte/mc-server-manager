import { fileURLToPath } from 'node:url'
import tailwindcss from '@tailwindcss/vite'
import react from '@vitejs/plugin-react'
import { defineConfig } from 'vite'

// During development the master runs on :8080 and serves the API (`mcsm-master serve`).
export default defineConfig({
  plugins: [react(), tailwindcss()],
  resolve: { alias: { '@': fileURLToPath(new URL('./src', import.meta.url)) } },
  server: { proxy: { '/api': 'http://127.0.0.1:8080' } },
  // Assets are never inlined as data: URIs, so the CSP can stay strict.
  build: { assetsInlineLimit: 0 },
})
