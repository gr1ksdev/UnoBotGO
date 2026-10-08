import { defineConfig } from 'vitest/config'
import react from '@vitejs/plugin-react'
import tailwind from '@tailwindcss/vite'

export default defineConfig({
 plugins: [react(), tailwind()],
 server: { proxy: { '/api': { target: 'http://127.0.0.1:8080', ws: true } } },
 build: { outDir: 'dist', emptyOutDir: true },
 test: { environment: 'jsdom', setupFiles: ['./src/test/setup.ts'], restoreMocks: true },
})
