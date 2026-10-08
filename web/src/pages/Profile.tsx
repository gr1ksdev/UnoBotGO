import {
 useInfiniteQuery,
 useMutation,
 useQuery,
 useQueryClient,
} from '@tanstack/react-query'
import { useSearchParams } from 'react-router'
import {
 apiGet,
 getUserPrivacy,
 setUserPrivacy,
 type PlayerProfile,
 type PositionResponse,
 type System,
} from '../api/client'
import { Avatar } from '../components/Ranking'
import {
 Brand,
 BottomNav,
 Empty,
 ErrorState,
 Icon,
 Loading,
 PolicyPicker,
} from '../components/Mobile'
import { formatScore } from '../lib/score'
import { useTelegram } from '../lib/telegram'
export function ProfilePage() {
 const [params, setParams] = useSearchParams()
 const system: System = params.get('system') === 'legacy' ? 'legacy' : 'updated'
 useTelegram(undefined, { headerColor: '#10292F' })
 const client = useQueryClient()
 const privacy = useQuery({
  queryKey: ['me', 'privacy'],
  queryFn: ({ signal }) => getUserPrivacy(signal),
 })
 const profile = useInfiniteQuery({
  queryKey: ['profile'],
  initialPageParam: '',
  queryFn: ({ signal, pageParam }) =>
   apiGet<PlayerProfile>(
    `me${pageParam ? `?cursor=${encodeURIComponent(pageParam)}` : ''}`,
    signal,
   ),
  getNextPageParam: (page) => page.next_cursor || undefined,
 })
 const mine = useQuery({
  queryKey: ['position', system],
  queryFn: ({ signal }) =>
   apiGet<PositionResponse>(`me/position?system=${system}`, signal),
 })
 const mutation = useMutation({
  mutationFn: setUserPrivacy,
  onSuccess: (data) => {
   client.setQueryData(['me', 'privacy'], data)
   void client.invalidateQueries({ queryKey: ['rankings'] })
   void client.invalidateQueries({ queryKey: ['position'] })
  },
 })
 const data = profile.data?.pages[0]
 const user = window.Telegram?.WebApp.initDataUnsafe?.user
 const name = privacy.data?.anonymous
  ? 'Anônimo'
  : data?.identity?.name ||
    [user?.first_name, user?.last_name].filter(Boolean).join(' ') ||
    user?.username ||
    'Jogador'
 const stats = data?.stats.find((s) => s.system === system)
 const games = data?.stats.reduce((n, s) => n + s.games, 0) ?? 0
 const wins = data?.stats.reduce((n, s) => n + s.wins, 0) ?? 0
 const avatar = data?.identity ?? mine.data?.entry
 return (
  <main className="page">
   <div className="app-top">
    <Brand />
    <PolicyPicker system={system} change={(s) => setParams({ system: s })} />
   </div>
   <div className="profile-head">
    <Avatar
     item={{
      position: 0,
      key: avatar?.key ?? 'me',
      name,
      score_units: '0',
      masked_id: '',
      avatar_url: avatar?.avatar_url ?? '',
      anonymous: privacy.data?.anonymous,
     }}
     large
    />
    <h1 title={name}>{name}</h1>
    <p className="sub">
     {!privacy.data?.anonymous && user?.username ? `@${user.username} · ` : ''}
     seu perfil de jogador
    </p>
    <span className="pill">
     <Icon name="cards" />
     Inline + Mini App
    </span>
   </div>
   {profile.isPending ? (
    <Loading label="Carregando perfil" />
   ) : profile.isError && !data ? (
    <ErrorState retry={() => void profile.refetch()} />
   ) : (
    <>
     <div className="summary-card">
      <div>
       <span className="sub">
        Pontuação total · {system === 'updated' ? 'Atualizado' : 'Legado'}
       </span>
       <strong className="big">
        {formatScore(stats?.score_units ?? '0', system)}
       </strong>
      </div>
      <div className="position">
       {mine.data?.entry ? `#${mine.data.entry.position}` : '—'}
       <small>
        {mine.data?.month_name} {mine.data?.month_start?.slice(0, 4)} · mensal
       </small>
      </div>
     </div>
     <div className="stats">
      <div>
       <strong>{games}</strong>
       <small>partidas elegíveis</small>
      </div>
      <div>
       <strong>{wins}</strong>
       <small>vitórias</small>
      </div>
      <div>
       <strong>{games ? `${Math.round((wins * 100) / games)}%` : '—'}</strong>
       <small>aproveitamento</small>
      </div>
     </div>
     <div className="rank-title">
      <h3>Últimas partidas</h3>
      <span>seu histórico</span>
     </div>
     <div className="list history">
      {profile.data?.pages
       .flatMap((page) => page.history)
       .map((h) => (
        <div className="row history-row" key={h.id}>
         <span className="avatar">
          <Icon name="cards" />
         </span>
         <div className="who">
          <strong title={h.group}>{h.group}</strong>
          <small>
           {new Date(h.finished_at).toLocaleDateString('pt-BR')}
           <span className="chip">
            {h.origin === 'webapp' ? 'Mini App' : 'Inline'}
           </span>
          </small>
         </div>
         <div className="points">
          {h.score_units != null
           ? `+${formatScore(h.score_units, h.system)}`
           : 'Sem pontos'}
          <small>
           {h.position ? `${h.position}º lugar` : 'Fora do ranking'} ·{' '}
           {h.system === 'updated' ? 'Atualizado' : 'Legado'}
          </small>
         </div>
        </div>
       ))}
     </div>
     {data?.history.length === 0 && (
      <Empty title="Sua história começa na mesa" />
     )}
     {profile.hasNextPage && (
      <button
       className="outline load-more"
       disabled={profile.isFetchingNextPage}
       onClick={() => void profile.fetchNextPage()}
      >
       Carregar mais
      </button>
     )}
    </>
   )}
   <section className="notice privacy-toggle-row">
    <div>
     <strong id="anon-label">Aparecer como Anônimo</strong>
     <p>Seu nome e foto ficam ocultos no ranking público.</p>
    </div>
    <button
     role="switch"
     aria-labelledby="anon-label"
     aria-checked={privacy.data?.anonymous ?? false}
     disabled={!privacy.data || mutation.isPending}
     onClick={() => mutation.mutate(!privacy.data?.anonymous)}
    >
     {privacy.data?.anonymous ? 'Ativado' : 'Desativado'}
    </button>
   </section>
   {privacy.isError && (
    <p role="alert">Não foi possível carregar a privacidade.</p>
   )}
   {mutation.isSuccess && <p role="status">Salvo com sucesso</p>}
   {mutation.isError && (
    <p role="alert">Não foi possível salvar a alteração.</p>
   )}
   <BottomNav />
  </main>
 )
}
