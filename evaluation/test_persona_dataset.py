import json
from pathlib import Path
import unittest

ROOT=Path(__file__).parent/'personas'


class PersonaDatasetTests(unittest.TestCase):
    def test_source_links_and_history_isolation(self):
        cases=json.loads((ROOT/'cases.json').read_text())['cases']
        self.assertEqual(len(cases),32)
        identities=set()
        for pid in ('nadia','marco','leena','owen'):
            events=[json.loads(line) for line in (ROOT/pid/'history.jsonl').read_text().splitlines()]
            self.assertEqual(len(events),18)
            self.assertEqual(len({e['sessionId'] for e in events}),6)
            ids={e['eventId'] for e in events}
            self.assertFalse(ids & identities)
            identities |= ids
            own=[c for c in cases if c['history_id']==pid]
            self.assertEqual(len(own),8)
            self.assertEqual({c['split'] for c in own},{'development' if pid in ('nadia','marco') else 'heldout'})
            for case in own:
                self.assertEqual(set(case['expected']),set(case['gold_sources']))
                for key, alternatives in case['gold_sources'].items():
                    self.assertTrue(alternatives)
                    self.assertTrue(set(alternatives) <= ids)
                    value=case['expected'][key]
                    if isinstance(value,str) and value!='assistant':
                        for source in alternatives:
                            text=next(e['message']['text'] for e in events if e['eventId']==source)
                            self.assertIn(value,text)
                if case['id'].endswith('-organizations'):
                    self.assertEqual(case['gold_sources']['first_city'],[f'{pid}-004',f'{pid}-010'])
                self.assertNotIn('gold_sources',case['input'])
                self.assertNotIn('expected',case['input'])

    def test_no_future_answer_artifacts_in_histories(self):
        for path in ROOT.glob('*/history.jsonl'):
            body=path.read_text()
            self.assertNotIn('gold_sources',body)
            self.assertNotIn('answer_pattern',body)
            self.assertNotIn('citations',body)
