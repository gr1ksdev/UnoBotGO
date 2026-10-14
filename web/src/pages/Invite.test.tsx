import { fireEvent, render, screen, waitFor } from '@testing-library/react'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { MemoryRouter, Route, Routes } from 'react-router'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import type { InvitePreview } from '../api/client'
import { InvitePage } from './Invite'

const preview: InvitePreview = {
 game_id: 'room-id', revision: 8, room_type: 'free', group: '', mode: 'caseiro', system: 'updated',
 players: [
  { key: 'owner', name: 'Ana', count: 0, me: false, current: false, active: true },
  { key: 'guest', name: 'Beto', count: 0, me: false, current: false, active: true },
 ], owner_key: 'owner', capacity: 10, joined: false, can_join: true, closed: false,
}

function wrap() {
 const client = new QueryClient({ defaultOptions: { queries: { retry: false } } })
 return render(<QueryClientProvider client={client}><MemoryRouter initialEntries={['/invite/opaque-token']}><Routes><Route path="/invite/:token" element={<InvitePage />} /><Route path="/game/:id" element={<p>Partida aberta</p>} /></Routes></MemoryRouter></QueryClientProvider>)
}

beforeEach(() => {
 window.Telegram = { WebApp: { initData: 'signed', ready: vi.fn(), expand: vi.fn() } }
 vi.spyOn(globalThis, 'fetch').mockImplementation(async (input, init) => {
  const path = String(input)
  if (path.endsWith('/invites/opaque-token')) return { ok: true, json: async () => preview } as Response
  if (path.endsWith('/invites/opaque-token/join') && init?.method === 'POST') return { ok: true, json: async () => ({ game_id: 'room-id' }) } as Response
  return { ok: false, json: async () => ({}) } as Response
 })
})

describe('invite entry screen', () => {
 it('shows only the responsible person and a clear join action', async () => {
  wrap()
  expect(await screen.findByRole('heading', { name: 'Você foi convidado' })).toBeInTheDocument()
  expect(screen.getByText('Convite enviado por')).toBeInTheDocument()
  expect(screen.getByLabelText('Responsável da sala: Ana')).toBeInTheDocument()
  expect(screen.getByText('Ana')).toBeInTheDocument()
  expect(screen.queryByText('Sala livre')).not.toBeInTheDocument()
  expect(screen.queryByText('Caseiro · Troca de mãos')).not.toBeInTheDocument()
  expect(screen.queryByText('Jogadores na sala')).not.toBeInTheDocument()
  expect(screen.getByRole('button', { name: 'Entrar na sala' })).toBeEnabled()
 })

 it('joins only after the player chooses the explicit action', async () => {
  wrap()
  const fetcher = vi.mocked(globalThis.fetch)
  await screen.findByRole('button', { name: 'Entrar na sala' })
  expect(fetcher.mock.calls.some(([url, init]) => String(url).endsWith('/join') && init?.method === 'POST')).toBe(false)
  fireEvent.click(screen.getByRole('button', { name: 'Entrar na sala' }))
  await waitFor(() => expect(fetcher.mock.calls.some(([url, init]) => String(url).endsWith('/join') && init?.method === 'POST')).toBe(true))
  expect(await screen.findByText('Partida aberta')).toBeInTheDocument()
 })

 it('explains when the room is full and disables joining', async () => {
  vi.mocked(globalThis.fetch).mockImplementation(async input => String(input).endsWith('/invites/opaque-token')
   ? { ok: true, json: async () => ({ ...preview, can_join: false }) } as Response
   : { ok: false, json: async () => ({}) } as Response)
  wrap()
  expect(await screen.findByText('Esta sala está cheia ou não aceita mais participantes.')).toBeInTheDocument()
  expect(screen.getByRole('button', { name: 'Entrar na sala' })).toBeDisabled()
 })
})
