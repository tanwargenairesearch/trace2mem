# Trace2Mem documentation

Trace2Mem turns one user's agent experience into persistent, cited memory across conversations. Start with the repository [README](../README.md) for the problem and workflow.

## Use it with your agent

1. [Quickstart](QUICKSTART.md): start Docker, authenticate, configure models, import evidence, and check publication.
2. [Integration guide](INTEGRATION.md): capture events, retry safely, load an initial index, and expose retrieval tools in any harness.
3. [Pi](../integrations/pi/README.md) and [Hermes](../integrations/hermes/README.md): native capture and retrieval adapters.
4. [LangChain reference](../integrations/langchain/README.md): install the callback adapter with its durable local spool and run a two-conversation example.
5. [Kimi tool-agent example](KIMI_AGENT.md): generate a substantial trajectory and recall it in a fresh conversation. Requires explicit live-model credentials and budgets.

## Understand and operate it

| Document | What it answers |
|---|---|
| [Architecture](ARCHITECTURE.md) | How do ingestion, Dream, publication, models, and retrieval fit together? |
| [Brain alignment](BRAIN_ALIGNMENT.md) | Which principles are implemented, adapted, or deferred? |
| [Implementation status](IMPLEMENTATION.md) | What is implemented, tested, or still unverified? |
| [MVP review](reviews/2026-09-08-brain-review.md) | What concrete source and memory-quality gaps should be addressed next? |
| [Outcome measurement](OUTCOMES.md) | How should an agent integration demonstrate quality, currentness, and cost deltas? |
| [Evaluation](EVALUATION.md) | What did measured tests establish, and what did they not establish? |
| [Operations](OPERATIONS.md) | How do I deploy, back up, restore, and manage credentials? |
| [Contributing](../CONTRIBUTING.md) | Where should changes go and which checks should I run? |
| [Security](../SECURITY.md) | How should I report a vulnerability? |

The [Protobuf schema](../proto/trace2mem/v1/trace2mem.proto) is the API contract. [Delivery notes](DELIVERY.md) are the development/review record; they are not the onboarding guide. Cloud Terraform is optional. Local Docker validation does not establish cloud deployment or identity-provider interoperability.
