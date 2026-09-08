#!/usr/bin/env python3
"""Render recorded persona development and held-out results with complete accounting."""
import argparse
import hashlib
import json
from pathlib import Path, PurePosixPath
import re

from persona_evaluation import choose
from persona_verify import verify

LABELS={'existing_memory':'Full history','notes_sessions':'Notes + sessions','trace2mem':'Full wiki'}


def redact(value):
    if isinstance(value,str):
        return re.sub(r'("resume_token"\s*:\s*)"[^"]*"',r'\1"[redacted provider handle]"',value)
    if isinstance(value,list):return [redact(v) for v in value]
    if isinstance(value,dict):return {k:redact(v) for k,v in value.items()}
    return value


def copy_corpus(source,target,frozen):
    for persona,expected_hash in frozen['manifest_sha256'].items():
        if not re.fullmatch(r'[a-z0-9_-]+',persona):raise ValueError('unsafe persona path')
        folder=source/persona
        manifest=(folder/'manifest.json').read_bytes()
        if hashlib.sha256(manifest).hexdigest()!=expected_hash:raise ValueError('frozen manifest changed')
        files={}
        for entry in json.loads(manifest)['files']:
            path=PurePosixPath(entry['path'])
            if path.is_absolute() or '..' in path.parts or str(path)!=entry['path'] or str(path) in ('','.', 'manifest.json') or str(path) in files:
                raise ValueError('unsafe or duplicate manifest path')
            file=folder/'memory'/str(path)
            if not file.resolve().is_relative_to((folder/'memory').resolve()):raise ValueError('manifest file escapes snapshot')
            content=file.read_bytes()
            if len(content)!=int(entry['size']) or hashlib.sha256(content).hexdigest()!=entry['sha256']:
                raise ValueError('snapshot content changed')
            files[str(path)]=content
        destination=target/persona;destination.mkdir(parents=True)
        (destination/'manifest.json').write_bytes(manifest)
        for path,content in files.items():
            file=destination/'memory'/path;file.parent.mkdir(parents=True,exist_ok=True);file.write_bytes(content)


def token_delta(wiki,base):
    if not base['accounted_tokens'] or any(m[k] for m in (wiki,base) for k in ('missing_usage_trials','unresolved_usage_trials')):
        return 'N/A'
    return f"{100*(wiki['accounted_tokens']/base['accounted_tokens']-1):+.1f}%"


def render(experiment,out):
    root=Path(experiment);out=Path(out)
    if out.exists():raise ValueError('choose a new output directory')
    evaluation=root/'evaluation'
    selection=json.loads((evaluation/'selection.json').read_text())
    frozen=json.loads((evaluation/'freeze.json').read_text())
    dataset_path=Path(__file__).parent/'personas/cases.json'
    if hashlib.sha256(dataset_path.read_bytes()).hexdigest()!=frozen['dataset_sha256']:
        raise ValueError('report requires the authored persona dataset')
    for name,sha in frozen['suite_sha256'].items():
        if hashlib.sha256((evaluation/name).read_bytes()).hexdigest()!=sha:raise ValueError('frozen suite changed')
    reports={}
    original_hashes={}
    for name,entry in selection['development_reports'].items():
        path=evaluation/entry['file']
        if hashlib.sha256(path.read_bytes()).hexdigest()!=entry['sha256']:raise ValueError('development report changed')
        reports[name]=json.loads(path.read_text())
        if reports[name]['suite']!=json.loads((evaluation/f'{name}-development-suite.json').read_text()):raise ValueError('development did not use frozen suite')
    winner,scores=choose(reports)
    if winner!=selection['selected'] or scores!=selection['scores']:raise ValueError('selection mismatch')
    heldout_paths=[evaluation/'heldout.json']+sorted(evaluation.glob('heldout-continuation-*.json'))
    heldout=None
    for path in heldout_paths:
        if path.exists():
            candidate=json.loads(path.read_text())
            if 'summary' in candidate:heldout=candidate;heldout_file=path.name;break
    if heldout is None:raise ValueError('held-out evaluation incomplete')
    selected_suite=json.loads((evaluation/f'{winner}-heldout-suite.json').read_text())
    if heldout['suite']!=selected_suite:raise ValueError('heldout did not use selected suite')
    reports['heldout']=heldout
    summaries={name:verify(report) for name,report in reports.items()}
    out.mkdir(parents=True)
    # Share all checkpoints, not just favorable final results; redact opaque continuation handles.
    (out/'evaluation').mkdir()
    for path in sorted(evaluation.glob('*.json')):
        original_hashes[f'evaluation/{path.name}']=hashlib.sha256(path.read_bytes()).hexdigest()
        (out/'evaluation'/path.name).write_text(json.dumps(redact(json.loads(path.read_text())),indent=2)+'\n')
    compilation=[]
    for attempt in ('snapshots','repair'):
        for path in sorted((root/attempt).glob('*/compilation.json')):
            row=json.loads(path.read_text());row['attempt']=attempt
            compilation.append(row)
            folder=out/'compilation'/attempt/path.parent.name;folder.mkdir(parents=True)
            for name in ('compilation.json','proposals.json'):
                source=path.parent/name
                if source.exists():
                    original_hashes[str(source.relative_to(root))]=hashlib.sha256(source.read_bytes()).hexdigest()
                    (folder/name).write_text(json.dumps(redact(json.loads(source.read_text())),indent=2)+'\n')
    # The model-generated memory is itself evidence; preserve exact file hashes.
    copy_corpus(root/'corpus',out/'corpus',frozen)
    (out/'source-hashes.json').write_text(json.dumps(original_hashes,indent=2)+'\n')
    (out/'summary.json').write_text(json.dumps({'selection':selection,'metrics':summaries,'compilation':compilation},indent=2)+'\n')
    h=summaries['heldout']
    lines=['# User-history memory evaluation','',
           'Four authored synthetic users, six conversations and eight questions each. Two users were used for development; the other two were evaluated only after candidate selection. The same task templates are shared across users, so this is held-out persona evaluation within a narrow distribution, not broad generalization or a reproduction of Brain\'s proprietary benchmark.','',
           '![Development and held-out comparison](comparison.png)','',
           '## Development hill-climb','',
           'Candidate A uses optional memory tools. Candidate B requires a structured action and a successful memory read before answering in file-memory conditions; cited sources must be resolved. Generation settings are matched. Within each candidate, its full-history baseline uses that candidate\'s finalization protocol. The gold answers and questions were frozen before foreground inference. The protocol hypothesis follows the earlier Harbor failure analysis; this development run tests and selects it.','',
           '| Candidate | Full-history tasks | Notes/session tasks | Wiki tasks | Notes/wiki execution errors | Notes/wiki tokens |','|---|---:|---:|---:|---:|---:|']
    for name in ('optional','controlled'):
        m=summaries[name]
        passed=[f"{m[c]['successes']}/{m[c]['attempts']}" for c in LABELS]
        lines.append(f"| {name} | "+' | '.join(passed)+f" | {scores[name]['errors']} | {scores[name]['tokens']:,} |")
    lines += ['',f"The pre-registered rule selected **{winner}**: maximize combined notes/wiki task successes, then minimize execution errors, then compare usage with uncertainty penalized. The selected configuration was frozen before held-out inference. This is one development candidate change, not an open-ended search for favorable results.",'',
              '## Root-cause evidence','']
    for name in ('optional','controlled'):
        for condition in ('notes_sessions','trace2mem'):
            m=summaries[name][condition]
            errors=', '.join(f'{code}: {n}' for code,n in m['errors'].items()) or 'none'
            lines.append(f"- {name} / {LABELS[condition]}: {m['no_file_read_trials']}/{m['attempts']} trials without a successful file read; execution failures: {errors}.")
    lines += ['', 'The strict JSON scorer rejects prose preambles and malformed artifacts even when they contain correct facts. Exact-task success measures the requested machine-readable contract, not a blanket judgment of semantic knowledge. The controlled candidate changes both tool-use and finalization, so their individual causal contributions are not isolated.', '', 'All supplied compilation attempts, including failures and repairs, are retained below. Compilation failures are separate from foreground task scores; a publishable revision is required before the comparison starts.','',
              '## Held-out results','',
              '| Condition | Tasks correct | Fields correct | Gold-source coverage¹ | Gold-citation agreement² | Median latency³ | Foreground tokens |',
              '|---|---:|---:|---:|---:|---:|---:|']
    for condition,m in h.items():
        recall='N/A' if m['gold_source_coverage'] is None else f"{100*m['gold_source_coverage']:.1f}%"
        lines.append(f"| {LABELS[condition]} | {m['successes']}/{m['attempts']} | {m['passed_checks']}/{m['checks']} | {recall} | {100*m['gold_citation_agreement']:.1f}% | {m['median_latency_seconds']:.2f} s | {m['accounted_tokens']:,} |")
    lines += ['', '¹ Fraction of expected answer fields for which an acceptable gold source was completely read. Repeated fields can share a source; this is not unique-event recall. Full-history exposure is not a tool read, so its retrieval metric is N/A.','',
              '² Fraction of fields citing at least one acceptable gold event ID. This checks source agreement, not independent semantic support or the correctness of every additional cited claim.','',
              '³ Medians exclude interrupted trials with unknown latency; counts are disclosed below. Lower latency/tokens on failed work do not establish savings.','',
              '## Held-out deltas','', '| Full wiki compared with | Task-success change | Field-correctness change | Foreground token change |','|---|---:|---:|---:|']
    wiki=h['trace2mem']
    for condition in ('existing_memory','notes_sessions'):
        base=h[condition]
        task=100*(wiki['successes']/wiki['attempts']-base['successes']/base['attempts'])
        fields=100*(wiki['passed_checks']/wiki['checks']-base['passed_checks']/base['checks'])
        tokens=token_delta(wiki,base)
        lines.append(f"| {LABELS[condition]} | {task:+.1f} pp | {fields:+.1f} pp | {tokens} |")
    lines += ['', '## Held-out categories','', '| Category | Full history | Notes + sessions | Full wiki |','|---|---:|---:|---:|']
    for family in sorted(h['trace2mem']['families']):
        cells=[]
        for condition in LABELS:
            m=h[condition]['families'][family];cells.append(f"{m['successes']}/{m['attempts']}")
        lines.append(f"| {family} | "+' | '.join(cells)+' |')
    lines += ['', '## Accounting and failures','', '| Compilation attempt | Persona | Published | Generation tokens | Estimated embedding tokens |','|---|---|---:|---:|']
    compilation_total=0
    for c in compilation:
        usage=c.get('usage',[])
        gen=sum(u['input_tokens']+u['output_tokens'] for u in usage if 'generation' in u['operation'])
        emb=sum(u['input_tokens']+u['output_tokens'] for u in usage if 'embedding' in u['operation'])
        compilation_total+=gen+emb
        lines.append(f"| {c['attempt']} | {c['history_id']} | {c['published']} | {gen:,} | {emb:,} |")
    foreground=sum(m['accounted_tokens'] for summary in summaries.values() for m in summary.values())
    lines += ['',f"Recorded foreground usage across development and held-out trials: **{foreground:,} tokens**. Recorded compilation usage across all retained attempts: **{compilation_total:,} tokens**, including estimated embeddings. No monetary cost or break-even claim is made: no price schedule is supplied, and hosted cache billing may differ from token accounting. Authored histories did not require model generation.",'']
    for condition,m in h.items():
        lines.append(f"- Held-out {LABELS[condition]}: errors {json.dumps(m['errors'],sort_keys=True)}; {m['missing_usage_trials']} missing-usage, {m['unresolved_usage_trials']} unresolved-usage, {m['incomplete_latency_trials']} incomplete-latency trials.")
    for row in heldout['results']:
        if not row['passed']:
            reason=row.get('error') or ', '.join(k for k,v in row.get('checks',{}).items() if not v)
            lines.append(f"- {row['task_id']} / {LABELS[row['condition']]} / repeat {row['repeat']}: {reason}.")
    lines += ['', '## Interpretation limits','',
              'Only two held-out personas and shared task templates were tested. Dates, entities and status fields are exact-value checks; the reason clause must be copied verbatim. Semantically equivalent paraphrases can fail, and a correct field does not prove all accompanying claims. No independent semantic judge or confidence interval is claimed. All scheduled attempts, including failures, remain in the denominator.','',
              'The comparison uses the real Dream-generated wiki and a consuming agent over hash-verified directory snapshots. Search is bounded keyword matching, not live API or embedding retrieval. Six conversations are compiled together in one initial run per persona; this does not measure incremental publication freshness. Full history fits comfortably in context and is deliberately a strong baseline. Deployment, public hosting and native macOS FUSE are outside this evaluation.','',
              '## Reproduce and inspect','',
              '- [Dataset and pre-registered protocol](../../evaluation/personas/README.md)',
              f'- [Offline artifact-score verifier](../../evaluation/persona_verify.py) — run with `--report evaluation/{heldout_file}` from this report directory; no model calls.',
              '- [Frozen suite/binary/manifest hashes](../../evaluation/personas/frozen/freeze.json)',
              '- [Candidate selection](evaluation/selection.json)',
              '- [All metrics and compilation accounting](summary.json)',
              '- [Source hashes before provider-handle redaction](source-hashes.json)',
              '- [Generated memory snapshots](corpus/)',
              '- [Complete evaluation checkpoints](evaluation/)',
              '- [Compilation attempts and verifier diagnostics](compilation/)',
              '', 'Opaque provider continuation handles in message text are redacted; answer artifacts and measured scores are preserved. No real-user histories or credentials are intentionally included.']
    (out/'README.md').write_text('\n'.join(lines)+'\n')
    import matplotlib
    matplotlib.use('Agg')
    import matplotlib.pyplot as plt
    fig,axes=plt.subplots(1,2,figsize=(12,4.5),layout='constrained')
    colors=['#979b95','#567d8c','#237e80']
    for offset,(condition,label) in enumerate(LABELS.items()):
        values=[100*summaries[candidate][condition]['successes']/summaries[candidate][condition]['attempts'] for candidate in ('optional','controlled')]
        bars=axes[0].bar([offset*.25,1+offset*.25],values,width=.23,label=label,color=colors[offset])
        axes[0].bar_label(bars,fmt='%.0f%%',padding=3,fontsize=9)
    axes[0].set_xticks([.25,1.25],['Optional tools','Controlled actions'])
    axes[0].set_title('Development: candidate task success')
    values=[100*m['successes']/m['attempts'] for m in h.values()]
    bars=axes[1].bar(['Full history','Notes + sessions','Full wiki'],values,color=colors,width=.6)
    axes[1].bar_label(bars,fmt='%.1f%%',padding=4)
    axes[1].set_title(f'Held-out: frozen {winner} candidate')
    for ax in axes:
        ax.set_ylim(0,115);ax.set_ylabel('Exact-task success (%)');ax.spines[['top','right']].set_visible(False)
    axes[0].legend(loc='upper center',bbox_to_anchor=(.5,-.12),ncols=3,fontsize=8)
    fig.suptitle('Synthetic user-memory pilot • development tuning kept separate',fontsize=13)
    fig.savefig(out/'comparison.png',dpi=160,bbox_inches='tight');plt.close(fig)


if __name__=='__main__':
    p=argparse.ArgumentParser(description=__doc__)
    p.add_argument('--experiment',required=True);p.add_argument('--output',required=True)
    a=p.parse_args();render(a.experiment,a.output)
