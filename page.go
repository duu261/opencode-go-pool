package main

const quotaPageHTML = `<!doctype html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width,initial-scale=1">
<title>OpenCode Go Quota</title>
<style>
:root{color-scheme:light dark;font:14px/1.45 system-ui,sans-serif}body{margin:0;background:#101412;color:#e9f4ee}main{max-width:1100px;margin:auto;padding:24px}h1{margin:0 0 6px}p{color:#a8b7af}.unlock{display:flex;gap:8px;margin:20px 0}input,button{font:inherit;border:1px solid #415049;border-radius:7px;padding:9px 11px}input{min-width:260px;background:#171d1a;color:inherit}button{background:#72e0aa;color:#102017;font-weight:700;cursor:pointer}.status{min-height:22px;margin:10px 0;color:#ffad8f}table{width:100%;border-collapse:collapse;background:#171d1a;border:1px solid #34423b}th,td{text-align:left;padding:10px;border-bottom:1px solid #2b3731}th{color:#a8b7af}.healthy{color:#72e0aa}.unavailable,.disabled{color:#e4bd69}.error{color:#ff8f7a}.hidden{display:none}@media(max-width:760px){.table-wrap{overflow:auto}.unlock{display:grid}input{min-width:0}}
</style>
</head>
<body>
<main>
<h1>OpenCode Go Quota</h1>
<p>Read-only pool usage. Keys stay inside CLIProxyAPI and are represented by one-way fingerprints.</p>
<div class="unlock"><input id="key" type="password" autocomplete="off" placeholder="Management key"><button type="button" id="load">Load quota</button></div>
<div class="status" id="status">Enter the CLIProxyAPI management key. It is used once and never stored.</div>
<div class="table-wrap hidden" id="table-wrap"><table><thead><tr><th>Provider</th><th>Key</th><th>Status</th><th>5h used</th><th>5h reset</th><th>Weekly used</th><th>Weekly reset</th><th>Monthly used</th><th>Monthly reset</th></tr></thead><tbody id="rows"></tbody></table></div>
</main>
<script>
(()=>{const button=document.getElementById('load'),input=document.getElementById('key'),status=document.getElementById('status'),wrap=document.getElementById('table-wrap'),rows=document.getElementById('rows');const text=(tag,value,cls)=>{const el=document.createElement(tag);el.textContent=value;if(cls)el.className=cls;return el};const pct=w=>w&&Number.isFinite(w.percent)?w.percent+'%':'-';const reset=w=>w&&w.resetsAt?new Date(w.resetsAt).toLocaleString():'-';const load=async()=>{const key=input.value;input.value='';if(!key){status.textContent='Management key is required';return}status.textContent='Loading…';wrap.classList.add('hidden');rows.replaceChildren();try{const response=await fetch('/v0/management/plugins/opencode-go-quota/quotas',{headers:{Authorization:'Bearer '+key,Accept:'application/json'},cache:'no-store'});if(!response.ok)throw new Error(response.status===401?'Invalid management key':'Quota request failed: HTTP '+response.status);const data=await response.json();for(const r of data.results||[]){const usage=r.usage||{},rolling=usage.rolling,weekly=usage.weekly,monthly=usage.monthly,tr=document.createElement('tr');tr.append(text('td',r.provider_name||'-'),text('td',r.key_id||'-'),text('td',r.status||'-',r.status),text('td',pct(rolling)),text('td',reset(rolling)),text('td',pct(weekly)),text('td',reset(weekly)),text('td',pct(monthly)),text('td',reset(monthly)));rows.append(tr)}status.textContent=(data.results||[]).length+' credential(s), checked '+new Date(data.generated_at).toLocaleString();wrap.classList.remove('hidden')}catch(error){status.textContent=error instanceof Error?error.message:String(error)}};button.addEventListener('click',load);input.addEventListener('keydown',event=>{if(event.key==='Enter'){event.preventDefault();void load()}})})();
</script>
</body>
</html>`
