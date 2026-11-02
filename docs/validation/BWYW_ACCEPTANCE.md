# Build for Your World — acceptance, test and evidence matrix

**Status: proposed tests; NOT RUN by this documentation PR.** References [roadmap](../roadmap/BUILD_FOR_YOUR_WORLD_2026.md), [architecture](../architecture/MEANING_CONTINUITY_EXTENSION.md), [video](../demo/META_HACKATHON_2026_STORYBOARD.md). Existing MIE scenario-B coverage is not evidence that proposed Timeline, Passport or Action Bridge flows are shipped.

## 1. Test levels and evidence

Every executed result must record `commit_sha`, `environment`, `build_digest`, `test_name`, `fixture_sha`, `timestamp_utc`, `test_result`, `sanitized_evidence_uri`, `reviewer`. Keep all synthetic text explicitly synthetic; never log real user stories or provider secrets. Client screenshots cannot replace API/DB assertions.

- **Unit**: timeline dates/filters, action state transitions, source-version selection, semantic assertions and accessibility rendering.
- **Integration**: PostgreSQL owner isolation, action export/deletion, MIE transactionality, share and CareRequest read gate, provider consent and budget.
- **E2E**: real browser against isolated Docker/staging with synthetic guest and fictional partner, source corrections, share/revoke, no AI path.
- **Live Meta**: only after authorized consent/provider configuration, scoped budget and identifiable actual model IDs; separate from deterministic mock fixtures.
- **Manual accessibility**: keyboard, screen reader, 320px, 200% zoom, ID/EN, focus/announcement checks. Internal review is not independent audit.
- **Safety**: crisis and safety prompts are predeclared fictional fixtures; don't call real support lines as test.

## 2. Traceability matrix

| ID | Priority | Journey / threat | Test oracle | Stage |
| --- | --- | --- | --- | --- |
| BW-T01 | P0 | Guest opens empty Timeline | Empty state; no AI request; no cross-user data | Unit + E2E |
| BW-T02 | P0 | Three saved source versions across dates | Chronology and exact source versions, no accidental duplicates | Integration + UI |
| BW-T03 | P0 | Source quote differs from AI summary | Meaning Diff flags and review required | Unit + E2E |
| BW-T04 | P0 | User rejects wrong AI interpretation | No accepted wrong memory or trusted summary | DB + E2E |
| BW-T05 | P0 | User edits source after approved Passport share | Summary stale, future share denied, care request cancelled | DB + concurrency |
| BW-T06 | P0 | Owner reapproves corrected Passport | Fresh version only; old frozen share remains ended | DB + UI |
| BW-T07 | P0 | Passport with 1 unrelated private source | Excludes unrelated source from preview/export/recipient | Snapshot diff test |
| BW-T08 | P0 | User has AI consent OFF | Manual Timeline/Passport/Action still available; provider calls 0 | Integration + E2E |
| BW-T09 | P0 | One manually written action | Saved once with owner scope, text unchanged | DB + UI |
| BW-T10 | P0 | Meta proposes action | Nothing persisted until user explicitly approves | AI route + DB |
| BW-T11 | P0 | Source change affects linked action | Action needs_review, not auto-removed from history | DB + UI |
| BW-T12 | P0 | Actor B guesses owner A's id | Uniform non-disclosing denial across new routes | API security |
| BW-T13 | P0 | Separate sharing permission absent | Passport may be private; create-share denied | API + UI |
| BW-T14 | P0 | Partner staff opens summary | Restricted by org, role and active share | DB + API |
| BW-T15 | P0 | Model failure mid-suggestion | Clear unavailable state, no unvalidated output, draft retained | Fault injection |
| BW-T16 | P0 | Prompt injection in user source | Content treated as data, no tool/role switch or secret leak | AI guard |
| BW-T17 | P0 | Older snapshot opened after correction commit | Future read denied; no wrong history as current | DB concurrent |
| BW-T18 | P0 | Action deleted and owner export requested | Deleted action absent, no stray MIE edges for authority | DB/export |
| BW-T19 | P0 | Care partner synthetic flow | Explicit SIMULASI, no real provider/hospital contact | Browser + API |
| BW-T20 | P0 | Low-vision/keyboard flow | Can review source, reject, approve passport, revoke | Manual accessibility |
| BW-T21 | P1 | Multilingual casual speech, code-switch | Negation and actor preserved, errors localized | Model offline+live |
| BW-T22 | P1 | DST/clock skew/same timestamp | Stable timeline, local time formatted correctly | Unit + browser |
| BW-T23 | P1 | Action declined or paused | No nagging/points/streaks or notifications | UI + API |
| BW-T24 | P1 | Community excerpt not explicitly copied | No private source linkage/automatic memory | API + DB |
| BW-T25 | P0 | Deletion with `derived=keep` | Historical owner data stale, old share blocked | DB + E2E |
| BW-T26 | P0 | User data age/guardian gates | Current age policy applied consistently | Integration |
| BW-T27 | P1 | Metadata-only audit | No story, quote, token, private names, user identifier in public log | Code and runtime |
| BW-T28 | P0 | Video on committed build | Capture's SHA equals prepared artifact/recorded provenance | Demo audit |

## 3. Benchmark hypothesis (separate from patient health outcomes)

Compare on **same** synthetic source corpus and genuine model/version:
- **B0**: Meta Spark produces a response without MIE deterministic checks.
- **B1**: same Meta Spark + deterministic Meaning Diff.
- **B2**: B1 + distinct Meta Wonder verifier.
- **T**: B2 + user-governed MIE source/descendant and user-approved Passport/Action workflow.

Quantify: unsupported-inference rate, wrong negation frequency, stale-share leakage (test suite, target 0), successful task completion, time-to-prepare a short sheet, wrong recipient fields (target 0), source quote fidelity, abstentions, p50/p95 latency and cost per successful task.

**Do not** compare a user-corrected T case against an uncorrected B0 model and call it an autonomous model quality improvement; label user interventions. Report count, sample split, Wilson/bootstrap uncertainty and false flags. Existing `eval/mie/cases.jsonl` contains 128 synthetics and prior offline held-out critical 9/9 with benign false flags 1/9; that is not a live model superiority result. Verify actual numbers on the final commit.

## 4. Go / No-Go gates

**Block demo feature immediately** if any of these reproduces: cross-account story disclosure, consent bypass, stale post-revocation recipient access, unauthorized partner role reading, unsafe crisis response, AI output presented as user-authoritative without review, automatic medical diagnosis/urgent outreach, real patient/clinician information in synthetic fixture, missing provider-label when claiming live Meta.

**Scope cut to existing features** if Timeline/Action implementation is unstable by recording time: the **current** MIE correction + existing Summary/CareRequest journey remains a credible technical demo. Show newly proposed concepts only as *future roadmap slides with clear “proposed” labels*. This contingency is essential to avoid overstating implementation.

## 5. Test command candidates

Use repo's actual tool versions and CI scripts; do not assume every planned command passes just because listed.

```sh
go vet ./...
SELA_REQUIRE_DB_TESTS=1 go test ./internal/...
go test -race ./...
(cd web && npm ci && npm run typecheck && npm test && npm run build)
# Only after test harness is prepared:
# Run existing browser e2e using isolated PostgreSQL, synthetic seeds and no live real-user data.
# Run cmd/sela-eval with pinned synthetic fixture and documented mode.
```

Critical review must include integration with migrations 1200 (MIE) and 1300 (partner invitations), no collisions if Action records need new migration (next new number chosen from live tree), and full export/deletion impact.

## 6. Planned release/evidence record

```yaml
feature_pack: "bwyw-2026"
status: NOT_IMPLEMENTED_IN_THIS_PR
features:
  context_timeline: PLANNED
  care_passport_preset: PLANNED
  personal_action_bridge: PLANNED
  video_shotlist: AUTHORED
  video_recording: NOT_RECORDED
test_evidence:
  repo_sha: null
  deployed_sha: null
  ci: NOT_RUN_FOR_PROPOSED_FEATURES
  live_meta_ablation: NOT_RUN
  scenario_e2e: NOT_RUN
  independent_accessibility: NOT_RUN
  independent_security: NOT_RUN
  submission_receipt: null
owner_signoff: false
```
