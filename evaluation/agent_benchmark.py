#!/usr/bin/env python3
"""Run a trusted external agent under matched memory conditions; no model dependency."""
import argparse
import hashlib
import json
import math
import os
from pathlib import Path
import selectors
import signal
import subprocess
import tempfile
import time

CONDITIONS = ("existing_memory", "notes_sessions", "trace2mem")
MAX_BYTES = 2 << 20


class ProtocolError(ValueError):
    def __init__(self, code):
        super().__init__(code)
        self.code = code


def invoke(command, request, timeout):
    payload = json.dumps(request).encode()
    if len(payload) > MAX_BYTES:
        raise ProtocolError("input_limit")
    with tempfile.TemporaryFile() as source:
        source.write(payload)
        source.seek(0)
        process = subprocess.Popen(command, stdin=source, stdout=subprocess.PIPE,
                                   stderr=subprocess.PIPE, start_new_session=True)
        chunks = []
        size = 0
        deadline = time.monotonic() + timeout
        try:
            with selectors.DefaultSelector() as selector:
                selector.register(process.stdout, selectors.EVENT_READ, "stdout")
                selector.register(process.stderr, selectors.EVENT_READ, "stderr")
                while selector.get_map():
                    remaining = deadline - time.monotonic()
                    if remaining <= 0:
                        raise TimeoutError("agent deadline exceeded")
                    for key, _ in selector.select(min(remaining, 0.1)):
                        data = os.read(key.fileobj.fileno(), 65536)
                        if not data:
                            selector.unregister(key.fileobj)
                            continue
                        size += len(data)
                        if size > MAX_BYTES:
                            raise ProtocolError("output_limit")
                        if key.data == "stdout":
                            chunks.append(data)
                code = process.wait(timeout=max(0.001, deadline - time.monotonic()))
                if code:
                    raise ProtocolError("agent_exit")
            try:
                result = json.loads(b"".join(chunks))
            except (ValueError, UnicodeError) as error:
                raise ProtocolError("invalid_json") from error
            if not isinstance(result, dict) or not isinstance(result.get("artifact"), dict):
                raise ProtocolError("invalid_artifact")
            return result
        finally:
            # Also stop descendants retaining descriptors after their parent exits.
            try:
                os.killpg(process.pid, signal.SIGKILL)
            except ProcessLookupError:
                pass
            process.wait()
            process.stdout.close()
            process.stderr.close()


def field(artifact, path):
    value = artifact
    for part in path.split("."):
        if not isinstance(value, dict) or part not in value:
            return None, False
        value = value[part]
    return value, True


def score(case, result):
    checks = {}
    for path, expected in case["expected"].items():
        actual, found = field(result["artifact"], path)
        # JSON equality preserves boolean versus integer meaning.
        checks[path] = found and json.dumps(actual, sort_keys=True, allow_nan=False) == json.dumps(expected, sort_keys=True, allow_nan=False)
    return checks


def validate(suite, repeats, timeout):
    if not 1 <= repeats <= 5 or not math.isfinite(timeout) or not 0 < timeout <= 900:
        raise ValueError("repeats must be 1–5 and timeout 0–900 seconds")
    if not isinstance(suite.get("model"), str) or not suite["model"].strip() or "REPLACE" in suite["model"] or "latest" in suite["model"].lower():
        raise ValueError("explicit model identifier required")
    if not isinstance(suite.get("max_tokens"), int) or not 0 < suite["max_tokens"] <= 1000000:
        raise ValueError("explicit max_tokens between 1 and 1000000 required")
    cases = suite.get("cases", [])
    if not 1 <= len(cases) <= 100:
        raise ValueError("provide 1–100 cases")
    ids = set()
    for case in cases:
        if not case.get("id") or case["id"] in ids or not case.get("history_id") or not case.get("revision") or "REPLACE" in case["revision"]:
            raise ValueError("cases need unique IDs, history IDs and pinned revisions")
        ids.add(case["id"])
        if case.get("split") not in ("development", "heldout"):
            raise ValueError("case split must be development or heldout")
        if not isinstance(case.get("expected"), dict) or not 1 <= len(case["expected"]) <= 50:
            raise ValueError("cases need 1–50 expected artifact fields")
        if any(not isinstance(key, str) or not key for key in case["expected"]):
            raise ValueError("expected fields must be named paths")
        if "input" not in case:
            raise ValueError("case input required")


def run(suite, command, repeats, timeout, output, call=invoke, resume=None):
    validate(suite, repeats, timeout)
    report = {"suite_sha256": hashlib.sha256(json.dumps(suite, sort_keys=True).encode()).hexdigest(),
              "suite": suite, "repeats": repeats, "timeout_seconds": timeout,
              "conditions": CONDITIONS, "results": [],
              "limitations": "Trusted adapter enforces memory conditions and budgets. Usage and evidence access are adapter-reported, not independently attested."}
    if resume is not None:
        previous_bytes = Path(resume).read_bytes()
        if len(previous_bytes) > 16 << 20:
            raise ValueError("prior report exceeds limit")
        previous = json.loads(previous_bytes)
        if previous.get("suite") != suite or previous.get("repeats") != repeats or previous.get("timeout_seconds") != timeout:
            raise ValueError("resume configuration differs")
        report["results"] = previous["results"]
        report["resumed_from_sha256"] = hashlib.sha256(previous_bytes).hexdigest()
        if previous.get("in_flight"):
            interrupted = previous["in_flight"]
            interrupted.update(passed=False,error="interrupted_unknown_usage",latency_ms=0,latency_incomplete=True)
            report["results"].append(interrupted)
    resume_count = len(report["results"])
    if resume_count > len(suite["cases"]) * repeats * len(CONDITIONS):
        raise ValueError("resume contains too many trials")
    resume_index = 0
    output = Path(output)
    output.parent.mkdir(parents=True, exist_ok=True)
    if output.exists():
        raise ValueError("choose a new report path; existing reports are immutable")
    # Reserve the output path once, then replace it atomically with valid checkpoints.
    fd = os.open(output, os.O_WRONLY | os.O_CREAT | os.O_EXCL, 0o600)
    os.close(fd)
    def checkpoint():
        data = json.dumps(report, indent=2, allow_nan=False).encode()
        if len(data) > 16 << 20:
            raise ValueError("report exceeds 16 MiB; split the suite")
        fd, name = tempfile.mkstemp(prefix=".benchmark-", dir=output.parent)
        try:
            with os.fdopen(fd, "wb") as stream:
                stream.write(data)
                stream.flush()
                os.fsync(stream.fileno())
            os.replace(name, output)
        finally:
            if os.path.exists(name):
                os.unlink(name)
    checkpoint()
    for repeat in range(repeats):
        for index, case in enumerate(suite["cases"]):
            rotation = (repeat + index) % len(CONDITIONS)
            order = CONDITIONS[rotation:] + CONDITIONS[:rotation]
            for condition in order:
                # Gold expectations stay outside the agent's input.
                request = {"task_id": case["id"], "history_id": case["history_id"],
                           "input": case["input"], "condition": condition,
                           "revision": case["revision"] if condition != "existing_memory" else None,
                           "model": suite["model"], "max_tokens": suite["max_tokens"],
                           "repeat": repeat + 1}
                if suite.get("agent_protocol"):
                    request["protocol"] = suite["agent_protocol"]
                if suite.get("agent_config_sha256"):
                    request["config_sha256"] = suite["agent_config_sha256"]
                if resume_index < resume_count:
                    previous_row = report["results"][resume_index]
                    if previous_row.get("request") != request or any(previous_row.get(k) != request[k] for k in ("task_id", "history_id", "condition", "repeat")):
                        raise ValueError("resume is not the expected trial prefix")
                    resume_index += 1
                    continue
                started = time.monotonic()
                row = {"task_id": case["id"], "history_id": case["history_id"],
                       "split": case["split"], "condition": condition, "repeat": repeat + 1,
                       "passed": False, "request": request}
                report["in_flight"] = dict(row)
                checkpoint()
                try:
                    result = call(command, request, timeout)
                    # Reject non-JSON numeric values before checkpointing the trial.
                    json.dumps(result, allow_nan=False)
                    if result.get("model") != suite["model"]:
                        raise ProtocolError("model_mismatch")
                    if condition != "existing_memory" and result.get("revision") != case["revision"]:
                        raise ProtocolError("revision_mismatch")
                    row["result"] = result
                    if result.get("error"):
                        code = result["error"]
                        if not isinstance(code, str) or not code or len(code) > 64 or any(c not in "abcdefghijklmnopqrstuvwxyz_" for c in code):
                            code = "adapter_failure"
                        raise ProtocolError(code)
                    if suite.get("agent_config_sha256") and result.get("config_sha256") != suite["agent_config_sha256"]:
                        raise ProtocolError("configuration_mismatch")
                    row["checks"] = score(case, result)
                    row["passed"] = all(row["checks"].values())
                except (OSError, ValueError, TypeError, KeyError, RuntimeError, TimeoutError, subprocess.TimeoutExpired) as error:
                    # Provider stderr or exception details may contain credentials.
                    row["error"] = (error.code if isinstance(error, ProtocolError) else
                                    "agent_timeout" if isinstance(error, (TimeoutError, subprocess.TimeoutExpired)) else
                                    type(error).__name__)
                row["latency_ms"] = round((time.monotonic() - started) * 1000)
                report["results"].append(row)
                report.pop("in_flight", None)
                checkpoint()
    report["summary"] = {condition: {
        "attempts": sum(row["condition"] == condition for row in report["results"]),
        "successes": sum(row["condition"] == condition and row["passed"] for row in report["results"])
    } for condition in CONDITIONS}
    checkpoint()
    return report


if __name__ == "__main__":
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--suite", required=True)
    parser.add_argument("--output", required=True)
    parser.add_argument("--repeats", type=int, default=2)
    parser.add_argument("--timeout", type=float, default=120)
    parser.add_argument("--agent", nargs=argparse.REMAINDER, required=True)
    args = parser.parse_args()
    if not args.agent:
        parser.error("--agent requires an executable and optional arguments")
    suite_bytes = Path(args.suite).read_bytes()
    if len(suite_bytes) > MAX_BYTES:
        parser.error("suite exceeds 2 MiB")
    result = run(json.loads(suite_bytes), args.agent, args.repeats, args.timeout, args.output)
    print(json.dumps(result["summary"], indent=2))
