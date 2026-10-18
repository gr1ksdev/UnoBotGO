// A recorded local fixture for reviewing presentation. No Telegram account/backend.
import { chromium } from 'playwright'
import { mkdir, writeFile } from 'node:fs/promises'
const out=process.env.UNO_PREVIEW_OUTPUT || '.reports/experiencia-completa/preview'
const base=process.env.UNO_VISUAL_URL || 'http://127.0.0.1:5173'
await mkdir(out,{recursive:true})
const browser=await chromium.launch({executablePath:process.env.PLAYWRIGHT_EXECUTABLE_PATH})
try{
 const context=await browser.newContext({viewport:{width:390,height:844},hasTouch:true,reducedMotion:'no-preference',recordVideo:{dir:out,size:{width:390,height:844}}})
 await context.addInitScript(()=>{window.Telegram={WebApp:{initData:'preview-only',ready(){},expand(){}}}})
 const page=await context.newPage()
 await page.route('https://telegram.org/**',r=>r.abort())
 const commands=[],errors=[]
 page.on('pageerror',e=>errors.push(e.message))
 let view={game_id:'preview',revision:1,phase:0,group:'Demonstração local',mode:'classic',system:'updated',players:[{key:'me',name:'Gabriel',count:0,me:true,current:true,active:true},{key:'other',name:'Rafael',count:0,me:false,current:false,active:true}],hand:[],top:{ID:'top',Color:1,Rank:2},active_color:1,direction:1,closed:false,owner:true,can_start:true,my_turn:true,drawn_card_id:'',can_bluff:false,deadline:null,server_time:new Date().toISOString(),result:null,events:[]}
 await page.route('**/api/v1/**',r=>{const u=new URL(r.request().url());return r.fulfill({json:u.pathname.endsWith('/rooms')?[]:u.pathname.endsWith('/me')?{stats:[],history:[]}:u.pathname.endsWith('/config')?{bot_username:'fixture'}:view})})
 let socket
 await page.routeWebSocket('**/api/v1/live',ws=>{socket=ws;ws.onMessage(raw=>{const m=JSON.parse(String(raw));if(m.init_data)ws.send(JSON.stringify({type:'snapshot',view,recovery:true}));else commands.push(m)})})
 const push=(next,accepted=false)=>{view=next;socket.send(JSON.stringify({type:accepted?'accepted':'snapshot',request_id:accepted?commands.at(-1)?.request_id:undefined,view}))}
 const pause=ms=>page.waitForTimeout(ms)
 await page.goto(`${base}/game/preview`)
 await page.getByRole('button',{name:'Começar partida',exact:true}).tap()
 const hand=[[1,2],[1,3],[4,4],[3,7],[2,1],[0,13],[0,14]].map(([Color,Rank],i)=>({Card:{ID:`c${i}`,Color,Rank},Playable:Color===1 || Rank>=13}))
 push({...view,revision:2,phase:1,hand,players:view.players.map(p=>({...p,count:7})),events:[{id:'2:0',revision:2,type:'game_started',player:'me'}]},true)
 await page.locator('[data-scene-ready=true]').waitFor()
 await page.evaluate(()=>{
  const mark=document.createElement('div');mark.textContent='PRÉVIA LOCAL · DADOS DE TESTE';mark.style.cssText='position:fixed;top:44px;left:50%;transform:translateX(-50%);z-index:40;pointer-events:none;font:9px sans-serif;letter-spacing:1px;color:#bed8e0;background:#0b172bbf;padding:4px 8px;border-radius:10px;white-space:nowrap';document.body.append(mark)
 })
 await pause(2400)
 const card=page.locator('[data-card-id=c1]')
 await card.tap();await pause(500)
 const b=await card.boundingBox()
 await page.mouse.move(b.x+5,b.y+35);await page.mouse.down();await page.mouse.move(b.x+38,b.y-35,{steps:12});await pause(450);await page.mouse.up();await pause(800)
 // Three-card purchase exercises the second row and a visible landing destination.
 await page.getByRole('button',{name:'Comprar carta',exact:true}).tap()
 const extra=Array.from({length:3},(_,i)=>({Card:{ID:`new${i}`,Color:2,Rank:2+i},Playable:i===0}))
 push({...view,revision:3,hand:[...view.hand,...extra],players:view.players.map(p=>({...p,count:p.me?10:p.count})),events:[{id:'3:0',revision:3,type:'cards_drawn',player:'me',count:3}]},true)
 await pause(2300)
 await page.locator('.hand').evaluate(n=>n.scrollTop=0);await pause(450)
 const wild=page.locator('[data-card-id=c5]')
 await wild.tap();await pause(500);await wild.tap()
 push({...view,revision:4,phase:2,top:hand[5].Card,hand:view.hand.filter(c=>c.Card.ID!=='c5'),players:view.players.map(p=>({...p,count:p.me?9:p.count})),events:[{id:'4:0',revision:4,type:'card_played',player:'me',card_id:'c5'}]},true)
 await pause(950)
 await page.getByRole('button',{name:'Amarelo',exact:true}).tap()
 push({...view,revision:5,phase:1,active_color:4,my_turn:false,players:view.players.map(p=>({...p,current:!p.me})),hand:view.hand.map(c=>({...c,Playable:false})),events:[{id:'5:0',revision:5,type:'color_chosen',player:'me'},{id:'5:1',revision:5,type:'turn_changed',player:'other'}]},true)
 await pause(2000)
 push({...view,revision:6,direction:-1,top:{ID:'other-reverse',Color:4,Rank:11},events:[{id:'6:0',revision:6,type:'card_played',player:'other',card_id:'other-reverse'},{id:'6:1',revision:6,type:'direction_changed',player:'other'}]})
 await pause(1700)
 push({...view,revision:7,closed:true,phase:3,players:view.players.map((p,i)=>({...p,active:false,current:false,position:i+1})),result:{id:'preview',score_units:'1000',system:'updated',position:1,status:'scored'},awards:[{key:'me',score_units:'1000',position:1},{key:'other',score_units:'0',position:2}],rematch:{revision:1,required:['me','other'],accepted:[],next_game_id:'',ready:true},events:[{id:'7:0',revision:7,type:'game_finished',player:'me'}]})
 await pause(2000)
 await page.getByRole('button',{name:'Aceitar revanche',exact:true}).tap()
 push({...view,revision:8,rematch:{...view.rematch,accepted:['me']},events:[]},true)
 await pause(1800)
 await page.screenshot({path:`${out}/result.png`})
 await writeFile(`${out}/preview.json`,JSON.stringify({fixture:true,production:false,audioRecorded:false,steps:['deal','selection/tilt/return','three-card draw and new row','wild color held for 1.5 seconds','opponent flip/reverse','winner/crown/rematch acceptance'],errors},null,2))
 if(errors.length)throw Error(errors.join('\n'))
 const video=page.video();await context.close();await writeFile(`${out}/video-path.txt`,await video.path())
 console.log('Local experience preview recorded')
}finally{await browser.close()}
