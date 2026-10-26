# Sela v2 — automated metadata-screening protocol

**Date:** 2026-10-26. **Status:** discovery and human-review queue, NOT evidence validation. [SPEC.md](../../../SPEC.md) remains the normative product source of truth.

## Research objective

Select a manageable, explainable queue of relevant literature candidates from the already committed **7,580 Europe PMC metadata records**, without representing the output as a systematic review or as proof that Sela has clinical effectiveness.

### Candidate universe and source integrity

- Source: [data/research/sela_research_candidates_7580.jsonl](../../../data/research/sela_research_candidates_7580.jsonl).
- Source manifest: [data/research/sela_research_candidates_manifest.json](../../../data/research/sela_research_candidates_manifest.json).
- Exactly 7,580 bibliographic records, publication years 2022–2026; strong recency bias.
- Separate [100-title/abstract-annotated register](research_100.json) is retained, not automatically promoted or combined with this screening.
- **Only article titles** (and identifiers/years for review-queue management) are used to assign priority. Publisher abstracts, patient stories, copyrighted PDFs and article full texts are **not ingested**.
- Earlier broad search tags are retained in the original rows but deliberately do not increase screening scores; they are too noisy for an inclusion decision.

## Machine triage (reproducible, lexicon-based)

Script: [scripts/research/screen_candidates.py](../../../scripts/research/screen_candidates.py). Version: `sela-title-screen-v1.0`.

> **Superseded note (2026-10-26, research v3).** The committed outputs are now `sela-title-screen-v1.1`, a new version under criterion 6. The title triage, scores and priorities are unchanged (803 high, 559 medium, 6,218 deferred). Two things changed:
> - **Line endings (audit P1).** v1.0 hashed CSV bytes with CRLF line endings, while git stores LF, so the committed index and worksheet never matched their manifest. v1.1 writes LF and hashes exactly the committed bytes; `data/research/.gitattributes` keeps them LF on checkout.
> - **Publication type (audit P5).** Protocols, corrections/errata, editorials and retraction notices are now excluded from the queue by publication type, with explicit title cues as a fallback. This affects 412 of the 7,580 rows; each keeps its priority and the basis is recorded in the index. 11 of the old 400 queue rows (3 corrections, 8 protocols) were replaced by the next eligible rows.
>
> v1.0 still reproduces byte for byte with `--ruleset sela-title-screen-v1.0`; its manifest is frozen in `data/research/screening_history/`. Criterion 5 is still open: a deterministic, **blinded** false-negative audit sheet (audit id, title and DOI only; blank human columns; the machine labels are in a separate key joined after the audit; it also samples rows excluded by publication type) is prepared in `data/research/v3_gap_search/2026-10-26_search-1.1/screening/`, and nobody has audited it yet. The earlier sample in `data/research/v3_gap_search/2026-10-26/screening/` showed the machine labels next to the auditor columns and is superseded. The v3 title/abstract screening is a separate instrument ([V3_GAP_SEARCH.md](../V3_GAP_SEARCH.md)).
>
> **Dated note (2026-10-26, review round 3).** The current audit sheet is `data/research/v3_gap_search/2026-10-26_search-1.2/screening/fn_audit_sheet.csv` (run v1.2). Its machine-label key is no longer committed next to it: only the key's SHA-256 is in the screening manifest, the key stays in a local private directory until every row is audited, and `v3_gap_search.py release-audit-key` adds it afterwards. The v1.1 sheet is superseded, and its key was moved out of the repository too. Criterion 5 is still open: nobody has audited either sheet.

The rule engine examines title phrases in ten explainable dimensions: mental health; care navigation / help-seeking / referral / continuity; AI and language models; source fidelity and summarization; privacy/consent; model safety and bias; multilingual/cultural equity; Indonesia/ASEAN; digital health; engagement/usability. Combinations such as `mental_health + care_pathway` and `mental_health + ai_models` receive extra prioritization. Obvious off-domain clinical fields receive a penalty if no mental-health/care-pathway signal is present. Study-design clues (randomized, systematic review, meta-analysis, protocol, qualitative) are **only title flags**, not quality evidence.

Triage labels:

| Priority label | Interpretation | Human action |
| --- | --- | --- |
| `high` | Title suggests a direct match for Sela's mental-health navigation, consent or AI use | Read abstract and confirm subject/population; then assess full text |
| `medium` | Relevant context or transferable methodological caution, not necessarily mental-health-specific | Manually inspect before including in evidence map |
| `deferred` | Weak/no match in title or off-domain result; **not an automatic scientific exclusion** | Keep retrievable; sample false negatives and revisit when needed |

No title-only score is a quality-of-evidence grade. Reviews may overlap primary research; RCT labels require inspection of methods and comparator; real-world clinical safety is out of scope.

## Generated artifacts

Run after checking out the Sela repository:

```sh
python3 -m unittest discover -s tests/research -p 'test_*.py'
python3 scripts/research/screen_candidates.py \
  --input data/research/sela_research_candidates_7580.jsonl \
  --output data/research \
  --priority-limit 400
```

- `data/research/sela_screening_index.csv`: **all 7,580 candidates** with priority, score, title signals, potential study-type clues and audit reasons.
- `data/research/sela_priority_review_queue.jsonl`: up to 400 high/medium candidates with full original bibliographic metadata and triage descriptors. Year caps (45% each for 2025 and 2026 when feasible) reduce, but do not remove, recency dominance.
- `data/research/sela_screening_manifest.json`: counts per family, priority and year; file hashes; what was actually screened.
- `data/research/sela_human_review_worksheet.csv`: the same review queue with 400 unfilled human appraisal decisions; every study starts at `NOT_REVIEWED`, and `rag_ready=NO` until a reviewer explicitly establishes suitability and reuse rights.

Generated data is committed only after the script and tests pass and hash/count assertions succeed. Existing raw metadata and curated 100-register remain unchanged.

## Required human review before RAG

Reviewer should open the DOI/publisher landing page and article abstract, confirm date and status (including retractions/corrections), identify study design and sample, assess relevance to Sela's **non-clinical** use case, record comparator/outcomes/counterevidence/conflicts/limitations, and check reuse license.

For full-text appraisal of priority studies, use an appropriate design-specific risk-of-bias framework (e.g. RoB 2, ROBINS-I or AMSTAR 2 where applicable), record *independent review status*, and de-duplicate studies that appear across multiple reviews. Do not claim full-text screening from an automated score. Balance Indonesia/ASEAN applicability, multilingual evidence, null findings and negative outcomes.

### Downstream RAG readiness gate

Only human-selected, verified, appropriately licensed source material should feed the content index. Retrieval must retain paper ID, DOI, title, source fragment, publication date, licensing basis, review state and uncertainty. Default to **metadata/link retrieval** when content reuse rights are unknown. Never train on papers merely because DOI/abstract is publicly accessible. Restrict Meta inference to synthetic cases, with Go verifying user-controlled evidence, consent and clinical non-diagnosis boundaries.

## Acceptance criteria

1. All 7,580 input rows receive **exactly one** machine-priority label.
2. Source SHA-256 is recorded and original metadata remains immutable.
3. Priority shortlist includes at most 400 records and no deferred rows.
4. Exact rule version and trigger signals are traceable for every score.
5. A random sample of high, medium and deferred rows receives manual false-positive / false-negative audit before calling the shortlist "relevant".
6. Any change to vocabulary, scoring or publication period requires a new version and regenerated checksums; no silent shifts.

## Known limitations

Title-based lexicons miss paraphrases and Indonesian-local terms and may over-prioritize generic AI or stress papers. Search tags were broad and the original sampling was skewed to 2026. No human screening is claimed by this automation. No claim of impact, evidence grade, RAG licensing or safety efficacy should be made until separate review and testing.
