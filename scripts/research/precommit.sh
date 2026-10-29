#!/bin/sh
# Required before every commit that touches scripts/research, tests/research, data/research or docs/research
# (docs/research/CORPUS_PIPELINE.md, "Required before committing"). Run it where the local caches exist:
# the leak check compares cached abstracts and full texts with the committed files, which CI cannot do.
# Exits non-zero on any failure, including a missing cache. Not installed as a git hook by the repository.
set -e
cd "$(dirname "$0")/../.."
python3 -m unittest discover -s tests/research -p 'test_*.py'
python3 scripts/research/corpus_pipeline.py verify
python3 scripts/research/leak_check.py --require-cache
echo "research pre-commit checks: OK"
