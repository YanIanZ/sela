# Frozen inputs of frozen corpus snapshots

A corpus snapshot (`data/research/corpus/snapshots/<name>/`) is immutable and
must rebuild byte for byte. Its manifest records the SHA-256 of every input.
When one of those inputs is edited after the freeze (for example a dated
"superseded" note added to doc 10 on 2026-10-26), the exact pre-edit bytes are
kept here at the same repository-relative path:

    frozen/<snapshot name>/<repository path of the input>.frozen

`python3 scripts/research/corpus_pipeline.py reproduce --snapshot <name>` uses
the live file when its bytes still match the manifest and the frozen copy
otherwise; `verify` reports a violation when neither matches. These copies are
our own register text, kept only for reproducibility (the `.frozen` suffix
keeps them out of documentation tooling and `-text` in
`data/research/.gitattributes` keeps their bytes exact); the live documents
are the ones to read.

The 1.2.0 snapshot (`2026-10-26_pipeline-1.2.0`) also hashes
`data/research/register_inputs/reviewer_decisions.json`. Before the first
reviewer entry is added, copy the file's current bytes to
`frozen/2026-10-26_pipeline-1.2.0/data/research/register_inputs/reviewer_decisions.json.frozen`
(and likewise for any register document edited after that snapshot), then
build the reviewed corpus as a new snapshot version.

| Snapshot | Frozen file | Why |
| --- | --- | --- |
| `2026-10-26` (pipeline 1.1.0) | `docs/research/sela-relay/07_EVIDENCE_TO_FEATURE.md.frozen` | dated note with the R-9 claim-check results added on 2026-10-26 |
| `2026-10-26` (pipeline 1.1.0) | `docs/research/sela-relay/10_EVIDENCE_APPRAISAL_AND_CLAIMS.md.frozen` | dated superseded notes added on 2026-10-26 (research audit P2) |
| `2026-10-26_pipeline-1.2.0` | `docs/research/sela-relay/10_EVIDENCE_APPRAISAL_AND_CLAIMS.md.frozen` | decision IDs in the dated v3 note renumbered on 2026-10-26 after main was merged (owner brief v3 D-025 → D-026, minors D-030 → D-031) |
