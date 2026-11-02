# Alignment with the 2026-11-01 owner direction

Status: **reconciliation note**. Date: 2026-11-01 (WIB). The owner added scope after this package was written. This note maps that scope onto the plan so the package and the repository stay consistent. The repository's working plan is [`docs/roadmap/SAFE_FLOW_AND_TODO.md`](https://github.com/YanIanZ/sela/blob/main/docs/roadmap/SAFE_FLOW_AND_TODO.md) on `main`; its decisions are D-066 to D-072 in the decision log.

## What changed

| Owner direction | Built on main | Hackathon task | Boundary kept |
| --- | --- | --- | --- |
| Natural replies in two modes ("Ngobrol & cari solusi", "Didengarkan saja") | D-066, D-067 | — (listening baseline) | Guards and verifier unchanged |
| Stricter guardrails: danger words go straight to emergency help | D-068 | H-021 | Deterministic, before any model; Sela contacts no one |
| Anti-jailbreak, multilingual, typo-tolerant | D-070 | H-021 | Can only raise safety; everyday phrases pinned by tests |
| Planned care uses check-ins, reflections, notes and chat history to show a stage, then professionals | D-071 condition map | H-022, H-023 | Opt-in consent; saved records only; fixed rules; not diagnosis or triage; no score |
| Professionals simulated for the product demo | D-071 `/api/v3/care-demo` | H-023 | Fictional, labelled, cannot be contacted or receive data |
| Anonymous story sharing with Meta detecting bullying and removing replies (Dengar.in successor) | D-072 sharing space | H-024, H-025 | Adults only; moderated **before** publish; crisis never stored; operators never see authors |
| Professional repository: Code of Conduct, CONTRIBUTING, SECURITY, MIT credits, favicon, product link | Done | — | Dengar.in Team notice kept as the forked origin (MIT) |
| Production at https://sela.vertalt.my.id | Read-only checks pass | H-001, H-026 | Release SHA still unverified |

## Effect on the plan

1. **The flagship is unchanged.** MIE (H-003–H-009) remains P0 and the 3-minute demo still tells the correction-and-invalidation story. The new features run beside it.
2. **They are MIE-ready by construction.** The condition map and planned care read only saved, versioned records the person controls; the sharing space holds only text the author chose to publish. Once H-007 lands, a corrected or deleted source invalidates what was derived from it.
3. **Demo use.** The condition map, the simulated professionals and the sharing space may appear in the backup demo with synthetic identities and their visible labels ("bukan diagnosis", "Simulasi produk · fiktif", "Diperiksa sebelum tampil").
4. **New no-go triggers.** A crisis text published in the sharing space, an author identity visible to an operator or another person, a condition stage shown without its reasons, or a demo professional that can receive data each stop the feature.
5. **Open evidence.** Moderation evaluation (H-025), multilingual safety evaluation, release SHA in production (H-026).
