import { StrictMode } from 'react'
import { act, renderHook } from '@testing-library/react'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { useTelegram } from './telegram'

function client() {
 const events = new Map<string, () => void>()
 const app = {
   initData: 'fixture', ready: vi.fn(), expand: vi.fn(),
   isVersionAtLeast: vi.fn(() => true), isFullscreen: false,
   requestFullscreen: vi.fn(), setHeaderColor: vi.fn(),
   safeAreaInset: { top: 24, bottom: 12, left: 0, right: 0 },
   contentSafeAreaInset: { top: 32, bottom: 0, left: 0, right: 0 },
   viewportStableHeight: 780,
   onEvent: vi.fn((event: string, fn: () => void) => { events.set(event, fn) }),
   offEvent: vi.fn((event: string, fn: () => void) => { if (events.get(event) === fn) events.delete(event) }),
 }
 window.Telegram = { WebApp: app }
 return { app, events }
}

afterEach(() => {
 window.Telegram = undefined
 document.documentElement.removeAttribute('style')
})

describe('Telegram fullscreen', () => {
 it('initializes once under StrictMode and registers events before requesting fullscreen', () => {
   const { app, events } = client()
   app.requestFullscreen.mockImplementation(() => {
     expect(events.has('fullscreenChanged')).toBe(true)
     expect(events.has('fullscreenFailed')).toBe(true)
   })
   const { rerender } = renderHook(() => useTelegram(undefined, { initialize: true, manageBackButton: false }), {
     wrapper: ({ children }) => <StrictMode>{children}</StrictMode>,
   })
   rerender()
   expect(app.ready).toHaveBeenCalledTimes(1)
   expect(app.expand).toHaveBeenCalledTimes(1)
   expect(app.requestFullscreen).toHaveBeenCalledTimes(1)
 })

 it('does not reinitialize from the page hook', () => {
   const { app } = client()
   renderHook(() => useTelegram(undefined, { headerColor: '#073b82' }))
   expect(app.requestFullscreen).not.toHaveBeenCalled()
   expect(app.ready).not.toHaveBeenCalled()
   expect(app.setHeaderColor).toHaveBeenCalledWith('#073b82')
 })

 it('keeps expanded mode when the API version lacks fullscreen support', () => {
   const { app } = client()
   app.isVersionAtLeast.mockReturnValue(false)
   renderHook(() => useTelegram(undefined, { initialize: true }))
   expect(app.expand).toHaveBeenCalledTimes(1)
   expect(app.requestFullscreen).not.toHaveBeenCalled()
 })

 it('supports older SDKs without fullscreen methods', () => {
   const app = { initData: 'fixture', ready: vi.fn(), expand: vi.fn() }
   window.Telegram = { WebApp: app }
   renderHook(() => useTelegram(undefined, { initialize: true }))
   expect(app.expand).toHaveBeenCalledTimes(1)
 })

 it('does not request fullscreen if already active', () => {
   const { app } = client()
   app.isFullscreen = true
   renderHook(() => useTelegram(undefined, { initialize: true }))
   expect(app.requestFullscreen).not.toHaveBeenCalled()
 })

 it('continues in expanded mode when requesting fullscreen throws', () => {
   const { app } = client()
   app.requestFullscreen.mockImplementation(() => { throw new Error('Unsupported') })
   expect(() => renderHook(() => useTelegram(undefined, { initialize: true }))).not.toThrow()
   expect(app.expand).toHaveBeenCalledTimes(1)
 })

 it('updates safe areas and viewport on fullscreen changes and failure without retrying', () => {
   const { app, events } = client()
   const { unmount } = renderHook(() => useTelegram(undefined, { initialize: true }))
   expect(document.documentElement.style.getPropertyValue('--telegram-top')).toBe('56px')
   app.safeAreaInset.top = 28
   app.contentSafeAreaInset.top = 40
   app.viewportStableHeight = 844
   act(() => events.get('fullscreenChanged')?.())
   expect(document.documentElement.style.getPropertyValue('--telegram-top')).toBe('68px')
   expect(document.documentElement.style.getPropertyValue('--viewport-height')).toBe('844px')
   act(() => events.get('fullscreenFailed')?.())
   expect(app.requestFullscreen).toHaveBeenCalledTimes(1)
   unmount()
   expect(events.size).toBe(0)
 })

 it('changes native control contrast with the page header color', () => {
   const { app } = client()
   const { rerender } = renderHook(({ color }) => useTelegram(undefined, { headerColor: color }), {
     initialProps: { color: '#073b82' },
   })
   rerender({ color: '#99121f' })
   expect(app.setHeaderColor).toHaveBeenLastCalledWith('#99121f')
 })

 it('registers and cleanly unregisters BackButton callbacks without leaks', () => {
   const { app } = client()
   const backButton = {
     show: vi.fn(),
     hide: vi.fn(),
     onClick: vi.fn(),
     offClick: vi.fn(),
   }
   Object.assign(app, { BackButton: backButton })
   const backHandler = vi.fn()
   const { unmount, rerender } = renderHook(({ back }) => useTelegram(back), {
     initialProps: { back: backHandler as (() => void) | undefined },
   })

   expect(backButton.show).toHaveBeenCalledTimes(1)
   expect(backButton.onClick).toHaveBeenCalledWith(backHandler)
   expect(backButton.offClick).not.toHaveBeenCalled()

   // Switching back to undefined (e.g. Navigating back to Global view)
   rerender({ back: undefined })
   expect(backButton.offClick).toHaveBeenCalledWith(backHandler)
   expect(backButton.hide).toHaveBeenCalled()

   // Unmounting
   unmount()
   expect(backButton.hide).toHaveBeenCalled()
 })
})
