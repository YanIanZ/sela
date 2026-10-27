# Sela Europe PMC collection audit — 2026-10-26

**Status: bulk bibliographic candidate discovery only.** Not a systematic review, clinical validation or full-text corpus.

## Completed GitHub Actions runs

- [Recent collection](https://github.com/YanIanZ/sela/actions/runs/40251550092): succeeded; 5,000 metadata candidates, 18 topical categories, 8,500 raw API records seen, no API request errors.
- [Historical collection](https://github.com/YanIanZ/sela/actions/runs/40251929177): succeeded; 3,000 metadata candidates from 2022–2025, 15 topical categories reached before target, 4,700 raw API records seen, no API request errors.
- Both workflows run from branch `feat/research-europepmc-candidate-harvester`, and upload JSONL+manifest as artifacts for 14 days. The original 100-study curated register remains unchanged.

## Offline merge audit

| Measure | Measured |
| --- | ---: |
| Records in two outputs | 8,000 |
| Duplicate records across outputs | 420 |
| Unique bibliographic candidates | **7,580** |
| Unique records with a DOI | 7,551 |
| Records dated 2022 | 3 |
| Records dated 2023 | 25 |
| Records dated 2024 | 185 |
| Records dated 2025 | 2,814 |
| Records dated 2026 | 4,553 |
| Source API request errors | 0 |

Combined candidate records have both source batch labels and topic tags. Merging reconciled DOI, PMID and title/year matches. The actual merged output and manifest were provided to the project owner as a downloaded ZIP; it is **not** silently committed to Git source control.

> **Superseded note (2026-10-26, research v3).** The sentence above is out of date. The merged pool was later committed openly, through the publication workflow: commit `21c3551` ("data(research): publish 7580 Europe PMC metadata candidates") added [data/research/sela_research_candidates_7580.jsonl](../../../data/research/sela_research_candidates_7580.jsonl) and its manifest (SHA-256 checked), and commit `89ac3cf` added the title screening. Both are metadata only (no abstracts). The pool has no publication-language field; all 7,580 titles are English, because the search used English-language Europe PMC MEDLINE queries only. The v3 gap search (2026-10-26) adds Crossref and records the language: [V3_GAP_SEARCH.md](../V3_GAP_SEARCH.md).

## Important validity limitations

1. The search strategy intentionally spans broad phrases and may return off-topic papers: every item still needs title/abstract eligibility screening.
2. The API's result order strongly favors recent publications; **2026 and 2025 dominate** despite the historical supplementary retrieval. Neither theme tags nor counts estimate how much evidence exists or how strong it is.
3. A record with a DOI is not a full-text-reviewed paper, an independent trial, an authoritative clinical guideline or evidence that Sela works.
4. Bibliographic metadata alone is collected; abstracts, copyrighted PDFs and full-text articles are not exported for republication. Any later text mining or training must respect each source license.
5. Review status remains `candidate_metadata_only_unreviewed`. Keep this separate from the original 100 annotated studies.

## Recommended next research tasks

- Apply transparent inclusion/exclusion rules and manual full-text screening on a prioritized, balanced sample; include contradictory and null findings.
- Stratify the next sampling run by calendar year and clinical design to correct this collection's strong newest-publication bias.
- Identify systematic reviews that overlap primary trials; keep intervention and participant outcomes separate from the count of publications.
- Build RAG only from source materials with adequate reuse permission, traceable citations and explicit certainty/transfer limits.
- Do not present the 7,580-count as an efficacy or clinical safety claim during judging.

See [pipeline operations](RESEARCH_EXPANSION_PIPELINE.md), [research methods](06_RESEARCH_METHOD.md) and [existing 100-study register](09_RESEARCH_REGISTER.md).
