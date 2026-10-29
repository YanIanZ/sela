# Frozen v1 crisis tests

These four files are a read-only snapshot of the v1 JavaScript crisis tests.
`TestVectorsCoverV1` parses their string literals to prove that every v1 input
is either present in `testdata/vectors.json` or listed as a known non-input.
Keeping the snapshot preserves that protection after the Next.js app and
`tests/crisis/` are removed.

Source: `tests/crisis/*.test.mjs` at commit `0de6755`, MIT-licensed original
project. Do not edit these files; add new regression vectors to
`testdata/vectors.json` instead.
