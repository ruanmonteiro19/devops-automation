// Browser QA validates the frontend actually embedded in the Go Docker image.
// Run exclusively against a disposable local stack.
const { chromium } = require('playwright');
const fs = require('fs');
const path = require('path');
const dir = path.resolve(__dirname, '../../qa/screenshots');
fs.mkdirSync(dir,{recursive:true});
const sleep=(t)=>new Promise(r=>setTimeout(r,t));
(async()=>{
 const browser=await chromium.launch({headless:true,args:['--no-sandbox']});
 const page=await browser.newPage({viewport:{width:1600,height:900},deviceScaleFactor:1});
 const base='http://127.0.0.1:8080';
 try {
  await page.goto(base,{waitUntil:'networkidle',timeout:45000});
  await page.getByRole('heading',{name:'Conectar ao Observa Lite'}).waitFor({timeout:30000});
  const color=await page.evaluate(()=>({
   primary:getComputedStyle(document.documentElement).getPropertyValue('--primary').trim(),
   background:getComputedStyle(document.documentElement).getPropertyValue('--background').trim()
  }));
  if (!(color.primary.includes('151') && /(76%|0\.76)/.test(color.primary)) || !(color.background.includes('151') && /(10\.5%|0\.105)/.test(color.background))){
   throw Error('Black-green theme not embedded in Docker: '+JSON.stringify(color));
  }
  await page.screenshot({path:path.join(dir,'01-login-black-green.png'),fullPage:true});
  await page.getByRole('combobox',{name:'Interface language'}).click();
  await page.getByRole('option',{name:'English'}).click();
  await page.getByText('Overview',{exact:true}).first().waitFor({timeout:8000});
  await page.getByRole('combobox',{name:'Interface language'}).click();
  await page.getByRole('option',{name:'PT-BR'}).click();
  if(!process.env.OBSERVA_QA_ADMIN_TOKEN || process.env.OBSERVA_QA_ADMIN_TOKEN.length < 24)throw Error('test token missing');
  await page.getByLabel('Token da API').fill(process.env.OBSERVA_QA_ADMIN_TOKEN);
  await page.getByRole('button',{name:'Conectar',exact:true}).click();
  await page.getByRole('link',{name:'Visão geral'}).waitFor({timeout:15000});
  await page.screenshot({path:path.join(dir,'02-authenticated-overview.png'),fullPage:true});
  for(const route of ['agentes','host-groups','network-zones','gateways','process-mapping','service-mapping']){
   await page.goto(base+'/'+route,{waitUntil:'networkidle',timeout:15000});
   const title=route==='agentes'?'Agentes':route==='host-groups'?'Host Groups':route==='network-zones'?'Network Zones':route==='process-mapping'?'Process Mapping':route==='service-mapping'?'Service Mapping':'Gateways';
   await page.getByRole('heading',{name:title,exact:true}).waitFor({timeout:15000});
   if (route==='service-mapping') await page.getByText('Nenhum serviço APM instrumentado').waitFor({timeout:15000});
   if (route==='gateways') {
    await page.getByPlaceholder('gate-a').fill('gateway-ci');
    await page.getByPlaceholder('zone-a').fill('qa-zone');
    await page.getByPlaceholder('https://gateway.example:18443').fill('https://gateway-ci.example:18443');
    await page.getByRole('button',{name:'Cadastrar'}).click();
    await page.getByRole('cell',{name:'gateway-ci',exact:true}).waitFor({timeout:15000});
    await page.screenshot({path:path.join(dir,'04-gateway-registered.png'),fullPage:true});
    page.once('dialog',d=>d.accept());
    await page.getByRole('button',{name:'Excluir'}).click();
    await page.getByRole('cell',{name:'gateway-ci',exact:true}).waitFor({state:'detached',timeout:15000});
   }
   await page.screenshot({path:path.join(dir,'03-'+route+'.png'),fullPage:true});
  }
  console.log('BROWSER_QA_OK',color);
 }finally{
  await browser.close();
 }
})().catch(e=>{console.error(e);process.exit(1)});
