'use strict';
const {test} = require('node:test');
const assert = require('node:assert/strict');
const http = require('node:http');
const zlib = require('node:zlib');
process.env.VETRA_RUM_SCRIPT_URL='https://collector.test/rum/vetra.js?app=portal';
require('../internal/instrumentation/node.cjs');
const html='<!doctype html><html><head><title>Portal</title></head><body>App</body></html>';
test('Node preload modifies eligible HTML, preserving stream, binary, CSP, cookies and callbacks',async()=>{
 const server=http.createServer((req,res)=>{
 res.setHeader('Content-Type',req.url==='/json'?'application/json':'text/html; charset=utf-8');res.setHeader('Set-Cookie',['a=1; HttpOnly','b=2']);res.setHeader('ETag','original');
 if(req.url==='/comment')return res.end('<!doctype html><!-- <head>ignored</head> --><html><head><title>App</title></head></html>');
 if(req.url==='/no-transform'){res.setHeader('Cache-Control','no-transform');return res.end(html);}
 if(req.url==='/csp')res.setHeader('Content-Security-Policy',"script-src 'none'");
 if(req.url==='/gzip'){res.setHeader('Content-Encoding','gzip');return res.end(zlib.gzipSync(html));}
 if(req.url==='/br'){res.setHeader('Content-Encoding','br');return res.end(zlib.brotliCompressSync(html));}
 if(req.url==='/stream'){res.write(html.slice(0,20));return res.end(html.slice(20));}
 if(req.url==='/head'){res.writeHead(200);return res.end(html);}
 if(req.url==='/json')return res.end('{"ok":true}');
 if(req.url==='/duplicate')return res.end(html.replace('<head>','<head><script data-vetra-injected="true"></script>'));
 if(req.url==='/meta')return res.end(html.replace('<head>','<head><meta http-equiv="Content-Security-Policy" content="script-src none">'));
 if(req.url==='/large')return res.end(html+' '.repeat(1024*1024));
 res.end(html,'utf8',()=>{});
 });await new Promise(r=>server.listen(0,'127.0.0.1',r));const url=`http://127.0.0.1:${server.address().port}`;
 try{for(const path of ['/','/gzip','/br','/comment','/no-transform','/csp','/login','/stream','/head','/json','/duplicate','/meta','/large']){const r=await fetch(url+path);const text=await r.text();const injected=['/','/gzip','/br','/comment'].includes(path);assert.equal(r.headers.get('x-vetra-injection')==='injected-node',injected,path);if(injected){assert(text.includes(process.env.VETRA_RUM_SCRIPT_URL));if(path==='/comment')assert(text.includes('<!-- <head>ignored</head> --><html><head><script'));assert.equal(r.headers.get('etag'),null);assert.equal(r.headers.get('cache-control'),'no-store');}else if(!['/duplicate','/meta','/large','/json'].includes(path))assert.equal(text,html,path);assert(r.headers.get('set-cookie').includes('HttpOnly'));}
 const head=await fetch(url,{method:'HEAD'});assert.equal(await head.text(),'');
 process.env.VETRA_RUM_ENABLED='false';const disabled=await fetch(url);assert.equal(await disabled.text(),html);delete process.env.VETRA_RUM_ENABLED;
 }finally{server.closeAllConnections();await new Promise(r=>server.close(r));}
});
