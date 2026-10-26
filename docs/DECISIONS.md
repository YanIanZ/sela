# Sela v2 decisions

Authority: [SPEC](../SPEC.md). This record explains origin; it does not override the spec.

| Decision | Origin | Status / implication |
| --- | --- | --- |
| Product name Sela | User | Confirmed; prior tagline remains branding, not an anonymity-only product definition |
| Full care journey | User | Confirmed: toward professional/medical support and follow-up; actual demo care is fictional |
| Indonesian and English | User | Confirmed across the complete journey |
| Synthetic demonstration | User | Confirmed for people, professionals, slots, consultation and follow-up |
| Backend Go | User | Confirmed; replaces earlier Go/Swift uncertainty |
| Meta-aligned AI and hackathon | User | Confirmed: Meta/Muse Spark remains primary generation and source of Meta hackathon AI contribution; access remains unverified |
| Claude Sonnet 5.5 secondary verifier | User (26 October 2026) | Confirmed optional independent-provider *review role* only, separate consent; no backup generation or medical claims. Isolated Go prototype, not yet integrated |
| Single static-web deployment | Engineering default | React/TypeScript + Vite, Go serves API/assets; not implemented |
| In-memory bounded demo state | Engineering default | Per visitor; no database/Redis in target demo; no real-data migration |
| Text-first, adult fixtures | Engineering default | Optional synthetic audio later; no live microphone/open patient intake in MVP |
| Clinical handoff simulation | Engineering default consistent with user scope | Fictional inbox, separate sharing consent, scripted consultation/plan, explicit demo labels |
| Legacy documentation archive | User-requested documentation refactor | Historical content and credits preserved; active requirements replaced by SPEC |

Changing a user-confirmed decision requires new user direction. Engineering defaults may change with recorded rationale and consistent spec/doc updates; they must not silently expand product scope. No row implies implementation, clinical validation, real partnerships or event acceptance.
