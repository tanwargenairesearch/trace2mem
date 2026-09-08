const $=id=>document.getElementById(id);
async function api(path,body){const r=await fetch(path,{method:body===undefined?'GET':'POST',headers:{'Content-Type':'application/json','X-Trace2Mem-CSRF':'1'},body:body===undefined?undefined:JSON.stringify(body)});if(!r.ok)throw Error(await r.text());return r.json()}
const management=(route,body)=>api('/api/'+route,body);
const rpc=(service,method,body)=>api('/trace2mem.v1.'+service+'/'+method,body||{});
function action(id,fn){$(id).onclick=async()=>{try{$('notice').style.display='none';$(id).disabled=true;await fn()}catch(e){$('notice').textContent=e.message;$('notice').style.display='block'}finally{$(id).disabled=id==='ask'&&!askReady}}}
function show(id,v){if(id==='status'){const details=[];if(v.error)details.push(v.error);if(v.configured)details.push('Model configuration saved.');if(v.scheduled)details.push('Compilation queued.');if(v.status)details.push(v.status);if(v.jobStatus)details.push('Compilation: '+v.jobStatus);if(v.accepted!==undefined)details.push(v.accepted+' events accepted');if(v.revision)details.push('Revision '+v.revision.slice(0,8));if(v.lastError)details.push(v.lastError);$(id).textContent=details.join(' · ')||'Workspace ready';return}$(id).textContent=JSON.stringify(v,null,2)}
let askReady=false;
async function refresh(){await refreshReadiness()}
async function refreshReadiness(){
 askReady=false;$('ask').disabled=true;
 const [status,models,schedule]=await Promise.all([rpc('IngestionService','GetIngestionStatus',{}),management('model-status'),management('schedule')]);
 $('next-run').textContent=schedule.next_run?'Next queued compilation: '+new Date(schedule.next_run).toLocaleString():'No future compilation queued.';
 show('status',status);
 askReady=!!(models.generation_configured&&models.embedding_configured&&status.revision);
 $('ask').disabled=!askReady;
 $('ask-readiness').textContent=!models.generation_configured||!models.embedding_configured?'Ask is unavailable: configure generation and embedding models in Models.':!status.revision?'Models configured. Import events and compile memory to enable Ask.':models.test_provider?'Test provider active — answers are scripted fixtures, not real model responses.':'Ready to ask · '+models.generation_model+' · '+models.embedding_model;
}

action('refresh',refresh);
action('configure',async()=>{
 if(!$('model-form').reportValidity())return;
 const generation=$('provider').value,embedding=$('embedding-provider').value;
 const body={provider:generation==='openrouter'?'openai':generation,model:$('model').value.trim(),embedding_provider:embedding,embedding_model:$('embedding').value.trim()};
 if(generation!=='scripted')body.endpoint=$('endpoint').value.trim()||(generation==='openrouter'?'https://openrouter.ai/api/v1':'');
 if(generation==='openai'||generation==='openrouter')body.key=$('key').value;
 if(embedding!=='scripted')body.embedding_endpoint=$('embedding-endpoint').value.trim();
 if(embedding==='openai'||embedding==='gemini')body.embedding_key=$('embedding-key').value;
 if(embedding==='vertex'){body.embedding_project=$('embedding-project').value.trim();body.embedding_location=$('embedding-location').value.trim();}
 if((embedding==='vertex'||embedding==='gemini')&&$('embedding-dimensions').value)body.embedding_dimensions=Number($('embedding-dimensions').value);
 await management('model',body);$('key').value='';$('embedding-key').value='';show('status',{configured:true});await refreshReadiness();
});
action('import',async()=>{const f=$('file').files[0];if(!f)throw Error('Select a JSONL file');const lines=(await f.text()).split('\n').filter(x=>x.trim());for(let i=0;i<lines.length;i+=64){await rpc('IngestionService','AppendEvents',{events:lines.slice(i,i+64).map(JSON.parse)})}await refresh()});
action('compile',async()=>show('status',await management('compile',{})));action('forget',async()=>{if(confirm('Forget this source and rebuild derived memory?')){show('status',await management('forget',{event_id:$('forget-id').value}))}});
action('search',async()=>show('result',await rpc('MemoryService','Search',{query:$('query').value})));action('index',async()=>show('result',await rpc('MemoryService','ReadFile',{path:'knowledge/index.md'})));action('evidence',async()=>show('result',await rpc('MemoryService','GetEvidence',{eventId:$('evidence-id').value})));action('export',async()=>{const v=await management('export');const url=URL.createObjectURL(new Blob([JSON.stringify(v,null,2)],{type:'application/json'}));const a=document.createElement('a');a.href=url;a.download='trace2mem-'+v.revision+'.json';a.click();URL.revokeObjectURL(url)});
action('revisions',async()=>show('admin-result',await management('revisions')));action('usage',async()=>show('admin-result',await management('usage')));action('token',async()=>show('token-result',await api('/api/tokens',{scopes:$('token-scopes').value.split(',')})));
action('evaluate',async()=>show('eval-result',await management('evaluate',{cases:JSON.parse($('cases').value)})));action('candidate',async()=>show('eval-result',await management('candidates',{evaluation_id:$('evaluation-id').value})));action('promote',async()=>{if(confirm('Promote this evaluated candidate for future compilations?'))show('eval-result',await management('candidates',{promote_id:$('candidate-id').value}))});refresh().catch(e=>{show('status',{error:e.message})});
for(const button of document.querySelectorAll('[data-panel]')){button.addEventListener('click',()=>{for(const panel of document.querySelectorAll('.panel'))panel.classList.toggle('active',panel.id==='panel-'+button.dataset.panel);for(const nav of document.querySelectorAll('[data-panel]'))nav.classList.toggle('active',nav===button);$('view-title').textContent=button.querySelector('span').textContent})}
action('ask',async()=>{if(!askReady)throw Error('Configure models and publish memory before asking.');const v=await rpc('MemoryService','GetContext',{query:$('query').value});$('result').textContent=v.synthesis||'No supported answer found in this memory.'});
$('query').addEventListener('keydown',e=>{if(e.key==='Enter')$('ask').click()});

function modelFields(role){
 const generation=role==='generation';const selected=$(generation?'provider':'embedding-provider').value;
 const fields=$(generation?'generation-fields':'embedding-fields');fields.hidden=!selected;fields.disabled=!selected;
 const endpoint=$(generation?'endpoint':'embedding-endpoint');const key=$(generation?'key':'embedding-key');
 $(generation?'model':'embedding').value='';endpoint.value='';key.value='';
 const cloudKey=generation?(selected==='openai'||selected==='openrouter'):(selected==='openai'||selected==='gemini');
 $(role+'-key-field').hidden=!cloudKey;key.disabled=!cloudKey;key.required=cloudKey;
 $(role+'-endpoint-field').hidden=selected==='scripted';endpoint.disabled=selected==='scripted';endpoint.required=selected==='ollama';
 $(role+'-endpoint-note').textContent=selected==='ollama'?'required':'optional';
 endpoint.placeholder=selected==='ollama'?'http://host.docker.internal:11434':selected==='openrouter'?'https://openrouter.ai/api/v1':selected==='openai'?'https://api.openai.com/v1':'Provider default';
 $(role+'-help').textContent=selected==='ollama'?'Use an installed model and an Ollama endpoint reachable from the service.':selected==='vertex'?'Uses the service’s application default credentials (ADC). No API key is required.':selected==='scripted'?'Deterministic test provider; does not run a real model.':selected==='openrouter'?'Routes generation through OpenRouter. Enter its model ID, such as moonshotai/kimi-k3.':'Enter a model available to your account. No model is selected automatically.';
 if(!generation){$('vertex-fields').hidden=selected!=='vertex';$('vertex-fields').disabled=selected!=='vertex';$('embedding-project').value='';$('embedding-location').value='';const dimensions=selected==='vertex'||selected==='gemini';$('embedding-dimensions-field').hidden=!dimensions;$('embedding-dimensions').disabled=!dimensions;$('embedding-dimensions').value='';}
}
$('provider').addEventListener('change',()=>modelFields('generation'));
$('embedding-provider').addEventListener('change',()=>modelFields('embedding'));
$('model-form').addEventListener('submit',e=>{e.preventDefault();$('configure').click()});

$('mcp-endpoint').textContent=location.origin+'/mcp';
function scheduleFields(){const daily=$('schedule-mode').value==='daily';$('daily-schedule').hidden=!daily;$('daily-schedule').disabled=!daily}
$('schedule-mode').onchange=scheduleFields;
management('schedule').then(c=>{$('schedule-mode').value=c.mode;$('schedule-time').value=c.time;$('schedule-zone').value=c.timezone;scheduleFields()}).catch(e=>show('status',{error:e.message}));
action('save-schedule',async()=>{if(!$('schedule-form').reportValidity())return;await management('schedule',{mode:$('schedule-mode').value,time:$('schedule-time').value,timezone:$('schedule-zone').value});await refresh();$('notice').textContent='Compilation schedule saved.';$('notice').style.display='block'});
