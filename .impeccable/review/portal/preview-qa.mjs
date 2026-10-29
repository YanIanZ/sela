import { chromium } from 'playwright';
import http from 'node:http';
const proxy=http.createServer((req,res)=>{
 const upstream=http.request({hostname:'host.docker.internal',port:5173,path:req.url,method:req.method,headers:{...req.headers,host:'localhost:5173'}},response=>{res.writeHead(response.statusCode,response.headers);response.pipe(res)});
 upstream.on('error',()=>{res.writeHead(502);res.end()});req.pipe(upstream);
});
await new Promise(resolve=>proxy.listen(5173,'127.0.0.1',resolve));
import { readFileSync, writeFileSync } from 'node:fs';
const out='/repo/.impeccable/review/portal';
const browser=await chromium.launch({headless:true,executablePath:'/usr/bin/chromium-browser',args:['--no-sandbox']});
const page=await browser.newPage({locale:"id-ID"});
const apiRequests=[],errors=[],checks=[];
page.on('request',r=>{if(new URL(r.url()).pathname.startsWith('/api/'))apiRequests.push(r.url())});
page.on('pageerror',e=>errors.push(e.message));
await page.goto('http://localhost:5173/dashboard-preview.html',{waitUntil:'networkidle'});
await page.addScriptTag({content:readFileSync('/repo/web/node_modules/axe-core/axe.min.js','utf8')});
for(const [name,width,height,scheme] of [['desktop',1440,960,'light'],['tablet',1024,900,'light'],['mobile',390,844,'light'],['narrow',320,800,'light'],['dark',1440,960,'dark'],['contrast-dark',390,844,'dark'],['large-text',320,800,'light']]){
 await page.setViewportSize({width,height});await page.emulateMedia({colorScheme:scheme,contrast:name==='contrast-dark'?'more':'no-preference'});
 await page.waitForFunction(expected=>document.documentElement.dataset.scheme===expected,name==='contrast-dark'?'contrast-dark':scheme);
 await page.evaluate(name=>{if(name==='large-text')document.documentElement.setAttribute('data-text-scale','150');else document.documentElement.removeAttribute('data-text-scale')},name);
 await page.screenshot({path:`${out}/dashboard-${name}.png`,fullPage:true});
 const result=await page.evaluate(async()=>({overflow:document.documentElement.scrollWidth>innerWidth+1,violations:(await window.axe.run(document,{runOnly:{type:'tag',values:['wcag2a','wcag2aa','wcag21aa','wcag22aa']}})).violations.filter(v=>v.impact==='serious'||v.impact==='critical').map(v=>({id:v.id,nodes:v.nodes.map(n=>({html:n.html,summary:n.failureSummary}))}))}));
 checks.push({name,...result});
}
await page.setViewportSize({width:1440,height:960});
await page.getByRole('main').getByRole('link',{name:'Masuk ke Beranda',exact:true}).click();
checks.push({name:'portal opens separate personal Home in preview',passed:await page.getByRole('heading',{name:'Beranda',exact:true}).isVisible()});
await page.getByRole('main').getByRole('link',{name:'Mulai bercerita',exact:true}).click();
checks.push({name:'preview actions blocked',passed:page.url().endsWith('/dashboard-preview.html')});
checks.push({name:'preview does not call API',passed:apiRequests.length===0,requests:apiRequests});
checks.push({name:'no page errors',passed:errors.length===0,errors});
await page.evaluate(()=>{document.activeElement?.blur(); document.body.tabIndex=-1; document.body.focus()});
await page.keyboard.press('Tab');
checks.push({name:'first keyboard target is skip link',passed:await page.evaluate(()=>document.activeElement?.getAttribute('href')==='#main')});
const result={checks,pass:checks.every(c=>c.passed!==false&&!c.overflow&&!(c.violations?.length))};
writeFileSync(`${out}/preview-results.json`,JSON.stringify(result,null,2));console.log(JSON.stringify(result));
await browser.close();await new Promise(resolve=>proxy.close(resolve));if(!result.pass)process.exitCode=1;
