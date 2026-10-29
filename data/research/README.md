# Sela Research — Europe PMC candidate metadata

This folder contains a **reproducible, unreviewed bibliographic candidate pool**, obtained on 26 October 2026 from the Europe PMC REST API. It is **not** a clinical dataset, systematic review, patient dataset, or a dataset licensed for model training.

## Data

- `sela_research_candidates_7580.jsonl` — 7,580 deduplicated candidate records, one JSON object per line (UTF-8).
- `sela_research_candidates_manifest.json` — creation date, source runs, SHA-256 checksum, sample bias, themes, year distribution and integrity counts.
- The existing, independently selected 100-paper evidence register stays in [docs/research/sela-relay/research_100.json](../../docs/research/sela-relay/research_100.json) and is **not** merged into these unreviewed candidates.

**Record fields:** `title`, `authors`, `doi`, `pmid`, `first_publication_date`, `journal`, `publication_type`, `themes`, `europepmc_url`, `screening_status`, `source_batches`. No abstracts, PDFs, article full texts, or private patient data.

## Preliminary title-only screening (not inclusion or evidence quality)

Current version: `sela-title-screen-v1.1` (2026-10-26).

- [Complete scoring index](sela_screening_index.csv): machine priority for all 7,580 metadata candidates, plus publication-type eligibility and its basis.
- [400-item next-review queue](sela_priority_review_queue.jsonl): eligible high/medium title matches with reasons; no abstract/full-text screening yet.
- [Human appraisal worksheet](sela_human_review_worksheet.csv): blank decision columns. `rag_ready=NO` unless separately verified by a reviewer.
- [Screening manifest](sela_screening_manifest.json): reproducible totals, family distributions, SHA-256 and exact rule version.
- [Screening protocol](../../docs/research/sela-relay/14_METADATA_SCREENING_PROTOCOL.md): methodological constraints, biases, and human-review steps (with a dated note on v1.1).

What changed in v1.1 (no priority changed):

- **Line endings.** v1.0 wrote CSV with CRLF line endings and hashed those bytes, while git stores LF, so the committed files never matched their manifest. v1.1 writes LF and hashes the exact committed bytes; [.gitattributes](.gitattributes) keeps them LF on checkout.
- **Publication type.** Protocols, corrections/errata, editorials and retraction notices (412 rows) leave the queue by publication type, with explicit title cues as a fallback. 11 of the old 400 queue rows were replaced.
- **History.** v1.0 still reproduces byte for byte with `--ruleset sela-title-screen-v1.0`; its manifest is frozen in [screening_history/](screening_history/).

Regenerate reproducibly:

```sh
python3 -m unittest discover -s tests/research -p 'test_*.py'
python3 scripts/research/screen_candidates.py --priority-limit 400
```

## Other research data in this folder

| Path | What it is |
| --- | --- |
| [corpus/](corpus/) | The 100-record register rebuilt by the corpus pipeline (current pipeline 1.2.0; frozen 1.1.0 snapshot in `corpus/snapshots/2026-10-26/`), plus the v3 evidence map and the doc 07/10 claim check. See [CORPUS_PIPELINE.md](../../docs/research/CORPUS_PIPELINE.md) and [EVIDENCE_MAP_V3.md](../../docs/research/EVIDENCE_MAP_V3.md). |
| [v3_gap_search/2026-10-26_search-1.2/](v3_gap_search/2026-10-26_search-1.2/) | Current frozen v3 gap search (run v1.2, screening v1.2): 29,508 metadata-only candidates (Europe PMC + Crossref, publication language recorded), with every hit of the D-048 gap queries and 1,000 rows of each Indonesian-language Crossref query; machine title/abstract screening, a dual-screening worksheet, and a **blinded** false-negative audit sheet whose machine-label key is withheld from the repository until the audit is complete (only its SHA-256 is committed); all human columns blank. See [V3_GAP_SEARCH.md](../../docs/research/V3_GAP_SEARCH.md). |
| [v3_gap_search/2026-10-26_search-1.1/](v3_gap_search/2026-10-26_search-1.1/) | Run v1.1 (8,712 candidates, screening v1.1), frozen and superseded by run v1.2; kept so it rebuilds byte for byte. Its audit key was moved to the local private cache directory on 2026-10-26 (hash unchanged); do not use its sheet for the audit. |
| [v3_gap_search/2026-10-26/](v3_gap_search/2026-10-26/) | Run v1.0 of the v3 gap search (4,818 candidates, screening v1.0), frozen and superseded; kept so it rebuilds byte for byte. Its audit sample is not blinded; do not use it for the audit. |
| [register_inputs/reviewer_decisions.json](register_inputs/reviewer_decisions.json) | The only place where human reviewer decisions about register records are entered (schema `sela-register-review-v2`: human attestation, optional second reviewer and adjudication, appraisal tool required for full-text reviews; empty: nobody has reviewed a record yet). It is an input of the 1.2.0 snapshot: freeze its bytes before editing (see the file's rules). |
| [register_inputs/frozen/](register_inputs/frozen/) | Exact pre-edit bytes of register documents that frozen snapshots hashed, so the snapshots stay reproducible. |
| [screening_history/](screening_history/) | Frozen manifests of superseded screening versions. |

No AI model was used to rank these metadata candidates and none were automatically approved for RAG or fine-tuning.

## Source and reproduction

Metadata comes from these two historical GitHub Actions runs:

- [Recent 18-category harvest: run 40251550092](https://github.com/YanIanZ/sela/actions/runs/40251550092), artifact `sela-research-candidates`, 5,000 records.
- [Historical supplement: run 40251929177](https://github.com/YanIanZ/sela/actions/runs/40251929177), artifact `sela-research-historical-candidates`, 3,000 records.

A one-time publication workflow downloads the archived artifacts, runs `scripts/research/publish_candidates_from_artifacts.py`, validates exact counts and no overlap with the separate 100-paper register, and commits the JSONL plus manifest. The artifacts may expire; committed data remains independently usable.

Check integrity locally after cloning:

```sh
python3 - <<'PY'
import hashlib, json
from pathlib import Path
p = Path('data/research')
metadata = json.loads((p/'sela_research_candidates_manifest.json').read_text())
raw = (p/metadata['data_file']).read_bytes()
rows = [json.loads(x) for x in raw.splitlines()]
assert len(rows) == metadata['record_count'] == 7580
assert hashlib.sha256(raw).hexdigest() == metadata['data_sha256']
assert all(r['screening_status'] == 'candidate_metadata_only_unreviewed' for r in rows)
print('Validated:', len(rows), 'metadata-only candidates')
PY
```

## Interpretation and responsible use

These broad queries produce off-topic papers, favor 2025–2026, and contain literature that has **not** undergone individual eligibility screening, critical appraisal, DOI verification, independent de-duplication against all databases, or full-text review. Counts are not measures of clinical benefit, intervention efficacy, publication quality, population prevalence or evidence sufficiency. Use them as an **initial search pool** only.

For Sela RAG, first select and independently review relevant articles, verify DOI and licenses, document whether full text may be processed, and retain provenance and contradictory findings. Keep sensitive mental-health user data and copyrighted paper contents out of the public repository.

See [collection audit](../../docs/research/sela-relay/RESEARCH_COLLECTION_2026-10-26.md) and [research method](../../docs/research/sela-relay/06_RESEARCH_METHOD.md).
