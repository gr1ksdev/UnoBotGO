import { describe, expect, it } from 'vitest'
import { handLayout, orderedHand } from './handLayout'
const hand = Array.from({ length: 30 }, (_, i) => ({ Card: { ID: `c${i}`, Color: i % 4 + 1, Rank: i % 15 }, Playable: true }))
describe('physical hand layout', () => {
 for(const viewport of [320,360,390,430]) for(const count of [1,2,7,8,9,15,16,17,30]) it(`${count} cards at ${viewport}: centered rows and exclusive touch strips`, () => {
  const layout = handLayout(hand.slice(0,count),viewport)
  expect(layout.rows).toBe(Math.ceil(count/8))
  expect(new Set(layout.slots.map(c=>c.id)).size).toBe(count)
  for(let row=0;row<layout.rows;row++) {
   const slots = layout.slots.filter(s=>s.row===row), first=slots[0],last=slots.at(-1)!
   expect(slots.length).toBeLessThanOrEqual(8)
   expect(first.x).toBeGreaterThanOrEqual(16)
   expect(last.x+last.width).toBeLessThanOrEqual(viewport-16+.001)
   expect(first.x).toBeCloseTo(viewport-last.x-last.width)
   slots.forEach((slot,i)=> { expect(slot.hitWidth).toBeGreaterThanOrEqual(24); if(i)expect(slot.x).toBeCloseTo(slots[i-1].x+slots[i-1].hitWidth) })
  }
 })
 it('groups red/yellow/green/blue/wild, ranks then physical ID; engine order is immutable', () => {
  const cards = [
   {Card:{ID:'b',Color:2,Rank:0},Playable:true}, {Card:{ID:'r2',Color:1,Rank:2},Playable:true},
   {Card:{ID:'w',Color:0,Rank:13},Playable:true}, {Card:{ID:'g',Color:3,Rank:11},Playable:true},
   {Card:{ID:'y',Color:4,Rank:0},Playable:true}, {Card:{ID:'r1',Color:1,Rank:2},Playable:true},
  ]
  expect(orderedHand(cards).map(c=>c.Card.ID)).toEqual(['r1','r2','y','g','b','w'])
  expect(cards[0].Card.ID).toBe('b')
  expect(orderedHand([...cards].reverse()).map(c=>c.Card.ID)).toEqual(orderedHand(cards).map(c=>c.Card.ID))
 })
})
