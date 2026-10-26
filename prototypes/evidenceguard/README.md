# Sela EvidenceGuard — Go prototype

**Status:** standalone Go verifier prototype; **not** integrated with the current Next.js v1, planned Go v2 runtime or live frontend. Tests use mock HTTP, not paid inference. No Anthropic API key has been provided or used.

- Meta Muse Spark Open: generates source-linked draft statements.
- Sela Go domain validators: verify IDs, evidence spans, session, consent and versions.
- Claude Sonnet 5.5: optional **second-pass model review**, never a generator fallback, judge of clinical safety, or approver of information sharing.
- User: independently reviews and approves the exact version, with explicit sharing consent.

Read [the governing verifier contract](../../docs/CLAUDE_VERIFIER.md).

## Test (no API spend)

Run from this directory using Go 1.23 or newer:

    go test ./...

The model ID is **claude-sonnet-5-5**. API access uses the official Anthropic Messages endpoint and a server-side bearer key. For production, configure ANTHROPIC_API_KEY in your secret manager or environment; never place it in a repo, browser bundle, public CI log or the inherited v1 environment template.

## Future v2 integration

The package exports New(key) and Verify(ctx, Input), but cannot yet be used by any runnable Sela v2 route because the Go runtime has not been built. Build and test the first-party API layer and its deterministic crisis/consent gates before connecting it. The Input requires an explicit Anthropic verification consent, synthetic scenario marker, fixture/brief versions, limited source quotes and claims with valid evidence IDs.

The model output is revalidated against the exact claim/evidence identifiers. A review_status of model_supported means **Claude judged the submitted text consistent with cited synthetic quotes**, not independently confirmed clinical or scientific truth. Source licenses and the research evidence quality still matter.

Store results only after rechecking the session generation, consent and exact brief version after inference. Expired/revoked/edited sessions discard in-flight responses. When Claude fails, surface verification_unavailable and an explicit manual-review option without a hidden provider substitution or fabricated verification badge.

Only synthetic fixture excerpts are allowed during this hackathon. No real user journals, personal identifiers, patient narratives, full copyrighted papers, API secrets or provider output logs are permitted. Verify hackathon rules for use of a non-Meta secondary model before treating this as part of the competitive submission.

## References

- https://platform.claude.com/docs/en/models/sonnet-5-5/overview
- https://platform.claude.com/docs/en/build-with-claude/structured-outputs
- https://platform.claude.com/docs/en/manage-claude/api-and-data-retention
- https://platform.claude.com/docs/en/api/overview
