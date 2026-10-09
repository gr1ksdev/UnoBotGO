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
  expect(screen.getByRole('button', { name: 'Comprar carta' })).toBeDisabled()
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
 it('keeps accepted motion metadata when same-revision HTTP recovery arrives late', () => {
  expect(acceptSnapshot(view,{...view,recovery:true}).recovery).toBeUndefined()
  expect(acceptSnapshot(view,{...view,revision:view.revision+1,recovery:true}).recovery).toBe(true)
 })
 it('uses a dedicated visual for hand swap', () => {
  expect(cardAsset({ ID: 'swap', Color: 0, Rank: 15 })).toBe(
   '/assets/cards/swap_hands.png',
  )
 })
})

describe('v2 resources and consensus', () => {
 it('maps wild display variants without changing the physical card', () => {
  const card = { ID: 'physical-wild', Color: 0, Rank: 14 }
  expect(cardAsset(card)).toBe('/assets/cards/wild_draw4.png')
  expect(cardAsset(card, 4)).toBe('/assets/cards/yellow_wild_draw4.png')
  expect(card).toEqual({ ID: 'physical-wild', Color: 0, Rank: 14 })
  expect(cardAsset({ ID:'reverse', Color:2, Rank:11 })).toBe('/assets/cards/blue_reverse.png')
 })
 it('deduplicates rapid color taps and restores choices after server rejection', () => {
  const send = vi.fn()
  const choice = { ...view, phase:2 }
  const props = { connected:true, error:'', send, back:vi.fn() }
  const rendered = render(<MemoryRouter><GameTable {...props} view={choice} pending={false} /></MemoryRouter>)
  const red = screen.getByRole('button',{name:'Vermelho'})
  fireEvent.click(red);fireEvent.click(red)
  expect(send).toHaveBeenCalledTimes(1)
  rendered.rerender(<MemoryRouter><GameTable {...props} view={choice} pending={true} /></MemoryRouter>)
  expect(red).toBeDisabled()
  rendered.rerender(<MemoryRouter><GameTable {...props} view={choice} pending={false} error="Comando rejeitado" /></MemoryRouter>)
  fireEvent.click(screen.getByRole('button',{name:'Azul'}))
  expect(send).toHaveBeenLastCalledWith('color',{color:2})
  expect(send).toHaveBeenCalledTimes(2)
 })
 it('disables an engine-playable card when the authorized turn belongs to another player', () => {
  wrap({...view,my_turn:false})
  expect(screen.getByRole('button',{name:/Vermelho 2/})).toBeDisabled()
 })
 it('shows real vote status and sends acceptance instead of local reset', () => {
  const send = wrap({...view,closed:true,phase:3,hand:[],my_turn:false,rematch:{revision:1,required:['me','other'],accepted:['other'],next_game_id:'',ready:true}})
  expect(screen.getByLabelText('Outra pessoa aceitou a revanche')).toBeInTheDocument()
  expect(screen.getByText('1/2 aceitaram · faltam você')).toBeInTheDocument()
  fireEvent.click(screen.getByRole('button',{name:'Aceitar revanche'}))
  expect(send).toHaveBeenCalledWith('rematch')
 })
 it('ignores delayed vote snapshots and keeps confirmed score and private hand through terminal DB outages', () => {
  const terminal = {...view,closed:true,rematch:{revision:3,required:['me','other'],accepted:['me'],next_game_id:'',ready:true},result:{id:'one',group:'Mesa',origin:'webapp' as const,system:'updated' as const,position:1,score_units:'1000',status:'scored',finished_at:'now'}}
  expect(acceptSnapshot(terminal,{...terminal,rematch:{...terminal.rematch,revision:2}})).toBe(terminal)
  const recovered = acceptSnapshot(terminal,{...terminal,result:null,hand:[]})
  expect(recovered.result).toEqual(terminal.result)
  expect(recovered.hand).toEqual(view.hand)
 })
})

it('keeps touch targets by CardID while new cards reorganize, and disables immediately on turn change', () => {
 const send = vi.fn(), props = {connected:true,pending:false,error:'',send,back:vi.fn()}
 const cards = [{Card:{ID:'r2',Color:1,Rank:2},Playable:true},{Card:{ID:'b7',Color:2,Rank:7},Playable:true}]
 const rendered=render(<MemoryRouter><GameTable {...props} view={{...view,hand:cards}} /></MemoryRouter>)
 const button=screen.getByRole('button',{name:'Vermelho 2 · pode jogar'})
 const x=button.style.left
 fireEvent.pointerDown(button)
 const added=[...cards,{Card:{ID:'r1',Color:1,Rank:1},Playable:true}]
 rendered.rerender(<MemoryRouter><GameTable {...props} view={{...view,revision:4,hand:added}} /></MemoryRouter>)
 expect(button.style.left).toBe(x)
 fireEvent.pointerUp(button);fireEvent.click(button)
 expect(button.style.left).not.toBe(x)
 fireEvent.click(screen.getByRole('button',{name:'Jogar carta'}))
 expect(send).toHaveBeenCalledWith('play',{card_id:'r2'})
 rendered.rerender(<MemoryRouter><GameTable {...props} view={{...view,revision:5,my_turn:false,hand:added}} /></MemoryRouter>)
 expect(button).toBeDisabled()
 expect(screen.getByRole('button',{name:'Escolher carta'})).toBeDisabled()
})
