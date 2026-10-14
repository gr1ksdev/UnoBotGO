// Development presentation fixture, not live engine/Telegram validation.
import { chromium } from 'playwright'
import assert from 'node:assert/strict'
import { mkdir, writeFile } from 'node:fs/promises'
const out = process.env.UNO_PERFORMANCE_OUTPUT || '.reports/desempenho-fullscreen'
const base = process.env.UNO_VISUAL_URL || 'http://127.0.0.1:5173'
const expectFix = process.env.UNO_PERFORMANCE_EXPECT_FIX !== '0'
await mkdir(out, { recursive: true })
const browser = await chromium.launch({ executablePath: process.env.PLAYWRIGHT_EXECUTABLE_PATH })
try {
 const page = await browser.newPage({ viewport: { width: 390, height: 844 }, deviceScaleFactor: 2, hasTouch: true })
 const errors = [], commands = []
 page.on('pageerror', e => errors.push(e.message))
 await page.addInitScript(() => {
  window.__fullscreenCalls = 0
  window.Telegram = { WebApp: { platform: 'tdesktop', initData: 'performance-fixture', ready() {}, expand() {}, isVersionAtLeast() { return true }, requestFullscreen() { window.__fullscreenCalls++ } } }
 })
 await page.route('https://telegram.org/**', r => r.abort())
 const view = { game_id: 'performance', revision: 10, recovery: true, phase: 1, group: 'Performance · fixture', mode: 'classic', system: 'updated', players: Array.from({ length: 10 }, (_, i) => ({ key: i ? `p${i}` : 'me', name: `Teste ${i}`, count: i ? 7 : 30, me: i === 0, current: i === 0, active: true })), hand: Array.from({ length: 30 }, (_, i) => ({ Card: { ID: `c${i}`, Color: 1 + i % 4, Rank: i % 10 }, Playable: true })), top: { ID: 'top', Color: 1, Rank: 2 }, active_color: 1, direction: 1, closed: false, owner: true, my_turn: true, drawn_card_id: '', can_bluff: false, deadline: new Date(Date.now() + 120000).toISOString(), server_time: new Date().toISOString(), result: null, events: [] }
 await page.route('**/api/v1/**', r => { const u = new URL(r.request().url()); return r.fulfill({ json: u.pathname.endsWith('/rooms') ? [] : u.pathname.endsWith('/me') ? { stats: [], history: [] } : u.pathname.endsWith('/config') ? { bot_username: 'fixture' } : u.pathname.includes('/rankings/') ? { items: [], system: 'updated', month_start: '2026-10-01', month_name: 'Outubro' } : u.pathname.endsWith('/position') ? { entry: null } : view }) })
 await page.routeWebSocket('**/api/v1/live', ws => ws.onMessage(raw => { const m = JSON.parse(String(raw)); if (m.init_data) ws.send(JSON.stringify({ type: 'snapshot', view, recovery: true })); else commands.push(m) }))
 await page.goto(`${base}/game/performance`)
 await page.locator('[data-scene-ready=true]').waitFor()
 await page.waitForTimeout(700)
 // Instrument only this dev harness. No production globals or logging are added.
 await page.evaluate(async () => {
  const { TableScene } = await import('/src/components/scene/TableScene.ts')
  window.__perf = { sync: 0, layout: 0, sprites: 0, tweens: 0, frames: [] }
  for (const [name, counter] of [['sync', 'sync'], ['layout', 'layout'], ['sprite', 'sprites'], ['tween', 'tweens']]) {
   const original = TableScene.prototype[name]
   TableScene.prototype[name] = function (...args) {
    window.__perf[counter]++
    if (!window.__scene) {
     window.__scene = this
     const render = this.app.render.bind(this.app)
     this.app.render = () => { window.__perf.frames.push(performance.now()); render() }
    }
    return original.apply(this, args)
   }
  }
 })
 const cards = page.locator('.hand button')
 await cards.first().tap()
 await page.waitForTimeout(550)
 const snapshot = () => page.evaluate(() => structuredClone(window.__perf))
 const delta = (a, b) => Object.fromEntries(['sync', 'layout', 'sprites', 'tweens'].map(k => [k, b[k] - a[k]]))
 let before = await snapshot()
 await page.evaluate(() => { const s = window.__scene; for (let i = 0; i < 20; i++) s.sync(s.previous, s.selected) })
 await page.waitForTimeout(550)
 const unchanged = delta(before, await snapshot())
 before = await snapshot()
 await cards.nth(1).tap()
 await page.waitForTimeout(550)
 const selection = delta(before, await snapshot())
 before = await snapshot()
 await page.evaluate(async () => {
  const hand = document.querySelector('.hand')
  for (let i = 1; i <= 16; i++) {
   hand.scrollTop = (hand.scrollHeight - hand.clientHeight) * i / 16
   await new Promise(requestAnimationFrame)
  }
 })
 await page.waitForTimeout(100)
 const scroll = delta(before, await snapshot())
 await page.locator('.hand').evaluate(n => { n.scrollTop = 0 })
 await cards.first().tap()
 await page.waitForTimeout(550)
 const card = await cards.first().boundingBox()
 const cdp = await page.context().newCDPSession(page)
 const x = card.x + 5, y = card.y + 25
 await page.evaluate(() => {
  window.__dragSamples = []
  // CDP can acknowledge a touch before Chromium dispatches its pointer event.
  // Observe the coordinates after the scene handles that same bubbling event.
  window.addEventListener('pointermove', event => {
   const d = window.__scene.drag
   if (!d?.moved) return
   window.__dragSamples.push({
    x: event.clientX, y: event.clientY,
    error: Math.hypot(d.node.group.x - d.origin.x - (event.clientX - d.startX), d.node.group.y - d.origin.y - (event.clientY - d.startY)),
   })
  })
 })
 await cdp.send('Input.dispatchTouchEvent', { type: 'touchStart', touchPoints: [{ x, y }] })
 before = await snapshot()
 for (let i = 1; i <= 16; i++) {
  await cdp.send('Input.dispatchTouchEvent', { type: 'touchMove', touchPoints: [{ x: x + i, y: y - i * 2 }] })
 }
 await page.waitForFunction(({ x, y }) => {
  const last = window.__dragSamples.at(-1)
  return last && Math.abs(last.x - x) < .1 && Math.abs(last.y - y) < .1
 }, { x: x + 16, y: y - 32 })
 const drag = delta(before, await snapshot())
 const followingError = await page.evaluate(() => Math.max(...window.__dragSamples.map(sample => sample.error)))
 // Availability may change without a new accepted snapshot/revision.
 const unavailableCancelled = await page.evaluate(() => {
  const s = window.__scene
  s.root.dataset.inputEnabled = 'false'
  s.sync(s.previous, s.selected)
  const cancelled = !s.drag
  s.root.dataset.inputEnabled = 'true'
  return cancelled
 })
 await cdp.send('Input.dispatchTouchEvent', { type: 'touchCancel', touchPoints: [] })
 await page.waitForTimeout(650)
 const renderCount = (await snapshot()).frames.length
 await page.waitForTimeout(600)
 const idleFrames = (await snapshot()).frames.length - renderCount
 // Observe real renderer callbacks during selection; headless software numbers are not phone FPS.
 await page.evaluate(() => { window.__perf.frames = []; window.__rafSamples = []; const sample = t => { window.__rafSamples.push(t); if (window.__sampling) requestAnimationFrame(sample) }; window.__sampling = true; requestAnimationFrame(sample) })
 for (let i = 0; i < 6; i++) { await cards.nth(1 + i % 3).tap(); await page.waitForTimeout(150) }
 await page.waitForTimeout(550)
 const timing = await page.evaluate(() => {
  window.__sampling = false
  const summarize = values => { const dt = values.slice(1).map((v, i) => v - values[i]).sort((a, b) => a - b); return { count: values.length, medianMs: dt[Math.floor(dt.length * .5)] ?? null, p95Ms: dt[Math.floor(dt.length * .95)] ?? null, over50Ms: dt.filter(n => n > 50).length } }
  return { renderer: summarize(window.__perf.frames), browserRAF: summarize(window.__rafSamples) }
 })
 await page.getByRole('button', { name: 'Voltar ao início', exact: true }).click()
 await page.locator('.game-v2').waitFor({ state: 'detached' })
 const afterUnmount = (await snapshot()).frames.length
 await page.waitForTimeout(250)
 assert.equal((await snapshot()).frames.length, afterUnmount, 'destroyed scene must never render')
 assert.equal(commands.length, 0, 'performance interactions must not play cards')
 assert.deepEqual(errors, [])
 const fullscreenCalls = await page.evaluate(() => window.__fullscreenCalls)
 const detached = await page.evaluate(() => !window.__scene.renderSubscribed && !window.__scene.animations.size)
 if (expectFix) {
  assert.equal(unchanged.layout, 0, 'identical state must skip layout')
  assert.equal(unchanged.tweens, 0, 'identical state must not allocate tweens')
  assert.equal(selection.sprites, 0, 'selection must retain static sprites')
  assert(selection.tweens <= 4, 'selection should animate only old/new selected cards')
  assert.equal(scroll.sprites, 0, 'scroll must retain static sprites')
  assert.equal(drag.tweens, 0, 'pointer move must not allocate GSAP tweens')
  assert(followingError < 1, 'drag must follow every delivered pointer event immediately')
  assert(unavailableCancelled, 'availability loss with the same snapshot must cancel the drag')
  assert(detached, 'destroyed scene must unsubscribe from GSAP and clear animations')
  assert.equal(idleFrames, 0, 'idle scene must stop rendering')
  assert.equal(fullscreenCalls, 0, 'desktop must not request fullscreen')
 }
 const report = { passed: true, assertionsEnabled: expectFix, environment: 'Chromium headless, software rendering; not physical Telegram FPS', viewport: '390x844 @2x', cards: 30, players: 10, unchanged, selection, scroll, drag, followingErrorPx: followingError, followingErrorMeasurement: 'maximum scene position error in the same delivered pointermove event; not physical input-to-display latency', unavailableCancelled, detached, idleFrames, fullscreenCalls, timing }
 await writeFile(`${out}/checks.json`, JSON.stringify(report, null, 2))
 console.log(JSON.stringify(report, null, 2))
} finally { await browser.close() }
