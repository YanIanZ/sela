# Sela v2 API contract

Status: **implemented** in `internal/httpapi` and `internal/journey` (Go); behaviour tested by `go test ./...`. Authority: [SPEC](../SPEC.md) I.api, V1–V25. The inherited Next.js API is in the [v1 API archive](archive/v1/API_SPEC.md).

## Conventions

- Base path `/api/v2`, same origin as the frontend. JSON only (`Content-Type: application/json`); unknown fields, keys that match a field only case-insensitively, duplicate keys, nesting deeper than 32 and trailing data are rejected (400 `invalid_input`). Body limits: 16 KiB (32 KiB for `PUT /story`); any body over the limit, trailing data included, gets 413 `too_large`. A wrong method on a real route gets 405 `method_not_allowed` with an `Allow` header; any other path under `/api` (in any letter case) is a JSON 404, never the frontend.
- Success: `{"data": ...}`. Error: `{"error": {"code": "stable_code", "message": "localized text", "field": "optional"}}`. Message language: `X-Sela-Language: id|en` header, else `Accept-Language`, else Indonesian. Clients should localize by `code`.
- **Mutation guard** on every POST/PUT/PATCH/DELETE: header `X-Sela-Request: 1`, `Origin` (when sent) equal to the server origin (or `SELA_PUBLIC_ORIGIN`), and no cross-site `Sec-Fetch-Site`. Otherwise 403 `cross_origin`. No CORS is enabled.
- **Optimistic concurrency:** owner mutations send `expectedVersion` = the current `session.version`; a mismatch returns 409 `stale_version` with no side effect. Every successful mutation returns the new session view.
- **Cookies:** `sela_session` (owner; HttpOnly, SameSite=Strict, Path=/api/v2, Secure in production) and `sela_recipient` (fictional recipient; Path=/api/v2/recipient). IDs never appear in URLs. Unknown/expired sessions return 401 `session_missing` and clear the cookie.
- **Rate limits** (per client address, IPv6 grouped by /64; the limiter evicts its least recently used keys and never fails closed): session creation 30/min (burst 7), AI starts 12/min, recipient sign-in 10/min, other mutations 180/min, reads 600/min. At most `SELA_MAX_SESSIONS_PER_CLIENT` (default 50) live sessions per client address. Plus a per-session AI limit (default 8 operations per 10 minutes). All → 429 `rate_limited` with `Retry-After`.
- **Per-session caps** → 409 `limit_reached`: 24 ledger items, 60 story segments / 5000 story characters (clarification answers included), 20 handoffs.
- Logs record route template, status, duration and a request id only.

## Error codes

`invalid_input` 400 · `session_missing` 401 · `forbidden` 401 (metrics without the operator token) · `consent_required` 403 · `access_ended` 403 (all recipient access failures, uniform) · `cross_origin` 403 · `not_found` 404 · `method_not_allowed` 405 · `stale_version` 409 · `crisis_interrupted` 409 · `approval_required` 409 · `review_incomplete` 409 · `operation_in_progress` 409 · `invalid_transition` 409 · `idempotency_conflict` 409 · `handoff_exists` 409 · `slot_unavailable` 409 · `policy_outdated` 409 · `story_required` 409 · `limit_reached` 409 · `too_large` 413 · `unsupported_media` 415 · `contact_details_detected` 422 · `rate_limited` 429 · `adult_attestation_required` 400 · `idempotency_key_required` 400 · `ai_disabled` 503 · `capacity` 503 · `research_unavailable` 503 · `frontend_not_built` 503 · `internal` 500.

AI operation failures arrive as SSE `error` events with codes: `ai_unavailable`, `ai_timeout`, `ai_auth`, `ai_busy`, `ai_model_mismatch`, `ai_invalid_output`, `ai_ungrounded`, `stale_result`, `canceled` (with a reason such as `canceled_by_user`, `consent_withdrawn`, `story_replaced`, `crisis`, `deleted`, `session_ended`). On server shutdown an open stream ends with `error {code:"ai_unavailable", reason:"server_stopping"}`.

## Public routes (no session)

| Route | Result |
| --- | --- |
| `GET /health` | Liveness: `{status, version, mode: "synthetic_demo", uptimeSeconds}` |
| `GET /ready` | Readiness: fixtures version, AI mode, research index loaded, frontend built, `store: "memory"`, `persistent: false` |
| `GET /catalog` | Synthetic catalogue: scenarios (with story text), fictional professionals and slots, plans, check-in options, recipient reasons, next steps, clarification templates, verified help resources with sources, policies (versioned texts) |
| `GET /ai/status` | Provider mode/model/revision (no secrets), `available`, `likelyWarm`, cold-start estimate, slots in use. Does **not** contact Modal |
| `GET /metrics` | Content-free counters and store counts. Operator-only: requires `Authorization: Bearer <SELA_METRICS_TOKEN>` (else 401 `forbidden`); when the variable is unset the route does not exist (404 for every method) |

## Owner session

| Method and route | Body | Result / rules |
| --- | --- | --- |
| `POST /sessions` | `{language, identity:{mode:"anonymous"\|"pseudonym", pseudonym?}, adultAttested:true, privacyPolicy}` | 201, sets cookie. `privacyPolicy` must equal the current privacy policy version. A pseudonym passes the safety gate first: crisis language creates the session already interrupted, anonymous, without storing the pseudonym |
| `GET /session` | — | Session view (below) |
| `PATCH /session` | `{expectedVersion, language?, identity?}` | Language changes UI only; quotes stay in the user's words; approval unaffected. A crisis pseudonym interrupts the session and nothing in the request is applied |
| `DELETE /session` | — | Purges session, shares, recipient access and operations; clears cookie |
| `PUT /consents/{ai_processing\|storage}` | `{expectedVersion, granted, policyVersion}` | Independent decisions. Withdrawing AI cancels running AI operations. Storage grant → retention 24 h from the first grant (re-granting never extends it); withdraw → session-only retention, never later than before; a decline never extends a session |
| `PUT /story` | `{expectedVersion, mode:"scenario", scenarioId}` or `{expectedVersion, mode:"practice", text}` | Safety gate before any other validation, on any text in any mode. Crisis → `{safety:"crisis", session}` with story not stored and session interrupted. An interrupted session refuses new stories (409 `crisis_interrupted`). Practice text 20–3000 chars; contact details → 422 |
| `POST /ledger/draft` | `{expectedVersion}` | 202 `{operation, session}`. Requires AI consent, no crisis, no running draft |
| `GET /operations/{id}/events` | — | `text/event-stream` (below); own session only |
| `DELETE /operations/{id}` | — | Cancel; idempotent |
| `POST /ledger/items` | `{expectedVersion, segmentId, kind}` | Manual item from one whole segment (confirmed, `manual`) |
| `PATCH /ledger/items/{id}` | `{expectedVersion, action:"confirm"\|"use_quote"\|"edit"\|"remove"\|"set_kind", wording?, kind?}` | Any change → new ledger version, approval cleared. Any wording passes the safety gate before other validation |
| `POST /ledger/clarifications` | `{expectedVersion, field, answer?}` or `{expectedVersion, field, skip:true}` | Only for a field that is still unknown and not yet answered or skipped (else 409 `invalid_transition`). The answer passes the safety gate first, then becomes a new clarification segment and a confirmed item; a skipped or answered field is never asked again, also after AI drafts |
| `POST /ledger/approval` | `{expectedVersion, ledgerVersion}` | Every item must be confirmed; binds approval to that exact version and content hash |
| `PUT /preferences` | `{expectedVersion, language:id\|en\|either, format:online\|in_person\|either, budget:free_only\|low_or_free\|any, availability:weekday_morning\|weekday_daytime\|weekday_evening\|weekend\|any, barrier:cost\|schedule\|how_to_start\|privacy\|not_ready\|none}` | Stored choices only |
| `GET /next-steps` | — | `{exact[], near[], steps[], handoffFirst, barrier}`; each option has `reasons[{criterion, match, value}]` and available `slots[]`; catalogue order, no ranking |
| `POST /opening/template` | `{expectedVersion, recipient:"professional"\|"trusted_person"}` | Deterministic lines from the approved brief; provenance is `manual`, or the AI provenance of the item wording they carry |
| `POST /opening/draft` | same | 202 `{operation, session}`; AI lines validated against the approved items |
| `PUT /opening` | `{expectedVersion, lines:[1..3 strings]}` | User-written lines; safety gate |
| `POST /opening/practice` | `{attempt}` | `{coverage:{mentioned[], notMentioned[]}, safety}`; attempt not stored |
| `POST /handoffs/preview` | `{professionalId, slotId, fields[]}` | The exact projection a share would store; creates nothing |
| `POST /handoffs` | Header `Idempotency-Key` (16–64 `[A-Za-z0-9_-]`); `{expectedVersion, ledgerVersion, professionalId, slotId, fields[], durationHours:1\|24\|72, shareConsent:true, sharingPolicy}` | 201 `{handoff, token, code, replay:false, session}`; same key+payload → 200 same receipt with `replay:true`; changed payload → 409. `fields` must include `items`; optional `quotes, unknowns, opening, preferences, display_name, age_range, language` |
| `DELETE /handoffs/{id}/share` | none, `{}` or `{expectedVersion}` (ignored) | Revoke is version-independent and idempotent; revoking an ended share returns the current view |
| `POST /follow-up/check-ins` | `{expectedVersion, handoffId, optionId}` | Only after `completed`; once per option per handoff; not after a crisis route |

### Session view (abridged)

`{version, synthetic:true, language, identity, createdAt, expiresAt, idleExpiresAt, retention:"session"|"day", consents{ai_processing?, storage?: {granted, policyVersion, decidedAt, revision}}, policies{privacy, ai_processing, storage, sharing}, safety{level:"none"|"elevated"|"crisis", interrupted, thirdParty?}, stage, story?{version, mode, scenarioId?, segments[{id,text,origin}]}, ledger?{version, items[], unknowns[], skipped[], clarification?, provenance, model?, approved?{version,hash,at}, reviewed, rejected{reason:count}}, preferences?, opening?{lines[{text,basedOn[]}], provenance, briefVersion, userEdited}, handoffs[], checkIns[], ops[], ai, fixturesVersion}`

Ledger item: `{id, kind, segmentId, quote, start, end, wording, wordingOrigin:"model"|"quote"|"user", qualifiers{negated, subject:"self"|"other", time, certainty}, status:"unreviewed"|"confirmed", provenance, notes[]}`. `quote` is the user's own words (byte offsets into the segment); `wording` is Sela's interpretation or the user's edit.

Handoff receipt: `{id, version, professionalId, slotId, briefVersion, fields[], projection, status, share:"active"|"revoked"|"expired"|"locked", shareReason?, createdAt, expiresAt, timeline[{kind, at, actor}], clarifyReason?, declineReason?, planId?, replacesId?, opened, careReceived:false}`. `expiresAt` is the effective expiry, min(share expiry, session expiry). Projection: `{synthetic, language, briefVersion, items[], unknowns?, opening?, openingProvenance?, openingUserEdited, preferences?, displayName?, ageRange?, preferredLanguage?, slotId, provenance}`; each item carries its quote only when the `quotes` field was shared.

`idleExpiresAt` is the effective idle deadline: while a share is active the owner's idle timeout is suspended up to that share's expiry (the absolute `expiresAt` still applies).

### Server-sent events

`GET /operations/{id}/events` replays from `Last-Event-ID` (or `?after=`), sends a keep-alive comment every 15 s and ends after `done` or `error`. Event ids are monotonic. After a crisis, story replacement, deletion, expiry or eviction the operation's log keeps only a content-free terminal event; AI results are discarded (`stale_result`) when the user approved, skipped, edited or answered during a ledger draft, or changed the opening during an opening draft.

| Event | Data |
| --- | --- |
| `status` | `{state:"queued"\|"warming"\|"running"\|"validating", coldStartEstimateMs?, elapsedMs?, attempt?}`; `attempt` is present on each cold-start retry |
| `item` | Preview of a claim that **already passed Meaning Lock**: `{preview:true, index, kind, quote, wording, fromQuote, segmentId}`; not yet committed |
| `progress` | `{chunks, previewed}` |
| `done` | `{accepted, rejected{reason:count}, provenance, model, latencyMs, firstTokenMs, ledgerVersion, sessionVersion}` (ledger) or `{lines, ...}` (opening) |
| `error` | `{code, reason?}` |

The committed result is only what `GET /session` returns after `done`.

## Fictional recipient

A share yields a link `/r#t=<token>` (fragment: never sent to the server or in Referer) and a separate access code. The recipient principal requires both.

| Method and route | Body | Result / rules |
| --- | --- | --- |
| `POST /recipient/sessions` | `{token, code}` | 201 sets `sela_recipient`; returns the recipient handoff. Any failure → 403 `access_ended`. Five wrong codes lock the share. At most 3 live principals per handoff; a fourth sign-in ends the oldest |
| `GET /recipient/handoff` | — | `{id, version, professionalId, status, projection, expiresAt, timeline, plans[], synthetic:true, ...}` — only the immutable projection of selected fields |
| `POST /recipient/handoff/actions` | `{expectedVersion, action:"acknowledge"\|"clarify"\|"decline"\|"complete", reasonId?, planId?}` | `requested → acknowledged\|clarification_requested\|declined`; `acknowledged → completed\|clarification_requested\|declined`. `complete` needs a plan owned by that fictional professional. Blocked after the owner's crisis route |
| `DELETE /recipient/session` | — | Signs out |

Revocation, expiry, lockout, owner deletion and supersession all end access immediately and return the same `access_ended` error.

## Reviewer research corpus (no session access)

| Route | Result |
| --- | --- |
| `GET /research/manifest` | Corpus manifest: freeze/cutoff dates, index version, counts, discrepancies |
| `GET /research/search?q=&theme=&status=&limit=` | Ranked metadata/annotation matches (q ≤ 200 characters, counted as Unicode code points) |
| `GET /research/records/{id}` | One record (metadata, our annotations, licence/access, evidence locators) |

503 `research_unavailable` when the knowledge base file is absent.

## State machines

Ledger: `draft (unreviewed items) → reviewed → approved(version v)`; any change → `draft(v+1)`. Approval is required for Opening Practice drafts and handoffs; sharing references the immutable snapshot of `v`.

Handoff: `requested → acknowledged → completed`; `requested|acknowledged → clarification_requested | declined`. Share access: `active → revoked | expired | locked`, independent of handoff status. A clarification is answered with a new version, approval and share (the new share supersedes the old one). `careReceived` is always false in this demo.

## Second provider contract (planned only)

The optional Anthropic verifier is **not** a fallback generator. A provider-specific verification consent must be granted **after the deterministic crisis gate**, distinct from Meta AI processing, user approval and share consent. Its own report is version-bound. Successful JSON/schema/model review is not a clinical safety verdict or an authorization to share. An unavailable/contradictory result displays an explicit review state and a user-controlled manual path. See [Claude verifier](CLAUDE_VERIFIER.md). No v2 route currently exists; the planned routes below are from the [verifier contract](CLAUDE_VERIFIER.md) and are not implemented.

| Planned route | Body | Rule |
| --- | --- | --- |
| `PUT /api/v2/verification-consent` | `granted`, `expectedVersion` | Independent Anthropic consent for synthetic source excerpts; withdrawal cancels/invalidates verification; **planned only** |
| `POST /api/v2/brief/verify` | `expectedVersion` (no free-text from client) | Optional Claude source-fidelity report on current fixture/brief; never advances approval/handoff; **planned only** |
| `GET /api/v2/brief/verification` | Cookie | Read current session + brief version's report with provider/status; expired/stale reports omitted; **planned only** |

## Optional DeepSeek fallback contract (planned only)

The existing `POST /api/v2/brief/draft` always first attempts Meta where authorized. Only recognized Meta availability errors can trigger a separately consented, default-OFF DeepSeek `deepseek-flash` request, using session-owned synthetic evidence IDs rather than arbitrary client input. Never switch for policy refusal, invalid claims, source mismatch, unsafe output, denied consent, crisis fixture or request cancellation. Responses distinguish `live_meta` vs `live_deepseek_fallback` with immutable provider/model metadata. Fallback result requires the same evidence validation, brief reapproval and later share consent, and must be discarded if versions/consents have changed. No v2 routes yet exist. See [DeepSeek fallback](DEEPSEEK_FALLBACK.md).

Reconciliation with the implemented API: the Meta draft route is `POST /ledger/draft` (`POST /api/v2/brief/draft` is the earlier planned name). It calls only the configured Meta model; the v2 runtime has no DeepSeek path or fallback consent. The fallback exists only as the standalone, default-OFF prototype in [prototypes/generationfallback](../prototypes/generationfallback/README.md) (T5b open). The planned consent route from the [fallback contract](DEEPSEEK_FALLBACK.md) is not implemented:

| Planned route | Body | Rule |
| --- | --- | --- |
| `PUT /api/v2/fallback-consent` | `granted`, `enabled`, `expectedVersion` | Independent DeepSeek consent and opt-in; stale or revoked results canceled; planned only |
| `PUT /api/v2/fallback-consent` | `granted`, `enabled`, `expectedVersion` | Independent DeepSeek consent and opt-in; stale/revoked results canceled; **planned only** |

## Optional DeepSeek fallback contract (planned only)

The existing `POST /api/v2/brief/draft` attempts Meta first. Only explicit, recognized Meta availability errors may trigger separately consented DeepSeek `deepseek-flash` using session-owned synthetic evidence. Never switch for policy refusal, invalid claims, source mismatch, unsafe output, denied consent, crisis fixture or request cancellation. Responses distinguish `live_meta` vs `live_deepseek_fallback` with immutable provider/model metadata. Fallback requires identical source checks, human approval and recipient share consent; stale session/version/consent invalidates the response. No v2 routes yet exist. See [DeepSeek fallback](DEEPSEEK_FALLBACK.md).
