import { useCallback } from 'react'
import { useQuery } from '@tanstack/react-query'
import { useNavigate, useParams, useSearchParams } from 'react-router'
import {
 APIError,
 apiGet,
 type PositionResponse,
 type System,
} from '../api/client'
import {
 Brand,
 BottomNav,
 Empty,
 ErrorState,
 Loading,
 MyPosition,
 Podium,
 PolicyPicker,
 RankingRow,
 RankingTabs,
} from '../components/Mobile'
import { useRanking } from '../hooks/useRanking'
import { useTelegram } from '../lib/telegram'
export function RankingsPage() {
 const [params, setParams] = useSearchParams()
 const system: System = params.get('system') === 'legacy' ? 'legacy' : 'updated'
 const tab = params.get('tab') === 'groups' ? 'groups' : 'players'
 const { groupRef } = useParams()
 const navigate = useNavigate()
 const back = useCallback(
  () => navigate(`/ranking?system=${system}&tab=groups`),
  [navigate, system],
 )
 useTelegram(groupRef ? back : undefined, { headerColor: '#10292F' })
 const query = useRanking(
  groupRef ? `groups/${encodeURIComponent(groupRef)}` : tab,
  system,
 )
 const mine = useQuery({
  queryKey: ['position', system],
  queryFn: ({ signal }) =>
   apiGet<PositionResponse>(`me/position?system=${system}`, signal),
  enabled: !groupRef && tab === 'players',
 })
 const change = (s: System) => setParams({ system: s, tab })
 const rows = query.items.length >= 3 ? query.items.slice(3) : []
 return (
  <main className="page">
   <div className="app-top">
    <Brand />
    <PolicyPicker system={system} change={change} />
   </div>
   <header className="heading">
    <span className="eyebrow">TODO MUNDO NA MESMA DISPUTA</span>
    <h1>{groupRef ? 'Ranking do grupo' : 'Ranking'}</h1>
    <p className="sub">
     Inline e Mini App somam juntos.
     <br />
     Cada partida conta na sua história.
    </p>
   </header>
   {!groupRef && (
    <RankingTabs category={tab} change={(t) => setParams({ system, tab: t })} />
   )}
   <div className="rank-context">
    <span>
     {query.first
      ? `${query.first.month_name} ${query.first.month_start.slice(0, 4)}`
      : 'Ranking mensal'}
    </span>
    <span>{groupRef ? query.first?.group?.name : 'Classificação mensal'}</span>
   </div>
   {query.isPending ? (
    <Loading />
   ) : query.isError && !query.first ? (
    <ErrorState
     retry={() => void query.refetch()}
     unauthorized={
      query.error instanceof APIError && query.error.status === 401
     }
    />
   ) : query.items.length === 0 ? (
    <Empty title="Ainda sem pontuação">
     <p className="sub">Nenhuma partida elegível neste mês.</p>
    </Empty>
   ) : (
    <>
     <Podium items={query.items.slice(0, 3)} system={system} />
     {rows.length > 0 && (
      <>
       <div className="rank-title">
        <h3>{tab === 'groups' ? 'Grupos na disputa' : 'Na cola do pódio'}</h3>
        <span>pontuação mensal</span>
       </div>
       <div className="list">
        {rows.map((item) => (
         <RankingRow key={item.key} item={item} system={system} />
        ))}
       </div>
      </>
     )}
     {query.isFetchNextPageError ? (
      <ErrorState retry={() => void query.fetchNextPage()} />
     ) : (
      query.hasNextPage && (
       <button
        className="outline load-more"
        disabled={query.isFetchingNextPage}
        onClick={() => void query.fetchNextPage()}
       >
        {query.isFetchingNextPage ? 'Carregando mais…' : 'Carregar mais'}
       </button>
      )
     )}
    </>
   )}
   {!groupRef &&
    tab === 'players' &&
    mine.data &&
    mine.data.month_start === query.first?.month_start && (
     <MyPosition entry={mine.data.entry} system={system} />
    )}
   {!groupRef && <BottomNav />}
  </main>
 )
}

export { RankingsPage as RankingScreen }
