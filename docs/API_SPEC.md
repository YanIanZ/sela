# Sela v2 API contract

Status: **implemented** in `internal/httpapi` and `internal/journey` (Go); behaviour tested by `go test ./...`. Authority: [SPEC](../SPEC.md) I.api, V1–V19. The inherited Next.js API is in the [v1 API archive](archive/v1/API_SPEC.md).

## Conventions

- Base path `/api/v2`, same origin as the frontend. JSON only (`Content-Type: application/json`); unknown fields and trailing data are rejected (400 `invalid_input`). Body limits: 16 KiB (32 KiB for `PUT /story`); larger bodies get 413 `too_large`.
- Success: `{"data": ...}`. Error: `{"error": {"code": "stable_code", "message": "localized text", "field": "optional"}}`. Message language: `X-Sela-Language: id|en` header, else `Accept-Language`, else Indonesian. Clients should localize by `code`.
- **Mutation guard** on every POST/PUT/PATCH/DELETE: header `X-Sela-Request: 1`, `Origin` (when sent) equal to the server origin (or `SELA_PUBLIC_ORIGIN`), and no cross-site `Sec-Fetch-Site`. Otherwise 403 `cross_origin`. No CORS is enabled.
- **Optimistic concurrency:** owner mutations send `expectedVersion` = the current `session.version`; a mismatch returns 409 `stale_version` with no side effect. Every successful mutation returns the new session view.
- **Cookies:** `sela_session` (owner; HttpOnly, SameSite=Strict, Path=/api/v2, Secure in production) and `sela_recipient` (fictional recipient; Path=/api/v2/recipient). IDs never appear in URLs. Unknown/expired sessions return 401 `session_missing` and clear the cookie.
- **Rate limits** (per client address, token bucket): session creation 30/min (burst 7), AI starts 12/min, recipient sign-in 10/min, other mutations 180/min, reads 600/min. Plus per-session AI limit (default 8 operations per 10 minutes) → 429 `rate_limited` with `Retry-After`.
- Logs record route template, status, duration and a request id only.

## Error codes

`invalid_input` 400 · `session_missing` 401 · `consent_required` 403 · `access_ended` 403 (all recipient access failures, uniform) · `cross_origin` 403 · `not_found` 404 · `stale_version` 409 · `crisis_interrupted` 409 · `approval_required` 409 · `review_incomplete` 409 · `operation_in_progress` 409 · `invalid_transition` 409 · `idempotency_conflict` 409 · `handoff_exists` 409 · `slot_unavailable` 409 · `policy_outdated` 409 · `story_required` 409 · `too_large` 413 · `unsupported_media` 415 · `contact_details_detected` 422 · `rate_limited` 429 · `adult_attestation_required` 400 · `idempotency_key_required` 400 · `ai_disabled` 503 · `capacity` 503 · `research_unavailable` 503 · `frontend_not_built` 503 · `internal` 500.

AI operation failures arrive as SSE `error` events with codes: `ai_unavailable`, `ai_timeout`, `ai_auth`, `ai_busy`, `ai_model_mismatch`, `ai_invalid_output`, `ai_ungrounded`, `stale_result`, `canceled` (with a reason such as `canceled_by_user`, `consent_withdrawn`, `story_replaced`, `crisis`, `deleted`).

## Public routes (no session)

| Route | Result |
| --- | --- |
| `GET /health` | Liveness: `{status, version, mode: "synthetic_demo", uptimeSeconds}` |
| `GET /ready` | Readiness: fixtures version, AI mode, research index loaded, frontend built, `store: "memory"`, `persistent: false` |
| `GET /catalog` | Synthetic catalogue: scenarios (with story text), fictional professionals and slots, plans, check-in options, recipient reasons, next steps, clarification templates, verified help resources with sources, policies (versioned texts) |
| `GET /ai/status` | Provider mode/model/revision (no secrets), `available`, `likelyWarm`, cold-start estimate, slots in use. Does **not** contact Modal |
| `GET /metrics` | Content-free counters and store counts |

## Owner session

| Method and route | Body | Result / rules |
| --- | --- | --- |
| `POST /sessions` | `{language, identity:{mode:"anonymous"\|"pseudonym", pseudonym?}, adultAttested:true, privacyPolicy}` | 201, sets cookie. `privacyPolicy` must equal the current privacy policy version |
| `GET /session` | — | Session view (below) |
| `PATCH /session` | `{expectedVersion, language?, identity?}` | Language changes UI only; quotes stay in the user's words; approval unaffected |
| `DELETE /session` | — | Purges session, shares, recipient access and operations; clears cookie |
| `PUT /consents/{ai_processing\|storage}` | `{expectedVersion, granted, policyVersion}` | Independent decisions. Withdrawing AI cancels running AI operations. Storage grant → retention 24 h; withdraw → session-only retention |
| `PUT /story` | `{expectedVersion, mode:"scenario", scenarioId}` or `{expectedVersion, mode:"practice", text}` | Safety gate first. Crisis → `{safety:"crisis", session}` with story not stored and session interrupted. Practice text 20–3000 chars; contact details → 422 |
| `POST /ledger/draft` | `{expectedVersion}` | 202 `{operation, session}`. Requires AI consent, no crisis, no running draft |
| `GET /operations/{id}/events` | — | `text/event-stream` (below); own session only |
| `DELETE /operations/{id}` | — | Cancel; idempotent |
| `POST /ledger/items` | `{expectedVersion, segmentId, kind}` | Manual item from one whole segment (confirmed, `manual`) |
| `PATCH /ledger/items/{id}` | `{expectedVersion, action:"confirm"\|"use_quote"\|"edit"\|"remove"\|"set_kind", wording?, kind?}` | Any change → new ledger version, approval cleared. `edit` passes the safety gate |
| `POST /ledger/clarifications` | `{expectedVersion, field, answer?}` or `{expectedVersion, field, skip:true}` | Answer becomes a new clarification segment and a confirmed item; skip is never asked again |
| `POST /ledger/approval` | `{expectedVersion, ledgerVersion}` | Every item must be confirmed; binds approval to that exact version and content hash |
| `PUT /preferences` | `{expectedVersion, language:id\|en\|either, format:online\|in_person\|either, budget:free_only\|low_or_free\|any, availability:weekday_morning\|weekday_daytime\|weekday_evening\|weekend\|any, barrier:cost\|schedule\|how_to_start\|privacy\|not_ready\|none}` | Stored choices only |
| `GET /next-steps` | — | `{exact[], near[], steps[], handoffFirst, barrier}`; each option has `reasons[{criterion, match, value}]` and available `slots[]`; catalogue order, no ranking |
| `POST /opening/template` | `{expectedVersion, recipient:"professional"\|"trusted_person"}` | Deterministic lines (`manual`) from the approved brief |
| `POST /opening/draft` | same | 202 `{operation, session}`; AI lines validated against the approved items |
| `PUT /opening` | `{expectedVersion, lines:[1..3 strings]}` | User-written lines; safety gate |
| `POST /opening/practice` | `{attempt}` | `{coverage:{mentioned[], notMentioned[]}, safety}`; attempt not stored |
| `POST /handoffs/preview` | `{professionalId, slotId, fields[]}` | The exact projection a share would store; creates nothing |
| `POST /handoffs` | Header `Idempotency-Key` (16–64 `[A-Za-z0-9_-]`); `{expectedVersion, ledgerVersion, professionalId, slotId, fields[], durationHours:1\|24\|72, shareConsent:true, sharingPolicy}` | 201 `{handoff, token, code, replay:false, session}`; same key+payload → 200 same receipt with `replay:true`; changed payload → 409. `fields` must include `items`; optional `quotes, unknowns, opening, preferences, display_name, age_range, language` |
| `DELETE /handoffs/{id}/share` | optional `{expectedVersion}` | Revoke; repeat is safe |
| `POST /follow-up/check-ins` | `{expectedVersion, handoffId, optionId}` | Only after `completed`; once per option per handoff; not after a crisis route |

### Session view (abridged)

`{version, synthetic:true, language, identity, createdAt, expiresAt, idleExpiresAt, retention:"session"|"day", consents{ai_processing?, storage?: {granted, policyVersion, decidedAt, revision}}, policies{privacy, ai_processing, storage, sharing}, safety{level:"none"|"elevated"|"crisis", interrupted, thirdParty?}, stage, story?{version, mode, scenarioId?, segments[{id,text,origin}]}, ledger?{version, items[], unknowns[], skipped[], clarification?, provenance, model?, approved?{version,hash,at}, reviewed, rejected{reason:count}}, preferences?, opening?{lines[{text,basedOn[]}], provenance, briefVersion, userEdited}, handoffs[], checkIns[], ops[], ai, fixturesVersion}`

Ledger item: `{id, kind, segmentId, quote, start, end, wording, wordingOrigin:"model"|"quote"|"user", qualifiers{negated, subject:"self"|"other", time, certainty}, status:"unreviewed"|"confirmed", provenance, notes[]}`. `quote` is the user's own words (byte offsets into the segment); `wording` is Sela's interpretation or the user's edit.

Handoff receipt: `{id, version, professionalId, slotId, briefVersion, fields[], projection, status, share:"active"|"revoked"|"expired"|"locked", shareReason?, createdAt, expiresAt, timeline[{kind, at, actor}], clarifyReason?, declineReason?, planId?, replacesId?, opened, careReceived:false}`.

### Server-sent events

`GET /operations/{id}/events` replays from `Last-Event-ID` (or `?after=`), sends a keep-alive comment every 15 s and ends after `done` or `error`.

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
| `POST /recipient/sessions` | `{token, code}` | 201 sets `sela_recipient`; returns the recipient handoff. Any failure → 403 `access_ended`. Five wrong codes lock the share |
| `GET /recipient/handoff` | — | `{id, version, professionalId, status, projection, expiresAt, timeline, plans[], synthetic:true, ...}` — only the immutable projection of selected fields |
| `POST /recipient/handoff/actions` | `{expectedVersion, action:"acknowledge"\|"clarify"\|"decline"\|"complete", reasonId?, planId?}` | `requested → acknowledged\|clarification_requested\|declined`; `acknowledged → completed\|clarification_requested\|declined`. `complete` needs a plan owned by that fictional professional. Blocked after the owner's crisis route |
| `DELETE /recipient/session` | — | Signs out |

Revocation, expiry, lockout, owner deletion and supersession all end access immediately and return the same `access_ended` error.

## Reviewer research corpus (no session access)

| Route | Result |
| --- | --- |
| `GET /research/manifest` | Corpus manifest: freeze/cutoff dates, index version, counts, discrepancies |
| `GET /research/search?q=&theme=&status=&limit=` | Ranked metadata/annotation matches (q ≤ 200 chars) |
| `GET /research/records/{id}` | One record (metadata, our annotations, licence/access, evidence locators) |

503 `research_unavailable` when the knowledge base file is absent.

## State machines

Ledger: `draft (unreviewed items) → reviewed → approved(version v)`; any change → `draft(v+1)`. Approval is required for Opening Practice drafts and handoffs; sharing references the immutable snapshot of `v`.

Handoff: `requested → acknowledged → completed`; `requested|acknowledged → clarification_requested | declined`. Share access: `active → revoked | expired | locked`, independent of handoff status. A clarification is answered with a new version, approval and share (the new share supersedes the old one). `careReceived` is always false in this demo.
