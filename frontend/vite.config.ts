import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'
import tailwindcss from '@tailwindcss/vite'

// Vite configuration: React plugin, Tailwind v4 (CSS-first via @tailwindcss/vite),
// and a dev-server proxy that forwards /api and /ws to the Go backend (default :8080).
export default defineConfig({
  plugins: [react(), tailwindcss()],
  server: {
    port: 5173,
    proxy: {
      '/api': { target: 'http://localhost:8080', changeOrigin: true },
      '/ws': { target: 'ws://localhost:8080', ws: true, changeOrigin: true },
    },
  },
  build: {
    outDir: 'dist',
    // Do not delete the output directory on build. This keeps the build
    // non-destructive (no rmSync of dist) and is safe for hashed asset names.
    emptyOutDir: false,
    chunkSizeWarningLimit: 1500,
  },
})
