import { defineConfig } from 'vitest/config'
import react from '@vitejs/plugin-react'
import tailwind from '@tailwindcss/vite'

export default defineConfig({
 plugins: [react(), tailwind()],
 cacheDir: process.env.UNO_VITE_CACHE_DIR || 'node_modules/.vite',
 optimizeDeps: { include: ['pixi.js', 'pixi.js/unsafe-eval', 'gsap/PixiPlugin'] },
 server: { proxy: { '/api': { target: process.env.VITE_API_PROXY_TARGET || 'http://127.0.0.1:8080', ws: true } } },
 build: { outDir: 'dist', emptyOutDir: true },
 test: { environment: 'jsdom', setupFiles: ['./src/test/setup.ts'], restoreMocks: true },
})
