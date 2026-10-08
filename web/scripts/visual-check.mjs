// Browser-only test fixtures. Nothing in this harness is bundled into the app.
import { chromium } from 'playwright'
import { mkdir, writeFile } from 'node:fs/promises'
import assert from 'node:assert/strict'
const output = new URL('../../.reports/redesign-mobile/', import.meta.url)
await mkdir(output, { recursive: true })
const browser = await chromium.launch({
 headless: true,
 executablePath: process.env.PLAYWRIGHT_EXECUTABLE_PATH || undefined,
})
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
  hand: closed
   ? []
   : Array.from({ length: count }, (_, i) => ({
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
 const page = await context.newPage()
 await page.route('https://telegram.org/**', (route) => route.abort())
 const errors = []
 page.on('pageerror', (e) => errors.push(e.message))
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
  ws.onMessage((raw) => {
   const m = JSON.parse(String(raw))
   if (m.init_data) ws.send(JSON.stringify({ type: 'snapshot', view }))
   else
    ws.send(
     JSON.stringify({
      type: 'accepted',
      request_id: m.request_id,
      view: { ...view, revision: view.revision + 1 },
     }),
    )
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
   `http://127.0.0.1:5173/${screen === 'groups' ? 'ranking?tab=groups' : screen === 'game' || screen === 'result' ? 'game/test-room' : screen}`,
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
  await page.goto('http://127.0.0.1:5173/ranking')
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
 for (const count of [1, 2, 7, 15, 30])
  for (const players of [2, 3, 4, 5, 6, 10]) {
   view = game(count, players)
   await page.goto('http://127.0.0.1:5173/game/test-room')
   await page.locator('.hand button').first().waitFor()
   assert.equal(await page.locator('.hand button').count(), count)
   const bounds = await page.locator('.local-player').boundingBox()
   assert(
    bounds.y + bounds.height <= height + 1,
    `footer ${width}/${count}/${players}`,
   )
   const card = await page.locator('.hand button').first().boundingBox()
   assert(card.width >= 72 && card.height >= 100, 'cards shrunk')
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
   if (count === 30 || players === 10)
    await page.screenshot({
     path: new URL(`game-${width}-${players}p-${count}cards.png`, output)
      .pathname,
    })
   results.push({ width, height, cards: count, players, passed: true })
  }
 view = game(30, 10)
 await page.goto('http://127.0.0.1:5173/game/test-room?safe=yes')
 await page.locator('.hand button').first().waitFor()
 const safeFooter = await page.locator('.local-player').boundingBox()
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
  },
  null,
  2,
 ),
)
await browser.close()
console.log(
 `Screenshots captured; ${results.length} card/player/viewport cases passed. Test fixtures only.`,
)
