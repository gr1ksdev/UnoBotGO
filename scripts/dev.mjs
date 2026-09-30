import { spawn } from 'node:child_process'
import process from 'node:process'

console.log('[dev] Starting backend (Go --dev) and frontend (Vite)...')

const backend = spawn('go', ['run', './cmd/bot', '--dev'], {
  stdio: 'inherit',
  env: process.env,
})

const frontend = spawn('npm', ['--prefix', 'web', 'run', 'dev'], {
  stdio: 'inherit',
  env: process.env,
})

let shuttingDown = false

function cleanup(exitCode = 0) {
  if (shuttingDown) return
  shuttingDown = true
  console.log('[dev] Shutting down child processes...')

  if (!backend.killed) {
    backend.kill('SIGTERM')
  }
  if (!frontend.killed) {
    frontend.kill('SIGTERM')
  }

  setTimeout(() => {
    if (!backend.killed) backend.kill('SIGKILL')
    if (!frontend.killed) frontend.kill('SIGKILL')
    process.exit(exitCode)
  }, 3000)
}

backend.on('exit', (code) => {
  if (!shuttingDown) {
    console.log(`[dev] Backend exited with code ${code}`)
    cleanup(code ?? 1)
  }
})

frontend.on('exit', (code) => {
  if (!shuttingDown) {
    console.log(`[dev] Frontend exited with code ${code}`)
    cleanup(code ?? 1)
  }
})

process.on('SIGINT', () => cleanup(0))
process.on('SIGTERM', () => cleanup(0))
