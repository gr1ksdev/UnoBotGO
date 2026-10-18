import { useEffect, useRef } from 'react'
export interface TelegramUser {
 id?: number
 first_name?: string
 last_name?: string
 username?: string
 photo_url?: string
}

interface Insets { top: number; bottom: number; left: number; right: number }
interface TelegramApp {
 initData: string
 platform?: string
 initDataUnsafe?: {
  start_param?: string
  user?: TelegramUser
 }
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
 SettingsButton?: { show(): void; hide(): void; onClick(fn: () => void): void; offClick(fn: () => void): void }
 HapticFeedback?: { impactOccurred(style: 'light' | 'medium' | 'heavy' | 'rigid' | 'soft'): void }
}
declare global { interface Window { Telegram?: { WebApp: TelegramApp } } }

export function telegramSettingsAvailable(app = window.Telegram?.WebApp): boolean {
 const button = app?.SettingsButton
 return Boolean(button && typeof button.show === 'function' && typeof button.hide === 'function' && typeof button.onClick === 'function' && typeof button.offClick === 'function' && (app?.isVersionAtLeast?.('7.0') ?? true))
}

export function useTelegram(back?: () => void, options: { manageBackButton?: boolean; initialize?: boolean; headerColor?: string; settings?: () => void } = {}) {
 const manageBackButton = options.manageBackButton ?? true
 const initialize = options.initialize ?? false
 const headerColor = options.headerColor
 const settings = options.settings
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
     const mobile = app.platform === 'android' || app.platform === 'ios'
     if (mobile && !app.isFullscreen && app.requestFullscreen && app.isVersionAtLeast?.('8.0')) {
       try { app.requestFullscreen() } catch {
         // Some clients expose the method but reject it; expanded mode is ready.
         update()
       }
     }
   }
   if (manageBackButton) {
     if (back) { app.BackButton?.show(); app.BackButton?.onClick(back) } else app.BackButton?.hide()
   }
   const settingsButton = telegramSettingsAvailable(app) ? app.SettingsButton : undefined
   if (settingsButton && settings) { settingsButton.show(); settingsButton.onClick(settings) }
   return () => {
     events.forEach(event => app.offEvent?.(event, update))
     if (manageBackButton) {
       if (back) app.BackButton?.offClick(back)
       app.BackButton?.hide()
     }
    if (settingsButton) {
     if (settings) settingsButton.offClick(settings)
     settingsButton.hide()
    }
   }
 }, [back, manageBackButton, initialize, headerColor, settings])
}

// The launch parameter selects a route only. Room membership is verified by Go.
export function launchGamePath(): string | undefined {
 const value = window.Telegram?.WebApp.initDataUnsafe?.start_param
 if (!value) return
 if (value.startsWith('join_') && /^[a-zA-Z0-9_-]{1,480}$/.test(value.slice(5))) return `/invite/${value.slice(5)}`
 if (!value.startsWith('game_')) return
 const id = value.slice(5)
 if (!/^[a-zA-Z0-9_-]{1,128}$/.test(id)) return
 return `/game/${encodeURIComponent(id)}`
}
