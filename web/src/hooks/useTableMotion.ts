import { useEffect, useLayoutEffect, useRef, type RefObject } from 'react'
import type { Card, GameView } from '../api/client'
import type { TableScene } from '../components/scene/TableScene'

export function useTableMotion(root: RefObject<HTMLElement | null>, view: GameView, selected: string, asset: (card: Card, color?: number) => string, dealOnOpen = false) {
 const scene = useRef<TableScene | undefined>(undefined)
 const latest = useRef({ view, selected })
 const baseline = useRef(view)
 const lobby = view.phase === 0
 useLayoutEffect(() => {
  latest.current = { view, selected }
  scene.current?.sync(view, selected)
 })
 useEffect(() => {
  const node = root.current
  // Rendering is exercised in real Chromium; jsdom has no ResizeObserver/GPU.
  if (!node || lobby || typeof ResizeObserver === 'undefined') return
  let cancelled = false
  let owned: TableScene | undefined
  void import('../components/scene/TableScene').then(async ({ TableScene }) => {
   if(cancelled) return
   owned = new TableScene(node, asset)
   await owned.init()
   if(cancelled) return
   scene.current = owned
   owned.sync(latest.current.view, latest.current.selected, baseline.current)
   if(dealOnOpen) owned.dealInitial()
  }).catch(() => {
   if(cancelled) return
   owned?.destroy()
   node.dataset.sceneError = 'true'
   const error = document.createElement('p')
   error.className = 'connection-banner'; error.setAttribute('role', 'alert')
   error.textContent = 'Não foi possível carregar a mesa. Reabra a partida para tentar novamente.'
   node.append(error)
  })
  return () => { cancelled = true; scene.current = undefined; owned?.destroy() }
 }, [root, asset, view.game_id, lobby, dealOnOpen])
}
