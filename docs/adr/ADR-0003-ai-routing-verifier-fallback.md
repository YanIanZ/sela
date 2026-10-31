# ADR-0003 — AI routing: Meta generator, Claude verifier, DeepSeek fallback

> **2026-10-31 — superseded in part by [D-065](../decisions/DECISION_LOG.md#d-065):** the Claude verifier and the DeepSeek fallback are removed. One *different Meta model* (`muse-wonder-1.0` by default) verifies under the existing `ai_processing` consent, and there is no fallback generator or second verifier. Where this document names Anthropic/Claude, DeepSeek, `live_anthropic`, `live_deepseek_*` or `ai_provider_*` consents as current behaviour, D-065 governs; the routing, consent, minimisation, guard, provenance and "no unverified release" guarantees continue with the Meta verifier.
Status: **accepted as design** (PM decisions D-030 and D-049, 2026-10-26). Implementation: **in progress** (milestone P5): committed on branch `feat/sela-v3-ai` (`44845d3` and `63a5c98`, then `02051ae`, which merges `feat/sela-v3-pilot` at `59dba14`), not merged; nothing verified, and nothing of it is wired into the delivery branch `feat/sela-v3-pilot`. No provider in this ADR has been called live; live use is **blocked externally** (B-1, B-8, B-9, see §13). Date: 2026-10-26.
Authority: owner brief v3 §8 (AI strategy and verifier limits), §6 (ages 15–17), §11 (safety, hackathon); owner direction D-026; PM decisions D-030, D-031, D-035, D-036, D-038, D-040, D-046, D-049, D-050; [SPEC](../../SPEC.md) C4, C13 (the owner's DeepSeek fallback), C18 (partly supersedes C12), I.ai, I.fallback, I.meta, I.verifier, V3, V17, V18, V19–V21 (the owner's fallback guards), V22, V25, V35 (supersedes V9 and V10), V36 and V41 (together supersede V16), V42. The SPEC amendments are made and committed with the planning documents (§14). Owner contract for the fallback: [DEEPSEEK_FALLBACK](../architecture/DEEPSEEK_FALLBACK.md), reference implementation `prototypes/generationfallback`; where this ADR differs from it, §17 says which side governs. Related: [ADR-0001](ADR-0001-meta-open-weight-model-on-modal.md) (Meta model and Modal serving, unchanged), [ADR-0002](ADR-0002-persistence-accounts-and-encryption.md) (consent records, encryption at rest, environments).

Statement labels, as in ADR-0002: **PM decision** (taken; recorded here, not reopened), **engineering default** (revisable by a later ADR or by evaluation results), **observed** (read from the named primary source on 2026-10-26; sources in §16), **implemented** (code and tests exist today), **pending** (needs an owner, legal or provider action), **unverified**.

## 1. Context

What exists today (implemented, not deployed):

- **Meta generator path.** `internal/llm` is an OpenAI-compatible streaming client for Muse Glimmer 30B on Modal. Its modes are `modal`, `test_double` and `disabled`, and `New` rejects any other (`internal/llm/provider.go`). Retries happen only before the first content delta (`DefaultMaxRetries` 2, `DefaultAttemptTimeout` 90 s). Every chunk is checked against the pinned served model name. `internal/journey/ops.go` adds admission, cold-start polling every 10 s within twice the cold-start estimate, operation deadlines and stale-result discard. Live inference is blocked (B-1); no `live_meta` output exists.
- **Deterministic validation.** Meaning Lock checks quote grounding, negation, subject, time and certainty (`internal/ledger/validate.go`, `qualifiers.go`). Clinical-language checks are `ContainsDiagnosisLabel` and `ContainsMedicalInstruction` (`internal/ledger/clinical.go`). Opening lines have their own validation (`internal/ledger/opening.go`). There are 13 claim gates for product claims (`data/research/corpus/eval_fixtures/claim_gates.json`).
- **Provenance.** Draft provenance is `live_meta | recorded_demo | manual | test_double` (D-019, `internal/journey/model.go`).
- **EvidenceGuard prototype.** `prototypes/evidenceguard` is a standalone Go module that calls Claude, implemented as a v2 prototype. Its 6 mocked tests pass locally and in its own CI workflow (2026-10-26). It is not wired into any runtime and is not v3 evidence. Its verdict enum lacks `unsupported` ([CLAUDE_VERIFIER](../architecture/CLAUDE_VERIFIER.md) §3).
- **DeepSeek.** There is no DeepSeek code in the v2 or v3 Go runtime. The owner's standalone Go prototype `prototypes/generationfallback` (main, `f4ccc4b`; 9 tests with mocked providers and its own CI workflow) is the reference implementation of the owner's fallback contract ([DEEPSEEK_FALLBACK](../architecture/DEEPSEEK_FALLBACK.md); SPEC C13, V19–V21). It is not wired into any runtime and is not v3 evidence. v1 TypeScript `services/orchestrator/src/providers/deepseekProvider.ts` (model `deepseek-flash`, `json_object`, `temperature` 0.6) is wired only into the v1 orchestrator. The temperature is silently ignored because thinking is on by default (observed).
- **v2 rule.** "Meta is the only live generative provider; no fallback" (SPEC C12, V9; D-001). main's C13 and V9 (D-025) first allowed the owner's opt-in DeepSeek fallback, though the v2 runtime still has no DeepSeek route; D-026 and D-030 supersede the rule as policy, and SPEC records that as C18 and V35. It still describes the code until P5 lands.

Owner brief v3 §8 requires:

- Meta open-weight on Modal as primary.
- Claude Sonnet 5.5 as verifier through an application API. A Claude Code subscription is not that API (observed, §2).
- DeepSeek-V4.1-Flash-1017 as fallback.
- A routing policy that answers five questions: when Meta is used, when Claude must verify, when DeepSeek replaces the generator, when DeepSeek may be the alternative verifier, and when to abstain or fail safe.
- No fallback past a safety refusal, processing only with consent, minimised data.
- Eight verifier checks, four structured verdicts, and no requested or stored chain-of-thought.
- Health claims and professional summaries checked before release, urgent support never waiting for a model, and an evaluation of baseline, verifier benefit, false acceptance/rejection, unsupported statements, latency, cost and fallback quality.

## 2. Provider facts (observed 2026-10-26 unless marked)

| | Meta (generator) | Anthropic (verifier) | DeepSeek (fallback generator, alternative verifier) |
| --- | --- | --- | --- |
| Identifier in code | Served name `sela-glimmer-cec94fc` = `meta-models/Muse-Glimmer-30B` @ `cec94fcfd6512f2cfe708138c32533b346ea4f69` (pin still valid; ADR-0001) | `claude-sonnet-5-5`: a pinned snapshot with no dated variant, released 2026-09-28, retirement not before 2027-09-28 | `deepseek-flash`. It is now served by version **`DeepSeek-V4.1-Flash-1017`** (released 2026-10-17). That version string is not the API `model` value. Legacy `deepseek-v4-flash` is retired and routed to 1017. The alias can move, so Sela records the response `model` field (its content is unverified) |
| Endpoint | Sela's vLLM server on Modal, `POST /v1/chat/completions` | `POST https://api.anthropic.com/v1/messages` | OpenAI-compatible base `https://api.deepseek.com` (chat completions). Strict tools need `/beta`. The Anthropic-compatible `/anthropic` is not used |
| Auth | `Modal-Key`/`Modal-Secret` plus `Authorization: Bearer` (vLLM key) | `Authorization: Bearer` is primary (`x-api-key` is legacy). `anthropic-version` is required: latest `2026-10-21`, `2023-06-01` still listed. `anthropic-workspace-id` is needed only for a key that is not scoped to a workspace | `Authorization: Bearer` |
| Request constraints | No sampling override. `chat_template_kwargs.reasoning_strength: "low"`. Reasoning arrives in `delta.reasoning` and is discarded | These return 400: `temperature`/`top_p`/`top_k`, `thinking:{type:"disabled"}`, forced `tool_choice`, assistant prefill. The lowest thinking setting is `between_tools` (only at effort ≤ high). Default effort is high | Thinking is on by default at effort high. Toggle with `thinking:{type:"enabled"\|"disabled"}`; `reasoning_effort` takes low, high or max. Chain-of-thought arrives in `reasoning_content` |
| Structured output | `response_format: json_schema`, which may not be enforced under the reasoning parser, so Go validation is the guarantee (ADR-0001) | `output_config.format` json_schema is GA (grammar-constrained). Rules: `additionalProperties:false`; string enums are allowed; no length/numeric bounds and no `minItems` > 1; at most 24 optional and 16 union-type parameters; grammars cached 24 h. The schema guarantee breaks on `stop_reason:"refusal"` (HTTP 200, billed) and on `max_tokens` truncation. Enum casing is not guaranteed. It cannot be combined with Citations. A field asking for "reasoning" can trigger a `reasoning_extraction` refusal | Only `response_format:{type:"json_object"}`: no schema, the prompt must contain "json", and it "may occasionally return empty content". Schema validation exists only for strict tool calls (beta) |
| Limits | `--max-model-len` 16384, 8 sequences, `max_containers=1` (ADR-0001) | 1M context, 128K output. Start tier: 1,000 RPM, 2M ITPM, 400K OTPM, $500 monthly cap. 429 carries `retry-after`; 429 `enforced_spend_limit_reached` at the cap. New organizations may start on an "Evaluation" tier | 1M context, 384K output, 2,500 concurrent requests per account. The connection closes if inference has not started within 10 min |
| Price | GPU time: H100 $0.001097/s ≈ $3.95/h while warm | Per MTok: $2 input, $10 output; cache read $0.10. Batch −50 %. `inference_geo:"us"` multiplies by 1.1 | Per MTok, off-peak/peak: input $0.15/$0.30 (cache hit $0.003/$0.006), output $0.60/$1.20. Peak is 01:00–04:00 and 06:00–10:00 UTC on weekdays, which is 08:00–11:00 and 13:00–17:00 WIB |
| Processing location | Modal, US (Modal Servers default us-east) | Workspace storage geo `us` only. `inference_geo` is `global` (default) or `us`. Indonesia is a supported region | Hosted API: PRC ("collect, process and store … in People's Republic of China", privacy policy 2026-02-10). Option under B-9: Modal Shared Endpoint, which routes through us-west |
| Retention | Sela's vLLM request logging is off (ADR-0001). Modal's platform-side handling of request bodies is **unverified** | Deleted within 30 days by default. Flagged content is kept up to 2 years and classifier scores up to 7 years, even under ZDR. ZDR is a per-organization contract (none in place). Structured-output schemas are cached 24 h. Batch is not ZDR (29 days) | No API retention period or zero-retention option found. Whether API traffic is used for training is **unverified** |
| Minors | Usage Policy: "not intended for individuals under the age of 18". The card makes deployers responsible for assessing and mitigating risks | Allowed for products serving minors if Anthropic's minors guidance is followed: age verification, moderation, monitoring, the child-safety system prompt where provided, child-privacy compliance, AI disclosure | Terms of Use: "primarily intended for adults", and under-18s need guardian consent. Privacy policy: "not aimed at children" |
| Health and disclosure | Usage Policy bans unlicensed medical/health practice and inferring health information without a legal right. Disclose known dangers. Never present third-party output as Meta's | **AUP 2025-09-15** (in force until 2026-11-12) lists "therapy, mental health" as High-Risk: a qualified human reviews before advice is disseminated, and AI use is disclosed at session start. **AUP from 2026-11-12** narrows this to "High-risk AI Recommendations" and excludes internal summarization or analysis that is not the delivered recommendation. Every consumer chatbot must disclose AI | Open Platform ToS (effective 2026-04-29; read only as a summarised extract): the developer discloses its processing rules and that output is AI-generated; outputs are not medical advice; mainland-China law, Hangzhou court |

A Claude Code subscription (OAuth with Free, Pro or Max credentials) is not application API access. Products need a Claude Console organization and an API key (observed: code.claude.com legal and compliance; support article 9876003). Max and Team plans include monthly Platform credits that can fund a linked Console organization (support article 17154008). Whether the owner holds such a plan is unverified.

## 3. Task taxonomy (PM decision D-030; budgets are engineering defaults)

| ID | Task | Result and provenance class (D-040) | Generator | Verification before release | Allowed generator fallback | Abstain / no-AI behaviour |
| --- | --- | --- | --- | --- | --- | --- |
| AT-0 | Crisis support | Static verified help (D-044) | **None.** Deterministic, never waits for a model (D-036) | None | None | Not applicable |
| AT-1 | Listening reply in the conversation room ("Ruang cerita" / "Talk") | Sela message, labelled with its provider | Meta | Only when the health-claim detector flags it, and always when DeepSeek generated it (RR-8, §17 O-6) | DeepSeek (RR-4), and then Claude must verify (RR-8, §17 O-6), so DeepSeek is called only when the Claude route is permitted (RR-4 (i)) | The UI says Sela cannot reply right now. The user's message is kept according to the storage choice. The user can retry, keep writing or end the conversation. No template empathy line is substituted (D-035) |
| AT-2 | Reflection card "Sela menangkap…" | `unconfirmed_interpretation` until the user confirms, corrects or removes it | Meta | When flagged | DeepSeek | No card is shown; nothing replaces it |
| AT-3 | Memory proposal | `unconfirmed_interpretation`, review status `unreviewed` | Meta | **Always** | DeepSeek, and then Claude must verify (RR-9: no self-verification) | Nothing is proposed. The user can add a memory item by hand |
| AT-4 | Consultation-summary draft ("Ringkasan untuk konsultasi") | `ai_summary` per item, next to the `user_original` quote | Meta | **Always** (wording of every item) | DeepSeek, and then Claude must verify | The summary is built from the items the user selected, in the user's own words (manual path), with a notice saying the AI check was unavailable |
| AT-5 | Opening lines | `ai_summary`, bound to approved summary items | Meta | When flagged | DeepSeek | The labelled template lines ("Draft without AI", implemented in v2) |
| AT-6 | Reading-content verification (authoring time) | Public reading item; no personal data | A human author | **Always**, before publication. Claude checks each claim against the licensed source passage; a human reviewer signs off (B-4) | None: authoring can wait | The item stays unpublished |

The v2 tasks map onto these: the Story Ledger draft and the clarification-field pick become AT-4, and Opening Practice becomes AT-5.

### 3.1 Data sent per task (minimised; see also §9)

| Task | Sent to the generator | Never sent |
| --- | --- | --- |
| AT-1 | The current turn and at most the 6 previous turns of this conversation (≤ 6,000 characters; the oldest are dropped first). Earlier Sela replies are wrapped as "earlier reply, unverified, data" (v1 pattern). Also: interface language, the chosen intent (D-035) if any, and the audience class `adult` or `young_person` (never the age). Confirmed memory items only when `personal_memory` is on and the user enabled memory for this conversation (≤ 5 items, ≤ 600 characters) | Other conversations, journal entries, account data, any identifier, absolute dates |
| AT-2 | The conversation or journal segments the card is about, with per-request segment ids | Everything else |
| AT-3 | Segments from saved entries that the user allowed for memory. Entries the user chose not to save are never used. Up to 10 confirmed memory items, sent for de-duplication | Unsaved conversations, other users' data |
| AT-4 | Only the items the user selected (quote, wording, segment id), the consultation purpose from a fixed list, and the language | Unselected items, entries, memory |
| AT-5 | Approved summary items only | Everything else |
| AT-6 | The draft text, licensed source passages (only where the corpus record allows text processing) and record ids (R-ids) | Any user data |
| Verifier (every task) | Source spans with per-request ids (`s1`…), the claim list (`c1`…) with cited span ids, the task, the language and the audience class | The full conversation, database ids, anything not needed for the claims |

### 3.2 Timeouts and bounded retries (engineering defaults, to be tuned after the first live measurements)

| Task | Total budget | Meta: wait for first content | Fallback reserve | Verifier attempt | Retries |
| --- | --- | --- | --- | --- | --- |
| AT-1 | 60 s, interactive | 20 s, including any cold start. Beyond that the cold start is "beyond budget" | 20 s (DeepSeek attempt), plus the 15 s verifier attempt that every DeepSeek reply needs (§17 O-6) | 15 s (flagged replies, and every DeepSeek reply, §17 O-6). A reply that needs the check abstains if less than 15 s remain | Each provider: 1 retry before content on connection error, 5xx or 429 with `retry-after` ≤ 3 s. Plus 1 same-provider regeneration after a guard rejection if ≥ 30 s remain (RR-5) |
| AT-2, AT-5 | 120 s, background operation | Until the remaining budget equals the fallback reserve | 40 s | 20 s (when flagged) | As AT-1, with `retry-after` ≤ 10 s |
| AT-3, AT-4 | `SELA_LLM_OP_TIMEOUT` (implemented, default 6 min, maximum 15 min) | Cold start is waited for as in v2 (`journey.generate`), until the remaining budget equals fallback reserve + verification reserve | 60 s | 30 s per call. At most 3 calls of ≤ 12 claims each for AT-4. Verification reserve 90 s | Meta: v2 behaviour (`MaxRetries` 2 before content). DeepSeek and Claude: 1 retry on 429, 5xx, 529 or connection error with `retry-after` ≤ 10 s. No retry on 400, 401/403, refusal, truncation or invalid output |
| AT-6 | No user waiting | — | — | Batch API allowed (no personal data) | Retried until success, or the item stays unpublished |

"Cold start beyond budget" has a precise meaning: Meta has produced no content when the remaining operation budget reaches the reserve that the later steps need. Each attempt, retry and regeneration also stays inside the operation deadline (V22).

## 4. Failure classes

| Class | Examples | Generator fallback? | Alternative verifier? |
| --- | --- | --- | --- |
| **Availability** (before any content was received) | Cold start beyond budget (Modal answers 503 while no container is ready). `llm.KindUnavailable` (connection failure, 5xx after retries). `KindTimeout` before the first content. `KindRateLimited` (429 after retries). Sela's global admission queue for Meta is full (capacity). `KindAuth` (401/403: a credential problem on our side). Anthropic 429 (including `enforced_spend_limit_reached`), 5xx, 529 or overloaded. A provider billing or balance error. A Sela daily spend ceiling reached (§11) | Yes, if RR-4 holds, for the upstream Meta errors only (note below) | Yes, if RR-9 holds |
| **Content or safety** | The generator sets `declined: true`, or a deterministic refusal-phrase match. A raised risk flag. A provider content-moderation rejection. Claude `stop_reason:"refusal"`. Claude `max_tokens` truncation. A Meaning Lock rejection or output-guard hit. Schema-invalid, ID-invalid or empty content. `KindOutputTooLong`. **Any failure after the first content delta** | **Never** | **Never** |
| **Integrity** | `KindModelMismatch` (the served model differs from the pin). A DeepSeek served version that differs from the evaluated one, once the response field has been calibrated (§13) | No: abstain and alert the operator | No |
| **Caller or policy** | The user cancels. Sela's own per-user rate limit. `KindRejected` (another 4xx: a request bug, not an outage). Consent withdrawn mid-flight. The entry or summary version changed. The operator kill switch (`SELA_LLM_MODE=disabled`) | No | No |

Sela's own per-user rate limit is never treated as an outage. Otherwise the fallback would bypass the cost guard (D-022).

**Owner contract (§17 O-1).** For the generator fallback only upstream Meta availability errors count (V19): cold start beyond budget, `KindUnavailable`, `KindTimeout` before the first content and `KindRateLimited`. Sela's own admission queue being full, `KindAuth` (a credential problem on Sela's side), a billing or balance error on Sela's account and a Sela spend ceiling end in the no-AI path with an operator alert. The whole availability row still applies to the alternative verifier (RR-9).

## 5. Routing state machine

```mermaid
stateDiagram-v2
    [*] --> Screen
    Screen --> CrisisHold: crisis rule matched (RR-1)
    Screen --> Preflight: no crisis match
    Preflight --> NoAI: consent, age band, limit, kill switch or no verifier route (RR-2)
    Preflight --> Primary: Meta (RR-3)
    Primary --> Guards: content received
    Primary --> FallbackCheck: availability failure before content
    Primary --> SafeStop: declined or risk raised
    Primary --> Abstain: content or integrity failure
    FallbackCheck --> Fallback: RR-4 holds
    FallbackCheck --> NoAI: otherwise
    Fallback --> Guards: content received
    Fallback --> SafeStop: declined or risk raised
    Fallback --> NoAI: any other failure (RR-6)
    Guards --> Release: passed and no verification needed
    Guards --> Verify: verification required or flagged (RR-8)
    Guards --> SafeStop: risk raised
    Guards --> Abstain: rejected after the one same-provider regeneration
    Verify --> Release: verdicts applied (RR-12)
    Verify --> AltVerify: Claude availability failure and RR-9 holds
    Verify --> Abstain: refusal, truncation, invalid output or no alternative (RR-10, RR-11)
    AltVerify --> Release: verdicts applied
    AltVerify --> Abstain: any failure
    Release --> Commit
    Commit --> Discarded: version, consent or crisis state changed (RR-13)
```

Rules (PM decision D-030 where cited; otherwise engineering default):

- **RR-1 Deterministic screen first (D-036).**
  - Every user text is screened by the safety lexicon before storage and before any model call.
  - A crisis match makes zero calls to any provider. It shows inline verified help and the Immediate help page. It pauses generation for that conversation until the user chooses to continue after seeing help.
  - The text is kept only if the storage choice allows it, and is never erased or refused.
  - Crisis support never waits for a model (AT-0).
- **RR-2 Preflight.** A provider call starts only if all of these hold, checked in Go immediately before each call (not only at operation start):
  - the consents the call needs are granted on the current policy version (§8);
  - the age band allows that provider (§8.3);
  - Sela's per-user limits and daily spend ceilings allow it;
  - no kill switch is set.
  
  A verifier-gated task (AT-3, AT-4) starts only if the Claude route is permitted for this user: consent, age band and configuration. The alternative verifier covers a Claude outage (RR-9), never a missing Claude consent. Sela does not generate what it could not release (minimisation). Otherwise the user sees the manual path with an explanation. The same holds for an AT-1 reply that only DeepSeek could write: Claude always verifies it (RR-8, §17 O-6), so DeepSeek is not called for it unless the Claude route is permitted and the per-user verifier limit and Sela's verifier daily spend ceiling allow the check (RR-4 (i)); no alternative verifier can stand in for that check (RR-9).
- **RR-3 Meta is primary for every generative task (D-030).** Meta never verifies and never sees another user's data, the research corpus or account data.
- **RR-4 When DeepSeek replaces the generator (D-030).** All of the following must hold:
  - (a) Meta failed with an upstream availability failure before any content was received (§4, as narrowed by the owner contract, §17 O-1);
  - (b) the task allows a fallback (§3);
  - (c) `ai_provider_deepseek` is granted on its current policy version;
  - (d) the age band is `18_plus` (§8.3);
  - (e) `SELA_AI_FALLBACK=deepseek` is set;
  - (f) the remaining budget is at least the task's fallback reserve;
  - (g) DeepSeek is configured;
  - (h) the owner's checks hold (C13, V19–V21): the crisis screen passed, the Meta consent (`ai_processing`) and the DeepSeek consent are both current, and the enablement comes only from server-side state (the deployment flag and the user's stored consent), never from the flag alone or from a request field (§17 O-2);
  - (i) for AT-1, the Claude route is permitted for this user (consent, age band, configuration and the per-user verifier limit) and Sela's verifier daily spend ceiling (`SELA_VERIFIER_DAILY_USD`, §11) is not yet reached, because a DeepSeek reply is always verified by Claude before release (RR-8, §17 O-6) and the alternative verifier never checks DeepSeek output (RR-9). No text is sent to DeepSeek only for the reply to abstain on a state Sela knows before the call (RR-2, minimisation); a Claude failure after the DeepSeek call still withholds the reply (RR-9, RR-11). The test is `routing.deepseek_listening_calls_without_claude_route` = 0 (§12).
  
  Otherwise the result is the honest no-AI path.
- **RR-5 Never fall back past a refusal or safety block (D-030).**
  - These outcomes are terminal for the operation's provider chain: a decline, a raised risk flag, a provider content rejection, a Meaning Lock rejection, an output-guard hit, invalid or empty content, output that is too long, or a model mismatch.
  - For AT-1, AT-2 and AT-5 only, one regeneration **on the same provider** is allowed after a Meaning Lock or output-guard rejection, with the rejection reason added as a constraint.
  - No regeneration follows a decline or a raised risk flag.
- **RR-6 One switch at most.**
  - An operation switches generator at most once (Meta → DeepSeek). A DeepSeek failure ends in no-AI.
  - Claude never writes replies, reflections, memory items, summaries or opening lines; its only output is verdicts with short reasons (§6). It is never a generator fallback.
  - Anthropic's server-side `fallbacks` option is never enabled.
- **RR-7 The same guards for every generator.** Meaning Lock, the output guards, the health-claim detector and the risk flag apply identically to Meta and DeepSeek output:
  - The generator schema carries `risk` (`none | elevated | crisis`) and `declined` (boolean). The risk flag can only raise the route, never lower it (D-036).
  - `crisis` → as RR-1 (help shown, generation paused, this output not shown).
  - `elevated` → the output may be released, with support information shown beside it.
- **RR-8 When verification is required (D-030).**
  - Always: AT-3 and AT-4, and AT-6 at authoring time.
  - AT-1, AT-2 and AT-5: when the deterministic health-claim detector flags the output.
  - **Hard guards** reject outright and are never released through verification: a diagnosis addressed to the user, a medication or treatment directive (`ContainsMedicalInstruction`), a professional persona, exclusivity ("cuma aku yang ngerti kamu"), dependency cues, guilt or pressure.
  - **Flags** send the output to the verifier: a condition or symptom interpretation, a statement about research or statistics, an efficacy claim, general health information. The detector is planned. It is built from `ContainsDiagnosisLabel`, `ContainsMedicalInstruction` and the claim-gate patterns, with new bilingual patterns.
  - Ordinary AT-1/AT-2/AT-5 output that passes the hard guards and is not flagged is released without a verifier (D-030).
  - Exception (V20 applied to AT-1, a brief-v3 task; the Claude check is this ADR's means, an engineering default under D-053, not owner direction; §17 O-6, O-8): an AT-1 reply that DeepSeek generated as the fallback has no source-linked claims for Meaning Lock to check, so it is always verified by Claude against the user's turns before release. RR-4 (i) calls DeepSeek for AT-1 only when the Claude route is permitted; if Claude then fails, or the route is no longer permitted, the reply abstains (RR-9, RR-11: DeepSeek never verifies its own output).
- **RR-9 When DeepSeek may be the alternative verifier (D-030).** All of the following must hold, and the verdicts are labelled "alternative verifier":
  - (a) Claude failed with an availability failure;
  - (b) the generator of this output was Meta (**never verify a model's own output**);
  - (c) `ai_provider_deepseek` is granted;
  - (d) the age band is `18_plus`;
  - (e) `SELA_AI_ALT_VERIFIER=deepseek` is set. The default is off, and it is enabled only after evaluation EV-6 passes;
  - (f) budget remains.
  
  When DeepSeek generated the output, only Claude may verify it.
- **RR-10 Claude refusal or truncation → `verification_unavailable`.** No alternative verifier is tried. The same holds for invalid output (VC-6). Each reason is recorded (`refused`, `truncated`, `invalid_output`).
- **RR-11 Abstain when no permitted verifier is available (D-030).**
  - Verifier-gated outputs are not released.
  - The user's own words are kept, the manual path is offered, and the UI says that the check was unavailable and why ("no consent", "service unavailable").
- **RR-12 Release by verdict.** See VC-8.
- **RR-13 Commit re-check (V18, V22).**
  - Before an output is stored or shown, Sela rechecks the conversation or entry version, the consent revision of every provider used, the crisis state and the account session.
  - Any change discards the output (`stale_result`).
- **RR-14 Fallback is per operation and visible.**
  - There is no sticky or silent switch. Every output records its route (§10), and the UI labels the provider.
  - Circuit breaker (engineering default): after 3 consecutive availability failures of Meta within 2 minutes, AT-1 stops waiting for a Meta cold start for 60 s. It still sends every request to Meta first (owner contract, §17 O-12): Modal's immediate 503 while no container is ready counts as the availability failure at once, and only then does the RR-4 check run. Background tasks still wait for the cold start.
- **RR-15 Kill switches and profiles.**
  - `SELA_LLM_MODE=disabled` (implemented) turns Meta off. That is not an outage, so it never triggers the fallback: all generation stops.
  - `SELA_AI_FALLBACK=off` is the default in every environment (V19) and is the default in hackathon mode (C13), not a fixed setting there. Turning it on is an explicit deployment decision, only after B-9 and EV-5, and for real users after B-10; it is never a default. A hackathon demonstration of the fallback is the owner's choice and is disclosed (§17 O-2).
  - `SELA_VERIFIER_MODE=disabled`: verifier-gated tasks abstain.
  - `SELA_AI_PROFILE=meta_only` is the Meta-only hackathon demo path (D-050), kept beside the full demo (D-046, §14). It means no DeepSeek and no Claude. AT-1, AT-2 and AT-5 work fully; AT-3 and AT-4 abstain under RR-11.
- **RR-16 Prompt-injection hygiene.**
  - User text, earlier replies, source spans and claims are wrapped and marked as data in every prompt.
  - No model output is executed, followed as a URL or used to call a tool.
  - No model output can grant consent, approve, share or change a state (V7, V17).

## 6. Verifier contract

- **VC-1 Input (built only by Go from owned, current records).**

  ```json
  {"task": "consultation_summary", "language": "id", "audience": "adult",
   "spans":  [{"id": "s1", "text": "Aku belum pernah cerita ke siapa pun soal ini."}],
   "claims": [{"id": "c1", "text": "Kamu sudah pernah bercerita kepada temanmu.", "span_ids": ["s1"]}]}
  ```

  Limits:
  - ≤ 12 claims and ≤ 16 spans per call (as in the prototype). A larger AT-4 summary is split into at most 3 calls.
  - Span text ≤ 1,600 bytes, claim ≤ 800 bytes, ≤ 5 cited spans per claim.
  - Ids are opaque and per request. Go keeps the mapping to database ids.
  - For a flagged AT-1 reply, and for every AT-1 reply DeepSeek wrote (RR-8), each reply sentence (deterministic segmentation) is a claim, and the user's turns are the spans.
- **VC-2 Output schema** (no personal data in names or enums, because Anthropic caches schemas for 24 h):

  ```json
  {"type": "object", "additionalProperties": false, "required": ["claims", "issues"],
   "properties": {
    "claims": {"type": "array", "items": {"type": "object", "additionalProperties": false,
      "required": ["claim_id", "verdict", "span_ids", "reason"],
      "properties": {
        "claim_id": {"type": "string"},
        "verdict":  {"type": "string", "enum": ["supported", "unsupported", "contradicted", "insufficient_evidence"]},
        "span_ids": {"type": "array", "items": {"type": "string"}},
        "reason":   {"type": "string"}}}},
    "issues": {"type": "array", "items": {"type": "object", "additionalProperties": false,
      "required": ["category", "claim_id", "span_ids", "reason"],
      "properties": {
        "category": {"type": "string", "enum": ["lost_context", "citation_invalid", "medical_language", "manipulation", "age_inappropriate", "information_leak", "safety_signal"]},
        "claim_id": {"type": "string"},
        "span_ids": {"type": "array", "items": {"type": "string"}},
        "reason":   {"type": "string"}}}}}}
  ```

  Shape:
  - Every field is required. There are no union types and no length bounds, which keeps the schema inside the observed limits.
  - `claim_id: ""` marks an issue that concerns the whole output.
  - The field is called `reason`, never "reasoning". The instructions ask for one short sentence naming what differs, not step-by-step thinking, which avoids `reasoning_extraction` refusals.
- **VC-3 Verdict meaning.**
  - `supported`: every important detail follows from the cited spans, with no change of negation, speaker, time, certainty or quantity and no added content.
  - `unsupported`: the claim adds something no supplied span states (a detail, cause, interpretation, diagnosis, number or time), although nothing in the spans conflicts with it.
  - `contradicted`: a supplied span conflicts with the claim (negation flip, another person's experience presented as the user's, a changed time or certainty).
  - `insufficient_evidence`: the spans are too short, ambiguous or off-topic to decide.
  
  "Unsupported" is a positive finding of an addition. "Insufficient evidence" means the verifier cannot tell.
- **VC-4 Claude request (planned).**

  ```
  POST https://api.anthropic.com/v1/messages
  Authorization: Bearer <SELA_ANTHROPIC_API_KEY, workspace-scoped>
  anthropic-version: 2026-10-21
  content-type: application/json
  {"model": "claude-sonnet-5-5", "max_tokens": 2000, "system": "<versioned verifier instructions>",
   "messages": [{"role": "user", "content": "<VC-1 JSON as data>"}],
   "thinking": {"type": "between_tools"},
   "output_config": {"effort": "low", "format": {"type": "json_schema", "schema": "<VC-2>"}},
   "inference_geo": "us"}
  ```

  - `inference_geo: "us"` (×1.1 price) makes the "processed in the US" disclosure true. The exact field placement must be confirmed against the Data residency doc at implementation.
  - Not used: `temperature`/`top_p`/`top_k`, prefill, tools, `tool_choice`, Citations, server-side `fallbacks`, `metadata.user_id`, and the Batch API for personal data (Batch is not ZDR).
  - The call is server to server only, and the key never reaches the browser.
- **VC-5 DeepSeek alternative-verifier request (planned; only under RR-9).**
  - `POST https://api.deepseek.com/chat/completions` (the base URL is observed; the path follows the OpenAI format, and the v1 code uses the `/v1/chat/completions` form) with `model: "deepseek-flash"`, `response_format: {"type": "json_object"}`, `thinking: {"type": "disabled"}`, `max_tokens` 2000 and `stream: false`.
  - The system text contains the word "json" and the VC-2 schema as text. No `user_id` is sent.
  - Strict tool calls on `/beta` are an evaluated option, not the default, because they are a beta feature.
  - Empty content → `verification_unavailable`.
  - Any `reasoning_content` is dropped unread and never stored or logged.
- **VC-6 Go validation (fail closed, extending the prototype's checks).**
  - Claude: HTTP 200, response `model` recorded and equal to `claude-sonnet-5-5`, `stop_reason == "end_turn"`, exactly one text block, body ≤ 64 KiB.
  - Strict JSON decoding: unknown fields and trailing data are rejected.
  - The claim-id set must equal the request's exactly: none missing, extra or duplicated.
  - Verdict and category are compared **case-insensitively** against the enums.
  - `span_ids` ⊆ the supplied spans, and a `supported` verdict cites ≥ 1 of the claim's own spans.
  - `reason` is non-empty, ≤ 300 characters, control characters are stripped, and it is rendered as text, never HTML.
  - An issue's `claim_id` is `""` or a known claim.
  - Any violation rejects the whole report (`invalid_output`). Partial reports are never used.
- **VC-7 No chain-of-thought.**
  - No prompt asks for reasoning steps.
  - Claude runs with `between_tools`, without tools, so the response is text only.
  - DeepSeek runs with thinking disabled, and any `reasoning_content` is discarded.
  - Glimmer's `delta.reasoning` is discarded (implemented).
  - Only verdicts, span ids, categories and short reasons are stored.
- **VC-8 Release rules (RR-12).**
  - AI wording is released only for claims that are `supported` and carry no issue.
  - `unsupported`, `contradicted` and `insufficient_evidence` fall back to the user's own quote where one exists, with Sela's fixed explanation per verdict. Without a quote (an opening line, a reply sentence), the sentence is dropped.
  - For AT-1, any non-supported sentence withholds the whole reply.
  - An output-level issue (`medical_language`, `manipulation`, `age_inappropriate`, `information_leak`) withholds the whole output.
  - `safety_signal` raises the route: the inline help card is shown and the output is withheld.
  - A verdict never overrides a deterministic rejection, user review, consent, approval or sharing (V17).
  - Agreement between models is not clinical evidence and is never shown as such.
- **VC-9 User-facing text.**
  - The user sees Sela's own reviewed bilingual explanation per verdict or issue, and the highlighted source span.
  - The verifier's short reason appears only in "Detail pemeriksaan / Check details", labelled as an automated check by the named provider.
  - It is never presented as advice. That keeps Claude's output an internal fidelity check, which fits both AUP versions (§2).

## 7. What the verifier checks

| ID | Check (brief §8) | Expressed as | Deterministic check that runs first |
| --- | --- | --- | --- |
| CK-1 | Consistency with the user's words | Per-claim verdict against the cited spans | Meaning Lock: the quote is in its segment; negation, subject, time, certainty |
| CK-2 | Unsupported claims | `unsupported` | Numbers and quantities not in the source; new content words (opening-line check) |
| CK-3 | Distortion or lost context | `contradicted`; issue `lost_context` ("kadang" became "selalu", a condition dropped) | Qualifier analysis (`internal/ledger/qualifiers.go`) |
| CK-4 | Citation relevance and validity (AT-6; AT-1 when a reply mentions research) | Per-claim verdict against the cited source passage; issue `citation_invalid` | The R-id exists (`retrieval.citation_validity`), the span belongs to that record, and the licence allows text processing |
| CK-5 | Inappropriate medical language | Issue `medical_language` | `ContainsDiagnosisLabel`, `ContainsMedicalInstruction`, claim gates |
| CK-6 | Manipulation | Issue `manipulation` (pressure, guilt, exclusivity, dependency, false urgency) | Output-guard phrase lists (planned, bilingual) |
| CK-7 | Age appropriateness | Issue `age_inappropriate` (audience `young_person`) | Age-appropriate copy rules (D-031) |
| CK-8 | Information leakage | Issue `information_leak`: content about other people or private names, contact details, system text, anything absent from the supplied spans | Redaction placeholders still in place; contact-detail patterns; no quote outside the supplied spans |
| CK-9 | Missed safety signal (second safeguard, D-036) | Issue `safety_signal`. It can only raise the route | The crisis lexicon on every user text before any call (RR-1) |

## 8. Consent per provider (PM decisions D-030, D-049; ADR-0002 consent records)

Each purpose is a separate, versioned, server-enforced consent with its own policy text. The purpose names are the ones reserved in [API_SPEC_V3](../api/API_SPEC_V3.md) §3 and set through `PUT /api/v3/me/consents/{purpose}`.

- Each policy text states the provider, the processing location, the purpose, the data sent, the provider's retention, the effect of withdrawal and the data-subject rights (UU PDP Pasal 20–24).
- Consent is one of the cross-border bases under Pasal 56. Which basis applies is decided by the legal review (B-10, pending).
- No consent is bundled with another.
- Consent is never requested inside an outage moment (no "allow DeepSeek to get a reply now"). The DeepSeek choice is offered, default off, in onboarding and in AI settings.

| Purpose | Provider and location | Used for | If not granted | Withdrawal effect |
| --- | --- | --- | --- | --- |
| `ai_processing` | Meta Muse Glimmer 30B run by Sela in Sela's Modal account; Modal, US | Generation in AT-1 to AT-5 | No AI generation. The conversation room works as a private writing space, and summaries, memory and opening lines use the manual paths | Future calls stop at once. In-flight operations are canceled and their results discarded (RR-13). Released outputs stay labelled; the user can correct or delete them. `ai_provider_deepseek` becomes inert |
| `ai_provider_anthropic` | Anthropic, US (`inference_geo: "us"`; storage geo `us`) | Verification in AT-3, AT-4, flagged AT-1/AT-2/AT-5 and every DeepSeek AT-1 reply (RR-8, §17 O-6) | Verifier-gated tasks (AT-3, AT-4) do not start, and the manual path is offered (RR-2). Flagged outputs abstain. AT-1 gets no DeepSeek fallback (RR-4 (i)). The alternative verifier does not replace a missing Claude consent | Future calls stop. In-flight checks are canceled and their outputs not released. Past verdicts stay as history |
| `ai_provider_deepseek` | DeepSeek (Hangzhou DeepSeek Artificial Intelligence Co., Ltd.), PRC, hosted API. If B-9 picks the Modal Shared Endpoint instead, the location is Modal, US (us-west), under a different policy version | Fallback generator (RR-4; for AT-1 replies only when the Anthropic route is permitted too, RR-4 (i)); alternative verifier when enabled (RR-9) | No fallback: Meta outages end in no-AI. No alternative verifier | Future calls stop. In-flight calls are canceled and discarded. Released DeepSeek outputs keep their label |

### 8.1 Disclosure content that must appear before consent (plain ID/EN, D-049)

- **Meta/Modal:** the text goes to a Meta open-weight model that Sela runs on Modal's servers in the US. Sela does not log it there. Modal's own platform retention is unverified and must be stated as such until confirmed.
- **Anthropic:** only the sentences being checked and the claims go to Anthropic in the US. They are deleted within 30 days by default. Content that Anthropic's safety systems flag can be kept up to 2 years. No zero-retention agreement is in place.
- **DeepSeek:** the text is processed and stored in the People's Republic of China, under DeepSeek's terms and Chinese law. DeepSeek publishes no retention period for API data that Sela could find, and whether the data is used for training is not confirmed. DeepSeek is used only when Sela's main model is unavailable. A conversation reply that DeepSeek writes is checked by Claude before it is shown, so DeepSeek replies only when the Anthropic consent is given too (RR-4 (i), §17 O-6).
- Each disclosure says that the output is AI-generated and that it is not medical advice.
- Changing a provider, endpoint or region changes the policy version, so consent must be given again (`policy_outdated`).

### 8.2 Withdrawal

- Withdrawal takes effect for every call that has not yet started. A call already sent cannot be recalled from the provider, which keeps data according to its retention (§2). The withdrawal screen says so.
- Released outputs are the user's data. They stay, with their provenance, until the user corrects or deletes them. Deleting the account deletes them (crypto-shredding, ADR-0002).

### 8.3 Age bands (engineering default per provider; the legal review B-10 may change it)

| Age band (ADR-0002) | Meta | Anthropic | DeepSeek |
| --- | --- | --- | --- |
| `18_plus` | Yes | Yes | Yes (consent, flags) |
| `15_17`, and `unknown` (treated as not 18+) | Only where the band is enabled: today development, test, demo and staging. Production needs the documented deployer risk assessment that the Glimmer card asks for (B-10, D-031) | Only where the band is enabled, with Anthropic's minors-guidance safeguards. Their sufficiency for production is pending B-10 | **No.** This combines guardian-consent wording that is unverified for a third-party app with PRC processing of children's data. So minors get no fallback generator and no alternative verifier |

Memory and pattern features are opt-in only for minors (D-031, PP 17/2025 Pasal 19). Production allows `18_plus` only until the legal review clears minors (ADR-0002 §3, D-031).

## 9. Data minimisation rules

- **DM-1 Fields.** Only the fields in §3.1 are sent. No account, user, session, conversation or database ids. No username, IP address, `metadata.user_id` (Anthropic) or `user_id` (DeepSeek). Dates are sent only as relative phrases when the task needs time context.
- **DM-2 Redaction before every call.**
  - Contact details (email, phone, NIK, card numbers, URLs, social handles) and names the user marked private are replaced by placeholders such as `[NAMA-1]` and `[KONTAK-1]`.
  - Pattern source: the v1 `services/validator/src/piiRedactor.ts` (inherited from Dengar.in), to be ported to Go.
  - Meaning Lock matches quotes against the redacted text that was sent. Placeholders are mapped back only in the owner's own view, never in anything sent to another provider or recipient.
- **DM-3 Per-request span ids.** Span ids are opaque and per request, so a provider never sees stable identifiers.
- **DM-4 Storage choice respected.** A conversation the user chose not to save is never used for memory (AT-3) or later context. Its AI outputs keep only content-free counters.
- **DM-5 Logs carry metadata only (V12).** Provider, task, route, outcome class, latency and token counts. Never prompts, spans, outputs, verdict reasons or model payloads.

## 10. Provenance recorded per AI output (D-030, D-040)

Records live in the P5 migration range 0500–0599 ([API_SPEC_V3](../api/API_SPEC_V3.md) §9). Content fields are encrypted per user (ADR-0002 §5); enum and metric fields stay plaintext.

| Field | Values |
| --- | --- |
| `task` | `listening_reply`, `reflection`, `memory_proposal`, `summary_draft`, `opening_lines`, `reading_verification` |
| `route` | `primary`, `fallback`. Verification route: `verifier`, `alternative_verifier`, `none` |
| `fallback_reason` | `null`, `cold_start_budget`, `unavailable`, `timeout`, `rate_limited`: the upstream Meta availability failures that may trigger the fallback (§4 note, RR-4 (a), §17 O-1). Sela's admission queue (capacity) and `KindAuth` never trigger it, so they are not values here |
| Generator | Provider (`meta_modal`, `deepseek`, `test_double`, `none`); configured model id; **served model or version as reported** (Meta served name and HF revision; DeepSeek response `model` field); prompt and schema version; latency to first content and in total; input/output tokens; outcome |
| Verification | `status` (`checked`, `not_required`, `verification_unavailable`, `abstained`, `not_requested`); provider (`anthropic`, `deepseek`, `test_double`) and model as reported; `anthropic-version`; per-claim verdict and span ids, issues and reasons (reasons encrypted); counts per verdict; latency; tokens; `unavailable_reason`, one or more in order of occurrence (`refused`, `truncated`, `invalid_output`, `timeout`, `rate_limited`, `spend_cap`, `auth`, `unavailable`, `no_consent`, `age_band`, `flag_off`, `self_verification_blocked`, `budget`, `user_limit`). The meaning of each `status` is in §10.1 |
| Guards | Meaning Lock version, rejected and reverted counts, health-claim flags, risk flag |
| Consent | The consent revision of every purpose used, rechecked at commit |
| Record | Provenance class (`ai_summary`, `unconfirmed_interpretation`), review status `unreviewed`, created time, version |
| Cost | Estimated cost from tokens (or GPU seconds) and the price-table version (`ai-prices-2026-10-26`) |

The truthful draft values `live_meta`, `test_double`, `manual` and `recorded_demo` stay. `live_deepseek_fallback` (the owner's value, SPEC V10 and V20; earlier drafts of this ADR called it `live_deepseek`) is added. Verification values become `live_anthropic`, `live_deepseek_alternative`, `test_double` (a labelled test-double verifier, §10.1), `unavailable`, `not_requested`, `not_required` and `manual_review` (SPEC V35, which supersedes V10).

The UI labels every AI output with its generator and verifier, for example "Ditulis dengan bantuan AI (Meta Muse Glimmer) · diperiksa oleh Claude" or, for a fallback output, the owner's wording "Meta tidak tersedia; draf ini dibuat DeepSeek Flash 1017" / "Meta was unavailable; this draft was generated by DeepSeek Flash 1017" (§17 O-7). Claude and DeepSeek work is never counted or shown as Meta tooling (D-050).

### 10.1 Verification status and provenance (engineering default)

Each AI output stores one verification `status` and the V35 verification provenance value that follows from it. The state shown to the user is derived from these and is never stored ([CLAUDE_VERIFIER](../architecture/CLAUDE_VERIFIER.md) §5).

| `status` | Set when | V35 verification provenance | AI wording released |
| --- | --- | --- | --- |
| `not_required` | RR-8 asks for no check: the task has no mandatory verification, the health-claim detector did not flag the output and, for AT-1, Meta wrote the reply (a DeepSeek reply is always checked, §17 O-6) | `not_required` | Yes, after the deterministic guards |
| `checked` | A permitted verifier returned a report that passed VC-6, and VC-8 was applied | `live_anthropic` (route `verifier`) or `live_deepseek_alternative` (route `alternative_verifier`). A test-double verifier report takes `test_double` (below) | Per claim (VC-8) |
| `verification_unavailable` | A permitted verifier route existed but produced no usable report: a refusal, truncation or invalid output (RR-10), or an availability failure (`timeout`, `rate_limited`, `spend_cap`, `auth`, `unavailable`) with no permitted alternative that produced one (RR-11). When the alternative verifier was not tried, its reason (`no_consent`, `age_band`, `flag_off`, `self_verification_blocked`) follows the first | `unavailable` | No. The user's words are kept and the manual path is offered (RR-11) |
| `abstained` | A check was required but no verifier was called: no route was permitted for this output (`no_consent`, `age_band`, `flag_off`, including `SELA_VERIFIER_MODE=disabled` and the `meta_only` profile), or Sela's operation or per-user limits stopped the call (`budget`: less remaining time than the verifier attempt, §3.2; `user_limit`, §11). These are not outages, so no alternative verifier is tried (§4). A daily spend ceiling is an availability failure and is recorded as `spend_cap` under `verification_unavailable` | `unavailable` | No, as above |
| `not_requested` | The item has no new AI wording to check: the manual path, a template line ("Draft without AI") or the user's own correction | `not_requested` | Not applicable |

- `verification_unavailable` and `abstained` both withhold the output under RR-11. They differ only in whether a permitted verifier route was tried.
- A verifier-gated task that RR-2 stops before generation produces no AI output, so no status is stored. The UI shows the manual path and the reason.
- The router never sets V35's `manual_review`. It is reserved for a recorded human review. How AT-6 records the reviewer's sign-off next to the Claude check is decided with the reading-content workflow (P6, B-4).
- **Test-double verifier reports** exist in development, test (EV-7) and the demo (D-046), so verifier-gated tasks (AT-3, AT-4) run end to end on synthetic data when no live verifier is configured there. In those environments a configured test-double verifier stands in for the Claude route in RR-2 and for the alternative verifier in RR-9, under the same consent, age-band and release rules. Each report is labelled "TEST DOUBLE — not Claude" or "TEST DOUBLE — not DeepSeek" (engineering default, as the v2 "TEST DOUBLE — not Meta"). It is never stored as `live_anthropic` or `live_deepseek_alternative` and never shown as a check by Claude or DeepSeek. Production refuses it (§14).
- A test-double report is stored under the V35 verification value `test_double` (added to SPEC V35 under D-046; [conflict matrix](../status/CONFLICT_MATRIX.md#documentation-and-contract-follow-ups-open) row 58). It reopens no decision.
- A stored status is never rewritten after release. A later edit, consent withdrawal or version change makes the result stale; the UI derives that state at read time (RR-13, V18).

## 11. Cost (engineering estimates from observed list prices; not measurements)

Assumptions:
- Token counts are estimates for Sela's prompts and must be replaced by measured usage.
- Claude prices include ×1.1 for `inference_geo: "us"` ($2.20 input, $11.00 output per MTok). Prompt caching is not assumed.
- DeepSeek is priced at peak ($0.30/$1.20), because peak covers Indonesian mornings and afternoons. The Modal Shared Endpoint costs the same per token. Off-peak halves the DeepSeek figures.
- Meta is billed by GPU time while a container is warm, whether or not requests arrive. "Attributed" is the generation time an operation occupies (assumed, unmeasured), not marginal cost.

| Operation | Input / output tokens | Claude | DeepSeek | Meta (attributed GPU time) |
| --- | --- | --- | --- | --- |
| AT-1 reply generation | 2,500 / 250 | — | $0.0011 | ~12 s ≈ $0.013 |
| AT-1/2/5 flagged verification; every DeepSeek AT-1 reply (§17 O-6) | 1,500 / 250 | $0.0061 | $0.0008 (alternative; never for a DeepSeek reply, RR-9) | — |
| AT-2 reflection card | 2,000 / 300 | — | $0.0010 | ~12 s ≈ $0.013 |
| AT-3 memory proposals (≤ 5) | gen 3,000 / 600; verify 2,000 / 400 | $0.0088 | gen $0.0016; alternative $0.0011 | ~20 s ≈ $0.022 |
| AT-4 summary (≤ 12 items) | gen 3,500 / 1,500; verify 3,000 / 700 per call | $0.0143 per call (≤ 3 calls: ≤ $0.043) | gen $0.0029; alternative $0.0017 | ~45 s ≈ $0.049 |
| AT-5 opening lines | 1,200 / 300 | — | $0.0007 | ~10 s ≈ $0.011 |
| AT-6 reading item (Batch, global, −50 %) | 6,000 / 1,000 | $0.011 | — | — |

Per session:

| Session | API cost (Claude + DeepSeek) | Meta GPU |
| --- | --- | --- |
| S1 listening: 12 replies, 2 cards, 1 flagged check | $0.006 | Attributed ≈ $0.18. A solo user on a cold GPU pays 4 min cold start + 20 min use + 5 min scale-down ≈ 29 GPU-min ≈ **$1.91** |
| S2 consultation preparation: 1 summary, 3 memory proposals, opening lines | $0.023 | Attributed ≈ $0.08 |
| S3 = S1 + S2 with Meta down and DeepSeek fallback; Claude checks every DeepSeek reply (§17 O-6) | ≈ $0.12 ($0.020 DeepSeek + $0.102 Claude, of which about $0.073 are the 12 reply checks that O-6 adds) | — |
| S4 = S2 with Claude down and the alternative verifier on | ≈ $0.003 | Attributed ≈ $0.08 |

- **The GPU dominates.** API cost is about $0.03 or less per session while Meta serves (S1, S2, S4) and about $0.12 in S3, where Claude checks every DeepSeek reply (§17 O-6); a 36-item summary (3 verifier calls) adds up to about $0.03. With `max_containers=1` the Modal burn is capped at about $3.95/h (ADR-0001).
- **Pilot illustration (not a budget):** 50 users × 8 sessions a month gives at most about $20/month of API cost while Meta serves (≤ $0.05 per session, a 36-item summary included), and at most about $48/month if every session ran on the fallback as in S3. Keeping Glimmer warm 4 h/day costs about $474/month on an H100. The warm hours are the owner's budget decision (B-1).
- **Cost guards (engineering defaults):**
  - At most 40 verifier calls and 60 fallback generations per user per day.
  - Daily ceilings `SELA_VERIFIER_DAILY_USD` and `SELA_DEEPSEEK_DAILY_USD`. Reaching one is an availability failure with an operator alert.
  - The Anthropic Console monthly spend limit.
  - A prepaid DeepSeek balance.

## 12. Evaluation plan (pre-registered here 2026-10-26; to be added to `eval/thresholds.json` as a new version before the first verifier run)

**Labelled synthetic set (EV-0).**

- Built from v2 fixtures (`internal/fixtures/data/scenarios.json` gold facts, the `cmd/sela-eval` fault battery) plus new ID, EN and code-switched items.
- Test split of at least 400 claims: ≥ 120 supported, ≥ 100 unsupported (invented time, number, cause, diagnosis), ≥ 100 contradicted (negation, speaker, time and certainty flips), ≥ 50 insufficient_evidence.
- Also ≥ 80 issue cases (medical language, manipulation, age-inappropriate, leakage, safety signal), ≥ 40 prompt-injection items, and AT-1 replies.
- Two team members label independently; Cohen's κ is reported; a third adjudicates disagreements.
- Labels are hashed and frozen before any verifier run. Prompts are tuned only on a separate development split.
- The labellers are not clinicians. The set measures engineering properties on synthetic text, not clinical validity (C10, V15).

Evaluations:

| ID | Evaluation | Needs |
| --- | --- | --- |
| EV-1 | **Baseline without verifier:** Meta + deterministic guards; rate of released AI statements that gold labels mark unsupported or contradicted | B-1 |
| EV-2 | **With verifier:** the same rate after Claude; **verifier benefit** = baseline − with verifier, absolute and relative. Verification stays required (D-030) even if the benefit is small; a small benefit triggers a prompt review | B-1, B-8 |
| EV-3 | **False acceptance and false rejection** of Claude on EV-0 | B-8 |
| EV-4 | **Latency and cost**, measured live (tokens × price table) | B-1, B-8 |
| EV-5 | **Fallback quality:** DeepSeek as generator on the same scenarios and guards. `SELA_AI_FALLBACK` stays off in production until EV-5 passes | B-9 |
| EV-6 | **Alternative verifier:** DeepSeek on EV-0 with the same thresholds as Claude. Agreement with Claude is reported but is not accuracy. `SELA_AI_ALT_VERIFIER` stays off until EV-6 passes | B-9 |
| EV-7 | **Routing invariants** with labelled test doubles for all three providers (any mode), including the owner's validation matrix and the ported `prototypes/generationfallback` cases (§17 O-11) | none |

Proposed thresholds (names for `eval/thresholds.json`):

| Metric | Op | Value | Mode | Why |
| --- | --- | --- | --- | --- |
| `verifier.false_acceptance_rate` | ≤ | 0.03 | live | Share of gold unsupported + contradicted claims marked `supported`. This is the error that lets a distortion through |
| `verifier.false_acceptance_rate_contradicted` | ≤ | 0.01 | live | Negation, speaker and time flips are the most consequential (same reasoning as `meaning_lock.gold_qualifier_negation_accuracy`) |
| `verifier.false_rejection_rate` | ≤ | 0.15 | live | Share of gold supported claims rejected. Rejection is safe (it falls back to the user's words), but above 15 % the AI draft is of little use |
| `verifier.issue_recall_critical` | ≥ | 0.90 | live | `medical_language`, `information_leak`, `safety_signal` |
| `verifier.issue_recall_other` | ≥ | 0.80 | live | `manipulation`, `age_inappropriate`, `lost_context`, `citation_invalid`: more subjective |
| `verifier.refusal_rate` | ≤ | 0.02 | live | Refusals become abstentions. Above 2 % the summary feature mostly abstains |
| `verifier.contract_valid_rate` | ≥ | 0.99 | live | HTTP 200 responses that pass VC-6 |
| `pipeline.released_unsupported_rate_verified` | ≤ | 0.01 | live | Statements in released verifier-gated output that gold labels mark unsupported or contradicted |
| `latency.verifier_p50_ms` / `latency.verifier_p95_ms` | ≤ | 8000 / 20000 | live | Inside the AT-3/AT-4 verification reserve |
| `latency.listening_reply_p95_warm_ms` | ≤ | 25000 | live | Includes flagged replies; well inside the 60 s AT-1 budget |
| `cost.verifier_per_call_usd_mean` | ≤ | 0.02 | live | §11 estimate $0.006–0.014 per call |
| `cost.api_per_session_usd` | ≤ | 0.10 | live | Sessions while Meta serves: §11 S1, S2 and S4 ≈ $0.03 or less, about $0.05 with a 36-item summary |
| `cost.api_per_session_usd_fallback` | ≤ | 0.20 | live | Sessions on the DeepSeek fallback, every reply checked by Claude: §11 S3 ≈ $0.12, about $0.15 with a 36-item summary. Added 2026-10-26 with §17 O-6, which took S3 above the 0.10 bar; the threshold above keeps its value and now names its scope |
| `fallback.meaning_lock_residual_forbidden_hits` | ≤ | 0 | any | Same bar as the primary |
| `fallback.important_fact_coverage` | ≥ | 0.70 | live | Same bar as `ledger.important_fact_coverage` |
| `fallback.bilingual_wording_language_match` | ≥ | 0.90 | live | Same bar as the primary |
| `fallback.released_guard_violations` | ≤ | 0 | any | No hard-guard phrase in any released output |
| `alt_verifier.false_acceptance_rate` (and the other `verifier.*` thresholds) | as `verifier.*` | | live | EV-6 |
| `routing.fallback_after_content_failure` | ≤ | 0 | any | RR-5: refusal, safety block, Meaning Lock or guard rejection never reaches DeepSeek |
| `routing.self_verification` | ≤ | 0 | any | RR-9 |
| `routing.provider_calls_without_consent` | ≤ | 0 | any | RR-2, including withdrawal mid-operation |
| `routing.deepseek_calls_minor_bands` | ≤ | 0 | any | §8.3 |
| `routing.verifier_gated_released_without_check` | ≤ | 0 | any | RR-11, including every DeepSeek AT-1 reply (RR-8, §17 O-6) |
| `routing.deepseek_listening_calls_without_claude_route` | ≤ | 0 | any | RR-4 (i), RR-2: DeepSeek calls for an AT-1 reply while, at preflight, the Claude route is not permitted for that user (§10 reasons `no_consent`, `age_band`, `flag_off`, including `SELA_VERIFIER_MODE=disabled` and the `meta_only` profile, or `user_limit`, the per-user verifier limit of §11) or Sela's verifier daily spend ceiling `SELA_VERIFIER_DAILY_USD` is already reached (§11; recorded as `spend_cap`, §10.1). Sela knows each of these states before it calls DeepSeek. `routing.provider_calls_without_consent` counts only each provider's own consent, so it cannot see this case. Added 2026-10-26 with §17 O-6 |

`safety.model_calls_on_crisis_path ≤ 0` already exists and covers RR-1 for every provider. Changing any threshold follows the existing rule: a new version, a dated rationale, old results kept, and no lowering to make a run pass.

## 13. Live-test blockers (external)

| ID | Blocks | Owner action |
| --- | --- | --- |
| B-1 | Live Meta on Modal; EV-1, EV-2 and EV-4; real cold-start behaviour for RR-4 | Modal credentials and an approved GPU budget ([BLOCKERS](../status/BLOCKERS.md)) |
| B-8 | Every live Claude call: EV-2, EV-3, EV-4, the refusal rate on Sela's synthetic mental-health excerpts, the response `model` field | Create an Anthropic Console organization and a **workspace-scoped server API key** in the secret manager. A Claude Code, Pro or Max subscription is not application access; Max or Team Platform credits could fund the organization (unverified whether the owner has them). Note the starting rate tier |
| B-9 | Every live DeepSeek call: EV-5, EV-6, the empty-content frequency, the content of the response `model` field (needed to calibrate the version pin) | A DeepSeek API key, **or** a Modal proxy token for the Shared Endpoint "DeepSeek V4.1 Flash" (check `GET /v1/models`: whether it serves 1017 or the earlier 0909 build is unverified). Real-user use also needs the legal OK for PRC processing (B-10) |

Related: B-10 (legal review before real users: DPIA, processor agreements, the cross-border basis under Pasal 56, minors), B-12 (the hackathon organizers' answer on non-Meta providers, D-050), B-7 (generator model decision). Blocker numbers follow the [blockers](../status/BLOCKERS.md) list (B-1 to B-15, PM decisions of 2026-10-26); the email provider is B-11, which ADR-0002 §6 now uses too. Synthetic-data evaluation needs only B-1, B-8 and B-9.

## 14. Consequences and follow-ups

- **Spec amendments (PM): made** in [SPEC](../../SPEC.md) on 2026-10-26 and committed with the planning documents (`4cadbfd`, `59dba14`):
  - C12 partly superseded → C18 (multi-provider AI with consent): Meta is primary, the DeepSeek fallback is permitted under RR-4, Claude is the verifier.
  - V9 and V10 superseded → V35 (routing and provenance, including the §10 values).
  - V16 superseded → V36 (verifier) and V41 (real-user data minimised under §9, replacing "synthetic excerpts only").
  - V17 and V18 extended to the alternative verifier.
  - I.ai added; I.meta and I.verifier amended; T5a carried into T18 (P5).
  - C5 and V25 gain per-provider spend ceilings (§11); I.eval gains EV-0 to EV-7 (§12).
  - The superseded decisions are listed in D-026.
  - After the merge of main `7fc6678` ([D-053](../decisions/DECISION_LOG.md#d-053)): the v3 IDs were renumbered; the owner's C13, I.fallback, V19–V21 and T5b (carried into T18) apply; §17 lists where this ADR differs from the owner's fallback contract.
- **Code (P5; in progress on branch `feat/sela-v3-ai`, not merged, nothing verified; package names are proposals for the P5 owner):**
  - A router that implements §4 and §5 as a pure, table-tested state machine, in `internal/ai`: the production integration (being built), which must satisfy C13 and V19–V21 as well as this ADR. The owner's `prototypes/generationfallback` is its reference implementation, and its test cases are ported (§17 O-11).
  - `internal/llm` gains a DeepSeek mode (OpenAI-compatible, `json_object`, thinking disabled, `reasoning_content` dropped).
  - A verifier package grown from `prototypes/evidenceguard` (gaps listed in [CLAUDE_VERIFIER](../architecture/CLAUDE_VERIFIER.md) §3).
  - The health-claim detector, output guards and the Go port of the PII redactor.
  - Provenance records (migration range 0500–0599).
  - The consent purposes in §8, and `/api/v3/ai` routes specified in API_SPEC_V3 §9 before implementation.
- **Configuration (planned):**
  - `SELA_ANTHROPIC_API_KEY`, `SELA_ANTHROPIC_VERSION` (`2026-10-21`), `SELA_VERIFIER_MODEL` (`claude-sonnet-5-5`), `SELA_VERIFIER_INFERENCE_GEO` (`us`), `SELA_VERIFIER_MODE`.
  - `SELA_DEEPSEEK_API_KEY`, `SELA_DEEPSEEK_BASE_URL` (`https://api.deepseek.com`), `SELA_DEEPSEEK_MODEL` (`deepseek-flash`), `SELA_DEEPSEEK_EXPECTED_VERSION` (`DeepSeek-V4.1-Flash-1017`). The owner's prototype reads `DEEPSEEK_API_KEY`; the runtime keeps the `SELA_` prefix of its other settings.
  - `SELA_AI_FALLBACK`, `SELA_AI_ALT_VERIFIER` (both default `off`), `SELA_AI_PROFILE` (`full` or `meta_only`).
  - Production refuses test doubles (ADR-0002 §12) and starts with fallback and alternative verifier off.
  - **Demo (PM decision D-046; not reopened).** The demo uses synthetic data with every feature really working. The generator and the verifier may each be a live provider on synthetic data or a labelled test double (§10.1). No demo output is ever shown as live unless a live provider produced it (V35, V42). ADR-0002 §12 ("`demo` refuses `SELA_LLM_MODE=test_double`") and the `internal/config/config.go` guard (committed with the foundation, `0ff3341`), which allows the test double only in development and test, are a narrower implementation. That is tracked in [conflict matrix](../status/CONFLICT_MATRIX.md#documentation-and-contract-follow-ups-open) row 52 for the backend foundation agent, who owns both. Until it is resolved the stricter guard runs, so a demo without configured live providers shows the honest unavailable state. The routing rules are the same in every environment.
- **Documentation:** [AI policy](../architecture/AI_POLICY.md), [Meta integration](../architecture/META_INTEGRATION.md) and [Claude verifier](../architecture/CLAUDE_VERIFIER.md) are updated to match this ADR. The runbook (environment variables), the documentation index and the document inventory need entries when P5 lands.
- **Hackathon (D-050):** the `meta_only` profile is the Meta-only demo path, kept beside the full demo (D-046). In it, AT-3 and AT-4 abstain because no verifier is permitted.

## 15. Alternatives considered

| Option | Verdict |
| --- | --- |
| Claude as fallback generator | Rejected. The owner brief makes Claude the verifier, and a model that verifies must not also be the generator of what it verifies |
| DeepSeek as primary | Rejected (owner brief §8) |
| Fallback on any failure, including invalid output | Rejected. Invalid output is content-based, and switching would launder refusals and distortions (D-030) |
| Anthropic server-side `fallbacks` | Rejected. It would route around a refusal |
| Keep the v1 OpenRouter "Second Brain" debiaser, which rewrites drafts | Not migrated (D-038). A free-tier third party may train on the data, and a verifier must return verdicts, not silently rewrite |
| Self-host DeepSeek-V4.1-Flash-1017 on Modal (MIT, ungated, ~511 GB, ≥ 614 GB VRAM, ~$12.50–18.16/h warm) | Deferred. A poor fit for a scale-from-zero fallback, with a long cold start (unmeasured), and it needs its own budget approval |
| Modal Shared Endpoint for DeepSeek ($0.30/$0.03/$1.20 per MTok, us-west) | Kept as a B-9 option. Data stays off DeepSeek's servers, but the build it serves (0909 or 1017) is unverified and it needs its own policy version |
| DeepSeek strict tool calls (beta) instead of `json_object` | An evaluated option in EV-6, not the default |
| Releasing verifier-gated outputs unchecked in the demo | Rejected (D-030). The demo checks them with a live verifier on synthetic data or a labelled test-double verifier (D-046, §10.1); the `meta_only` profile abstains |

## 16. Sources (fetched 2026-10-26 by the external-verification pass)

- **Anthropic:**
  - platform.claude.com/docs/en: `about-claude/models/overview`, `about-claude/models/sonnet-5-5/overview`, `about-claude/pricing`, `build-with-claude/structured-outputs`, `build-with-claude/thinking`, `build-with-claude/refusals-and-fallback`, `api/overview`, `api/versioning`, `api/rate-limits`, `manage-claude/authentication`, `manage-claude/api-and-data-retention`, `manage-claude/data-residency`
  - privacy.claude.com/en/articles/7996866
  - anthropic.com/legal/aup (effective 2026-11-12) and /legal/archive/22742366-2ef0-4c7a-a833-6523f10d3944 (2025-09-15)
  - anthropic.com/legal/commercial-terms; anthropic.com/supported-countries
  - support.claude.com/en/articles/9307344, 9876003, 17154008
  - code.claude.com/docs/en/legal-and-compliance
- **DeepSeek:**
  - api-docs.deepseek.com (first call, `/quick_start/pricing/`, `/quick_start/rate_limit/`, `/guides/json_mode/`, `/guides/tool_calls/`, `/guides/thinking_mode/`, `/news/news261017/`)
  - cdn.deepseek.com/policies/en-US: privacy policy, terms of use, open platform terms of service (the last read only as a summarised extract)
  - huggingface.co/deepseek-ai/DeepSeek-V4.1-Flash-1017 at `1bae0e1088993b9b82de08f59de5be5a34e42170`
  - recipes.vllm.ai/deepseek-ai/DeepSeek-V4.1-Flash-1017
- **Meta and Modal:**
  - huggingface.co/meta-models/Muse-Glimmer-30B README and USAGE_POLICY at `cec94fc`
  - modal.com/pricing, /docs/guide/scale, /docs/guide/shared-endpoints, /library/deepseek/deepseek-v4-1-flash, /changelog
- **Indonesian law** (official JDIH Komdigi and Kemkes texts; not legal advice): UU 27/2022 (PDP) Pasal 4, 20–25, 34, 51, 56; PP 17/2025 Pasal 9, 19, 21, 22. The PP 33/2026 text is unverified (secondary sources only).
- **Repository:** `internal/llm/{llm,provider,client}.go`, `internal/journey/{ops,service,model,prompts}.go`, `internal/ledger/{validate,clinical,qualifiers,opening}.go`, `prototypes/evidenceguard/verifier.go`, `services/orchestrator/src/providers/deepseekProvider.ts`, `services/validator/src/piiRedactor.ts`, `data/research/corpus/eval_fixtures/claim_gates.json`, `eval/thresholds.json`.

## 17. Owner fallback contract: differences and resolutions

Added 2026-10-26, when main `7fc6678` was merged ([D-053](../decisions/DECISION_LOG.md#d-053)). The owner's contract is [DEEPSEEK_FALLBACK](../architecture/DEEPSEEK_FALLBACK.md) together with SPEC C13, I.fallback and V19–V21. Its reference implementation is the standalone prototype `prototypes/generationfallback` (9 tests with mocked providers; not wired into any runtime, not v3 evidence). The production integration is `internal/ai` (P5, being built), which must satisfy C13 and V19–V21 as well as this ADR. Where the two differ in substance, the owner contract wins unless owner brief v3 says otherwise. The rules above are read with these resolutions, and the ones that changed point here.

Already aligned, no change: Meta is tried first, and DeepSeek is never called after unsafe, invalid or refused Meta output, a crisis, missing consent, a cancellation or an error that is not an availability failure (RR-1, RR-2, RR-5, §4). The DeepSeek consent is separate and comes on top of the Meta consent (§8; withdrawing `ai_processing` makes `ai_provider_deepseek` inert). The call is bounded, server to server, with its own key, quota and timeout (§3.2, §11). Versions and consents are rechecked before anything is kept (RR-13). The client never supplies a prompt, model or provider URL (VC-1). Reasoning is disabled and any reasoning text discarded (VC-5, VC-7). The served `model` is recorded because the alias moves (§10). The Meta-only and no-AI paths stay (RR-15), and DeepSeek work is never counted as Meta (D-050).

| # | Topic | Owner contract | This ADR as drafted | Governs | Resolution |
| --- | --- | --- | --- | --- | --- |
| O-1 | Fallback triggers | Only Meta upstream availability errors: timeout, transient 5xx, a documented 429, a temporary upstream outage (V19). Anything not clearly transient or availability-related never falls back | §4 also counted Sela's own admission queue being full, `KindAuth` (401/403 on Sela's credentials), a billing or balance error and a Sela spend ceiling | Owner | For the generator fallback only cold start beyond budget, `KindUnavailable`, `KindTimeout` before content and `KindRateLimited` count (§4 note, RR-4 (a)); the recorded `fallback_reason` takes only these four values (§10). The others end in the no-AI path with an operator alert. The full availability class still applies to the alternative verifier (RR-9), which the owner contract does not cover. Widening the list needs owner direction |
| O-2 | Default and enablement | OFF by default (V19) and in hackathon mode (C13). Enabled for the session from server-side state, never by a silent global switch | `SELA_AI_FALLBACK` off "until B-9 and EV-5"; per-user consent plus the deployment flag | Owner | Off by default in every environment and in hackathon mode (RR-15). Turning it on is an explicit deployment decision after B-9 and EV-5 (and B-10 for real users), never a default. For each operation the router derives the enablement from server state only: the deployment flag and the user's current `ai_provider_deepseek` consent, which is given outside any outage moment (§8). The flag alone never enables it, and no request field can (RR-4 (h)). If the owner chooses to demonstrate the fallback in the hackathon, the switch is disclosed and the Meta pipeline is shown working on its own (owner contract, judging section) |
| O-3 | Data sent to DeepSeek | Synthetic, versioned scenarios only; no real patient histories (V21) | Minimised real-user text with consent in a real-use pilot (§3.1, §9, SPEC V41) | Brief v3, gated | Owner brief v3 makes Sela a real-use app (C24) and asks for consented, minimised processing (§8), so the pilot design stands. It waits for the legal review of PRC processing (B-10) and the owner's confirmation that V21's limit is lifted for the pilot. Until then DeepSeek receives synthetic data only, in every environment, as V21 says. The owner's prototype keeps its synthetic-only gate |
| O-4 | Provenance value | `live_deepseek_fallback` (V10, V20) | `live_deepseek` | Owner | Renamed everywhere (§10, SPEC V35). `live_deepseek_alternative` stays as a separate verification value, because the owner contract has no alternative verifier (O-5) |
| O-5 | DeepSeek as a verifier | DeepSeek is a fallback generator only, not an Anthropic verifier replacement; Claude stays review-only | DeepSeek may be a default-off alternative verifier for Meta output during a Claude outage (RR-9) | Brief v3 | Owner brief v3 §8 asks when DeepSeek may be the alternative verifier, so RR-9 stays: off by default (`SELA_AI_ALT_VERIFIER`), only after EV-6, never for DeepSeek's own output, never in place of a missing Claude consent, and labelled as the alternative verifier |
| O-6 | Source check of fallback output | Every fallback output is source and schema validated (V20). A Claude review is not part of that requirement (O-8) | The same guards for every generator (RR-7); an AT-1 reply is verified only when flagged (RR-8) | Owner for the V20 requirement; this ADR for the means on AT-1 replies (engineering default, D-053) | Every fallback output passes the schema, Meaning Lock and guard checks (V20). AT-1 is a brief-v3 task outside the owner's scope (O-9), and an AT-1 listening reply from DeepSeek has no source-linked claims for Meaning Lock to check. To meet V20 for it, this ADR has Claude verify every such reply against the user's turns before release (RR-8, §3). That is V20 applied to a brief-v3 task, and the means is this ADR's choice, not owner direction: the owner contract leaves Claude review to its own consent (O-8). A later ADR may replace the means with another source check that meets V20. DeepSeek is called for an AT-1 reply only when the Claude route is permitted and Sela's verifier daily spend ceiling is not yet reached (RR-2, RR-4 (i); threshold `routing.deepseek_listening_calls_without_claude_route`, §12), so no text goes to DeepSeek only for the reply to abstain; if Claude then fails, the reply abstains (RR-9: DeepSeek never verifies itself). The budget (§3.2), the consent texts (§8, §8.1) and the cost (§11: S3 rises from about $0.05 to about $0.12 with 12 extra checks; a separate threshold in §12) follow. The design documents follow through [conflict matrix](../status/CONFLICT_MATRIX.md#documentation-and-contract-follow-ups-open) row 62 |
| O-7 | Fallback label | "Meta tidak tersedia; draf ini dibuat DeepSeek Flash 1017" / "Meta was unavailable; this draft was generated by DeepSeek Flash 1017"; Meta named "Meta Muse Spark" | "Ditulis oleh DeepSeek (cadangan)"; Meta named "Muse Glimmer" | Owner for the meaning; B-7 for the model name | Every fallback output says that Meta was unavailable and that DeepSeek Flash 1017 wrote it (§10, [content style §7.9](../design/CONTENT_STYLE.md#79-ai-provenance-and-checks)). The Meta model is named as served (Muse Glimmer 30B) until the owner decides B-7 |
| O-8 | Verification of fallback drafts | Claude review optional, under its own consent ([DEEPSEEK_FALLBACK](../architecture/DEEPSEEK_FALLBACK.md) flow: without the Anthropic verification consent the user reviews the draft) | Required before release for AT-3 and AT-4 (RR-8, RR-11); since O-6, also for every DeepSeek AT-1 reply (RR-8) | Brief v3 for AT-3 and AT-4; this ADR for AT-1 replies (O-6, engineering default, D-053) | Owner brief v3 §8 requires health claims and professional summaries to be checked before release, so RR-8 stays for AT-3 and AT-4. For a DeepSeek AT-1 reply the Claude check is this ADR's means of meeting V20 for a brief-v3 task (O-6), not owner direction; brief v3 itself asks for a check of a reply only when it makes a health claim (the RR-8 flag). The user still decides by consent: without the Claude consent AT-3 and AT-4 take the manual path, and AT-1 gets no DeepSeek fallback (RR-4 (i)) |
| O-9 | Scope | Intake and brief drafting (main's V9) | Every generative task, AT-1 to AT-5 | Brief v3 | Owner brief v3 §8 names DeepSeek as Sela's fallback generator, so the scope stays, with O-1 to O-7 applied to every task |
| O-10 | Consent route | `PUT /api/v2/fallback-consent` (`granted`, `enabled`), planned for v2 (T5b) | `PUT /api/v3/me/consents/ai_provider_deepseek` ([API_SPEC_V3](../api/API_SPEC_V3.md) §3) | Both, per runtime | The v2 route stays the owner's plan for T5b; v3 uses its consent purpose. Neither accepts an enablement flag from the browser |
| O-11 | Evidence | The owner's validation matrix; Meta uptime, fallback use rate, task success, error rate, cost and latency tracked separately | EV-5, EV-7 and the §10 records | Owner | The owner's validation matrix and the prototype's test cases are ported into EV-7. Meta availability and the fallback use rate are reported per provider next to EV-4 and EV-5 |
| O-12 | Meta always first | Meta is always attempted first; DeepSeek only after a Meta availability error in that request (prototype README, V19) | The RR-14 circuit breaker let AT-1 skip Meta for 60 s after repeated outages | Owner | The breaker no longer skips Meta. It only stops the cold-start wait: Meta is still called first, and its immediate 503 (no container ready) is the availability failure that RR-4 needs (RR-14) |

A resolution marked "Owner" changes only with owner direction. Those marked "Brief v3" follow the brief and remain open to the owner (B-9, B-10, B-12). Those marked "this ADR" (the Claude check of DeepSeek AT-1 replies in O-6 and O-8) are engineering defaults under D-053: a later ADR or an evaluation result may change them, as long as V20 stays met. Nothing here is implemented: the router, the fallback and the verifier are in progress on branch `feat/sela-v3-ai`, not merged and not verified (P5, SPEC T18).
