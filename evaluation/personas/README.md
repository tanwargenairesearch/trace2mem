# User-history memory evaluation

Four wholly synthetic users, six conversations and 18 events per user; eight questions per user. Nadia and Marco are development histories; Leena and Owen are held out from model/prompt tuning. Their schemas share task families but entities, dates, preferences, owners and domains differ. This is a small authored pilot, not a recreation of Brain's proprietary benchmark or broad independent distribution.

Histories include document preferences, two visits introduced in separate conversations, an authoritative date correction, a historical assistant recap, an unapproved suggestion, reaffirmed preferences, unknown lodging and irrelevant discussion. Every expected answer field has a gold source set in cases.json. These sets identify alternative supporting sources; for preference reaffirmations, the original statement supplies the values. Dates and exact entities use deterministic checks; explanations copy a source clause. This measures constrained factual answering, not free-form semantic quality.

Compilation receives history.jsonl only, never questions, expected values or gold sources. Distilled notes and wiki pages must be created by the real Trace2Mem Dream pipeline. No hand-authored wiki is supplied. One isolated memory is provisioned for each persona in a disposable database; the normal application is untouched.

## Pre-registered development loop

1. Freeze histories, question wording, gold fields and source sets before inference.
2. Compile each history with the configured real Kimi/Vertex profile. Retain failed compilation diagnostics and all recorded usage. Do not weaken semantic validation to obtain a publishable wiki.
3. Run the existing optional-tool agent (candidate A) on the 16 development questions under all three conditions, one repeat: 48 trials.
4. RCA: classify failed trials into no retrieval, failed tool execution, invalid final answer, budget/provider failure, incorrect currentness/authority, and unsupported evidence. Candidate B changes the action protocol to require a structured action and successful memory read before answering in memory conditions. The full-history baseline uses the same final-answer protocol.
5. Run candidate B on the same development matrix: 48 trials. This is development, not held-out evidence. Selection rule: maximize combined notes/wiki exact-task successes; break ties using fewer execution errors, then lower accounted foreground tokens. Keep full-history results alongside both.
6. Freeze the chosen protocol, then run the 16 held-out questions under all three conditions with two repeats: 96 trials. Do not tune on these answers. Total planned foreground trials: 192. Report failed compilations, failed trials and budget-limited runs explicitly.

Budget: 64,000 accounted foreground tokens and 150 seconds per trial, 10 model rounds, single provider attempt per call. Corpus compilation uses the explicit existing model YAML budget. These are token limits, not a dollar estimate; no price schedule is supplied. No real-user data, cloud deployment or public posting is involved.

## Metrics

Report exact task success and per-field correctness, broken down by family and split. Also report actual source reads, gold source coverage and gold-citation agreement; none is an independent semantic support judgment. Full-history evidence exposure is not equivalent to a tool read, so retrieval recall is N/A for that baseline. Show foreground tokens/latency and compilation usage separately. Include all attempts and avoid claiming savings from incomplete work.

## Work tracking

- [x] Author reproducible histories and source-linked question/answer keys.
- [x] Dataset/protocol validation and three-lens review: source alternatives corrected, cumulative input bounded, and failed compilation jobs terminalized with fencing. Twelve Python checks and the database failure-isolation race test passed.
- [ ] Real-model compilation of isolated user memories.
- [ ] Development baseline, RCA, candidate comparison and frozen selection.
- [ ] Held-out evaluation, charts and reproducible report.

## Compilation RCA recorded before evaluation

The first Nadia compilation failed with `verification cites unavailable source`. Its saved verdict placed a sentence about a duplicate citation in the `conflicts` array, whose entries must be evidence IDs. The schema previously described arrays of strings without constraining their values. The correction enumerates supplied evidence IDs and explicitly sends prose to `reason`; publication validation is unchanged. The failed attempt and usage are retained, and any retry is identified separately. This is a protocol repair learned on development history, not held-out answer tuning.

The controlled-agent review added explicit rejection of tool-free completions and contiguous paginated source-read accounting shared with the optional agent. Merely seeing a citation or skipping the first source range does not count as a complete source read.
