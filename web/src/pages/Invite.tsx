import { useRef } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { Link, useNavigate, useParams } from 'react-router'
import { apiGet, apiPost, type GameView, type InvitePreview } from '../api/client'
import { Brand, Icon } from '../components/Mobile'
import { useTelegram } from '../lib/telegram'
export function InvitePage() {
 const { token = '' } = useParams()
 const navigate = useNavigate()
 useTelegram(() => navigate('/play'), { headerColor: '#10292F' })
 const client = useQueryClient()
 const request = useRef(crypto.randomUUID())
 const preview = useQuery({ queryKey: ['invite', token], queryFn: ({ signal }) => apiGet<InvitePreview>(`invites/${encodeURIComponent(token)}`, signal), retry: false })
 const join = useMutation({ mutationFn: () => apiPost<GameView>(`invites/${encodeURIComponent(token)}/join`, { request_id: request.current, expected_revision: preview.data?.revision }),
  onSuccess: view => { void client.invalidateQueries({ queryKey: ['rooms'] }); navigate(`/game/${view.game_id}`, { replace: true }) },
  onError: () => { request.current = crypto.randomUUID(); void preview.refetch() },
 })
 const view = preview.data
 const responsible = view?.players.find(p => p.key === view.owner_key)
 const responsibleName = responsible?.name.trim() || 'Responsável da sala'
 return <main className="page invite-page">
  <header className="invite-header"><Brand /><Link to="/play">Início</Link></header>
  {preview.isPending ? <section className="invite-card invite-loading"><span className="eyebrow">ABRINDO CONVITE</span><span className="invite-spinner" aria-hidden="true" /><p role="status">Conferindo se a sala ainda aceita jogadores…</p></section> : view ? <section className="invite-card" aria-labelledby="invite-title">
   <div className="invite-mark"><Icon name="cards" /></div>
   <span className="eyebrow">CONVITE PARA JOGAR</span>
   <h1 id="invite-title">Você foi convidado</h1>
   <p className="invite-intro">Convite enviado por</p>
   <div className="invite-responsible" aria-label={`Responsável da sala: ${responsibleName}`}>
    <span className="avatar">{responsibleName.trim().slice(0, 1).toUpperCase() || 'J'}</span>
    <span><small>Responsável da sala</small><strong>{responsibleName}</strong></span>
   </div>
   {join.isError && <p role="alert" className="invite-notice">A entrada não foi concluída. A sala pode ter mudado; confira a lista e tente novamente.</p>}
   {!view.can_join && !view.joined && <p role="status" className="invite-notice">Esta sala está cheia ou não aceita mais participantes.</p>}
   {view.joined ? <Link className="primary invite-cta" to={`/game/${view.game_id}`}>Abrir sala <Icon name="arrow" /></Link> : <button className="primary invite-cta" disabled={!view.can_join || join.isPending} onClick={() => join.mutate()}>{join.isPending ? 'Entrando na sala…' : 'Entrar na sala'}<Icon name="arrow" /></button>}
   <p className="invite-footnote">Você poderá ver os outros jogadores no lobby.</p>
  </section> : <section className="invite-card invite-unavailable" aria-labelledby="invite-error-title">
   <span className="eyebrow">CONVITE INDISPONÍVEL</span><h1 id="invite-error-title">Não foi possível abrir esta sala</h1>
   <p role="alert">O convite pode ter expirado ou a sala não aceita mais participantes.</p>
   <Link className="primary invite-cta" to="/play">Voltar ao início <Icon name="arrow" /></Link>
  </section>}
 </main>
}
