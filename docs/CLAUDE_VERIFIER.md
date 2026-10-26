# Sela EvidenceGuard: optional Claude Sonnet 5.5 verifier

**Status:** adopted product/contract direction; optional **Go prototype exists** in [prototypes/evidenceguard](../prototypes/evidenceguard/README.md), but no Sela v2 runtime, deployed verifier route, successful live Claude request, or demonstrated clinical validation yet. Governed by [SPEC](../SPEC.md), especially C12, I.verifier and V16–V18.

**As of 26 October 2026**, Anthropic officially lists Claude Sonnet 5.5 (released 28 September 2026), API model ID **claude-sonnet-5-5**, at $2 per million input tokens and $10 per million output tokens. Structured outputs support output_config.format with JSON Schema. The model rejects temperature, top_p and top_k. See https://platform.claude.com/docs/en/models/sonnet-5-5/overview and https://platform.claude.com/docs/en/build-with-claude/structured-outputs .

## Product decision: second-pass review, never an automatic truth oracle

- **Muse Spark Open on Modal remains Sela's primary generative AI**. Meta creates constrained, source-linked intake/brief drafts; the Go application owns all patient/fixture version rules and data transitions.
- A consented DeepSeek `deepseek-flash` may generate a fallback draft only after an eligible Meta outage, **not** as an Anthropic verifier replacement. See [fallback](DEEPSEEK_FALLBACK.md).
- **Claude Sonnet 5.5 is optional** and has a different job: audit whether each proposed claim preserves the supplied, versioned source quotation without changing negation, speaker, timeline, uncertainty or important context.
- Claude **cannot** generate alternate free-form clinical advice, make clinical risk/urgency determinations, select providers, replace a failed Meta request, approve handoff, issue a diagnostic conclusion or create a safety certification.
- **Agreement between two LLMs is not independent validation.** The hard constraints are explicit source mapping, deterministic rule checks, user review and—when clinically relevant—real licensed-professional oversight. Claude receives source quotes, not only Meta's answer.
- A literature citation identifier or DOI alone is **not** factual proof of a study's finding. For research claims, only allow verified, licensed abstracts/text passages paired with source metadata and human critical appraisal. The 7,580-publication metadata candidate dataset is discovery material, not a verified knowledge base.

## Go pipeline / sequence

1. Scenario selection: synthetic adult + chosen ID/EN language; reject arbitrary user histories in hackathon mode.
2. Deterministic crisis fixture gate **before any model**; interrupt with non-clinical support state. No Meta/Claude call on this path.
3. Separate, explicit **Meta processing consent**, then call Muse Spark Open and run hard local evidence/schema checks.
4. Explicit **Anthropic second-provider verification consent** with purpose, data fields and withdrawal controls. Declined = no Claude request; manual validation and review remain available.
5. Create a bounded claim set (max 12 claims, 16 evidence snippets, total constrained bytes) linked by IDs to a versioned synthetic fixture. Send minimum quoted evidence + model draft to Claude, server to server.
6. Use Anthropic Messages API POST /v1/messages with model `claude-sonnet-5-5`, bearer API key and `anthropic-version: 2023-06-01`. Request schema-constrained JSON via `output_config.format`, with `thinking: {type: "between_tools"}` and `output_config.effort: "low"` for a short text-only verifier call. This is Sonnet 5.5-specific; `thinking.type: "disabled"` and temperature/top_p/top_k return HTTP 400. Avoid disallowed sampling params.
7. Validate output: exact claim ID set, allowed evidence ID sets, no duplicate/unknown IDs, status enum supported/contradicted/insufficient_evidence, bounded explanation. This remains **model judgment**, not clinical verification.
8. Before storing, recheck session generation, current brief version and **both provider consents**; reset, expiry, edit and withdrawal invalidate pending output. Inferences are cancellable and concurrency-/rate-limited.
9. UI distinguishes Meta-generated, Claude-reviewed, and user-approved stages and always clearly labels **synthetic case / simulated care**. If Claude is unavailable, show an honest unverified/manual path with no fabricated approval label.
10. Only the user can approve the exact version and decide what to share with a selected fictional professional; a verifier result cannot trigger sharing.

## Truthfulness and user-visible states

| Status | Meaning (ID) | Meaning (EN) |
| --- | --- | --- |
| not_requested | Pemeriksaan Claude belum diminta | Claude review was not requested |
| awaiting_consent | Perlu izin terpisah untuk Claude | Separate Claude consent required |
| reviewing | Claude sedang meninjau catatan fiktif | Claude is reviewing the fictional note |
| model_supported | Claude menilai teks sesuai sumber yang diberikan; **bukan** verifikasi medis | Claude judged it consistent with supplied sources; **not** medical verification |
| needs_human_review | Ada kemungkinan kontradiksi atau sumber tidak cukup; tinjau kembali | Possible contradiction or insufficient evidence; review required |
| verification_unavailable | Pemeriksaan Claude tidak tersedia; lanjutkan dengan review manual | Claude review unavailable; continue with manual review |
| stale_or_revoked | Hasil tidak berlaku karena data/izin berubah | Review invalid due to changed data/consent |

**Do not** use a bright green "medically safe" badge; use neutral text and source visibility.

## Planned API change (not implemented)

- POST /api/v2/verification-consent with granted and expectedVersion; independent of Meta AI consent and handoff sharing consent.
- POST /api/v2/brief/verify with current expectedVersion; no raw free-text inputs: the server derives all quotes and claims from the owned fixture/current draft. Must pass both consents if Meta-generated.
- GET /api/v2/brief/verification reads current-session, current-version report only. No cross-session lookups or patient story logging.

Example report shape (illustrative, not executable response):

    {
      "draftOrigin": "live_meta",
      "verifierProvider": "anthropic",
      "verifierModel": "claude-sonnet-5-5",
      "briefVersion": 2,
      "reviewStatus": "needs_human_review",
      "findings": [
        {"claimId": "c1", "status": "contradicted", "evidenceIds": ["e1"], "reason": "The source says the person still attends class."}
      ]
    }

**Data constraints:** No real psychiatric histories, real names/contacts, full journal entries, unlicensed paper text, secrets or model responses in operational logs. A verifier only sees the synthetic evidence required for the operation, with its own explicit consent. Check [Anthropic data retention](https://platform.claude.com/docs/en/manage-claude/api-and-data-retention); zero-data-retention eligibility is **not** the same as having a ZDR arrangement enabled. Check data-handling requirements before handling genuine health data in any later product.

## Fail-closed and adversarial behaviors

- Unknown/forged evidence IDs → deterministic rejection before Claude.
- Meta hallucination/negation inversion → Claude may flag, but even Claude "supported" cannot bypass deterministic/source and user approval requirements.
- Claude returns extra/missing/duplicate claim or references / invalid status / unauthorized tool request → reject entire report.
- Timeout, 429, 5xx, malformed output, refusal or missing key → mark verification unavailable; do not silently skip, claim success or switch providers.
- User withdraws Meta or Claude consent / brief edited during inference / expiry / reset → discard stale response and any previous report.
- Crisis fixture → **zero** calls to both providers; no handoff or generated clinical decisions.
- Quoted source text contains jailbreak-like instructions → treat it as untrusted data; do not execute model-suggested URLs, calls or scripts.
- Indonesian, English, code-switching and negations tested separately, with mock provider reports and independent human-labeled truth sets. Compare baseline Meta+Go validators vs Meta+Go+Claude; count residual wrong claims, disagreements, latency, rejection/review rate and real cost. Do **not** report agreement as accuracy.

## Engineering budget and gating

Prototype is written using Go standard-library HTTP and JSON, no SDK dependency, with fixed Anthropic production endpoint, request timeout, strict input limits, output size cap and mock-network tests. It does **not** persist or authorize anything; future Go application must supply atomic state checks.

Suggested optimization: call Claude only for sources/claims that survive deterministic checks, and prioritize high-uncertainty or translated meaning changes; measure cost/latency first. Source-grounding verifier is not a replacement for academic critical appraisal.

## Hackathon positioning

The [Meta hackathon](https://dev.meta.ai/events/global-hackathon) prioritizes Meta models. Show **Muse Spark's actual generative contribution and its provenance** independently of the secondary Claude verifier. Keep a Meta-only runnable mode and evaluate the verifier as an *optional safety/correctness overlay*. Current public event information inspected did not establish definitive approval of non-Meta verifiers; **check the final Official Rules and ask the organizers** before relying on Anthropic in the competitive demo. Do not count Claude calls as use of Meta tooling.

## Implementation evidence

- [Go prototype](../prototypes/evidenceguard/verifier.go), [mock tests](../prototypes/evidenceguard/verifier_test.go)
- GitHub Actions Go prototype tests must pass before marking adapter prototype acceptance.
- No actual Anthropic API key/access check, quota/cost measurement or deployed Sela endpoint has been performed.
