#!/usr/bin/env node
// Real runtime + proxy + TLS gateway + Chromium. Optional Playwright uses the same configuration as smoke-rum.mjs.
import assert from 'node:assert/strict';
import {createServer} from 'node:http';
import https from 'node:https';
import {spawn,spawnSync} from 'node:child_process';
import {mkdtemp,writeFile,rm,readFile} from 'node:fs/promises';
import {tmpdir} from 'node:os';
import {join,resolve} from 'node:path';
import {randomBytes} from 'node:crypto';
const bin=resolve(process.argv[2]||'/tmp/vetra-auto-server'),proxyBin=resolve(process.argv[3]||'/tmp/vetra-auto-proxy'),gatewayBin=resolve(process.argv[4]||'/tmp/vetra-auto-gateway');
const temp=await mkdtemp(join(tmpdir(),'vetra-auto-'));const processes=[];let browser;
const admin=randomBytes(24).toString('hex'),agent=randomBytes(24).toString('hex');const delay=ms=>new Promise(r=>setTimeout(r,ms));
const reserve=async()=>{const s=createServer();await new Promise(r=>s.listen(0,'127.0.0.1',r));const p=s.address().port;await new Promise(r=>s.close(r));return p;};
const cp=await reserve(),gp=await reserve(),pp=await reserve(),np=await reserve(),bp=await reserve();const collector=`http://127.0.0.1:${cp}`,gateway=`https://127.0.0.1:${gp}`,proxy=`http://127.0.0.1:${pp}`,node=`http://127.0.0.1:${np}`,backup=`http://127.0.0.1:${bp}`;
const env={...process.env,OBSERVA_ADMIN_TOKEN:admin,OBSERVA_AGENT_TOKEN:agent};const sdk=gateway+'/rum/vetra.js?app=portal';
const start=(binary,args,extras={})=>{const p=spawn(binary,args,{env:{...env,...extras},stdio:['ignore','ignore','pipe']});p.stderr.on('data',b=>{if(process.env.VETRA_VERBOSE)process.stderr.write(b)});processes.push(p);return p;};
const tlsGet=url=>new Promise((resolve,reject)=>{https.get(url,{rejectUnauthorized:false},r=>{let text='';r.on('data',b=>text+=b);r.on('end',()=>resolve({status:r.statusCode,text}));}).on('error',reject)});
const ready=async(url,tls=false)=>{for(let i=0;i<100;i++){try{if(tls?(await tlsGet(url)).status===200:(await fetch(url)).ok)return;}catch{}await delay(100);}throw Error('Startup timeout '+url);};
async function api(path,body){const r=await fetch(collector+path,{method:body?'POST':'GET',headers:{'X-API-Key':admin,'Content-Type':'application/json'},...(body?{body:JSON.stringify(body)}:{})});assert(r.ok,await r.clone().text());return r.json();}
try{
 const cert=join(temp,'tls.crt'),key=join(temp,'tls.key');const openssl=spawnSync('openssl',['req','-x509','-newkey','rsa:2048','-nodes','-days','1','-keyout',key,'-out',cert,'-subj','/CN=127.0.0.1','-addext','subjectAltName=IP:127.0.0.1'],{stdio:'ignore'});assert.equal(openssl.status,0,'openssl test certificate');
 start(bin,['server','--listen',`127.0.0.1:${cp}`,'--data',join(temp,'state.json')]);await ready(collector+'/healthz');
 start(gatewayBin,['--listen',`127.0.0.1:${gp}`,'--upstream',collector,'--tls-cert',cert,'--tls-key',key]);await ready(gateway+'/healthz',true);
 const fixture=join(temp,'app.cjs');await writeFile(fixture,`const http=require('node:http');http.createServer((q,r)=>{r.setHeader('Content-Type','text/html; charset=utf-8');r.end('<!doctype html><html><head><title>Portal</title></head><body><h1>Portal '+process.env.APP_LABEL+'</h1><input type="password" value="SECRET_PASSWORD"><p>PRIVATE_PERSON_EMAIL@example.test</p></body></html>');}).listen(Number(process.env.APP_PORT),'127.0.0.1');`);
 start(process.execPath,['--require',resolve('internal/instrumentation/node.cjs'),fixture],{APP_PORT:String(np),APP_LABEL:'Node',VETRA_RUM_SCRIPT_URL:sdk});await ready(node);
 start(process.execPath,[fixture],{APP_PORT:String(bp),APP_LABEL:'Backup'});await ready(backup);
 const direct=await fetch(node);assert.equal(direct.headers.get('x-vetra-injection'),'injected-node');assert((await direct.text()).includes(sdk));
 const routesFile=join(temp,'routes.json');const config={routes:[{id:'portal',host:'127.0.0.1',path_prefix:'/',upstreams:[node,backup],script_url:sdk,excluded_paths:['/login'],csp_approved:false,injection_enabled:true}]};await writeFile(routesFile,JSON.stringify(config));
 start(proxyBin,['--listen',`127.0.0.1:${pp}`,'--config',routesFile]);await ready(proxy+'/__vetra/health');
 await api('/api/v1/rum/apps',{id:'portal',name:'Portal',origins:[proxy,node],enabled:true,require_consent:true,consent_banner:true,replay_enabled:true,sample_rate:1,replay_sample_rate:1,masking:'all',public_text_selectors:[],block_selectors:[],exclude_paths:[],capture_path:false,identify_users:false,apdex_t_ms:1000});
 for(let i=0;i<2;i++){const response=await fetch(proxy);const html=await response.text();assert.equal((html.match(/<script data-vetra-injected/g)||[]).length,1,'duplicate injection');assert(['injected','already-present'].includes(response.headers.get('x-vetra-injection')));}
 const sdkResponse=await tlsGet(sdk);assert.equal(sdkResponse.status,200);assert(sdkResponse.text.length>100000,'gateway truncated SDK');assert.equal((await tlsGet(gateway+'/api/v1/snapshot')).status,404);
 if(process.env.VETRA_PLAYWRIGHT_MODULE){const {chromium}=await import(process.env.VETRA_PLAYWRIGHT_MODULE);browser=await chromium.launch({headless:true,executablePath:process.env.CHROMIUM_PATH,args:JSON.parse(process.env.VETRA_CHROMIUM_ARGS||'[]')});const context=await browser.newContext({ignoreHTTPSErrors:true});const page=await context.newPage();const wire=[];const errors=[];page.on('pageerror',e=>errors.push(e.message));page.on('request',q=>{if(q.url().startsWith(gateway+'/rum/beacon/'))wire.push(q.postData()||'')});await page.goto(proxy);await page.waitForFunction(()=>window.Vetra);await page.getByRole('dialog',{name:'Privacidade do diagnóstico'}).waitFor();assert.equal((await api('/api/v1/rum/analysis?app_id=portal')).sessions_total,0);await page.evaluate(()=>window.Vetra.setConsent({rum:true,replay:true}));await page.waitForFunction(()=>window.Vetra.status().active&&window.Vetra.status().replay);await page.waitForTimeout(350);await page.evaluate(()=>window.Vetra.flush());const analysis=await api('/api/v1/rum/analysis?app_id=portal');assert.equal(analysis.sessions_total,1);assert(analysis.apdex>=0);assert(wire.length>0);assert(!wire.join('').includes('SECRET_PASSWORD'));assert(!wire.join('').includes('PRIVATE_PERSON_EMAIL'));assert.deepEqual(errors,[]);
 // Installation UI downloads the actual preload and route file.
 const ui=await context.newPage();await ui.addInitScript(t=>sessionStorage.setItem('observa-lite-api-token',t),admin);await ui.goto(collector+'/rum');await ui.getByRole('button',{name:'Instalação',exact:true}).click();await ui.getByRole('heading',{name:'Injeção no processo Node.js'}).waitFor();const download=ui.waitForEvent('download');await ui.getByRole('button',{name:'Baixar módulo Node.js'}).click();assert.equal((await download).suggestedFilename(),'vetra-node.cjs');const configDownload=ui.waitForEvent('download');await ui.getByRole('button',{name:'Baixar configuração do proxy'}).click();const downloaded=await configDownload;const exported=JSON.parse(await readFile(await downloaded.path(),'utf8'));assert.equal(exported.routes[0].id,'portal');assert.equal(exported.routes[0].injection_enabled,true);await ui.screenshot({path:'/tmp/vetra-auto-injection-ui.png',fullPage:true});
 console.log('PASS: actual Chromium → Node/proxy injection → SDK/config via TLS gateway → consent → masked replay/beacons → central RUM and Apdex → preload download UI');}
 config.routes[0].injection_enabled=false;config.routes[0].upstreams=[backup];await writeFile(routesFile,JSON.stringify(config));await delay(5500);const disabled=await fetch(proxy);assert.equal(disabled.status,200);assert(!(await disabled.text()).includes('data-vetra-injected'),'injection bypass');
 await writeFile(routesFile,'INVALID');await delay(5500);assert.equal((await fetch(proxy)).status,200,'invalid update broke last valid route');
 console.log('PASS: unchanged Node application + process preload → multi-backend proxy → no duplicate injection → full SDK through restricted TLS gateway → hot reload + injection bypass + invalid-config retention');
}finally{await browser?.close();for(const p of processes.reverse())p.kill('SIGTERM');await delay(150);await rm(temp,{recursive:true,force:true});}
