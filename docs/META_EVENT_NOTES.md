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
