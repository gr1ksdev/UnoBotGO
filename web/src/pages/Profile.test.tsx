import { beforeEach, describe, expect, it, vi } from 'vitest'
import { render, screen, fireEvent, waitFor } from '@testing-library/react'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { MemoryRouter } from 'react-router'
import { ProfilePage } from './Profile'
function wrap() {
 return render(
  <QueryClientProvider
   client={new QueryClient({ defaultOptions: { queries: { retry: false } } })}
  >
   <MemoryRouter>
    <ProfilePage />
   </MemoryRouter>
  </QueryClientProvider>,
 )
}
beforeEach(() => {
 window.Telegram = {
  WebApp: {
   initData: 'test',
   initDataUnsafe: {
    user: { first_name: 'Lucas', last_name: 'Pereira', username: 'lucas' },
   },
   ready: vi.fn(),
   expand: vi.fn(),
  },
 }
 vi.spyOn(globalThis, 'fetch').mockImplementation(async (input, options) => {
  const url = String(input)
  const data = url.endsWith('/privacy')
   ? { anonymous: options?.method === 'PUT' }
   : url.includes('/position')
     ? { entry: null, month_name: 'Outubro' }
     : {
        stats: [
         { system: 'updated', score_units: '1000', games: 2, wins: 1 },
         { system: 'legacy', score_units: '100', games: 1, wins: 1 },
        ],
        history: [
         {
          id: 'one',
          group: 'Grupo anônimo',
          origin: 'inline',
          system: 'updated',
          position: 1,
          score_units: '1000',
          finished_at: '2026-10-08',
         },
         {
          id: 'two',
          group: 'Mesa',
          origin: 'webapp',
          system: 'legacy',
          position: 2,
          score_units: '0',
          finished_at: '2026-10-08',
         },
        ],
       }
  return { ok: true, json: async () => data } as Response
 })
})
describe('Real profile', () => {
 it('uses the Telegram identity and combined eligible statistics', async () => {
  wrap()
  await screen.findByText('10,00')
  expect(
   screen.getByRole('heading', { name: 'Lucas Pereira' }),
  ).toBeInTheDocument()
  expect(screen.getByText('3')).toBeInTheDocument()
  expect(screen.getByText('67%')).toBeInTheDocument()
 })
 it('identifies Inline and Mini App history once each', async () => {
  wrap()
  await screen.findByText('Grupo anônimo')
  expect(screen.getAllByText('Inline')).toHaveLength(1)
  expect(screen.getAllByText('Mini App')).toHaveLength(1)
 })
 it('changes privacy through the authenticated API', async () => {
  wrap()
  const toggle = await screen.findByRole('switch')
  await waitFor(() => expect(toggle).not.toBeDisabled())
  fireEvent.click(toggle)
  await screen.findByText('Salvo com sucesso')
  expect(toggle).toHaveAttribute('aria-checked', 'true')
  expect(screen.getByRole('heading', { name: 'Anônimo' })).toBeInTheDocument()
 })
 it('does not add scores from incompatible policies', async () => {
  wrap()
  await screen.findByText('10,00')
  fireEvent.click(screen.getByRole('button', { name: 'Legado' }))
  await screen.findByText('Pontuação total · Legado')
  expect(screen.queryByText('11,00')).not.toBeInTheDocument()
 })
 it('reports a failed privacy write', async () => {
  const base = vi.mocked(fetch).getMockImplementation()!
  vi
   .mocked(fetch)
   .mockImplementation((input, options) =>
    options?.method === 'PUT'
     ? Promise.resolve({ ok: false, status: 500 } as Response)
     : base(input, options),
   )
  wrap()
  const toggle = await screen.findByRole('switch')
  await waitFor(() => expect(toggle).not.toBeDisabled())
  fireEvent.click(toggle)
  await screen.findByText('Não foi possível salvar a alteração.')
  expect(toggle).toHaveAttribute('aria-checked', 'false')
 })
})
