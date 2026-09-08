# A Kimi agent that writes and reads memory

This opt-in exercise runs a real model-managed tool loop against a synthetic engineering project. It uses the configured generation model for both the task agent and Dream; the tested profile is OpenRouter Kimi K3 with Vertex Gemini embeddings. It is an integration example, not a general-purpose autonomous coding agent.

The fictional task is a webhook-platform migration. Two conversations cover architecture alternatives, throughput calculations, retention and storage, backpressure, ownership, a changed deadline, higher peak load, and launch gates. Six scripted user turns provide decisions and corrections; Kimi generates its own responses and selects tools. The calculator executes actual arithmetic against fictional supplied measurements. No infrastructure is deployed and no vendor prices are invented.

The runner captures user and assistant messages plus tool calls/results through the public ingestion API. Provider call IDs are mapped to stable capture IDs to satisfy the event contract; the transcript retains the original IDs. Models receive no shell or network-execution tools.

Dream compiles the original design conversation, then applies the correction conversation as a second incremental update. Each compilation drains its accepted watermark before continuing. After publication, the runner exports the final revision. A third Kimi conversation starts with a clean message history and these tools:

1. memory_index: orient using the compact index.
2. memory_search: discover relevant paths using bounded excerpts.
3. memory_read: inspect selected files from the pinned revision.
4. memory_evidence: resolve original source events.

These are agent-side tool wrappers backed by the Go HTTP/Connect client; this exercise does not test the MCP transport.

It asks for a current-state handoff and then an operational checklist. The fixture checks actual successful retrieval calls, resolved citations, and expected facts. Fact checks establish term presence; current-versus-historical interpretation still requires review. These checks are a narrow rubric, not proof of general memory quality or a wiki-on/off evaluation.

## Run locally

Requirements: Docker, an OpenRouter API key, an explicit model YAML, and host ADC authorized for the Vertex project in that YAML. Credentials must remain in environment variables or private files. For other providers, adapt the runner's credential injection; the Go test itself uses the existing provider-neutral model configuration.

Copy [the explicit Kimi exercise profile](../configs/models.kimi-agent.example.yaml) to .local/models.kimi-agent.yaml and set its Vertex project before running. It uses ADC for embeddings; do not commit credentials.

~~~sh
export OPENROUTER_API_KEY=... # use your local secret management
export TRACE2MEM_MODEL_CONFIG="$PWD/.local/models.kimi-agent.yaml"
export GOOGLE_APPLICATION_CREDENTIALS="$HOME/.config/gcloud/application_default_credentials.json"
export TRACE2MEM_REPORT_DIR="$PWD/.local/harbor-$(date +%s)"
bash scripts/kimi-agent.sh
~~~

This starts a separate database and local API inside a test container. It does not add fictional project decisions to your normal web-app user. Credential-bearing containers and the test database are removed on exit; the source image has neither ADC nor API keys. Outputs survive in the report directory, including partial trajectories on model or compilation failures.

For the richer replay, generation.reasoning_effort is none, generation.request_timeout_seconds is 180 and budgets.max_output_tokens is 8192, with budgets.max_tokens set to 96000. Reasoning effort is an optional Responses-adapter setting; supported values vary by model and must be probed. Omission leaves the provider's effort default unchanged. The successful compilation/recall replay requested none and the endpoint accepted it; original planning used the provider default. OpenRouter metadata reported default=max with low/high/max supported on 2026-09-08, so probe support rather than assuming every endpoint accepts none.

Omission retains the existing 90-second timeout and 4096-token Responses default (2048 for Ollama). Output limits are capped at 32768 and request timeouts at 300 seconds; caller cancellation and the five-minute compilation deadline still apply. Reservations include the configured output allowance. Incomplete Responses results are rejected before any partial tool call executes. See the [Responses reference](https://developers.openai.com/api/reference/cli/resources/responses/methods/create) for the protocol fields; these describe the wire contract, not a guarantee about another provider's behavior.

The agent has eight tool rounds per user turn, a 180,000-token stopping threshold checked between requests (one request may exceed it), and a 25-minute overall deadline. Dream uses the YAML's separate compilation and daily budgets. This exercise supplies trusted maintenance guidance to propose at most 6 concise observations (under 80 words each); full source events remain available. Agent usage is recorded separately from Dream usage; no dollar total is implied without provider pricing. Live calls can incur costs, including unsuccessful attempts.

For a compilation/retrieval retry using already-generated planning history:

~~~sh
export TRACE2MEM_AGENT_REPLAY="/absolute/path/to/previous/history.jsonl"
export TRACE2MEM_REPORT_DIR="/absolute/path/to/new-report"
bash scripts/kimi-agent.sh
~~~

Replay preserves the original event envelopes and skips the previous handoff session. It avoids paying for the planning conversation again. It still calls live models for Dream and the fresh handoff.

## Inspect the output

- history.jsonl: Protobuf JSON event envelopes, suitable for later import.
- transcript.md: conversations and actual tool invocations.
- agent-usage.json: generation usage for the task/recall agent.
- service-usage.json: separately accounted compilation/embedding/retrieval usage, including estimated reservations.
- manifest.json and memory/: published, pinned Markdown wiki, notes, summaries, and source evidence.
- handoff.md: the first fresh-conversation answer.
- proposals.json: stored staged proposals and verifier judgments, including rejections.
- revisions/: an exported snapshot after every publication, retained even if a later stage fails.

The final handoff conversation is captured after the exported revision and remains pending for a future compilation. The snapshot therefore represents the planning and correction conversations; it does not recursively ingest its own handoff.

For production adapters, use durable batching/retries such as the LangChain integration. This test intentionally sends each event synchronously and stops on ingestion failure. Its local vault key is a test fixture and must never be used in a deployed service.

## Recorded exercise

See the [Harbor report](../reports/2026-09-08-harbor/README.md) for actual outputs and a known historical interpretation error. Search is optional when the index already identifies relevant files; this run used index, read, and evidence tools. Audit the saved capture without model calls:

```sh
TRACE2MEM_AGENT_REPORT="$PWD/reports/2026-09-08-harbor" go test -v ./tests/live -run TestSavedHandoff -count=1
```
