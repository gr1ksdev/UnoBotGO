import { useCallback, useEffect, useLayoutEffect, useMemo, useRef, useState, useSyncExternalStore } from 'react'
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
import { telegramSettingsAvailable, useTelegram } from '../lib/telegram'
import { tableFeedback } from '../lib/tableFeedback'
import { ErrorBoundary } from '../components/ErrorBoundary'
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
export function cardAsset(card: Card | null | undefined, activeColor = 0) {
 if (!card) return '/assets/backs/default.png'
 if (card.Rank === 15) return '/assets/cards/swap_hands.png'
 if (card.Rank >= 13) {
  const prefix = colorFiles[activeColor] ? `${colorFiles[activeColor]}_` : ''
  return `/assets/cards/${prefix}${card.Rank === 14 ? 'wild_draw4' : 'wild'}.png`
 }
 return `/assets/cards/${colorFiles[card.Color] ?? 'red'}_${card.Rank < 10 ? card.Rank : ['skip', 'reverse', 'draw2'][card.Rank - 10] ?? '0'}.png`
}
export function cardLabel(card: Card) {
 return `${colors[card.Color]} ${ranks[card.Rank]}`.trim()
}
function PlayerAvatar({ player }: { player: Seat }) {
 if (player.avatar_url) return <Avatar item={{ key: player.key, name: player.name, avatar_url: player.avatar_url, position: 0, score_units: '0', masked_id: '' }} />
 return <span className="avatar">{player.name.trim().slice(0, 1).toUpperCase() || 'J'}</span>
}
function PlayerBadge({ player, showCount = true }: { player: Seat; showCount?: boolean }) {
 const badge = useRef<HTMLDivElement>(null)
 const previousCount = useRef(player.count)
 useLayoutEffect(()=>{
  if(previousCount.current===player.count) return
  previousCount.current=player.count
  if(window.matchMedia?.('(prefers-reduced-motion: reduce)').matches) return
  const context=gsap.context(()=>{gsap.fromTo('.count',{scale:1.18,y:-3},{scale:1,y:0,duration:.45,ease:'elastic.out(.65,.7)',clearProps:'transform'})},badge)
  return ()=>context.revert()
 },[player.count])
 return (
  <div ref={badge} data-player-key={player.key} className={`seat ${player.current ? 'current' : ''}`}>
   <PlayerAvatar player={player} />
   <div className="player-line">
    <strong title={player.name}>{player.me ? 'Você' : player.name}</strong>
    {showCount && <span className="count">
     {player.count}
     <Icon name="cards" />
    </span>}
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
function safeShowModal(dialog: HTMLDialogElement | null) {
 try {
  if (dialog && !dialog.open) {
   dialog.showModal()
  }
 } catch (e) {
  console.warn('Dialog showModal prevented exception:', e)
 }
}

function safeClose(dialog: HTMLDialogElement | null) {
 try {
  if (dialog && dialog.open) {
   dialog.close()
  }
 } catch (e) {
  console.warn('Dialog close prevented exception:', e)
 }
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
  return () => { feedback.current?.kill(); context.current?.revert(); safeClose(node) }
 }, [])
 useLayoutEffect(() => {
  const previous=previousChoice.current
  previousChoice.current={choice,key:choiceKey}
  if(choice && (!previous.choice || previous.key!==choiceKey)) {
   // A fresh server-required choice supersedes feedback from an earlier action.
   feedback.current?.kill();sent.current=false;setChosen(0)
   gsap.killTweensOf(surface.current)
   gsap.killTweensOf(surface.current?.querySelector('.color-options') ?? [])
   gsap.killTweensOf(surface.current?.querySelectorAll('.color-choice') ?? [])
   context.current?.add(()=>{
    gsap.set(surface.current,{opacity:1,scale:1})
    gsap.set(surface.current?.querySelector('.color-options') ?? [],{rotation:45})
    gsap.set(surface.current?.querySelectorAll('.color-choice') ?? [],{opacity:1,scale:1,backgroundColor:(_i,el:HTMLElement)=>['','#fb3045','#1b71f6','#60c400','#ffbd10'][Number(el.dataset.color)]})
   })
  }
 },[choice,choiceKey])
 useLayoutEffect(() => {
  context.current?.add(() => {
   const reduced = window.matchMedia?.('(prefers-reduced-motion: reduce)').matches ?? false
   if (choice && !sent.current) {
    if(!dialog.current?.open) { safeShowModal(dialog.current); dialog.current?.focus(); tableFeedback.play('picker') }
    gsap.fromTo(surface.current, { opacity: 0, scale: .82 }, { opacity: 1, scale: 1, duration: reduced ? 0 : .55, ease: 'elastic.out(1,.7)' })
    const picker = surface.current?.querySelector('.color-options')
    if(picker) {
     gsap.fromTo(picker,{rotation:-20},{rotation:45,duration:reduced ? 0 : .8,ease:'elastic.out(1,.7)'})
     gsap.fromTo(picker.querySelectorAll('.color-choice'),{scale:.6,opacity:0},{scale:1,opacity:1,duration:reduced ? 0 : .5,stagger:reduced ? 0 : .08,ease:'back.out(1.4)'})
    }
   } else if (!choice && dialog.current?.open && !sent.current) {
    gsap.to(surface.current, { opacity: 0, scale: .94, duration: reduced ? 0 : .35, onComplete: () => safeClose(dialog.current) })
   } else if (!choice && sent.current && (!feedback.current || feedback.current.totalProgress() === 1)) { safeClose(dialog.current); sent.current = false; setChosen(0) }
  })
 }, [choice, choiceKey])
 useEffect(() => {
  if (!disabled && choice && error && sent.current) {
   feedback.current?.kill(); sent.current = false; setChosen(0)
   context.current?.add(() => {
    gsap.set(surface.current, { opacity: 1, scale: 1 })
    gsap.set(surface.current?.querySelector('.color-options') ?? [],{rotation:45})
    gsap.to(surface.current?.querySelectorAll('.color-choice') ?? [], { opacity:1,scale:1,backgroundColor: (_i, el: HTMLElement) => ['','#fb3045','#1b71f6','#60c400','#ffbd10'][Number(el.dataset.color)], duration: window.matchMedia?.('(prefers-reduced-motion: reduce)').matches ? 0 : .2 })
   })
  }
 }, [disabled, choice, error])
 const chooseColor = (color: number) => {
  if (disabled || sent.current || !choice) return
  sent.current = true; setChosen(color)
  tableFeedback.play('pick',true)
  send('color', { color })
  context.current?.add(() => {
   const reduced = window.matchMedia?.('(prefers-reduced-motion: reduce)').matches ?? false
   const picker = surface.current?.querySelector('.color-options')
   const petals = [...(surface.current?.querySelectorAll<HTMLElement>('.color-choice') ?? [])]
   const ordered = [...petals.filter(p=>Number(p.dataset.color)===color),...petals.filter(p=>Number(p.dataset.color)!==color)]
   // A tap can arrive during entry. This feedback owns every animated picker target.
   gsap.killTweensOf([surface.current,picker,...petals])
   gsap.set(petals,{opacity:1})
   feedback.current = gsap.timeline({ onComplete: () => {
    if (!latest.current.choice) { safeClose(dialog.current); sent.current = false; setChosen(0) }
    else gsap.set(surface.current, { opacity: 1, scale: 1 })
   } })
    .to(surface.current,{opacity:1,scale:1,duration:reduced ? 0 : .2},0)
    .to(ordered, { backgroundColor: ['','#fb3045','#1b71f6','#60c400','#ffbd10'][color] ?? '#fb3045', duration: reduced ? 0 : .3, stagger:reduced ? 0 : .05, ease: 'sine.inOut' },0)
    .to(ordered,{scale:1.04,duration:reduced ? 0 : .22,stagger:reduced ? 0 : .05,ease:'sine.out'},0)
    .to(ordered,{scale:1,duration:reduced ? 0 : .22,stagger:reduced ? 0 : .05,ease:'sine.inOut'},reduced ? 0 : .4)
    .to(surface.current, { opacity: 0, scale: .82, duration: reduced ? 0 : .35, ease: 'power2.inOut' }, reduced ? 0 : 1.15)
   if(picker) feedback.current
    .to(picker,{rotation:45,duration:reduced ? 0 : .25,ease:'sine.out'},0)
    .to(picker,{rotation:165,duration:reduced ? 0 : .35,ease:'power2.inOut'},reduced ? 0 : 1.15)
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
   gsap.fromTo('.result-row',{opacity:0,y:18,scale:.96},{opacity:1,y:0,scale:1,stagger:.09,duration:.5,ease:'back.out(1.25)'})
   gsap.fromTo('.rematch',{opacity:0,scale:.6},{opacity:1,scale:1,delay:.5,duration:.6,ease:'elastic.out(.65,.7)'})
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
    <button className={`rematch ${accepted ? 'accepted' : ''}`} aria-label="Aceitar revanche" disabled={disabled || (accepted && missing.length > 0) || !rematch.ready} onClick={() => { tableFeedback.play('rematch',true);send?.('rematch') }}><Icon name="reverse" /></button>
    <span className="rematch-label">Revanche</span>
    <p role="status" className="result-status">{rematch.next_game_id ? 'Todos aceitaram · abrindo nova partida' : !rematch.ready ? 'Aguardando confirmação do resultado' : `${rematch.accepted.length}/${rematch.required.length} aceitaram${missing.length ? ` · faltam ${missing.map(p => p.me ? 'você' : p.name).join(', ')}` : ''}`}</p>
    <p className="rematch-policy">Sair ou desconectar não remove participantes. A revanche exige todos.</p>
    {accepted && !rematch.next_game_id && <button className="result-link" disabled={disabled} onClick={() => send?.('rematch_leave')}>Retirar minha aceitação</button>}
   </>}
   <Link className="result-link" to={`/ranking?system=${view.system}`}>Ver ranking</Link>
   <Link className="result-link" to="/play">Voltar ao início</Link>
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
 settingsOpen: externalSettingsOpen,
 closeSettings,
 nativeSettings = false,
}: {
 view: GameView
 connected: boolean
 pending: boolean
 error: string
 send: (action: string, extra?: Partial<GameCommand>) => void
 back: () => void
 dealOnOpen?: boolean
 settingsOpen?: boolean
 closeSettings?: () => void
 nativeSettings?: boolean
}) {
 const [selected, setSelected] = useState('')
 const preferences = useSyncExternalStore(tableFeedback.subscribe,tableFeedback.getSnapshot)
 useEffect(()=>{
  const visibility = () => { if(document.hidden) tableFeedback.stop() }
  document.addEventListener('visibilitychange',visibility)
  return ()=>{document.removeEventListener('visibilitychange',visibility);tableFeedback.stop()}
 },[])
 const submitted = useRef(false)
 useEffect(() => { setSelected('') }, [view.game_id])
 useEffect(() => { if (!pending) submitted.current = false }, [pending,error,view.revision,view.game_id])
 useEffect(() => {
  if (!view.my_turn || view.closed || !view.hand.some(c => c.Card.ID === selected && c.Playable)) setSelected('')
 }, [view.my_turn,view.closed,view.hand,selected,view.game_id])
 const [initialDeal] = useState(dealOnOpen)
 const [localSettingsOpen, setLocalSettingsOpen] = useState(false)
 const settingsOpen = externalSettingsOpen ?? localSettingsOpen
 const dismissSettings = closeSettings ?? (() => setLocalSettingsOpen(false))
 const openSettings = () => setLocalSettingsOpen(true)
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
 const disabled = !connected || pending || view.closed
 const release = (id: string) => {
  if(disabled || submitted.current || !view.my_turn || view.phase !== 1 || visibleSelected!==id || !view.hand.some(c=>c.Card.ID===id && c.Playable)) return
  submitted.current=true;send('play',{card_id:id})
 }
 useTableMotion(root, view, visibleSelected, cardAsset, initialDeal, release)
 const opponents = view.players.filter((p) => !p.me && (p.active || view.closed))
 const me = view.players.find((p) => p.me)
 const current = view.players.find((p) => p.current)
 const lobbyPlayers = useMemo(() => view.players.filter(p => p.active), [view.players])
 const [recentLobbyPlayers, setRecentLobbyPlayers] = useState<string[]>([])
 const previousLobbyPlayers = useRef(lobbyPlayers.map(p => p.key))
 useEffect(() => {
  const keys = lobbyPlayers.map(p => p.key)
  const joined = keys.filter(key => !previousLobbyPlayers.current.includes(key))
  previousLobbyPlayers.current = keys
  if (view.phase === 0 && joined.length) setRecentLobbyPlayers(joined)
 }, [lobbyPlayers, view.phase])
 useEffect(() => {
  if (!recentLobbyPlayers.length) return
  const timeout = window.setTimeout(() => setRecentLobbyPlayers([]), 3500)
  return () => window.clearTimeout(timeout)
 }, [recentLobbyPlayers])
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
    <h1>{view.room_type === 'free' ? 'Sala livre' : view.group || 'Sua mesa'}</h1>
    <p className="sub">Compartilhe o convite para chamar jogadores. A partida começa quando houver pelo menos dois.</p>
    <section className="lobby-roster" aria-labelledby="lobby-roster-title">
     <header className="lobby-roster-heading">
      <h2 id="lobby-roster-title">Jogadores</h2>
      <span>{lobbyPlayers.length} de {view.capacity ?? 10}</span>
     </header>
     <div className="lobby-seats" role="list" aria-label="Participantes da sala">
      {lobbyPlayers.map(p => {
       const owner = p.key === view.owner_key
       const role = owner && p.me ? 'Você · responsável' : owner ? 'Responsável da sala' : p.me ? 'Você' : 'Jogador'
       const recentlyJoined = recentLobbyPlayers.includes(p.key)
       return <div className={`lobby-player ${owner ? 'room-owner' : ''} ${p.me ? 'current-user' : ''} ${recentlyJoined ? 'just-joined' : ''}`} key={p.key} role="listitem" aria-label={`${p.name}, ${role}${recentlyJoined ? ', entrou agora' : ''}`}>
        <PlayerBadge player={p} showCount={false} />
        <div className="lobby-player-meta"><small>{role}</small>{recentlyJoined && <strong className="joined-label">Entrou agora</strong>}</div>
       </div>
      })}
      {Array.from({length: Math.min(Math.max(0, (view.capacity ?? 10) - lobbyPlayers.length), 3)}, (_, i) => <div className="lobby-player waiting-seat" key={`waiting-${i}`} role="listitem"><span className="waiting-avatar" aria-hidden="true">+</span><span>Aguardando jogador</span></div>)}
      {(view.capacity ?? 10) - lobbyPlayers.length > 3 && <div className="lobby-player more-seats" role="listitem">+ {(view.capacity ?? 10) - lobbyPlayers.length - 3} vagas</div>}
     </div>
    </section>
    {error && (
     <p role="alert" className="notice">
      {error}
     </p>
    )}
    <p role="status" className="lobby-connection">{connected ? 'Sala atualizada em tempo real' : 'Conexão perdida. Reconectando à sala…'}</p>
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
    <SettingsDialog open={settingsOpen} close={dismissSettings} preferences={preferences} />
   </main>
  )
 return (
  <main ref={root} data-input-enabled={!disabled && view.my_turn && view.phase===1} data-hand-rows={layout.rows} style={{ '--table-top': layout.rows > 1 ? (opponents.length > 3 ? '44%' : '39%') : '45%' } as React.CSSProperties} className={`game game-v2 players-${opponents.length + 1} ${view.my_turn && !view.closed ? 'my-turn' : 'other-turn'}`}>
   <header className="game-header">
    <button
     className="icon-button"
     aria-label="Voltar ao início"
     onClick={back}
    >
     <Icon name="back" />
    </button>
    <span className="mini" title={view.room_type === 'free' ? 'Sala livre' : view.group}>
     <i />
     {view.room_type === 'free' ? 'Sala livre' : view.group || 'Partida'} ·{' '}
     {view.mode === 'caseiro' ? 'Caseiro' : 'Clássico'}
    </span>
    {!nativeSettings && <button className="icon-button settings-fallback" aria-label="Configuração" onClick={openSettings}>⚙</button>}
   </header>
   <SettingsDialog
    open={settingsOpen}
    close={dismissSettings}
    preferences={preferences}
    actions={[
     { label: 'Sair da tela e continuar na partida', description: 'A partida continua; você pode voltar pela Home.', onClick: () => { dismissSettings(); back() } },
     { label: 'Desistir da partida', description: 'Você sai desta partida.', disabled, onClick: () => { send('leave'); dismissSettings() } },
     ...(view.owner ? [{ label: 'Cancelar partida', description: 'Encerra a sala para todos os jogadores.', disabled, onClick: () => { send('cancel'); dismissSettings() } }] : []),
    ]}
   />
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
        else release(slot.id)
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
        ? 'Toque de novo ou arraste à mesa'
        : 'Toque em uma carta'}
    </span>
   </footer>
   {view.closed && <ResultScreen view={view} disabled={!connected || pending} send={send} />}
   <Choice key={view.game_id} view={view} disabled={disabled} error={error} send={send} />
  </main>
 )
}

function SettingsDialog({ open, close, preferences, actions = [] }: { open: boolean; close: () => void; preferences: ReturnType<typeof tableFeedback.getSnapshot>; actions?: { label: string; description: string; onClick: () => void; disabled?: boolean }[] }) {
 if (!open) return null
 return (
  <div className="game-settings-backdrop" onMouseDown={event => { if (event.target === event.currentTarget) close() }}>
   <section className="game-settings" role="dialog" aria-modal="true" aria-labelledby="game-settings-title">
    <header><h2 id="game-settings-title">Configuração</h2><button className="icon-button" aria-label="Fechar configuração" onClick={close}>×</button></header>
    <button className="setting-row" role="switch" aria-checked={preferences.sound} onClick={() => { tableFeedback.configure({sound:!preferences.sound}); if (!preferences.sound) tableFeedback.play('select') }}>
     <span><strong>Som</strong><small>{preferences.sound ? 'Efeitos sonoros ativados' : 'Efeitos sonoros desativados'}</small></span><span className={`setting-state ${preferences.sound ? 'enabled' : ''}`}>{preferences.sound ? 'Ativado' : 'Desativado'}</span>
    </button>
    <button className="setting-row" role="switch" aria-checked={preferences.haptics} onClick={() => tableFeedback.configure({haptics:!preferences.haptics})}>
     <span><strong>Vibração</strong><small>{preferences.haptics ? 'Resposta tátil ativada' : 'Resposta tátil desativada'}</small></span><span className={`setting-state ${preferences.haptics ? 'enabled' : ''}`}>{preferences.haptics ? 'Ativada' : 'Desativada'}</span>
    </button>
    {actions.length > 0 && <section className="settings-actions" aria-labelledby="settings-actions-title">
     <h3 id="settings-actions-title">Partida</h3>
     {actions.map(action => <button key={action.label} className="settings-action" disabled={action.disabled} onClick={action.onClick}><strong>{action.label}</strong><small>{action.description}</small></button>)}
    </section>}
   </section>
  </div>
 )
}
export function GamePage() {
 const { gameID = '' } = useParams()
 const navigate = useNavigate()
 const newRound = useRef('')
 const back = useCallback(() => navigate('/play'), [navigate])
 const [settingsOpen, setSettingsOpen] = useState(false)
 const openSettings = useCallback(() => setSettingsOpen(true), [])
 const closeSettings = useCallback(() => setSettingsOpen(false), [])
 useTelegram(back, { headerColor: '#10292F', settings: openSettings })
 const game = useGame(gameID)
 const nativeSettings = telegramSettingsAvailable()
 const next = game.view?.game_id === gameID ? game.view.rematch?.next_game_id : undefined
 useEffect(() => { if (next) { newRound.current=next; navigate(`/game/${encodeURIComponent(next)}`, { replace: true }) } }, [next, navigate])
 useEffect(() => { if(game.view?.game_id===gameID && newRound.current===gameID)newRound.current='' },[gameID,game.view])
 return game.view?.game_id === gameID ? (
  <ErrorBoundary>
   <GameTable key={gameID} {...game} view={game.view} back={back} dealOnOpen={newRound.current === gameID} settingsOpen={nativeSettings ? settingsOpen : undefined} closeSettings={nativeSettings ? closeSettings : undefined} nativeSettings={nativeSettings} />
  </ErrorBoundary>
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
