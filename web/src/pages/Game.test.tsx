import { describe, it, expect, vi } from 'vitest'
import { render, screen, fireEvent } from '@testing-library/react'
import { MemoryRouter } from 'react-router'
import type { GameView } from '../api/client'
import { GameTable, cardAsset } from './Game'
import { acceptSnapshot } from '../hooks/useGame'
const view: GameView = {
 game_id: 'one',
 revision: 3,
 phase: 1,
 group: 'Mesa',
 mode: 'classic',
 system: 'updated',
 players: [
  { key: 'me', name: 'Lucas', count: 7, me: true, current: true, active: true },
  {
   key: 'other',
   name: 'Outra pessoa',
   count: 8,
   me: false,
   current: false,
   active: true,
  },
 ],
 hand: [{ Card: { ID: 'card-one', Color: 1, Rank: 2 }, Playable: true }],
 top: { ID: 'top', Color: 1, Rank: 3 },
 active_color: 1,
 direction: 1,
 closed: false,
 owner: true,
 my_turn: true,
 drawn_card_id: '',
 can_bluff: false,
 deadline: null,
 server_time: new Date().toISOString(),
 result: null,
}
function wrap(v = view, connected = true) {
 const send = vi.fn()
 render(
  <MemoryRouter>
   <GameTable
    view={v}
    connected={connected}
    pending={false}
    error=""
    send={send}
    back={vi.fn()}
   />
  </MemoryRouter>,
 )
 return send
}
describe('Authoritative game UI', () => {
 it('selection does not play until confirmation and sends the physical ID', () => {
  const send = wrap()
  fireEvent.click(
   screen.getByRole('button', { name: 'Vermelho 2 · pode jogar' }),
  )
  expect(send).not.toHaveBeenCalled()
  fireEvent.click(screen.getByRole('button', { name: 'Jogar carta' }))
  expect(send).toHaveBeenCalledWith('play', { card_id: 'card-one' })
 })
 it('disables controls while disconnected', () => {
  wrap(view, false)
  expect(screen.getByRole('button', { name: 'Comprar carta' })).toBeDisabled()
  expect(screen.getByText('Conexão perdida. Reconectando…')).toBeInTheDocument()
 })
 it('finishes two-player controls without inventing committed score', () => {
  wrap({ ...view, closed: true, phase: 3, hand: [], my_turn: false })
  expect(
   screen.queryByRole('button', { name: 'Comprar carta' }),
  ).not.toBeInTheDocument()
  expect(
   screen.getByText('Aguardando confirmação da pontuação'),
  ).toBeInTheDocument()
  expect(screen.queryByText(/10,00 pontos/)).not.toBeInTheDocument()
 })
 it('ignores old revisions and other-room snapshots', () => {
  expect(acceptSnapshot(view, { ...view, revision: 2 })).toBe(view)
  expect(acceptSnapshot(view, { ...view, game_id: 'other' })).toBe(view)
  expect(acceptSnapshot(view, { ...view, revision: 9 }).revision).toBe(9)
 })
 it('uses a dedicated visual for hand swap', () => {
  expect(cardAsset({ ID: 'swap', Color: 0, Rank: 15 })).toBe(
   '/assets/cards/swap.svg',
  )
 })
})
