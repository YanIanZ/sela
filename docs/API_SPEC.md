# Sela v2 API contract

Status: proposed contract; none of these `/api/v2` endpoints are implemented. Authority: [SPEC](../SPEC.md) I.api, V1–V12. The inherited Next.js API is documented in the [v1 API archive](archive/v1/API_SPEC.md).

## Common rules

JSON only; bounded request/response sizes. Language is `id` or `en`; a locale never selects an emergency jurisdiction. Session cookie establishes an isolated demo session. Require a same-origin mutation guard and enforce an active session for every protected resource. Never accept a client role/profile as proof of real professional identity.

Use `{ "data": ... }` on success and `{ "error": { "code": "stable_code", "message": "localized text" } }` on errors. Typical errors: 400 invalid input/fixture, 401 absent/expired session, 403 forbidden shared access/origin, 404 unavailable session-owned resource, 409 stale version/invalid transition/changed idempotency payload, 413 too large, 429 admission limit, 503 model unavailable. Do not disclose foreign session resources.

Every mutable record has a version. Mutations send `expectedVersion`; mismatch returns 409 without side effects. Model operations use an operation ID and check session/version/consent again before committing results. Simulated request creation requires an `Idempotency-Key`, scoped to session + route; repeat payload returns the same receipt, changed payload conflicts. Key retention is bounded by session lifetime.

## Planned routes

| Method and route | Input | Result / guard |
| --- | --- | --- |
| `POST /api/v2/sessions` | `scenarioId`, `language` | New bounded session, `synthetic: true`, expiry and opaque cookie |
| `GET /api/v2/session` | Cookie | User-visible current state and version; no credentials |
| `DELETE /api/v2/session` | Mutation guard | Revoke all session shares, cancel operations, clear only this session |
| `GET /api/v2/catalog` | Language and fixed preference filters | Fictional scenarios/professionals/slots; no session content |
| `PUT /api/v2/ai-consent` | `granted`, `expectedVersion` | Explicit Meta processing decision; withdrawal cancels/discards inference |
| `PUT /api/v2/fallback-consent` | `granted`, `enabled`, `expectedVersion` | Independent DeepSeek consent and opt-in; stale or revoked results canceled; planned only |
| `PUT /api/v2/fallback-consent` | `granted`, `enabled`, `expectedVersion` | Independent DeepSeek consent and opt-in; stale/revoked results canceled; **planned only** |
| `PUT /api/v2/verification-consent` | `granted`, `expectedVersion` | Independent Anthropic consent for synthetic source excerpts; withdrawal cancels/invalidates verification; **planned only** |
| `POST /api/v2/brief/verify` | `expectedVersion` (no free-text from client) | Optional Claude source-fidelity report on current fixture/brief; never advances approval/handoff; **planned only** |
| `GET /api/v2/brief/verification` | Cookie | Read current session + brief version's report with provider/status; expired/stale reports omitted; **planned only** |
| `POST /api/v2/brief/draft` | `expectedVersion` | Crisis gate first; consent then Meta; validated draft or unavailable state |
| `PUT /api/v2/brief` | Allowed fixture/evidence/question selections, `expectedVersion` | Manual edit/create; increment version and clear approval; no arbitrary patient input |
| `POST /api/v2/brief/approval` | `expectedVersion` | Exact-version approval; unsupported content rejected |
| `POST /api/v2/handoffs` | Approved brief version, fixture recipient/slot, selected profile fields, explicit share consent | Immutable approved snapshot and simulated receipt; idempotent; crisis/slot checks apply |
| `GET /api/v2/professional/inbox` | Active demo session | Only this session's unrevoked shared snapshots, marked demo |
| `POST /api/v2/handoffs/{id}/acknowledge` | `expectedVersion` | Scripted fictional receipt transition |
| `POST /api/v2/handoffs/{id}/complete` | Allowed fixture plan ID, `expectedVersion` | Scripted consultation completion; plan must belong to scenario/recipient |
| `DELETE /api/v2/handoffs/{id}/share` | `expectedVersion` | Revoke shared projection, including completed requests; repeat revoke is safe |
| `GET /api/v2/follow-up` | Session | Fixture plan/timeline after valid completion; no invented treatment |
| `POST /api/v2/follow-up/check-ins` | Seeded check-in option ID, `expectedVersion` | Synthetic progress entry, once per fixture occurrence |
| `GET /api/v2/health` | None | Process readiness and demo mode; no secrets or raw provider errors |

## State and evidence

Brief: `draft → approved`; editing returns it to `draft`. Unavailability never creates approval. Sharing is separate from approval and references the exact approved snapshot.

Handoff: `simulated_requested → simulated_acknowledged → simulated_completed`. Share access is independent: `active → revoked`; reset/expiry invalidates all access regardless of handoff status. A revoked/expired share cannot be acknowledged/completed or queried by the professional perspective. A new share requires a new explicit consent action; it cannot revive an old revoked snapshot.

A snapshot records session generation, scenario/fixture version, brief version, selected recipient/slot, selected synthetic identity fields, consent timestamp, provenance and request version. No full private conversation or unrelated journal is included. The professional view never broadens the snapshot through lookup endpoints.

AI provenance: `live_meta` includes verified model ID and operation ID; `recorded_demo` includes recording fixture version; `manual` has no provider claim. No secret, prompt, raw story or clinical note enters logs. See [validation](VALIDATION.md) for edge-case checks. Endpoint names are the initial contract; changes require an explicit spec/doc update before clients depend on them.

## Second provider contract (planned only)

The optional Anthropic verifier is **not** a fallback generator. A provider-specific verification consent must be granted **after the deterministic crisis gate**, distinct from Meta AI processing, user approval and share consent. Its own report is version-bound. Successful JSON/schema/model review is not a clinical safety verdict or an authorization to share. An unavailable/contradictory result displays an explicit review state and a user-controlled manual path. See [Claude verifier](CLAUDE_VERIFIER.md). No v2 route currently exists.

## Optional DeepSeek fallback contract (planned only)

The existing `POST /api/v2/brief/draft` always first attempts Meta where authorized. Only recognized Meta availability errors can trigger a separately consented, default-OFF DeepSeek `deepseek-flash` request, using session-owned synthetic evidence IDs rather than arbitrary client input. Never switch for policy refusal, invalid claims, source mismatch, unsafe output, denied consent, crisis fixture or request cancellation. Responses distinguish `live_meta` vs `live_deepseek_fallback` with immutable provider/model metadata. Fallback result requires the same evidence validation, brief reapproval and later share consent, and must be discarded if versions/consents have changed. No v2 routes yet exist. See [DeepSeek fallback](DEEPSEEK_FALLBACK.md).

## Optional DeepSeek fallback contract (planned only)

The existing `POST /api/v2/brief/draft` attempts Meta first. Only explicit, recognized Meta availability errors may trigger separately consented DeepSeek `deepseek-flash` using session-owned synthetic evidence. Never switch for policy refusal, invalid claims, source mismatch, unsafe output, denied consent, crisis fixture or request cancellation. Responses distinguish `live_meta` vs `live_deepseek_fallback` with immutable provider/model metadata. Fallback requires identical source checks, human approval and recipient share consent; stale session/version/consent invalidates the response. No v2 routes yet exist. See [DeepSeek fallback](DEEPSEEK_FALLBACK.md).
