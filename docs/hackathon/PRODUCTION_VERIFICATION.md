# Production verification & controlled hackathon demonstration

Status: **production hosting confirmed by the owner at https://sela.vertalt.my.id and answering read-only checks (2026-11-01); deployed build SHA still unverified**. This is a **verification runbook**, not a new hosting build/deployment request. Do not claim production readiness or real-user safety from the presence of a URL.

Current main baseline `docs/status/IMPLEMENTATION_STATUS.md` predates the owner's update and says “no hosted deployment”. Its claim should be interpreted as **previously observed**, now requiring reconciliation. B-3 (hosting) should be reclassified on evidence; B-1 (Modal live provider), B-10 (privacy/legal) and B-15 (region) are independent gates. The owner-reported host does not resolve them automatically.

## 1. Verification scope and non-negotiables

- **No destructive or invasive tests against production.** Do not seed fictional records into a real-user database, cause model failures on real users, run fuzzing/probes, drain workloads or change hosting config from this documentation PR.
- Only perform safe read-only host checks with owner's approval and only publish sanitized metadata.
- If the hosted production app has real-user sessions or sensitive data, all write-side demos, adversarial tests, E2E and failure injection must run in **isolated staging, local containers or a controlled synthetic-only demo tenant**.
- Never paste private domain names if not intended for publication, tokens, secrets, raw request headers, cookies, user records, IPs, logs or provider billing identifiers into repo.
- Any needed production mutation has a separate approval, change plan, backup, rollback, downtime notification if necessary, and tenant isolation assessment.
- “Verified hosting” and “ready for controlled pilot/public release” are separate decisions.

## 2. Deployment fingerprint template

Owner/SRE fills the table only from actual evidence, with secrets redacted.

| Field | Expected source | Status / evidence |
| --- | --- | --- |
| Public URL / hostname | owner confirms exact published endpoint | **https://sela.vertalt.my.id** (owner-confirmed 2026-11-01) |
| Deployment provider | owner hosting dashboard | UNKNOWN |
| Release commit SHA / container digest | image labels, release dashboard | UNKNOWN — `version` is the literal `prod`; expose the SHA (roadmap S-001) |
| Build timestamp and health status | deployment dashboard | `/api/v2/health` → `mode: production`, `status: ok`, `version: prod` (read-only, 2026-11-01); build timestamp UNKNOWN |
| TLS certificate, HSTS and redirects | authorized safe check/browser | HTTPS 200; HSTS `max-age=31536000; includeSubDomains` present **twice** (proxy and app; roadmap S-002) |
| CSP / cache-control for private responses | read-only header inspection | Strict CSP (`default-src 'self'`, no inline script, `frame-ancestors 'none'`), `X-Frame-Options: DENY`, `nosniff` (duplicated) |
| Production profile (`SELA_ENV`) | redacted config key names, not values | NOT CHECKED |
| AI generator/provider/model ID | true observability/provenance, no prompt | `/api/v2/ready` reports `ai: meta_api`; model ID NOT CHECKED |
| Verifier provider/model (must differ) | true Meta provenance | NOT CHECKED |
| Regional storage / backup / KEK location | vendor configuration record | NOT CHECKED |
| PostgreSQL backup/restore status | operations evidence | `/api/v2/ready` reports `database: ok`, `schema: current`; backup/restore NOT CHECKED |
| Redis role and fallback | operations evidence | NOT CHECKED |
| Safety and consent policy versions | committed code/published UI | NOT CHECKED |
| Production users / sensitive data present? | owner privacy assessment, yes/no only | UNKNOWN |
| Incident contact and rollback owner | operator runbook | UNKNOWN |

If deployment SHA != repository submission SHA, document exactly which build is shown. A healthy HTTP 200 only establishes availability of a route, not that the system or Meta call actually works.

## 3. Low-risk read-only confirmation

Permitted *after owner review*:
1. Visit a public landing page without submitting personal information; check it renders and correctly identifies environment/build where the public UI exposes a safe build marker.
2. Check HTTPS, expected redirects, HSTS and CSP via a normal browser/security header inspection. Do not send payloads or bypass challenge mechanisms.
3. Verify static assets load at desktop/320px and ID/EN selection works without creating accounts.
4. Cross-check release SHA from provider dashboard against commit and known SBOM/deployment provenance; do not log secrets.
5. Check health/monitoring dashboards for errors and request budgets **without opening real user requests**.
6. Check operator-provided anonymized last backup timestamp and restore rehearsal reference. Do not initiate restore in production.
7. If operator has existing *content-free* Meta inference metrics, confirm observed model IDs, latency/failure counters and provider availability; do not claim a live call happened solely from config values.

Explicitly **NOT** permitted under “read-only”: making a real person submit symptoms, injection testing, attempts to access admin endpoints, raw log dumps, PII scraping, synthetic fixture creation in real-user DB, altering backend env vars or trying to force a 500. Such actions require a separate authorized controlled environment and change procedure.

## 4. Controlled demonstration environment (not a redeploy request)

Preferred: use already available isolated staging/synthetic-only tenant if one exists. If none exists, use local Docker + existing API gateway for owner-screen-shared demonstration, or record the actual execution locally and disclose it. Do not create an insecure Internet-facing clone or enable `SELA_LLM_MODE=meta_api` in a production profile in violation of current D-063.

Configuration acceptance:
- Unique demo database and per-environment encryption keys; fake user records only.
- Trusted TLS and correct cookies, restricted read/write origin, no external analytics capturing text, explicit session teardown.
- Test fixture gate rejecting non-synthetic payloads where feasible, and clear screen/banner “Skenario simulasi; bukan konsultasi sungguhan”.
- Meta endpoint with genuine API credentials stored only in secrets manager; budget set, rate limited, actual model IDs observed.
- Correct AI and verifier consent versions; no hidden generator fallbacks, no mislabelled doubles.
- Export/share links never published in README or demo video if still valid; revoke all fixture links after recording.
- Review contract of Meta model license and allowed sensitive-data/age usage before real-person access.

## 5. Controlled synthetic smoke sequence (requires safe environment)

| Step | Action | Expected evidence |
| --- | --- | --- |
| 1 | Capture source SHA, image digest, environment & demo owner | Consistent version |
| 2 | Start one fictional adult guest | Consent version and lifecycle correct |
| 3 | Refuse AI consent and submit temporary draft | No model request; manual path |
| 4 | Accept AI; ask one harmless synthetic prompt | Authentic Meta call and provider label |
| 5 | Review one proposed reflection and decline | No accepted memory side-effect |
| 6 | Approve only selected note/summary | Version-bound approval |
| 7 | Export/share selected synthetic summary | Snapshot fields match approved projection |
| 8 | Revoke share then retry access | Future access denied |
| 9 | End session/teardown | Demo data and access capabilities removed/expired |
| 10 | Check metadata-only health dashboards | No sensitive content leakage; budget bounded |

Record step results as `NOT RUN` until executed. If live API is unavailable, don't silently switch to prerecorded output; show separately clearly labelled recorded/test-double path and document what was actually proven.

## 6. Rollback and recovery

If a new implementation PR alters MIE/share/consent code, require:
- owner-reviewed backup plan with storage-region/legal approvals, migration dry-run + staged restore rehearsal;
- feature flag for MIE gated OFF until acceptance, and rollback commit/image digest;
- forward-compatible schema or explicit migration rollback path; never run destructive down migrations against real data without separate approval;
- content-free error metrics and alert thresholds, incident owner, change timestamp;
- if consent or share authority is uncertain, suspend the affected AI/share feature rather than continuing optimistically;
- prevent previously revoked capabilities from reviving during backup restoration (deletion-journal gate unresolved per existing D-055).

## 7. Production and compliance reconciliation

| Blocker / status | New owner information | Evidence needed to mark resolved |
| --- | --- | --- |
| B-3 hosting | Owner reports deployment already in production | Verifiable URL, deployment/provider, SHA/digest, HTTPS, secrets/DB/backup architecture; do not auto-close |
| B-1 Meta Modal live | Not established by hosting | Model/provider mode evidence, permitted deployment, live evaluation |
| D-063 meta_api production policy | Code says development/test/demo only | Do not bypass restrictions; separate approved architecture/compliance change |
| B-15 region | Still uncertain | Confirm actual app/db/backup/key and external AI processing regions |
| B-10 legal/privacy for real users | Still open | Legal review, DPIA, processor terms, minors and Indonesian obligations as applicable |
| B-5 verified help lines | Still open | Human verification of actual displayed help resources |
| B-6 user research | Still open | Consented ethical study; not replaced by E2E |
| Independent security/a11y audits | Still open | Named qualified reviewers + reports, not internal agent assertions |

Production deployment is an operations milestone; it is **not** evidence of clinical safety, partner endorsement or eligibility for unrestricted use.

## 8. Owner sign-off form

```yaml
owner_report: "production hosting already present, 2026-11-01"
production_url: null
deployment_sha: null
provider: null
uses_real_user_data: UNKNOWN
synthetic_demo_isolated: UNKNOWN
verified_readonly_checks: []
skipped_due_to_privacy: []
outstanding_blockers: [B-1, B-3-verification, B-5, B-6, B-10, B-15]
reviewed_by: null
status: OWNER_REPORTED_PENDING_VERIFICATION
```
