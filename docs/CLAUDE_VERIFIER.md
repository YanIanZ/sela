# Sela verifier: Claude Sonnet 5.5 (EvidenceGuard → v3 integration)

Status:
- **Role accepted** (PM decision D-029). The contract is in [ADR-0003](adr/ADR-0003-ai-routing-verifier-fallback.md) §6–§7, and the stored verification status in §10.1.
- **Runtime integration planned** (milestone P5). No verifier route, consent purpose or persistence exists yet.
- **What exists** is [prototypes/evidenceguard](../prototypes/evidenceguard/README.md), **implemented as a standalone v2 prototype**. Its 6 mocked tests pass locally (`go test -count=1 ./...` in `prototypes/evidenceguard`, 2026-10-26) and in its own CI workflow (`.github/workflows/evidenceguard-prototype.yml`, run 40284245346, 2026-10-26). It is not wired into any runtime, and it is not v3 evidence.
- **No live Claude request** has been made: no Console organization or API key exists (**blocked externally**, B-8). There is no clinical validation.

Governed by [SPEC](../SPEC.md) C17 (partly supersedes C12), I.ai, I.verifier, V17, V18, V33, V34 and V39 (V34 and V39 together supersede V16), and by ADR-0003. The prototype is the **starting point** of the integration (§3).

> Reconciliation note (D-024): the implemented generator is the Meta open-weight **Muse Glimmer 30B** on Modal ([ADR-0001](adr/ADR-0001-meta-open-weight-model-on-modal.md)). Older Muse Spark Open references await the owner's decision ([B-7](BLOCKERS.md)). This contract applies unchanged to whichever Meta model generates the draft, and to DeepSeek when it is the fallback generator.

## 1. Facts (observed 2026-10-26)

**As of 26 October 2026**, Anthropic officially lists Claude Sonnet 5.5 (released 28 September 2026) with API model ID **claude-sonnet-5-5**, at $2 per million input tokens and $10 per million output tokens. Structured outputs support `output_config.format` with JSON Schema. The model rejects `temperature`, `top_p` and `top_k`. See https://platform.claude.com/docs/en/models/sonnet-5-5/overview and https://platform.claude.com/docs/en/build-with-claude/structured-outputs.

The model and the API:
- `claude-sonnet-5-5` is a pinned snapshot with no dated variant. Retirement is not sooner than 2027-09-28.
- Endpoint: `POST https://api.anthropic.com/v1/messages`.
- Headers:
  - `Authorization: Bearer <key>` is primary (`x-api-key` is legacy).
  - `anthropic-version` is required. The latest is `2026-10-21`; `2023-06-01` is still listed.
  - `content-type: application/json`.
  - `anthropic-workspace-id` is needed only for a key that is not scoped to a workspace.
- Request constraints:
  - `thinking:{type:"disabled"}`, forced `tool_choice` and assistant prefill return 400.
  - The lowest thinking setting is `between_tools`, at effort ≤ high.

Structured output:
- `output_config.format` json_schema is GA and grammar-constrained.
- The schema needs `additionalProperties:false`. String enums are allowed. Length bounds and `minItems` > 1 are not.
- **The guarantee breaks on `stop_reason:"refusal"` (HTTP 200, billed) and on `max_tokens` truncation.**
- **Enum casing is not guaranteed.**
- It cannot be combined with Citations.
- A field that asks for "reasoning" can trigger a `reasoning_extraction` refusal.

Data handling:
- API data is deleted within 30 days by default. Flagged content is kept up to 2 years and classifier scores up to 7 years.
- Zero data retention (ZDR) is a per-organization contract, not a switch, and none is in place.
- Schemas are cached 24 h, so they must hold no personal data.
- Storage is US only. `inference_geo` is `global` (default) or `us` (×1.1 price).
- Indonesia is a supported region.

Usage policy:
- Every consumer chatbot must disclose AI use.
- Products serving minors must follow Anthropic's minors guidance.
- The 2025-09-15 Usage Policy (in force until 2026-11-12) treats mental-health advice to individuals as high-risk.
- The version effective 2026-11-12 excludes internal summarization or analysis that is not the delivered recommendation.

Access:
- A Claude Code, Pro or Max subscription is **not** application API access. The runtime needs a Console organization and an API key (B-8).

Sources and the remaining facts (rate limits, Batch, caching) are in ADR-0003 §2 and §16.

## 2. Product decision: a second check, never a truth oracle

- **Meta generates; Claude checks.** Meta (Muse Glimmer 30B on Modal) generates every draft, and DeepSeek may generate it only as the fallback (ADR-0003 RR-4). The Go application owns all version rules and data transitions.
- **What Claude Sonnet 5.5 checks.** Whether each proposed claim preserves the supplied, versioned source spans: the user's own words, or for reading content the licensed source passage. It looks for changed negation, speaker, time, uncertainty or important context, and for the eight checks in ADR-0003 §7: consistency, unsupported claims, distortion or lost context, citation relevance and validity, inappropriate medical language, manipulation, age appropriateness and information leakage. It can also raise a missed safety signal.
- **When verification is required (D-029).** Before release of:
  - consultation-summary wording;
  - memory proposals;
  - any generated text that the deterministic health-claim detector flags.
  
  Reading content is verified at authoring time. Each user still decides by the separate `ai_provider_anthropic` consent. Without it, or when no permitted verifier is available, those outputs **abstain**: the user's own words are kept and the manual path is offered.
- **What Claude cannot do.** It cannot:
  - generate replies, summaries or alternative free-form clinical advice;
  - make clinical risk or urgency determinations;
  - select providers or replace a failed Meta request;
  - approve a share;
  - issue a diagnostic conclusion or create a safety certification.
- **Verdicts, not rewrites.** The v1 OpenRouter "Second Brain" debiaser rewrote drafts. It is not migrated (D-037); its purpose is merged here as structured verdicts. Claude never silently rewrites.
- **Agreement between two LLMs is not independent validation.** The hard constraints are explicit source mapping, deterministic rule checks and user review. Claude receives the source spans, not only the generator's answer.
- **A citation identifier or DOI alone is not factual proof** of a study's finding. Research claims are checked only against verified, licensed abstract or text passages paired with source metadata, and they still need human critical appraisal (B-4). The 7,580-publication metadata candidate dataset is discovery material, not a verified knowledge base.

## 3. EvidenceGuard prototype: what to keep and what to change

`prototypes/evidenceguard/verifier.go` is implemented as a prototype (standard-library Go, no SDK).

| Aspect | Prototype today | v3 contract (ADR-0003) | Change |
| --- | --- | --- | --- |
| Verdicts | `supported \| contradicted \| insufficient_evidence`. The prompt says "Otherwise choose insufficient_evidence", which merges additions into "insufficient" | `supported \| unsupported \| contradicted \| insufficient_evidence` (VC-3) | **Add the missing `unsupported` verdict** and separate "adds something" from "cannot tell" |
| Issues | None | `issues[]` with 7 categories, including `safety_signal` (VC-2) | Add |
| Enum matching | Case-sensitive. A casing change fails closed | Case-insensitive (casing not guaranteed) | Change |
| Request | `claude-sonnet-5-5`, `Authorization: Bearer`, `thinking: between_tools`, `effort: low`, json_schema, `max_tokens` 1000, no workspace header | The same, plus `max_tokens` 2000 for ≤ 12 claims and `inference_geo: "us"`, with a workspace-scoped key (VC-4) | Keep the call shape; adjust |
| API version | `anthropic-version: 2023-06-01` (accepted) | `2026-10-21` from configuration | Change |
| Input gate | `Synthetic=true` and a `VerificationConsent` boolean in the input. Limits: 12 claims, 16 evidence spans, quote ≤ 1,600, claim ≤ 800, ≤ 5 evidence per claim, ids ≤ 48 | Server-built input from owned current records. The consent revision is read from the consent store and rechecked at commit. Span ids are per request (VC-1) | Replace the synthetic gate (D-025); keep the limits |
| Output checks | Response `model` equals the pin, `stop_reason == "end_turn"`, exactly one text block, ≤ 64 KiB, unknown fields rejected, exact claim set, evidence ids within the claim's own, reason non-empty and ≤ 500 | The same, plus span ids ⊆ supplied spans and reason ≤ 300 characters (VC-6) | Keep and extend |
| Refusal and truncation | Rejected as `ErrInvalidResponse` because `stop_reason` is not `end_turn` (fails closed). **No test covers either case** | `verification_unavailable` with the reason `refused` or `truncated`. No alternative verifier is tried (RR-10) | Distinguish the reasons; add tests |
| HTTP failures | `ErrUnavailable` with the status only; the provider body never leaks (`TestApiFailureDoesNotLeakProviderBody`); 12 s client timeout; no retry | Availability failure: 1 bounded retry, then the alternative verifier only if RR-9 holds, else abstain. Per-task attempt timeouts of 15–30 s | Extend |
| Result | `review_status` is `model_supported` or `needs_human_review` | Per-claim release rules (VC-8), the stored status and provenance (ADR-0003 §10.1) and the UI states derived from them (§5) | Change |
| Provenance | `provider`, `model`, fixture and brief version | ADR-0003 §10: served model, API version, route, latency, tokens, cost | Extend |
| Wiring | None: no route, consent purpose, persistence or cancellation | P5 router, `ai_provider_anthropic` consent, encrypted records, cancellation on withdrawal or version change | Build |

Existing tests: `TestSupportedAndCorrectRequest`, `TestContradictedClaimNeedsHumanReview`, `TestRefuseUnauthorizedInputsBeforeApi`, `TestRejectMalformedOrForgedOutput` (unknown claim, unknown evidence, wrong status, missing claim, extra field, garbage), `TestApiFailureDoesNotLeakProviderBody`, `TestCanceledContext`.

## 4. Pipeline (v3; planned)

1. **Crisis screen.** The deterministic crisis screen runs on every user text before any model. A match makes **zero** calls to any provider and shows verified help at once.
2. **Generation.** Meta generates (or DeepSeek as fallback) under its own consent. Meaning Lock, the output guards and the health-claim detector run first. Only claims that survive them are sent for verification.
3. **Preflight.** `ai_provider_anthropic` must be granted on the current policy version, the age band must allow the provider (ADR-0003 §8.3), and limits and spend ceilings must allow the call.
4. **Input.** Go builds a bounded claim set from the owned current records: ≤ 12 claims and ≤ 16 spans per call, and at most 3 calls for one summary. Contact details and private names are redacted, and ids are per request. Only the needed spans and claims are sent, server to server.
5. **Call.** Claude is called as in ADR-0003 VC-4. Server-side `fallbacks` are never enabled.
6. **Validation.** The response is validated strictly (VC-6). Any violation rejects the whole report.
7. **Release.** The release rules apply (VC-8): only `supported` claims without issues keep AI wording; everything else reverts to the user's own words or is dropped. Before storing, Sela rechecks the entry or summary version and **every consent used**. Withdrawal, an edit or account deletion discards pending output (RR-13).
8. **Claude unavailable.** The alternative verifier (DeepSeek) may check only if Claude had an availability failure, the generator was Meta, the user granted DeepSeek consent and the feature flag is on (RR-9). Otherwise the output abstains.
9. **Labels.** The UI distinguishes "written with AI (provider)", "checked by Claude" or "checked by the alternative verifier", and "confirmed by you".
10. **User control.** Only the user can approve the exact version and decide what to share. A verdict cannot trigger sharing.

## 5. User-visible states

Planned. These are **UI states**, derived for display and never stored. What is stored is the verification `status`, the V33 verification provenance, the per-claim verdicts and issues (ADR-0003 §10 and §10.1), plus the consent and version records the state is checked against.

| UI state | Shown when (stored values or live state) | Meaning (ID) | Meaning (EN) |
| --- | --- | --- | --- |
| `not_required` | `status` `not_required` (provenance `not_required`) | Teks ini tidak perlu diperiksa ulang | This text did not need a second check |
| `awaiting_consent` | Before a verifier-gated task starts, `ai_provider_anthropic` is not granted on the current policy version (RR-2). No AI output is generated, and the manual path is offered beside it | Perlu izinmu yang terpisah untuk pemeriksaan oleh Claude (Anthropic, AS) | Your separate permission is needed for a check by Claude (Anthropic, US) |
| `checking` | A verifier call is in flight (operation state; nothing stored yet) | Sedang diperiksa terhadap kata-katamu | Being checked against your own words |
| `checked` | `status` `checked`, provenance `live_anthropic`, and every claim `supported` with no issue | Sudah dicek terhadap kata-katamu oleh Claude; **bukan** pemeriksaan medis | Checked against your own words by Claude; **not** a medical check |
| `checked_alternative` | `status` `checked`, provenance `live_deepseek_alternative`, and every claim `supported` with no issue | Dicek oleh pemeriksa alternatif (DeepSeek) karena Claude tidak tersedia; **bukan** pemeriksaan medis | Checked by the alternative verifier (DeepSeek) because Claude was unavailable; **not** a medical check |
| `needs_your_check` | `status` `checked` (either provenance) and at least one claim not `supported` or carrying an issue, so its AI wording was replaced by the user's words or dropped (VC-8). The label still names the verifier | Ada bagian yang tidak bisa dipastikan; kalimat aslimu dipakai, silakan tinjau | Some parts could not be confirmed; your original sentence is used, please review |
| `verification_unavailable` | `status` `verification_unavailable` or `abstained` (provenance `unavailable`). The reason shown comes from `unavailable_reason`, for example "tanpa izin / no consent", "layanan tidak tersedia / service unavailable" (RR-11) | Pemeriksaan tidak tersedia; Sela memakai kata-katamu sendiri | The check is unavailable; Sela uses your own words |
| `stale_or_revoked` | Derived at read time: the record's content version or a consent revision it used no longer matches the current one (RR-13, V18) | Hasil tidak berlaku karena isi atau izin berubah | The result no longer applies because the content or permission changed |

- `status` `not_requested` (provenance `not_requested`) has no verification state: the item has no new AI wording and carries its own provenance label instead (D-039).
- Provenance `manual_review` is reserved (ADR-0003 §10.1) and has no UI state yet.
- A labelled test-double verifier report (development, test and the demo, D-045; ADR-0003 §10.1) uses the same states, with "TEST DOUBLE — not Claude" (or "not DeepSeek") in place of the verifier's name. It is never shown as a check by Claude or DeepSeek.
- When an output is withheld entirely, the task's own no-AI behaviour applies (ADR-0003 §3): a listening reply shows that Sela cannot reply right now, a reflection card is not shown, opening lines use the labelled template lines, and a summary uses the user's words with the `verification_unavailable` notice. A `safety_signal` issue shows the inline help card (VC-8).

Per-claim explanations are Sela's own reviewed text, shown with the highlighted source span:

| Verdict | ID | EN |
| --- | --- | --- |
| `unsupported` | "Kalimat ini menambahkan hal yang tidak kamu tulis." | "This sentence adds something you did not write." |
| `contradicted` | "Kalimat ini berbeda dari yang kamu tulis." | "This sentence differs from what you wrote." |
| `insufficient_evidence` | "Sela tidak bisa memastikan kalimat ini dari tulisanmu." | "Sela could not confirm this sentence from your writing." |

The verifier's own short reason appears only under "Detail pemeriksaan / Check details", labelled as an automated check. It is never presented as advice. **Do not** use a bright green "medically safe" badge; use neutral text and source visibility.

## 6. API (planned; to be specified in [API_SPEC_V3](API_SPEC_V3.md) §9 before implementation)

- **Consent:** `PUT /api/v3/me/consents/ai_provider_anthropic` (purpose name reserved in API_SPEC_V3 §3). It is independent of `ai_processing`, `ai_provider_deepseek`, storage and sharing consents.
- **When verification runs:** inside the owning feature's operation (consultation summary, memory), using server-derived spans and claims only. There are no raw free-text verifier inputs from the browser.
- **Results:** read through those features and the reserved `/api/v3/ai` prefix (provenance records, migration range 0500–0599). Access is owner-scoped only.

Illustrative report shape (not an executable response; API_SPEC_V3 §9 will set the field names). `verificationStatus` and `verification` are stored values (ADR-0003 §10.1, V33). `uiState` is a UI state derived for display (§5) and is not stored.

    {
      "draftOrigin": "live_meta",
      "route": "verifier",
      "verifierProvider": "anthropic",
      "verifierModel": "claude-sonnet-5-5",
      "anthropicVersion": "2026-10-21",
      "verificationStatus": "checked",
      "verification": "live_anthropic",
      "uiState": "needs_your_check",
      "claims": [
        {"claimId": "c1", "verdict": "contradicted", "spanIds": ["s1"], "reason": "The source says the person has not told anyone."},
        {"claimId": "c2", "verdict": "unsupported", "spanIds": ["s2"], "reason": "The source does not mention sleep."}
      ],
      "issues": []
    }

## 7. Data handling

- **Real use, minimised (D-025).** Only the spans and claims a check needs leave the Go backend, with contact details and private names redacted. No account, user, session or database ids are sent, and no `metadata.user_id`. This replaces the v2 "synthetic excerpts only" rule.
- **Disclosure before consent:** processed by Anthropic in the US, deleted within 30 days by default, flagged content kept up to 2 years, no zero-retention agreement in place (ADR-0003 §8.1). [Anthropic data retention](https://platform.claude.com/docs/en/manage-claude/api-and-data-retention): zero-data-retention eligibility is **not** the same as having a ZDR arrangement enabled.
- **No personal data** in JSON schema names or enums.
- **The Batch API** (not ZDR) is used only for reading-content checks, which hold no personal data.
- **Logs and storage.** Operational logs never contain spans, claims, verdict reasons or model responses (V12). Verdict reasons are stored encrypted per user (ADR-0002 §5).
- **Minors.** For the `15_17` and `unknown` bands (enabled only outside production today, D-030), the verifier also checks age appropriateness. Production use needs the minors-guidance safeguards confirmed (B-10).

## 8. Fail-closed and adversarial behaviour

- **Unknown or forged span ids** are rejected deterministically before Claude is called.
- **Generator distortion.** A generator hallucination or negation inversion may be caught by Meaning Lock first. Claude may flag it too, but even a Claude `supported` cannot bypass deterministic rules, source checks, user review or approval.
- **Malformed reports.** Extra, missing or duplicate claims or references, an invalid verdict, unknown fields or an unauthorized tool request reject the entire report (`verification_unavailable`, `invalid_output`).
- **Refusals and truncation.** `stop_reason:"refusal"` or `max_tokens` truncation → `verification_unavailable`. The alternative verifier is never tried after a refusal or truncation.
- **Availability failures.** Timeout, 429 (including the spend cap), 5xx or a missing key → `verification_unavailable`, unless the alternative verifier is permitted (RR-9). Never silently skip or claim success.
- **Changed state.** If the user withdraws any consent, edits the content, the version changes, or the account is deleted, the stale response and any earlier report for that version are discarded.
- **Crisis.** **Zero** calls to every provider.
- **Embedded instructions.** Source text that contains jailbreak-like instructions is untrusted data. Model-suggested URLs, calls or scripts are never executed.
- **Testing.** Indonesian, English, code-switched text and negations are tested separately, with mock provider reports and independently labelled synthetic truth sets (ADR-0003 §12):
  - Compare the baseline (Meta + Go validators) with Meta + Go + Claude.
  - Count false acceptances, false rejections, residual unsupported statements, refusals, latency and real cost.
  - Do **not** report agreement as accuracy.

## 9. Engineering budget and gating

The prototype uses Go standard-library HTTP and JSON with no SDK dependency, a fixed Anthropic production endpoint, a request timeout, strict input limits, an output size cap and mock-network tests. It does **not** persist or authorize anything; the P5 integration must supply atomic state checks.

Estimated cost (ADR-0003 §11, US inference):
- about $0.006 for a flagged reply;
- $0.009 for a memory-proposal batch;
- $0.014 per summary call of ≤ 12 claims.

Pre-registered limits are `cost.verifier_per_call_usd_mean ≤ 0.02` and `latency.verifier_p95_ms ≤ 20000` (live). Sela-side caps are 40 checks per user per day and a daily spend ceiling, plus the Console monthly spend limit.

Call Claude only for claims that survive deterministic checks. The source-grounding verifier is not a replacement for academic critical appraisal.

## 10. Hackathon positioning

The [Meta hackathon](https://dev.meta.ai/events/global-hackathon) prioritizes Meta models.
- Show **the Meta model's actual generative contribution and its provenance** independently of the verifier.
- Keep a runnable Meta-only profile (D-049). In it, summary and memory outputs abstain, because no verifier is permitted.
- The Official Rules neither allow nor forbid non-Meta models. **Ask the organizers** before relying on Anthropic in the competitive demo (B-12).
- Label every check with its provider, and never count Claude calls as use of Meta tooling.

## 11. Implementation evidence

- [Go prototype](../prototypes/evidenceguard/verifier.go) and [mock tests](../prototypes/evidenceguard/verifier_test.go): implemented as a standalone v2 prototype. Its 6 tests pass offline (`go test -count=1 ./...` in `prototypes/evidenceguard`, 2026-10-26), and GitHub Actions runs the same tests (`.github/workflows/evidenceguard-prototype.yml`, run 40284245346 passed, 2026-10-26). Not wired into any runtime; not v3 evidence.
- Not done:
  - no Anthropic API key or access check;
  - no measurement of the refusal rate on Sela's synthetic mental-health excerpts;
  - no quota or cost measurement;
  - no deployed Sela endpoint (B-8).

## 12. History: superseded statements

| Earlier statement | Superseded by | Now |
| --- | --- | --- |
| "Claude Sonnet 5.5 is **optional**" (second-pass review) | D-029 | Required before release for summary wording, memory proposals and flagged text. Without the user's consent or a permitted verifier, those outputs abstain |
| "**Muse Spark Open on Modal remains Sela's primary generative AI**" | D-024 (reconciliation), B-7 | The implemented generator is Muse Glimmer 30B; the owner decides B-7 |
| "Scenario selection: synthetic adult + chosen ID/EN language; reject arbitrary user histories in hackathon mode"; "Only synthetic fixture excerpts" | D-025, D-030 | Real use with minimised data. Synthetic data is for tests, evaluation and demo. Age bands replace the adult-only rule |
| Status enum `supported/contradicted/insufficient_evidence`; aggregate `model_supported` / `needs_human_review` | ADR-0003 VC-2, VC-8; SPEC V34 | Four verdicts including `unsupported`, an issue list, per-claim release rules, the stored statuses of ADR-0003 §10.1 and the UI states in §5 |
| "Timeout, 429, 5xx … mark verification unavailable; do not … switch providers" | D-029 (partly) | Still never after a refusal, truncation or invalid output. After an availability failure, the alternative verifier is allowed under RR-9 only, and labelled |
| Planned `POST /api/v2/verification-consent`, `POST /api/v2/brief/verify`, `GET /api/v2/brief/verification` | API_SPEC_V3 consent purposes; `/api/v3/ai` | §6 |
| UI states "Claude is reviewing the **fictional** note", "synthetic case / simulated care" labels | D-025 | Demo-environment banners stay (ADR-0002 §12); real-use copy in §5 |
