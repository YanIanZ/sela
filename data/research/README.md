# Sela Research — Europe PMC candidate metadata

This folder contains a **reproducible, unreviewed bibliographic candidate pool**, obtained on 26 October 2026 from the Europe PMC REST API. It is **not** a clinical dataset, systematic review, patient dataset, or a dataset licensed for model training.

## Data

- `sela_research_candidates_7580.jsonl` — 7,580 deduplicated candidate records, one JSON object per line (UTF-8).
- `sela_research_candidates_manifest.json` — creation date, source runs, SHA-256 checksum, sample bias, themes, year distribution and integrity counts.
- The existing, independently selected 100-paper evidence register stays in [docs/research/sela-relay/research_100.json](../../docs/research/sela-relay/research_100.json) and is **not** merged into these unreviewed candidates.

**Record fields:** `title`, `authors`, `doi`, `pmid`, `first_publication_date`, `journal`, `publication_type`, `themes`, `europepmc_url`, `screening_status`, `source_batches`. No abstracts, PDFs, article full texts, or private patient data.

## Preliminary title-only screening (not inclusion or evidence quality)

- [Complete scoring index](sela_screening_index.csv): machine priority for all 7,580 metadata candidates.
- [400-item next-review queue](sela_priority_review_queue.jsonl): high/medium title matches with reasons; no abstract/full-text screening yet.
- [Human appraisal worksheet](sela_human_review_worksheet.csv): blank decision columns. `rag_ready=NO` unless separately verified by a reviewer.
- [Screening manifest](sela_screening_manifest.json): reproducible totals, family distributions, SHA-256 and exact rule version.
- [Screening protocol](../../docs/research/sela-relay/14_METADATA_SCREENING_PROTOCOL.md): methodological constraints, biases, and human-review steps.

Regenerate reproducibly:

```sh
python3 -m unittest discover -s tests/research -p 'test_*.py'
python3 scripts/research/screen_candidates.py --priority-limit 400
```

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
