import { useCallback, useEffect, useLayoutEffect, useRef, useState } from 'react'
import { Link, useNavigate, useParams } from 'react-router'
import type { Card, GameCommand, GameView, Seat } from '../api/client'
import { gsap } from 'gsap'
import { useTableMotion } from '../hooks/useTableMotion'
import { handLayout } from '../lib/handLayout'
import { ShareRoom } from '../components/ShareRoom'
import { Icon } from '../components/Mobile'
import { Avatar } from '../components/Ranking'
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
export function cardAsset(card: Card, activeColor = 0) {
 if (card.Rank === 15) return '/assets/cards/swap_hands.png'
 if (card.Rank >= 13) {
  const prefix = colorFiles[activeColor] ? `${colorFiles[activeColor]}_` : ''
  return `/assets/cards/${prefix}${card.Rank === 14 ? 'wild_draw4' : 'wild'}.png`
 }
 return `/assets/cards/${colorFiles[card.Color]}_${card.Rank < 10 ? card.Rank : ['skip', 'reverse', 'draw2'][card.Rank - 10]}.png`
}
export function cardLabel(card: Card) {
 return `${colors[card.Color]} ${ranks[card.Rank]}`.trim()
}
function PlayerAvatar({ player }: { player: Seat }) {
 if (player.avatar_url) return <Avatar item={{ key: player.key, name: player.name, avatar_url: player.avatar_url, position: 0, score_units: '0', masked_id: '' }} />
 return <span className="avatar">{player.name.trim().slice(0, 1).toUpperCase() || 'J'}</span>
}
function PlayerBadge({ player }: { player: Seat }) {
 return (
  <div data-player-key={player.key} className={`seat ${player.current ? 'current' : ''}`}>
   <PlayerAvatar player={player} />
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
function Choice({ view, disabled, error, send }: {
 view: GameView; disabled: boolean; error: string; send: (action: string, extra?: Partial<GameCommand>) => void
}) {
 const dialog = useRef<HTMLDialogElement>(null)
 const surface = useRef<HTMLDivElement>(null)
 const sent = useRef(false)
 const feedback = useRef<gsap.core.Timeline | undefined>(undefined)
 const context = useRef<gsap.Context | undefined>(undefined)
 const [chosen, setChosen] = useState(0)
 const choice = view.my_turn && [2, 4].includes(view.phase) && !view.closed
 const latest = useRef({ choice, disabled, error })
 const choiceKey = `${view.game_id}:${view.top?.ID ?? ''}:${view.phase}`
 const previousChoice = useRef({choice,key:choiceKey})
 useLayoutEffect(() => { latest.current = { choice, disabled, error } }, [choice, disabled, error])
 useLayoutEffect(() => {
  context.current = gsap.context(() => {}, surface)
  const node = dialog.current
  return () => { feedback.current?.kill(); context.current?.revert(); node?.close() }
 }, [])
 useLayoutEffect(() => {
  const previous=previousChoice.current
  previousChoice.current={choice,key:choiceKey}
  if(choice && (!previous.choice || previous.key!==choiceKey)) {
   // A fresh server-required choice supersedes feedback from an earlier action.
   feedback.current?.kill();sent.current=false;setChosen(0)
   gsap.killTweensOf(surface.current)
   gsap.killTweensOf(surface.current?.querySelectorAll('.color-choice') ?? [])
   context.current?.add(()=>{
    gsap.set(surface.current,{opacity:1,scale:1})
    gsap.set(surface.current?.querySelectorAll('.color-choice') ?? [],{backgroundColor:(_i,el:HTMLElement)=>['','#fb3045','#1b71f6','#60c400','#ffbd10'][Number(el.dataset.color)]})
   })
  }
 },[choice,choiceKey])
 useLayoutEffect(() => {
  context.current?.add(() => {
   const reduced = window.matchMedia?.('(prefers-reduced-motion: reduce)').matches ?? false
   if (choice && !dialog.current?.open) {
    dialog.current?.showModal(); dialog.current?.focus()
    gsap.fromTo(surface.current, { opacity: 0, scale: .82 }, { opacity: 1, scale: 1, duration: reduced ? 0 : .5, ease: 'power2.out' })
   } else if (!choice && dialog.current?.open && !sent.current) {
    gsap.to(surface.current, { opacity: 0, scale: .94, duration: reduced ? 0 : .35, onComplete: () => dialog.current?.close() })
   } else if (!choice && sent.current && !feedback.current?.isActive()) { dialog.current?.close(); sent.current = false; setChosen(0) }
  })
 }, [choice])
 useEffect(() => {
  if (!disabled && choice && error && sent.current) {
   feedback.current?.kill(); sent.current = false; setChosen(0)
   context.current?.add(() => {
    gsap.set(surface.current, { opacity: 1, scale: 1 })
    gsap.to(surface.current?.querySelectorAll('.color-choice') ?? [], { backgroundColor: (_i, el: HTMLElement) => ['','#fb3045','#1b71f6','#60c400','#ffbd10'][Number(el.dataset.color)], duration: window.matchMedia?.('(prefers-reduced-motion: reduce)').matches ? 0 : .2 })
   })
  }
 }, [disabled, choice, error])
 const chooseColor = (color: number) => {
  if (disabled || sent.current || !choice) return
  sent.current = true; setChosen(color)
  send('color', { color })
  context.current?.add(() => {
   const reduced = window.matchMedia?.('(prefers-reduced-motion: reduce)').matches ?? false
   feedback.current = gsap.timeline({ onComplete: () => {
    if (!latest.current.choice) { dialog.current?.close(); sent.current = false; setChosen(0) }
    else gsap.set(surface.current, { opacity: 1, scale: 1 })
   } })
    .to(surface.current?.querySelectorAll('.color-choice') ?? [], { backgroundColor: ['','#fb3045','#1b71f6','#60c400','#ffbd10'][color], duration: reduced ? 0 : .45, ease: 'sine.inOut' })
    .to(surface.current, { opacity: 0, scale: .94, duration: reduced ? 0 : .35, ease: 'power2.inOut' }, reduced ? 0 : '+=.7')
  })
 }
 const target = view.phase === 4 && !chosen
 return <dialog ref={dialog} tabIndex={-1} className={`choice-sheet v2-choice ${target ? 'target-sheet' : ''}`} aria-labelledby="choice-title" onCancel={event => event.preventDefault()}>
  <div ref={surface} className="choice-surface">
   <h2 id="choice-title" className={target ? '' : 'sr-only'}>{target ? 'Trocar com quem?' : 'Escolha uma cor'}</h2>
   {!target ? <div className="color-options" role="group" aria-label="Escolher cor">
    {[1, 4, 3, 2].map(color => <button key={color} data-color={color} className={`color-choice color-${color}`} aria-label={colors[color]} aria-pressed={chosen === color} disabled={disabled || !!chosen} onClick={() => chooseColor(color)} />)}
   </div> : <div className="target-options">
    {view.players.filter(p => p.active && !p.me).map(p => <button key={p.key} disabled={disabled} onClick={() => send('target', { target: p.key })}>{p.name}</button>)}
    <button disabled={disabled} onClick={() => send('keep')}>Manter minha mão</button>
   </div>}
  </div>
 </dialog>
}
export function ResultScreen({ view, disabled = true, send }: { view: GameView; disabled?: boolean; send?: (action: string) => void }) {
 const panel = useRef<HTMLElement>(null)
 const me = view.players.find(p => p.me)
 const rematch = view.rematch
 const accepted = !!me && !!rematch?.accepted.includes(me.key)
 const missing = view.players.filter(p => rematch?.required.includes(p.key) && !rematch.accepted.includes(p.key))
 useLayoutEffect(() => {
  const mm = gsap.matchMedia()
  mm.add('(prefers-reduced-motion: no-preference)', () => {
   gsap.fromTo(panel.current, { opacity: 0, y: 16 }, { opacity: 1, y: 0, duration: .55, ease: 'power2.out' })
   gsap.to('.crown-light', { xPercent: 240, duration: 1.8, repeat: -1, repeatDelay: .8, ease: 'sine.inOut' })
   gsap.to('.crown', { boxShadow: '0 0 25px #ffd400bb', duration: 1.2, yoyo: true, repeat: -1, ease: 'sine.inOut' })
  }, panel)
  return () => mm.revert()
 }, [])
 return <section className="result-page" role="region" aria-label="Resultado da partida">
  <section ref={panel} className="result-panel">
   <h1>{view.close_reason === 'cancelled' ? 'Partida cancelada' : me?.position === 1 ? 'Você venceu' : 'Você perdeu'}</h1>
   <div className="result-list">
    {[...view.players].sort((a, b) => (a.position || 99) - (b.position || 99)).map(p => {
     const award = view.awards?.find(a => a.key === p.key)
     return <div className={`result-row ${p.position === 1 ? 'winner' : ''}`} key={p.key}>
      {p.position === 1 ? <span className="crown"><svg viewBox="0 0 24 24" aria-hidden="true"><path d="m3 7 5 3 4-6 4 6 5-3-2 13H5Z" /></svg><i className="crown-light" /></span> : <PlayerAvatar player={p} />}
      <span className="who"><strong title={p.name}>{p.me ? 'Você' : p.name}</strong><small>{p.position === 1 ? 'VENCEDOR' : p.position ? `${p.position}º lugar` : 'Fora do ranking'}</small></span>
      {rematch?.accepted.includes(p.key) && <span className="ready-mark" aria-label={`${p.name} aceitou a revanche`}>↻</span>}
      {award?.score_units != null && <span className="points">+{formatScore(award.score_units, view.system)}</span>}
     </div>
    })}
   </div>
   <p className="result-status">{view.result ? view.result.score_units != null ? `+${formatScore(view.result.score_units, view.result.system)} pontos confirmados` : 'Você não recebeu pontos nesta partida' : view.close_reason === 'cancelled' ? 'Partida cancelada · sem pontuação' : 'Aguardando confirmação da pontuação'}</p>
   {rematch && <>
    <button className="rematch" aria-label="Aceitar revanche" disabled={disabled || (accepted && missing.length > 0) || !rematch.ready} onClick={() => send?.('rematch')}><Icon name="reverse" /></button>
    <span className="rematch-label">Revanche</span>
    <p role="status" className="result-status">{rematch.next_game_id ? 'Todos aceitaram · abrindo nova partida' : !rematch.ready ? 'Aguardando confirmação do resultado' : `${rematch.accepted.length}/${rematch.required.length} aceitaram${missing.length ? ` · faltam ${missing.map(p => p.me ? 'você' : p.name).join(', ')}` : ''}`}</p>
    <p className="rematch-policy">Sair ou desconectar não remove participantes. A revanche exige todos.</p>
    {accepted && !rematch.next_game_id && <button className="result-link" disabled={disabled} onClick={() => send?.('rematch_leave')}>Retirar minha aceitação</button>}
   </>}
   <Link className="result-link" to={`/ranking?system=${view.system}`}>Ver ranking</Link>
   <Link className="result-link" to="/home">Voltar ao início</Link>
  </section>
 </section>
}
export function GameTable({
 view,
 connected,
 pending,
 error,
 send,
 back,
 dealOnOpen = false,
}: {
 view: GameView
 connected: boolean
 pending: boolean
 error: string
 send: (action: string, extra?: Partial<GameCommand>) => void
 back: () => void
 dealOnOpen?: boolean
}) {
 const [selected, setSelected] = useState('')
 const submitted = useRef(false)
 useEffect(() => { setSelected('') }, [view.game_id])
 useEffect(() => { if (!pending) submitted.current = false }, [pending,error,view.revision,view.game_id])
 useEffect(() => {
  if (!view.my_turn || view.closed || !view.hand.some(c => c.Card.ID === selected && c.Playable)) setSelected('')
 }, [view.my_turn,view.closed,view.hand,selected,view.game_id])
 const [initialDeal] = useState(dealOnOpen)
 const [menu, setMenu] = useState(false)
 const root = useRef<HTMLElement>(null)
 const handViewport = useRef<HTMLDivElement>(null)
 const [handSize, setHandSize] = useState({width:320,height:166})
 useLayoutEffect(() => {
  const node = handViewport.current
  if (!node) return
  const measure = () => setHandSize({width:node.clientWidth || 320,height:node.clientHeight || 166})
  measure()
  const observer = typeof ResizeObserver !== 'undefined' ? new ResizeObserver(measure) : undefined
  observer?.observe(node)
  return () => observer?.disconnect()
 }, [view.phase, view.closed])
 const layout = handLayout(view.hand, handSize.width, handSize.height)
 const [touching, setTouching] = useState(false)
 const frozenLayout = useRef(layout)
 useLayoutEffect(() => { if (!touching) frozenLayout.current = layout }, [touching, layout])
 const presented = touching ? frozenLayout.current : layout
 const selectedCard = view.hand.find(
  (c) => c.Card.ID === selected && c.Playable && view.my_turn,
 )
 const visibleSelected = selectedCard?.Card.ID ?? ''
 useTableMotion(root, view, visibleSelected, cardAsset, initialDeal)
 const disabled = !connected || pending || view.closed
 const opponents = view.players.filter((p) => !p.me && (p.active || view.closed))
 const me = view.players.find((p) => p.me)
 const current = view.players.find((p) => p.current)
 const topOpponents = opponents
 if (view.phase === 0)
  return (
   <main ref={root} className="page lobby">
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
     Compartilhe o convite. Um jogador inscrito pode iniciar quando houver pelo menos dois jogadores.
    </p>
    <div className="lobby-seats">
     {view.players
      .filter((p) => p.active)
      .map((p) => (
       <div key={p.key}><PlayerBadge player={p} /><small className="seat-role">{p.key === view.owner_key ? 'Responsável' : 'Jogador'}</small></div>
      ))}
    </div>
    {error && (
     <p role="alert" className="notice">
      {error}
     </p>
    )}
    <p role="status">
     {connected
      ? `${view.players.filter((p) => p.active).length}/${view.capacity ?? 10} jogadores na sala`
      : 'Reconectando…'}
    </p>
    <ShareRoom gameID={view.game_id} />
    {(view.can_start ?? view.owner) && (
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
  <main ref={root} data-hand-rows={layout.rows} style={{ '--table-top': layout.rows > 1 ? (opponents.length > 3 ? '44%' : '39%') : '45%' } as React.CSSProperties} className={`game game-v2 players-${opponents.length + 1} ${view.my_turn && !view.closed ? 'my-turn' : 'other-turn'}`}>
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
    {topOpponents.map((p) => (
     <PlayerBadge player={p} key={p.key} />
    ))}
   </section>
   <section className="table" aria-label="Pilhas de cartas">
    <div className={`table-orbit ${view.direction === -1 ? 'reversed' : ''}`} role="img" aria-label={view.direction === 1 ? 'Sentido horário' : 'Sentido anti-horário'}>
     <span className="arrow top" />
     <span className="arrow bottom" />
    </div>
    {view.top && (
     <span className="table-card discard" role="img" aria-label={`Carta na mesa: ${cardLabel(view.top)}${view.top.Rank>=13 && colors[view.active_color] ? ` · cor ativa ${colors[view.active_color]}` : ''}`} />
    )}
    <button
     className={`draw ${view.my_turn && !disabled ? 'ready' : ''}`}
     aria-label="Comprar carta"
     disabled={
      disabled || !view.my_turn || view.phase !== 1 || !!view.drawn_card_id
     }
     onClick={() => send('draw')}
    >

    </button>
   </section>
   <div className="turn-info" role="status">
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
   <div className="turn-glow" aria-hidden="true" />
   <section className="hand-area" aria-label="Sua mão">
    <div ref={handViewport} className="hand" tabIndex={0} aria-label="Cartas organizadas por cor, até oito por fileira; deslize verticalmente para ver todas">
     <div className="hand-track" style={{ height: presented.height }} onPointerDown={() => setTouching(true)} onPointerUp={() => setTouching(false)} onPointerCancel={() => setTouching(false)} onLostPointerCapture={() => setTouching(false)}>
     {presented.slots.map((slot, index) => {
      const c = view.hand.find(c => c.Card.ID === slot.id)
      if (!c) return null
      return <button
       key={slot.id} data-card-id={slot.id} data-card-width={slot.width} data-slot={index} data-row={slot.row}
       style={{ left: slot.x, top: slot.y, width: slot.hitWidth, height: slot.height }}
       className={`${c.Playable && view.my_turn ? 'playable' : ''} ${visibleSelected === slot.id ? 'selected' : ''}`}
       aria-label={`${cardLabel(c.Card)} · ${c.Playable && view.my_turn ? 'pode jogar' : 'indisponível neste turno'}`}
       aria-pressed={visibleSelected === slot.id}
       disabled={disabled || !view.my_turn || !c.Playable}
       onClick={() => {
        if (disabled || submitted.current || !view.my_turn || !c.Playable) return
        if (visibleSelected !== slot.id) setSelected(slot.id)
        else { submitted.current = true; send('play', {card_id:slot.id}) }
       }}
      />
     })}
     </div>
    </div>
   </section>
   <footer className="local-player">
    <div className={`me ${view.my_turn && !view.closed ? 'current' : ''}`} data-player-key={me?.key}>
     {me && <PlayerAvatar player={me} />}
     <div className="local-name">
      Você{' '}
      <span className="count">
       {me?.count ?? 0}
       <Icon name="cards" />
      </span>
      <small title={me?.name}>{me?.name}</small>
     </div>
    </div>
    <span className="hand-hint" role="status">
     {pending
      ? 'Confirmando…'
      : selectedCard
        ? 'Toque novamente para jogar'
        : 'Toque em uma carta'}
    </span>
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
   </footer>
   {view.closed && <ResultScreen view={view} disabled={!connected || pending} send={send} />}
   <Choice key={view.game_id} view={view} disabled={disabled} error={error} send={send} />
  </main>
 )
}
export function GamePage() {
 const { gameID = '' } = useParams()
 const navigate = useNavigate()
 const newRound = useRef('')
 const back = useCallback(() => navigate('/home'), [navigate])
 useTelegram(back, { headerColor: '#10292F' })
 const game = useGame(gameID)
 const next = game.view?.game_id === gameID ? game.view.rematch?.next_game_id : undefined
 useEffect(() => { if (next) { newRound.current=next; navigate(`/game/${encodeURIComponent(next)}`, { replace: true }) } }, [next, navigate])
 useEffect(() => { if(game.view?.game_id===gameID && newRound.current===gameID)newRound.current='' },[gameID,game.view])
 return game.view?.game_id === gameID ? (
  <GameTable key={gameID} {...game} view={game.view} back={back} dealOnOpen={newRound.current === gameID} />
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
