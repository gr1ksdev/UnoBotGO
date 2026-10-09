// Real two-browser game. Only Telegram SDK launch metadata is supplied locally;
// HTTP, WebSocket, engine, persistence and score are the actual Go implementation.
import { chromium } from 'playwright'
import { spawn } from 'node:child_process'
import { mkdir, writeFile } from 'node:fs/promises'
import assert from 'node:assert/strict'
const backend = process.env.UNO_E2E_BACKEND
assert(backend && process.env.UNO_E2E_AUTH_A && process.env.UNO_E2E_AUTH_B)
const out = process.env.UNO_E2E_OUTPUT || '.reports/partida-pixi/integration'
await mkdir(out, { recursive: true })
const port = 5184
const production = process.env.UNO_E2E_PRODUCTION === '1'
const vite = production ? undefined : spawn(process.execPath, ['node_modules/vite/bin/vite.js', '--host', '127.0.0.1', '--port', String(port), '--strictPort'], {
 cwd: process.cwd() + '/web', env: { ...process.env, VITE_API_PROXY_TARGET: backend, UNO_VITE_CACHE_DIR: 'node_modules/.vite-e2e' }, stdio: 'ignore',
})
// Vite CLI path is relative to web cwd.
vite?.on('error', error => { throw error })
const base = production ? backend : `http://127.0.0.1:${port}`
let browser
const errors = []
const videoStarted = []
const report = { transport: 'real WebSocket', backend: 'real Go engine and PostgreSQL', frontend: production ? 'Go embedded production bundle, unchanged strict CSP' : 'Vite development bundle', identities: 'two test identities; no live Telegram', commands: {}, steps: [] }
try {
 for (let n = 0; n < 100; n++) {
  try { if ((await fetch(base)).ok) break } catch { /* startup */ }
  await new Promise(resolve => setTimeout(resolve, 100))
  if (n === 99) throw new Error('Vite did not start')
 }
 browser = await chromium.launch({ executablePath: process.env.PLAYWRIGHT_EXECUTABLE_PATH, headless: true })
 const pages = []
 for (const auth of [process.env.UNO_E2E_AUTH_A, process.env.UNO_E2E_AUTH_B]) {
  const context = await browser.newContext({ viewport: { width: 390, height: 844 }, deviceScaleFactor: 2, recordVideo: { dir: `${out}/video`, size: { width: 390, height: 844 } } })
  await context.route('https://telegram.org/**', route => route.abort())
  await context.addInitScript(raw => {
   window.__motions = JSON.parse(sessionStorage.getItem('test-motions') ?? '[]')
   window.__videoActions = JSON.parse(sessionStorage.getItem('test-video-actions') ?? '[]')
   window.__cspViolations = []
   document.addEventListener('securitypolicyviolation',event=>window.__cspViolations.push(event.violatedDirective))
   new MutationObserver(records => { for (const record of records) for (const node of record.addedNodes) if (node instanceof HTMLElement && node.dataset.motionKind) { window.__motions.push({ kind: node.dataset.motionKind, at: Date.now(), private:node.dataset.motionPrivate==='true', my_turn:window.__view?.my_turn }); sessionStorage.setItem('test-motions',JSON.stringify(window.__motions)) } }).observe(document, { childList: true, subtree: true })
   const events = {}
   window.Telegram = { WebApp: { initData: raw, initDataUnsafe: { start_param: new URL(location.href).searchParams.get('test_launch') ?? undefined }, ready() {}, expand() {},
    onEvent(name, fn) { (events[name] ??= []).push(fn) },
    offEvent(name, fn) { events[name] = (events[name] ?? []).filter(f => f !== fn) },
   } }
   window.__activate = () => (events.activated ?? []).forEach(fn => fn())
   window.__commands = JSON.parse(sessionStorage.getItem('test-commands') ?? '[]')
   const Native = window.WebSocket
   window.WebSocket = class extends Native {
    constructor(...args) {
     super(...args)
     this.addEventListener('message', event => {
      const message = JSON.parse(event.data)
      if (message.view && (!window.__view || message.view.game_id !== window.__view.game_id || message.view.revision >= window.__view.revision)) window.__view = message.view
      window.__message = message
     })
     window.__socket = this
    }
    send(data) {
     const command = JSON.parse(data)
     if (command.action) { window.__commands.push(command); sessionStorage.setItem('test-commands',JSON.stringify(window.__commands)); window.__videoActions.push({action:command.action,at:Date.now()});sessionStorage.setItem('test-video-actions',JSON.stringify(window.__videoActions)) }
     super.send(data)
    }
   }
  }, auth)
  videoStarted.push(Date.now())
  const page = await context.newPage()
  page.on('pageerror', error => { errors.push(error.message); console.error('Browser error:',error.message) })
  page.setDefaultTimeout(10000)
  pages.push(page)
  await page.goto(`${base}/home`)
  await page.getByText(/Nenhuma sala disponível/).waitFor()
  assert(await page.getByRole('button', { name: 'Criar sala', exact: true }).isVisible())
 }
 await pages[0].screenshot({ path: `${out}/home-empty.png` })
 // Every room/admission command now comes from the real Mini App UI.
 await pages[0].getByRole('button', { name: 'Criar sala', exact: true }).click()
 await pages[0].getByLabel('Grupo vinculado').selectOption({ label: 'Sala WebApp E2E' })
 await pages[0].getByLabel('Modo de jogo').selectOption('classic')
 await pages[0].getByRole('button', { name: 'Criar e entrar', exact: true }).click()
 await pages[0].waitForFunction(() => window.__view?.phase === 0 && window.__view?.players.length === 1)
 const gameID = await pages[0].evaluate(() => window.__view.game_id)
 report.game_id = gameID
 await pages[0].getByRole('button', { name: 'Compartilhar convite' }).click()
 const invitation = await pages[0].getByLabel('Convite da sala', { exact: true }).inputValue()
 const parameter = new URL(invitation).searchParams.get('startapp')
 assert(parameter.startsWith('join_'))
 await pages[1].goto(`${base}/ranking?test_launch=${parameter}`)
 await pages[1].getByRole('button', { name: 'Entrar na sala', exact: true }).click()
 for (const [i,page] of pages.entries()) {
  await page.waitForFunction(() => window.__view?.phase === 0 && window.__view.players.length === 2)
  await page.screenshot({ path: `${out}/lobby-${i+1}.png` })
 }
 // The joined room is discoverable and internally reopenable at every viewport.
 await pages[0].getByRole('button', { name: 'Voltar ao início' }).click()
 for (const [width,height] of [[320,568],[360,800],[390,844],[430,932]]) {
  await pages[0].setViewportSize({width,height})
  await pages[0].getByRole('link', { name: /Sala WebApp E2E.*Abrir na Mini App/ }).waitFor()
  assert(await pages[0].evaluate(() => document.documentElement.scrollWidth <= innerWidth))
  await pages[0].screenshot({ path: `${out}/rooms-${width}.png`, fullPage:true })
 }
 await pages[0].setViewportSize({width:390,height:844})
 await pages[0].getByRole('link', { name: /Sala WebApp E2E.*Abrir na Mini App/ }).click()
 await pages[0].waitForFunction(() => window.__view?.phase === 0)
 report.steps.push('Account A creates inside Mini App, shares a direct invite; account B opens it and joins via the UI; both reopen the same room')
 await pages[0].getByRole('button', { name: 'Começar partida' }).click()
 for (const [i, page] of pages.entries()) {
  await page.waitForFunction(() => window.__view?.hand.length === 7)
  await page.waitForFunction(() => window.__motions.some(m=>m.kind === 'deal'))
  await page.waitForFunction(() => !document.querySelector('[data-motion-active]'))
  if(![2,4].includes(await page.evaluate(()=>window.__view.phase))) await Promise.all(pages.map(p=>p.locator('.v2-choice').waitFor({state:'hidden'})))
  await page.screenshot({ path: `${out}/private-hand-${i + 1}.png` })
 }
 let views = await Promise.all(pages.map(p => p.evaluate(() => window.__view)))
 const ids = views.map(v => new Set(v.hand.map(c => c.Card.ID)))
 assert(views[1].hand.every(c => !ids[0].has(c.Card.ID)))
 assert(views.every(v => v.players.length === 2 && v.players.filter(p => p.me).length === 1))
 report.steps.push('Start via WebSocket; each browser receives only its distinct seven-card private hand')
 // Reload account B mid-game: HTTP recovery and a new authorized socket, no new game.
 const motionsBeforeReconnect=await pages[1].evaluate(()=>window.__motions.length)
 await pages[1].goto(`${base}/ranking?test_launch=game_${gameID}`)
 await pages[1].waitForFunction(() => window.__view?.hand.length === 7)
 assert(new URL(pages[1].url()).pathname === `/game/${gameID}`)
 assert.equal(await pages[1].evaluate(()=>window.__motions.length),motionsBeforeReconnect,'Reconnect must not replay dealing')
 report.steps.push('Account B opens the room deep link from the ranking URL and reconnects to the same game')
 let forcedDraw = false
 let colorsCaptured = false
 for (let n = 0; n < 1200; n++) {
  if(n % 10 === 0) await writeFile(`${out}/progress.json`,JSON.stringify({commands:n,views:await Promise.all(pages.map(p=>p.evaluate(()=>window.__view)))}))
  views = await Promise.all(pages.map(p => p.evaluate(() => window.__view)))
  if (views.every(v => v.closed && v.result?.status === 'scored')) break
  if (views.some(v => v.closed)) {
   await Promise.all(pages.map(p => p.waitForFunction(() => window.__view?.result?.status === 'scored')))
   break
  }
  const actor = views.findIndex(v => v.my_turn)
  assert(actor >= 0, 'Server must identify current player')
  const page = pages[actor]
  const v = views[actor]
  if (v.phase === 2) {
   if(!colorsCaptured) {
    for(const [width,height] of [[320,568],[360,800],[390,844],[430,932]]) {
     await page.setViewportSize({width,height})
     await page.screenshot({path:`${out}/color-picker-${width}.png`})
    }
    await page.setViewportSize({width:390,height:844});colorsCaptured=true
   }
   await page.getByRole('button', { name: 'Vermelho', exact: true }).click()
  } else if (!forcedDraw && v.phase === 1 && !v.drawn_card_id) {
   await page.getByRole('button', { name: 'Comprar carta', exact: true }).click()
   forcedDraw = true
  } else {
   const playable = v.hand.filter(c => c.Playable)
   if (playable.length) {
    // Exercise legal wild color choice; every selection comes from server Playable.
    playable.sort((a,b) => (a.Card.Rank === 13 ? 100 : a.Card.Color === 0 ? -1 : a.Card.Rank) - (b.Card.Rank === 13 ? 100 : b.Card.Color === 0 ? -1 : b.Card.Rank))
    const card = playable.at(-1)
    const selectedCard = page.locator(`[data-card-id="${card.Card.ID}"]`)
    await selectedCard.scrollIntoViewIfNeeded()
    await selectedCard.click()
    await page.waitForTimeout(420)
    assert(Number(await selectedCard.getAttribute('data-visual-lift'))<=-17.9, 'Selected Pixi card must visibly rise without reflow')
    await page.getByRole('button', { name: 'Jogar carta', exact: true }).click()
   } else if (v.drawn_card_id) {
    await page.getByRole('button', { name: 'Passar turno', exact: true }).click()
   } else {
    await page.getByRole('button', { name: 'Comprar carta', exact: true }).click()
   }
  }
  await page.waitForFunction(revision => window.__view?.revision > revision || window.__message?.type === 'rejected', v.revision)
  assert.notEqual(await page.evaluate(() => window.__message?.type), 'rejected', 'UI sent a rejected command')
  await page.waitForFunction(() => !document.querySelector('[data-motion-active]'))
  if(![2,4].includes(await page.evaluate(()=>window.__view.phase))) await Promise.all(pages.map(p=>p.locator('.v2-choice').waitFor({state:'hidden'})))
  const revision = await page.evaluate(() => window.__view.revision)
  await Promise.all(pages.map(p => p.waitForFunction(r => window.__view?.revision >= r, revision)))
  if (n === 1199) throw new Error('Game did not finish normally')
 }
 assert(forcedDraw)
 const results = await Promise.all(pages.map(p => p.evaluate(() => window.__view)))
 assert(results.every(v => v.close_reason === 'completed' && v.closed && v.result.status === 'scored' && v.hand.length === 0 && !v.my_turn))
 assert(results.every(v => v.awards.length === 2 && v.awards.every(a => typeof a.score_units === 'string')))
 report.steps.push('Buy, select and confirm play through UI; turns synchronized; normal two-player completion committed')
 const clientCommands = await Promise.all(pages.map(p => p.evaluate(() => window.__commands)))
 assert(clientCommands.every(commands => commands.some(c => c.action === 'play')), 'Both clients must play via their own socket')
 const allCommands = clientCommands.flat()
 for (const c of allCommands) report.commands[c.action] = (report.commands[c.action] ?? 0) + 1
 assert(report.commands.color>=1,'The complete flow must choose a color')
 if (report.commands.color) report.steps.push('Wild color choice exercised through the accessible UI')
 const last = allCommands.filter(c => c.action).sort((a,b) => b.expected_revision - a.expected_revision)[0]
 const actor = await Promise.all(pages.map(p => p.evaluate(id => window.__commands.some(c => c.request_id === id), last.request_id)))
 const finalPage = pages[actor.findIndex(Boolean)]
 const motionsBeforeDuplicate=await finalPage.evaluate(()=>window.__motions.length)
 await finalPage.evaluate(c => window.__socket.send(JSON.stringify(c)), last)
 await finalPage.waitForFunction(id => window.__message?.request_id === id, last.request_id)
 assert.equal(await finalPage.evaluate(() => window.__message.type), 'accepted')
 await finalPage.waitForTimeout(350)
 assert.equal(await finalPage.evaluate(() => window.__motions.length),motionsBeforeDuplicate,'Duplicate must not animate again')
 assert.equal(await finalPage.evaluate(() => window.__view.revision), results[0].revision)
 report.steps.push('Retry of final request ID accepted without changing final revision')
 for (const [i,page] of pages.entries()) {
  await page.getByText(/pontos confirmados/).waitFor()
  await page.screenshot({ path: `${out}/result-${i+1}.png` })
  await page.getByRole('link', { name: 'Ver ranking' }).click()
  await page.getByText('Conta A', { exact: true }).first().waitFor()
  await page.getByText('Conta B', { exact: true }).first().waitFor()
  await page.screenshot({ path: `${out}/ranking-${i+1}.png`, fullPage: true })
  await page.getByRole('tab', { name: 'Grupos', exact: true }).click()
  await page.getByText('Sala WebApp E2E', { exact: true }).waitFor()
  await page.screenshot({ path: `${out}/groups-${i+1}.png`, fullPage: true })
  await page.getByRole('link', { name: 'Perfil', exact: true }).click()
  await page.getByText('Mini App', { exact: true }).waitFor()
  await page.screenshot({ path: `${out}/profile-${i+1}.png`, fullPage: true })
 }
 report.steps.push('Both clients see committed results, updated player/group monthly ranking and one WebApp history entry')
 // Reopen the retained result; vote, reconnect and reach consensus in real transport.
 for (const page of pages) await page.goto(`${base}/game/${gameID}`)
 await Promise.all(pages.map(page => page.getByRole('button', { name: 'Aceitar revanche' }).waitFor()))
 await pages[0].getByRole('button', { name: 'Aceitar revanche' }).click()
 await Promise.all(pages.map(page => page.waitForFunction(() => window.__view?.rematch?.accepted.length === 1)))
 assert((await pages[1].evaluate(() => window.__view)).closed, 'one vote must not restart')
 await pages[0].screenshot({ path: `${out}/rematch-one-vote.png` })
 await pages[0].reload()
 await pages[0].waitForFunction(() => window.__view?.rematch?.accepted.length === 1)
 await pages[0].screenshot({ path: `${out}/rematch-reconnected.png` })
 await pages[1].getByRole('button', { name: 'Aceitar revanche' }).click()
 await Promise.all(pages.map(page => page.waitForFunction(old => window.__view?.game_id !== old && !window.__view.closed && window.__view.hand.length === 7, gameID)))
 const nextIDs = await Promise.all(pages.map(page => page.evaluate(() => window.__view.game_id)))
 await Promise.all(pages.map(page=>page.waitForFunction(()=>document.querySelector('[data-scene-ready]') && !document.querySelector('[data-motion-active]'))))
 assert.equal(nextIDs[0], nextIDs[1]); assert.notEqual(nextIDs[0], gameID)
 report.next_game_id = nextIDs[0]
 for (const [i,page] of pages.entries()) await page.screenshot({ path: `${out}/rematch-new-round-${i+1}.png` })
 report.steps.push('Real server vote synced to both clients, reconnect preserved vote, unanimous vote atomically opened the same new engine ID with private 7-card hands')
 report.motion_counts = {}
 report.video_segments = []
 for(const [i,page] of pages.entries()) {
  const motions=await page.evaluate(()=>window.__motions), actions=await page.evaluate(()=>window.__videoActions)
  for(const [kind,event,seconds] of [['deal',motions.find(m=>m.kind==='deal'),4.5],['draw-and-flip',motions.find(m=>m.kind==='draw'&&m.private&&m.my_turn),2.5],['color',actions.find(m=>m.action==='color'),3.5]]) {
   if(event)report.video_segments.push({account:i+1,kind,start:Math.max(0,(event.at-videoStarted[i])/1000-1),seconds})
  }
 }
 for(const page of pages)for(const motion of await page.evaluate(()=>window.__motions))report.motion_counts[motion.kind]=(report.motion_counts[motion.kind]??0)+1
 await writeFile(`${out}/motion-counts.json`,JSON.stringify(report.motion_counts,null,2))
 assert(report.motion_counts.deal>=14 && report.motion_counts.play>=1 && report.motion_counts.draw>=1,`Real GSAP card flights must be observed: ${JSON.stringify(report.motion_counts)}`)
 assert.deepEqual(errors, [], 'No browser errors during gameplay/lifecycle')
 for(const page of pages) assert.deepEqual(await page.evaluate(()=>window.__cspViolations),[],'Production CSP must remain satisfied')
 report.passed = true
 await writeFile(`${out}/browser.json`, JSON.stringify(report, null, 2))
 console.log(JSON.stringify(report))
} catch (error) {
 if (browser) for (const [i, context] of browser.contexts().entries()) {
  const page = context.pages()[0]
  if (page) {
   await page.screenshot({ path: `${out}/failure-${i}.png`, fullPage: true }).catch(() => {})
   console.error('Last authoritative view:', JSON.stringify(await page.evaluate(() => window.__view).catch(() => null)))
  }
 }
 throw error
} finally {
 if(browser) {
  for(const [i,context] of browser.contexts().entries()) {
   const video=context.pages()[0]?.video()
   await context.close()
   if(video)await video.saveAs(`${out}/video/account-${i+1}.webm`)
  }
 }
 await browser?.close()
 vite?.kill('SIGTERM')
}
