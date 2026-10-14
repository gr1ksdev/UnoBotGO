import { Application, Assets, Container, Graphics, PerspectiveMesh, Sprite, Texture, type Spritesheet } from 'pixi.js'
import 'pixi.js/unsafe-eval'
import { gsap } from 'gsap'
import { PixiPlugin } from 'gsap/PixiPlugin'
import * as PIXI from 'pixi.js'
gsap.registerPlugin(PixiPlugin)
PixiPlugin.registerPIXI(PIXI)
import type { Card, GameView } from '../../api/client'
import { cardCorners } from '../../lib/cardMotion'
import { dragPose, releasedOnDiscard } from '../../lib/cardGesture'
import { tableFeedback } from '../../lib/tableFeedback'

type Position = { x: number; y: number; width: number; height: number }
type CardNode = { group: Container; face: Sprite; shadow: Sprite; lift: Container; target: Position }
const back = '/assets/backs/default.png'
const arrow = '/assets/effects/direction_arrow-ohEFIQ@2x.png'
const shadow = '/assets/effects/card_shadow_pile-yPF2eQ@2x.png'
const cardShadow = '/assets/effects/card_shadow-ZiLvNA@2x.png'
const skipHighlight = '/assets/effects/skip_highlight-IGDjwA@2x.png'
const playerHighlight = '/assets/effects/player_highlight-6TgerA@2x.png'
const cardAtlas = '/assets/atlases/cards-cxSAsg@2x.png.json'
const swap = '/assets/cards/swap_hands.png'

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
 private moving = new Map<Container, HTMLElement>()
 private observer?: ResizeObserver
 private scroll?: HTMLElement
 private disposed = false
 private initialized = false
 private renderDirty = false
 private renderSubscribed = false
 private tableLayoutKey = ''
 private maskLayoutKey = ''
 private buttons = new Map<string, HTMLElement>()
 private context: gsap.Context
 private reduced = window.matchMedia('(prefers-reduced-motion: reduce)')
 private deck?: Position
 private deckVisual?: Container
 private discard?: Position
 private discardFace?: Sprite
 private pendingDiscard = ''
 private discardHistory: {id:string; source:string}[] = []
 private players = new Map<string, Position>()
 private pendingArrivals = new Map<string, {card:Card;from?:Position;kind:string}>()
 private controlledScroll = false
 private drag?: { id: string; node: CardNode; button: HTMLElement; pointer: number; startX: number; startY: number; origin: Position; moved: boolean }
 private swallowClick = false
 private hovered = ''
 private deckPressed = false
 constructor(private root: HTMLElement, private asset: (card: Card, color?: number) => string, private release?: (id: string) => void) {
  this.context = gsap.context(() => {}, root)
 }
 async init() {
  // Keep the application's existing strict CSP: texture decoding needs no blob worker.
  Assets.setPreferences({ preferWorkers: false })
  await this.app.init({ width: this.root.clientWidth, height: this.root.clientHeight, backgroundAlpha: 0, resolution: Math.min(window.devicePixelRatio || 1, 2), autoDensity: true, antialias: true, preference: 'webgl', sharedTicker: false, autoStart: false, eventFeatures: { move: false, globalMove: false, click: false, wheel: false } })
  this.initialized = true
  if (this.disposed) { this.destroyApp(); return }
  const [atlas, extras] = await Promise.all([
   Assets.load<Spritesheet>(cardAtlas),
   Assets.load<Texture>([swap, back, arrow, shadow, cardShadow, skipHighlight, playerHighlight]),
  ])
  this.textures = extras
  for(const [name,texture] of Object.entries(atlas.textures)) {
   if(name !== 'card_overlay.png') this.textures[`/assets/cards/${name}`] = texture
  }
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
  this.root.addEventListener('pointerdown', this.pointerDown)
  this.root.addEventListener('pointermove', this.pointerMove)
  this.root.addEventListener('pointerup', this.pointerUp)
  this.root.addEventListener('pointercancel', this.pointerCancel)
  this.root.addEventListener('lostpointercapture', this.pointerCancel)
  this.root.addEventListener('click', this.captureClick, true)
  this.root.addEventListener('pointerover', this.pointerOver)
  this.root.addEventListener('pointerout', this.pointerOut)
  this.root.dataset.sceneReady = 'true'
  this.root.dataset.cardAtlas = cardAtlas
 }
 private rect(node: Element | null, r = this.root.getBoundingClientRect()): Position | undefined {
  if (!node) return
  const b = node.getBoundingClientRect()
  return { x: b.left - r.left + b.width / 2, y: b.top - r.top + b.height / 2, width: b.width, height: b.height }
 }
 private sprite(src: string, p: Position, layer: Container) {
  const sprite = new Sprite({ texture: this.textures[src], anchor: .5, x: p.x, y: p.y, eventMode: 'none' })
  sprite.setSize(p.width, p.height)
  layer.addChild(sprite)
  return sprite
 }
 private positionFor(id: string, origin = this.root.getBoundingClientRect()): Position | undefined {
  const button = this.buttons.get(id)
  if (!button) return
  const r = this.rect(button, origin)!
  const width = Number(button.dataset.cardWidth)
  return { ...r, x: r.x - r.width / 2 + width / 2, width, height: width * 344 / 256 }
 }
 private tween(target: object, vars: gsap.TweenVars) {
  let tween!: gsap.core.Tween
  this.context.add(() => {
   tween = gsap.to(target, { ...vars, onUpdate: () => { vars.onUpdate?.(); this.requestRender() }, onInterrupt: () => { this.animations.delete(tween); vars.onInterrupt?.() }, onComplete: () => { this.animations.delete(tween); vars.onComplete?.(); this.requestRender() } })
   if(tween.isActive() || tween.totalProgress()<1) this.animations.add(tween)
  })
 }
 // GSAP calls non-prioritized listeners after updating its timeline. Flush once
 // in that same browser frame instead of chaining a second requestAnimationFrame.
 private requestRender() {
  if(this.disposed || !this.initialized) return
  this.renderDirty = true
  if(!this.renderSubscribed) {
   this.renderSubscribed = true
   gsap.ticker.add(this.flushRender)
  }
 }
 private flushRender = () => {
  if(this.disposed || !this.initialized) { this.detachRender(); return }
  if(this.renderDirty) {
   this.renderDirty = false
   this.app.render()
  }
  if(!this.renderDirty && !this.animations.size) this.detachRender()
 }
 private detachRender() {
  if(this.renderSubscribed) gsap.ticker.remove(this.flushRender)
  this.renderSubscribed = false
 }
 private stop() {
  this.cancelDrag(true)
  // Kill the scoped Pixi tweens before destroying sprites. Revert is only safe for live DOM targets.
  this.context.kill()
  this.controlledScroll = false
  this.pendingArrivals.clear()
  this.animations.forEach(a => a.kill()); this.animations.clear()
  this.deckVisual?.scale.set(1)
  this.table.children.filter(c=>c.label==='direction-arrow').forEach(c=>{c.alpha=.87})
  this.moving.forEach((marker, flight) => { marker.remove(); flight.destroy({ children: true }) }); this.moving.clear()
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
  this.detachRender(); this.renderDirty = false
  this.buttons.clear()
  this.observer?.disconnect()
  this.scroll?.removeEventListener('scroll', this.onScroll)
  this.scroll?.removeEventListener('pointerdown', this.interruptScroll)
  this.scroll?.removeEventListener('wheel', this.interruptScroll)
  this.reduced.removeEventListener('change', this.motionPreference)
  this.root.removeEventListener('pointerdown', this.pointerDown)
  this.root.removeEventListener('pointermove', this.pointerMove)
  this.root.removeEventListener('pointerup', this.pointerUp)
  this.root.removeEventListener('pointercancel', this.pointerCancel)
  this.root.removeEventListener('lostpointercapture', this.pointerCancel)
  this.root.removeEventListener('click', this.captureClick, true)
  this.root.removeEventListener('pointerover', this.pointerOver)
  this.root.removeEventListener('pointerout', this.pointerOut)
  this.stop(); this.context.revert()
  delete this.root.dataset.sceneReady
  delete this.root.dataset.cardAtlas
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
 private hitButton(event: PointerEvent) {
  const button = (event.target as Element | null)?.closest<HTMLButtonElement>('[data-card-id]')
  return button && this.root.contains(button) && !button.disabled ? button : undefined
 }
 private pointerDown = (event: PointerEvent) => {
  this.swallowClick = false
  if((event.target as Element | null)?.closest<HTMLButtonElement>('.draw:not(:disabled)')) { this.deckPressed=true;this.bounceDeck(.96,.12) }
  const button = this.hitButton(event), id = button?.dataset.cardId
  if(event.button !== 0 || !event.isPrimary || !id || id !== this.selected || this.root.dataset.inputEnabled !== 'true') return
  const node = this.cards.get(id)
  if(!node || !node.group.visible || this.drag) return
  this.drag = { id, node, button: button!, pointer: event.pointerId, startX: event.clientX, startY: event.clientY, origin: { ...node.target, x: node.group.x, y: node.group.y }, moved: false }
  button!.setPointerCapture(event.pointerId)
 }
 private pointerMove = (event: PointerEvent) => {
  const drag = this.drag
  if(!drag || drag.pointer !== event.pointerId) return
  const dx = event.clientX - drag.startX, dy = event.clientY - drag.startY
  if(!drag.moved && Math.hypot(dx, dy) < 8) return
  event.preventDefault()
  if(!drag.moved) {
   drag.moved = true
   gsap.killTweensOf([drag.node.group, drag.node.lift, drag.node.lift.scale])
   this.flights.addChild(drag.node.group)
   this.root.dataset.dragCardId = drag.id
   tableFeedback.play('select', true)
  }
  const pose = dragPose({x:dx,y:dy}), node = drag.node
  // Pointer tracking is immediate; easing belongs to release/return, not input.
  node.group.position.set(drag.origin.x + dx, drag.origin.y + dy)
  node.group.rotation = pose.rotation
  node.lift.y = -18-pose.lift
  this.handShadow(node)
  this.requestRender()
 }
 private pointerUp = (event: PointerEvent) => {
  if(this.deckPressed) { this.deckPressed=false;this.bounceDeck(1,.45) }
  const drag = this.drag
  if(!drag || drag.pointer !== event.pointerId) return
  const rect = this.root.getBoundingClientRect()
  const end = { x: event.clientX - rect.left, y: event.clientY - rect.top }
  const start = { x: drag.startX - rect.left, y: drag.startY - rect.top }
  const play = drag.moved && !!this.discard && releasedOnDiscard(start, end, this.discard) && this.root.dataset.inputEnabled === 'true' && !drag.button.hasAttribute('disabled')
  this.swallowClick = drag.moved
  const id = drag.id
  this.cancelDrag(false)
  if(play) this.release?.(id)
 }
 private pointerCancel = (event: PointerEvent) => {
  if(this.deckPressed) { this.deckPressed=false;this.bounceDeck(1,.35) }
  if(this.drag?.pointer === event.pointerId) { this.swallowClick = this.drag.moved; this.cancelDrag(false) }
 }
 private captureClick = (event: MouseEvent) => {
  if(this.swallowClick && (event.target as Element | null)?.closest('[data-card-id]')) {
   event.preventDefault(); event.stopImmediatePropagation(); this.swallowClick = false
  }
 }
 private cancelDrag(immediate: boolean) {
  const drag = this.drag
  if(!drag) return
  this.drag = undefined
  delete this.root.dataset.dragCardId
  if(drag.button.hasPointerCapture(drag.pointer)) drag.button.releasePointerCapture(drag.pointer)
  const node = drag.node
  gsap.killTweensOf([node.group, node.lift])
  this.hand.addChild(node.group)
  if(immediate) { node.group.position.set(node.target.x,node.target.y); node.group.rotation=0; node.lift.y = drag.id === this.selected ? -18 : 0; this.handShadow(node) }
  else if(drag.moved) {
   tableFeedback.play('return')
   this.tween(node.group, { x: node.target.x, y: node.target.y, rotation: 0, duration: this.reduced.matches ? 0 : .5, ease:'elastic.out(.65,1)', overwrite:true })
   this.tween(node.lift, { y: drag.id === this.selected ? -18 : 0, duration: this.reduced.matches ? 0 : .35, ease:'power2.out', overwrite:true, onUpdate:()=>this.handShadow(node) })
  }
  this.requestRender()
 }
 private pointerOver = (event: PointerEvent) => {
  if(event.pointerType !== 'mouse' || this.drag) return
  if((event.target as Element | null)?.closest('.draw:not(:disabled)')) this.bounceDeck(1.035,.4)
  const button = this.hitButton(event), id = button?.dataset.cardId
  if(!id || id === this.selected || id === this.hovered || this.root.dataset.inputEnabled !== 'true') return
  this.hovered = id
  const node = this.cards.get(id)
  if(node) { tableFeedback.play('hover'); this.tween(node.lift,{y:-10,duration:.25,ease:'power2.out',overwrite:true,onUpdate:()=>this.handShadow(node)}) }
 }
 private pointerOut = (event: PointerEvent) => {
  if((event.target as Element | null)?.closest('.draw')) this.bounceDeck(1,.35)
  const id = this.hitButton(event)?.dataset.cardId
  if(!id || id !== this.hovered) return
  this.hovered = ''
  const node = this.cards.get(id)
  if(node && id !== this.selected && !this.drag) this.tween(node.lift,{y:0,duration:.3,ease:'elastic.out(.65,1)',overwrite:true,onUpdate:()=>this.handShadow(node)})
 }
 private bounceDeck(scale: number, duration: number) {
  if(this.deckVisual && !this.deckVisual.destroyed) this.tween(this.deckVisual.scale,{x:scale,y:scale,duration:this.reduced.matches ? 0 : duration,ease:'elastic.out(.65,.7)',overwrite:true})
 }
 private arrivals(cards: Card[], from = this.deck, kind = 'draw') {
  this.cancelDrag(true)
  const ready = cards.filter(c => this.cards.has(c.ID))
  cards.filter(c => !this.cards.has(c.ID)).forEach(card => this.pendingArrivals.set(card.ID,{card,from,kind}))
  if(!ready.length) return
  ready.forEach(c=>{this.cards.get(c.ID)!.group.visible=false})
  const hand = this.scroll, region = this.rect(hand ?? null)
  const bottom = Math.max(...ready.map(c => { const p=this.cards.get(c.ID)!.target; return p.y+p.height/2 }))
  const launch = () => ready.forEach((card,i) => {
   // A newer accepted state may already have removed this card during the scroll.
   if(!this.previous?.hand.some(c=>c.Card.ID===card.ID)) return
   const node=this.cards.get(card.ID)
   this.fly(from,node?.target,this.asset(card),kind,i*.13,node,undefined,card.ID)
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
  const origin = this.root.getBoundingClientRect()
  this.buttons = new Map([...this.root.querySelectorAll<HTMLElement>('[data-card-id]')].map(n=>[n.dataset.cardId!,n]))
  this.deck = this.rect(this.root.querySelector('.draw'), origin)
  this.discard = this.rect(this.root.querySelector('.discard'), origin)
  this.players.clear()
  this.root.querySelectorAll<HTMLElement>('[data-player-key]').forEach(n => { const p = this.rect(n.querySelector('.avatar') ?? n, origin); if(p) this.players.set(n.dataset.playerKey!, p) })
  const others = view.players.filter(p => !p.me && (p.active || view.closed))
  const arrows = [...this.root.querySelectorAll('.table-orbit .arrow')].map(n=>this.rect(n,origin))
  const staticKey = JSON.stringify([this.deck,this.discard,arrows,this.root.clientWidth,others.map(p=>[p.key,p.count]),view.my_turn,view.closed,view.phase,view.drawn_card_id,view.direction,this.discardHistory])
  if(staticKey !== this.tableLayoutKey) {
   this.tableLayoutKey = staticKey
   this.table.removeChildren().forEach(c => { gsap.killTweensOf(c); gsap.killTweensOf(c.scale); c.destroy({ children: true }) })
   this.deckVisual = undefined
   if (this.deck) {
    const stack = new Container({x:this.deck.x,y:this.deck.y,eventMode:'none'})
    this.deckVisual=stack;this.table.addChild(stack)
    const p={...this.deck,x:0,y:0}
    this.sprite(shadow, { ...p, y:14, width:p.width*1.3,height:p.height*1.3 }, stack)
    if(view.my_turn && !view.closed && view.phase===1 && !view.drawn_card_id) stack.addChild(new Graphics().roundRect(-p.width/2-7,-p.height/2+2,p.width+14,p.height+14,18).fill(0x0789c6))
    for (let i=3;i>=0;i--) this.sprite(back,{...p,y:i*3},stack)
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
   arrows.forEach((p, i) => {
    if (!p) return
    const s = this.sprite(arrow, p, this.table)
    s.label = 'direction-arrow'
    s.scale.y *= -1
    if (i) s.rotation = Math.PI
    if (view.direction === -1) s.scale.x *= -1
    s.alpha = .87
   })
   this.opponents.removeChildren().forEach(c => c.destroy())
   if (others.length === 1) {
    const count = Math.min(others[0].count, 10), w = 79, stride = Math.min(51, (this.root.clientWidth - w) / Math.max(1, count - 1))
    for (let i=0;i<count;i++) this.sprite(back, { x: this.root.clientWidth / 2 + (i - (count - 1)/2) * stride, y: 18, width: w, height: w * 344 / 256 }, this.opponents)
   }
  }
  const area = this.root.querySelector('.hand')
  const region = this.rect(area, origin)
  const maskKey = JSON.stringify(region)
  if (region && maskKey !== this.maskLayoutKey) {
   this.maskLayoutKey = maskKey
   this.mask.clear().rect(region.x - region.width / 2, region.y - region.height / 2, region.width, region.height).fill(0xffffff)
  }
  const ids = new Set(view.hand.map(c => c.Card.ID))
  this.cards.forEach((c, id) => { if(!ids.has(id)) { gsap.killTweensOf([c.group,c.lift,c.lift.scale,c.face,c.shadow]); c.group.destroy({ children: true }); this.cards.delete(id) } })
  view.hand.forEach(c => {
   const p = this.positionFor(c.Card.ID, origin)
   if (!p) return
   let node = this.cards.get(c.Card.ID)
   if (!node) {
    const group = new Container({ label:c.Card.ID, x: p.x, y: p.y }), lift = new Container()
    const castShadow = this.sprite(cardShadow, {x:0,y:6,width:p.width*1.1,height:p.height*1.1}, lift)
    const face = this.sprite(this.asset(c.Card), {x:0,y:0,width:p.width,height:p.height}, lift)
    group.addChild(lift); this.hand.addChild(group)
    node = { group, lift, face, shadow:castShadow, target: p }; this.cards.set(c.Card.ID, node)
   }
   node.face.texture = this.textures[this.asset(c.Card)]
   const button = this.buttons.get(c.Card.ID)
   if(button) button.dataset.cardAsset = this.asset(c.Card)
   node.face.setSize(p.width, p.height)
   const tint = view.my_turn && c.Playable && !view.closed ? 0xffffff : 0x6b6b6b
   if(animate && !this.reduced.matches && node.face.tint !== tint) this.tween(node.face, { pixi: { tint }, duration: .3, ease: 'power2.inOut', overwrite: 'auto' })
   else node.face.tint = tint
   node.group.zIndex = c.Card.ID === this.selected ? 1000 : Number(button?.dataset.slot || 0)
   const changed = Math.abs(node.target.x - p.x) > .5 || Math.abs(node.target.y - p.y) > .5
   node.target = p
   if(!animate && !gsap.isTweening(node.lift)) node.lift.y = c.Card.ID === this.selected ? -18 : 0
   if(!animate && !gsap.isTweening(node.lift.scale)) node.lift.scale.set(c.Card.ID === this.selected ? 1.08 : 1)
   this.handShadow(node)
   if (animate && changed) this.tween(node.group, { pixi: { x: p.x, y: p.y }, duration: .45, ease: 'power3.out', overwrite: 'auto' })
   else if (!animate && !gsap.isTweening(node.group)) node.group.position.set(p.x, p.y)
  })
  this.requestRender()
 }
 private handShadow(node: CardNode) {
  const elevation = Math.max(0, -node.lift.y) / 18
  node.shadow.position.set(elevation * 3, 6 + elevation * 8)
  node.shadow.setSize(node.target.width * (1.08 + elevation * .08), node.target.height * (1.08 + elevation * .04))
  node.shadow.alpha = Math.min(.8, .48 + elevation * .12)
  node.face.rotation = -.025 * elevation
 }
 private fly(from: Position | undefined, to: Position | undefined, front: string | undefined, kind: string, delay: number, destination?: CardNode, onLand?: () => void, cardID?: string, revealOpponent = false) {
  if (!from || !to) return
  const duration = kind === 'play' ? .6 : kind === 'deal' ? .65 : .78
  const opponentPlay = kind === 'play' && revealOpponent
  const startWidth = Math.min(95, from.width), startHeight = startWidth * 344 / 256
  const flight = new Container({ x:from.x, y:from.y, visible:false, label:cardID, eventMode:'none' })
  this.flights.addChild(flight)
  const castShadow = this.sprite(cardShadow, {x:0,y:6,width:startWidth*1.1,height:startHeight*1.1},flight)
  const mesh = new PerspectiveMesh({ texture:this.textures[kind==='play' && front && !opponentPlay ? front : back], verticesX:8, verticesY:8, eventMode:'none' })
  mesh.setCorners(...cardCorners(startWidth,startHeight,0))
  flight.addChild(mesh)
  const marker = document.createElement('span')
  marker.hidden = true; marker.dataset.motionKind = kind
  marker.dataset.motionPerspective = 'true'
  marker.dataset.motionPrivate = String(!!front && kind !== 'play')
  marker.dataset.flightFace = kind==='play' && front && !opponentPlay ? 'front' : 'back'
  marker.dataset.flightAngle = '0'
  if(cardID)marker.dataset.flightCardId=cardID
  this.root.append(marker); this.moving.set(flight, marker)
  this.root.dataset.motionActive = 'true'
  if (destination) destination.group.visible = false
  this.context.add(() => {
   const progress = { value: 0 }, flip = { angle: 0 }
   const render = () => {
    const t = progress.value, landing = destination?.target ?? to
    const elevation = Math.sin(Math.PI*t)
    flight.x = from.x + (landing.x-from.x)*t
    flight.y = from.y + (landing.y-from.y)*t - 7*(1-t) - elevation*Math.min(38,Math.abs(landing.y-from.y)*.12)
    flight.rotation = -.06*(1-t) + (kind==='play' ? -.052*t : 0)
    // An opponent's avatar is square, but an anonymous card keeps its card aspect ratio.
    const landingHeight = destination || kind==='play' ? landing.height : landing.width*344/256
    const width = startWidth+(landing.width-startWidth)*t, height = startHeight+(landingHeight-startHeight)*t
    const revealed = !!front && (kind==='play' && !opponentPlay || flip.angle>=Math.PI/2)
    const texture = this.textures[revealed ? front! : back]
    if(mesh.texture!==texture) mesh.texture=texture
    mesh.setCorners(...cardCorners(width,height,flip.angle))
    mesh.tint = revealed && destination ? destination.face.tint : 0xffffff
    castShadow.setSize(width*(1.1+elevation*.12)*Math.max(.16,Math.abs(Math.cos(flip.angle))),height*(1.1+elevation*.05))
    castShadow.position.set(elevation*5,6+elevation*14)
    castShadow.alpha = .48+elevation*.12
    marker.dataset.flightFace = revealed ? 'front' : 'back'
    marker.dataset.flightAngle = String(flip.angle)
   }
   const timeline = gsap.timeline({ onUpdate: () => this.requestRender(), onComplete: () => {
    if (destination && !destination.group.destroyed) destination.group.visible = true
    onLand?.()
    marker.remove(); this.moving.delete(flight); flight.destroy({ children:true }); this.animations.delete(timeline)
    if (!this.moving.size) delete this.root.dataset.motionActive
    this.requestRender()
   } })
   this.animations.add(timeline)
   timeline.set(flight, { visible: true }, delay)
   timeline.call(()=>tableFeedback.play(kind === 'play' ? 'play' : kind === 'deal' ? 'deal' : 'draw', kind === 'play'),[],delay)
   // A bowed path with acceleration/deceleration, small anticipation and a quiet landing.
   timeline.to(flight, { y:from.y-7, rotation:-.06, duration:.08, ease:'power1.out' }, delay)
   timeline.to(progress, { value:1, duration:duration-.08, ease:'power2.inOut', onUpdate:render },delay+.08)
   if (front && (kind !== 'play' || opponentPlay)) {
    timeline.to(flip, { angle:Math.PI, duration:kind === 'draw' ? .5 : .4, ease:'sine.inOut', onUpdate:render },delay+(opponentPlay ? .04 : .15))
   }
   // A small settling beat leaves the revealed face visible before the hand takes over.
   timeline.to(flight.scale,{x:1.025,y:1.025,duration:.06,repeat:1,yoyo:true,ease:'sine.inOut'},delay+duration)
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
 private effect(src: string, p: Position | undefined, kind: string, duration = .75, fadeOnly = false) {
  if(!p || this.reduced.matches) return
  const group = new Container({eventMode:'none'})
  this.flights.addChild(group)
  const sprite = this.sprite(src,p,group)
  if(kind==='color') sprite.rotation=-.052
  const marker = document.createElement('span')
  marker.hidden=true; marker.dataset.actionEffect=kind
  this.root.append(marker); this.moving.set(group,marker); this.root.dataset.motionActive='true'
  this.context.add(()=>{
   const timeline=gsap.timeline({onUpdate:()=>this.requestRender(),onComplete:()=>{
    marker.remove();this.moving.delete(group);group.destroy({children:true});this.animations.delete(timeline)
    if(!this.moving.size) delete this.root.dataset.motionActive
    this.requestRender()
   }})
   this.animations.add(timeline)
   if(fadeOnly) timeline.to(sprite,{alpha:0,duration,ease:'sine.inOut'})
   else timeline.fromTo(sprite,{alpha:0},{alpha:.9,duration:.2,ease:'power2.out'}).to(sprite,{alpha:0,duration:duration-.2,ease:'sine.inOut'})
  })
 }
 sync(view: GameView, selected: string, baseline?: GameView) {
  if(this.disposed || !this.root.dataset.sceneReady) return
  const before = this.previous ?? baseline
  if(before?.game_id === view.game_id && view.revision < before.revision) return
  const advanced = !!before && view.revision > before.revision
  if(this.drag && (advanced || selected !== this.drag.id || this.root.dataset.inputEnabled !== 'true')) { this.swallowClick=this.drag.moved;this.cancelDrag(true) }
  // Pending/connection UI must still cancel an unavailable gesture before skipping.
  if(this.previous === view && selected === this.selected && !this.pendingArrivals.size) return
  const events = (view.events ?? []).filter(e => before && e.revision > before.revision && e.revision <= view.revision)
  const contiguous = !!before && (view.revision <= before.revision + 1 || events.some(e => e.revision === before.revision + 1))
  const motion = advanced && contiguous && !view.recovery && !this.reduced.matches
  const oldPositions = new Map((motion ? [...this.cards] : []).map(([id, node]) => [id, { ...node.target, x: node.group.x, y: node.group.y + node.lift.y }]))
  const oldPlayers = motion ? new Map(this.players) : new Map<string, Position>()
  // A baseline recovery remains on the view while the user selects/touches cards.
  // Re-rendering that same baseline must not cancel a newly captured gesture.
  if((view.recovery && (advanced || before?.game_id!==view.game_id)) || (advanced && !contiguous) || (this.reduced.matches && advanced)) this.stop()
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
      },event.card_id,!player?.me)
      this.root.dataset.discardVisibleLayers=String(this.discardHistory.length-1)
     }
     if(event.type === 'cards_drawn' && event.count) {
      if(player?.me) this.arrivals(incoming.splice(0,event.count).map(c=>c.Card))
      else for(let i=0;i<event.count;i++) this.fly(this.deck, this.players.get(event.player), undefined, 'draw', i * .13)
     }
     if(event.type === 'player_skipped') {
      tableFeedback.play('skip',!!player?.me)
      const p=this.players.get(event.player)
      this.effect(skipHighlight,p && {...p,width:p.width*2.3,height:p.height*2.3},'skip')
     }
     if(event.type === 'direction_changed') {
      tableFeedback.play('reverse')
      this.table.children.filter(c=>c.label==='direction-arrow').forEach(c=>this.tween(c,{alpha:1,duration:.35,repeat:1,yoyo:true,ease:'sine.inOut'}))
      this.effect(playerHighlight,this.discard && {...this.discard,width:this.discard.width*1.6,height:this.discard.height*1.6},'reverse')
     }
     if(event.type === 'color_chosen') {
      tableFeedback.play('color')
      if(before!.top?.ID === view.top?.ID && this.discard) this.effect(this.asset(before!.top!,before!.phase===2 ? undefined : before!.active_color),this.discard,'color',.45,true)
     }
     if(event.type === 'turn_changed') {
      const p=this.players.get(event.player)
      this.effect(playerHighlight,p && {...p,width:p.width*2.2,height:p.height*2.2},'turn',.8)
     }
     if(event.type === 'hands_swapped') {
      const me=view.players.find(p=>p.me), other=event.player === me?.key ? event.target : event.target === me?.key ? event.player : undefined
      if(other) {
       const origin=oldPlayers.get(other)
       this.arrivals(incoming.splice(0).map(c=>c.Card),origin,'swap')
       this.effect(playerHighlight,this.players.get(other),'swap')
      }
     }
     if(event.type === 'game_finished' && view.close_reason !== 'cancelled') tableFeedback.play(view.players.find(p=>p.me)?.position===1 ? 'win' : 'lose',true)
    }
   }
  }
  this.pendingArrivals.forEach((c,id)=>{if(!view.hand.some(card=>card.Card.ID===c.card.ID))this.pendingArrivals.delete(id)})
  const deferred=[...this.pendingArrivals.values()].filter(c=>this.cards.has(c.card.ID))
  deferred.forEach(c=>this.pendingArrivals.delete(c.card.ID))
  const batches=new Map<string,{cards:Card[];from?:Position;kind:string}>()
  deferred.forEach(c=>{
   const key=`${c.kind}:${c.from?.x}:${c.from?.y}`
   const batch=batches.get(key) ?? {cards:[],from:c.from,kind:c.kind}
   batch.cards.push(c.card);batches.set(key,batch)
  })
  batches.forEach(batch=>this.arrivals(batch.cards,batch.from,batch.kind))
  if(selected !== this.selected) {
   const wasSelected=this.selected
   this.selected = selected
   if(selected && selected!==wasSelected) tableFeedback.play('select',true)
   for(const id of new Set([wasSelected,selected])) {
    const node=this.cards.get(id)
    if(!node) continue
    const button=this.buttons.get(id)
    node.group.zIndex = id === selected ? 1000 : Number(button?.dataset.slot || 0)
    this.tween(node.lift.scale,{x:id===selected ? 1.08 : 1,y:id===selected ? 1.08 : 1,duration:this.reduced.matches ? 0 : .4,ease:'elastic.out(.65,.7)',overwrite:'auto'})
    this.tween(node.lift, { pixi: { y: id === selected ? -18 : 0 }, duration: this.reduced.matches ? 0 : .4, ease: 'elastic.out(.65,.7)', overwrite: 'auto', onUpdate: () => { this.handShadow(node); if(button) button.dataset.visualLift=String(node.lift.y) } })
   }
  }
 }
}
