# Sela v2 safety and privacy

Status: **implemented and tested as software behaviour** (`internal/safety`, `internal/journey`, `internal/httpapi`); not a clinical validation or safety certification. Authority: [SPEC](../SPEC.md) V1–V8, V11–V13, V22–V25. Historical v1 controls are [archived](archive/v1/SAFETY.md).

## Care boundary

Sela prepares and simulates a handoff to **fictional** professionals. It does not diagnose, prescribe, triage, give treatment advice, act as a therapist or provide consultations. Fictional follow-up plans are fixture text attributed to a fictional professional; the model cannot write or change them. Every care surface carries a synthetic/simulated label in Indonesian and English, and receipts state that a sent or acknowledged request is not care received.

## Deterministic safety routing

`internal/safety` assesses **every user-authored text before any other check, before it is stored and before it is sent to a model**: the story (as a whole and per sentence, in any mode), clarification answers, wording edits, opening lines, rehearsal attempts and pseudonyms. Length limits, contact-detail checks and approval requirements never pre-empt the crisis route. It is a lexicon ported from the Dengar.in crisis engine with the same rule ids, extended for clause-level explicit denial ("aku tidak mau bunuh diri" alone does not count, but "… tapi aku ingin mengakhiri hidupku" does), quotations and third-party framing (routed to *elevated* support information rather than ignored), slang, leetspeak and code-switching, and documented idiom exceptions. The exceptions are narrow: an accident or medication-adherence marker must sit next to the injury or dose and is ignored when the sentence also carries a risk, method, farewell or intent word (and never covers wrists, veins or the neck), and a lyrics or news frame covers only what it introduces, so a speaker who adopts the act is still crisis. Ambiguity takes the safer path. Each assessment records the ruleset version (currently `sela-safety-2026.10.26-4`, checked by 1329 test vectors in `internal/safety/testdata/vectors.json`; assessment time grows linearly with input length). Its limitations are listed in the package documentation and the test vectors.

- **crisis** → the session is interrupted: the triggering text and the draft are discarded, running AI operations are canceled and their event logs reduced to a content-free terminal event, and no further story, model call, approval, share, recipient transition, check-in or plan display is possible. The UI goes to Immediate help and every session page shows the interrupted state; revoking access and deleting data stay available. The help page lists verified Indonesian services with region, sources and verification dates and says that language does not determine jurisdiction. "Delete this session and start again" is offered.
- **elevated** → the journey continues and support information is shown.
- Routing outcomes are counted (content-free counters), never logged per request; the counters are operator-only (`SELA_METRICS_TOKEN`), so a visitor cannot watch them change in real time.

No ML classifier is used (D-017); the model never decides routing.

## Consent, approval and sharing

Four independent, versioned, server-enforced decisions: AI processing, storage (retention), brief approval (bound to one ledger version and content hash) and sharing (per recipient, selected fields, duration, current policy version). Any ledger change clears approval. A share can never outlive the session. The recipient needs the link (token in the URL fragment, removed from the address bar on load) **and** a separate access code; five wrong codes lock the share; one uniform error hides why access failed. Revocation, expiry, supersession and deletion end access immediately; the UI says already-viewed information cannot be recalled.

## Privacy

- **What is processed:** story text the user selects or writes, clarification answers and edits; with AI consent, the sentences are sent to the Meta model on Modal (configured not to log requests; Modal processes data as infrastructure provider). The hosting platform processes IP addresses and timing.
- **What is stored:** only in server memory under the retention choice; restart erases everything. Practice text with obvious contact details (emails, phone-like numbers, 16-digit IDs, URLs) is rejected.
- **Identity:** anonymous by default or a pseudonym; not technical anonymity.
- **Logs:** route template, status, duration and request id; model call metadata (status, latency, token counts). Never story text, prompts, model output, tokens, access codes or credentials — verified by tests that search logs for canary strings.
- **Browser:** HttpOnly SameSite=Strict cookies, strict CSP (`default-src 'self'`, no inline scripts), `Referrer-Policy: no-referrer`, `frame-ancestors 'none'`. Only the interface language is kept in `localStorage`.

## Abuse and cost bounds

Per-address token buckets (IPv6 by /64, least-recently-used eviction, never failing closed) for session creation, AI starts, recipient sign-in and other requests; at most 50 live sessions per address by default; per-session caps on ledger items, story size and handoffs; at most 3 recipient principals per handoff; per-session AI operation limit; global admission semaphore; Modal proxy authentication before any GPU starts; at most one GPU container; scale to zero after five minutes.

Tests demonstrate this software behaviour on synthetic data only. Private security reports: [SECURITY.md](../SECURITY.md).
