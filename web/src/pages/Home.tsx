import { useEffect, useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import { Link, useNavigate } from 'react-router'
import { CreateRoom } from '../components/CreateRoom'
import {
 apiGet,
 type PlayerProfile,
 type PositionResponse,
 type Room,
} from '../api/client'
import { Brand, BottomNav, Icon, RankingRow } from '../components/Mobile'
import { useRanking } from '../hooks/useRanking'
import { useTelegram } from '../lib/telegram'
export function HomePage() {
 const [creating, setCreating] = useState(false)
 const navigate = useNavigate()
 useTelegram(undefined, { headerColor: '#10292F' })
 const rooms = useQuery({
  queryKey: ['rooms'],
  queryFn: ({ signal }) => apiGet<Room[]>('rooms', signal),
 })
 const refreshRooms = rooms.refetch
 useEffect(() => {
  const refresh = () => { void refreshRooms() }
  const visible = () => { if (document.visibilityState === 'visible') refresh() }
  const app = window.Telegram?.WebApp
  app?.onEvent?.('activated', refresh)
  window.addEventListener('focus', refresh)
  document.addEventListener('visibilitychange', visible)
  return () => {
   app?.offEvent?.('activated', refresh)
   window.removeEventListener('focus', refresh)
   document.removeEventListener('visibilitychange', visible)
  }
 }, [refreshRooms])
 const profile = useQuery({
  queryKey: ['profile', ''],
  queryFn: ({ signal }) => apiGet<PlayerProfile>('me', signal),
 })
 const config = useQuery({
  queryKey: ['config'],
  queryFn: ({ signal }) => apiGet<{ bot_username: string }>('config', signal),
 })
 const ranking = useRanking('players', 'updated')
 const mine = useQuery({
  queryKey: ['position', 'updated'],
  queryFn: ({ signal }) =>
   apiGet<PositionResponse>('me/position?system=updated', signal),
 })
 const games = profile.data?.stats.reduce((n, s) => n + s.games, 0)
 const wins = profile.data?.stats.reduce((n, s) => n + s.wins, 0)
 return (
  <main className="page">
   <Brand />
   <section className="welcome">
    <span className="eyebrow">UMA CONTA. TODO O JOGO.</span>
    <h1>
     Bora pra
     <br />
     próxima
     <br />
     partida?
    </h1>
    <p className="sub">
     Jogue na Mini App ou no Telegram.
     <br />A sua pontuação segue com você.
    </p>
   </section>
   <section className="hero">
    <div className="hero-art" aria-hidden="true">
     <img src="/assets/cards/red-7.svg" />
     <img src="/assets/cards/blue-reverse.svg" />
     <img src="/assets/cards/yellow-2.svg" />
    </div>
    <h2>
     A mesa
     <br />é sua.
    </h2>
    <p className="sub">
     Cartas na mão,
     <br />
     amigos na disputa.
    </p>
    {rooms.data?.[0] ? (
     <Link className="primary" to={`/game/${rooms.data[0].game_id}`}>
      Abrir partida <Icon name="arrow" />
     </Link>
    ) : (
     <button className="primary" onClick={() => setCreating(true)}>Criar sala <Icon name="arrow" /></button>
    )}
   </section>
   {rooms.data?.[0] && <button className="outline load-more" onClick={() => setCreating(true)}>Criar sala</button>}
   {config.data?.bot_username && (
    <a
     className="outline telegram-link"
     href={`https://t.me/${config.data.bot_username}`}
    >
     <Icon name="chat" />
     Jogar pelo Telegram
     <Icon name="arrow" />
    </a>
   )}
   <section id="salas" aria-labelledby="rooms-title" className="rooms-section">
    <div className="section-head">
     <h2 id="rooms-title">Suas salas</h2>
     <button onClick={() => void rooms.refetch()} disabled={rooms.isFetching}>
      {rooms.isFetching ? 'Atualizando…' : 'Atualizar salas'}
     </button>
    </div>
    {rooms.isPending && <p role="status" className="notice">Buscando suas salas…</p>}
    {rooms.isError && <p role="alert" className="notice">
     Não foi possível buscar suas salas. Use Atualizar salas para tentar novamente.
    </p>}
    {rooms.data?.length === 0 && <p className="notice">
     Nenhuma sala disponível. Crie uma sala aqui ou abra um convite para entrar.
     {' '}Você não precisa usar comandos no Telegram.
    </p>}
    <div className="list">
     {rooms.data?.map((room) => (
      <Link className="row room-link" key={room.game_id} to={`/game/${encodeURIComponent(room.game_id)}`}>
       <Icon name="cards" />
       <div><strong>{room.group || 'Partida'}</strong>
        <small>{room.phase === 0 ? 'Aguardando início' : 'Em andamento'}</small>
       </div>
       <span className="room-action">Abrir na Mini App <Icon name="arrow" /></span>
      </Link>
     ))}
    </div>
   </section>
   <div className="stats">
    <div>
     <strong>{games ?? '—'}</strong>
     <small>partidas</small>
    </div>
    <div>
     <strong>{wins ?? '—'}</strong>
     <small>vitórias</small>
    </div>
    <div>
     <strong>{mine.data?.entry ? `#${mine.data.entry.position}` : '—'}</strong>
     <small>posição mensal</small>
    </div>
   </div>
   {(rooms.isError || profile.isError) && (
    <p role="alert" className="notice">
     Não foi possível carregar seus dados.{' '}
     <button
      onClick={() => {
       void rooms.refetch()
       void profile.refetch()
      }}
     >
      Tentar novamente
     </button>
    </p>
   )}
   <div className="section-head">
    <h2>No topo do jogo</h2>
    <Link to="/ranking">Ver ranking</Link>
   </div>
   <p className="sub period">
    {ranking.first &&
     `${ranking.first.month_name} ${ranking.first.month_start.slice(0, 4)} · Atualizado`}
   </p>
   <div className="list">
    {ranking.items.slice(0, 3).map((item) => (
     <RankingRow key={item.key} item={item} system="updated" />
    ))}
   </div>
   <BottomNav />
   {creating && <CreateRoom close={() => setCreating(false)} created={view => navigate(`/game/${view.game_id}`)} />}
  </main>
 )
}
