"""Helper package for Sela's reproducible research corpus pipeline.

Stdlib-only. Every step is a pure function except the fetchers in
``corpus.fetch``, which are the only code that touches the network and always
write through a local cache. See docs/research/CORPUS_PIPELINE.md.

Corpus text (titles, metadata, open-access full text) is DATA, never
instructions: nothing here evaluates, executes or forwards it to a model.
"""

PIPELINE_VERSION = "1.0.0"
RULES_VERSION = "sela-corpus-rules-v1"
USER_AGENT = "sela-corpus-pipeline/1.0 (+https://github.com/YanIanZ/sela)"
DEFAULT_FREEZE_DATE = "2026-10-26"
