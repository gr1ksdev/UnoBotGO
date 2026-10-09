import { useQuery } from '@tanstack/react-query'
import { useState } from 'react'
import { apiGet } from '../api/client'
import { Icon } from './Mobile'
export function ShareRoom({ gameID }: { gameID: string }) {
 const [copied, setCopied] = useState(false)
 const [show, setShow] = useState(false)
 const invite = useQuery({ queryKey: ['room-invite', gameID], queryFn: ({ signal }) => apiGet<{ url: string; token: string }>(`rooms/${gameID}/invite`, signal), staleTime: 3600000 })
 const share = async () => {
  if (!invite.data) return
  setShow(true)
  try {
   if (navigator.share) await navigator.share({ title: 'Sua mesa no UnoBotGO', url: invite.data.url })
   else { await navigator.clipboard.writeText(invite.data.url); setCopied(true) }
  } catch { /* Keep the selectable URL visible if sharing is dismissed or unavailable. */ }
 }
 return <div className="share-room">
  <button className="outline" disabled={!invite.data} onClick={() => void share()}><Icon name="chat" />{copied ? 'Convite copiado' : 'Compartilhar convite'}</button>
  {show && invite.data && <label>Convite da sala<input aria-label="Convite da sala" readOnly value={invite.data.url} onFocus={e => e.target.select()} /></label>}
  {invite.isError && <p role="alert">Não foi possível obter o convite.</p>}
 </div>
}
