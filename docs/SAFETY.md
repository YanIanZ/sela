# Sela v2 safety and privacy

Status: **implemented and tested as software behaviour** (`internal/safety`, `internal/journey`, `internal/httpapi`); not a clinical validation or safety certification. Authority: [SPEC](../SPEC.md) V1–V8, V11–V13, V16–V19. Historical v1 controls are [archived](archive/v1/SAFETY.md).

## Care boundary

Sela prepares and simulates a handoff to **fictional** professionals. It does not diagnose, prescribe, triage, give treatment advice, act as a therapist or provide consultations. Fictional follow-up plans are fixture text attributed to a fictional professional; the model cannot write or change them. Every care surface carries a synthetic/simulated label in Indonesian and English, and receipts state that a sent or acknowledged request is not care received.

## Deterministic safety routing

`internal/safety` assesses **every user-authored text before it is stored or sent to a model**: the story (as a whole and per sentence), clarification answers, wording edits, opening lines and rehearsal attempts. It is a lexicon ported from the Dengar.in crisis engine with the same rule ids, extended for clause-level explicit denial ("aku tidak mau bunuh diri" alone does not count, but "… tapi aku ingin mengakhiri hidupku" does), quotations and third-party framing (routed to *elevated* support information rather than ignored), slang, leetspeak and code-switching, and documented idiom exceptions. Ambiguity takes the safer path. Its limitations are listed in the package documentation and the test vectors.

- **crisis** → the session is interrupted: the triggering text and the draft are discarded, running AI operations are canceled, and no further model call, approval, share, recipient transition, check-in or plan display is possible. The help page lists verified Indonesian services with region, sources and verification dates and says that language does not determine jurisdiction. "Delete this session and start again" is offered.
- **elevated** → the journey continues and support information is shown.
- Routing outcomes are counted (content-free counters), never logged per request.

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

Per-address token buckets for session creation, AI starts, recipient sign-in and other requests; per-session AI operation limit; global admission semaphore; Modal proxy authentication before any GPU starts; at most one GPU container; scale to zero after five minutes.

Tests demonstrate this software behaviour on synthetic data only. Private security reports: [SECURITY.md](../SECURITY.md).
