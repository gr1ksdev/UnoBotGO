import { useQuery } from '@tanstack/react-query'
import { useState } from 'react'
import { apiGet, type Seat } from '../api/client'
import { Icon } from './Mobile'

export function ShareRoom({ gameID, players, capacity = 4 }: { gameID: string; players?: Seat[]; capacity?: number }) {
 const [copied, setCopied] = useState(false)
 const [show, setShow] = useState(false)
 const invite = useQuery({ queryKey: ['room-invite', gameID], queryFn: ({ signal }) => apiGet<{ url: string; token: string }>(`rooms/${gameID}/invite`, signal), staleTime: 3600000 })

 const activePlayers = (players ?? []).filter(p => p.active)
 const count = activePlayers.length
 const max = capacity
 const playerLines = activePlayers.length > 0 ? activePlayers.map(p => `• ${p.name.trim() || 'Jogador'}`).join('\n') : '• Aguardando jogadores'
 const textDescription = `🎮 Mesa de UNO no UnoBotGO!\n\nJogadores na sala (${count}/${max}):\n${playerLines}\n\nEntre pelo link:`

 const share = async () => {
  if (!invite.data) return
  setShow(true)
  const fullText = `${textDescription}\n${invite.data.url}`
  try {
   if (window.Telegram?.WebApp && typeof (window.Telegram.WebApp as unknown as { openTelegramLink?: (url: string) => void }).openTelegramLink === 'function') {
    const shareUrl = `https://t.me/share/url?url=${encodeURIComponent(invite.data.url)}&text=${encodeURIComponent(`🎮 Mesa de UNO no UnoBotGO!\n\nJogadores na sala (${count}/${max}):\n${playerLines}`)}`
    ;(window.Telegram.WebApp as unknown as { openTelegramLink: (url: string) => void }).openTelegramLink(shareUrl)
    return
   }
   if (navigator.share) {
    await navigator.share({ title: 'Mesa de UNO no UnoBotGO', text: `🎮 Mesa de UNO no UnoBotGO!\n\nJogadores na sala (${count}/${max}):\n${playerLines}`, url: invite.data.url })
   } else {
    await navigator.clipboard.writeText(fullText)
    setCopied(true)
   }
  } catch {
   try {
    await navigator.clipboard.writeText(fullText)
    setCopied(true)
   } catch {
    /* Keep the selectable URL visible if sharing is dismissed or unavailable. */
   }
  }
 }
 return <div className="share-room">
  <button className="outline" disabled={!invite.data} onClick={() => void share()}><Icon name="chat" />{copied ? 'Convite copiado' : 'Compartilhar convite'}</button>
  {show && invite.data && <label>Convite da sala<input aria-label="Convite da sala" readOnly value={invite.data.url} onFocus={e => e.target.select()} /></label>}
  {invite.isError && <p role="alert">Não foi possível obter o convite.</p>}
 </div>
}
