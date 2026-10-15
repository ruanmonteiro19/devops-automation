#!/usr/bin/env node
// Isolated real-browser test. Requires Playwright; optional CHROMIUM_PATH and VETRA_CHROMIUM_MODULE.
import assert from 'node:assert/strict';
import {createServer} from 'node:http';
import {spawn} from 'node:child_process';
import {mkdtemp,rm} from 'node:fs/promises';
import {tmpdir} from 'node:os';
import {join,resolve} from 'node:path';
import {randomBytes} from 'node:crypto';
const {chromium}=await import(process.env.VETRA_PLAYWRIGHT_MODULE||'playwright');
const binary=resolve(process.argv[2]||'/tmp/vetra-rum-server');const injector=resolve(process.argv[3]||'/tmp/vetra-rum-injector');
const admin=randomBytes(24).toString('hex'),agent=randomBytes(24).toString('hex');const temp=await mkdtemp(join(tmpdir(),'vetra-rum-smoke-'));
const delay=ms=>new Promise(r=>setTimeout(r,ms));
const reserve=async()=>{const server=createServer();await new Promise(r=>server.listen(0,'127.0.0.1',r));const port=server.address().port;await new Promise(r=>server.close(r));return port;};
const collectorPort=await reserve(),proxyPort=await reserve();const collector=`http://127.0.0.1:${collectorPort}`,proxy=`http://127.0.0.1:${proxyPort}`;
const fixture=createServer((req,res)=>{if(req.url.startsWith('/api')){res.writeHead(req.url.startsWith('/api/error')?500:200,{'Content-Type':'application/json'});res.end('{"ok":true}');return;}
res.writeHead(200,{'Content-Type':'text/html; charset=utf-8'});res.end(`<!doctype html><html><head><title>Vetra synthetic browser test</title></head><body><main style="padding:24px"><h1 data-public>Public test heading</h1><p id="private">PRIVATE_EMAIL_PERSON@example.test</p><input aria-label="Senha" type="password" value="PASSWORD_SECRET"><textarea>FORM_TEXT_SECRET</textarea><div data-private>BLOCK_SECRET</div><button id="continue">Continuar</button><p id="mutating">Initial</p><img src="/api/image?token=IMAGE_SECRET"><script>document.querySelector('#continue').onclick=()=>{document.querySelector('#mutating').textContent='MUTATION_SECRET';};</script></main></body></html>`);});
await new Promise(r=>fixture.listen(0,'127.0.0.1',r));const upstream=`http://127.0.0.1:${fixture.address().port}`;
let server,edge,browser;
const env={...process.env,OBSERVA_ADMIN_TOKEN:admin,OBSERVA_AGENT_TOKEN:agent};delete env.OBSERVA_TLS_CERT_FILE;delete env.OBSERVA_TLS_KEY_FILE;
async function ready(url){for(let i=0;i<100;i++){try{if((await fetch(url)).ok)return;}catch{}await delay(100);}throw Error('Server startup timeout');}
async function api(path,body,method=body?'POST':'GET'){const r=await fetch(collector+path,{method,headers:{'X-API-Key':admin,'Content-Type':'application/json'},...(body?{body:JSON.stringify(body)}:{})});assert(r.ok,`${path}: ${r.status} ${await r.clone().text()}`);return r.json();}
try{
 server=spawn(binary,['server','--listen',`127.0.0.1:${collectorPort}`,'--data',join(temp,'state.json')],{env,stdio:['ignore','ignore','pipe']});await ready(collector+'/healthz');
 const config={id:'web-smoke',name:'Browser smoke',origins:[proxy],enabled:true,require_consent:true,consent_banner:true,replay_enabled:true,sample_rate:1,replay_sample_rate:1,masking:'all',public_text_selectors:[],block_selectors:[],exclude_paths:['/login'],capture_path:false,identify_users:true,apdex_t_ms:1000};await api('/api/v1/rum/apps',config);
 edge=spawn(injector,['-listen',`127.0.0.1:${proxyPort}`,'-upstream',upstream,'-script-url',`${collector}/rum/vetra.js?app=web-smoke`],{stdio:['ignore','ignore','pipe']});await ready(proxy+'/');
 let args=JSON.parse(process.env.VETRA_CHROMIUM_ARGS||"[]");if(process.env.VETRA_CHROMIUM_MODULE){args=(await import(process.env.VETRA_CHROMIUM_MODULE)).default.args;}
 browser=await chromium.launch({headless:true,args,...(process.env.CHROMIUM_PATH?{executablePath:process.env.CHROMIUM_PATH}:{}),env:{...process.env}});
 const context=await browser.newContext();const page=await context.newPage();const wire=[];const browserErrors=[];page.on('pageerror',e=>browserErrors.push(e.message));
 page.on('request',req=>{if(req.url().startsWith(collector+'/rum/beacon/'))wire.push(req.postData()||'');});
 const response=await page.goto(proxy+'/?token=PAGE_QUERY_SECRET');assert.equal(response.headers()['x-vetra-injection'],'injected');await page.waitForFunction(()=>window.Vetra);await page.getByRole('dialog',{name:'Privacidade do diagnóstico'}).waitFor();
 assert.equal((await api('/api/v1/rum/analysis?app_id=web-smoke')).sessions_total,0);assert.equal(await page.evaluate(()=>Object.keys(sessionStorage).filter(k=>k.startsWith('vetra.rum.')).length),0);
 await page.evaluate(()=>window.Vetra.setConsent({rum:true,replay:true}));await page.waitForFunction(()=>window.Vetra.status().active&&window.Vetra.status().replay);
 await page.getByLabel('Senha').fill('TYPED_PASSWORD_SECRET');await page.getByRole('button',{name:'Continuar'}).click();
 await page.evaluate(async()=>{await window.Vetra.identify('OPAQUE_INTERNAL_USER_SECRET');await fetch('/api/orders?token=NETWORK_QUERY_SECRET',{method:'POST',body:'REQUEST_BODY_SECRET'});await new Promise(r=>{const x=new XMLHttpRequest();x.open('GET','/api/error?password=XHR_QUERY_SECRET');x.onloadend=r;x.send();});window.dispatchEvent(new ErrorEvent('error',{error:new TypeError('ERROR_MESSAGE_SECRET')}));});
 await page.waitForTimeout(350);await page.evaluate(()=>window.Vetra.flush());
 let analysis=await api('/api/v1/rum/analysis?app_id=web-smoke');assert.equal(analysis.sessions_total,1);assert(analysis.requests>=2);assert(analysis.errors>=2);assert(analysis.apdex>=0&&analysis.apdex<=1);assert.equal(analysis.identified_users,1);assert(analysis.vitals_p75.LCP>0,'Native LCP measurement missing');assert(analysis.vitals_p75.TTFB>=0,'Native TTFB missing');
 const session=analysis.sessions[0];let replay=await api(`/api/v1/rum/apps/web-smoke/sessions/${session.id}/replay`);assert(replay.chunks.some(c=>c.events.some(e=>e.type===2)),'Initial replay snapshot missing');
 const forbidden=['PRIVATE_EMAIL_PERSON','PASSWORD_SECRET','TYPED_PASSWORD_SECRET','FORM_TEXT_SECRET','BLOCK_SECRET','MUTATION_SECRET','IMAGE_SECRET','PAGE_QUERY_SECRET','NETWORK_QUERY_SECRET','REQUEST_BODY_SECRET','XHR_QUERY_SECRET','ERROR_MESSAGE_SECRET','OPAQUE_INTERNAL_USER_SECRET'];for(const text of forbidden){assert(!wire.join('').includes(text),`Wire privacy leak: ${text}`);assert(!JSON.stringify({analysis,replay}).includes(text),`Stored privacy leak: ${text}`);}
 await page.evaluate(()=>history.pushState({},'', '/second?token=ROUTE_SECRET'));await page.waitForTimeout(100);await page.evaluate(()=>window.Vetra.flush());analysis=await api('/api/v1/rum/analysis?app_id=web-smoke');assert(analysis.events.some(e=>e.kind==='navigation'));assert.equal(analysis.apdex_samples,1,'SPA created a fabricated timing sample');
 await page.evaluate(()=>{history.pushState({},'', '/login');document.querySelector('#private').textContent='EXCLUDED_PAGE_SECRET';});await page.evaluate(()=>window.Vetra.flush());assert(!wire.join('').includes('EXCLUDED_PAGE_SECRET'));
 await page.evaluate(()=>{history.pushState({},'', '/second');window.Vetra.setConsent(false);});const count=wire.length;await page.evaluate(async()=>{await fetch('/api/after-revoke');await window.Vetra.flush();});await page.waitForTimeout(100);assert.equal(wire.length,count,'Revoked consent still transmits');
 // New allowlisted session: safe public text is useful, inputs and private regions stay masked.
 await api('/api/v1/rum/apps',{...config,masking:'allowlist',public_text_selectors:['[data-public]']});const p2=await context.newPage();const wire2=[];p2.on('request',req=>{if(req.url().includes('/rum/beacon/'))wire2.push(req.postData()||'');});await p2.goto(proxy+'/');await p2.waitForFunction(()=>window.Vetra);await p2.evaluate(()=>window.Vetra.setConsent({rum:true,replay:true}));await p2.waitForTimeout(250);await p2.evaluate(()=>window.Vetra.flush());assert(wire2.join('').includes('Public test heading'),'Reviewed public text not captured');for(const text of forbidden)assert(!wire2.join('').includes(text),`Allowlist privacy leak: ${text}`);
 // DNT blocks capture, even after consent API calls.
 const dnt=await browser.newContext();await dnt.addInitScript(()=>Object.defineProperty(navigator,'doNotTrack',{get:()=> '1'}));const dp=await dnt.newPage();await dp.goto(proxy+'/');await dp.waitForFunction(()=>window.Vetra);await dp.evaluate(()=>window.Vetra.setConsent({rum:true,replay:true}));assert.equal(await dp.evaluate(()=>window.Vetra.status().active),false);await dnt.close();
 // Real application UI: authenticated session list and sandboxed player.
 const ui=await context.newPage();await ui.addInitScript(({origin,token})=>{if(location.origin===origin)sessionStorage.setItem('observa-lite-api-token',token);},{origin:collector,token:admin});await ui.goto(collector+'/session-replay');await ui.getByRole('heading',{name:'Session Replay',exact:true}).waitFor();await ui.getByRole('button',{name:'Investigar'}).first().click();await ui.locator('.vetra-replay-root iframe').waitFor();assert.equal(await ui.locator('.vetra-replay-root iframe').getAttribute('sandbox'),'allow-same-origin');await ui.getByRole('button',{name:'Reproduzir',exact:true}).click();await ui.getByRole('button',{name:'Pausar',exact:true}).waitFor();await ui.screenshot({path:'/tmp/vetra-rum-ui-qa.png',fullPage:true});
 // Regression check for the earlier black minimap and system-process leakage.
 await ui.keyboard.press('Escape');
 const discovery=await fetch(collector+'/api/v1/ingest',{method:'POST',headers:{'X-API-Key':agent,'Content-Type':'application/json'},body:JSON.stringify({host_id:'qa-host',hostname:'Synthetic host',os:'Linux',addresses:['127.0.0.1'],processes:[{pid:10,name:'bash',service:'bash',runtime:'native',rss_bytes:100},{pid:11,name:'java',service:'Orders',runtime:'java',rss_bytes:2048}],connections:[]})});assert.equal(discovery.status,202);
 await ui.goto(collector+'/process-mapping');await ui.locator('.react-flow__node').first().waitFor();assert.equal(await ui.locator('.react-flow__node').filter({hasText:'bash'}).count(),0);await ui.waitForTimeout(800);await ui.screenshot({path:'/tmp/vetra-minimap-qa.png',fullPage:true});await ui.locator('.react-flow__minimap-node').first().waitFor({timeout:3000});assert.equal(await ui.locator('.react-flow__minimap-node').first().evaluate(el=>getComputedStyle(el).fill),'rgb(34, 168, 121)');
 await ui.screenshot({path:'/tmp/vetra-minimap-qa.png',fullPage:true});
 assert.deepEqual(browserErrors,[],'SDK broke the fixture');
 // Delete/persist/restart is covered by store and API tests, not simulated by the UI.
 console.log('PASS: proxy HTML injection → real Chromium → opt-in → fetch/XHR + errors + native vitals → masked rrweb snapshot/mutations → sessions/Apdex → SPA → revocation → public allowlist → DNT → authenticated replay player');
}finally{await browser?.close();edge?.kill('SIGTERM');server?.kill('SIGTERM');await new Promise(r=>fixture.close(r));await rm(temp,{recursive:true,force:true});}
