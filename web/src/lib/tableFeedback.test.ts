import { afterEach, describe, expect, it, vi } from 'vitest'
import { TableFeedback } from './tableFeedback'
afterEach(()=>{localStorage.clear();vi.restoreAllMocks()})
describe('optional table feedback',()=>{
 it('does not load audio until enabled, persists preferences, and muting stops active sound',()=>{
  const audio={play:vi.fn().mockResolvedValue(undefined),pause:vi.fn(),volume:0,addEventListener:vi.fn(),removeEventListener:vi.fn()}
  const make=vi.fn(()=>audio),feedback=new TableFeedback(make)
  feedback.play('draw');expect(make).not.toHaveBeenCalled()
  feedback.configure({sound:true});feedback.play('select')
  expect(make).toHaveBeenCalledWith('/assets/audio/card_select-YPFdVw.mp3')
  feedback.configure({sound:false});expect(audio.pause).toHaveBeenCalled()
  expect(new TableFeedback(make).getSnapshot().sound).toBe(false)
 })
 it('swallows autoplay failures without leaving a sound playing',async()=>{
  const audio={play:vi.fn().mockRejectedValue(new Error('NotAllowedError')),pause:vi.fn(),volume:0,addEventListener:vi.fn(),removeEventListener:vi.fn()}
  const feedback=new TableFeedback(()=>audio)
  feedback.configure({sound:true});feedback.play('play');await Promise.resolve()
  expect(audio.pause).toHaveBeenCalledTimes(1)
  feedback.stop();expect(audio.pause).toHaveBeenCalledTimes(1)
 })
 it('caps concurrent audio during bursts and cleans every active track',()=>{
  const sounds: {play:ReturnType<typeof vi.fn>;pause:ReturnType<typeof vi.fn>;volume:number;addEventListener:ReturnType<typeof vi.fn>;removeEventListener:ReturnType<typeof vi.fn>}[]=[]
  const feedback=new TableFeedback(()=>{const a={play:vi.fn().mockResolvedValue(undefined),pause:vi.fn(),volume:0,addEventListener:vi.fn(),removeEventListener:vi.fn()};sounds.push(a);return a})
  feedback.configure({sound:true})
  for(const kind of ['select','draw','deal','play','color','reverse'] as const) feedback.play(kind)
  expect(sounds.filter(a=>!a.pause.mock.calls.length)).toHaveLength(4)
  feedback.stop();expect(sounds.every(a=>a.pause.mock.calls.length===1)).toBe(true)
 })
})
