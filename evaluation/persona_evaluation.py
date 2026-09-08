#!/usr/bin/env python3
"""Freeze persona suites, compare development candidates, and evaluate a frozen selection."""
import argparse
from collections import Counter
import hashlib
import json
import os
from pathlib import Path
import statistics
import subprocess

from agent_benchmark import run

PROTOCOLS={'optional':'optional_v1','controlled':'controlled_v1'}
CONDITIONS=('existing_memory','notes_sessions','trace2mem')


def digest(path):
    return hashlib.sha256(Path(path).read_bytes()).hexdigest()


def save(path,value):
    fd=os.open(path,os.O_WRONLY|os.O_CREAT|os.O_EXCL,0o600)
    with os.fdopen(fd,'w') as f:
        json.dump(value,f,indent=2,allow_nan=False)
        f.write('\n')


def metrics(report):
    cases={c['id']:c for c in report['suite']['cases']}
    expected={(c,condition,repeat) for c in cases for condition in CONDITIONS for repeat in range(1,report['repeats']+1)}
    actual=[(r['task_id'],r['condition'],r['repeat']) for r in report['results']]
    if 'summary' not in report or set(actual)!=expected or len(actual)!=len(expected):
        raise ValueError('complete trial matrix required')
    result={}
    for condition in CONDITIONS:
        rows=[r for r in report['results'] if r['condition']==condition]
        errors=Counter(r['error'] for r in rows if r.get('error'))
        checks=sum(len(cases[r['task_id']]['expected']) for r in rows)
        source_hits=citation_hits=0
        no_reads=0
        tokens=0;missing_usage=0;unresolved_usage=0
        families={}
        for row in rows:
            case=cases[row['task_id']]
            data=row.get('result',{})
            usage=data.get('usage')
            if usage is None:missing_usage+=1
            else:
                tokens+=usage.get('input_tokens',0)+usage.get('output_tokens',0)
                unresolved_usage+=bool(usage.get('unresolved'))
            citations=data.get('artifact',{}).get('citations',{})
            if not isinstance(citations,dict):citations={}
            sources=set(data.get('evidence_read',[]))
            for field,alternatives in case['gold_sources'].items():
                source_hits+=bool(sources & set(alternatives))
                cited=citations.get(field,[])
                if isinstance(cited,list):citation_hits+=any(isinstance(c,str) and c in alternatives for c in cited)
            read=False
            for turn in data.get('trace',[]):
                tool=turn.get('Result')
                if not tool:continue
                try:value=json.loads(tool['Text'])
                except (KeyError,ValueError,TypeError):continue
                values=value if isinstance(value,list) else [value]
                if any(isinstance(v,dict) and 'content' in v and 'path' in v and not v.get('error') for v in values):read=True
            if not read:no_reads+=1
            family=families.setdefault(case['family'],{'attempts':0,'successes':0})
            family['attempts']+=1;family['successes']+=bool(row['passed'])
        result[condition]={'attempts':len(rows),'successes':sum(r['passed'] for r in rows),'checks':checks,
            'passed_checks':sum(sum(r.get('checks',{}).values()) for r in rows if not r.get('error')),
            'errors':dict(errors),'no_file_read_trials':no_reads if condition!='existing_memory' else None,
            'gold_source_coverage':source_hits/checks if condition!='existing_memory' else None,
            'gold_citation_agreement':citation_hits/checks,'accounted_tokens':tokens,'missing_usage_trials':missing_usage,'unresolved_usage_trials':unresolved_usage,
            'median_latency_seconds':statistics.median([r['latency_ms'] for r in rows if not r.get('latency_incomplete')] or [0])/1000,'incomplete_latency_trials':sum(bool(r.get('latency_incomplete')) for r in rows),'families':families}
    return result


def choose(reports):
    scores={}
    for name,report in reports.items():
        m=metrics(report)
        scores[name]={'successes':sum(m[c]['successes'] for c in CONDITIONS[1:]),
                     'errors':sum(sum(m[c]['errors'].values()) for c in CONDITIONS[1:]),
                     'tokens':sum(m[c]['accounted_tokens'] for c in CONDITIONS[1:]),
                     'missing_usage':sum(m[c]['missing_usage_trials']+m[c]['unresolved_usage_trials'] for c in CONDITIONS[1:])}
    # Unknown usage cannot win a cost tie. Name makes an exact tie deterministic.
    winner=min(scores,key=lambda n:(-scores[n]['successes'],scores[n]['errors'],
                                   scores[n]['missing_usage']>0,scores[n]['tokens'],n))
    return winner,scores


def prepare(root,agent,snapshots):
    dataset=json.loads((Path(__file__).parent/'personas/cases.json').read_text())
    manifests={}
    for pid in ('nadia','marco','leena','owen'):
        manifest=json.loads((snapshots/pid/'manifest.json').read_text())
        provenance=json.loads((snapshots/pid/'compilation.json').read_text())
        if not provenance.get('published') or provenance['revision']!=manifest['revision']:
            raise ValueError('all personas require published memory; retain compilation failures before retrying')
        # Compare raw event envelopes to the independently authored history, not model summaries.
        expected={e['eventId']:e for e in map(json.loads,(Path(__file__).parent/'personas'/pid/'history.jsonl').read_text().splitlines())}
        actual={}
        for entry in manifest['files']:
            path=Path(entry['path'])
            if path.is_absolute() or '..' in path.parts:raise ValueError('unsafe manifest')
            body=(snapshots/pid/'memory'/path).read_bytes()
            if digest(snapshots/pid/'memory'/path)!=entry['sha256'] or len(body)!=int(entry['size']):raise ValueError('snapshot altered')
            if str(path).startswith('sessions/evidence/'):
                event=json.loads(body)['event'];actual[event['eventId']]=event
        if expected!=actual:raise ValueError('snapshot evidence differs from authored history')
        manifests[pid]=manifest
    for name,protocol in PROTOCOLS.items():
        request={'model':'moonshotai/kimi-k3','history_id':'nadia','condition':'trace2mem',
                 'revision':manifests['nadia']['revision'],'max_tokens':64000,'protocol':protocol}
        env=dict(os.environ,TRACE2MEM_AGENT_DESCRIBE_CONFIG='1',TRACE2MEM_AGENT_SNAPSHOT_ROOT=str(snapshots))
        config=json.loads(subprocess.check_output([str(agent)],input=json.dumps(request).encode(),env=env,timeout=10))
        if 'config_sha256' not in config:raise ValueError('model configuration probe failed')
        for split in ('development','heldout'):
            cases=[dict(c,revision=manifests[c['history_id']]['revision']) for c in dataset['cases'] if c['split']==split]
            suite={'model':'moonshotai/kimi-k3','max_tokens':64000,'agent_protocol':protocol,'agent_config_sha256':config['config_sha256'],
                   'configuration':config['configuration'],'cases':cases}
            save(root/f'{name}-{split}-suite.json',suite)
    save(root/'freeze.json',{'agent_sha256':digest(agent),'dataset_sha256':digest(Path(__file__).parent/'personas/cases.json'),
         'manifest_sha256':{pid:digest(snapshots/pid/'manifest.json') for pid in manifests},
         'suite_sha256':{p.name:digest(p) for p in root.glob('*-suite.json')}})


def check_freeze(root,agent,snapshots):
    frozen=json.loads((root/'freeze.json').read_text())
    if digest(agent)!=frozen['agent_sha256']:raise ValueError('agent binary changed after freeze')
    if digest(Path(__file__).parent/'personas/cases.json')!=frozen['dataset_sha256']:raise ValueError('dataset changed after freeze')
    for name,sha in frozen['suite_sha256'].items():
        if digest(root/name)!=sha:raise ValueError('suite changed after freeze')
    for pid,sha in frozen['manifest_sha256'].items():
        if digest(snapshots/pid/'manifest.json')!=sha:raise ValueError('manifest changed after freeze')


def evaluate_suite(root,stem,suite,agent,repeats):
    original=root/f'{stem}.json'
    paths=([original] if original.exists() else [])+sorted(root.glob(f'{stem}-continuation-*.json'))
    prior=None
    for path in paths:
        report=json.loads(path.read_text())
        if report.get('suite')!=suite or report.get('repeats')!=repeats or report.get('timeout_seconds')!=180:
            raise ValueError('existing candidate report differs from frozen suite')
        if 'summary' in report:
            metrics(report)
            return report,path
        prior=path
    target=original if prior is None else root/f'{stem}-continuation-{len(paths):03d}.json'
    return run(suite,[str(agent)],repeats,180,target,resume=prior),target


def save_or_verify(path,value):
    if path.exists():
        if json.loads(path.read_text())!=value:raise ValueError('existing evidence summary differs')
    else:save(path,value)


def main():
    parser=argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--stage',choices=['prepare','development','heldout'],required=True)
    parser.add_argument('--output',required=True)
    parser.add_argument('--snapshots',required=True)
    parser.add_argument('--agent',required=True)
    args=parser.parse_args()
    root=Path(args.output).resolve();root.mkdir(parents=True,exist_ok=True)
    snapshots=Path(args.snapshots).resolve();agent=Path(args.agent).resolve()
    if args.stage=='prepare':prepare(root,agent,snapshots);return
    check_freeze(root,agent,snapshots)
    os.environ['TRACE2MEM_AGENT_SNAPSHOT_ROOT']=str(snapshots)
    os.environ.pop('TRACE2MEM_AGENT_DESCRIBE_CONFIG',None)
    if args.stage=='development':
        reports={};paths={}
        for name in PROTOCOLS:
            print(f'Running {name}: development only',flush=True)
            suite=json.loads((root/f'{name}-development-suite.json').read_text())
            reports[name],paths[name]=evaluate_suite(root,f'{name}-development',suite,agent,1)
            save_or_verify(root/f'{name}-rca.json',metrics(reports[name]))
        winner,scores=choose(reports)
        save_or_verify(root/'selection.json',{'selected':winner,'protocol':PROTOCOLS[winner],'scores':scores,
             'development_reports':{name:{'file':paths[name].name,'sha256':digest(paths[name])} for name in PROTOCOLS}})
        print(json.dumps({'selected':winner,'scores':scores}),flush=True)
    else:
        selection=json.loads((root/'selection.json').read_text())
        reports={}
        for name,entry in selection['development_reports'].items():
            path=root/entry['file']
            if digest(path)!=entry['sha256']:raise ValueError('development evidence changed')
            reports[name]=json.loads(path.read_text())
        winner,scores=choose(reports)
        if winner!=selection['selected'] or scores!=selection['scores']:raise ValueError('selection differs from development rule')
        suite=json.loads((root/f'{winner}-heldout-suite.json').read_text())
        print(f'Running frozen {winner}: held-out histories',flush=True)
        report,_=evaluate_suite(root,'heldout',suite,agent,2)
        summary=metrics(report);save_or_verify(root/'heldout-summary.json',summary)
        print(json.dumps(summary),flush=True)


if __name__=='__main__':main()
