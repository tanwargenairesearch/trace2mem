import copy
import json
from pathlib import Path
import unittest

from persona_verify import verify


class OfflineVerificationTests(unittest.TestCase):
    def report(self):
        case=json.loads((Path(__file__).parent/'personas/cases.json').read_text())['cases'][0]
        rows=[{'task_id':case['id'],'condition':c,'repeat':1,'passed':True,
               'result':{'artifact':dict(case['expected'])},'checks':{k:True for k in case['expected']},'latency_ms':1}
              for c in ('existing_memory','notes_sessions','trace2mem')]
        return {'suite':{'cases':[case]},'repeats':1,'results':rows,
                'summary':{r['condition']:{'attempts':1,'successes':1} for r in rows}}

    def test_recomputes_artifact_scores(self):
        report=self.report()
        self.assertEqual(verify(report)['trace2mem']['successes'],1)
        report['results'][0]['result']['artifact']['format']='wrong'
        with self.assertRaises(ValueError):verify(report)

    def test_rejects_summary_or_failure_success_mismatch(self):
        report=self.report()
        bad=copy.deepcopy(report);bad['summary']['trace2mem']['successes']=0
        with self.assertRaises(ValueError):verify(bad)
        bad=copy.deepcopy(report);bad['results'][0]['error']='provider_failure'
        with self.assertRaises(ValueError):verify(bad)
        bad=copy.deepcopy(report);bad['results'][0]['result']['error']='provider_failure'
        with self.assertRaises(ValueError):verify(bad)


if __name__=='__main__':unittest.main()
