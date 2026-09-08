# Development selection — frozen before held-out inference

The rule selected `controlled_v1` after all 96 development trials completed. The compiled memories, both candidate suites, model settings and Go executable were frozen before this comparison. The consuming executable was built with Go 1.26.6 from `5f09938`; its SHA is in [freeze.json](freeze.json). The suite freeze was committed in `11d20d1`.

| Candidate | Full history | Notes + sessions | Full wiki | Combined memory tasks |
|---|---:|---:|---:|---:|
| Optional tools | 15/16 | 5/16 | 9/16 | 14/32 |
| Controlled actions | 13/16 | 10/16 | 11/16 | 21/32 |

The combined memory score increased by 21.9 percentage points. The selection rule maximizes combined memory-condition successes, then minimizes execution errors, then penalizes uncertain usage before comparing tokens. It does not select the lowest-cost candidate. Recorded memory-condition tokens increased from 155,730 to 525,496; the latter includes three unresolved-usage trials, so no precise cost ratio or savings is claimed. Full-history performance regressed, including two provider failures and one field-scoring failure.

## Root-cause observations

- The earlier Harbor pilot frequently skipped memory tools. In this new optional development run, notes/session trials had no successful file read in 8/16 cases; wiki trials in 6/16. Controlled actions reduced each to 1/16.
- Optional Nadia preferences in notes/session mode returned the literal field type `string` for every requested value without reading memory. This was an agent action failure, not absent source evidence.
- Optional Nadia current-date in notes/session mode found the correct date, but prefixed its JSON with prose. The scorer rejected the artifact. Five optional notes/session trials and three wiki trials failed the JSON contract. Such failures do not imply every stated fact was wrong.
- Controlled actions require a read before answering in file-memory conditions and complete reads of cited evidence. They also require a structured answer action. These changes are bundled; this comparison does not isolate their individual causal contribution.
- Controlled failures remain: notes/sessions had one step-budget and five provider failures; wiki had two step-budget and one provider failure. Citation-shape repair loops are visible in the failed traces. The new protocol improves successful completion but adds work and does not solve provider reliability.
- The first Nadia compilation put prose into an evidence-ID array. The production verifier schema now enumerates known IDs and directs explanation to `reason`. Its successful retry retained the same publication checks. Both compilation attempts and usage are retained; no held-out answers informed that repair.

## Frozen next step

Run only the selected protocol on Leena and Owen: 16 questions × three conditions × two repeats = 96 held-out trials. Do not tune on these answers. Full-history prompting remains a strong baseline because all 18 events fit in context. Compare full wiki with notes/sessions separately to assess the wiki's additional contribution. This is a small synthetic persona pilot with shared templates, not broad generalization.

[selection.json](selection.json) pins the exact development report hashes. The [optional RCA](optional-rca.json) and [controlled RCA](controlled-rca.json) retain condition/category metrics and uncertainty counts. This decision records development evidence only; it is not a held-out result or an automatic promotion of a service-wide agent policy.
