#!/usr/bin/env python3
"""Recompute exact answer checks and metrics from a shared persona report, offline."""
import argparse
import json
from pathlib import Path

from agent_benchmark import score, CONDITIONS
from persona_evaluation import metrics


def verify(report):
    cases={case['id']:case for case in report['suite']['cases']}
    for row in report['results']:
        if row.get('result',{}).get('error') and not row.get('error'):
            raise ValueError('adapter failure missing from trial status')
        if row.get('error'):
            if row['passed']:raise ValueError('execution failure recorded as success')
            continue
        checks=score(cases[row['task_id']],row['result'])
        if row.get('checks')!=checks or row['passed']!=all(checks.values()):
            raise ValueError('saved score differs from answer artifact')
    expected={condition:{'attempts':sum(r['condition']==condition for r in report['results']),
                         'successes':sum(r['condition']==condition and r['passed'] for r in report['results'])} for condition in CONDITIONS}
    if report.get('summary')!=expected:raise ValueError('saved task summary differs from trials')
    return metrics(report)


if __name__=='__main__':
    parser=argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--report',required=True)
    args=parser.parse_args()
    print(json.dumps(verify(json.loads(Path(args.report).read_text())),indent=2))
