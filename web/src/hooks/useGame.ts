import { useQueryClient } from '@tanstack/react-query'
import { useCallback, useEffect, useRef, useState } from 'react'
import { apiGet, type GameCommand, type GameView } from '../api/client'
export interface LiveMessage {
 type: string
 request_id?: string
 reason?: string
 view: GameView
}
export function acceptSnapshot(
 previous: GameView | undefined,
 incoming: GameView,
): GameView {
 if (
  previous &&
  (previous.game_id !== incoming.game_id ||
   incoming.revision < previous.revision)
 )
  return previous
 return incoming
}
export function useGame(gameID: string) {
 const client = useQueryClient()
 const [view, setView] = useState<GameView>()
 const [connected, setConnected] = useState(false)
 const [pending, setPending] = useState(false)
 const [error, setError] = useState('')
 const socket = useRef<WebSocket>(null)
 const command = useRef<GameCommand>(null)
 useEffect(() => {
  let disposed = false
  let retry: ReturnType<typeof setTimeout>
  let attempt = 0
  let ws: WebSocket
  let committed = false
  const recover = async () => {
   try {
    const data = await apiGet<GameView>(`rooms/${encodeURIComponent(gameID)}`)
    if (!disposed) setView((previous) => acceptSnapshot(previous, data))
   } catch {
    /* The socket reports connection/authentication failures. */
   }
  }
  const open = () => {
   if (disposed) return
   ws = new WebSocket(
    `${location.protocol === 'https:' ? 'wss:' : 'ws:'}//${location.host}/api/v1/live`,
   )
   socket.current = ws
   ws.onopen = () =>
    ws.send(
     JSON.stringify({
      init_data: window.Telegram?.WebApp.initData ?? '',
      game_id: gameID,
     }),
    )
   ws.onmessage = (event) => {
    const message = JSON.parse(String(event.data)) as LiveMessage
    if (message.view?.game_id !== gameID) return
    setView((previous) => acceptSnapshot(previous, message.view))
    setConnected(true)
    attempt = 0
    if (message.view.closed && message.view.result && !committed) {
     committed = true
     for (const key of ['rankings', 'position', 'profile', 'rooms'])
      void client.invalidateQueries({ queryKey: [key] })
    }
    if (
     message.request_id &&
     message.request_id === command.current?.request_id
    ) {
     command.current = null
     setPending(false)
     setError(
      message.type === 'rejected'
       ? message.reason === 'stale_revision'
         ? 'A mesa mudou. Confira sua mão e tente novamente.'
         : 'O servidor recusou esta ação. Confira o turno e as regras.'
       : '',
     )
    } else if (command.current && message.type === 'snapshot') {
     // An interrupted request is retried verbatim, including its action ID/revision.
     ws.send(JSON.stringify(command.current))
    }
   }
   ws.onclose = (event) => {
    if (disposed) return
    setConnected(false)
    if (event.code === 1008) {
     setError(
      'Acesso recusado. Abra novamente pelo Telegram e confira sua participação na sala.',
     )
     return
    }
    void recover()
    retry = setTimeout(open, Math.min(15000, 500 * 2 ** attempt++))
   }
   ws.onerror = () => ws.close()
  }
  void recover()
  open()
  return () => {
   disposed = true
   clearTimeout(retry)
   ws?.close()
   socket.current = null
   command.current = null
  }
 }, [gameID, client])
 const send = useCallback(
  (action: string, extra: Partial<GameCommand> = {}) => {
   if (
    !view ||
    view.closed ||
    !connected ||
    command.current ||
    socket.current?.readyState !== WebSocket.OPEN
   )
    return
   const next: GameCommand = {
    ...extra,
    game_id: gameID,
    request_id: crypto.randomUUID(),
    expected_revision: view.revision,
    action,
   }
   command.current = next
   setPending(true)
   setError('')
   socket.current.send(JSON.stringify(next))
  },
  [view, connected, gameID],
 )
 return { view, connected, pending, error, send }
}
