import type { GameView } from '../api/client'
export type HandCard = GameView['hand'][number]
export type HandSlot = { id: string; x: number; y: number; width: number; height: number; hitWidth: number; row: number }
const colorOrder: Record<number, number> = { 1: 0, 4: 1, 3: 2, 2: 3, 0: 4 }
// This order is presentation only. The engine's array and card identities are untouched.
export function orderedHand(hand: HandCard[]): HandCard[] {
 return [...hand].sort((a, b) => {
  const color = (c: HandCard) => c.Card.Rank >= 13 ? 4 : (colorOrder[c.Card.Color] ?? 4)
  return color(a) - color(b) || a.Card.Rank - b.Card.Rank || a.Card.ID.localeCompare(b.Card.ID, 'en', { numeric: true })
 })
}
export function handLayout(hand: HandCard[], viewportWidth: number, viewportHeight = Infinity) {
 const ordered = orderedHand(hand), rows = Math.ceil(ordered.length / 8)
 const available = Math.max(0, viewportWidth - 32)
 const width = Math.min(rows > 1 ? 80 : 96, available / 3.6, Math.max(56, (viewportHeight - 32) * 256 / 344))
 const height = width * 344 / 256, rowHeight = height + 24
 const slots: HandSlot[] = []
 for (let start = 0; start < ordered.length; start += 8) {
  const cards = ordered.slice(start, start + 8)
  const stride = Math.min(width * .58, (available - width) / Math.max(1, cards.length - 1))
  const rowWidth = width + stride * (cards.length - 1)
  const left = (viewportWidth - rowWidth) / 2
  cards.forEach((card, index) => slots.push({ id: card.Card.ID, x: left + index * stride, y: 24 + Math.floor(start / 8) * rowHeight, width, height, hitWidth: index === cards.length - 1 ? width : stride, row: Math.floor(start / 8) }))
 }
 return { ordered, slots, rows, height: rows ? 32 + (rows - 1) * rowHeight + height : 0 }
}
