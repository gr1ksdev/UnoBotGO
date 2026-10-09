import { useRef } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { Link, useNavigate, useParams } from 'react-router'
import { apiGet, apiPost, type GameView, type InvitePreview } from '../api/client'
import { Brand, Icon, Loading } from '../components/Mobile'
import { useTelegram } from '../lib/telegram'
export function InvitePage() {
 const { token = '' } = useParams()
 const navigate = useNavigate()
 useTelegram(() => navigate('/home'), { headerColor: '#10292F' })
 const client = useQueryClient()
 const request = useRef(crypto.randomUUID())
 const preview = useQuery({ queryKey: ['invite', token], queryFn: ({ signal }) => apiGet<InvitePreview>(`invites/${encodeURIComponent(token)}`, signal), retry: false })
 const join = useMutation({ mutationFn: () => apiPost<GameView>(`invites/${encodeURIComponent(token)}/join`, { request_id: request.current, expected_revision: preview.data?.revision }),
  onSuccess: view => { void client.invalidateQueries({ queryKey: ['rooms'] }); navigate(`/game/${view.game_id}`, { replace: true }) },
  onError: () => { request.current = crypto.randomUUID(); void preview.refetch() },
 })
 const view = preview.data
 return <main className="page invite-page"><Brand />
  <span className="eyebrow">VOCÊ FOI CONVIDADO</span>
  {preview.isPending ? <Loading /> : view ? <>
   <h1>{view.group}</h1><p className="sub">{view.mode === 'classic' ? 'Clássico' : 'Caseiro'} · {view.players.filter(p => p.active).length}/{view.capacity} jogadores</p>
   <div className="lobby-seats">{view.players.filter(p => p.active).map(p => <div className="player" key={p.key}><span className="avatar">{p.name.slice(0, 1)}</span><strong>{p.name}</strong><small>{p.key === view.owner_key ? 'Responsável' : 'Jogador'}</small></div>)}</div>
   {view.joined ? <Link className="primary" to={`/game/${view.game_id}`}>Abrir partida <Icon name="arrow" /></Link> : <button className="primary" disabled={!view.can_join || join.isPending} onClick={() => join.mutate()}>{join.isPending ? 'Entrando…' : 'Entrar na sala'}<Icon name="arrow" /></button>}
   {!view.can_join && !view.joined && <p role="status">Esta sala está fechada ou completa.</p>}
   {join.isError && <p role="alert">A sala mudou ou a entrada foi recusada. Confira os jogadores e tente novamente.</p>}
  </> : <p role="alert" className="notice">Convite indisponível. Ele pode ter expirado ou você não participa do grupo vinculado.</p>}
  <Link className="outline load-more" to="/home">Voltar ao início</Link>
 </main>
}
