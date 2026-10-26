# Sela Research Expansion (metadata candidates)

The curated 100-paper register at `docs/research/sela-relay/research_100.json` is the **reviewed-at-title/abstract research starting point**. The collector builds a **separate, unreviewed candidate pool**, not 5,000 validated or independently reviewed studies.

## Run

Python 3.10+; no third-party dependencies required.

```bash
python3 scripts/research/collect_europepmc.py --target 5000
python3 -m unittest discover -s tests/research -p 'test_*.py'
```

The command contacts the [Europe PMC REST API](https://europepmc.org/RestfulWebService) sequentially, using `resultType=lite`, `pageSize=100`, cursor pagination, 0.5-second pacing, retries, and a fixed first-publication window 2021-10-27 through 2026-10-26. It uses 18 relevant topic queries and skips DOIs in the original 100 records, deduplicating candidates by DOI, PMID and normalized title/year. Collection uses two passes: up to 200 accepted records per theme first, then backfills toward 5,000; this improves thematic coverage over stopping after the earliest seven themes. **It does not fix publication-year skew or screening relevance.** Records may be fewer than 5,000 when queries have insufficient unique hits or the service returns errors. Examine the generated manifest rather than asserting the target was reached.

Outputs (generated; not automatically committed):
- `data/research/sela_candidates.jsonl` — bibliographic metadata only, candidate flag, theme tags, stable source link
- `data/research/sela_candidates_manifest.json` — actual counts, query limits, errors and collection time

Do **not** commit publication abstracts, bulk PDF files or full text without checking each item’s license. Europe PMC's API allows retrieval, but publisher rights differ. For legally reusable full text, use the PMC Open Access subset or documented OAI-PMH/FTP routes and license-check every article. The collector does not fetch abstracts or full text.

## After collection

1. Exclude duplicates, protocols, corrections, off-topic and withdrawn papers; verify DOI metadata.
2. Review publication type, country/language, study design, limitations and overlap among systematic reviews.
3. Move selected high-quality studies into the curated evidence mapping only after human screening, with citation and evidence-grade recorded.
4. For Sela RAG, index authorized source material with license/provenance and chunk/page anchors, not raw unverified candidate pool.
5. Preserve the original 100 as stable IDs; new candidates must never silently become 5,000 clinical evidence claims.

This is not a systematic review or a substitute for clinical evaluation. Never use bibliographic frequency or model response as evidence of safety/efficacy.

## Optional older-year sampling

The general relevance search may overrepresent newly indexed papers. Run a separate sample excluding 2026 and inspect it **as a distinct unreviewed pool**, rather than calling the main run representative:

```bash
python3 scripts/research/collect_europepmc.py \
  --target 2000 \
  --to-date 2025-12-31 \
  --output data/research/sela_pre2026_candidates.jsonl \
  --manifest data/research/sela_pre2026_manifest.json
```

Maintain explicit per-year/per-theme counts and perform human quality appraisal before inclusion in evidence-backed product requirements.
