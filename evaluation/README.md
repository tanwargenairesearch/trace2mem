# Evaluate your own agent

This runner compares a trusted external agent under three conditions: its **existing memory**, Trace2Mem **notes + sessions**, and **full Trace2Mem**. It runs a fresh process for each trial, rotates condition order, withholds expected answers, checks exact structured artifact fields, and preserves failures in the denominator. Python 3.10+ on Linux/macOS; no model SDK is required by the runner.

The [Harbor directory-agent pilot](harbor/README.md) now runs this protocol over the earlier frozen export. Its 36 live trials exposed poor tool-use reliability and no memory benefit; they do not measure new compilation or held-out generalization. Deterministic runner tests alone establish no quality gains.

## Agent adapter contract

Wrap your existing agent with a program that reads one JSON object from stdin and writes one JSON object to stdout. The request includes `input`, `task_id`, `history_id`, `condition`, `revision`, `model`, `max_tokens`, and `repeat`. Credentials remain in your local environment. Use one user's credential/history per invocation. Stdout must contain only the JSON result; stderr is bounded and excluded from reports.

The **trusted adapter**, not the process launcher, must implement these conditions:

- `existing_memory`: use your documented existing memory mechanism. Do not connect Trace2Mem accidentally through inherited tools or cached context.
- `notes_sessions`: use the pinned revision; hide knowledge pages, wiki index and wiki cache. Search with `withoutWiki:true`; expose only notes/session reads and their evidence. Construct initial orientation from those permitted surfaces.
- `trace2mem`: load the pinned knowledge index and expose bounded search/read/evidence tools.

Keep generation model, task prompt, non-memory tools, maximum foreground tokens and acceptance criteria matched. Verify condition isolation in your adapter's tests. An environment flag is not a security boundary: the runner cannot independently prove which tools an arbitrary executable uses. The model/revision echo is checked for consistency, not independently attested.

A result has this shape:

```json
{
  "model": "the-exact-requested-model",
  "revision": "the-requested-memory-revision",
  "artifact": {
    "launch_date": "2026-12-01",
    "raw_retention_days": 7,
    "consumer_count": 9,
    "decision_status": "approved",
    "decision_authority": "user"
  },
  "trace": [],
  "usage": {"input_tokens": 0, "output_tokens": 0, "estimated": false},
  "evidence_read": []
}
```

`artifact` and `model` are required. The two Trace2Mem conditions also require the exact `revision`; existing memory may omit it. An optional stable lowercase `error` code marks an adapter execution failure; its result and usage are retained without artifact scoring. An optional suite `agent_config_sha256` is passed to the adapter and checked in its response; the adapter must verify the fingerprint before calling its provider. Other fields are retained as adapter-reported evidence. Include actual tool calls/results, provider request IDs, errors/retries, and usage to make a real result auditable. Zero usage in this schema example is a placeholder, not measured consumption. The runner does not calculate semantic citation support or total monetary cost automatically.

## Run

Copy [example-suite.json](example-suite.json), replace model/revision placeholders, and connect the history IDs to your adapter's isolated fixtures. The Harbor example is a development case; use separate histories for held-out evaluation. Expected fields use dotted object paths, such as `decision.status`. Checks compare exact JSON values; they do not infer semantic equivalence or validate unlisted extra claims.

```sh
python3 evaluation/agent_benchmark.py \
  --suite /absolute/path/to/pinned-suite.json \
  --output "$PWD/.local/agent-comparison.json" \
  --repeats 3 --timeout 120 \
  --agent python3 /absolute/path/to/your-agent-adapter.py
```

The adapter runs as a local executable without a shell wrapper. The launcher enforces a per-trial timeout and 2 MiB aggregate stdout/stderr limit, and kills the process group on completion/failure. Token budgets are enforced by the adapter/provider. There are at most 100 cases and five repeats; report checkpoints are atomic, private (0600), capped at 16 MiB, and never overwrite an existing report path. Split larger runs into separate reports; if the cap is exceeded, the previous valid checkpoint remains. Reports contain source-derived artifacts and gold expectations: keep real-user reports private.

```sh
python3 -m unittest discover -s evaluation -v
```

Tests cover condition rotation, hidden gold answers, retained failures, wrong revision/status, sanitized protocol error codes, subprocess invocation, timeouts and output bounds. They use a scripted fixture, not an inference model. See [the outcome plan](../docs/OUTCOMES.md) for independent histories, task success/currentness, source support, uncertainty intervals and amortized compilation costs. Do not publish an “agent delta” until your actual adapter is verified and the paired model trials are run.


## Recorded persona hill-climb

The [user-history dataset](personas/README.md) and [192-trial report](../reports/2026-09-08-persona-evaluation/README.md) add real Dream snapshots, development-only candidate selection, held-out runs, source-read accounting and complete failure records. Use `python3 evaluation/persona_verify.py --report /path/to/shared-report.json` to recompute saved artifact scores and metrics without model access. Read the [RCA](../reports/2026-09-08-persona-evaluation/RCA.md) before interpreting the small positive wiki comparison as semantic-quality gains.
