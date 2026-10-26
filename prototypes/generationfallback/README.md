# DeepSeek Flash 1017 — Sela opt-in fallback (Go prototype)

The current Sela app is **Next.js v1**. This directory is a standalone *Go prototype* that demonstrates **primary Meta routing with a consented DeepSeek alternative**. The real Meta adapter, new Go API, frontend and session lifecycle are not built or connected here.

Official DeepSeek Chat Completions API:
- Endpoint: https://api.deepseek.com/chat/completions
- Model alias: `deepseek-flash` (maps to DeepSeek-V4.1-Flash-1017 as of 26 October 2026)
- Authentication: server-side DeepSeek key; do not put keys in a repo or frontend
- JSON mode with fixed schema instructions and non-thinking request for a bounded source-linked demo task.

To run **offline mocked tests**, without Anthropic/DeepSeek/Modal credentials or paid inference:

    cd prototypes/generationfallback
    go test -race ./...

## Required integration in future Go runtime

Create Meta Generator with a validated live Muse Spark adapter, add DeepSeekClient via `NewDeepSeekClient(os.Getenv("DEEPSEEK_API_KEY"))`, and connect both to Router. Only application code may set server-origin flags `CrisisGatePassed`, `Synthetic`, `MetaConsent`, `FallbackConsent` and `EnableFallback` based on a session's validated fixture, safety checks and persisted consents, not a browser-provided boolean.

Call the route with context cancellation, then **revalidate session generation, consent and current brief version before persisting**. Meta is always attempted first; DeepSeek is invoked only for ErrUnavailable and only with explicit opt-in. Meta bad responses, missing evidence, model refusals or canceled contexts must not trigger switching. Provide readable fallback attribution and allow manual note completion.

Note: The prototype enforces structural ID matching but cannot establish factual/clinical correctness and does not implement real session revocation/atomic state. Use the existing Go EvidenceGuard prototype for optional *advisory* Sonnet review only after distinct Anthropic consent.

See [contract](../../docs/DEEPSEEK_FALLBACK.md), [SPEC](../../SPEC.md) and [Claude verifier](../../docs/CLAUDE_VERIFIER.md).
