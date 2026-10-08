import { useQuery } from '@tanstack/react-query'
import { Link } from 'react-router'
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
 useTelegram(undefined, { headerColor: '#10292F' })
 const rooms = useQuery({
  queryKey: ['rooms'],
  queryFn: ({ signal }) => apiGet<Room[]>('rooms', signal),
 })
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
     <a
      className="primary"
      href={
       config.data?.bot_username
        ? `https://t.me/${config.data.bot_username}`
        : undefined
      }
     >
      Começar pelo bot <Icon name="arrow" />
     </a>
    )}
   </section>
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
   {rooms.data?.length === 0 && (
    <p className="notice">
     Crie uma sala com /novo e entre com /entrar no grupo. A partida aparecerá
     aqui para os participantes.{' '}
     <button onClick={() => void rooms.refetch()}>Atualizar salas</button>
    </p>
   )}
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
   {rooms.data && rooms.data.length > 1 && (
    <div className="list">
     {rooms.data.map((room) => (
      <Link
       className="row room-link"
       key={room.game_id}
       to={`/game/${room.game_id}`}
      >
       <Icon name="cards" />
       <strong>{room.group || 'Partida'}</strong>
       <Icon name="arrow" />
      </Link>
     ))}
    </div>
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
  </main>
 )
}
