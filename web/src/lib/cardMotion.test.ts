import { describe, expect, it } from 'vitest'
import { cardCorners } from './cardMotion'

describe('card flip projection', () => {
 it('lands at the slot dimensions on either face, including the differently sized swap asset', () => {
  for (const [width, height] of [[95, 127.65625], [80, 107.5], [42, 42]]) {
   for (const angle of [0, Math.PI]) {
    const corners = cardCorners(width, height, angle)
    const expected = [-width/2,-height/2,width/2,-height/2,width/2,height/2,-width/2,height/2]
    corners.forEach((value, i) => expect(value).toBeCloseTo(expected[i]))
   }
  }
 })
 it('projects the near edge larger and reverses the depth after the face changes', () => {
  const before = cardCorners(80, 107.5, Math.PI/3)
  const after = cardCorners(80, 107.5, Math.PI*2/3)
  expect(before[7]-before[1]).toBeGreaterThan(before[5]-before[3])
  expect(after[7]-after[1]).toBeLessThan(after[5]-after[3])
 })
 it('never folds, mirrors text or degenerates when crossing the texture swap', () => {
  for (let step=0;step<=180;step++) {
   const c = cardCorners(80,107.5,step*Math.PI/180)
   expect(c.every(Number.isFinite)).toBe(true)
   expect(c[0]).toBeLessThan(c[2])
   expect(c[6]).toBeLessThan(c[4])
   expect(c[1]).toBeLessThan(c[7])
   expect(c[3]).toBeLessThan(c[5])
  }
  const edge = cardCorners(80,107.5,Math.PI/2)
  expect(edge[2]-edge[0]).toBeLessThan(2)
  expect(edge[2]-edge[0]).toBeGreaterThan(0)
 })
})
