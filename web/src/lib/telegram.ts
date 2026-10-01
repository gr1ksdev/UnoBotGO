import { useEffect } from 'react'

interface Insets { top: number; bottom: number; left: number; right: number }
interface TelegramApp {
 initData: string
 ready(): void
 expand(): void
 close?(): void
 safeAreaInset?: Insets
 contentSafeAreaInset?: Insets
 viewportStableHeight?: number
 onEvent?(event: string, fn: () => void): void
 offEvent?(event: string, fn: () => void): void
 BackButton?: { show(): void; hide(): void; onClick(fn: () => void): void; offClick(fn: () => void): void }
}
declare global { interface Window { Telegram?: { WebApp: TelegramApp } } }

export function useTelegram(back?: () => void, options: { manageBackButton?: boolean } = {}) {
 const manageBackButton = options.manageBackButton ?? true
 useEffect(() => {
   const app = window.Telegram?.WebApp
   if (!app) return
   app.ready(); app.expand()
   const update = () => {
     for (const side of ['top', 'bottom', 'left', 'right'] as const) {
       const value = (app.safeAreaInset?.[side] ?? 0) + (app.contentSafeAreaInset?.[side] ?? 0)
       document.documentElement.style.setProperty(`--telegram-${side}`, `${value}px`)
     }
     if (app.viewportStableHeight) document.documentElement.style.setProperty('--viewport-height', `${app.viewportStableHeight}px`)
   }
   update()
   const events = ['safeAreaChanged', 'contentSafeAreaChanged', 'viewportChanged']
   events.forEach(event => app.onEvent?.(event, update))
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
 }, [back, manageBackButton])
}
