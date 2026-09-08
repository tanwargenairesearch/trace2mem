import json
from pathlib import Path
import sys
import tempfile
import unittest

from agent_benchmark import ProtocolError, invoke, run


class BenchmarkTests(unittest.TestCase):
    def suite(self):
        return {"model": "scripted-fixture", "max_tokens": 1000, "cases": [{
            "id": "approval", "history_id": "harbor", "revision": "r1", "split": "heldout",
            "input": "Produce current decision state", "expected": {
                "decision.approved": True, "decision.authority": "user", "decision.status": "current"}}]}

    def test_conditions_gold_isolation_and_failure_denominator(self):
        requests = []
        def agent(command, request, timeout):
            requests.append(request)
            self.assertNotIn("expected", request)
            if request["condition"] == "existing_memory":
                raise TimeoutError()
            return {"model":request["model"],"revision": request["revision"], "artifact": {"decision": {
                "approved": True, "authority": "user", "status": "current"}}}
        with tempfile.TemporaryDirectory() as root:
            report = run(self.suite(), ["fixture"], 3, 1, Path(root) / "report.json", call=agent)
            self.assertEqual(len(report["results"]), 9)
            self.assertEqual(report["summary"]["existing_memory"], {"attempts": 3, "successes": 0})
            self.assertEqual([requests[n]["condition"] for n in (0, 3, 6)],
                             ["existing_memory", "notes_sessions", "trace2mem"])
            self.assertEqual(report["summary"]["trace2mem"]["successes"], 3)

    def test_wrong_status_and_revision_fail(self):
        def agent(command, request, timeout):
            return {"model":request["model"],"revision": "wrong", "artifact": {"decision": {
                "approved": "not approved", "authority": "assistant", "status": "historical"}}}
        with tempfile.TemporaryDirectory() as root:
            report = run(self.suite(), ["fixture"], 1, 1, Path(root) / "report.json", call=agent)
            self.assertFalse(any(row["passed"] for row in report["results"]))
            self.assertEqual(report["results"][1]["error"], "revision_mismatch")

    def test_subprocess_round_trip_and_timeout(self):
        result = invoke([sys.executable, "-c", 'import json,sys; r=json.load(sys.stdin); print(json.dumps({"artifact":{"echo":r["input"]}}))'], {"input": "hello"}, 2)
        self.assertEqual(result["artifact"]["echo"], "hello")
        with self.assertRaises(TimeoutError):
            invoke([sys.executable, "-c", "import time; time.sleep(10)"], {}, 0.1)

    def test_protocol_error_codes(self):
        for code, program in [("invalid_json", 'print("not-json")'), ("invalid_artifact", 'print("{}")'), ("agent_exit", 'raise SystemExit(1)')]:
            with self.subTest(code=code), self.assertRaises(ProtocolError) as caught:
                invoke([sys.executable, "-c", program], {}, 2)
            self.assertEqual(caught.exception.code, code)

    def test_output_bound(self):
        with self.assertRaises(ValueError):
            invoke([sys.executable, "-c", 'print("x"*(3<<20))'], {}, 2)


if __name__ == "__main__":
    unittest.main()
