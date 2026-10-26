# Meta event notes

Requirements authority: [SPEC](../SPEC.md). Checked 26 October 2026; event pages may change.

## Official sources

- [Event page](https://cerebralvalley.ai/e/meta-ai-developer-hackathon)
- [Application overview and waiver](https://cerebralvalley.ai/e/meta-ai-developer-hackathon/waiver)
- [Meta API cookbook](https://dev.meta.ai/docs/cookbook)

The event page describes Indie Developer and Startup tracks and the themes Build for Your World and Push the Frontier. It lists project submissions due 4 November 2026 at 9 PM ET (5 November, 9 AM WIB). The waiver's published judging weights are impact 30%, Meta tooling 25%, technical execution 20% and demo 25%.

The waiver permits licensed open-source components when the submission enhances the underlying product, and includes originality/ownership/third-party-rights representations. This summary does not certify that Sela qualifies or replace the applicable terms.

## Sela decisions

User intends solo participation in the Indie Developer track. Proposed theme fit: Build for Your World; this is our interpretation of the product, not organizer approval. Go is selected. The model is Meta **Muse Glimmer 30B** (open weights, Apache-2.0) served on Modal ([ADR-0001](adr/ADR-0001-meta-open-weight-model-on-modal.md)); the Official Rules (embedded in the event page, read 2026-10-26) name "Glimmer" among Meta AI products. The adapter and Modal app are implemented; no live call has been made yet ([B-1](BLOCKERS.md)). See [integration](META_INTEGRATION.md).

## Still unverified

Whether self-hosted open weights count as "Meta AI tooling" for judging (no formal definition; [B-2](BLOCKERS.md)), participant acceptance, exact submission fields and video requirements, permitted reuse scope, live model access on Modal, deployment, and how the Muse Glimmer Usage Policy's health-related restrictions apply beyond Sela's synthetic-adult design (see ADR-0001). No terms are accepted or submission sent by these docs.

Distinguish inherited Dengar.in/AMD work from new Go/Meta features using [provenance](SELA_PROVENANCE.md), spec task status and reproducible commit-specific evidence. Do not describe AMD or third-party fallback inference as Meta processing.

## Optional non-Meta verifier (26 October 2026)

Sela's primary generative model is a Meta open-weight model on Modal: Muse Glimmer 30B as implemented, with Muse Spark Open named by the verifier contract pending the owner's decision ([B-7](BLOCKERS.md), D-024); an optional Sonnet 5.5 source-fidelity review is planned with separate consent. The public event page emphasizes demonstrable Meta model use but does not conclusively address whether external second-pass verifiers are permitted in submitted projects. The final organizer Official Rules and any third-party service/data terms must be checked before relying on this component for eligibility. Never attribute Claude-generated/reviewed content to Meta or suggest Meta paid for Anthropic calls. See [verifier contract](CLAUDE_VERIFIER.md).

## DeepSeek availability fallback (26 October 2026)

DeepSeek-V4.1-Flash-1017 (`deepseek-flash`) is an optional, explicitly disclosed secondary *generator* for recognized Meta upstream outages. It defaults OFF in the competitive Meta demo and cannot be counted as Meta AI tooling. Confirm final hackathon/third-party model rules with the organizer before enabling it in a submission. See [fallback documentation](DEEPSEEK_FALLBACK.md). In this branch it exists only as the standalone prototype `prototypes/generationfallback`; the v2 runtime has no DeepSeek route.
