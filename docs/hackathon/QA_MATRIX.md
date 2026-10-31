# Quality assurance, privacy/security and adversarial test matrix

Status: **planned verification protocol**. This document is not a passing test report. For each test record: exact SHA, environment, synthetic fixture, command, exit code, timestamp, reviewer and sanitized evidence URL. Reference: [SCENARIOS](SCENARIOS.md), [MIE](MIE_IMPLEMENTATION.md), [EVALUATION](EVALUATION.md).

## 1. Minimum CI & local validation commands

Use repository-specified toolchains and database environment; adapt to current project scripts after implementation. Do not claim a command passed because it appears below.

```sh
go version
go vet ./...
SELA_REQUIRE_DB_TESTS=1 go test ./internal/...
go test -race ./...
(cd web && npm ci && npm run typecheck && npm test && npm run build)
go run ./cmd/sela-eval -mode offline
go run ./cmd/sela-eval -mode test_double
# Run existing e2e scripts per docs/operations/DEVELOPMENT.md with
# isolated PostgreSQL, Redis, and labelled synthetic fixtures.
```

A completed prior CI run cannot validate a new commit. Capture live branch GitHub Actions statuses and compare workflow SHA. Do not use live API credentials in PR tests or expose env files. Provider smoke test is separately authorized and budgeted.

## 2. Traceability matrix

| Test ID | Feature / invariant | Scenarios | Test layer | Required expected result |
| --- | --- | --- | --- | --- |
| Q-001 | Consent gate: no Meta calls without consent | D, E | Unit + integration + e2e | 0 external provider calls |
| Q-002 | Storage choice for unsaved talk | A, D | DB + browser | No durable saved content |
| Q-003 | Correct quote / source version | A, B, F | AI schema + DB | Claim rejected on wrong source |
| Q-004 | Distinct Meta verifier model | A, B, E | Router + integration | Generator ID != verifier ID; truthful provenance |
| Q-005 | Rejected claim never trusted again | B | DB + retrieval | 0 authoritative reuse |
| Q-006 | Transitive stale propagation | B | DB transaction | Descendants stale, no partial update |
| Q-007 | Read-time export/share guard | B, C | API + e2e | Stale projection blocked |
| Q-008 | Recipient has only selected data | C | API + snapshot diff | Unselected and private names absent |
| Q-009 | Revoke blocks subsequent share reads | B, C | API + concurrency | No future successful recipient GET |
| Q-010 | Another owner cannot read an ID | A–D | API + DB | Uniform 404/denied with no metadata leak |
| Q-011 | Race: late AI completion after edit | B, D | Fault injection | Discard late result |
| Q-012 | Race: approval vs source edit | B, C | DB concurrent | Exactly one authoritative branch |
| Q-013 | Redis down, spend still bounded | E | Integration | PostgreSQL budget authoritative |
| Q-014 | Meta timeout, 429, schema failure | E | Fault injection | No unvalidated output / honest unavailable |
| Q-015 | Crisis gate before AI | G | API + model-spy | Safety fallback available with 0 model calls |
| Q-016 | False positives (quoted/negated risk) | F, G | AI/guard eval | Correct distinction reported |
| Q-017 | 320px, high zoom, touch/focus | F | Browser manual/axe | No blocked primary interaction |
| Q-018 | Screen reader announcement | F | Manual AT | Review actions and errors understandable |
| Q-019 | Minor band & guardian gate | D, G | Server integration | 15 blocked under current policy, teen gate enforced |
| Q-020 | Content editorial approval | C, G | API | Unapproved item never published |
| Q-021 | Quick exit on shared device | D | Browser/privacy | Correct session/preview behaviour |
| Q-022 | Source delete derived=keep/delete | B, C | DB + API | Explicit impact and safe stale share semantics |
| Q-023 | Retry/idempotency | B, C, E | API + DB | One review/share outcome per key |
| Q-024 | Auth, cookies, CSP, CORS, CSRF | All | Security checks | No bypass or credential leakage |
| Q-025 | Claim graph cycle/depth limit | B | DB/fuzz | No infinite loop; fail closed |
| Q-026 | Backup/restore consent deletion | D | Ops rehearsal | No stale grants accidentally resurrected; if fails remains release blocker |
| Q-027 | Owner-visible original history | B | DB/UI | Verbatim source preserved, correctly versioned |
| Q-028 | No claim of real care or booking | C, G | Product copy/UI | Synthetic label on every fixture/recipient |
| Q-029 | Audit/log privacy | All | Static + runtime | Metadata only; no story/token/key |
| Q-030 | Production fingerprint | All | Safe operational read | Build SHA, TLS, config/profile, release evidence |

## 3. Concurrency and transaction matrix

| Operation A | Operation B | Expected serialization/guard |
| --- | --- | --- |
| Correct C@v2 | Read frozen share using C@v1 | Read started after correction commit is denied |
| Approve summary | Edit underlying source | Version check aborts one; no stale approval |
| Revoke share | Recipient access with code | After commit, future access fails; prior copied data cannot be undone |
| Withdraw AI consent | Meta request finishes | No new unauthorized release/persistence |
| Delete original | Memory proposal completes | Proposal rejected/discarded on version mismatch |
| Retry same review key | Network retry | Exactly once effect / same response |
| Redis outage | Cost admission | Postgres rate/spend ceilings remain enforced |
| Backup restore | Prior deletion | Must honor deletion journal or remain non-release-ready |

Use controlled barriers/failpoints in integration tests, not timing sleeps as the only synchronization. Use SERIALIZABLE/locking strategy aligned with database conventions. Record race detector failures and PostgreSQL deadlock/timeouts.

## 4. Security and privacy checklists

### Authentication / authorization
- Owner session cannot use operator endpoints; staff role cannot browse journals or sealed prompts.
- Session cookie SameSite, Secure, HttpOnly and host prefixes validated in *actual* hosting environment.
- All claim/source/share endpoints enforce owner scope before query result disclosure.
- Share tokens high entropy, never logged, never transmitted in query parameters if fragment design applies; access code stored hashed, attempts rate limited and content not cached.
- Session expiry, account deletion, guest purge and logout invalidate access; test with two isolated browser contexts.

### Data and model processing
- Consent scope verified server-side before *each* model call and final release; no model inference under revoked consent.
- No user story/AI prompt in Redis, CDN logs, error telemetry or analytics.
- Encrypt claim bodies with established per-user envelope keys. No raw source text in SQL audit tables.
- Prompt injection inside original text or directory content treated as data; tool allowlists are server-enforced.
- Strict output schemas, size limits, Unicode boundaries and model IDs; never send unchecked streamed tokens.
- Provider charges limited by PostgreSQL budget gate; fake request IDs and test doubles visibly labelled.

### Web/client and operational
- CSP and strict MIME, static asset caching separate from sensitive API caching.
- No sensitive text in browser URL/history/title, preview or server access logs.
- CSRF protection for cookie-authenticated state mutations and cross-origin policy check.
- Re-authentication for irreversible account actions and one-time disclosure of backup/share codes.
- Secrets in host-managed secret storage; no production secrets in GitHub Actions logs or documentation.
- Database backup encryption, restore drills, retention and deletion journal before real-user release.

## 5. Accessibility manual script

Devices: 320px low-end Android browser, desktop keyboard-only, and at least one screen reader/browser combination. Record versions, settings and limitations.

Tasks: enter guest mode; read consent; start chat; review a proposal; identify original vs AI text; reject/correct; inspect impact; approve an updated summary; create share; revoke share; recover from error. For each task, capture completion, incorrect action, reason, focus order, visible focus, announcement text, zoom/reflow and perceived comprehension. Use Indonesian and English. Automated axe gate is necessary but not sufficient.

## 6. Fail-fast severity

- **P0 block**: cross-owner or recipient leak, silent consent bypass, unsafe patient-facing advice, bad crisis behavior, invalidation bypass, untruthful model provenance, exposure of credentials, ability to access revoked snapshots after commit.
- **P1 before broad demo**: broken main journey, incorrectly localized approval, inaccessible action, inconsistent status or frequent unexpected latency.
- **P2 defer**: visual micro-polish, unneeded admin or content breadth, token-level streaming that could weaken final safety validation.

No-Go test failure means document and fix it before public claims; do not hide failures or change expected assertions after the fact.

## 7. Test evidence log template

```yaml
test_id: Q-001
status: NOT_RUN
sha: null
environment: null
fixture_sha256: null
command: null
timestamp_utc: null
passed: 0
failed: 0
sanitized_evidence: null
reviewer: null
notes: null
```

Record all Q-IDs as NOT RUN until this new plan's checks have evidence. Existing project CI can be linked as prior background, but does not auto-fulfill a new acceptance criterion.
