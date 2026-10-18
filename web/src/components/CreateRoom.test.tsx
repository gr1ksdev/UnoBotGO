import { fireEvent, render, screen, waitFor } from '@testing-library/react'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { CreateRoom } from './CreateRoom'

describe('CreateRoom', () => {
 beforeEach(() => {
  window.Telegram = { WebApp: { initData: 'signed', ready: vi.fn(), expand: vi.fn() } }
  vi.restoreAllMocks()
 })

 it('creates and enters a free Caseiro room with Updated scoring immediately', async () => {
  const fetcher = vi.spyOn(globalThis, 'fetch').mockResolvedValue({ ok: true, json: async () => ({ game_id: 'free-id', room_type: 'free' }) } as Response)
  const created = vi.fn()
  render(<QueryClientProvider client={new QueryClient({ defaultOptions: { queries: { retry: false } } })}><CreateRoom close={vi.fn()} created={created} /></QueryClientProvider>)
  await waitFor(() => expect(created).toHaveBeenCalled())
  expect(screen.queryByRole('dialog')).not.toBeInTheDocument()
  expect(screen.queryByLabelText('Modo de jogo')).not.toBeInTheDocument()
  expect(screen.queryByLabelText('Regra de pontuação individual')).not.toBeInTheDocument()
  const request = fetcher.mock.calls.find(([url, options]) => String(url).endsWith('/rooms') && options?.method === 'POST')
  expect(JSON.parse(String(request?.[1]?.body))).toMatchObject({ room_type: 'free', system: 'updated', mode: 'caseiro' })
  expect(JSON.parse(String(request?.[1]?.body))).not.toHaveProperty('group_ref')
 })

 it('retries a failed create with the same idempotency key', async () => {
  const fetcher = vi.spyOn(globalThis, 'fetch')
   .mockResolvedValueOnce({ ok: false, json: async () => ({ error: 'temporary' }) } as Response)
   .mockResolvedValueOnce({ ok: true, json: async () => ({ game_id: 'free-id', room_type: 'free' }) } as Response)
  const created = vi.fn()
  render(<QueryClientProvider client={new QueryClient({ defaultOptions: { queries: { retry: false } } })}><CreateRoom close={vi.fn()} created={created} /></QueryClientProvider>)
  fireEvent.click(await screen.findByRole('button', { name: 'Tentar novamente' }))
  await waitFor(() => expect(created).toHaveBeenCalled())
  const payloads = fetcher.mock.calls.filter(([url]) => String(url).endsWith('/rooms')).map(([, options]) => JSON.parse(String(options?.body)))
  expect(payloads).toHaveLength(2)
  expect(payloads[0].request_id).toBe(payloads[1].request_id)
 })
})
