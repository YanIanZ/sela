# Sela v2 decisions

Authority: [SPEC](../SPEC.md). This record explains origin; it does not override the spec.

| Decision | Origin | Status / implication |
| --- | --- | --- |
| Product name Sela | User | Confirmed; prior tagline remains branding, not an anonymity-only product definition |
| Full care journey | User | Confirmed: toward professional/medical support and follow-up; actual demo care is fictional |
| Indonesian and English | User | Confirmed across the complete journey |
| Synthetic demonstration | User | Confirmed for people, professionals, slots, consultation and follow-up |
| Backend Go | User | Confirmed; replaces earlier Go/Swift uncertainty |
| Meta-aligned AI and hackathon | User | Confirmed; Meta open-weight model on Modal ([ADR-0001](adr/ADR-0001-meta-open-weight-model-on-modal.md)); adapter and serving app implemented, live access not yet verified ([B-1](BLOCKERS.md)) |
| DeepSeek-V4.1-Flash-1017 fallback | User (26 October 2026) | Confirmed opt-in availability fallback only, Meta remains primary; explicit consent, fail-closed and truthful provider attribution. Standalone Go prototype, not integrated |
| Claude Sonnet 5.5 secondary verifier | User (26 October 2026) | Confirmed optional independent-provider *review role* only, separate consent; no backup generation or medical claims. Isolated Go prototype, not yet integrated |
| Single static-web deployment | Engineering default | React/TypeScript + Vite, Go serves API/assets; implemented |
| In-memory bounded demo state | Engineering default | Per visitor; no database/Redis; implemented with idle/absolute expiry (D-005, D-010) |
| Text-first, adult fixtures | Engineering default | Implemented: seeded fictional adults plus bounded fictional practice text (D-003); no live microphone or open patient intake |
| Clinical handoff simulation | Engineering default consistent with user scope | Implemented: per-owner fictional recipient, link token + access code, separate sharing consent, scripted consultation/plan, explicit demo labels (D-006, D-007) |
| Legacy documentation archive | User-requested documentation refactor | Historical content and credits preserved; active requirements replaced by SPEC |

Engineering decisions made during implementation are recorded in the [decision log](DECISION_LOG.md). Changing a user-confirmed decision requires new user direction. Engineering defaults may change with recorded rationale and consistent spec/doc updates; they must not silently expand product scope. No row implies implementation, clinical validation, real partnerships or event acceptance.
