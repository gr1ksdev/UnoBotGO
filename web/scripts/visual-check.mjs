// Browser-only test fixtures. Nothing in this harness is bundled into the app.
import { chromium } from 'playwright'
import { mkdir, writeFile } from 'node:fs/promises'
import assert from 'node:assert/strict'
const output = new URL(process.env.UNO_VISUAL_OUTPUT || '../../.reports/partida-pixi/', import.meta.url)
await mkdir(output, { recursive: true })
const browser = await chromium.launch({
 headless: true,
 executablePath: process.env.PLAYWRIGHT_EXECUTABLE_PATH || undefined,
})
const base = process.env.UNO_VISUAL_URL || 'http://127.0.0.1:5173'
const dimensions = [
 [320, 568],
 [360, 800],
 [390, 844],
 [430, 932],
]
const names = ['Marina', 'Rafael', 'Luiza', 'Pedro', 'Ana', 'Bruno']
const items = names.map((name, i) => ({
 name,
 position: i + 1,
 key: `test-${i}`,
 score_units: String(128450 - i * 9475),
 masked_id: '',
 avatar_url: '',
}))
const now = new Date().toISOString()
function game(count = 7, players = 2, closed = false) {
 return {
  game_id: 'test-room',
  revision: 20,
  phase: closed ? 3 : 1,
  group: 'Mesa dos Amigos',
  mode: 'classic',
  system: 'updated',
  players: Array.from({ length: players }, (_, i) => ({
   key: `seat-${i}`,
   name:
    i === 0
     ? 'Gabriel'
     : i === 1
       ? 'Rafael'
       : `Jogador de nome muito longo ${i}`,
   count: i === 0 ? count : 8,
   me: i === 0,
   current: i === 0,
   active: !closed,
   position: closed ? i + 1 : undefined,
  })),
  hand: Array.from({ length: count }, (_, i) => ({
      Card: { ID: `c${i}`, Color: (i % 4) + 1, Rank: i % 13 },
      Playable: i % 3 === 0,
     })),
  top: { ID: 'top', Color: 3, Rank: 6 },
  active_color: 3,
  direction: 1,
  closed,
  owner: true,
  my_turn: !closed,
  drawn_card_id: '',
  can_bluff: false,
  deadline: new Date(Date.now() + 84000).toISOString(),
  server_time: new Date().toISOString(),
  awards: closed
   ? [
      { key: 'seat-0', score_units: '1000', position: 1 },
      { key: 'seat-1', score_units: '0', position: 2 },
     ]
   : [],
  rematch: closed ? { revision: 1, required: Array.from({length: players}, (_, i) => `seat-${i}`), accepted: [], next_game_id: '', ready: true } : undefined,
  result: closed
   ? {
      id: 'test-room',
      score_units: '1000',
      system: 'updated',
      position: 1,
      status: 'scored',
     }
   : null,
 }
}
const results = []
for (const [width, height] of dimensions) {
 const context = await browser.newContext({
  viewport: { width, height },
  deviceScaleFactor: 2,
  reducedMotion: 'reduce',
  hasTouch: true,
 })
 await context.addInitScript(() => {
  window.Telegram = {
   WebApp: {
    initData: 'browser-test-only',
    safeAreaInset: {
     top: new URLSearchParams(location.search).has('safe') ? 24 : 0,
     bottom: new URLSearchParams(location.search).has('safe') ? 12 : 0,
     left: 0,
     right: 0,
    },
    contentSafeAreaInset: {
     top: new URLSearchParams(location.search).has('safe') ? 32 : 0,
     bottom: 0,
     left: 0,
     right: 0,
    },
    viewportStableHeight: innerHeight,
    initDataUnsafe: { user: { first_name: 'Gabriel', username: 'gabriel' } },
    ready() {},
    expand() {},
    setHeaderColor() {},
   },
  }
 })
 let rankingState = 'ready'
 let view = game()
 let visualSocket
 const page = await context.newPage()
 await page.route('https://telegram.org/**', (route) => route.abort())
 const errors = []
 page.on('pageerror', (e) => { errors.push(e.message); console.error(e.message) })
 await page.route('**/api/v1/**', async (route) => {
  const url = new URL(route.request().url())
  let data
  if (url.pathname.includes('/rankings/')) {
   if (rankingState === 'loading') return
   if (rankingState === 'error')
    return route.fulfill({ status: 404, json: { error: 'test-only-error' } })
   data = {
    items: url.pathname.endsWith('groups')
     ? items.map((item, i) => ({
        ...item,
        name: [
         'UNO da Galera',
         'Cartas & Café',
         'Noite de UNO',
         'Mesa dos Amigos',
         'Clube das Cartas',
         'Grupo',
        ][i],
       }))
     : rankingState === 'empty'
       ? []
       : rankingState === 'few'
         ? items.slice(0, 2)
         : rankingState === 'long'
           ? items.map((item) => ({
              ...item,
              name:
               'Nome muito longo de jogador para verificar truncamento e acessibilidade',
              score_units: '9007199254740993',
             }))
           : items,
    system: 'updated',
    month_start: '2026-10-01',
    month_name: 'Outubro',
    server_time: now,
    month_ends_at: '2026-11-01T03:00:00Z',
   }
  } else if (url.pathname.endsWith('/position'))
   data = {
    entry: { ...items[3], position: 4, name: 'Gabriel' },
    month_start: '2026-10-01',
    month_name: 'Outubro',
    system: 'updated',
   }
  else if (url.pathname.endsWith('/privacy')) data = { anonymous: false }
  else if (url.pathname.endsWith('/me'))
   data = {
    stats: [{ system: 'updated', score_units: '64550', games: 42, wins: 18 }],
    history: [
     {
      id: 'h1',
      group: 'Mesa dos Amigos',
      origin: 'webapp',
      system: 'updated',
      position: 1,
      score_units: '1000',
      status: 'scored',
      finished_at: now,
     },
     {
      id: 'h2',
      group: 'Clube das Cartas',
      origin: 'inline',
      system: 'updated',
      position: 2,
      score_units: '500',
      status: 'scored',
      finished_at: now,
     },
    ],
   }
  else if (url.pathname.endsWith('/config'))
   data = { bot_username: 'browser_test_only_bot' }
  else if (url.pathname.endsWith('/rooms'))
   data = [{ game_id: 'test-room', group: 'Mesa dos Amigos', phase: 1 }]
  else data = view
  await route.fulfill({ json: data })
 })
 await page.routeWebSocket('**/api/v1/live', (ws) => {
  visualSocket=ws
  ws.onMessage((raw) => {
   const m = JSON.parse(String(raw))
   if (m.init_data) ws.send(JSON.stringify({ type: 'snapshot', view }))
   else if(m.action === 'color') {
    setTimeout(() => { view = { ...view, revision:view.revision+1, phase:1, active_color:m.color, top:{ID:'wild',Color:0,Rank:13} }; ws.send(JSON.stringify({type:'accepted',request_id:m.request_id,view})) }, 400)
   } else if(m.action === 'rematch') {
    view = {...view, rematch:{...view.rematch, revision:view.rematch.revision+1, accepted:['seat-0']}}
    ws.send(JSON.stringify({type:'accepted',request_id:m.request_id,view}))
   } else ws.send(JSON.stringify({type:'accepted',request_id:m.request_id,view:{...view,revision:view.revision+1}}))
  })
 })
 for (const screen of [
  'home',
  'ranking',
  'groups',
  'profile',
  'game',
  'result',
 ]) {
  view = game(7, 2, screen === 'result')
  await page.goto(
   `${base}/${screen === 'groups' ? 'ranking?tab=groups' : screen === 'game' || screen === 'result' ? 'game/test-room' : screen}`,
  )
  await page
   .locator(
    screen === 'game'
     ? '.hand'
     : screen === 'result'
       ? '.result-panel'
       : screen === 'ranking' || screen === 'groups'
         ? '.podium'
         : screen === 'profile'
           ? '.summary-card'
           : '.hero',
   )
   .waitFor()
  if(screen === 'game' || screen === 'result')await page.locator('[data-scene-ready="true"]').waitFor()
  await page.evaluate(() => document.fonts.ready)
  assert(await page.evaluate(() => getComputedStyle(document.body).fontFamily.includes('MatchoNunito') && getComputedStyle(document.body).fontWeight === '550'), 'original base typography')
  if(screen === 'result') assert(await page.locator('.result-panel h1').evaluate(el => getComputedStyle(el).fontFamily.includes('MatchoLexend') && getComputedStyle(el).fontWeight === '900' && document.fonts.check('900 32px MatchoLexend')), 'original result typography')
  await page.screenshot({
   path: new URL(`${screen}-${width}.png`, output).pathname,
  })
  assert.equal(
   await page.evaluate(
    () => document.documentElement.scrollWidth <= innerWidth,
   ),
   true,
   `${screen} overflow at ${width}`,
  )
  if (screen === 'game') {
   const footer = await page.locator('.local-player').boundingBox()
   assert(
    footer.y + footer.height <= height + 1,
    'turn controls outside viewport',
   )
   assert.equal(await page.locator('nav').count(), 0)
  }
 }
 for (const state of ['empty', 'error', 'loading', 'few', 'long']) {
  rankingState = state
  await page.goto(`${base}/ranking`)
  await page
   .locator(
    state === 'empty'
     ? '.empty'
     : state === 'error'
       ? '.state-card'
       : state === 'loading'
         ? '.loading-hero'
         : state === 'few'
           ? '.few-results'
           : '.podium',
   )
   .waitFor()
  assert.equal(
   await page.evaluate(
    () => document.documentElement.scrollWidth <= innerWidth,
   ),
   true,
   `ranking ${state} overflow`,
  )
  await page.screenshot({
   path: new URL(`ranking-${state}-${width}.png`, output).pathname,
  })
 }
 rankingState = 'ready'
 let reuseTable=false, presentationRevision=100
 for (const count of (process.env.UNO_VISUAL_COUNTS?.split(',').map(Number) || [1, 2, 7, 8, 9, 15, 16, 17, 30]))
  for (const players of (process.env.UNO_VISUAL_PLAYERS?.split(',').map(Number) || [2, 3, 4, 5, 6, 7, 8, 9, 10])) {
   view = game(count, players)
   view.revision=++presentationRevision
   if(!reuseTable) {await page.goto(`${base}/game/test-room`);reuseTable=true}
   else visualSocket.send(JSON.stringify({type:'snapshot',view}))
   await page.waitForFunction(({count,players})=>document.querySelectorAll('.hand button').length===count && document.querySelectorAll('.opponents .seat').length===players-1,{count,players})
   await page.waitForTimeout(40)
   await page.locator('.hand button').first().waitFor()
   assert.equal(await page.locator('.hand button').count(), count)
   const bounds = await page.locator('.local-player').boundingBox()
   assert(
    bounds.y + bounds.height <= height + 1,
    `footer ${width}/${count}/${players}`,
   )
   await page.evaluate(() => document.fonts.ready)
   assert(await page.evaluate(() => document.fonts.check('550 16px MatchoNunito')), 'Nunito not loaded')
   await page.locator('[data-scene-ready="true"]').waitFor()
   const handBox=await page.locator('.hand').boundingBox()
   const opponentBounds=await page.locator('.opponents').boundingBox()
   const tableBounds=await page.locator('.table').boundingBox()
   assert(opponentBounds.y + opponentBounds.height <= tableBounds.y + 1, `opponents crowd table ${width}/${players}`)
   assert(handBox.y+24 >= tableBounds.y+tableBounds.height+8, 'hand covers piles')
   assert(await page.locator('.hand').evaluate(el=>el.scrollWidth<=el.clientWidth+1), 'horizontal hand overflow')
   const slots=await page.locator('.hand button').evaluateAll(nodes=>nodes.map(n=>({id:n.dataset.cardId,row:Number(n.dataset.row),x:parseFloat(n.style.left),hit:parseFloat(n.style.width),width:Number(n.dataset.cardWidth)})))
   for(let row=0;row<Math.ceil(count/8);row++) {
    const cards=slots.filter(c=>c.row===row), first=cards[0], last=cards.at(-1)
    assert(cards.length<=8 && first.x>=15.9 && last.x+last.width<=width-15.9, 'row limit or margins')
    assert(Math.abs(first.x-(width-last.x-last.width))<1, 'incomplete row not centered')
    assert(cards.every((c,i)=>!i || Math.abs(c.x-cards[i-1].x-cards[i-1].hit)<1),'overlapping touch strips')
   }
   if(count<=8)assert(await page.locator('.hand').evaluate(el=>el.scrollHeight<=el.clientHeight+1),'normal hand scrolls')
   if(count===30&&players===10) {
    const session=await context.newCDPSession(page), x=width-35, y=handBox.y+handBox.height-12
    await session.send('Input.dispatchTouchEvent',{type:'touchStart',touchPoints:[{x,y}]})
    for(let step=1;step<=8;step++) {await session.send('Input.dispatchTouchEvent',{type:'touchMove',touchPoints:[{x,y:y-step*(handBox.height-35)/8}]});await page.waitForTimeout(16)}
    await session.send('Input.dispatchTouchEvent',{type:'touchEnd',touchPoints:[]})
    await page.waitForTimeout(250)
    assert(await page.locator('.hand').evaluate(el=>el.scrollTop)>20,'vertical touch swipe did not scroll hand')
    await session.detach()
   }
   await page.locator('.hand button').last().scrollIntoViewIfNeeded()
   const last=await page.locator('.hand button').last().boundingBox()
   assert(last.y>=handBox.y-1 && last.y+last.height<=handBox.y+handBox.height+1,'last card unreachable')
   if(count===30&&players===10)await page.screenshot({path:new URL(`hand-last-${width}.png`,output).pathname})
   await page.locator('.hand').evaluate(el=>{el.scrollTop=0})
   const playable=page.locator('.hand button.playable').first()
   await playable.tap()
   await page.waitForFunction(()=>Number(document.querySelector('.hand button[aria-pressed="true"]')?.dataset.visualLift)<=-17.9)
   assert(Number(await playable.getAttribute('data-visual-lift'))<=-17.9,'Pixi selection did not rise')
   const selected=await playable.boundingBox()
   assert(selected.y-18>=handBox.y,'selected top is clipped')
   const table = await page.locator('.table').boundingBox()
   assert(table.height >= 95, `piles crowded ${width}/${count}/${players}`)
   assert.equal(
    await page.evaluate(
     () => document.documentElement.scrollWidth <= innerWidth,
    ),
    true,
   )
   const sides = await page.locator('.side-opponent').all()
   const piles = await page.locator('.table-card,.draw').all()
   for (const side of sides)
    for (const pile of piles) {
     const a = await side.boundingBox(),
      b = await pile.boundingBox()
     assert(
      !(
       a.x < b.x + b.width &&
       a.x + a.width > b.x &&
       a.y < b.y + b.height &&
       a.y + a.height > b.y
      ),
      `opponent covers pile at ${width}/${players}`,
     )
    }
   if ([8,9,17,30].includes(count) || players === 10)
    await page.screenshot({
     path: new URL(`game-${width}-${players}p-${count}cards.png`, output)
      .pathname,
    })
   results.push({ width, height, cards: count, players, passed: true })
  }
 for (const state of ['other-turn', 'selected', 'picker', 'picker-chosen', 'result-vote', 'result-all', 'reconnection']) {
  view = game(7, 2, state.startsWith('result'))
  if(state === 'other-turn') {view.my_turn=false;view.players[0].current=false;view.players[1].current=true;view.hand.forEach(c=>c.Playable=false)}
  if(state.startsWith('picker')) {view.phase=2;view.top={ID:'wild',Color:0,Rank:13};view.hand.forEach(c=>c.Playable=false)}
  if(state === 'result-all') view.rematch={...view.rematch,revision:3,accepted:['seat-0','seat-1']}
  await page.goto(`${base}/game/test-room`)
  await page.locator('.game-v2').waitFor()
  await page.evaluate(() => document.fonts.ready)
  await page.locator('[data-scene-ready="true"]').waitFor()
  if(state === 'selected') {await page.locator('.hand button').first().tap();await page.waitForTimeout(210)}
  if(state === 'picker-chosen') {
   await page.getByRole('button',{name:'Amarelo',exact:true}).tap()
   const colors = await page.locator('.color-choice').evaluateAll(nodes=>nodes.map(n=>getComputedStyle(n).backgroundColor))
   assert(colors.every(c=>c==='rgb(255, 189, 16)'), 'chosen petals must share color')
  }
  if(state === 'result-vote') {await page.getByRole('button',{name:'Aceitar revanche',exact:true}).tap();await page.getByLabel('Gabriel aceitou a revanche').waitFor()}
  if(state === 'reconnection') await page.reload()
  await page.screenshot({path:new URL(`${state}-${width}.png`,output).pathname})
  if(state === 'picker-chosen') {await page.waitForTimeout(450);assert((await page.locator('.game-v2').getAttribute('data-discard-asset')).endsWith('yellow_wild.png'));await page.screenshot({path:new URL(`wild-confirmed-${width}.png`,output).pathname})}
 }
 // Result lists retain all participants and scroll independently at capacity.
 for(const players of [2,3,4,5,6,7,8,9,10]) {
  view=game(7,players,true)
  await page.goto(`${base}/game/test-room`)
  await page.locator('.result-row').last().waitFor()
  await page.evaluate(() => document.fonts.ready)
  assert.equal(await page.locator('.result-row').count(),players)
  if(players===10) await page.screenshot({path:new URL(`result-10p-${width}.png`,output).pathname})
  await page.locator('.result-row').last().scrollIntoViewIfNeeded()
  const last=await page.locator('.result-row').last().boundingBox(), list=await page.locator('.result-list').boundingBox()
  assert(last.y>=list.y-1 && last.y+last.height<=list.y+list.height+1, 'last result participant unreachable')
  assert(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth), 'result overflow')
 }
 view=game(7,2,true);view.players[0].position=2;view.players[1].position=1
 await page.goto(`${base}/game/test-room`)
 await page.getByRole('heading',{name:'Você perdeu',exact:true}).waitFor()
 await page.evaluate(() => document.fonts.ready)
 await page.screenshot({path:new URL(`result-lost-${width}.png`,output).pathname})
 // Touch every exposed card on a narrow viewport, including after another rises.
 if(width === 320) for(const count of [7,30]) {
  view=game(count,2);view.hand.forEach(c=>c.Playable=true)
  await page.goto(`${base}/game/test-room`)
  await page.locator('.hand button').last().waitFor()
  for(let i=0;i<count;i++) {
   const button=page.locator('.hand button').nth(i)
   await button.scrollIntoViewIfNeeded()
   await button.tap()
   assert.equal(await button.getAttribute('aria-pressed'),'true',`touch card ${i}/${count}`)
  }
 }
 view = game(30, 10)
 await page.goto(`${base}/game/test-room?safe=yes`)
 await page.locator('.hand button').first().waitFor()
 const safeFooter = await page.locator('.local-player').boundingBox()
 await page.screenshot({path:new URL(`game-safe-area-${width}.png`,output).pathname})
 if(safeFooter.y+safeFooter.height>height-12+1)console.log('Safe layout bounds',await page.locator('.game').evaluate(el=>[...el.children].map(n=>({class:n.className,y:n.getBoundingClientRect().top,height:n.getBoundingClientRect().height}))))
 assert(
  safeFooter.y + safeFooter.height <= height - 12 + 1,
  `safe-area controls clipped at ${width}`,
 )
 await page.screenshot({
  path: new URL(`game-safe-area-${width}.png`, output).pathname,
 })
 assert.deepEqual(errors, [])
 await context.close()
}
await writeFile(
 new URL('checks.json', output),
 JSON.stringify(
  {
   generated_at: new Date().toISOString(),
   browser: browser.version(),
   device_scale_factor: 2,
   dimensions,
   game_cases: results,
   ranking_state_cases: 20,
   safe_area_cases: 4,
   states_per_viewport: ['my-turn', 'other-turn', 'selection', 'picker', 'picker-chosen', 'wild-server-confirmed', 'result', 'one-vote', 'all-votes', 'reconnection'],
   narrow_touch_card_selections: 37,
   result_list_cases: 36,
   loss_states: 4,
   local_fonts: ['MatchoNunito 550', 'MatchoLexend 900'],
   integration: 'Presentation snapshots through the fixture socket; real transport, engine, PostgreSQL and production CSP checked separately in production/browser.json',
  },
  null,
  2,
 ),
)
await browser.close()
console.log(
 `Screenshots captured; ${results.length} card/player/viewport cases passed. Test fixtures only.`,
)
