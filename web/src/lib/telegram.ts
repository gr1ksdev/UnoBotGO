import { useEffect, useRef } from 'react'

interface Insets { top: number; bottom: number; left: number; right: number }
interface TelegramApp {
 initData: string
 ready(): void
 expand(): void
 close?(): void
 isVersionAtLeast?(version: string): boolean
 isFullscreen?: boolean
 requestFullscreen?(): void
 setHeaderColor?(color: string): void
 safeAreaInset?: Insets
 contentSafeAreaInset?: Insets
 viewportStableHeight?: number
 onEvent?(event: string, fn: () => void): void
 offEvent?(event: string, fn: () => void): void
 BackButton?: { show(): void; hide(): void; onClick(fn: () => void): void; offClick(fn: () => void): void }
}
declare global { interface Window { Telegram?: { WebApp: TelegramApp } } }

export function useTelegram(back?: () => void, options: { manageBackButton?: boolean; initialize?: boolean; headerColor?: string } = {}) {
 const manageBackButton = options.manageBackButton ?? true
 const initialize = options.initialize ?? false
 const headerColor = options.headerColor
 const initializedApp = useRef<TelegramApp | undefined>(undefined)
 useEffect(() => {
   const app = window.Telegram?.WebApp
   if (!app) return
   const update = () => {
     for (const side of ['top', 'bottom', 'left', 'right'] as const) {
       const value = (app.safeAreaInset?.[side] ?? 0) + (app.contentSafeAreaInset?.[side] ?? 0)
       document.documentElement.style.setProperty(`--telegram-${side}`, `${value}px`)
     }
     if (app.viewportStableHeight) document.documentElement.style.setProperty('--viewport-height', `${app.viewportStableHeight}px`)
   }
   update()
   const events = ['safeAreaChanged', 'contentSafeAreaChanged', 'viewportChanged', 'fullscreenChanged', 'fullscreenFailed']
   events.forEach(event => app.onEvent?.(event, update))
   if (headerColor && app.setHeaderColor && (app.isVersionAtLeast?.('6.9') ?? true)) {
     app.setHeaderColor(headerColor)
   }
   // Only the root initializes the WebApp. Navigation and StrictMode must not
   // request fullscreen again after a user exits it or a client refuses it.
   if (initialize && initializedApp.current !== app) {
     initializedApp.current = app
     app.ready(); app.expand()
     if (!app.isFullscreen && app.requestFullscreen && app.isVersionAtLeast?.('8.0')) {
       try { app.requestFullscreen() } catch {
         // Some clients expose the method but reject it; expanded mode is ready.
         update()
       }
     }
   }
   if (manageBackButton) {
     if (back) { app.BackButton?.show(); app.BackButton?.onClick(back) } else app.BackButton?.hide()
   }
   return () => {
     events.forEach(event => app.offEvent?.(event, update))
     if (manageBackButton) {
       if (back) app.BackButton?.offClick(back)
       app.BackButton?.hide()
     }
   }
 }, [back, manageBackButton, initialize, headerColor])
}
