package httpapi

const fallbackIndexHTML = `<!doctype html><html lang="zh-CN"><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1">
<title>YYB Go 控制台</title>
<body style="margin:0;background:oklch(0.974 0.004 250);color:oklch(0.19 0.025 252);font-family:system-ui,-apple-system,BlinkMacSystemFont,'Segoe UI','Microsoft YaHei',sans-serif">
<main style="max-width:960px;margin:48px auto;padding:0 24px">
<section style="background:oklch(1 0 0);border:1px solid oklch(0.885 0.012 250);border-radius:8px;padding:24px">
<h1 style="margin:0 0 8px;font-size:24px">YYB Go 控制台</h1>
<p style="margin:0 0 20px;color:oklch(0.43 0.025 252)">资源模板未找到，服务仍可通过以下入口使用。</p>
<p style="display:flex;gap:10px;flex-wrap:wrap;margin:0">
<a style="padding:10px 12px;border-radius:8px;background:oklch(0.54 0.205 3);color:oklch(1 0 0);text-decoration:none" href="/scan">扫码添加</a>
<a style="padding:10px 12px;border-radius:8px;border:1px solid oklch(0.885 0.012 250);color:inherit;text-decoration:none" href="/docs/index.html">Swagger 文档</a>
<a style="padding:10px 12px;border-radius:8px;border:1px solid oklch(0.885 0.012 250);color:inherit;text-decoration:none" href="/openapi.json">OpenAPI JSON</a>
</p>
</section>
</main>
</body></html>`

const fallbackScanHTML = `<!doctype html><html lang="zh-CN"><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1">
<title>扫码添加账号</title>
<body style="margin:0;min-height:100vh;display:grid;place-items:center;background:oklch(0.974 0.004 250);color:oklch(0.19 0.025 252);font-family:system-ui,-apple-system,BlinkMacSystemFont,'Segoe UI','Microsoft YaHei',sans-serif">
<main style="width:min(420px,calc(100vw - 32px));background:oklch(1 0 0);border:1px solid oklch(0.885 0.012 250);border-radius:8px;padding:24px;text-align:center">
<h1 style="margin:0 0 8px;font-size:22px">扫码添加账号</h1>
<p id="s" style="margin:0 0 18px;color:oklch(0.43 0.025 252)">正在生成二维码</p>
<p id="cd" style="margin:0 0 12px;color:oklch(0.55 0.025 252);font-size:13px"></p>
<p><label><input id="px" type="checkbox" onchange="document.getElementById('pa').hidden=!this.checked"> 使用品赞白名单代理</label></p>
<p id="pa" hidden><input id="area" value="all" placeholder="优质池地区编码，如 440300" style="width:86%;padding:8px"></p>
<div id="qr" style="width:240px;height:240px;margin:0 auto 18px;display:grid;place-items:center;border:1px solid oklch(0.885 0.012 250);border-radius:8px;background:oklch(0.986 0.004 250)">请稍候</div>
<p style="display:flex;gap:10px;justify-content:center;margin:0">
<button onclick="newQR()" style="border:0;border-radius:8px;padding:10px 12px;background:oklch(0.54 0.205 3);color:oklch(1 0 0)">重新生成</button>
<a href="/" style="border:1px solid oklch(0.885 0.012 250);border-radius:8px;padding:9px 12px;color:inherit;text-decoration:none">返回首页</a>
</p>
</main>
<script>
let sid,pollTimer,countdownTimer,generation=0;
function stopTimers(){clearInterval(pollTimer);clearInterval(countdownTimer);pollTimer=countdownTimer=null}
function expireQR(){stopTimers();sid=null;document.getElementById('qr').textContent='已过期';document.getElementById('s').textContent='二维码已过期，请点击“重新生成”';document.getElementById('cd').textContent='有效期已结束'}
function countdown(seconds){document.getElementById('cd').textContent='二维码 '+seconds+'s 后过期';countdownTimer=setInterval(()=>{seconds--;document.getElementById('cd').textContent=seconds>0?'二维码 '+seconds+'s 后过期':'二维码已过期';if(seconds<=0)expireQR()},1000)}
async function api(url,options){
 const resp=await fetch(url,options);
 const text=await resp.text();
 let data=null;
 try{data=text?JSON.parse(text):null}catch(e){data=text}
 const isEnvelope=data&&typeof data==='object'&&!Array.isArray(data)&&Object.prototype.hasOwnProperty.call(data,'code')&&Object.prototype.hasOwnProperty.call(data,'msg')&&Object.prototype.hasOwnProperty.call(data,'data');
 if(!resp.ok||(isEnvelope&&data.code!==0)){throw new Error(isEnvelope?data.msg:'HTTP '+resp.status)}
 return isEnvelope?data.data:data;
}
async function newQR(){
 const current=++generation;
 stopTimers();sid=null;
 document.getElementById('qr').textContent='请稍候';
 const useProxy=document.getElementById('px').checked;
 document.getElementById('s').textContent=useProxy?'正在提取并检测代理':'正在生成二维码';
 document.getElementById('cd').textContent='';
 const r=await api('/qr',{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify({use_proxy:useProxy,area:useProxy?document.getElementById('area').value:'all'})});
 if(current!==generation)return;
 sid=r.session_id;
 document.getElementById('qr').innerHTML='<img alt="二维码" style="width:240px;height:240px" src="'+r.image_url+'">';
 document.getElementById('s').textContent='等待扫码'+(r.proxy_enabled?'（代理检测延迟 '+r.proxy_latency_ms+'ms）':'');
 countdown(Number(r.expires_in)||(r.proxy_enabled?60:110));
 pollTimer=setInterval(poll,1500);
}
async function poll(){
 const current=sid;if(!current)return;
 const r=await api('/qr/'+current+'/poll');if(current!==sid)return;
 document.getElementById('s').textContent=r.status;
 if(r.status==='authorized'){
  stopTimers();
  const a=await api('/qr/'+sid+'/confirm',{method:'POST'});
  document.getElementById('s').textContent='添加成功: '+(a.nickname||a.openid);
 }
 if(r.status==='expired'){expireQR()}
 if(['cancelled','unknown'].includes(r.status)){stopTimers();sid=null}
}
newQR();
</script></body></html>`

const docsHTML = `<!doctype html><html lang="zh-CN"><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1">
<title>YYB Go API 文档</title>
<body style="margin:32px;font-family:system-ui,-apple-system,BlinkMacSystemFont,'Segoe UI','Microsoft YaHei',sans-serif;line-height:1.6">
<h1>YYB Go API</h1>
<p>Swagger UI: <a href="/docs/index.html">/docs/index.html</a></p>
<p>OpenAPI JSON: <a href="/openapi.json">/openapi.json</a></p>
</body></html>`

var openAPISpec = newOpenAPISpec()
