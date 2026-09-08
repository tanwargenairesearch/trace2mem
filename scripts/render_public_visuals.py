"""Render shareable architecture and measured evaluation figures from repository sources."""
import json
from pathlib import Path
import matplotlib
matplotlib.use('Agg')
import matplotlib.pyplot as plt
from matplotlib.patches import FancyBboxPatch, FancyArrowPatch

ROOT = Path(__file__).resolve().parents[1]
OUT = ROOT / 'docs/assets'
BG, INK, MUTED = '#FAF3E7', '#2A1F14', '#6B5D4D'
RUST, GOLD, OLIVE = '#CC785C', '#C97B23', '#788B38'
plt.rcParams.update({'font.family': 'DejaVu Sans', 'text.color': INK, 'axes.labelcolor': INK,
                     'svg.fonttype': 'none', 'font.size': 11})

def canvas():
    fig = plt.figure(figsize=(16, 10), facecolor=BG)
    ax = fig.add_axes([0, 0, 1, 1], xlim=(0, 16), ylim=(0, 10))
    ax.axis('off')
    return fig, ax

def text(ax, x, y, s, size=12, color=INK, weight='normal', **kwargs):
    ax.text(x, y, s, fontsize=size, color=color, weight=weight, va='top', **kwargs)

def box(ax, x, y, w, h, title, body, color=RUST):
    ax.add_patch(FancyBboxPatch((x, y), w, h, boxstyle='round,pad=0.02,rounding_size=0.16',
                              facecolor='#FFFDF7', edgecolor='#E8D9BD', linewidth=1.3))
    ax.plot([x+.2, x+.2], [y+.22, y+h-.22], color=color, lw=4, solid_capstyle='round')
    text(ax, x+.4, y+h-.23, title, 15, weight='bold')
    text(ax, x+.4, y+h-.66, body, 11, MUTED, linespacing=1.5)

def arrow(ax, start, end, dashed=False):
    ax.add_patch(FancyArrowPatch(start, end, arrowstyle='-|>', mutation_scale=15,
                                color=MUTED, linewidth=1.4, linestyle='--' if dashed else '-'))

def save(fig, name):
    for ext in ('svg','png','pdf'):
        fig.savefig(OUT / f'{name}.{ext}', dpi=180, facecolor=BG)
    plt.close(fig)

fig, ax = canvas()
text(ax, .65, 9.58, 'TRACE2MEM  /  SYSTEM ARCHITECTURE', 11, RUST, 'bold')
text(ax, .65, 9.13, 'Experience becomes memory. Memory informs action.', 27, weight='bold')
text(ax, .65, 8.55, 'One logical memory per authenticated user, shared by that user’s agents.', 14, MUTED)
text(ax, .65, 7.97, '01   DURING THE CONVERSATION', 10, GOLD, 'bold')
box(ax, .65, 6.22, 4.15, 1.37, 'Your agent', 'LangChain or a custom harness\nCapture live events or import saved JSONL')
box(ax, 5.45, 6.22, 4.65, 1.37, 'Trace2Mem server', 'Authenticated ingestion + retrieval\nHTTP / gRPC · MCP tools', GOLD)
box(ax, 10.75, 6.22, 4.6, 1.37, 'Progressive memory access', 'Index → search → pages → evidence\nOptional pinned directory / Linux FUSE', OLIVE)
arrow(ax, (4.83,6.92), (5.42,6.92)); arrow(ax,(10.13,6.92),(10.72,6.92))
text(ax, .65, 5.86, '02   DURABLE MEMORY', 10, GOLD, 'bold')
box(ax,.65,4.0,4.15,1.42,'Sessions','Original events + precise evidence\nModel-authored session summaries')
box(ax,5.45,4.0,4.65,1.42,'Notes','Distilled Markdown observations\nCitations, corrections, temporal status',GOLD)
box(ax,10.75,4.0,4.6,1.42,'Knowledge wiki','Synthesized subject pages + links\nCompact index + revision changelog',OLIVE)
ax.plot([7.77,7.77,2.72], [6.19,5.65,5.65], color=MUTED, lw=1.4)
arrow(ax,(2.72,5.65),(2.72,5.45)); text(ax,5.65,5.98,'durable capture',9,MUTED)
arrow(ax,(13.05,5.45),(13.05,6.19)); text(ax,13.23,5.98,'published reads',9,MUTED)

text(ax,5.65,3.79,'Notes and wiki claims cite source events; subject links provide context.',10,MUTED)
text(ax,1.35,3.35,'03   BACKGROUND MAINTENANCE · DREAM WORKER',10,GOLD,'bold')
box(ax,.65,1.87,14.7,1.1,'Orient → Summarize → Investigate → Compose → Verify → Publish',
    'Inspects and updates all three layers · bounded tools · verified, atomic publication · justified no-op',RUST)
arrow(ax,(.95,3.97),(.95,3.01),True); arrow(ax,(14.95,3.01),(14.95,3.97),True)
text(ax,.85,1.48,'PostgreSQL',12,weight='bold'); text(ax,.85,1.17,'Events · jobs · revision coordination',10,MUTED)
text(ax,6.0,1.48,'Interchangeable blob storage',12,weight='bold'); text(ax,6.0,1.17,'Uploaded sources and artifacts',10,MUTED)
text(ax,11.25,1.48,'Configured models',12,weight='bold'); text(ax,11.25,1.17,'Independent generation + embeddings',10,MUTED)
text(ax,.65,.59,'Capture is separate from MCP retrieval. Dashed arrows: background maintenance. Local files pin one revision.',10,MUTED)
text(ax,.65,.29,'Inspired by Brain by Perplexity · Implementation: docs/ARCHITECTURE.md · github.com/tanwargenairesearch/trace2mem',9,MUTED)
save(fig,'trace2mem-architecture')

summary = json.loads((ROOT/'reports/2026-09-08-persona-evaluation/evaluation/heldout-summary.json').read_text())
keys = ['existing_memory','notes_sessions','trace2mem']
rows = [summary[k] for k in keys]
labels = ['Full history', 'Notes + sessions', 'Full wiki']
colors = ['#BAAA91', GOLD, OLIVE]
fig, ax = canvas()
text(ax,.65,9.58,'TRACE2MEM  /  MEASURED RESULTS',11,RUST,'bold')
text(ax,.65,9.12,'A small task-score gain. A real token trade-off.',27,weight='bold')
text(ax,.65,8.53,'Held-out synthetic user histories · 2 personas × 8 questions × 2 repeats · 32 trials per condition',13,MUTED)
metrics = [([r['successes']/r['attempts']*100 for r in rows], 'Exact-task success', 'Higher is better', 100,
            [f"{r['successes']}/{r['attempts']}  ·  {r['successes']/r['attempts']:.1%}" for r in rows]),
           ([r['accounted_tokens']/1000 for r in rows], 'Foreground tokens', 'Thousands · lower is better · 32 trials total', 600,
            [f"{r['accounted_tokens']:,}" for r in rows]),
           ([r['median_latency_seconds'] for r in rows], 'Median answer latency', 'Lower is better · seconds', 25,
            [f"{r['median_latency_seconds']:.2f} s" for r in rows])]
for i,(values,title,subtitle,limit,values_text) in enumerate(metrics):
    x=.65+i*5.12
    text(ax,x,7.82,title,17,weight='bold'); text(ax,x,7.39,subtitle,10,MUTED)
    chart=fig.add_axes([x/16,.435,4.52/16,.255],facecolor=BG)
    chart.barh([2,1,0],values,color=colors,height=.34)
    chart.set_xlim(0,limit); chart.set_ylim(-.6,2.65)
    chart.set_yticks([]); chart.tick_params(axis='x',colors=MUTED,labelsize=9,length=0,pad=8)
    chart.set_axisbelow(True); chart.grid(axis='x',color='#E8D9BD',linewidth=.8)
    for spine in chart.spines.values(): spine.set_visible(False)
    for j,(label,value,note) in enumerate(zip(labels,values,values_text)):
        chart.text(0,2-j+.25,label,fontsize=10,weight='bold',va='bottom')
        chart.text(value+limit*.025,2-j,note,fontsize=9,va='center')
    chart.set_xlim(0,limit*1.18)
a,b,c=rows
success_delta=(c['successes']/c['attempts']-a['successes']/a['attempts'])*100
token_notes=(1-c['accounted_tokens']/b['accounted_tokens'])*100
token_history=(c['accounted_tokens']/a['accounted_tokens']-1)*100
box(ax,.65,2.68,4.68,1.13,f'+{success_delta:.1f} percentage points','Wiki vs full history · exact-task success',OLIVE)
box(ax,5.66,2.68,4.68,1.13,f'{token_notes:.1f}% fewer tokens','Wiki vs notes + sessions · foreground only',OLIVE)
box(ax,10.67,2.68,4.68,1.13,f'{token_history:.1f}% more tokens','Wiki vs full history · foreground only',RUST)
text(ax,.65,2.24,'WHAT THIS SUPPORTS',10,RUST,'bold')
text(ax,.65,1.92,'A narrow observed improvement in task completion, not established broad semantic gains or cost savings.',12,weight='bold')
text(ax,.65,1.52,'Exact JSON / expected-value scoring; execution and formatting affect results. All failed trials remain included.\nShared task templates, two held-out personas; no confidence intervals. Snapshot preparation is outside latency.\nForeground charts exclude compilation: 402,516 recorded tokens across development + held-out compilation attempts,\nincluding estimated embeddings. That total is not a per-condition charge. No monetary cost comparison.',10,MUTED,linespacing=1.5)
text(ax,.65,.40,'Inspired by Brain by Perplexity · Trace2Mem results, not Perplexity benchmarks · Source: reports/2026-09-08-persona-evaluation',9,MUTED)
save(fig,'trace2mem-performance')
print('Rendered architecture and performance as SVG, PNG and PDF')
