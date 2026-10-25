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
| `PUT /api/v2/ai-consent` | `granted`, `expectedVersion` | Explicit decision; withdrawal cancels/discards inference |
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
