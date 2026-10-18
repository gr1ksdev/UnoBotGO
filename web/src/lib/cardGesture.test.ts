import { describe, expect, it } from 'vitest'
import { dragPose, releasedOnDiscard } from './cardGesture'
const discard = {x:150,y:250,width:95,height:128}
describe('physical card release',()=>{
 it('requires travel toward the discard, rejecting taps and sideways/away gestures',()=>{
  expect(releasedOnDiscard({x:150,y:480},{x:150,y:250},discard)).toBe(true)
  expect(releasedOnDiscard({x:150,y:250},{x:151,y:249},discard)).toBe(false)
  expect(releasedOnDiscard({x:150,y:480},{x:280,y:480},discard)).toBe(false)
  expect(releasedOnDiscard({x:150,y:480},{x:150,y:550},discard)).toBe(false)
  expect(releasedOnDiscard({x:150,y:180},{x:150,y:270},discard)).toBe(true)
 })
 it('bounds tilt and shadow elevation even on long drags',()=>{
  expect(dragPose({x:1000,y:-1000})).toEqual({rotation:.35,lift:28})
  expect(dragPose({x:-1000,y:1000}).rotation).toBe(-.35)
  expect(dragPose({x:0,y:0})).toEqual({rotation:0,lift:0})
 })
})
