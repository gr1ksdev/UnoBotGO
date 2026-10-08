import { useCallback, useEffect, useRef, useState } from 'react'
import { Link, useNavigate, useParams } from 'react-router'
import type { Card, GameCommand, GameView, Seat } from '../api/client'
import { Icon } from '../components/Mobile'
import { formatScore } from '../lib/score'
import { useGame } from '../hooks/useGame'
import { useTelegram } from '../lib/telegram'
const colors = ['', 'Vermelho', 'Azul', 'Verde', 'Amarelo']
const colorFiles = ['', 'red', 'blue', 'green', 'yellow']
const ranks = [
 '0',
 '1',
 '2',
 '3',
 '4',
 '5',
 '6',
 '7',
 '8',
 '9',
 'Bloqueio',
 'Inversão',
 '+2',
 'Coringa',
 '+4',
 'Troca de mãos',
]
export function cardAsset(card: Card) {
 if (card.Rank === 15) return '/assets/cards/swap.svg'
 if (card.Rank >= 13)
  return `/assets/cards/${card.Rank === 14 ? 'wild-draw4' : 'wild'}.svg`
 return `/assets/cards/${colorFiles[card.Color]}-${card.Rank < 10 ? card.Rank : ['skip', 'reverse', 'draw2'][card.Rank - 10]}.svg`
}
export function cardLabel(card: Card) {
 return `${colors[card.Color]} ${ranks[card.Rank]}`.trim()
}
function PlayerBadge({ player }: { player: Seat }) {
 return (
  <div className={`seat ${player.current ? 'current' : ''}`}>
   <span className="avatar">
    {player.name.trim().slice(0, 1).toUpperCase() || 'J'}
   </span>
   <div className="player-line">
    <strong title={player.name}>{player.me ? 'Você' : player.name}</strong>
    <span className="count">
     {player.count}
     <Icon name="cards" />
    </span>
   </div>
  </div>
 )
}
function Timer({ view }: { view: GameView }) {
 const [now, setNow] = useState(() => Date.now())
 const received = useRef({ server: view.server_time, client: Date.now() })
 useEffect(() => {
  received.current = { server: view.server_time, client: Date.now() }
 }, [view.server_time])
 useEffect(() => {
  const interval = setInterval(() => setNow(Date.now()), 1000)
  return () => clearInterval(interval)
 }, [])
 if (!view.deadline || view.closed) return null
 const remaining = Math.max(
  0,
  Math.ceil(
   (Date.parse(view.deadline) -
    Date.parse(received.current.server) -
    (now - received.current.client)) /
    1000,
  ),
 )
 return (
  <span className="timer" aria-label="Tempo restante do turno">
   {Math.floor(remaining / 60)
    .toString()
    .padStart(2, '0')}
   :{(remaining % 60).toString().padStart(2, '0')}
  </span>
 )
}
function Choice({
 view,
 disabled,
 send,
}: {
 view: GameView
 disabled: boolean
 send: (action: string, extra?: Partial<GameCommand>) => void
}) {
 const dialog = useRef<HTMLDialogElement>(null)
 const choice = view.my_turn && [2, 4].includes(view.phase) && !view.closed
 useEffect(() => {
  if (choice) {
   if (!dialog.current?.open) dialog.current?.showModal()
  } else dialog.current?.close()
 }, [choice])
 return (
  <dialog
   ref={dialog}
   className="choice-sheet"
   aria-labelledby="choice-title"
   onCancel={(event) => event.preventDefault()}
  >
   <h2 id="choice-title">
    {view.phase === 2 ? 'Escolha uma cor' : 'Trocar com quem?'}
   </h2>
   <p className="sub">Confirme sua escolha para continuar o turno.</p>
   {view.phase === 2 ? (
    <div className="color-options">
     {[1, 2, 3, 4].map((color) => (
      <button
       key={color}
       className={`color-choice color-${color}`}
       disabled={disabled}
       onClick={() => send('color', { color })}
      >
       {colors[color]}
      </button>
     ))}
    </div>
   ) : (
    <div className="target-options">
     {view.players
      .filter((p) => p.active && !p.me)
      .map((p) => (
       <button
        key={p.key}
        disabled={disabled}
        onClick={() => send('target', { target: p.key })}
       >
        {p.name}
       </button>
      ))}
     <button disabled={disabled} onClick={() => send('keep')}>
      Manter minha mão
     </button>
    </div>
   )}
  </dialog>
 )
}
export function ResultScreen({ view }: { view: GameView }) {
 const me = view.players.find((p) => p.me)
 return (
  <main className="result-page">
   <section className="result-panel">
    <span className="eyebrow">{view.group} · FIM DA PARTIDA</span>
    <span className="avatar gold">{me?.name.slice(0, 1) || 'J'}</span>
    <h1>
     {me?.position === 1
      ? `Boa, ${me.name.split(' ')[0]}!`
      : 'Partida encerrada'}
    </h1>
    <p className="sub">
     {view.close_reason === 'cancelled'
      ? 'Esta partida foi cancelada.'
      : me?.position
        ? `Você ficou em ${me.position}º nesta mesa.`
        : 'Você saiu desta partida.'}
    </p>
    <div className="result-points">
     <Icon name="trophy" />
     {view.result
      ? view.result.score_units != null
        ? `+${formatScore(view.result.score_units, view.result.system)} pontos confirmados`
        : 'Você não recebeu pontos nesta partida'
      : view.close_reason === 'cancelled'
        ? 'Partida cancelada · sem pontuação'
        : 'Aguardando confirmação da pontuação'}
    </div>
    <div className="list">
     {[...view.players]
      .sort((a, b) => (a.position || 99) - (b.position || 99))
      .map((p) => {
       const award = view.awards?.find((a) => a.key === p.key)
       return (
        <div className={`row result-row ${p.me ? 'self' : ''}`} key={p.key}>
         <span className="rank">{p.position || '—'}</span>
         <span className="avatar">{p.name.slice(0, 1)}</span>
         <div className="who">
          <strong>
           {p.name}
           {p.me ? ' · você' : ''}
          </strong>
          <small>
           {p.position ? `${p.position}º lugar` : 'Fora do ranking'}
          </small>
         </div>
         {award?.score_units != null && (
          <div className="points">
           +{formatScore(award.score_units, view.system)}
           <small>pontos</small>
          </div>
         )}
        </div>
       )
      })}
    </div>
    <p className="sub">
     Inline e Mini App contam no mesmo ranking mensal,
     <br />
     sob a política desta sala.
    </p>
    <Link className="primary" to={`/ranking?system=${view.system}`}>
     Ver ranking
     <Icon name="arrow" />
    </Link>
    <Link className="outline" to="/home">
     Voltar ao início
    </Link>
   </section>
  </main>
 )
}
export function GameTable({
 view,
 connected,
 pending,
 error,
 send,
 back,
}: {
 view: GameView
 connected: boolean
 pending: boolean
 error: string
 send: (action: string, extra?: Partial<GameCommand>) => void
 back: () => void
}) {
 const [selected, setSelected] = useState('')
 const [menu, setMenu] = useState(false)
 const previousTop = useRef(view.top?.ID)
 const [animatedTop, setAnimatedTop] = useState('')
 useEffect(() => {
  if (previousTop.current !== view.top?.ID) {
   setAnimatedTop(view.top?.ID ?? '')
   previousTop.current = view.top?.ID
  }
 }, [view.top?.ID])
 const previousHand = useRef(new Set(view.hand.map((c) => c.Card.ID)))
 const [receivedCards, setReceivedCards] = useState<string[]>([])
 useEffect(() => {
  const incoming = view.hand.map((c) => c.Card.ID)
  const added = incoming.filter((id) => !previousHand.current.has(id))
  if (added.length) setReceivedCards(added)
  previousHand.current = new Set(incoming)
 }, [view.hand])
 const hand = useRef<HTMLDivElement>(null)
 const previousCount = useRef(view.hand.length)
 useEffect(() => {
  if (view.hand.length > previousCount.current)
   hand.current?.lastElementChild?.scrollIntoView({
    block: 'nearest',
    inline: 'nearest',
    behavior: window.matchMedia('(prefers-reduced-motion: reduce)').matches
     ? 'auto'
     : 'smooth',
   })
  previousCount.current = view.hand.length
 }, [view.hand.length])
 const selectedCard = view.hand.find(
  (c) => c.Card.ID === selected && c.Playable,
 )
 const disabled = !connected || pending || view.closed
 const opponents = view.players.filter((p) => !p.me && p.active)
 const me = view.players.find((p) => p.me)
 const current = view.players.find((p) => p.current)
 const sideOpponents =
  opponents.length >= 3 && opponents.length <= 5 ? opponents.slice(-2) : []
 const topOpponents = sideOpponents.length ? opponents.slice(0, -2) : opponents
 if (view.closed) return <ResultScreen view={view} />
 if (view.phase === 0)
  return (
   <main className="page lobby">
    <button
     className="icon-button"
     aria-label="Voltar ao início"
     onClick={back}
    >
     <Icon name="back" />
    </button>
    <span className="eyebrow">
     SALA REAL · {view.mode === 'caseiro' ? 'CASEIRO' : 'CLÁSSICO'}
    </span>
    <h1>{view.group || 'Sua mesa'}</h1>
    <p className="sub">
     Entre pelo grupo com /entrar. O responsável pode começar com pelo menos
     dois jogadores.
    </p>
    <div className="lobby-seats">
     {view.players
      .filter((p) => p.active)
      .map((p) => (
       <PlayerBadge player={p} key={p.key} />
      ))}
    </div>
    {error && (
     <p role="alert" className="notice">
      {error}
     </p>
    )}
    <p role="status">
     {connected
      ? `${view.players.filter((p) => p.active).length} jogadores na sala`
      : 'Reconectando…'}
    </p>
    {view.owner && (
     <button
      className="primary"
      disabled={disabled || view.players.filter((p) => p.active).length < 2}
      onClick={() => send('start')}
     >
      Começar partida
      <Icon name="arrow" />
     </button>
    )}
    <button
     className="outline load-more"
     disabled={disabled}
     onClick={() => send('leave')}
    >
     Desistir da sala
    </button>
   </main>
  )
 return (
  <main className={`game players-${opponents.length + 1}`}>
   <header className="game-header">
    <button
     className="icon-button"
     aria-label="Voltar ao início"
     onClick={back}
    >
     <Icon name="back" />
    </button>
    <span className="mini" title={view.group}>
     <i />
     {view.group || 'Partida'} ·{' '}
     {view.mode === 'caseiro' ? 'Caseiro' : 'Clássico'}
    </span>
    <button
     className="icon-button"
     aria-label="Opções da partida"
     onClick={() => setMenu(!menu)}
    >
     <Icon name="cards" />
    </button>
   </header>
   {menu && (
    <div className="game-menu">
     <button onClick={back}>Sair da tela e continuar na partida</button>
     <button
      disabled={disabled}
      onClick={() => {
       send('leave')
       setMenu(false)
      }}
     >
      Desistir da partida
     </button>
     {view.owner && (
      <button
       disabled={disabled}
       onClick={() => {
        send('cancel')
        setMenu(false)
       }}
      >
       Cancelar partida
      </button>
     )}
    </div>
   )}
   <section
    className={`opponents ${opponents.length > 3 ? 'many-opponents' : ''}`}
    aria-label="Oponentes"
   >
    {opponents.length === 1 && (
     <div className="opponent-hand" aria-hidden="true">
      {Array.from({ length: Math.min(opponents[0].count, 10) }, (_, i) => (
       <img key={i} src="/assets/cards/back.svg" alt="" />
      ))}
     </div>
    )}
    {topOpponents.map((p) => (
     <PlayerBadge player={p} key={p.key} />
    ))}
   </section>
   <section className="table" aria-label="Pilhas de cartas">
    {sideOpponents.map((p, i) => (
     <div key={p.key} className={`side-opponent side-${i}`}>
      <PlayerBadge player={p} />
     </div>
    ))}
    <svg
     className={`table-orbit ${view.direction === -1 ? 'reversed' : ''}`}
     viewBox="0 0 265 242"
     aria-label={
      view.direction === 1 ? 'Sentido horário' : 'Sentido anti-horário'
     }
    >
     <path d="M23 62 Q126 16 218 64 M207 49 L220 66 L197 68 M241 178 Q133 224 44 179 M57 194 L42 178 L65 175" />
    </svg>
    {view.top && (
     <img
      key={view.top.ID}
      className={`table-card discard ${animatedTop === view.top.ID ? 'confirmed-card' : ''}`}
      src={cardAsset(view.top)}
      alt={`Carta na mesa: ${cardLabel(view.top)}`}
     />
    )}
    <button
     className="draw"
     aria-label="Comprar carta"
     disabled={
      disabled || !view.my_turn || view.phase !== 1 || !!view.drawn_card_id
     }
     onClick={() => send('draw')}
    >
     <img src="/assets/cards/back.svg" alt="" />
    </button>
    <div className="pile-label">
     <span>carta na mesa · {colors[view.active_color]}</span>
     <span>comprar carta</span>
    </div>
   </section>
   <div className="turn-info">
    <Icon name="spark" />
    <span>
     {view.my_turn
      ? 'Sua vez de jogar'
      : current
        ? `Vez de ${current.name}`
        : 'Aguardando turno'}
    </span>
    <Timer view={view} />
   </div>
   {(!connected || error) && (
    <p className="connection-banner" role={error ? 'alert' : 'status'}>
     {error || 'Conexão perdida. Reconectando…'}
    </p>
   )}
   <section className="hand-area" aria-label="Sua mão">
    <div className="hand-label">
     <span>{view.my_turn ? 'Escolha uma carta' : 'Aguarde seu turno'}</span>
     <span>{view.hand.length} cartas · arraste para ver mais</span>
    </div>
    <div ref={hand} className="hand">
     {view.hand.map((c) => (
      <button
       key={c.Card.ID}
       className={`${c.Playable ? 'playable' : ''} ${selected === c.Card.ID ? 'selected' : ''} ${receivedCards.includes(c.Card.ID) ? 'confirmed-draw' : ''}`}
       aria-label={`${cardLabel(c.Card)} · ${c.Playable ? 'pode jogar' : 'indisponível neste turno'}`}
       aria-pressed={selected === c.Card.ID}
       disabled={disabled || !c.Playable}
       onClick={() => setSelected(c.Card.ID)}
      >
       <img src={cardAsset(c.Card)} alt="" />
      </button>
     ))}
    </div>
   </section>
   <footer className="local-player">
    <div className="me">
     {me && <span className="avatar">{me.name.slice(0, 1)}</span>}
     <div className="local-name">
      Você{' '}
      <span className="count">
       {me?.count ?? 0}
       <Icon name="cards" />
      </span>
      <small title={me?.name}>{me?.name}</small>
     </div>
    </div>
    <button
     className="play-button"
     disabled={disabled || !selectedCard}
     onClick={() =>
      selectedCard && send('play', { card_id: selectedCard.Card.ID })
     }
    >
     {pending
      ? 'Confirmando…'
      : selectedCard
        ? 'Jogar carta'
        : 'Escolher carta'}
    </button>
   </footer>
   {view.my_turn && (view.drawn_card_id || view.can_bluff) && (
    <div className="extra-actions">
     {view.drawn_card_id && (
      <button disabled={disabled} onClick={() => send('pass')}>
       Passar turno
      </button>
     )}
     {view.can_bluff && (
      <button disabled={disabled} onClick={() => send('bluff')}>
       Desafiar +4
      </button>
     )}
    </div>
   )}
   <Choice view={view} disabled={disabled} send={send} />
  </main>
 )
}
export function GamePage() {
 const { gameID = '' } = useParams()
 const navigate = useNavigate()
 const back = useCallback(() => navigate('/home'), [navigate])
 useTelegram(back, { headerColor: '#10292F' })
 const game = useGame(gameID)
 return game.view ? (
  <GameTable {...game} view={game.view} back={back} />
 ) : (
  <main className="page">
   <h1>Abrindo sua mesa</h1>
   <p role="status" className="notice">
    {game.error || 'Conectando ao servidor…'}
   </p>
   <button className="outline" onClick={back}>
    Voltar ao início
   </button>
  </main>
 )
}
