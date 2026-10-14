import { useCallback, useEffect, useRef } from 'react'
import { useMutation } from '@tanstack/react-query'
import { apiPost, type GameView } from '../api/client'
export function CreateRoom({ close, created }: { close: () => void; created: (game: GameView) => void }) {
 const requestID = useRef('')
 if (!requestID.current) requestID.current = crypto.randomUUID()
 const started = useRef(false)
 const { mutate, isPending, isError } = useMutation({
  mutationFn: () => apiPost<GameView>('rooms', { room_type: 'free', system: 'updated', mode: 'caseiro', request_id: requestID.current }),
  onSuccess: created,
 })
 const start = useCallback(() => mutate(), [mutate])
 useEffect(() => {
  if (started.current) return
  started.current = true
  start()
 }, [start])
 return <section className="room-create-status" aria-live="polite">
  {isPending && <p role="status">Criando sua sala livre · Caseiro · ranking Atualizado…</p>}
  {isError && <div role="alert"><p>Não foi possível criar a sala.</p><button className="outline" onClick={start}>Tentar novamente</button><button className="outline" onClick={close}>Voltar</button></div>}
 </section>
}
