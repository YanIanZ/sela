# Verifier evaluation inputs (ADR-0003 EV-0 seed set)

Status: **inputs only. No evaluation has been run on this set.** It was written and registered on 2026-10-26, before any verifier, alternative-verifier or fallback run, together with the thresholds version `sela-eval-thresholds-2026.10.26-2` in [`eval/thresholds.json`](../thresholds.json).

## Provenance

- **Synthetic.** Every span and claim was written by hand for this file. There is no real user text, no text from the v1/v2 corpora and no personal data. Names, places and numbers are invented; contact-like strings are fake.
- **Author and labeller.** Written and labelled by the AI engineer of the Sela v3 workflow (an AI agent acting in that role), 2026-10-26. One labeller only: there is no second independent labelling yet, so no Cohen's κ exists.
- **Not clinical evidence.** The labeller is not a clinician. The gold labels are engineering judgements of *fidelity* (does the claim follow from the cited spans?) under the verdict definitions of ADR-0003 VC-3 and the issue categories of VC-2. They say nothing about clinical validity, risk assessment or the quality of care (SPEC C10, V15). Agreement between a model and these labels is not clinical evidence either.
- **Personas are fictional** and stand for age and register groups only (see below). They are not based on any person.

## Files

| File | Content |
| --- | --- |
| `claims.jsonl` | 168 records, one claim per line, UTF-8 |
| `README.md` | this file |

`claims.jsonl` SHA-256 at registration: `e05a809031432f27ba9d40b86e0b6383ba157c1545a9c3ab068c26c7c934a5ab`. The labels are frozen with that hash. Any change to the file needs a new thresholds version that names the new hash, a dated rationale, and the old results kept.

## Record format

| Field | Meaning |
| --- | --- |
| `id` | `vc-0001` ... |
| `split` | `test` (157) or `dev` (11). Prompts may be tuned on `dev` only. |
| `language` | `id`, `en` or `mixed` (code-switched) |
| `persona`, `persona_group`, `age_band`, `audience` | Fictional persona; group `adult`, `15_17`, `50_plus` or `none` (AT-6 reading content); the ADR-0002 band; the audience class sent to the verifier (`adult` / `young_person`) |
| `task` | The ADR-0003 task name (`summary_draft`, `memory_proposal`, `listening_reply`, `reflection`, `opening_lines`, `reading_verification`) |
| `spans` | The source spans with per-request ids, as a VC-1 request would carry them (redaction placeholders such as `[NAMA-1]` appear where the minimiser would put them) |
| `claim` | One VC-1 claim (`id`, `text`, `span_ids`) |
| `gold_verdict` | `supported`, `unsupported`, `contradicted` or `insufficient_evidence` |
| `gold_issues` | Expected issues: `category` (VC-2 enum) and `scope` (`claim`, or `output` for `claim_id: ""`) |
| `checks` | The CK-n checks the item exercises (derived from the verdict and the issues) |
| `perturbation` | How the claim departs from the spans (`none`, `negation`, `speaker`, `time`, `frequency`, `certainty`, `quantity`, `condition_dropped`, `added_cause`, `added_number`, `added_diagnosis`, `added_name`, `added_contact`, `added_research`, `too_short`, `ambiguous_referent`, ...) |
| `adversarial` | `null` or the attack: `prompt_injection_span`, `prompt_injection_claim`, `fake_json_in_span`, `reasoning_bait`, `homoglyph`, `unicode_control`, `verdict_bait`, `double_negation`, `code_switch`, `sarcasm`, `quoted_speech`, `third_party_risk`, `emoji`, `informal_spelling`, `contact_in_span` |
| `note` | One line on why the label is what it is |

To build a VC-1 request from a record: `{"task": task, "language": language == "mixed" ? "id" : language, "audience": audience, "spans": spans, "claims": [claim]}`. Records may also be grouped by persona and task into requests of at most 12 claims and 16 spans.

## Coverage

| Verdict | Claims | | Issue category | Claims | | Check | Items |
| --- | --- | --- | --- | --- | --- | --- | --- |
| supported | 55 | | lost_context | 10 | | CK-1 consistency | 168 |
| unsupported | 44 | | citation_invalid | 6 | | CK-2 unsupported | 44 |
| contradicted | 45 | | medical_language | 14 | | CK-3 distortion | 45 |
| insufficient_evidence | 24 | | manipulation | 7 | | CK-4 citation | 6 |
| | | | age_inappropriate | 4 | | CK-5 medical | 14 |
| | | | information_leak | 6 | | CK-6 manipulation | 7 |
| | | | safety_signal | 3 | | CK-7 age | 4 |
| | | | | | | CK-8 leakage | 6 |
| | | | | | | CK-9 safety signal | 3 |

- Languages: Indonesian 89, English 49, code-switched 30.
- Persona groups: adult 86, 15–17 33, 50+ 37, reading content 12.
- Tasks: summary_draft 104, listening_reply 19, memory_proposal 15, reading_verification 12, reflection 9, opening_lines 9.
- Adversarial items: 18.

## Personas (fictional)

| Id | Group | Description |
| --- | --- | --- |
| P-A1 | adult | 24, final-year student, informal Indonesian |
| P-A2 | adult | 31, hospital nurse on night shifts, English |
| P-A3 | adult | 28, freelance designer, Indonesian-English code-switching |
| P-A4 | adult | 40, office worker and parent of two, formal Indonesian |
| P-T1 | 15_17 | 16, high-school student, informal Indonesian (audience `young_person`) |
| P-T2 | 15_17 | 17, vocational-school intern, mixed Indonesian and English (audience `young_person`) |
| P-S1 | 50_plus | 56, recently retired teacher, formal Indonesian |
| P-S2 | 50_plus | 63, carer for a spouse after a stroke, English |
| P-R0 | none | AT-6 reading content: synthetic source passages written for this file, not real studies |

## Known limits (must be closed before EV-3 counts as final)

- **Size.** 168 claims; ADR-0003 §12 EV-0 asks for a test split of at least 400 (≥ 120 supported, ≥ 100 unsupported, ≥ 100 contradicted, ≥ 50 insufficient_evidence), ≥ 80 issue cases and ≥ 40 prompt-injection items. This file is the seed set: 43 issue-bearing items and 4 prompt-injection items (2 in spans, 1 in a claim, 1 fake JSON report in a span).
- **One labeller, no κ.** A second team member must label independently, Cohen's κ must be reported, and a third person adjudicates disagreements, then the labels are hashed again and frozen.
- **The boundary between `unsupported` and `insufficient_evidence`** is a judgement call in the ambiguous-referent items; the notes say which reading was taken.
- **Reading passages are synthetic**, not licensed corpus records; CK-4 against real R-ids needs the P6 corpus (B-4).
- **No live run.** Every metric that needs a live provider is marked live-only in the thresholds version and stays `not_measured` until B-1 (Meta), B-8 (Anthropic) and B-9 (DeepSeek) are resolved.
