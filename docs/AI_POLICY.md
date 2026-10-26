# Sela AI policy

Status:
- The **v3 routing policy is planned** (milestone P5). It is defined in [ADR-0003](adr/ADR-0003-ai-routing-verifier-fallback.md), following PM decisions D-029 and D-048.
- The **v2 pipeline is implemented** in `internal/journey` (pipeline, prompts) and `internal/ledger` (validation). Live model behaviour is unverified ([B-1](BLOCKERS.md)).
- The Claude verifier and the DeepSeek fallback have no runtime code yet, and their live access is blocked externally (B-8, B-9).

Authority: owner brief v3 §8; [SPEC](../SPEC.md) C4, C17 (partly supersedes C12), I.ai, V3, V4, V7, V8, V12, V15, V17, V18, V20, V33 (supersedes V9 and V10), V34 and V39 (together supersede V16), V40. The superseded IDs keep their v2 text in SPEC. The v1 multi-provider orchestration is [historical](archive/v1/AI_POLICY.md). Statements that v3 superseded are kept in the [history](#history-superseded-statements) at the end.

## Roles (PM decision D-029)

| Role | Provider and identifier | Used when | Status |
| --- | --- | --- | --- |
| Generator (primary) | Meta Muse Glimmer 30B on Modal, `meta-models/Muse-Glimmer-30B` @ `cec94fc…` ([ADR-0001](adr/ADR-0001-meta-open-weight-model-on-modal.md); B-7 open) | Every generative task | Adapter implemented and tested against the labelled test double; not deployed (B-1) |
| Fallback generator | DeepSeek, API model `deepseek-flash`, currently served by `DeepSeek-V4.1-Flash-1017` | Only when Meta is unavailable before any content arrived, and only with the separate DeepSeek consent (ADR-0003 RR-4). Never after a refusal, a safety block, a Meaning Lock rejection or any other content-based failure | Planned; flag off by default; B-9 |
| Verifier | Anthropic Claude Sonnet 5.5, `claude-sonnet-5-5` | Before release of consultation-summary wording, memory proposals and any text the health-claim detector flags. Reading content is verified at authoring time (ADR-0003 RR-8) | Standalone prototype only ([EvidenceGuard](CLAUDE_VERIFIER.md)); B-8 |
| Alternative verifier | DeepSeek | Only when Claude is unavailable, the generator was Meta (never a model's own output), the user consented, and the feature flag is on after evaluation (RR-9) | Planned; off by default |
| Rules, safety, consent | Go | Safety routing, consent, approval, sharing, release decisions, provenance | Implemented for v2 |

A Claude Code subscription is a development tool, not application API access. The runtime verifier needs its own Console organization and API key (B-8).

## What a model may do

| Task | Model work | Verification before release |
| --- | --- | --- |
| AT-1 Listening reply | A contextual reply in the user's language | Only when flagged |
| AT-2 Reflection card "Sela menangkap…" | Statements quoting the user's words, editable, confirmable and removable by the user | Only when flagged |
| AT-3 Memory proposal | Proposed items from saved entries the user allowed for memory | Always |
| AT-4 Consultation-summary draft | Wording for the items the user selected; at most one clarification field from Sela's fixed list (the question text is Sela's own) | Always |
| AT-5 Opening lines | Two or three lines drawn only from approved summary items | Only when flagged |
| AT-6 Reading content | No generation. A human author writes; Claude checks claims against licensed source passages; a human reviewer signs off | Always (authoring time) |

The v2 equivalents are implemented. The Meta model proposes Story Ledger claims, where each claim quotes one story sentence verbatim and has a kind, a short wording in the interface language and qualifier flags. It also picks at most one clarification field and drafts opening lines from **approved** brief items only.

A model may not decide safety routing, consent, approval, sharing, recipients, slots, matching, plans or any transition. It never sees other users' data, the professional catalogue, account data or the research corpus. The exception is the licensed passages sent to the verifier in AT-6. No model output can grant a permission or trigger sharing.

### Listening style (PM decision D-034)

Prompts and deterministic output guards together enforce these rules:
- Replies are contextual, with no mechanical empathy templates, no judgement and no diagnosis.
- No professional persona, no exclusive relationship and no dependency cues.
- At most one question per reply.
- Suggestions are offered, never pushed, and seeking a professional is never required.

Hard guards reject a reply outright: a diagnosis addressed to the user, a medication or treatment directive, a professional persona, exclusivity, dependency or pressure phrases. A flagged health statement goes to the verifier first (ADR-0003 RR-8). When no reply can be released, the user is told so honestly; no template line is substituted.

## Pipeline

**v2 (implemented):**
1. Session/version checks and the deterministic safety gate (on submission).
2. Explicit AI consent, then the per-session limit and global admission.
3. A bounded streamed request with a JSON Schema constraint and fixed prompts (`journey.PromptVersion`) that mark the user's text as data.
4. Strict parse, then Meaning Lock.
5. Recheck of the story, ledger and consent versions and the crisis state.
6. Commit as **unreviewed** items with provenance.

Cancellation, timeout, withdrawal or staleness discards the result.

**v3 (planned, ADR-0003 §5):**
1. Deterministic crisis screen of every user text before storage and before any call. A crisis match makes no model call to any provider and shows verified help at once (D-035).
2. A preflight check of each provider's consent, the age band, limits and kill switches. A verifier-gated task does not start unless a permitted verifier route exists.
3. Meta generates.
4. On an availability failure before content (cold start beyond budget, 5xx, timeout, capacity, our own auth failure), DeepSeek may generate if RR-4 holds; otherwise the honest no-AI path follows. At most one switch per operation.
5. The same deterministic guards apply to every generator: Meaning Lock, the output guards, the health-claim detector, and the generator's risk flag, which can only raise the route.
6. Claude verifies where required. If Claude is unavailable, the alternative verifier may check (RR-9). A refusal, truncation or invalid output means `verification_unavailable`, with no alternative.
7. If no permitted verifier is available, verifier-gated outputs abstain: the user's own words are kept, the manual path is offered, and the UI says why.
8. The commit re-check runs again (V18, V20).

Meaning Lock (implemented):
- It rejects a claim when the segment is unknown, the quote is not in that segment, a governing negator was cut off, another person's statement is presented as the user's own experience, or the claim asserts a diagnosis or a medication/treatment directive.
- When only the wording is wrong, the claim keeps the user's own quote as its wording and records why. That covers: negation changed for any shared word; a changed subject, time or certainty marker; invented numbers or quantities; content the quote does not contain; unsafe content; too long.
- A hedge such as "not sure" or "belum yakin" is certainty, not negation (D-023). Qualifiers always come from deterministic analysis of the source.
- Opening lines are dropped when they add content words, numbers, identifiers, diagnoses, medical instructions or pressure language, or reverse the negation of any approved item. Template lines ("Draft without AI") carry the provenance of the item wording they reuse.
- Every item must be reviewed by the user before approval.

In v3 a verifier verdict can only remove or revert AI wording. It never overrides these rules, user review, consent or approval (ADR-0003 VC-8).

## Truthful modes and provenance

Draft provenance (implemented, v2):
- `live_meta`: the configured Modal provider returned the pinned served model name and the output passed validation.
- `test_double`: `cmd/fakellm` in development and CI, labelled "TEST DOUBLE — not Meta". It is refused in production (implemented, `internal/config`).
- `manual`: the user's own sentences, no model.
- `recorded_demo`: reserved for a labelled recording of a real live run. None exists, and none will be fabricated.

**Demo environment (PM decision D-045; not reopened).** The demo uses synthetic data with every feature really working. Its generator and its verifier may each be a live provider on synthetic data or a labelled test double. No demo output is ever shown as live unless a live provider produced it (V33, V40). ADR-0002 §12 ("`demo` refuses `SELA_LLM_MODE=test_double`") and the uncommitted `internal/config/config.go` guard, which allows the test double only in development and test, are a narrower implementation of D-045. That is tracked in [conflict matrix](CONFLICT_MATRIX.md#documentation-and-contract-follow-ups-open) row 52 for the backend foundation agent, who owns both. Until it is resolved, the stricter guard is what runs: a demo without configured live providers shows the honest unavailable state.

Planned additions (ADR-0003 §10):
- Draft provenance gains `live_deepseek`. Verification records become `live_anthropic`, `live_deepseek_alternative`, `unavailable`, `not_requested`, `not_required` or `manual_review` (SPEC V33). Each also stores a verification status; ADR-0003 §10.1 maps the two.
- A labelled test-double verifier report (development, test and the demo) is never stored as `live_anthropic` or `live_deepseek_alternative`. V33 has no value for it yet; ADR-0003 §10.1 uses the V33 value once SPEC adds one.
- Every AI output records provider, model id, served version, route (primary, fallback, verifier, alternative verifier), verifier verdicts and latency.
- The UI names the generator and the verifier on every AI item.

Chain-of-thought and reasoning tokens are discarded and never requested from the verifier: Glimmer `delta.reasoning` (implemented), DeepSeek `reasoning_content` (planned). A schema, a mock test or hackathon affiliation does not establish clinical effectiveness or model quality; see [evaluation](EVALUATION.md). Agreement between models is not clinical evidence.

## Consent and data handling

- **Separate consent per provider (D-029, D-048).** Each provider has its own versioned consent: `ai_processing` (Meta on Modal, US), `ai_provider_anthropic` (Claude, US) and `ai_provider_deepseek` (DeepSeek, PRC), the purpose names reserved in [API_SPEC_V3](API_SPEC_V3.md) §3. Consenting to one never authorizes another. As in v2, consenting to Meta does not authorize sending anything to Anthropic.
- **Disclosure before consent.** It names the provider, location, purpose, data sent, provider retention and withdrawal effect in plain Indonesian and English (ADR-0003 §8.1). Withdrawing a consent stops that provider's use at once and discards in-flight results. Data already sent cannot be recalled from the provider, and the UI says so.
- **Minimisation (ADR-0003 §9).** Only the segments a task needs are sent. Contact details and names the user marked private are redacted. Nothing is sent from conversations the user chose not to save. No account identifiers are sent, and span ids are per request.
- **Logs** keep no prompts, narratives, claims, verdict reasons or result text; metadata only (V12).
- **Minors.** DeepSeek is not used for `15_17` or `unknown` age bands. Meta and Claude serve those bands only where they are enabled (non-production today, D-030).
- **Research data.** The research registry of 7,580 Europe PMC candidates is bibliographic discovery data, not validated facts or free-to-train copyrighted text. User notes are never used for training (D-047).
- **Verification outcomes.** Every outcome shows the verification provider's name and an honest per-claim result. An unavailable verification never silently becomes a "checked" label.

## Usage policies and provider terms (observed 2026-10-26; details in ADR-0003 §2)

- **Muse Glimmer Usage Policy:**
  - Prohibits unlicensed medical/health practice, inferring health information without a legal right, facilitating self-harm and presenting outputs as human-written.
  - Requires disclosing known dangers, and forbids presenting third-party output as Meta's.
  - Says the model is not intended for people under 18, and the model card makes deployers responsible for assessing and mitigating those risks.
  - Sela's response: crisis screen before inference, AI labels on every AI wording, no medical outputs, disclosed limitations, provider labels, and a documented risk assessment before minors are enabled in production (B-10).
- **Anthropic Usage Policy:**
  - Every consumer chatbot discloses AI use at the start of each session.
  - In the version in force until 2026-11-12, mental health is a high-risk use when outputs advise individuals.
  - Sela uses Claude only as an internal fidelity check whose verdicts remove or revert wording, and never presents its output as advice.
  - Products serving minors must follow Anthropic's minors guidance.
- **DeepSeek Open Platform terms:**
  - Disclose processing rules and that output is AI-generated; outputs are not medical advice; Chinese law applies.
  - The DeepSeek Terms of Use require guardian consent for under-18s.
  - No API retention period was found, and whether API data is used for training is unverified. The DeepSeek consent text says both.
- **Hackathon (D-049):**
  - A Meta-only demo profile is kept beside the full demo (D-045). In it, AT-3 and AT-4 abstain, because no verifier is permitted (ADR-0003 RR-15).
  - Every output is labelled with its provider.
  - Claude and DeepSeek are never counted as Meta tooling.
  - The organizers' answer on non-Meta providers is pending (B-12).
- **Legacy deployments (D-046).** Two legacy Dengar.in deployments outside this repository may process real users' text with DeepSeek, Gemini and a free-tier OpenRouter model. That is an owner risk (B-13), not part of Sela's routing.

## History: superseded statements

These v2 statements were true for the v2 scope. They stay here as history and, where noted, still describe the current code until P5 lands.

| v2 statement | Superseded by | Now |
| --- | --- | --- |
| "The Meta model is the only live draft generator." "No other AI provider exists in v2." (also SPEC C12, V9; D-001 "no fallback provider exists") | D-025, D-029 (SPEC C17, V33) | Meta stays primary. DeepSeek may replace it only under ADR-0003 RR-4. The current code still has only `modal`, `test_double` and `disabled` (`internal/llm/provider.go`) |
| "Optional Anthropic Claude Sonnet 5.5 is a separate source-fidelity reviewer, never fallback generative AI" | D-029 (partly) | Still never a generator. It is no longer optional for the system: consultation-summary wording, memory proposals and flagged text must be verified before release, and without a permitted verifier they abstain. Each user still decides by consent |
| "Both providers process synthetic demo content only" | D-025 (SPEC V16 → V34, V39) | Real use with minimised data, per-provider consent and a legal review before a real-user pilot (B-10). Synthetic data is for tests, evaluation and demo |
| Usage-policy response "fictional/synthetic data, adult attestation" | D-025, D-030 | Age bands with protective defaults; minors disabled in production until legal review |
| Verifier outcomes `model_supported` / `needs_human_review` / `verification_unavailable` | ADR-0003 VC-2, VC-8 (SPEC V34) | Per-claim verdicts `supported` / `unsupported` / `contradicted` / `insufficient_evidence`, plus issues and `verification_unavailable` |
