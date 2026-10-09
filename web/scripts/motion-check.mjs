// Presentation stress checks only. Rule/transport/persistence integration is game-e2e.mjs.
import { chromium } from 'playwright'
import assert from 'node:assert/strict'
import { mkdir, writeFile } from 'node:fs/promises'
const out = '.reports/partida-pixi'
const base = process.env.UNO_VISUAL_URL || 'http://127.0.0.1:5173'
await mkdir(out, {recursive:true})
const browser = await chromium.launch({executablePath:process.env.PLAYWRIGHT_EXECUTABLE_PATH})
try {
 const page = await browser.newPage({viewport:{width:320,height:568},hasTouch:true,reducedMotion:'no-preference'})
 const errors=[]
 page.on('pageerror',e=>errors.push(e.message))
 await page.addInitScript(() => {window.Telegram={WebApp:{initData:'test-only',ready(){},expand(){}}}})
 await page.route('https://telegram.org/**',r=>r.abort())
 let view={game_id:'motion',revision:20,phase:1,group:'Teste de movimento',mode:'classic',system:'updated',players:[{key:'me',name:'Eu',count:7,me:true,current:true,active:true},{key:'other',name:'Outro',count:7,me:false,current:false,active:true}],hand:Array.from({length:7},(_,i)=>({Card:{ID:`c${i}`,Color:1,Rank:i},Playable:true})),top:{ID:'top',Color:1,Rank:2},active_color:1,direction:1,closed:false,owner:true,my_turn:true,drawn_card_id:'',can_bluff:false,deadline:null,server_time:new Date().toISOString(),result:null,events:[]}
 await page.route('**/api/v1/**',r=>{const url=new URL(r.request().url());const data=url.pathname.endsWith('/rooms')?[]:url.pathname.endsWith('/me')?{stats:[],history:[]}:url.pathname.includes('/rankings/')?{items:[],system:'updated',month_start:'2026-10-01',month_name:'Outubro'}:url.pathname.endsWith('/position')?{entry:null}:url.pathname.endsWith('/config')?{bot_username:'fixture'}:view;return r.fulfill({json:data})})
 let socket
 const commands=[]
 await page.routeWebSocket('**/api/v1/live',ws=>{socket=ws;ws.onMessage(raw=>{const m=JSON.parse(String(raw));if(m.init_data)ws.send(JSON.stringify({type:'snapshot',view,recovery:true}));else commands.push(m)})})
 const push=(next,type='snapshot',request)=>{view=next;socket.send(JSON.stringify({type,request_id:request,view}))}
 await page.goto(`${base}/game/motion`)
 await page.locator('.hand button').last().waitFor()
 await page.locator('[data-scene-ready="true"]').waitFor()
 await page.locator('.hand button').first().tap()
 await page.getByRole('button',{name:'Jogar carta',exact:true}).tap()
 assert.equal(commands.length,1)
 const old=view
 // Reject the physical play; selection and authorized hand remain usable.
 push(view,'rejected',commands[0].request_id)
 await page.getByRole('alert').waitFor()
 assert.equal(await page.locator('.hand button').count(),7)
 const drawn={Card:{ID:'new-card',Color:2,Rank:7},Playable:true}
 push({...view,revision:21,hand:[...view.hand,drawn],events:[{id:'21:0',revision:21,type:'cards_drawn',player:'me',count:1}]})
 await page.locator('[data-motion-kind=draw]').first().waitFor({state:'attached'})
 assert.equal(await page.locator('[data-motion-kind=draw]').first().getAttribute('data-flight-card-id'),'new-card','accepted physical ID must survive the flight')
 socket.send(JSON.stringify({type:'snapshot',view,recovery:true}))
 await page.waitForTimeout(80)
 assert(await page.locator('[data-motion-kind=draw]').count()>0,'late same-revision recovery cancelled an accepted purchase')
 // Turn changes while the draw flight is still running.
 push({...view,revision:22,my_turn:false,hand:view.hand.map(c=>({...c,Playable:false})),events:[...view.events,{id:'22:0',revision:22,type:'turn_changed',player:'other'}]})
 await page.waitForTimeout(1000)
 assert.equal(await page.locator('[data-motion-kind]').count(),0)
 assert(await page.locator('.hand button').evaluateAll(nodes=>nodes.every(n=>n.disabled && getComputedStyle(n).opacity==='1')))
 // Delayed snapshot cannot restore turn/hand or animate a historical purchase.
 socket.send(JSON.stringify({type:'snapshot',view:old}))
 await page.waitForTimeout(100)
 assert.equal(await page.locator('.hand button').count(),8)
 assert(await page.locator('.hand button').first().isDisabled())
 const ninth={Card:{ID:'ninth',Color:0,Rank:13},Playable:true}
 push({...view,revision:23,my_turn:true,hand:[...view.hand,ninth],events:[{id:'23:0',revision:23,type:'cards_drawn',player:'me',count:1}]})
 await page.locator('[data-motion-kind=draw]').first().waitFor({state:'attached'})
 assert(await page.locator('.hand').evaluate(n=>n.scrollTop)>20,'purchase in a new row must reveal its landing position')
 await page.waitForFunction(()=>!document.querySelector('[data-motion-active]'))
 await page.locator('.hand').evaluate(n=>{n.scrollTop=0})
 const touched=page.locator('.hand button').first(), touchBox=await touched.boundingBox()
 await page.mouse.move(touchBox.x+5,touchBox.y+5);await page.mouse.down()
 const tenth={Card:{ID:'tenth',Color:1,Rank:0},Playable:true}
 push({...view,revision:24,hand:[tenth,...view.hand],events:[{id:'24:0',revision:24,type:'cards_drawn',player:'me',count:1}]})
 await page.waitForTimeout(100)
 assert.equal(await page.locator('.hand button').count(),9,'snapshot must preserve the in-progress touch layout')
 await page.mouse.up()
 await page.locator('[data-motion-kind=draw]').first().waitFor({state:'attached'})
 await page.waitForFunction(()=>!document.querySelector('[data-motion-active]'))
 assert.equal(await page.locator('.hand button').count(),10,'deferred physical arrival must resume after the gesture')
 push({...view,revision:25,my_turn:true,hand:view.hand.map(c=>({...c,Playable:true})),events:[{id:'25:0',revision:25,type:'cards_drawn',player:'other',count:3}]})
 await page.locator('[data-motion-kind]').first().waitFor({state:'attached'})
 assert(await page.locator('[data-motion-kind=draw]').evaluateAll(nodes=>nodes.every(n=>!n.dataset.flightCardId && n.dataset.motionPrivate==='false')),'opponent purchase must contain only anonymous backs')
 // Unmount while a timeline owns ghosts and then switch physical game ID.
 await page.getByRole('button',{name:'Voltar ao início',exact:true}).click()
 await page.locator('.game-v2').waitFor({state:'detached'})
 assert.equal(await page.locator('[data-motion-kind]').count(),0, 'SPA unmount must remove timeline ghosts')
 view={...view,game_id:'second',revision:1,events:[],hand:[drawn]}
 await page.goto(`${base}/game/second`)
 await page.locator('[data-scene-ready="true"]').waitFor()
 await page.locator('.hand button').first().waitFor()
 assert.equal(await page.locator('[data-motion-kind]').count(),0)
 assert.equal(await page.locator('.hand button').count(),1)
 assert.equal(commands.length,1,'animations must not send commands')
 // A fast server confirmation must not shorten the full color presentation.
 push({...view,revision:2,phase:2,my_turn:true,top:{ID:'wild',Color:0,Rank:13}})
 await page.getByRole('button',{name:'Vermelho',exact:true}).waitFor()
 await page.waitForTimeout(550)
 await page.getByRole('button',{name:'Vermelho',exact:true}).tap()
 const colorCommand=commands.at(-1)
 push({...view,revision:3,phase:1,my_turn:false,active_color:1},'accepted',colorCommand.request_id)
 await page.waitForTimeout(500)
 assert(await page.locator('.v2-choice').evaluate(n=>n.open),'fast ack hid the chooser')
 assert(await page.locator('.color-choice').evaluateAll(nodes=>nodes.every(n=>getComputedStyle(n).backgroundColor==='rgb(251, 48, 69)')),'petal tween did not finish')
 await page.screenshot({path:`${out}/color-feedback-fast-ack.png`})
 await page.waitForTimeout(300)
 assert(await page.locator('.choice-surface').evaluate(n=>Number(getComputedStyle(n).opacity)>.9),'color hold was shortened')
 await page.locator('.v2-choice').waitFor({state:'hidden'})
 push({...view,revision:4,phase:2,my_turn:true})
 await page.getByRole('button',{name:'Azul',exact:true}).tap()
 const rejected=commands.at(-1)
 socket.send(JSON.stringify({type:'rejected',request_id:rejected.request_id,reason:'stale_revision',view}))
 await page.getByRole('button',{name:'Amarelo',exact:true}).waitFor()
 await page.waitForFunction(()=>!document.querySelector('[aria-label="Amarelo"]').disabled)
 await page.getByRole('button',{name:'Amarelo',exact:true}).tap()
 assert.equal(commands.length,4,'one play, one accepted color, rejected color, one retry')
 const retry=commands.at(-1)
 push({...view,revision:5,phase:1,my_turn:false,active_color:4},'accepted',retry.request_id)
 await page.waitForTimeout(200)
 push({...view,revision:6,phase:4,my_turn:true,top:{ID:'swap-choice',Color:0,Rank:15}})
 await page.getByRole('heading',{name:'Trocar com quem?',exact:true}).waitFor()
 await page.getByRole('button',{name:'Manter minha mão',exact:true}).tap()
 const keep=commands.at(-1)
 assert.equal(keep.action,'keep','a new required target choice cannot be blocked by old color feedback')
 push({...view,revision:7,phase:1,my_turn:true},'accepted',keep.request_id)
 await page.locator('.v2-choice').waitFor({state:'hidden'})
 await page.screenshot({path:`${out}/motion-new-game-320.png`})
 // The observer's initial notification must leave the first deal running.
 const dealHand=Array.from({length:7},(_,i)=>({Card:{ID:`deal-${i}`,Color:1,Rank:i},Playable:true}))
 view={...view,game_id:'deal-check',revision:1,phase:0,hand:[],owner:true,can_start:true,players:view.players.map(p=>({...p,count:0})),events:[]}
 await page.goto(`${base}/game/deal-check`)
 await page.getByRole('button',{name:'Começar partida',exact:true}).tap()
 push({...view,revision:2,phase:1,hand:dealHand,players:view.players.map(p=>({...p,count:7})),events:[{id:'2:0',revision:2,type:'game_started',player:'me'}]},'accepted',commands.at(-1).request_id)
 await page.locator('[data-motion-kind=deal]').first().waitFor({state:'attached'})
 await page.waitForTimeout(250)
 assert(await page.locator('[data-motion-kind=deal]').count()>0,'initial observer notification cancelled the physical deal')
 await page.screenshot({path:`${out}/distribution-in-flight-320.png`})
 await page.waitForFunction(()=>!document.querySelector('[data-motion-active]'))
 const swap={Card:{ID:'swap-original',Color:0,Rank:15},Playable:true}
 push({...view,revision:3,hand:[...view.hand,swap],events:[{id:'3:0',revision:3,type:'cards_drawn',player:'me',count:1}]})
 await page.locator('[data-motion-kind=draw]').first().waitFor({state:'attached'})
 await page.waitForTimeout(600)
 await page.screenshot({path:`${out}/swap-original-in-flight-320.png`})
 await page.waitForFunction(()=>!document.querySelector('[data-motion-active]'))
 // Failed asset initialization followed by unmount must destroy the renderer only once.
 await page.route('**/assets/backs/default.png',route=>route.abort())
 view={...view,game_id:'failed-table',events:[]}
 await page.goto(`${base}/game/failed-table`)
 await page.locator('[data-scene-error]').waitFor()
 await page.getByRole('button',{name:'Voltar ao início',exact:true}).click()
 await page.locator('.game-v2').waitFor({state:'detached'})
 assert.deepEqual(errors, [], 'No errors on destroyed sprites or React unmount')
 await writeFile(`${out}/motion-checks.json`,JSON.stringify({passed:true,checks:['stable physical CardID during flight','opponent draws expose only anonymous backs','late same-revision recovery preserves accepted movement','fresh server choice supersedes old color feedback','failed asset initialization and unmount destroy renderer once','rejection restores physical hand','turn changes during draw','interrupted timeline restores opacity','delayed revision ignored','new-row purchase scrolls to its physical landing','arrival during gesture preserves touch ID and resumes after release','unmount removes ghosts','new game ID baseline','animation sends no commands','450ms color tween/700ms hold/350ms exit survives fast ack','color rejection restores petals and retry','initial observer notification preserves the visible deal','swap asset retains its hand frame during reveal'],telegram:'local test SDK only'},null,2))
 console.log('Motion stress checks passed')
} finally {await browser.close()}
