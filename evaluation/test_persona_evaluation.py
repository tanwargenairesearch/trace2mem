import copy
import unittest
import tempfile
import json
from pathlib import Path
from persona_evaluation import metrics, choose, evaluate_suite


class PersonaEvaluationTests(unittest.TestCase):
    def report(self):
        case={'id':'q','family':'currentness','expected':{'date':'2026-10-14'},'gold_sources':{'date':['correction','confirmation']}}
        rows=[]
        for condition in ('existing_memory','notes_sessions','trace2mem'):
            rows.append({'task_id':'q','condition':condition,'repeat':1,'passed':True,'checks':{'date':True},'latency_ms':100,
                         'result':{'artifact':{'date':'2026-10-14','citations':{'date':['confirmation']}},
                                   'evidence_read':['confirmation'],'usage':{'input_tokens':100,'output_tokens':10},
                                   'trace':[{'Result':{'Text':'{"path":"sessions/evidence/confirmation.json","content":"confirmed"}'}}]}})
        return {'suite':{'cases':[case]},'repeats':1,'results':rows,'summary':{}}

    def test_alternative_sources_and_baseline_na(self):
        summary=metrics(self.report())
        self.assertIsNone(summary['existing_memory']['gold_source_coverage'])
        self.assertEqual(summary['trace2mem']['gold_source_coverage'],1)
        self.assertEqual(summary['trace2mem']['gold_citation_agreement'],1)
        self.assertEqual(summary['trace2mem']['no_file_read_trials'],0)

    def test_failures_kept_in_scores_and_selection(self):
        good=self.report();bad=copy.deepcopy(good)
        bad['results'][2].update(passed=False,error='provider_failure')
        del bad['results'][2]['checks']
        bad['results'][2]['result']={'usage':{'input_tokens':200}}
        summary=metrics(bad)
        self.assertEqual(summary['trace2mem']['checks'],1)
        self.assertEqual(summary['trace2mem']['passed_checks'],0)
        self.assertEqual(summary['trace2mem']['accounted_tokens'],200)
        winner,scores=choose({'optional':bad,'controlled':good})
        self.assertEqual(winner,'controlled')
        self.assertEqual(scores['optional']['successes'],1)
        # A hypothesis is not promoted when its measured success regresses.
        winner,_=choose({'optional':good,'controlled':bad})
        self.assertEqual(winner,'optional')

    def test_requires_complete_comparison(self):
        report=self.report();report['results'].pop()
        with self.assertRaises(ValueError):metrics(report)

    def test_uncertain_usage_cannot_win_tie(self):
        exact=self.report();uncertain=copy.deepcopy(exact)
        uncertain['results'][2]['result']['usage']={'input_tokens':1,'unresolved':True,'estimated':True}
        winner,scores=choose({'optional':exact,'controlled':uncertain})
        self.assertEqual(winner,'optional')
        self.assertEqual(scores['controlled']['missing_usage'],1)

    def test_completed_candidate_reused_without_model_call(self):
        report=self.report();report['timeout_seconds']=180
        with tempfile.TemporaryDirectory() as root:
            path=Path(root)/'optional-development.json'
            path.write_text(json.dumps(report))
            reused,actual=evaluate_suite(Path(root),'optional-development',report['suite'],Path('/does-not-exist'),1)
            self.assertEqual(reused,report)
            self.assertEqual(actual,path)
