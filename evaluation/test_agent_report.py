import unittest
import tempfile
import json
from pathlib import Path
from report_agent_benchmark import summarize, render


class ReportTests(unittest.TestCase):
    def report(self):
        return {"summary": {}, "repeats": 1,
                "suite": {"cases": [{"id": "x", "expected": {"a": 1, "b": 2}}]},
                "results": [
                    {"task_id": "x", "condition": "existing_memory", "repeat": 1,
                     "passed": True, "checks": {"a": True, "b": True}, "latency_ms": 100,
                     "result": {"usage": {"input_tokens": 50, "output_tokens": 10}}},
                    {"task_id": "x", "condition": "notes_sessions", "repeat": 1,
                     "passed": False, "error": "provider_failure", "latency_ms": 200,
                     "result": {"usage": {"input_tokens": 80, "estimated": True, "unresolved": True}}},
                    {"task_id": "x", "condition": "trace2mem", "repeat": 1,
                     "passed": False, "error": "agent_timeout", "latency_ms": 300}]}

    def test_failures_remain_in_field_and_token_totals(self):
        m = summarize(self.report())
        self.assertEqual(m['existing_memory']['passed_checks'], 2)
        self.assertEqual(m['notes_sessions']['checks'], 2)
        self.assertEqual(m['notes_sessions']['passed_checks'], 0)
        self.assertEqual(m['notes_sessions']['input_tokens'], 80)
        self.assertEqual(m['notes_sessions']['unresolved_trials'], 1)
        self.assertEqual(m['trace2mem']['missing_usage_trials'], 1)

    def test_historical_narrative_rejects_other_results(self):
        with tempfile.TemporaryDirectory() as root:
            source = Path(root) / "source.json"
            source.write_text(json.dumps(self.report()))
            with self.assertRaisesRegex(ValueError, "historical renderer"):
                render(source, Path(root) / "output")
            self.assertFalse((Path(root) / "output").exists())

    def test_rejects_incomplete_and_duplicate_trials(self):
        report = self.report()
        report['results'].pop()
        with self.assertRaises(ValueError):
            summarize(report)
        report = self.report()
        report['results'].append(report['results'][0])
        with self.assertRaises(ValueError):
            summarize(report)
        report = self.report()
        del report['summary']
        with self.assertRaises(ValueError):
            summarize(report)


if __name__ == '__main__':
    unittest.main()
