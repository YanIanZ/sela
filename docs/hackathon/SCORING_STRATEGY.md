# Competition strategy and evidence-based internal scorecard

Status: **planning rubric, not official jury scoring**. Related: [README](README.md), [evaluation](EVALUATION.md), [demo](DEMO_AND_SUBMISSION.md).

## 1. What is actually confirmed

The organizer's public Indie description explicitly highlights **creativity, technical ambition, and whether the solution works**; the theme Build for Your World emphasizes solving a real domain problem with Meta models. Sources: https://cerebralvalley.ai/e/meta-ai-developer-hackathon and https://dev.meta.ai/events/global-hackathon .

Our historic 30% impact / 25% Meta tooling / 20% technical / 25% demo breakdown is **an internal heuristic drawn from earlier event research**, not independently verified from the authenticated final Official Rules. Use it to allocate effort, never as a statement of guaranteed jury weights. Verify participant acceptance, eligibility, submission/video requirements and rights representations in the actual portal.

## 2. Internal 100-point framework

| Category | Internal weight | What could justify full internal marks | Automatic cap |
| --- | ---: | --- | --- |
| Human value / problem | 30 | Compelling privacy and meaning-fidelity problem; one measurable consultation-prep task; limits stated; 1+ stakeholder feedback if ethically obtained | No user study: do not claim efficacy |
| Meta integration | 25 | Real task-specific Meta calls; structured interpretation; different-model verifier; source fidelity; valid consent; reproducible ablation vs baseline | No live provenance or no baseline = no full marks |
| Engineering | 20 | Source-bound version graph, transaction/idempotency/race guards, access isolation, no-AI, CSP, budget limits, tests | Any high-severity safety/privacy hole blocks ship |
| Working demo and reproducibility | 25 | One clear synthetic journey, hosted/controlled demo, smoke checks, concise video, accessible fallback, commit-pinned steps | Cannot replay = no full marks |

Formula: `InternalScore = 0.30*human + 0.25*meta + 0.20*engineering + 0.25*demo` where each input is 0–100. The number is **descriptive**, not a win probability. Report both quantitative evidence and uncertainty. Example “target 100” is a goal, not performance measured today.

## 3. Differentiation claims ladder

| Claim | Evidence required | Allowed public wording before evidence |
| --- | --- | --- |
| “Privacy-first listening” | Consent contract and server tests, non-logging/privacy review, hosted checks | “Designed for user-controlled privacy; rollout gates outstanding” |
| “AI does not rewrite your story” | Original verbatim preserved, quote grounding, correction and rejection flow with negative tests | “Source-bound reflections with user review” |
| “Corrections propagate” | Transactional descendant invalidation incl. exporting/recipient reads and race tests | “Cross-surface invalidation is being implemented” |
| “Safer than baseline Meta” | Preregistered held-out comparisons with confidence intervals/human adjudication | “Designed to catch unsupported claims; advantage not yet measured” |
| “Accessible to all” | A11y automated and manual assistive-technology task evidence by representative users | “Accessibility-conscious; independent evaluation outstanding” |
| “Clinical benefit” | Ethical prospective clinical research + regulatory review | **Never claim based on this hackathon** |
| “Connected to hospitals” | Contracts, credential verification, actual integration evidence | “Recipient workflow simulated with synthetic participants” |
| “First in the world” | Extensive validated originality/IP landscape evidence | **Do not claim** |

## 4. Novelty hypothesis and alternatives

Existing wellbeing products already have chat, journal, prompts, and therapist/provider directories. The hypothesis worth testing is a **versioned, user-governed semantic dependency graph** across chat, memory and care summary—not that no one has ever implemented source quotes or editable memory.

Differentiator demonstrations:
1. AI overstates “sometimes” as “always”; detect and abstain or ask the person.
2. An interpretation is rejected; it cannot be reintroduced as trusted context.
3. A source revision makes an approved derivative stale; export and recipient read detect it.
4. Consent revoked during an in-flight generation; output not persisted/released after revocation.
5. Shared snapshot stays explicit: user sees precisely what recipient can access, and revocation blocks future reads.

Each requires an acceptance test and a visible UI trace. If an innovation cannot be shown and benchmarked, label it a design hypothesis.

## 5. Alternative scenarios and expected decision

**A — Full competitive**: stable repeatable Meta live path + MIE propagation + valid benchmark + accessible full journey. Submit full portfolio only if all P0 pass.

**B — Focused flagship (preferred)**: focus on synthetic adult listening, correction → stale → reapprove, selective share and revoke; no rush to publish reading library or expand multi-hospital features. If difficult, freeze to this scope.

**C — Reliability first**: if live inference unstable, retain an honestly labelled manual/recorded or test-double demo for technical mechanics, and show any authentic recorded live runs separately with date/model/commit. Never claim recorded output was live, and do not imply it meets live requirement.

**D — Stop/limit**: if private-data leakage, unsafe crisis handling, uncontrolled consent, forged verifier provenance, or non-reproducible deployment exists, stop live public demo, isolate it, and use local synthetic walkthrough while remediating. Safety is not a stretch goal.

## 6. Prioritization by marginal judge impact

| Work | Impact | Effort | Priority / rationale |
| --- | --- | --- | --- |
| Hosted/synthetic smoke verification of existing production | Very high | Low/medium | P0; validates working product without rebuilding hosting |
| Atomic correction propagation & stale share reads | Very high | High | P0; strongest technical originality |
| Preregistered baseline vs MIE comparison | Very high | Medium | P0; demonstrates actual Meta value |
| One polished 3-minute journey | Very high | Medium | P0; converts architecture into understandable story |
| Synthetic crisis/no-consent/no-AI adversarial tests | High | Medium | P0; prevents catastrophic misses |
| Manual screen-reader/320px/ID-EN task run | High | Medium | P1; useful credibility |
| Real partner booking, new LLMs, voice, avatar, huge corpus | Low before deadline | Very high | Defer |

## 7. Evidence packet per scored claim

Each recorded claim must have: `claim_id`, relevant criterion, pre-registered expectation, fixture hash, baseline/model version, commit SHA, environment and region (if verified), UTC time, anonymized trace ID, measured result, screenshot/video locator, failure/limitations, and reviewer sign-off. Never include prompt/story content from real users. If checks are not run, mark **NOT RUN**, not passing.

## 8. Final review questions

- Would a skeptical reviewer understand what Sela does in 20 seconds?
- Can a correction be shown to affect an actual downstream artifact in less than one minute?
- Can the demo expose a failure path without invented success?
- Is every result correctly attributed to Meta generator vs verifier, Go checks, and user decisions?
- Does every displayed provider/recipient/data-source label match the code and deployment?
- Is the project ownership/provenance of inherited Dengar.in accurately disclosed?
- Can a second developer reproduce the result from the pinned commit?
