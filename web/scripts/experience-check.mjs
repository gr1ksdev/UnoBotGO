// Local presentation fixtures only: run with an existing Vite server.
import { chromium } from 'playwright'
import assert from 'node:assert/strict'
import { mkdir, writeFile } from 'node:fs/promises'
const out=process.env.UNO_EXPERIENCE_OUTPUT || '.reports/experiencia-completa/interaction'
const base=process.env.UNO_VISUAL_URL || 'http://127.0.0.1:5173'
await mkdir(out,{recursive:true})
const browser=await chromium.launch({executablePath:process.env.PLAYWRIGHT_EXECUTABLE_PATH})
try {
 const context=await browser.newContext({viewport:{width:390,height:844},hasTouch:true,reducedMotion:'no-preference'})
 const page=await context.newPage(),errors=[],audio=[],commands=[]
 page.setDefaultTimeout(8000)
 page.on('pageerror',e=>errors.push(e.message))
 page.on('response',r=>{if(r.url().includes('/assets/audio/'))audio.push({file:r.url().split('/').at(-1),status:r.status()})})
 await page.addInitScript(()=>{
  window.__drawCalls=0
  for(const Type of [window.WebGLRenderingContext,window.WebGL2RenderingContext].filter(Boolean)) {
   const original=Type.prototype.drawElements
   Type.prototype.drawElements=function(...args){window.__drawCalls++;return original.apply(this,args)}
  }
  window.Telegram={WebApp:{initData:'fixture-only',ready(){},expand(){},isVersionAtLeast(){return true},HapticFeedback:{impactOccurred(){window.__haptics=(window.__haptics||0)+1}}}}
 })
 await page.route('https://telegram.org/**',r=>r.abort())
 let view={game_id:'experience',revision:10,phase:1,group:'Prévia local · dados de teste',mode:'classic',system:'updated',players:[{key:'me',name:'Gabriel',count:7,me:true,current:true,active:true},{key:'other',name:'Rafael',count:7,me:false,current:false,active:true}],hand:Array.from({length:7},(_,i)=>({Card:{ID:`c${i}`,Color:1,Rank:i},Playable:true})),top:{ID:'top',Color:1,Rank:2},active_color:1,direction:1,closed:false,owner:true,my_turn:true,drawn_card_id:'',can_bluff:false,deadline:null,server_time:new Date().toISOString(),result:null,events:[]}
 await page.route('**/api/v1/**',r=>{const u=new URL(r.request().url());return r.fulfill({json:u.pathname.endsWith('/rooms')?[]:u.pathname.endsWith('/me')?{stats:[],history:[]}:u.pathname.endsWith('/config')?{bot_username:'fixture'}:view})})
 let socket
 await page.routeWebSocket('**/api/v1/live',ws=>{socket=ws;ws.onMessage(raw=>{const m=JSON.parse(String(raw));if(m.init_data)ws.send(JSON.stringify({type:'snapshot',view,recovery:true}));else commands.push(m)})})
 const push=(next,type='snapshot',request_id)=>{view=next;socket.send(JSON.stringify({type,request_id,view}))}
 await page.goto(`${base}/game/experience`)
 await page.locator('[data-scene-ready=true]').waitFor()
 assert.equal(audio.length,0,'audio must be opt-in, not loaded while muted')
 await page.getByRole('button',{name:'Opções da partida',exact:true}).click()
 await page.getByRole('button',{name:'Ativar som',exact:true}).click()
 await page.waitForFunction(()=>JSON.parse(localStorage.getItem('unobotgo-table-feedback-v1')).sound===true)
 await page.getByRole('button',{name:'Opções da partida',exact:true}).click()
 const cdp=await context.newCDPSession(page)
 const touch=async(type,x,y)=>cdp.send('Input.dispatchTouchEvent',{type,touchPoints:type==='touchEnd'||type==='touchCancel'?[]:[{x,y,radiusX:3,radiusY:3}]})
 const startDrag=async()=>{
  const card=page.locator('[data-card-id=c3]')
  if(await card.getAttribute('aria-pressed')!=='true')await card.tap()
  await page.waitForTimeout(500)
  const p=await card.boundingBox(),start={x:p.x+5,y:p.y+35}
  await touch('touchStart',start.x,start.y)
  return start
 }
 // A cancelled physical touch returns the card and sends no command.
 let p=await startDrag()
 await touch('touchMove',p.x+40,p.y-45)
 await page.waitForTimeout(150)
 await page.locator('[data-drag-card-id=c3]').waitFor()
 await touch('touchCancel',0,0)
 await page.waitForTimeout(650)
 assert.equal(commands.length,0)
 assert.equal(await page.locator('[data-drag-card-id]').count(),0)
 await page.screenshot({path:`${out}/card-return.png`})
 // Drop on the discard sends one physical ID, without the following click replaying it.
 p=await startDrag()
 const discard=await page.locator('.discard').boundingBox(),target={x:discard.x+discard.width/2,y:discard.y+discard.height/2}
 for(let i=1;i<=8;i++){await touch('touchMove',p.x+(target.x-p.x)*i/8,p.y+(target.y-p.y)*i/8);await page.waitForTimeout(25)}
 await page.screenshot({path:`${out}/card-drag.png`})
 await touch('touchEnd',0,0)
 await page.waitForTimeout(100)
 assert.equal(commands.length,1,'drag must emit exactly one command')
 assert.equal(commands[0].card_id,'c3')
 assert.equal(commands[0].action,'play')
 push(view,'rejected',commands[0].request_id)
 await page.getByRole('alert').waitFor()
 // A turn update during an ongoing touch cancels it before any command can escape.
 p=await startDrag();await touch('touchMove',p.x,p.y-35)
 await page.locator('[data-drag-card-id=c3]').waitFor()
 push({...view,revision:11,my_turn:false,hand:view.hand.map(c=>({...c,Playable:false})),events:[{id:'11:0',revision:11,type:'turn_changed',player:'other'}]})
 await page.locator('[data-drag-card-id]').waitFor({state:'detached'})
 await touch('touchMove',target.x,target.y);await touch('touchEnd',0,0)
 assert.equal(commands.length,1,'obsolete gesture must not send a play')
 await page.waitForFunction(()=>!document.querySelector('[data-motion-active]'))
 // Public opponent play reveals its front through a flip, never through private hand data.
 push({...view,revision:12,top:{ID:'opponent-card',Color:2,Rank:11},direction:-1,events:[{id:'12:0',revision:12,type:'card_played',player:'other',card_id:'opponent-card'},{id:'12:1',revision:12,type:'direction_changed',player:'other'}]})
 await page.locator('[data-motion-kind=play]').waitFor({state:'attached'})
 assert.equal(await page.locator('[data-motion-kind=play]').getAttribute('data-flight-face'),'back')
 await page.waitForTimeout(280)
 assert.equal(await page.locator('[data-motion-kind=play]').getAttribute('data-flight-face'),'front')
 await page.screenshot({path:`${out}/opponent-reveal.png`})
 await page.waitForFunction(()=>!document.querySelector('[data-motion-active]'))
 push({...view,revision:13,events:[{id:'13:0',revision:13,type:'player_skipped',player:'me'}]})
 await page.locator('[data-action-effect=skip]').waitFor({state:'attached'})
 await page.screenshot({path:`${out}/skip-highlight.png`})
 await page.waitForFunction(()=>!document.querySelector('[data-motion-active]'))
 socket.send(JSON.stringify({type:'snapshot',view,recovery:true}))
 await page.waitForTimeout(100)
 assert.equal(await page.locator('[data-action-effect]').count(),0,'same-revision recovery must not repeat action effects')
 await page.getByRole('button',{name:'Opções da partida',exact:true}).click()
 await page.getByRole('button',{name:'Desativar som',exact:true}).click()
 const audioBefore=audio.length
 await page.getByRole('button',{name:'Opções da partida',exact:true}).click()
 push({...view,revision:14,my_turn:true,hand:view.hand.map(c=>({...c,Playable:true})),events:[]})
 await page.locator('[data-card-id=c2]').tap()
 await page.waitForTimeout(450)
 assert.equal(audio.length,audioBefore,'muting must stop further audio requests')
 assert(audio.length>0 && audio.every(r=>r.status===200 || r.status===206),'original sounds must be served locally')
 assert(await page.evaluate(()=>window.__haptics>0),'physical interactions must reach optional Telegram haptics')
 await page.emulateMedia({reducedMotion:'reduce'})
 push({...view,revision:15,events:[{id:'15:0',revision:15,type:'player_skipped',player:'me'}]})
 await page.waitForTimeout(100)
 assert.equal(await page.locator('[data-action-effect]').count(),0,'reduced motion must not animate effects')
 await page.emulateMedia({reducedMotion:'no-preference'})
 await page.waitForTimeout(80) // Let the MediaQueryList change cancel the old presentation first.
 const swapped=Array.from({length:16},(_,i)=>({Card:{ID:`swapped-${i}`,Color:1+i%4,Rank:i%10},Playable:true}))
 push({...view,revision:16,hand:swapped,players:view.players.map(p=>({...p,count:p.me?16:7})),events:[{id:'16:0',revision:16,type:'hands_swapped',player:'me',target:'other'}]})
 await page.locator('[data-motion-kind=swap]').first().waitFor({state:'attached'})
 assert(await page.locator('[data-motion-kind=swap]').evaluateAll(nodes=>nodes.every(n=>n.dataset.flightCardId?.startsWith('swapped-') && n.dataset.motionPrivate==='true')),'hand swap may animate only the received private IDs')
 await page.waitForFunction(()=>!document.querySelector('[data-motion-active]'))
 assert.equal(await page.locator('.hand button').count(),16)
 assert.equal(await page.locator('.hand').evaluate(n=>n.scrollWidth<=n.clientWidth),true)
 await page.locator('.hand button').last().scrollIntoViewIfNeeded()
 assert(await page.locator('.hand button').last().isVisible(),'last received swap card must remain reachable')
 await page.waitForTimeout(300)
 const idle=await page.evaluate(()=>window.__drawCalls)
 assert(idle>0,'the scene must actually render')
 await page.waitForTimeout(450)
 assert.equal(await page.evaluate(()=>window.__drawCalls),idle,'idle table must not continuously redraw on WebGL')
 assert.deepEqual(errors,[])
 await writeFile(`${out}/checks.json`,JSON.stringify({passed:true,checks:['real touch drag to discard emits one CardID command','touch cancel returns card without command','turn change cancels captured drag','public opponent reveal flips back/front','confirmed reverse/skip effects do not replay on recovery','audio opt-in, local files, persistent mute','optional Telegram haptic API','reduced motion suppresses effects','sixteen-card hand swap uses received own IDs and preserves rows/last-card access','idle table stops WebGL draw calls'],audio,telegram:'fixture only; no live accounts'},null,2))
 console.log('Complete experience interaction checks passed')
}finally{await browser.close()}
