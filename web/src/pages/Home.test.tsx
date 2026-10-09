import { render, screen, fireEvent, waitFor } from '@testing-library/react'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { MemoryRouter } from 'react-router'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { HomePage } from './Home'
import { launchGamePath } from '../lib/telegram'
let rooms: { game_id: string; group: string; phase: number }[]
let activated: (() => void) | undefined
function wrap() {
 return render(<QueryClientProvider client={new QueryClient({ defaultOptions: { queries: { retry: false } } })}>
  <MemoryRouter><HomePage /></MemoryRouter>
 </QueryClientProvider>)
}
beforeEach(() => {
 rooms = []
 window.Telegram = { WebApp: { initData: 'signed', ready: vi.fn(), expand: vi.fn(),
  onEvent: (name, fn) => { if (name === 'activated') activated = fn }, offEvent: vi.fn(),
 } }
 vi.spyOn(globalThis, 'fetch').mockImplementation(async input => {
  const path = String(input)
  const data = path.endsWith('/rooms') ? rooms : path.endsWith('/config') ? { bot_username: 'ConfiguredBot' }
   : path.endsWith('/me') ? { stats: [], history: [] } : path.includes('/position') ? { entry: null }
   : { items: [], month_start: '2026-10-01', server_time: '2026-10-08T15:00:00Z', month_ends_at: '2026-11-01T03:00:00Z', system: 'updated' }
  return { ok: true, json: async () => data } as Response
 })
})
describe('Access to real WebApp rooms', () => {
 it('keeps the primary empty-room action inside the app and offers direct creation without bot commands', async () => {
  wrap()
  await screen.findByText(/Nenhuma sala disponível/)
  expect(screen.getByRole('button', { name: 'Criar sala' })).toBeInTheDocument()
  expect(screen.getByText(/não precisa usar comandos/)).toBeInTheDocument()
  expect(screen.getByRole('link', { name: 'Jogar pelo Telegram' })).toHaveAttribute('href', 'https://t.me/ConfiguredBot')
 })
 it('shows a single room with an internal open action', async () => {
  rooms = [{ game_id: 'real-room', group: 'Meu grupo', phase: 0 }]
  wrap()
  const link = await screen.findByRole('link', { name: /Meu grupo.*Abrir na Mini App/ })
  expect(link).toHaveAttribute('href', '/game/real-room')
  expect(screen.getByRole('link', { name: 'Abrir partida' })).toHaveAttribute('href', '/game/real-room')
 })
 it('discovers a newly joined room when Telegram resumes, with manual refresh also available', async () => {
  wrap()
  await screen.findByText(/Nenhuma sala disponível/)
  rooms = [{ game_id: 'joined', group: 'Grupo depois de entrar', phase: 0 }]
  activated?.()
  await screen.findByRole('link', { name: /Grupo depois de entrar.*Abrir na Mini App/ })
  fireEvent.click(screen.getByRole('button', { name: 'Atualizar salas' }))
  await waitFor(() => expect(screen.getByRole('button', { name: 'Atualizar salas' })).toBeEnabled())
 })
 it('treats a launch parameter only as a route and rejects malformed room IDs', () => {
  window.Telegram!.WebApp.initDataUnsafe = { start_param: 'game_room-123' }
  expect(launchGamePath()).toBe('/game/room-123')
  window.Telegram!.WebApp.initDataUnsafe.start_param = 'game_../profile'
  expect(launchGamePath()).toBeUndefined()
 })
})
