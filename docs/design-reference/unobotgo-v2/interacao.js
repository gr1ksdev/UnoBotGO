/* Referência local com dados ilustrativos. Não envia comandos de jogo. */
const q=new URLSearchParams(location.search),stage=document.getElementById('stage');
const reduced=matchMedia('(prefers-reduced-motion: reduce)').matches;
const startingNames=['yellow_1','yellow_2','yellow_reverse','green_7','blue_2','blue_3','wild'];
let currentMode='mine',cards=[],nextId=0,selectedId=null,busy=false,colorTimer,rematchTimer;
let activeTimelines=[];
const hand=document.getElementById('hand'),viewport=document.getElementById('hand-viewport');
const colorOrder=['red','yellow','green','blue','wild'];
const valueOrder=['0','1','2','3','4','5','6','7','8','9','skip','reverse','draw2','wild','wild_draw4'];
function addCard(name){const c={id:'demo-'+nextId++,name};cards.push(c);return c;}
function setCount(n){cards=[];for(let i=0;i<n;i++)addCard(startingNames[i%7]);}
function sortedCards(){return [...cards].sort((a,b)=>{const [ac,...av]=a.name.split('_'),[bc,...bv]=b.name.split('_');return colorOrder.indexOf(ac)-colorOrder.indexOf(bc)||valueOrder.indexOf(av.join('_')||ac)-valueOrder.indexOf(bv.join('_')||bc)||Number(a.id.slice(5))-Number(b.id.slice(5));});}
function playable(c){return currentMode==='mine'&&(c.name.startsWith('green')||c.name==='wild');}
function layoutHand(animate=false){
 const old=new Map([...hand.children].map(b=>[b.dataset.id,b.getBoundingClientRect()]));
 const ordered=sortedCards(),n=ordered.length,rowN=Math.min(8,n),available=stage.clientWidth-32;
 const cardW=Math.min(innerHeight<=650?78:96,available/(1+Math.max(0,rowN-1)*.59));
 const stride=cardW*.59,cardH=cardW*344/256,rowStep=cardH*.83;
 const rows=Math.ceil(n/8),fullWidth=cardW+Math.max(0,rowN-1)*stride;
 const handH=cardH+Math.max(0,rows-1)*rowStep;
 const viewH=Math.min(handH+36,innerHeight*.34);
 hand.style.width=fullWidth+'px';hand.style.height=handH+'px';hand.style.setProperty('--card-w',cardW+'px');
 viewport.style.height=viewH+'px';
 const bottom=document.body.classList.contains('screenshot')?76:innerWidth<=480?116:76;
 viewport.style.bottom=bottom+'px';
 const tableY=Math.max(innerHeight<=650?214:222,Math.min(innerHeight*.46,(innerHeight-viewH-bottom+100)/2));
 document.querySelector('.table').style.top=tableY+'px';
 const keep=new Set(ordered.map(c=>c.id));for(const b of [...hand.children])if(!keep.has(b.dataset.id)){gsap.killTweensOf(b);b.remove();}
 ordered.forEach((c,i)=>{
  let b=hand.querySelector(`[data-id="${c.id}"]`);
  if(!b){b=document.createElement('button');b.dataset.id=c.id;const img=document.createElement('img');img.src=`assets/cards/${c.name}.png`;img.alt='';b.append(img);hand.append(b);}
  const row=Math.floor(i/8),col=i%8,inRow=Math.min(8,n-row*8),rowWidth=cardW+Math.max(0,inRow-1)*stride;
  b.style.left=((fullWidth-rowWidth)/2+col*stride)+'px';b.style.top=row*rowStep+'px';b.style.setProperty('--x',b.style.left);
  b.style.zIndex=String(i+1);b.classList.toggle('selected',selectedId===c.id);if(selectedId===c.id)b.style.zIndex='100';
  b.classList.toggle('playable',playable(c));b.disabled=!playable(c)||busy;b.setAttribute('aria-label',c.name.replaceAll('_',' '));
  b.setAttribute('aria-pressed',String(selectedId===c.id));b.dataset.row=String(row+1);
  if(animate&&!reduced&&old.has(c.id)){
   const before=old.get(c.id),after=b.getBoundingClientRect();
   gsap.fromTo(b,{x:before.left-after.left,y:before.top-after.top},{x:0,y:selectedId===c.id?-14:0,duration:.58,ease:'power3.out',overwrite:true});
  }else gsap.set(b,{x:0,y:selectedId===c.id?-14:0});
 });
 document.getElementById('my-count').textContent=n+' ▰';
}
function cancelAnimations(){
 clearTimeout(colorTimer);clearTimeout(rematchTimer);activeTimelines.forEach(t=>t.kill());activeTimelines=[];
 gsap.killTweensOf('.hand button,.picker,.picker button,.result-inner');document.querySelectorAll('.flight').forEach(f=>f.remove());
 [...hand.children].forEach(b=>{b.style.opacity='';gsap.set(b,{scale:1,scaleX:1,rotation:0});const c=cards.find(c=>c.id===b.dataset.id);if(c)b.querySelector('img').src=`assets/cards/${c.name}.png`;});busy=false;
}
function mode(m){
 cancelAnimations();currentMode=m;selectedId=null;
 stage.classList.toggle('other',m==='other');
 document.getElementById('my-avatar').classList.toggle('active',m==='mine'||m==='picker');
 document.getElementById('opponent-avatar').classList.toggle('active',m==='other');
 document.getElementById('deck').classList.toggle('ready',m==='mine');
 document.getElementById('picker-overlay').classList.toggle('show',m==='picker');
 document.getElementById('result').classList.toggle('show',m==='result');
 document.querySelectorAll('[data-color]').forEach(b=>{b.style.background='';b.disabled=false});
 layoutHand();
 if(m==='picker'&&!reduced)gsap.fromTo('.picker',{scale:.72,opacity:0},{scale:1,opacity:1,duration:.5,ease:'power3.out',overwrite:true});
 if(m==='result'&&!reduced)gsap.fromTo('.result-inner',{y:22,opacity:0},{y:0,opacity:1,duration:.55,ease:'power3.out',overwrite:true});
}
function flyCard(name,target,{delay=0,reveal=true,done=()=>{}}={}){
 if(reduced){done();return;}
 const r=stage.getBoundingClientRect(),from=document.getElementById('deck').getBoundingClientRect();
 const w=target.width,h=w*344/256,el=document.createElement('div');el.className='flight';el.style.width=w+'px';el.style.height=h+'px';
 el.innerHTML=`<div class="flipper"><img src="assets/backs/default.png" alt=""><img class="front" src="assets/cards/${name}.png" alt=""></div>`;
 stage.append(el);
 gsap.set(el,{x:from.left-r.left+(from.width-w)/2,y:from.top-r.top+(from.height-h)/2,scale:from.width/w,rotation:-5});
 const t=gsap.timeline({delay,onComplete:()=>{el.remove();done();}});
 t.to(el,{x:target.left-r.left,y:target.top-r.top,scale:1,rotation:0,duration:.78,ease:'power2.inOut'},0);
 if(reveal)t.to(el.querySelector('.flipper'),{rotationY:180,duration:.5,ease:'power2.inOut'},.2);
 activeTimelines.push(t);
}
function finishBusy(){busy=false;layoutHand();}
function dealDemo(){
 setCount(7);mode('mine');busy=true;layoutHand();
 const targets=cards.map(c=>({c,b:hand.querySelector(`[data-id="${c.id}"]`)}));
 targets.forEach(({b})=>b.style.opacity='0');let finished=0;
 const done=()=>{if(++finished===14)finishBusy();};
 targets.forEach(({c,b},i)=>{
  flyCard(c.name,b.getBoundingClientRect(),{delay:i*.18,done:()=>{b.style.opacity='';done();}});
  const back=document.getElementById('backs').children[i].getBoundingClientRect();
  flyCard(c.name,back,{delay:i*.18+.09,reveal:false,done});
 });
}
function drawDemo(){
 if(busy||currentMode!=='mine')return;
 busy=true;selectedId=null;const c=addCard(['green_4','red_6','blue_draw2','yellow_8'][cards.length%4]);
 layoutHand(true);const b=hand.querySelector(`[data-id="${c.id}"]`);b.style.opacity='0';
 b.scrollIntoView({block:'nearest',inline:'nearest',behavior:'instant'});
 flyCard(c.name,b.getBoundingClientRect(),{done:()=>{b.style.opacity='';finishBusy();}});
}
function flipDemo(){
 if(busy)return;const b=hand.querySelector('.playable')||hand.firstElementChild;if(!b)return;
 busy=true;layoutHand();const img=b.querySelector('img'),front=img.getAttribute('src');
 const t=gsap.timeline({onComplete:finishBusy});
 t.to(b,{scaleX:0,duration:reduced?0:.25,ease:'power2.in'}).call(()=>img.src='assets/backs/default.png')
  .to(b,{scaleX:1,duration:reduced?0:.25,ease:'power2.out'}).to({}, {duration:reduced?0:.3})
  .to(b,{scaleX:0,duration:reduced?0:.25,ease:'power2.in'}).call(()=>img.src=front)
  .to(b,{scaleX:1,duration:reduced?0:.25,ease:'power2.out'});
 activeTimelines.push(t);
}
document.getElementById('backs').innerHTML=Array.from({length:7},()=>'<img src="assets/backs/default.png" alt="">').join('');
document.querySelectorAll('[data-mode]').forEach(b=>b.onclick=()=>mode(b.dataset.mode));
hand.onclick=e=>{
 const b=e.target.closest('button');if(!b||b.disabled||busy)return;
 selectedId=selectedId===b.dataset.id?null:b.dataset.id;
 [...hand.children].forEach(x=>{const yes=x.dataset.id===selectedId;x.classList.toggle('selected',yes);x.style.zIndex=yes?'100':String(sortedCards().findIndex(c=>c.id===x.dataset.id)+1);x.setAttribute('aria-pressed',String(yes));gsap.to(x,{y:yes?-14:0,scale:yes?1.035:1,duration:reduced?0:.38,ease:'power3.out',overwrite:true});});
 if(cards.find(c=>c.id===selectedId)?.name==='wild')colorTimer=setTimeout(()=>mode('picker'),420);
};
document.querySelectorAll('[data-color]').forEach(b=>b.onclick=()=>{
 if(b.disabled)return;const picked=b.dataset.color;
 document.querySelectorAll('[data-color]').forEach(x=>x.disabled=true);
 const target=getComputedStyle(document.documentElement).getPropertyValue('--'+picked).trim();
 const t=gsap.timeline();
 t.to('.picker button',{backgroundColor:target,duration:reduced?0:.45,ease:'power2.inOut'})
  .to({}, {duration:reduced?0.1:0.7})
  .to('.picker',{scale:.88,opacity:0,duration:reduced?0:.35,ease:'power2.in'})
  .call(()=>{document.getElementById('discard').src=`assets/cards/${picked}_wild.png`;document.getElementById('discard').alt=`Coringa ${picked}`;mode('mine');});
 activeTimelines.push(t);
});
document.getElementById('many').onclick=()=>{cancelAnimations();const sizes=[7,8,9,16,30];setCount(sizes[(sizes.indexOf(cards.length)+1)%sizes.length]);selectedId=null;layoutHand();viewport.scrollTop=0;};
document.getElementById('deal').onclick=dealDemo;document.getElementById('buy').onclick=drawDemo;document.getElementById('deck').onclick=drawDemo;document.getElementById('flip').onclick=flipDemo;
document.getElementById('rematch').onclick=()=>{document.getElementById('mine-ready').classList.add('on');document.getElementById('result-status').textContent='Você aceitou · aguardando Rafael';checkReady();};
document.getElementById('other-accept').onclick=()=>{document.getElementById('other-ready').classList.add('on');checkReady();};
function checkReady(){if(document.getElementById('mine-ready').classList.contains('on')&&document.getElementById('other-ready').classList.contains('on')){document.getElementById('result-status').textContent='Ambos aceitaram · reiniciando demonstração';clearTimeout(rematchTimer);rematchTimer=setTimeout(()=>{document.querySelectorAll('.ready-mark').forEach(x=>x.classList.remove('on'));dealDemo();},850);}}
if(q.get('capture'))document.body.classList.add('screenshot');
setCount(Math.max(1,Math.min(30,Number(q.get('count'))||7)));mode(q.get('state')||'mine');
window.addEventListener('resize',()=>{cancelAnimations();layoutHand();});
window.addEventListener('pagehide',cancelAnimations);
if(!q.get('capture')&&!q.get('state')&&!q.get('count'))document.fonts.ready.then(dealDemo);
