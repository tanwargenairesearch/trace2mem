#!/usr/bin/env python3
"""Render the recorded Harbor pilot, preserving its failures and historical commentary."""
import argparse
import hashlib
import json
from pathlib import Path
import statistics
import re

LABELS = {"existing_memory": "Full original history", "notes_sessions": "Notes + sessions", "trace2mem": "Full Trace2Mem"}


def summarize(report):
    if "summary" not in report:
        raise ValueError("report is incomplete")
    cases = {case["id"]: case for case in report["suite"]["cases"]}
    expected_trials = {(case, condition, repeat) for case in cases for condition in LABELS
                       for repeat in range(1, report["repeats"] + 1)}
    keys = [(row["task_id"], row["condition"], row["repeat"]) for row in report["results"]]
    if len(set(keys)) != len(keys) or set(keys) != expected_trials:
        raise ValueError("incomplete or duplicated trial matrix")
    metrics = {}
    for condition in LABELS:
        rows = [row for row in report["results"] if row["condition"] == condition]
        checks = sum(len(cases[row["task_id"]]["expected"]) for row in rows)
        passed_checks = sum(sum(row.get("checks", {}).values()) for row in rows if not row.get("error"))
        usages = [row.get("result", {}).get("usage") for row in rows]
        metrics[condition] = {
            "attempts": len(rows), "successes": sum(row["passed"] for row in rows),
            "checks": checks, "passed_checks": passed_checks,
            "errors": sum(bool(row.get("error")) for row in rows),
            "no_tool_call_trials": sum(not any(turn.get("Calls") for turn in row.get("result", {}).get("trace", [])) for row in rows),
            "median_latency_seconds": statistics.median(row["latency_ms"] for row in rows) / 1000,
            "input_tokens": sum(u.get("input_tokens", 0) for u in usages if u is not None),
            "output_tokens": sum(u.get("output_tokens", 0) for u in usages if u is not None),
            "estimated_trials": sum(bool(u and u.get("estimated")) for u in usages),
            "unresolved_trials": sum(bool(u and u.get("unresolved")) for u in usages),
            "missing_usage_trials": sum(u is None for u in usages),
        }
    return metrics


def render(source, output):
    raw = Path(source).read_bytes()
    if hashlib.sha256(raw).hexdigest() not in {"58425b4cd61781a2f522327f4332dee95b46a82004058fb30fe3df1d99d8fe22", "c10774c540d724db6693aaa58d1da99f2441e9cbd3496926b244de1b0945277b"}:
        raise ValueError("historical renderer accepts only the recorded Harbor report or its shared redacted copy")
    report = json.loads(raw)
    metrics = summarize(report)
    if len(report["suite"]["cases"]) != 6 or report["repeats"] != 2 or any(c["history_id"] != "harbor" for c in report["suite"]["cases"]):
        raise ValueError("this renderer requires the six-task, two-repeat Harbor pilot")
    out = Path(output)
    out.mkdir(parents=True, exist_ok=True)
    for name in ("results.json", "summary.json", "README.md", "comparison.png"):
        if (out / name).exists():
            raise ValueError("choose an empty output directory")
    def redact(value):
        if isinstance(value, str):
            return re.sub(r'(\"resume_token\"\s*:\s*)\"[^\"]*\"', r'\1"[redacted provider handle]"', value)
        if isinstance(value, list):
            return [redact(v) for v in value]
        if isinstance(value, dict):
            return {k: redact(v) for k, v in value.items()}
        return value
    (out / "results.json").write_text(json.dumps(redact(report), indent=2) + "\n")
    (out / "summary.json").write_text(json.dumps(metrics, indent=2) + "\n")
    lines = ["# Harbor: matched memory-dependent tasks", "",
             "A synthetic development pilot: six tasks, three conditions, two repeats per task. All conditions use fresh Kimi processes and the same task prompts and foreground budget. The baseline receives the complete original history; it has memory. Other conditions read the same checkpoint through permitted file tools.", "",
             "The saved wiki predates the latest Dream verification changes. This measures consuming-agent behavior over that export, not new compilation quality, a production integration, or independent held-out generalization.", "",
             "![Matched task comparison](comparison.png)", "",
             "| Condition | Tasks passed | Fields correct | Execution failures | Median latency | Accounted tokens |",
             "|---|---:|---:|---:|---:|---:|"]
    for condition, m in metrics.items():
        lines.append(f"| {LABELS[condition]} | {m['successes']}/{m['attempts']} | {m['passed_checks']}/{m['checks']} | {m['errors']} | {m['median_latency_seconds']:.2f} s | {m['input_tokens'] + m['output_tokens']:,} |")
    lines += ["", "## Deltas", "", "Wiki minus comparison condition. Lower tokens or latency alongside failed tasks are not demonstrated savings or faster successful work.", "",
              "| Comparison | Task success change | Field accuracy change | Accounted token change | Median latency change |", "|---|---:|---:|---:|---:|"]
    wiki = metrics["trace2mem"]
    for condition in ("existing_memory", "notes_sessions"):
        base = metrics[condition]
        task_delta = 100 * (wiki['successes'] / wiki['attempts'] - base['successes'] / base['attempts'])
        field_delta = 100 * (wiki['passed_checks'] / wiki['checks'] - base['passed_checks'] / base['checks'])
        tokens = base['input_tokens'] + base['output_tokens']
        token_delta = f"{100 * ((wiki['input_tokens'] + wiki['output_tokens']) / tokens - 1):+.1f}%" if tokens and not (base['missing_usage_trials'] or wiki['missing_usage_trials']) else "N/A"
        latency_delta = 100 * (wiki['median_latency_seconds'] / base['median_latency_seconds'] - 1) if base['median_latency_seconds'] else 0
        lines.append(f"| Full Trace2Mem vs {LABELS[condition]} | {task_delta:+.1f} pp | {field_delta:+.1f} pp | {token_delta} | {latency_delta:+.1f}% |")
    lines += ["", "## By task", "", "| Task | Full history | Notes + sessions | Full Trace2Mem |", "|---|---:|---:|---:|"]
    for case in report['suite']['cases']:
        values = []
        for condition in LABELS:
            rows = [r for r in report['results'] if r['task_id'] == case['id'] and r['condition'] == condition]
            values.append(f"{sum(r['passed'] for r in rows)}/{len(rows)}")
        lines.append(f"| {case['id']} | " + " | ".join(values) + " |")
    lines += ["", "## Observed agent behavior", "",
              f"The notes/session agent made no memory-tool call in {metrics['notes_sessions']['no_tool_call_trials']}/12 trials; the wiki agent made none in {metrics['trace2mem']['no_tool_call_trials']}/12. In this pilot, making tools available did not reliably cause retrieval. Several other trials produced malformed final output; one exhausted its conservative retrieval budget. These are end-to-end agent failures, not evidence that wiki content is intrinsically worse after successful retrieval.", "",
              "The single full-history failure labeled the initial outbox status proposed rather than approved. The rubric targets the original user decision (harbor-0011), but the earlier assistant proposal (harbor-0010) makes the word initial potentially ambiguous. This guided development rubric should not be treated as an independent semantic judgment.", "",
              "The next controlled protocol should verify tool use and structured finalization before comparing memory quality, retain this failed pilot, and use new held-out histories after development. It must not silently replace these trials with favorable reruns."]
    lines += ["", "## Failures and incorrect fields", ""]
    failures = [r for r in report['results'] if not r['passed']]
    if not failures:
        lines.append("All requested exact-value artifacts passed; this does not assess unscored assertions or semantic citation support.")
    for row in failures:
        reason = row.get('error') or ', '.join(k for k, passed in row.get('checks', {}).items() if not passed)
        lines.append(f"- {row['task_id']} / {LABELS[row['condition']]} / repeat {row['repeat']}: {reason}.")
    lines += ["", "## Accounting and limits", "",
              "Foreground tokens cover all recorded calls, including failed trials. Estimated/unresolved usage is a conservative reservation, not a provider invoice. Outer-process failures may lack usage; no cost saving can be inferred from missing usage.", ""]
    for condition, m in metrics.items():
        lines.append(f"- {LABELS[condition]}: {m['estimated_trials']} estimated, {m['unresolved_trials']} unresolved, {m['missing_usage_trials']} missing-usage trials.")
    lines += ["", "No new compilation or embedding calls occurred. Preparing the existing checkpoint previously used 87,159 input + 13,443 output generation tokens and 240,044 estimated embedding tokens; that historical total excludes original planning and earlier failed attempts. Both file-memory conditions share this existing pipeline expense. The full-history baseline has no compilation expense. Monetary cost per successful task is unreported because no price schedule or complete historical bill is available.", "",
              "All tasks come from one known two-session history, with explicit field prompts and two repeats. Differences are descriptive and correlated; no significance or generalization claim is made. Exact fields are scored independently of citation IDs. A correct value with an unsupported citation can still pass; semantic support requires a separate assessment. Keyword file search is used, not live API/semantic retrieval. Latency includes process startup, file verification, tools and provider time; network variation and provider cache effects remain. Reproduction requires the same snapshot and configuration; hosted weights/routing can change.", "",
              "## Reproduce and audit", "",
              "- [Complete trial results and traces](results.json)", "- [Machine-readable summary](summary.json)",
              "- [Frozen protocol and commands](../../evaluation/harbor/README.md)",
              "- [Questions and gold fields](../../evaluation/harbor/suite.json)",
              "- [Original evidence ledger](../../evaluation/harbor/evidence-ledger.json)",
              "- [Prior compilation and known wiki defect](../2026-09-08-harbor/README.md)", "",
              f"Private input report SHA-256: `{hashlib.sha256(raw).hexdigest()}`. Opaque provider resume handles in message text are redacted in the shared trace; no task, gold value, memory page, artifact or score was changed after observing these trials."]
    (out / "README.md").write_text("\n".join(lines) + "\n")
    import matplotlib
    matplotlib.use("Agg")
    import matplotlib.pyplot as plt
    fig, axes = plt.subplots(1, 3, figsize=(13, 4.8), layout="constrained")
    colors = ["#9b9b93", "#517b8c", "#237e80"]
    labels = ["Full\nhistory", "Notes +\nsessions", "Full\nTrace2Mem"]
    series = [("Task success", [100*m['successes']/m['attempts'] for m in metrics.values()], "%"),
              ("Foreground tokens", [(m['input_tokens']+m['output_tokens'])/1000 for m in metrics.values()], "k"),
              ("Median task latency", [m['median_latency_seconds'] for m in metrics.values()], " s")]
    for ax, (title, values, suffix) in zip(axes, series):
        bars = ax.bar(labels, values, color=colors, width=.62)
        ax.set_title(title, fontsize=12, pad=16)
        ax.spines[['top', 'right']].set_visible(False)
        ax.bar_label(bars, labels=[f"{v:.1f}{suffix}" for v in values], padding=4)
        ax.set_ylim(0, max(max(values)*1.25, 115 if suffix == '%' else 1))
        ax.tick_params(axis='x', length=0)
    fig.suptitle("Harbor pilot • 6 tasks × 2 repeats per condition", fontsize=15)
    fig.text(.5, -.025, "One synthetic history; frozen older wiki; foreground tokens exclude compilation.\nLower tokens/latency reflect failed work, not demonstrated savings.", ha='center', fontsize=9)
    fig.savefig(out / 'comparison.png', dpi=160, bbox_inches='tight')
    plt.close(fig)
    return metrics


if __name__ == '__main__':
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--input', required=True)
    parser.add_argument('--output', required=True)
    args = parser.parse_args()
    print(json.dumps(render(args.input, args.output), indent=2))
