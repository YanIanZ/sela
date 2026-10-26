# Sela Research Expansion (metadata candidates)

The curated 100-paper register at `docs/research/sela-relay/research_100.json` is the **reviewed-at-title/abstract research starting point**. The collector builds a **separate, unreviewed candidate pool**, not 5,000 validated or independently reviewed studies.

## Run

Python 3.10+; no third-party dependencies required.

```bash
python3 scripts/research/collect_europepmc.py --target 5000
python3 -m unittest discover -s tests/research -p 'test_*.py'
```

The command contacts the [Europe PMC REST API](https://europepmc.org/RestfulWebService) sequentially, using `resultType=lite`, `pageSize=100`, cursor pagination, 0.5-second pacing, retries, and a fixed first-publication window 2021-10-27 through 2026-10-26. It uses 18 relevant topic queries and skips DOIs in the original 100 records, deduplicating candidates by DOI, PMID and normalized title/year. Records may be fewer than 5,000 when queries have insufficient unique hits or the service returns errors. Examine the generated manifest rather than asserting the target was reached.

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
