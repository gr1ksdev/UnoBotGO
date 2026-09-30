import { useCallback, useEffect, useRef } from 'react'
import { Link, useNavigate, useParams, useSearchParams } from 'react-router'
import { APIError, type System } from '../api/client'
import { Arrow, Avatar, Calendar, ErrorState, RankingCard, Score, Segmented, Skeleton } from '../components/Ranking'
import { useRanking } from '../hooks/useRanking'
import { useTelegram } from '../lib/telegram'

function More({ hasNext, busy, load }: { hasNext: boolean; busy: boolean; load: () => void }) {
 const ref = useRef<HTMLButtonElement>(null)
 useEffect(() => {
   if (!ref.current || !hasNext || busy || !('IntersectionObserver' in window)) return
   const observer = new IntersectionObserver(entries => { if (entries.some(e => e.isIntersecting)) load() }, { rootMargin: '120px' })
   observer.observe(ref.current); return () => observer.disconnect()
 }, [hasNext, busy, load])
 return hasNext ? <button className="load-more" ref={ref} disabled={busy} onClick={load}>{busy ? 'Carregando mais…' : 'Carregar mais'}</button> : null
}

export function RankingsPage() {
 const [params, setParams] = useSearchParams()
 const system: System = params.get('system') === 'legacy' ? 'legacy' : 'updated'
 const tab = params.get('tab') === 'players' ? 'players' : 'groups'
 const { groupRef } = useParams()
 const detail = !!groupRef
 const navigate = useNavigate()
 const back = useCallback(() => navigate(`/?system=${system}&tab=${tab}`), [navigate, system, tab])
 useTelegram(detail ? back : undefined)
 const query = useRanking(detail ? `groups/${encodeURIComponent(groupRef)}` : tab, system)
 const { fetchNextPage } = query
 const load = useCallback(() => { void fetchNextPage() }, [fetchNextPage])
 const unauthorized = query.error instanceof APIError && query.error.status === 401
 const reset = () => { void query.refetch() }
 const month = query.first?.month_name
 const group = query.first?.group
 return <main className={`app-shell ${detail ? 'detail-view' : ''}`}>
   <header className={`ranking-header ${detail ? 'detail-header' : ''}`}>
     <div className="title-bar">
       <button className="back-button" aria-label={detail ? 'Voltar ao Ranking Global' : 'Fechar Ranking Global'} onClick={detail ? back : () => window.Telegram?.WebApp.close?.()}><Arrow /></button>
       <h1>{detail ? 'Ranking do grupo' : <>Ranking Global{month && ` · ${month}`}</>}</h1>
       {!detail && <span className="calendar-icon"><Calendar /></span>}
     </div>
     {!detail && <Segmented label="Sistema de ranking" className="system-switch" value={system} options={[{ value: 'updated', label: 'Atualizado' }, { value: 'legacy', label: 'Legado' }]} onChange={value => setParams({ system: value, tab })} />}
     {detail && group && <section className="group-hero" aria-label="Resumo do grupo"><Avatar item={group} large /><div className="group-summary"><h2>{group.name}</h2><p className="hero-id">{group.masked_id}</p><Score item={group} system={system} /><p className="hero-caption">Total do grupo no mês de {month}</p></div><span className="uno-cards" aria-hidden="true"><i/><i/></span></section>}
     {detail && !group && query.isPending && <div className="hero-placeholder" />}
   </header>
   <section className="ranking-panel" aria-label={detail ? 'Ranking interno do grupo' : 'Ranking global'}>
     {detail ? <h2 className="section-title"><span aria-hidden="true">♟</span> Ranking interno do grupo{month && ` · ${month}`}</h2> : <Segmented label="Tipo de ranking" className="tab-switch" value={tab} options={[{ value: 'groups', label: 'Grupos' }, { value: 'players', label: 'Players' }]} onChange={value => setParams({ system, tab: value })} />}
     {query.isPending ? <Skeleton /> : query.isError && !query.first ? <><ErrorState unauthorized={unauthorized} retry={reset} />{detail && <Link className="load-more" to={`/?system=${system}&tab=${tab}`}>Voltar ao Ranking Global</Link>}</> : <>
       {query.items.length === 0 ? <p className="state-card">{detail ? 'Nenhum jogador pontuou neste grupo neste mês.' : tab === 'groups' ? 'Nenhum grupo pontuou neste mês.' : 'Nenhum jogador pontuou neste mês.'}</p> : <ol className="ranking-list">{query.items.map(item => <RankingCard key={item.key} item={item} system={system} tab={tab} />)}</ol>}
       {query.isFetchNextPageError ? <ErrorState retry={load} unauthorized={unauthorized} /> : <More hasNext={query.hasNextPage} busy={query.isFetchingNextPage} load={load} />}
     </>}
   </section>
 </main>
}
