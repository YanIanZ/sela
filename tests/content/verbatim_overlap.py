"""sela-verbatim-overlap-v1: the longest run of identical words between reading-library text and a
local copy of its source. Python 3.10+ standard library only.

The reading library never commits source text (the WHO PDFs are CC BY-NC-SA; the Kemenkes pages
state no licence). This tool reads a local copy that was obtained through a legitimate route (the
publisher's own page or PDF, or an archived copy of it), compares it with the card text, and prints
counts and locators only. The matched words themselves are printed only with --show-text, for the
person running the check; they are never stored in the repository.

Method (the same as the 2026-10-26 check of the 22 WHO cards):
- both texts are lower-cased and split into word tokens (letters and digits; an inner apostrophe
  stays in the word; hyphens and all other characters split words);
- for every text field of an item, the longest run of consecutive identical tokens that also occurs
  in the source is found (exact match, any position);
- a run of 5 or more words is reported, with its locator: the PDF page for a text file whose pages
  are separated by form feeds (pdftotext output), or the text block number for an HTML page, where
  the run starts. A run may continue across pages or blocks, so the count errs on the side of more.

Usage:
  python3 tests/content/verbatim_overlap.py --items data/content/reading/cards_kemenkes.json \\
      --ids tidur-cukup-tujuh-sampai-sembilan-jam --lang id --source /path/to/page.html
  python3 tests/content/verbatim_overlap.py --text "Sela's paraphrase ..." --source /path/to/source.txt

Options: --min-run N (default 5) sets the reporting threshold; --json prints machine-readable output;
--show-text adds the matched words (local use only; do not commit them).
"""
from __future__ import annotations

import argparse
import hashlib
import html
import json
import re
import sys
from pathlib import Path

METHOD = "sela-verbatim-overlap-v1"
MIN_RUN = 5
_TOKEN = re.compile(r"[^\W_]+(?:['’][^\W_]+)?")
_BLOCK_TAGS = re.compile(r"(?is)</?(?:p|div|br|li|ul|ol|h[1-6]|tr|td|th|table|section|article|header|footer|nav|blockquote)\b[^>]*>")


def tokens(text: str) -> list[str]:
    """Lower-cased word tokens; hyphenated words split ('sehari-hari' -> 'sehari', 'hari')."""
    text = text.lower().replace("’", "'").replace("-\n", "")
    return [t.replace("’", "'") for t in _TOKEN.findall(text)]


def html_blocks(raw: str) -> list[str]:
    """Visible text blocks of an HTML page: scripts, styles and comments removed, block tags split."""
    raw = re.sub(r"(?is)<!--.*?-->", " ", raw)
    raw = re.sub(r"(?is)<(script|style|noscript|svg)\b.*?</\1>", " ", raw)
    raw = _BLOCK_TAGS.sub("\n", raw)
    raw = re.sub(r"(?s)<[^>]+>", " ", raw)
    text = html.unescape(raw)
    return [b for b in (re.sub(r"\s+", " ", line).strip() for line in text.split("\n")) if b]


class Source:
    """A tokenised source with a locator for every token."""

    def __init__(self, path: Path):
        self.path = Path(path)
        data = self.path.read_bytes()
        self.sha256 = hashlib.sha256(data).hexdigest()
        raw = data.decode("utf-8", errors="replace")
        if self.path.suffix.lower() in (".html", ".htm"):
            units, self.unit = html_blocks(raw), "block"
        else:
            units, self.unit = raw.split("\f"), "PDF p."
        self.tokens: list[str] = []
        self.where: list[int] = []
        for n, unit in enumerate(units, 1):
            for t in tokens(unit):
                self.tokens.append(t)
                self.where.append(n)
        self.index: dict[tuple, list[int]] = {}
        self.first2: dict[tuple, int] = {}
        self.first1: dict[str, int] = {}
        for i, t in enumerate(self.tokens):
            self.first1.setdefault(t, i)
            if i + 2 <= len(self.tokens):
                self.first2.setdefault(tuple(self.tokens[i:i + 2]), i)
            if i + 3 <= len(self.tokens):
                self.index.setdefault(tuple(self.tokens[i:i + 3]), []).append(i)

    def locator(self, pos: int) -> str:
        return f"{self.unit} {self.where[pos]}" if self.unit == "PDF p." else f"{self.unit} {self.where[pos]} of the page text"

    def longest_run(self, text: str):
        """(run length, query start, source start) of the longest shared word run; (0, None, None) if none."""
        q = tokens(text)
        best = (0, None, None)
        for i in range(len(q) - 2):
            for j in self.index.get(tuple(q[i:i + 3]), ()):
                k = 3
                while i + k < len(q) and j + k < len(self.tokens) and q[i + k] == self.tokens[j + k]:
                    k += 1
                if k > best[0]:
                    best = (k, i, j)
        if best[0]:
            return best
        for i in range(len(q) - 1):
            j = self.first2.get(tuple(q[i:i + 2]))
            if j is not None:
                return (2, i, j)
        for i, t in enumerate(q):
            j = self.first1.get(t)
            if j is not None:
                return (1, i, j)
        return best


def item_fields(item: dict, lang: str):
    """(path, text) for every text field of a reading item in one language."""
    c = item["content"]
    for field in ("title", "summary", "why", "seek_help_when", "cautions"):
        if field in c:
            yield f"content.{field}.{lang}", c[field][lang]
    for i, step in enumerate(c.get("steps", {}).get(lang, [])):
        yield f"content.steps.{lang}[{i}]", step
    for si, sec in enumerate(c.get("sections", [])):
        yield f"content.sections[{si}].heading.{lang}", sec["heading"][lang]
        for pi, par in enumerate(sec["paragraphs"]):
            yield f"content.sections[{si}].paragraphs[{pi}].{lang}", par[lang]


def compare(source: Source, fields, *, min_run: int = MIN_RUN, show_text: bool = False):
    """Per field: its word count and longest shared run; runs of min_run or more carry a locator."""
    out = []
    for key, text in fields:
        q = tokens(text)
        k, i, j = source.longest_run(text)
        row = {"field": key, "words": len(q), "longest_shared_run_words": k}
        if k >= min_run:
            row["locator"] = source.locator(j)
            if show_text:
                row["run_text"] = " ".join(q[i:i + k])
        out.append(row)
    return out


def main(argv=None):
    ap = argparse.ArgumentParser(description=__doc__.split("\n")[0])
    ap.add_argument("--source", required=True, help="local copy of the source (.html, or .txt from pdftotext)")
    ap.add_argument("--items", help="reading items file (JSON)")
    ap.add_argument("--ids", help="comma-separated item ids (default: all items in the file)")
    ap.add_argument("--lang", choices=("id", "en"), default="en", help="language of the card text to compare")
    ap.add_argument("--text", action="append", default=[], help="a free text to compare (repeatable)")
    ap.add_argument("--min-run", type=int, default=MIN_RUN)
    ap.add_argument("--json", action="store_true")
    ap.add_argument("--show-text", action="store_true", help="print matched words (local use only; never commit)")
    args = ap.parse_args(argv)

    source = Source(Path(args.source))
    fields = [(f"text[{n}]", t) for n, t in enumerate(args.text)]
    if args.items:
        doc = json.loads(Path(args.items).read_text(encoding="utf-8"))
        wanted = set(args.ids.split(",")) if args.ids else None
        for item in doc["items"]:
            if wanted is None or item["id"] in wanted:
                fields += [(f"{item['id']}:{p}", t) for p, t in item_fields(item, args.lang)]
    rows = compare(source, fields, min_run=args.min_run, show_text=args.show_text)
    longest = max((r["longest_shared_run_words"] for r in rows), default=0)
    result = {"method": METHOD, "source_sha256": source.sha256, "min_run": args.min_run,
              "fields": len(rows), "longest_shared_run_words": longest,
              "at_or_over_min_run": [r for r in rows if r["longest_shared_run_words"] >= args.min_run]}
    if args.json:
        result["rows"] = rows
        print(json.dumps(result, ensure_ascii=False, indent=2))
    else:
        print(f"{METHOD}: source sha256 {source.sha256}; {len(rows)} fields; longest shared run {longest} words")
        for r in result["at_or_over_min_run"]:
            extra = f": {r['run_text']}" if "run_text" in r else ""
            print(f"  {r['field']}: run {r['longest_shared_run_words']}/{r['words']} words at {r['locator']}{extra}")
    return 0


if __name__ == "__main__":
    sys.exit(main())
