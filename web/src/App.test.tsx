import { describe, expect, it, vi, beforeEach } from 'vitest'
import { render, screen, fireEvent, waitFor } from '@testing-library/react'
import { MemoryRouter } from 'react-router'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import App from './App'
import type { RankingPage } from './api/client'
const items = Array.from({ length: 6 }, (_, i) => ({
 position: i + 1,
 key: `p${i}`,
 name: `Jogador ${i + 1}`,
 masked_id: '',
 score_units: '1000',
 avatar_url: '',
}))
const page: RankingPage = {
 month_start: '2026-10-01',
 month_name: 'Outubro',
 month_ends_at: '2026-11-01T03:00:00Z',
 server_time: '2026-10-08T15:00:00Z',
 timezone: 'America/Sao_Paulo',
 system: 'updated',
 items,
}
function wrap(path = '/ranking') {
 return render(
  <QueryClientProvider
   client={new QueryClient({ defaultOptions: { queries: { retry: false } } })}
  >
   <MemoryRouter initialEntries={[path]}>
    <App />
   </MemoryRouter>
  </QueryClientProvider>,
 )
}
beforeEach(() => {
 window.Telegram = {
  WebApp: { initData: 'test', ready: vi.fn(), expand: vi.fn() },
 }
 vi.spyOn(globalThis, 'fetch').mockImplementation(async (input) => {
  const url = String(input)
  const data = url.includes('/position')
   ? {
      entry: { ...items[0], name: 'Minha conta', position: 42 },
      month_start: page.month_start,
     }
   : url.includes('/privacy')
     ? { anonymous: false }
     : url.includes('/config')
       ? { bot_username: 'test_bot' }
       : url.endsWith('/rooms')
         ? []
         : url.endsWith('/me')
           ? { stats: [], history: [] }
           : page
  return { ok: true, json: async () => data } as Response
 })
})
describe('Mobile app', () => {
 it('requires Telegram authentication', () => {
  window.Telegram = undefined
  wrap()
  expect(
   screen.getByText('Abra o Ranking Global pelo Telegram.'),
  ).toBeInTheDocument()
 })
 it('identifies the month and has only player/group categories', async () => {
  wrap()
  await screen.findByText('Outubro 2026')
  expect(screen.getAllByRole('tab')).toHaveLength(2)
  expect(screen.queryByRole('tab', { name: 'Inline' })).not.toBeInTheDocument()
 })
 it('orders podium second/first/third without duplicating the first three in the list', async () => {
  const { container } = wrap()
  await screen.findByText('Jogador 6')
  expect(
   [...container.querySelectorAll('.podium-name')].map((n) => n.textContent),
  ).toEqual(['Jogador 2', 'Jogador 1', 'Jogador 3'])
  expect(screen.getAllByText('Jogador 1')).toHaveLength(1)
 })
 it('shows the real personal position outside the page', async () => {
  wrap()
  await screen.findByText('Minha conta')
  expect(screen.getByText('42')).toBeInTheDocument()
 })
 it('keeps Legacy and Updated separate', async () => {
  wrap()
  await screen.findByText('Jogador 1')
  fireEvent.click(screen.getByRole('button', { name: 'Legado' }))
  await waitFor(() =>
   expect(fetch).toHaveBeenCalledWith(
    expect.stringContaining('system=legacy'),
    expect.anything(),
   ),
  )
 })
 it('switches groups without adding transport categories', async () => {
  wrap()
  fireEvent.click(screen.getByRole('tab', { name: 'Grupos' }))
  await waitFor(() =>
   expect(fetch).toHaveBeenCalledWith(
    expect.stringContaining('/rankings/groups'),
    expect.anything(),
   ),
  )
 })
 it('uses the configured bot link and never displays fixture statistics', async () => {
  wrap('/home')
  await screen.findByRole('link', { name: 'Jogar pelo Telegram' })
  expect(
   screen.getByRole('link', { name: 'Jogar pelo Telegram' }),
  ).toHaveAttribute('href', 'https://t.me/test_bot')
  expect(screen.queryByText('#12')).not.toBeInTheDocument()
 })
 it('shows a small ranking without fabricated podium positions', async () => {
  vi
   .mocked(fetch)
   .mockResolvedValue({
    ok: true,
    json: async () => ({ ...page, items: items.slice(0, 1) }),
   } as Response)
  const { container } = wrap()
  await screen.findByText('Jogador 1')
  expect(container.querySelector('.podium')).toBeNull()
  expect(screen.queryByText('Jogador 2')).not.toBeInTheDocument()
 })
 it('has a real empty state', async () => {
  vi
   .mocked(fetch)
   .mockResolvedValue({
    ok: true,
    json: async () => ({ ...page, items: [] }),
   } as Response)
  wrap()
  await screen.findByText('Ainda sem pontuação')
  expect(screen.queryByText('Jogador 1')).not.toBeInTheDocument()
 })
 it('offers retry on errors', async () => {
  vi.mocked(fetch).mockResolvedValue({ ok: false, status: 404 } as Response)
  wrap()
  await screen.findByRole('button', { name: 'Tentar novamente' })
  fireEvent.click(screen.getByRole('button', { name: 'Tentar novamente' }))
 })
})
