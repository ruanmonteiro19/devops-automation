'use strict';
// Vetra Node HTTP/1 preload. No application source changes, no credentials or request bodies captured.
const http = require('node:http');
const zlib = require('node:zlib');
const marker = Symbol.for('vetra.rum.preload');
const stats = { injected: 0, skipped: 0, errors: 0 };
const original = http.ServerResponse.prototype.end;
const escape = s => s.replace(/[&<>"']/g, c => ({'&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;',"'":'&#39;'}[c]));
// Linear scan, skipping comments, raw script/style text and template contents.
function headEnd(text) {
  const lower=text.toLowerCase();let i=0,templates=0;
  while (i<text.length) {
    const start=text.indexOf('<',i);if(start<0)return -1;
    if(text.startsWith('<!--',start)){const end=text.indexOf('-->',start+4);if(end<0)return -1;i=end+3;continue;}
    let p=start+1;const closing=text[p]==='/';if(closing)p++;
    const from=p;while(p<text.length&&/[a-z0-9:-]/i.test(text[p]))p++;
    const name=lower.slice(from,p);let quote='';let end=p;
    for(;end<text.length;end++){const c=text[end];if(quote){if(c===quote)quote='';}else if(c==='"'||c==="'")quote=c;else if(c==='>')break;}
    if(end>=text.length)return -1;i=end+1;
    if(name==='template'){templates=Math.max(0,templates+(closing?-1:1));continue;}
    if(name==='head'&&!closing&&!templates)return i;
    if(!closing&&(name==='script'||name==='style')){const close=lower.indexOf('</'+name,i);if(close<0)return -1;i=close;}
  }
  return -1;
}
if (!http.ServerResponse.prototype[marker]) {
  Object.defineProperty(http.ServerResponse.prototype, marker, {value: true});
  http.ServerResponse.prototype.end = function(chunk, encoding, callback) {
    try {
      const src = process.env.VETRA_RUM_SCRIPT_URL;
      const req = this.req;
      const skip = reason => { stats.skipped++; if (!this.headersSent) this.setHeader('X-Vetra-Injection', reason); };
      let valid = false;
      if (src) { const u = new URL(src); valid = ['https:', 'http:'].includes(u.protocol) && !u.username && !u.password && !u.hash && !!u.searchParams.get('app'); }
      const contentType = String(this.getHeader('Content-Type') || '').toLowerCase();
      const paths = (process.env.VETRA_RUM_EXCLUDE_PATHS || '/login,/auth,/checkout,/payment').split(',').filter(Boolean);
      const hosts = (process.env.VETRA_RUM_HOSTS || '').toLowerCase().split(',').filter(Boolean);
      let hostname = ''; try { hostname = new URL('http://' + req.headers.host).hostname.toLowerCase(); } catch {}
      const path = (req.url || '/').split('?')[0];
      if (!valid || process.env.VETRA_RUM_ENABLED === 'false') { /* inert if not configured */ }
      else if (this.headersSent || this.statusCode !== 200 || req.method === 'HEAD' || req.headers.range || !contentType.includes('text/html')) { stats.skipped++; }
      else if (String(this.getHeader('Cache-Control')||'').toLowerCase().includes('no-transform') || String(this.getHeader('Content-Disposition')||'').toLowerCase().includes('attachment')) skip('excluded');
      else if (paths.some(p => path.startsWith(p)) || (hosts.length && !hosts.includes(hostname))) skip('excluded');
      else if (this.getHeader('Content-Security-Policy') && process.env.VETRA_RUM_CSP_APPROVED !== 'true') skip('skipped-csp');
      else if (/charset\s*=\s*["']?(?!utf-8|utf8|us-ascii)[\w-]+/i.test(contentType)) skip('skipped-charset');
      else if (typeof chunk === 'string' || Buffer.isBuffer(chunk) || chunk instanceof Uint8Array) {
        const raw = Buffer.isBuffer(chunk) ? chunk : Buffer.from(chunk, typeof encoding === 'string' ? encoding : undefined);
        if (raw.length > 1024 * 1024) skip('skipped-size');
        else {
          const coding = String(this.getHeader('Content-Encoding') || 'identity').toLowerCase();
          let body = raw;
          if (coding === 'gzip') body = zlib.gunzipSync(raw, {maxOutputLength:1024*1024});
          else if (coding === 'br') body = zlib.brotliDecompressSync(raw, {maxOutputLength:1024*1024});
          if (!['identity','gzip','br'].includes(coding)) skip('skipped-encoding');
          else {
            const text = body.toString('utf8');
            const at = headEnd(text);
            if (text.toLowerCase().includes('content-security-policy') && process.env.VETRA_RUM_CSP_APPROVED !== 'true') skip('skipped-meta-csp');
            else if (/data-vetra-injected|\/rum\/vetra\.js/i.test(text)) skip('already-present');
            else if (at < 0 || !Buffer.from(text,'utf8').equals(body)) skip('skipped-html');
            else {
              body = Buffer.from(text.slice(0,at) + '<script data-vetra-injected="true" src="' + escape(src) + '" crossorigin="anonymous"></script>' + text.slice(at));
              if (coding === 'gzip') body = zlib.gzipSync(body);
              if (coding === 'br') body = zlib.brotliCompressSync(body);
              for (const key of ['ETag','Last-Modified','Content-MD5','Digest','Content-Digest']) this.removeHeader(key);
              this.setHeader('Cache-Control','no-store');this.setHeader('Content-Length',body.length);this.setHeader('X-Vetra-Injection','injected-node');stats.injected++;
              if (typeof encoding === 'function') callback = encoding;
              return original.call(this,body,callback);
            }
          }
        }
      } else skip('skipped-body');
    } catch { stats.errors++; if (!this.headersSent) this.setHeader('X-Vetra-Injection','skipped-error'); }
    return original.apply(this,arguments);
  };
}
module.exports = {stats};
