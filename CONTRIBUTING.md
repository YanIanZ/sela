# Contributing to Sela

Sela is maintained by [YanIanZ](https://github.com/YanIanZ). Read the [README](README.md), [documentation index](docs/README.md), and [provenance](docs/SELA_PROVENANCE.md) before changing inherited behavior.

## Local setup

Use Node.js 22.18+ and npm 10+. With nvm, run `nvm install` and `nvm use`; CI uses Node.js 22.

```sh
npm ci
cp .env.example .env.local
npm run dev
```

Keep real credentials in ignored environment files. Use synthetic examples in tests, issues, screenshots and demo recordings.

## Changes and checks

Use a focused branch and describe the user-visible result, relevant limitations and validation in your pull request. Preserve the MIT notice and original contributor credits.

```sh
npm run lint
npm run lint:services
npm run typecheck
npm test
npm run build
```

The Redis tests start an isolated local instance using `redis-server` or Docker. Full CI runs the same tests plus the production Redis checks. Live provider probes and evaluations require explicit credentials and are separate from the mock test suite.

For a focused change, start with the relevant existing script, such as `npm run test:voice`, `npm run test:carebridge`, or `npm run test:crisis`. Add regression coverage when changing behavior; documentation-only edits need link and formatting checks.

## Product constraints

- Update Indonesian and English copy together; preserve dictionary key parity.
- Preserve consent, crisis gates, redaction, review and export boundaries.
- Do not introduce diagnostic or treatment claims.
- Retain existing storage and database identifiers unless a migration is part of the change.
- Distinguish inherited functionality, newly implemented work, mocks and verified live integrations.

Report reproducible bugs through [GitHub Issues](https://github.com/YanIanZ/sela/issues). For security vulnerabilities or exposed data, use the private process in [SECURITY.md](SECURITY.md).
