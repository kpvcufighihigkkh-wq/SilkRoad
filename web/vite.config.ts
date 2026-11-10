import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'
import { fileURLToPath, URL } from 'node:url'

// 开发期把 /v1 与 /health 代理到本地 Center 服务，避免跨域和硬编码地址。
// 代理目标可用 VITE_API_PROXY 覆盖，默认指向本地 8080。
const apiTarget = process.env.VITE_API_PROXY ?? 'http://127.0.0.1:8080'

export default defineConfig({
  plugins: [react()],
  resolve: {
    alias: {
      '@': fileURLToPath(new URL('./src', import.meta.url)),
    },
  },
  server: {
    port: 5173,
    proxy: {
      '/v1': { target: apiTarget, changeOrigin: true },
      '/health': { target: apiTarget, changeOrigin: true },
    },
  },
})
