import { Application, Assets, Container, Graphics, Sprite, Texture } from 'pixi.js'
import 'pixi.js/unsafe-eval'
import { gsap } from 'gsap'
import { PixiPlugin } from 'gsap/PixiPlugin'
import * as PIXI from 'pixi.js'
gsap.registerPlugin(PixiPlugin)
PixiPlugin.registerPIXI(PIXI)
import type { Card, GameView } from '../../api/client'

type Position = { x: number; y: number; width: number; height: number }
type CardNode = { group: Container; face: Sprite; lift: Container; target: Position }
const back = '/assets/backs/default.png'
const arrow = '/assets/effects/direction_arrow-ohEFIQ@2x.png'
const shadow = '/assets/effects/card_shadow_pile-yPF2eQ@2x.png'
const files = Object.keys(import.meta.glob('/public/assets/cards/*.png')).map(p => p.replace('/public', '')).filter(p => !p.endsWith('/card_overlay.png'))

// Presentation consumes accepted snapshots only. No command sender or rule logic lives here.
export class TableScene {
 private app = new Application()
 private table = new Container({ eventMode: 'none' })
 private opponents = new Container({ eventMode: 'none' })
 private clippedHand = new Container({ eventMode: 'none' })
 private hand = new Container({ sortableChildren: true, eventMode: 'none' })
 private mask = new Graphics()
 private flights = new Container({ eventMode: 'none' })
 private cards = new Map<string, CardNode>()
 private textures: Record<string, Texture> = {}
 private previous?: GameView
 private selected = ''
 private animations = new Set<gsap.core.Animation>()
 private moving = new Map<Sprite, HTMLElement>()
 private observer?: ResizeObserver
 private scroll?: HTMLElement
 private disposed = false
 private initialized = false
 private context: gsap.Context
 private reduced = window.matchMedia('(prefers-reduced-motion: reduce)')
 private deck?: Position
 private discard?: Position
 private discardFace?: Sprite
 private pendingDiscard = ''
 private discardHistory: {id:string; source:string}[] = []
 private players = new Map<string, Position>()
 private pendingArrivals = new Map<string, Card>()
 private controlledScroll = false
 constructor(private root: HTMLElement, private asset: (card: Card, color?: number) => string) {
  this.context = gsap.context(() => {}, root)
 }
 async init() {
  // Keep the application's existing strict CSP: texture decoding needs no blob worker.
  Assets.setPreferences({ preferWorkers: false })
  await this.app.init({ width: this.root.clientWidth, height: this.root.clientHeight, backgroundAlpha: 0, resolution: Math.min(window.devicePixelRatio || 1, 2), autoDensity: true, antialias: true, preference: 'webgl', sharedTicker: false, eventFeatures: { move: false, globalMove: false, click: false, wheel: false } })
  this.initialized = true
  if (this.disposed) { this.destroyApp(); return }
  this.textures = await Assets.load<Texture>([...files, back, arrow, shadow])
  if (this.disposed) return
  this.app.canvas.className = 'table-scene'
  this.app.canvas.setAttribute('aria-hidden', 'true')
  this.root.prepend(this.app.canvas)
  this.clippedHand.addChild(this.hand)
  this.app.stage.addChild(this.opponents, this.table, this.clippedHand, this.mask, this.flights)
  this.clippedHand.mask = this.mask
  this.app.stage.eventMode = 'none'
  this.observer = new ResizeObserver(this.resize)
  this.observer.observe(this.root)
  this.scroll = this.root.querySelector<HTMLElement>('.hand') ?? undefined
  this.scroll?.addEventListener('scroll', this.onScroll, { passive: true })
  this.scroll?.addEventListener('pointerdown', this.interruptScroll, { passive: true })
  this.scroll?.addEventListener('wheel', this.interruptScroll, { passive: true })
  this.reduced.addEventListener('change', this.motionPreference)
  this.root.dataset.sceneReady = 'true'
 }
 private rect(node: Element | null): Position | undefined {
  if (!node) return
  const b = node.getBoundingClientRect(), r = this.root.getBoundingClientRect()
  return { x: b.left - r.left + b.width / 2, y: b.top - r.top + b.height / 2, width: b.width, height: b.height }
 }
 private sprite(src: string, p: Position, layer: Container) {
  const sprite = new Sprite({ texture: this.textures[src], anchor: .5, x: p.x, y: p.y, eventMode: 'none' })
  sprite.setSize(p.width, p.height)
  layer.addChild(sprite)
  return sprite
 }
 private positionFor(id: string): Position | undefined {
  const button = [...this.root.querySelectorAll<HTMLElement>('[data-card-id]')].find(n => n.dataset.cardId === id)
  if (!button) return
  const r = this.rect(button)!
  const width = Number(button.dataset.cardWidth)
  return { ...r, x: r.x - r.width / 2 + width / 2, width, height: width * 344 / 256 }
 }
 private tween(target: object, vars: gsap.TweenVars) {
  let tween!: gsap.core.Tween
  this.context.add(() => {
   tween = gsap.to(target, { ...vars, onComplete: () => { this.animations.delete(tween); vars.onComplete?.() } })
   this.animations.add(tween)
  })
 }
 private stop() {
  // Kill the scoped Pixi tweens before destroying sprites. Revert is only safe for live DOM targets.
  this.context.kill()
  this.controlledScroll = false
  this.pendingArrivals.clear()
  this.animations.forEach(a => a.kill()); this.animations.clear()
  this.moving.forEach((marker, sprite) => { marker.remove(); sprite.destroy() }); this.moving.clear()
  this.cards.forEach(c => { c.group.visible = true })
  delete this.root.dataset.motionActive
  this.pendingDiscard = ''
  if(this.discardFace && !this.discardFace.destroyed) this.discardFace.visible = true
 }
 private destroyApp() {
  if (this.initialized) { this.initialized=false; this.clippedHand.mask = null; this.app.destroy({ removeView: true, releaseGlobalResources: true }, { children: true }) }
 }
 destroy() {
  if(this.disposed && !this.initialized)return
  this.disposed = true
  this.observer?.disconnect()
  this.scroll?.removeEventListener('scroll', this.onScroll)
  this.scroll?.removeEventListener('pointerdown', this.interruptScroll)
  this.scroll?.removeEventListener('wheel', this.interruptScroll)
  this.reduced.removeEventListener('change', this.motionPreference)
  this.stop(); this.context.revert()
  delete this.root.dataset.sceneReady
  if (this.initialized) this.destroyApp()
 }
 private resize = () => {
  if (this.disposed || !this.previous) return
  // ResizeObserver always emits once after observe(). That is not a resize:
  // cancelling here would erase every initial deal before its first rendered frame.
  if(this.app.screen.width===this.root.clientWidth && this.app.screen.height===this.root.clientHeight) return
  this.stop()
  this.app.renderer.resize(this.root.clientWidth, this.root.clientHeight)
  this.layout(this.previous, false)
 }
 private motionPreference = () => {
  if(this.disposed || !this.previous)return
  this.stop();this.layout(this.previous,false)
 }
 private onScroll = () => {
  if (!this.previous || this.disposed) return
  if(this.controlledScroll) { this.layout(this.previous, false); return }
  // Scrolling invalidates flight destinations; compose the current state immediately.
  this.stop(); this.layout(this.previous, false)
 }
 private interruptScroll = () => {
  if(this.controlledScroll && this.previous) { this.stop(); this.layout(this.previous, false) }
 }
 private arrivals(cards: Card[]) {
  const ready = cards.filter(c => this.cards.has(c.ID))
  cards.filter(c => !this.cards.has(c.ID)).forEach(c => this.pendingArrivals.set(c.ID,c))
  if(!ready.length) return
  ready.forEach(c=>{this.cards.get(c.ID)!.group.visible=false})
  const hand = this.scroll, region = this.rect(hand ?? null)
  const bottom = Math.max(...ready.map(c => { const p=this.cards.get(c.ID)!.target; return p.y+p.height/2 }))
  const launch = () => ready.forEach((card,i) => {
   // A newer accepted state may already have removed this card during the scroll.
   if(!this.previous?.hand.some(c=>c.Card.ID===card.ID)) return
   const node=this.cards.get(card.ID)
   this.fly(this.deck,node?.target,this.asset(card),'draw',i*.13,node,undefined,card.ID)
  })
  if(hand && region && bottom > region.y+region.height/2+1) {
   const target = Math.min(hand.scrollHeight-hand.clientHeight,hand.scrollTop+bottom-region.y-region.height/2+8)
   this.controlledScroll = true
   // Scroll only the hand, then land at the measured, visible destination.
   this.cards.forEach(c=>gsap.killTweensOf(c.group))
   this.tween(hand,{scrollTop:target,duration:.35,ease:'power2.inOut',onUpdate:()=>{if(this.previous)this.layout(this.previous,false)},onComplete:()=>{ launch();requestAnimationFrame(()=>{this.controlledScroll=false}) }})
  } else launch()
 }
 private layout(view: GameView, animate: boolean) {
  this.deck = this.rect(this.root.querySelector('.draw'))
  this.discard = this.rect(this.root.querySelector('.discard'))
  this.players.clear()
  this.root.querySelectorAll<HTMLElement>('[data-player-key]').forEach(n => { const p = this.rect(n.querySelector('.avatar') ?? n); if(p) this.players.set(n.dataset.playerKey!, p) })
  this.table.removeChildren().forEach(c => c.destroy({ children: true }))
  if (this.deck) {
   this.sprite(shadow, { ...this.deck, y: this.deck.y + 14, width: this.deck.width * 1.3, height: this.deck.height * 1.3 }, this.table)
   if(view.my_turn && !view.closed) this.table.addChild(new Graphics().roundRect(this.deck.x - this.deck.width / 2 - 7, this.deck.y - this.deck.height / 2 + 2, this.deck.width + 14, this.deck.height + 14, 18).fill(0x0789c6))
   for (let i=3;i>=0;i--) this.sprite(back, { ...this.deck, y: this.deck.y + i * 3 }, this.table)
  }
  if (view.top && this.discard) {
   const pile=this.discard
   this.sprite(shadow, { ...this.discard, y:this.discard.y+12, width:this.discard.width*1.25, height:this.discard.height*1.25 }, this.table)
   this.discardHistory.forEach((entry,index) => {
    const depth=this.discardHistory.length-1-index
    const card=this.sprite(entry.source,{...pile,x:pile.x+(depth%2 ? -3 : 2)*depth,y:pile.y+depth*2},this.table)
    card.label=entry.id
    card.rotation=depth ? (depth%2 ? .045 : -.085) : -.052
    if(!depth) { this.discardFace=card;card.visible=this.pendingDiscard!==entry.id }
   })
   this.root.dataset.discardAsset = this.discardHistory.at(-1)?.source ?? ''
   this.root.dataset.discardLayers = String(this.discardHistory.length)
   this.root.dataset.discardVisibleLayers = String(this.discardHistory.length-(this.pendingDiscard===view.top.ID ? 1 : 0))
  }
  this.root.querySelectorAll('.table-orbit .arrow').forEach((node, i) => {
   const p = this.rect(node)
   if (!p) return
   const s = this.sprite(arrow, p, this.table)
   s.scale.y *= -1
   if (i) s.rotation = Math.PI
   if (view.direction === -1) s.scale.x *= -1
   s.alpha = .87
  })
  this.opponents.removeChildren().forEach(c => c.destroy())
  const others = view.players.filter(p => !p.me && (p.active || view.closed))
  if (others.length === 1) {
   const count = Math.min(others[0].count, 10), w = 79, stride = Math.min(51, (this.root.clientWidth - w) / Math.max(1, count - 1))
   for (let i=0;i<count;i++) this.sprite(back, { x: this.root.clientWidth / 2 + (i - (count - 1)/2) * stride, y: 18, width: w, height: w * 344 / 256 }, this.opponents)
  }
  const area = this.root.querySelector('.hand')
  const region = this.rect(area)
  if (region) this.mask.clear().rect(region.x - region.width / 2, region.y - region.height / 2, region.width, region.height).fill(0xffffff)
  const ids = new Set(view.hand.map(c => c.Card.ID))
  this.cards.forEach((c, id) => { if(!ids.has(id)) { gsap.killTweensOf(c.group); gsap.killTweensOf(c.lift); c.group.destroy({ children: true }); this.cards.delete(id) } })
  view.hand.forEach(c => {
   const p = this.positionFor(c.Card.ID)
   if (!p) return
   let node = this.cards.get(c.Card.ID)
   if (!node) {
    const group = new Container({ label:c.Card.ID, x: p.x, y: p.y }), lift = new Container()
    const face = this.sprite(this.asset(c.Card), {x:0,y:0,width:p.width,height:p.height}, lift)
    group.addChild(lift); this.hand.addChild(group)
    node = { group, lift, face, target: p }; this.cards.set(c.Card.ID, node)
   }
   node.face.texture = this.textures[this.asset(c.Card)]
   const button = this.root.querySelector<HTMLElement>(`[data-card-id="${CSS.escape(c.Card.ID)}"]`)
   if(button) button.dataset.cardAsset = this.asset(c.Card)
   node.face.setSize(p.width, p.height)
   node.face.tint = view.my_turn && c.Playable && !view.closed ? 0xffffff : 0x6b6b6b
   node.group.zIndex = c.Card.ID === this.selected ? 1000 : Number(this.root.querySelector<HTMLElement>(`[data-card-id="${CSS.escape(c.Card.ID)}"]`)?.dataset.slot || 0)
   const changed = Math.abs(node.target.x - p.x) > .5 || Math.abs(node.target.y - p.y) > .5
   node.target = p
   if(!animate && !gsap.isTweening(node.lift)) node.lift.y = c.Card.ID === this.selected ? -18 : 0
   if (animate && changed) this.tween(node.group, { pixi: { x: p.x, y: p.y }, duration: .58, ease: 'power2.inOut', overwrite: 'auto' })
   else if (!animate && !gsap.isTweening(node.group)) node.group.position.set(p.x, p.y)
  })
 }
 private fly(from: Position | undefined, to: Position | undefined, front: string | undefined, kind: string, delay: number, destination?: CardNode, onLand?: () => void, cardID?: string) {
  if (!from || !to) return
  const duration = kind === 'play' ? .56 : .78
  const sprite = this.sprite(kind === 'play' && front ? front : back, { ...from, width: Math.min(95, from.width), height: Math.min(95, from.width) * 344 / 256 }, this.flights)
  sprite.visible = false
  if(cardID)sprite.label=cardID
  const marker = document.createElement('span')
  marker.hidden = true; marker.dataset.motionKind = kind
  marker.dataset.motionPrivate = String(!!front && kind !== 'play')
  if(cardID)marker.dataset.flightCardId=cardID
  this.root.append(marker); this.moving.set(sprite, marker)
  this.root.dataset.motionActive = 'true'
  if (destination) destination.group.visible = false
  this.context.add(() => {
   const progress = { value: 0 }, flip = { angle: 0 }
   const startScale = sprite.scale.x, endScale = to.width / sprite.texture.width
   const swap = front?.endsWith('/swap_hands.png')
   const startWidth=sprite.width, startHeight=sprite.height
   const timeline = gsap.timeline({ onComplete: () => {
    if (destination && !destination.group.destroyed) destination.group.visible = true
    onLand?.()
    marker.remove(); this.moving.delete(sprite); sprite.destroy(); this.animations.delete(timeline)
    if (!this.moving.size) delete this.root.dataset.motionActive
   } })
   this.animations.add(timeline)
   timeline.set(sprite, { visible: true }, delay)
   // A bowed path with acceleration/deceleration, small anticipation and a quiet landing.
   timeline.to(sprite, { y: from.y - 7, rotation: -.06, duration: .08, ease: 'power1.out' }, delay)
   timeline.to(progress, { value: 1, duration: duration - .08, ease: 'power2.inOut', onUpdate: () => {
    const t = progress.value
    sprite.x = from.x + (to.x - from.x) * t
    sprite.y = from.y + (to.y - from.y) * t - 7 * (1-t) - Math.sin(Math.PI * t) * Math.min(38, Math.abs(to.y - from.y) * .12)
    sprite.rotation = -.06 * (1 - t) + (kind === 'play' ? -.052 * t : 0)
    const size = startScale + (endScale - startScale) * t
    sprite.scale.y = size
    sprite.scale.x = size * (front && kind !== 'play' ? Math.max(.015, Math.abs(Math.cos(flip.angle))) : 1)
    if(swap) {
     // This original bot asset has different native dimensions from the v2 pack.
     sprite.scale.x=(startWidth+(to.width-startWidth)*t)/sprite.texture.width*(kind==='play'?1:Math.max(.015,Math.abs(Math.cos(flip.angle))))
     sprite.scale.y=(startHeight+(to.height-startHeight)*t)/sprite.texture.height
    }
    if(destination && flip.angle >= Math.PI/2) sprite.tint=destination.face.tint
   } }, delay + .08)
   if (front && kind !== 'play') {
    timeline.to(flip, { angle: Math.PI, duration: .5, ease: 'sine.inOut' }, delay + .2)
    timeline.set(sprite, { texture: this.textures[front] }, delay + .45)
   }
   timeline.to(sprite.scale, { x: swap ? to.width/this.textures[front!].width : endScale, y: swap ? to.height/this.textures[front!].height : endScale, duration: .08, ease: 'sine.out' }, delay + duration)
  })
 }
 dealInitial() {
  const view=this.previous
  if(!view || this.reduced.matches || view.closed || view.hand.length!==7 || view.players.some(p=>p.count!==7)) return
  const seats = view.players.filter(p => p.active), interval = seats.length <= 2 ? .09 : .025
  for(let round=0;round<7;round++) seats.forEach((p, index) => {
   const card = p.me ? view.hand[round] : undefined, node = card && this.cards.get(card.Card.ID)
   this.fly(this.deck, node?.target ?? this.players.get(p.key), card ? this.asset(card.Card) : undefined, 'deal', (round * seats.length + index) * interval, node,undefined,card?.Card.ID)
  })
 }
 sync(view: GameView, selected: string, baseline?: GameView) {
  if(this.disposed || !this.root.dataset.sceneReady) return
  const before = this.previous ?? baseline
  if(before?.game_id === view.game_id && view.revision < before.revision) return
  const advanced = !!before && view.revision > before.revision
  const events = (view.events ?? []).filter(e => before && e.revision > before.revision && e.revision <= view.revision)
  const contiguous = !!before && (view.revision <= before.revision + 1 || events.some(e => e.revision === before.revision + 1))
  const motion = advanced && contiguous && !view.recovery && !this.reduced.matches
  const oldPositions = new Map([...this.cards].map(([id, node]) => [id, { ...node.target, x: node.group.x, y: node.group.y + node.lift.y }]))
  const oldPlayers = new Map(this.players)
  if(view.recovery || (advanced && !contiguous) || this.reduced.matches) this.stop()
  if(before?.game_id !== view.game_id) this.discardHistory=[]
  const remember=(snapshot:GameView) => {
   if(!snapshot.top)return
   // A newly played wild still awaiting its owner's choice must remain neutral.
   const source=this.asset(snapshot.top,snapshot.phase===2 ? undefined : snapshot.active_color)
   const last=this.discardHistory.at(-1)
   if(last?.id===snapshot.top.ID) last.source=source
   else {
    this.discardHistory=this.discardHistory.filter(c=>c.id!==snapshot.top!.ID)
    this.discardHistory.push({id:snapshot.top.ID,source})
    this.discardHistory=this.discardHistory.slice(-5)
   }
  }
  if(!this.discardHistory.length && before?.game_id===view.game_id)remember(before)
  remember(view)
  this.previous = view
  this.layout(view, motion)
  if(motion) {
   const started = events.some(e => e.type === 'game_started')
   if(started) {
    this.dealInitial()
   } else {
    const incoming = view.hand.filter(c => !before!.hand.some(b => b.Card.ID === c.Card.ID))
    for(const event of events) {
     const player = view.players.find(p => p.key === event.player)
     if(event.type === 'card_played' && view.top && event.card_id && view.top.ID === event.card_id) {
      this.pendingDiscard = event.card_id
      if(this.discardFace) this.discardFace.visible = false
      this.fly(player?.me ? oldPositions.get(event.card_id) : oldPlayers.get(event.player), this.discard, this.asset(view.top), 'play', 0, undefined, () => {
       if(this.pendingDiscard === event.card_id) { this.pendingDiscard = ''; if(this.discardFace && !this.discardFace.destroyed) this.discardFace.visible = true }
       this.root.dataset.discardVisibleLayers=String(this.discardHistory.length-(this.pendingDiscard ? 1 : 0))
      },event.card_id)
      this.root.dataset.discardVisibleLayers=String(this.discardHistory.length-1)
     }
     if(event.type === 'cards_drawn' && event.count) {
      if(player?.me) this.arrivals(incoming.splice(0,event.count).map(c=>c.Card))
      else for(let i=0;i<event.count;i++) this.fly(this.deck, this.players.get(event.player), undefined, 'draw', i * .13)
     }
    }
   }
  }
  this.pendingArrivals.forEach((c,id)=>{if(!view.hand.some(card=>card.Card.ID===c.ID))this.pendingArrivals.delete(id)})
  const deferred=[...this.pendingArrivals.values()].filter(c=>this.cards.has(c.ID))
  deferred.forEach(c=>this.pendingArrivals.delete(c.ID))
  if(deferred.length)this.arrivals(deferred)
  if(selected !== this.selected || advanced || view.recovery) {
   this.selected = selected
   this.cards.forEach((node,id) => {
    node.group.zIndex = id === selected ? 1000 : Number(this.root.querySelector<HTMLElement>(`[data-card-id="${CSS.escape(id)}"]`)?.dataset.slot || 0)
    this.tween(node.lift, { pixi: { y: id === selected ? -18 : 0 }, duration: this.reduced.matches ? 0 : .38, ease: 'power2.out', overwrite: 'auto', onUpdate: () => { const button=this.root.querySelector<HTMLElement>(`[data-card-id="${CSS.escape(id)}"]`); if(button) button.dataset.visualLift=String(node.lift.y) } })
   })
  }
 }
}
