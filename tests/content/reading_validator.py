"""Validator for the Sela reading library (data/content/reading/). Python 3.10+ standard library only.

What it checks (see docs/content/READING_LIBRARY.md):

- every items file, sources.json and set.json against reading_item.schema.json (a small
  JSON Schema subset is implemented here: $ref, anyOf, type, const, enum, pattern, format
  "date", min/max length, min/max items, uniqueItems, minimum/maximum, required,
  properties, additionalProperties);
- the rules the schema cannot express: kind-specific fields (2-5 steps for an exercise, 2-6 for
  a safety plan, and a title or summary that states a step count must match it), the WHO
  CC BY-NC-SA 3.0 IGO attribution (licence URI and source link included) and adaptation notice,
  the directory's LICENSE.md notice, the commercial-use blocker B-14, audience rules (adult
  evidence is never generalised to 15-17), the basis of every guide paragraph, declared
  contacts against the help catalogue, faithful migration of the 30 Dengar.in v1 cards (a
  changed card says so, bumps its version and keeps the v1 hash), and the review status
  (every item stays machine_drafted_unreviewed until a human editor approves the exact text);
- that the stored machine checks are current: reading time, the plain-language metrics,
  the claim-gate hits and the text hashes are recomputed and must match;
- that every summary or "why" which names a source (WHO, CDC, Kemenkes, Stanley & Brown, "health
  agencies") carries a source_claims entry: the source ids, a locator, a quote-free paraphrase of what
  the source says (sela-verbatim-overlap-v1 run under 5 words) and a support verdict, which a human
  editor must confirm for the exact text before the item can be published;
- that every source whose text is adapted or paraphrased records a verbatim check
  (tests/content/verbatim_overlap.py); while it has not run, its items stay unpublishable, keep an
  open editor flag, are not cleared for commercial use, and nothing says "no text copied" without
  "not yet verified";
- across the repository (git-tracked and untracked, not ignored, text files): any file that holds
  CC BY-NC-SA 3.0 IGO card text (8-word runs of the WHO cards' current or v1 text) must carry the
  licence URI, each source's IRIS link and the adaptation notice. This scan makes the validator fail
  while packages/config/src/selfHelpCards.v1.json holds the 22 WHO adaptations without them.

Claim gates: the 13 product-claim gates published in
data/research/corpus/eval_fixtures/claim_gates.json (used as published, never edited here)
plus the reading-specific gates in data/content/reading/content_gates.json, which are wrapped
with the same negation-window prefix that claim_gates.json uses. A hit means a person rewords
or justifies the text; it is a prompt for review, not a verdict.

CLI:
  python3 tests/content/reading_validator.py              validate; exit 1 on any error
  python3 tests/content/reading_validator.py --update     recompute the machine-check fields
                                                          and rewrite the files canonically
                                                          (never touches review fields)
  python3 tests/content/reading_validator.py --check-stale [--today YYYY-MM-DD]
                                                          list sources/contacts past their
                                                          re-verify-by date, and verbatim
                                                          checks past their due date; exit 1 if any
"""
from __future__ import annotations

import argparse
import datetime as _dt
import hashlib
import json
import math
import os
import re
import subprocess
import sys
from pathlib import Path

HERE = Path(__file__).resolve().parent
if str(HERE) not in sys.path:
    sys.path.insert(0, str(HERE))

import verbatim_overlap as VO  # noqa: E402  (same directory; standard library only)

REPO = HERE.parents[1]
CONTENT = REPO / "data" / "content" / "reading"
SCHEMA_PATH = CONTENT / "reading_item.schema.json"
SET_PATH = CONTENT / "set.json"
SOURCES_PATH = CONTENT / "sources.json"
CONTENT_GATES_PATH = CONTENT / "content_gates.json"
LICENCE_NOTICE_PATH = CONTENT / "LICENSE.md"
CLAIM_GATES_PATH = REPO / "data" / "research" / "corpus" / "eval_fixtures" / "claim_gates.json"
V1_CARDS_PATH = REPO / "packages" / "config" / "src" / "selfHelpCards.v1.json"
HELP_CATALOGUE_PATH = REPO / "web" / "src" / "data" / "help.fallback.json"

LANGS = ("id", "en")
PLAIN_METHOD = "sela-plain-language-v1"
WORDS_PER_MINUTE = 180
MAX_WORDS_PER_SENTENCE = 20
MAX_AVG_WORDS = 15.0
MAX_FK_GRADE = 8.0
V1_GATE_SET = "dengarin-v1-forbidden-terms"
WHO_LICENCE = "CC BY-NC-SA 3.0 IGO"
# The licence URI as printed on the copyright page of every adapted WHO PDF. CC BY-NC-SA 3.0 IGO
# section 4(b)(I) requires it with every copy of each adaptation, so each card's attribution carries it.
WHO_LICENCE_URL = "https://creativecommons.org/licenses/by-nc-sa/3.0/igo"
WHO_LEGALCODE_URL = WHO_LICENCE_URL + "/legalcode"
COMMERCIAL_BLOCKER = "B-14"
ADULT_ONLY_POPULATIONS = {"adults", "workers", "clinical_population"}
WHO_404_URL = "https://www.who.int/publications/i/item/9789240112360"
# sprc.org: the US Suicide Prevention Resource Center, which publishes the Stanley & Brown quick guide
# with the authors' permission (the authorised open copy, not a mirror).
APPROVED_HOSTS = (
    "www.who.int", "iris.who.int", "www.cdc.gov", "ayosehat.kemkes.go.id", "keslan.kemkes.go.id",
    "kesprimkom.kemkes.go.id", "www.kemkes.go.id", "www.healing119.id", "doi.org", "www.w3.org",
    "sprc.org",
)
# An exercise ("latihan kecil") has 2-5 steps; a safety plan keeps the six steps of the
# Stanley & Brown structure.
MAX_STEPS = {"exercise": 5, "safety_plan": 6}
_NUMBER_WORDS = {
    "en": {"two": 2, "three": 3, "four": 4, "five": 5, "six": 6, "seven": 7, "eight": 8, "nine": 9, "ten": 10},
    "id": {"dua": 2, "tiga": 3, "empat": 4, "lima": 5, "enam": 6, "tujuh": 7, "delapan": 8, "sembilan": 9,
           "sepuluh": 10},
}
_STATED_STEPS = {
    "en": re.compile(r"(?i)\b(" + "|".join(_NUMBER_WORDS["en"]) + r"|[0-9]+)\s+(?:[A-Za-z+]+\s+)?steps?\b"),
    "id": re.compile(r"(?i)\b(" + "|".join(_NUMBER_WORDS["id"]) + r"|[0-9]+)\s+langkah\b"),
}
GENERAL_ADVICE_LABEL = {"id": "saran umum", "en": "general advice"}

# A summary or "why" that credits a source with a statement needs a source_claims entry.
SOURCE_NAMES = (
    ("WHO", re.compile(r"\bWHO\b")),
    ("CDC", re.compile(r"\bCDC\b")),
    ("Kemenkes", re.compile(r"\bKemenkes\b|\bKementerian Kesehatan\b|\bMinistry of Health\b")),
    ("Stanley & Brown", re.compile(r"\bStanley\b")),
    ("health agencies", re.compile(r"(?i)\bhealth agencies\b|\blembaga kesehatan\b")),
)
CLAIM_FIELDS = ("summary", "why")
QUOTE_MARKS = "\"“”„«»"
MIN_SHARED_RUN = VO.MIN_RUN  # a shared run of 5 or more words counts as a quotation
# "No text copied" is a claim; until the verbatim check has run it must say it is not yet verified.
COPY_CLAIM = re.compile(r"(?i)no text (?:was )?copied|without copying|tanpa menyalin")
NOT_YET_VERIFIED = re.compile(r"(?i)not yet verified|belum diverifikasi")
ARCHIVE_HOSTS = ("web.archive.org",)

# Repository licence scan: CC BY-NC-SA card text must not travel without its notice.
LICENCE_SCAN_RUN = 8
LICENCE_SCAN_MAX_BYTES = 32_000_000
NOTICE_MARKERS = ("not responsible", "tidak bertanggung jawab")
_SKIP_DIRS = {".git", "node_modules", "dist", "build", "__pycache__", ".venv", "venv"}

# Dengar.in v1 card test (tests/carebridge/selfHelpCards.test.mjs): forbidden terms with a prefix
# word boundary and the "never promises outcomes" patterns. Applied to migrated cards only, as a
# regression gate; they have no negation handling, so they would wrongly flag the disclaimers
# that the new guides need ("Sela tidak memberi diagnosis").
V1_FORBIDDEN_TERMS = (
    "diagnos", "depresi berat", "gangguan", "obat", "mg", "dosis", "antidepresan", "terapi kami",
    "sembuh total", "prescription", "medication", "dose", "cure",
)
V1_BANNED = (
    r"pasti sembuh", r"dijamin", r"\bguarantee", r"you have (depression|anxiety)",
    r"kamu (mengalami|menderita) (depresi|kecemasan)",
)

_WORD = re.compile(r"[0-9A-Za-zÀ-ɏ]+(?:['’-][0-9A-Za-zÀ-ɏ]+)*")
_SENT_SPLIT = re.compile(r"(?<=[.!?])[\"'”’)]*\s+")
_ANDA = re.compile(r"\bAnda\b")
_EMAIL = re.compile(r"[\w.+-]+@[\w-]+\.[\w.]+")
_DOMAIN = re.compile(r"(?i)\b(?:https?://|www\.)\S+|\b[a-z0-9-]+\.(?:go\.id|id|com|org|net|int)\b")
_DIGITS = re.compile(r"(?<![0-9A-Za-zÀ-ɏ])[0-9]{3,}(?![0-9])")
_YEAR_OR_PERCENT = re.compile(r"(?<![0-9])(?:19|20)[0-9]{2}(?![0-9])|(?<![0-9])[0-9]{3,}\s*(?:persen|percent|%)", re.I)
_NIK = re.compile(r"(?<![0-9])[0-9]{16}(?![0-9])")


# ---------------------------------------------------------------- loading and formatting

def load_json(path: Path):
    return json.loads(path.read_text(encoding="utf-8"))


def dump_json(obj) -> str:
    """Canonical file form: 2-space indent, UTF-8, keys in authored order, trailing newline."""
    return json.dumps(obj, ensure_ascii=False, indent=2) + "\n"


def canonical(obj) -> bytes:
    return json.dumps(obj, ensure_ascii=False, sort_keys=True, separators=(",", ":")).encode("utf-8")


def sha256(data: bytes) -> str:
    return hashlib.sha256(data).hexdigest()


def text_sha256(item) -> str:
    return sha256(canonical(item["content"]))


# ---------------------------------------------------------------- JSON Schema subset

def _is_type(value, name):
    if name == "integer":
        return isinstance(value, int) and not isinstance(value, bool)
    if name == "number":
        return isinstance(value, (int, float)) and not isinstance(value, bool)
    return {
        "string": isinstance(value, str), "array": isinstance(value, list), "object": isinstance(value, dict),
        "boolean": isinstance(value, bool), "null": value is None,
    }[name]


def _resolve(root, ref):
    if not ref.startswith("#/"):
        raise ValueError(f"only local $ref is supported: {ref}")
    node = root
    for part in ref[2:].split("/"):
        node = node[part]
    return node


def validate_schema(instance, schema, root, path="$"):
    errors = []
    if "$ref" in schema:
        errors += validate_schema(instance, _resolve(root, schema["$ref"]), root, path)
        schema = {k: v for k, v in schema.items() if k != "$ref"}
    if "anyOf" in schema and not any(not validate_schema(instance, s, root, path) for s in schema["anyOf"]):
        errors.append(f"{path}: matches none of anyOf")
    if "type" in schema:
        types = schema["type"] if isinstance(schema["type"], list) else [schema["type"]]
        if not any(_is_type(instance, t) for t in types):
            return errors + [f"{path}: expected type {schema['type']}, got {type(instance).__name__}"]
    if "const" in schema and instance != schema["const"]:
        errors.append(f"{path}: must equal {schema['const']!r}")
    if "enum" in schema and instance not in schema["enum"]:
        errors.append(f"{path}: {instance!r} not in {schema['enum']}")
    if isinstance(instance, str):
        if len(instance) < schema.get("minLength", 0):
            errors.append(f"{path}: shorter than {schema['minLength']}")
        if "maxLength" in schema and len(instance) > schema["maxLength"]:
            errors.append(f"{path}: longer than {schema['maxLength']}")
        if "pattern" in schema and not re.search(schema["pattern"], instance):
            errors.append(f"{path}: {instance!r} does not match {schema['pattern']}")
        if schema.get("format") == "date":
            try:
                _dt.date.fromisoformat(instance)
            except ValueError:
                errors.append(f"{path}: {instance!r} is not a valid date")
    if _is_type(instance, "number"):
        if "minimum" in schema and instance < schema["minimum"]:
            errors.append(f"{path}: below minimum {schema['minimum']}")
        if "maximum" in schema and instance > schema["maximum"]:
            errors.append(f"{path}: above maximum {schema['maximum']}")
    if isinstance(instance, list):
        if len(instance) < schema.get("minItems", 0):
            errors.append(f"{path}: fewer than {schema['minItems']} items")
        if "maxItems" in schema and len(instance) > schema["maxItems"]:
            errors.append(f"{path}: more than {schema['maxItems']} items")
        if schema.get("uniqueItems") and len({canonical(x) for x in instance}) != len(instance):
            errors.append(f"{path}: items are not unique")
        if "items" in schema:
            for i, x in enumerate(instance):
                errors += validate_schema(x, schema["items"], root, f"{path}[{i}]")
    if isinstance(instance, dict):
        for key in schema.get("required", []):
            if key not in instance:
                errors.append(f"{path}: missing required {key!r}")
        props = schema.get("properties", {})
        extra = schema.get("additionalProperties", True)
        for key, value in instance.items():
            if key in props:
                errors += validate_schema(value, props[key], root, f"{path}.{key}")
            elif extra is False:
                errors.append(f"{path}: unexpected property {key!r}")
            elif isinstance(extra, dict):
                errors += validate_schema(value, extra, root, f"{path}.{key}")
    return errors


# ---------------------------------------------------------------- text access

def user_texts(item, *, body_only=False):
    """(path, lang, text) for user-facing content. body_only drops titles and headings."""
    c = item["content"]
    fields = ("summary", "why", "seek_help_when", "cautions") if body_only else ("title", "summary", "why", "seek_help_when", "cautions")
    for field in fields:
        if field in c:
            for lang in LANGS:
                yield f"content.{field}.{lang}", lang, c[field][lang]
    if "steps" in c:
        for lang in LANGS:
            for i, step in enumerate(c["steps"][lang]):
                yield f"content.steps.{lang}[{i}]", lang, step
    for si, sec in enumerate(c.get("sections", [])):
        if not body_only:
            for lang in LANGS:
                yield f"content.sections[{si}].heading.{lang}", lang, sec["heading"][lang]
        for pi, par in enumerate(sec["paragraphs"]):
            for lang in LANGS:
                yield f"content.sections[{si}].paragraphs[{pi}].{lang}", lang, par[lang]


def scanned_texts(item):
    """Everything the claim gates scan: user-facing text, notices and the internal evidence note."""
    yield from user_texts(item)
    for field in ("attribution", "adaptation_notice"):
        if field in item:
            for lang in LANGS:
                yield f"{field}.{lang}", lang, item[field][lang]
    if "evidence_note" in item:
        yield "evidence_note", "en", item["evidence_note"]


# ---------------------------------------------------------------- plain language

def words(text):
    return _WORD.findall(text)


def sentences(text):
    return [s for s in _SENT_SPLIT.split(text.strip()) if words(s)]


def syllables(word):
    """English syllable heuristic (vowel groups, common silent endings). Approximate by design."""
    w = re.sub(r"[^a-z]", "", word.lower())
    if not w:
        return 1
    if len(w) <= 3:
        return 1
    w = re.sub(r"(?:[^laeiouy]es|ed|[^laeiouy]e)$", "", w)
    w = re.sub(r"^y", "", w)
    return max(1, len(re.findall(r"[aeiouy]{1,2}", w)))


def plain_language(item):
    metrics, flags = {}, []
    for lang in LANGS:
        sent_words, syll = [], 0
        for path, text_lang, text in user_texts(item, body_only=True):
            if text_lang != lang:
                continue
            for sentence in sentences(text):
                ws = words(sentence)
                sent_words.append(len(ws))
                if len(ws) > MAX_WORDS_PER_SENTENCE:
                    flags.append(f"{lang}:sentence_over_{MAX_WORDS_PER_SENTENCE}_words:{path}:{len(ws)}")
                if lang == "en":
                    syll += sum(syllables(w) for w in ws)
        n_sent, n_words = len(sent_words), sum(sent_words)
        avg = round(n_words / n_sent, 1)
        fk = round(0.39 * (n_words / n_sent) + 11.8 * (syll / n_words) - 15.59, 1) if lang == "en" else None
        metrics[lang] = {
            "sentences": n_sent, "words": n_words, "avg_words_per_sentence": avg,
            "max_words_per_sentence": max(sent_words),
            "sentences_over_20_words": sum(1 for n in sent_words if n > MAX_WORDS_PER_SENTENCE),
            "fk_grade": fk,
        }
        if avg > MAX_AVG_WORDS:
            flags.append(f"{lang}:avg_words_per_sentence_over_{MAX_AVG_WORDS:g}:{avg}")
        if fk is not None and fk > MAX_FK_GRADE:
            flags.append(f"en:fk_grade_over_{MAX_FK_GRADE:g}:{fk}")
    return {
        "status": "machine_checked_unreviewed", "method": PLAIN_METHOD, "metrics": metrics, "flags": flags,
        "result": "within_targets" if not flags else "needs_editor",
    }


def reading_time(item):
    out = {}
    for lang in LANGS:
        n = sum(len(words(t)) for _, l, t in user_texts(item) if l == lang)
        out[lang] = max(1, math.ceil(n / WORDS_PER_MINUTE))
    return out


# ---------------------------------------------------------------- claim gates

def negation_prefix(claim_gates):
    """The negation-window prefix shared by every published pattern of sela-claim-gates-v2."""
    prefix = None
    for gate in claim_gates["gates"]:
        if len(gate["patterns"]) != len(gate["claim_patterns"]):
            raise ValueError(f"claim gate {gate['id']}: patterns and claim_patterns differ in length")
        for bare, published in zip(gate["claim_patterns"], gate["patterns"]):
            suffix = "(" + bare + ")"
            if not published.endswith(suffix):
                raise ValueError(f"claim gate {gate['id']}: published pattern does not end with its claim")
            p = published[: -len(suffix)]
            if prefix is None:
                prefix = p
            elif p != prefix:
                raise ValueError(f"claim gate {gate['id']}: negation prefix differs from the other gates")
    return prefix


def load_gate_sets():
    claim_gates = load_json(CLAIM_GATES_PATH)
    content_gates = load_json(CONTENT_GATES_PATH)
    prefix = negation_prefix(claim_gates)
    product = [(g["id"], [re.compile(p, re.I) for p in g["patterns"]]) for g in claim_gates["gates"]]
    reading = [(g["id"], [re.compile(prefix + "(" + p + ")", re.I) for p in g["claim_patterns"]]) for g in content_gates["gates"]]
    return [(claim_gates["version"], product), (content_gates["version"], reading)]


_V1_TERM_PATTERNS = [(t, re.compile(r"(^|[^a-z0-9])" + re.escape(t), re.I)) for t in V1_FORBIDDEN_TERMS]
_V1_BANNED_PATTERNS = [re.compile(p, re.I) for p in V1_BANNED]


def scan_text(text, gate_sets):
    hits = []
    for set_id, gates in gate_sets:
        for gate_id, patterns in gates:
            for pattern in patterns:
                for m in pattern.finditer(text):
                    hits.append((set_id, gate_id, m.start(1) if m.re.groups else m.start(), m.group(1) if m.re.groups else m.group(0)))
    return hits


def scan_v1_terms(text):
    hits = [(V1_GATE_SET, f"term:{t}", m.start(), m.group(0).strip()) for t, p in _V1_TERM_PATTERNS for m in [p.search(text)] if m]
    hits += [(V1_GATE_SET, f"banned:{p.pattern}", m.start(), m.group(0)) for p in _V1_BANNED_PATTERNS for m in [p.search(text)] if m]
    return hits


def claims_check(item, gate_sets, existing=None):
    migrated = item["origin"]["kind"] == "migrated_v1"
    found, seen = [], set()
    for path, _lang, text in scanned_texts(item):
        raw = scan_text(text, gate_sets)
        if migrated and path.startswith("content."):
            raw += scan_v1_terms(text)
        for set_id, gate, start, match in sorted(raw, key=lambda h: (h[2], h[0], h[1])):
            key = (set_id, gate, path, match)
            if key not in seen:
                seen.add(key)
                found.append({"gate_set": set_id, "gate": gate, "path": path, "match": match})
    sets = [s for s, _ in gate_sets] + ([V1_GATE_SET] if migrated else [])
    return {
        "status": "machine_checked_unreviewed",
        "gate_sets": sets,
        "hits": found,
        "result": "hits" if found else "pass",
        "editor_flags": list((existing or {}).get("editor_flags", [])),
    }


def gate_examples_errors(gate_sets=None):
    """Forbidden examples must match their gate and allowed examples must not, in both gate files."""
    errors = []
    gate_sets = gate_sets or load_gate_sets()
    for path in (CLAIM_GATES_PATH, CONTENT_GATES_PATH):
        doc = load_json(path)
        version = doc["version"]
        compiled = dict(dict(gate_sets)[version])
        for gate in doc["gates"]:
            patterns = compiled[gate["id"]]
            for ex in gate.get("forbidden_examples", []):
                if not any(p.search(ex) for p in patterns):
                    errors.append(f"{version}:{gate['id']}: forbidden example not matched: {ex!r}")
            for ex in gate.get("allowed_examples", []):
                if any(p.search(ex) for p in patterns):
                    errors.append(f"{version}:{gate['id']}: allowed example matched: {ex!r}")
    return errors


# ---------------------------------------------------------------- v1 cards

def v1_content(card):
    c = {"title": card["title"], "summary": card["summary"], "steps": card["steps"], "why": card["why"],
         "seek_help_when": card["seekHelpWhen"]}
    if "cautions" in card:
        c["cautions"] = card["cautions"]
    return c


def load_v1_cards():
    if not V1_CARDS_PATH.exists():
        return None
    return {c["id"]: c for c in load_json(V1_CARDS_PATH)}


# ---------------------------------------------------------------- library

def load_library():
    schema = load_json(SCHEMA_PATH)
    set_doc = load_json(SET_PATH)
    sources = load_json(SOURCES_PATH)
    files = {f["path"]: load_json(CONTENT / f["path"]) for f in set_doc["files"]}
    return schema, set_doc, sources, files


def all_items(files):
    for path, doc in files.items():
        for item in doc["items"]:
            yield path, item


def fingerprint(items):
    rows = sorted([item["id"], item["version"], item["text_sha256"]] for item in items)
    return sha256(canonical(rows))


def compute_counts(items):
    items = list(items)
    counts = {"items": len(items)}
    for key, values in (("kind", ("exercise", "guide", "safety_plan")), ("audience", ("adult", "15-17", "all"))):
        for v in values:
            counts[f"{key}:{v}"] = sum(1 for i in items if i[key] == v)
    counts["origin:migrated_v1"] = sum(1 for i in items if i["origin"]["kind"] == "migrated_v1")
    counts["origin:new_v3"] = sum(1 for i in items if i["origin"]["kind"] == "new_v3")
    counts["licence:CC BY-NC-SA 3.0 IGO"] = sum(1 for i in items if WHO_LICENCE in i["licence"]["source_licence"])
    for v in ("blocked", "needs_legal_confirmation", "allowed"):
        counts[f"commercial_use:{v}"] = sum(1 for i in items if i["licence"]["commercial_use"] == v)
    for v in ("machine_drafted_unreviewed", "human_changes_requested", "human_approved"):
        counts[f"review:{v}"] = sum(1 for i in items if i["review"]["status"] == v)
    counts["publishable"] = sum(1 for i in items if i["review"]["publishable"])
    counts["claims:hits"] = sum(1 for i in items if i["checks"]["claims"]["result"] == "hits")
    counts["editor_flags:open"] = sum(1 for i in items for f in i["checks"]["claims"]["editor_flags"] if f["status"] == "open")
    counts["plain_language:needs_editor"] = sum(1 for i in items if i["checks"]["plain_language"]["result"] == "needs_editor")
    claims = [sc for i in items for sc in i.get("source_claims", [])]
    counts["source_claims"] = len(claims)
    for v in ("supported", "partly_supported", "not_supported", "not_checked"):
        counts[f"source_claims:{v}"] = sum(1 for sc in claims if sc["support"] == v)
    counts["source_claims:editor_confirmed"] = sum(1 for sc in claims if sc["editor_confirmation"]["status"] == "confirmed")
    counts["origin:text_changed"] = sum(1 for i in items if i["origin"].get("text_changed"))
    return counts


def _host(url):
    m = re.match(r"^https://([^/]+)/?", url)
    return m.group(1) if m else ""


def _strip_declared(text, item):
    values = sorted({v for c in item["contacts"] for v in c["values"]}, key=len, reverse=True)
    for v in values:
        text = text.replace(v, " ")
    return text


def _contains_url(text, url):
    """True when `url` occurs in `text` as a whole URL (not as the prefix of a longer one)."""
    return re.search(re.escape(url) + r"(?![0-9A-Za-z/_%#?=&-]|\.[0-9A-Za-z])", text) is not None


def _covered(path, flagged_paths):
    return any(path == fp or path.startswith(fp + ".") or path.startswith(fp + "[") for fp in flagged_paths)


def stated_step_counts(item):
    """(path, number) for every title or summary that states how many steps the item has."""
    out = []
    for field in ("title", "summary"):
        for lang in LANGS:
            for m in _STATED_STEPS[lang].finditer(item["content"][field][lang]):
                word = m.group(1).lower()
                out.append((f"content.{field}.{lang}", int(word) if word.isdigit() else _NUMBER_WORDS[lang][word]))
    return out


def source_link(source):
    """The link a WHO card's attribution gives for its source: the persistent IRIS handle, else the URL."""
    return source.get("persistent_url") or source["url"]


def named_sources(item):
    """{path: [names]} for every summary or 'why' that names a source in either language."""
    out = {}
    for field in CLAIM_FIELDS:
        if field not in item["content"]:
            continue
        text = " ".join(item["content"][field][lang] for lang in LANGS)
        names = [name for name, rx in SOURCE_NAMES if rx.search(text)]
        if names:
            out[f"content.{field}"] = names
    return out


def field_sha256(item, path):
    """SHA-256 of one content field (both languages) as canonical JSON: what an editor confirms."""
    return sha256(canonical(item["content"][path.split(".", 1)[1]]))


def source_claim_errors(item, p, sources_by_id):
    """A summary or 'why' that names a source says only what that source says, and shows where."""
    e = []
    flags = item["checks"]["claims"]["editor_flags"]
    open_paths = [f["path"] for f in flags if f["status"] == "open"]
    named = named_sources(item)
    seen = {}
    for ci, sc in enumerate(item.get("source_claims", [])):
        path = sc["path"]
        where = f"{p}: source_claims[{ci}] ({path})"
        if path in seen:
            e.append(f"{where}: more than one entry for this field")
        seen[path] = sc
        if path not in named:
            e.append(f"{where}: the field names no source; remove the entry or fix its path")
        elif sc["names"] != named[path]:
            e.append(f"{where}: names {sc['names']} do not match the names in the text {named[path]}")
        for sid in sc["source_ids"]:
            if sid not in sources_by_id:
                e.append(f"{where}: source {sid!r} is not in sources.json")
            elif sid not in item["source"]["source_ids"]:
                e.append(f"{where}: source {sid!r} is not one of the item's sources")
        says, qc = sc["source_says"], sc["quote_check"]
        if sc["support"] == "not_checked":
            if says is not None or qc is not None:
                e.append(f"{where}: a not_checked entry has no source_says and no quote_check")
            if not any(f["status"] == "open" and f["issue"] == "source_unreachable" and f.get("source_id") in sc["source_ids"]
                       for f in flags):
                e.append(f"{where}: not_checked needs an open source_unreachable editor flag for its source")
        else:
            if not says:
                e.append(f"{where}: needs source_says, what the source says in Sela's own words")
            else:
                if any(mark in says for mark in QUOTE_MARKS):
                    e.append(f"{where}: source_says paraphrases; it must not quote the source (no quotation marks)")
                field = item["content"][path.split(".", 1)[1]]
                if any(says.strip() == field[lang].strip() for lang in LANGS):
                    e.append(f"{where}: source_says describes the source; it must not repeat the card text")
            if qc is None:
                e.append(f"{where}: needs a quote_check ({VO.METHOD}) of source_says against the source")
            else:
                if qc["longest_shared_run_words"] >= MIN_SHARED_RUN:
                    e.append(f"{where}: source_says shares {qc['longest_shared_run_words']} words in a row with the source; "
                             f"reword it (a quote-free paraphrase shares fewer than {MIN_SHARED_RUN})")
                if not set(qc["compared_with"]) <= set(sc["source_ids"]):
                    e.append(f"{where}: quote_check.compared_with must be among the entry's sources")
            if sc["support"] != "supported" and not _covered(path, open_paths):
                e.append(f"{where}: support {sc['support']!r} needs an open editor flag on {path}")
        ec = sc["editor_confirmation"]
        if ec["status"] == "pending":
            if ec["editor"] is not None or ec["confirmed_on"] is not None or ec["confirmed_text_sha256"] is not None:
                e.append(f"{where}: a pending confirmation has no editor, date or text hash")
        else:
            if not ec["editor"] or not ec["confirmed_on"]:
                e.append(f"{where}: an editor confirmation needs the editor and the date")
            if item["review"]["status"] == "machine_drafted_unreviewed":
                e.append(f"{where}: only a human editor confirms a source claim, and this item is machine_drafted_unreviewed")
            if path.split(".", 1)[1] in item["content"]:
                if ec["confirmed_text_sha256"] != field_sha256(item, path):
                    e.append(f"{where}: the text changed after the editor confirmed it; the editor confirms it again")
    for path, names in named.items():
        if path not in seen:
            e.append(f"{p}: {path} names {', '.join(names)} but has no source_claims entry "
                     "(source ids, locator and a quote-free paraphrase of what the source says)")
    if item["review"]["publishable"]:
        for path, sc in seen.items():
            if sc["support"] != "supported" or sc["editor_confirmation"]["status"] != "confirmed":
                e.append(f"{p}: publishable needs every source claim supported and confirmed by the editor ({path})")
        for sid in item["source"]["source_ids"]:
            vc = sources_by_id.get(sid, {}).get("verbatim_check")
            if vc is not None and vc["status"] != "machine_checked_unreviewed":
                e.append(f"{p}: publishable needs the verbatim check of {sid} to have run")
    return e


def licence_notice_errors(text, sources):
    """LICENSE.md must give the licence, its URI, the scope, B-14, WHO's terms and every suggested citation."""
    required = [
        WHO_LICENCE, WHO_LICENCE_URL, WHO_LEGALCODE_URL, "cards_who.json", "packages/config/src/selfHelpCards.v1.json",
        "MIT", COMMERCIAL_BLOCKER, "D-045", "non-commercial", "share-alike", "logo", "endorse", "WIPO",
        "attribution", "adaptation_notice",
    ]
    errors = [f"LICENSE.md: must mention {r!r}" for r in required if r not in text]
    for s in sources["sources"]:
        if s["licence"].startswith(WHO_LICENCE) and s["kind"] == "who_publication" and s["use"].startswith("adapted"):
            for key in ("suggested_citation", "translation_disclaimer_verbatim"):
                if s.get(key) and s[key] not in text:
                    errors.append(f"LICENSE.md: must give {s['id']}'s {key} verbatim")
            if not _contains_url(text, source_link(s)):
                errors.append(f"LICENSE.md: must link {s['id']} ({source_link(s)})")
    return errors


def item_errors(item, *, file_path, file_doc, sources_by_id, gate_sets, v1_cards, help_catalogue):
    e = []
    iid = item["id"]
    p = f"{file_path}:{iid}"
    c = item["content"]
    kind = item["kind"]

    if item["languages"] != ["id", "en"]:
        e.append(f"{p}: languages must be ['id', 'en']")

    # kind-specific content
    if kind in ("exercise", "safety_plan"):
        for field in ("steps", "why", "seek_help_when"):
            if field not in c:
                e.append(f"{p}: {kind} needs content.{field}")
        if "sections" in c:
            e.append(f"{p}: {kind} must not have sections")
        if "steps" in c:
            n_id, n_en = len(c["steps"]["id"]), len(c["steps"]["en"])
            if n_id != n_en:
                e.append(f"{p}: id and en step counts differ ({n_id} vs {n_en})")
            if not 2 <= n_id <= MAX_STEPS[kind]:
                e.append(f"{p}: {kind} steps must number 2-{MAX_STEPS[kind]}, got {n_id}")
            flagged = [f["path"] for f in item["checks"]["claims"]["editor_flags"]]
            for path, stated in stated_step_counts(item):
                if stated != n_id and not _covered(path, flagged):
                    e.append(f"{p}: {path} states {stated} steps but the item has {n_id} (fix it or add an editor flag)")
    if kind == "guide":
        if "sections" not in c:
            e.append(f"{p}: guide needs content.sections")
        if "steps" in c:
            e.append(f"{p}: guide must not have steps")
    if kind == "safety_plan":
        if item["topics"] != ["safety_plan"]:
            e.append(f"{p}: safety_plan items carry only the safety_plan topic")
        if item["surfaces"] != ["help_page"]:
            e.append(f"{p}: safety_plan items are shown only on the help page")
        if "cautions" not in c:
            e.append(f"{p}: safety_plan items need cautions")
        for lang in LANGS:
            if "119 ext 8" not in c.get("seek_help_when", {}).get(lang, ""):
                e.append(f"{p}: safety_plan seek_help_when.{lang} must name 119 ext 8")
    elif "safety_plan" in item["topics"]:
        e.append(f"{p}: only safety_plan items may carry the safety_plan topic")
    if kind == "safety_plan" and item["ai_may_offer"]:
        e.append(f"{p}: the model may never offer a safety_plan item")

    # register, personal data, contacts
    for path, lang, text in user_texts(item):
        if lang == "id" and _ANDA.search(text):
            e.append(f"{p}: {path} uses formal 'Anda' (the reading register is 'kamu')")
        if _EMAIL.search(text) or _NIK.search(text):
            e.append(f"{p}: {path} looks like it contains personal data")
        rest = _strip_declared(text, item)
        for m in _DOMAIN.finditer(rest):
            e.append(f"{p}: {path} has an undeclared web address {m.group(0)!r}")
        rest = _YEAR_OR_PERCENT.sub(" ", rest)
        for m in _DIGITS.finditer(rest):
            e.append(f"{p}: {path} has an undeclared number {m.group(0)!r} (declare contacts in item.contacts)")
    all_user_text = "\n".join(t for _, _, t in user_texts(item))
    for contact in item["contacts"]:
        for v in contact["values"]:
            if v not in all_user_text:
                e.append(f"{p}: declared contact value {v!r} does not appear in the text")
        for sid in contact["source_ids"]:
            if sid not in sources_by_id:
                e.append(f"{p}: contact source {sid!r} is not in sources.json")
        if contact["reverify_by"] <= contact["checked_on"]:
            e.append(f"{p}: contact reverify_by must be after checked_on")
        if help_catalogue is not None:
            entry = help_catalogue.get(contact["catalogue_id"])
            if entry is None:
                e.append(f"{p}: contact catalogue_id {contact['catalogue_id']!r} is not in the help catalogue")
            else:
                phones = {ch["value"] for ch in entry.get("channels", []) if ch.get("kind") == "phone"}
                if contact["reverify_by"] > entry.get("reverifyBy", "9999-12-31"):
                    e.append(f"{p}: contact reverify_by is later than the help catalogue's ({entry.get('reverifyBy')})")
                if any(v.startswith("119") for v in contact["values"]) and "119 ext 8" not in phones:
                    e.append(f"{p}: help catalogue entry {contact['catalogue_id']} has no '119 ext 8' phone channel")

    # sources
    if item["source"]["relation"] != "sela_original" and not item["source"]["source_ids"]:
        e.append(f"{p}: adapted, paraphrased or structure-only items need at least one source")
    for sid in item["source"]["source_ids"]:
        if sid not in sources_by_id:
            e.append(f"{p}: source {sid!r} is not in sources.json")

    # licence, attribution, adaptation notice
    lic = item["licence"]
    is_who_cc = WHO_LICENCE in lic["source_licence"]
    in_who_file = WHO_LICENCE in file_doc["file_licence"]["text_licence"]
    if is_who_cc != in_who_file:
        e.append(f"{p}: CC BY-NC-SA 3.0 IGO items belong in the WHO file and only there")
    if is_who_cc:
        if lic["item_text_licence"] != WHO_LICENCE or not lic["share_alike"]:
            e.append(f"{p}: an adaptation of CC BY-NC-SA material is licensed CC BY-NC-SA 3.0 IGO (share-alike)")
        if not (lic["attribution_required"] and lic["adaptation_notice_required"]):
            e.append(f"{p}: WHO material needs attribution and the adaptation notice")
        if lic["commercial_use"] != "blocked" or lic.get("blocker") != COMMERCIAL_BLOCKER:
            e.append(f"{p}: WHO CC BY-NC-SA material is blocked for commercial use until {COMMERCIAL_BLOCKER} clears")
        if item["source"]["relation"] != "adapted":
            e.append(f"{p}: WHO cards are adaptations")
        if WHO_LICENCE_URL not in file_doc["file_licence"]["notice"] or "LICENSE.md" not in file_doc["file_licence"]["notice"]:
            e.append(f"{p}: the WHO file's file_licence.notice must give {WHO_LICENCE_URL} and point to LICENSE.md")
        cited = [sources_by_id[sid] for sid in item["source"]["source_ids"] if sid in sources_by_id]
        for lang in LANGS:
            att = item["attribution"][lang]
            if "WHO" not in att or WHO_LICENCE not in att:
                e.append(f"{p}: attribution.{lang} must name WHO and {WHO_LICENCE}")
            # CC BY-NC-SA 3.0 IGO s. 4(b)(I): the licence URI with every copy of each adaptation;
            # s. 4(d)(iii): the URI associated with the work. The attribution is shown with every view
            # and export, so both links live in its text.
            if not _contains_url(att, WHO_LICENCE_URL):
                e.append(f"{p}: attribution.{lang} must give the licence URI {WHO_LICENCE_URL}")
            for s in cited:
                if not _contains_url(att, source_link(s)):
                    e.append(f"{p}: attribution.{lang} must link its source {s['id']} ({source_link(s)})")
        notice = item.get("adaptation_notice")
        if not notice:
            e.append(f"{p}: WHO cards need an adaptation_notice")
        else:
            if not ("not responsible" in notice["en"] and "WHO" in notice["en"] and WHO_LICENCE in notice["en"]):
                e.append(f"{p}: adaptation_notice.en must say WHO is not responsible and give {WHO_LICENCE}")
            if not ("tidak bertanggung jawab" in notice["id"] and "WHO" in notice["id"] and WHO_LICENCE in notice["id"]):
                e.append(f"{p}: adaptation_notice.id must say WHO tidak bertanggung jawab and give {WHO_LICENCE}")
    if lic["adaptation_notice_required"] and "adaptation_notice" not in item:
        e.append(f"{p}: adaptation_notice_required but no adaptation_notice")

    # audience: adult evidence is never generalised to 15-17
    adult_sources = [sid for sid in item["source"]["source_ids"]
                     if sources_by_id.get(sid, {}).get("stated_population") in ADULT_ONLY_POPULATIONS]
    if item["audience"] in ("all", "15-17"):
        if item["source"]["relation"] in ("adapted", "paraphrased", "structure_only") and adult_sources:
            e.append(f"{p}: audience {item['audience']} cannot be adapted from adult-only sources {adult_sources}")
        for si, sec in enumerate(c.get("sections", [])):
            for pi, par in enumerate(sec["paragraphs"]):
                for b in par["basis"]:
                    if b["kind"] == "source" and b.get("support") == "paraphrase" and \
                            sources_by_id.get(b.get("source_id"), {}).get("stated_population") in ADULT_ONLY_POPULATIONS:
                        e.append(f"{p}: sections[{si}].paragraphs[{pi}] paraphrases an adult-only source for audience {item['audience']}")
    if item["audience"] == "15-17" and "availability_note" not in item:
        e.append(f"{p}: 15-17 items need an availability_note (the band is gated, C19)")
    if item["origin"]["kind"] == "migrated_v1" and item["audience"] != "adult" and item["review"]["teen_review"] != "reviewed_suitable":
        e.append(f"{p}: v1 cards were written for adults 18+; a wider audience needs a teen review")
    if item["review"]["teen_review"] == "reviewed_suitable" and item["review"]["status"] != "human_approved":
        e.append(f"{p}: a teen-suitability decision is a human decision")

    # guide paragraph basis
    for si, sec in enumerate(c.get("sections", [])):
        for pi, par in enumerate(sec["paragraphs"]):
            where = f"{p}: sections[{si}].paragraphs[{pi}]"
            for b in par["basis"]:
                k = b["kind"]
                if k == "source":
                    for field in ("source_id", "locator", "support"):
                        if field not in b:
                            e.append(f"{where}: a source basis needs {field}")
                    if b.get("source_id") and b["source_id"] not in sources_by_id:
                        e.append(f"{where}: source {b['source_id']!r} is not in sources.json")
                    if b.get("source_id") and b["source_id"] not in item["source"]["source_ids"]:
                        e.append(f"{where}: cited source {b['source_id']!r} is missing from item.source.source_ids")
                elif k == "product_basis":
                    if not b.get("refs") or "implementation_check" not in b:
                        e.append(f"{where}: a product basis needs refs and implementation_check")
                elif k == "general_advice":
                    if b.get("label") != GENERAL_ADVICE_LABEL:
                        e.append(f"{where}: general advice carries the label {GENERAL_ADVICE_LABEL}")
                elif k == "evidence_gap":
                    if not b.get("refs") or not b.get("note"):
                        e.append(f"{where}: an evidence gap needs refs and a note")

    # recomputed machine checks
    if item["text_sha256"] != text_sha256(item):
        e.append(f"{p}: text_sha256 is stale (run --update)")
    if item["reading_time_minutes"] != reading_time(item):
        e.append(f"{p}: reading_time_minutes is stale (run --update)")
    if item["checks"]["plain_language"] != plain_language(item):
        e.append(f"{p}: checks.plain_language is stale (run --update)")
    claims = item["checks"]["claims"]
    fresh = claims_check(item, gate_sets, claims)
    for key in ("gate_sets", "hits", "result", "status"):
        if claims[key] != fresh[key]:
            e.append(f"{p}: checks.claims.{key} is stale (run --update)")
    flagged_paths = [f["path"] for f in claims["editor_flags"]]
    for hit in claims["hits"]:
        if not _covered(hit["path"], flagged_paths):
            e.append(f"{p}: claim-gate hit at {hit['path']} ({hit['gate']}: {hit['match']!r}) has no editor flag")
    for flag in claims["editor_flags"]:
        if flag.get("source_id") and flag["source_id"] not in sources_by_id:
            e.append(f"{p}: editor flag cites unknown source {flag['source_id']!r}")

    # a summary or 'why' that names a source
    e += source_claim_errors(item, p, sources_by_id)

    # review status
    r = item["review"]
    if r["status"] == "machine_drafted_unreviewed":
        if r["human_editor"] is not None or r["reviewed_on"] is not None or r["approved_text_sha256"] is not None:
            e.append(f"{p}: an unreviewed item has no editor, review date or approved hash")
        if r["publishable"]:
            e.append(f"{p}: machine_drafted_unreviewed items are never publishable")
    if r["publishable"]:
        open_flags = [f for f in claims["editor_flags"] if f["status"] == "open"]
        if r["status"] != "human_approved" or not r["human_editor"] or not r["reviewed_on"]:
            e.append(f"{p}: publishable needs a human approval with editor and date")
        if r["approved_text_sha256"] != item["text_sha256"]:
            e.append(f"{p}: the approved text hash does not match the current text")
        if claims["result"] != "pass" or open_flags:
            e.append(f"{p}: publishable needs a clean claims check and no open editor flags")
        if r["verifier"]["status"] != "supported":
            e.append(f"{p}: publishable needs the authoring-time verifier verdict 'supported' (V36)")

    # origin and faithful migration
    o = item["origin"]
    if o["kind"] == "migrated_v1":
        for field in ("migrated_from", "v1_id", "v1_text_sha256", "v1_source", "v1_evidence_note", "text_changed", "migrated_on"):
            if field not in o:
                e.append(f"{p}: migrated item needs origin.{field}")
        if o.get("v1_id") != iid:
            e.append(f"{p}: migrated items keep their v1 id")
        if item.get("evidence_note") != o.get("v1_evidence_note"):
            e.append(f"{p}: evidence_note must stay the v1 evidenceNote")
        if not o.get("text_changed") and o.get("v1_text_sha256") != item["text_sha256"]:
            e.append(f"{p}: text differs from the v1 hash but origin.text_changed is false")
        if o.get("text_changed"):
            if o.get("v1_text_sha256") == item["text_sha256"]:
                e.append(f"{p}: origin.text_changed is true but the text equals the v1 card")
            if item["version"] < 2:
                e.append(f"{p}: a migrated card whose text changed bumps its version")
            if f"version {item['version']}" not in item["adaptation_note"]:
                e.append(f"{p}: adaptation_note must record what changed in version {item['version']} and why")
            if not any(f["status"] == "open" for f in item["checks"]["claims"]["editor_flags"]):
                e.append(f"{p}: a changed migrated card keeps an open editor flag until the editor confirms or reverts the change")
        if v1_cards is not None:
            card = v1_cards.get(iid)
            if card is None:
                e.append(f"{p}: no v1 card with this id in {V1_CARDS_PATH.relative_to(REPO)}")
            else:
                if o.get("v1_text_sha256") != sha256(canonical(v1_content(card))):
                    e.append(f"{p}: origin.v1_text_sha256 does not match the v1 card")
                if not o.get("text_changed") and c != v1_content(card):
                    e.append(f"{p}: text is not identical to the v1 card")
                if o.get("v1_source") != card["source"]:
                    e.append(f"{p}: origin.v1_source must be the v1 source object, verbatim")
                if o.get("v1_evidence_note") != card["evidenceNote"]:
                    e.append(f"{p}: origin.v1_evidence_note must be the v1 evidenceNote, verbatim")
    else:
        if item["source"]["relation"] != "sela_original":
            e.append(f"{p}: new items are Sela's own text (relation sela_original)")
        if "drafted_on" not in o:
            e.append(f"{p}: new items need origin.drafted_on")

    # the 404 form of the WHO loneliness URL must not come back
    if WHO_404_URL in json.dumps(item, ensure_ascii=False):
        e.append(f"{p}: uses the 404 URL {WHO_404_URL}")
    return e


def _host_ok(url, kind):
    host = _host(url)
    return host in APPROVED_HOSTS or (kind == "archived_copy" and host in ARCHIVE_HOSTS)


def _unqualified_copy_claim(text):
    return bool(text) and COPY_CLAIM.search(text) is not None and NOT_YET_VERIFIED.search(text) is None


def verbatim_check_errors(sources, files):
    """Every source whose text is adapted or paraphrased records sela-verbatim-overlap-v1 (counts and locators)."""
    errors = []
    items = [i for _p, i in all_items(files)]
    cited_by = {}
    for item in items:
        for sid in item["source"]["source_ids"]:
            cited_by.setdefault(sid, []).append(item)
    file_of = {i["id"]: path for path, i in all_items(files)}
    for s in sources["sources"]:
        sid, vc = s["id"], s.get("verbatim_check")
        where = f"sources.json:{sid}: verbatim_check"
        if s["use"].startswith(("adapted", "paraphrased")) and vc is None:
            errors.append(f"sources.json:{sid}: its text is {s['use'].split(':')[0]}, so it needs a verbatim_check "
                          f"({VO.METHOD}, tests/content/verbatim_overlap.py)")
        if vc is None:
            continue
        users = cited_by.get(sid, [])
        if sorted(vc["items_compared"]) != sorted(i["id"] for i in users):
            errors.append(f"{where}: items_compared must be exactly the items that cite this source "
                          f"({sorted(i['id'] for i in users)})")
        if vc["status"] == "machine_checked_unreviewed":
            for key in ("checked_on", "copy", "longest_shared_run_words"):
                if key not in vc:
                    errors.append(f"{where}: a check that ran records {key}")
            if "copy" in vc and not _host_ok(vc["copy"]["url"], vc["copy"]["kind"]):
                errors.append(f"{where}: copy host {_host(vc['copy']['url'])!r} is neither the publisher's approved host "
                              f"nor an archive ({', '.join(ARCHIVE_HOSTS)}) for an archived_copy")
            runs = vc.get("runs_at_or_over_5_words", [])
            longest = vc.get("longest_shared_run_words", 0)
            for run in runs:
                if run["item_id"] not in vc["items_compared"]:
                    errors.append(f"{where}: run listed for {run['item_id']}, which was not compared")
            if runs and longest != max(r["words"] for r in runs):
                errors.append(f"{where}: longest_shared_run_words must equal the longest listed run")
            if not runs and longest >= MIN_SHARED_RUN:
                errors.append(f"{where}: list every run of {MIN_SHARED_RUN} or more words (locators only)")
            if s.get("commercial_use") == "needs_legal_confirmation":
                # no licence statement: shared wording is a question for the editor and the legal review
                by_id = {i["id"]: i for i in users}
                for run in runs:
                    item = by_id.get(run["item_id"])
                    if item is None:
                        continue
                    field_path = run["field"].rsplit(".", 1)[0] if run["field"].endswith((".id", ".en")) else run["field"]
                    open_paths = [f["path"] for f in item["checks"]["claims"]["editor_flags"] if f["status"] == "open"]
                    if not _covered(field_path, open_paths) and not _covered(run["field"], open_paths):
                        errors.append(f"{where}: {run['item_id']} {run['field']} shares {run['words']} words with a source "
                                      "that states no licence; it needs an open editor flag")
        else:
            if not vc.get("attempts"):
                errors.append(f"{where}: a check that has not run records the attempts")
            if "due_by" not in vc:
                errors.append(f"{where}: a check that has not run needs due_by")
            elif vc["due_by"] > s["reverify_by"]:
                errors.append(f"{where}: due_by must not be later than the source's reverify_by ({s['reverify_by']})")
            for text in (s["licence"], s["check"]):
                if _unqualified_copy_claim(text):
                    errors.append(f"sources.json:{sid}: says no text was copied, but the verbatim check has not run "
                                  "(say 'not yet verified')")
            for item in users:
                ip = f"{file_of[item['id']]}:{item['id']}"
                if item["licence"]["commercial_use"] == "allowed":
                    errors.append(f"{ip}: commercial use cannot be allowed while the verbatim check of {sid} has not run")
                if not any(f["status"] == "open" and f.get("source_id") == sid for f in item["checks"]["claims"]["editor_flags"]):
                    errors.append(f"{ip}: needs an open editor flag for {sid} until its verbatim check has run")
                for key, text in (("adaptation_note", item["adaptation_note"]), ("licence.note", item["licence"].get("note"))):
                    if _unqualified_copy_claim(text):
                        errors.append(f"{ip}: {key} says no text was copied, but the verbatim check of {sid} has not run "
                                      "(say 'not yet verified')")
                notice = files[file_of[item["id"]]]["file_licence"]["notice"]
                if _unqualified_copy_claim(notice):
                    errors.append(f"{file_of[item['id']]}: file_licence.notice says no text was copied, but the verbatim "
                                  f"check of {sid} has not run (say 'not yet verified')")
    return sorted(set(errors), key=errors.index)


# ---------------------------------------------------------------- repository licence scan

def repository_files():
    """Text candidates for the licence scan: git-tracked and untracked files that are not ignored."""
    try:
        out = subprocess.run(["git", "-C", str(REPO), "ls-files", "--cached", "--others", "--exclude-standard", "-z"],
                             capture_output=True, check=True, timeout=60).stdout
        paths = [REPO / p for p in out.decode("utf-8", "surrogateescape").split("\0") if p]
    except (OSError, subprocess.SubprocessError):
        paths = []
        for root, dirs, names in os.walk(REPO):
            dirs[:] = [d for d in dirs if d not in _SKIP_DIRS]
            paths += [Path(root) / n for n in names]
    return sorted({p for p in paths if p.is_file()})


def _decoded(raw: bytes) -> str:
    text = raw.decode("utf-8", errors="replace")
    text = re.sub(r"\\u([0-9a-fA-F]{4})", lambda m: chr(int(m.group(1), 16)), text)
    return text.replace("\\n", " ").replace('\\"', '"')


def cc_card_runs(files, v1_cards):
    """{8-word run: card ids} over the current text of every CC BY-NC-SA card and its v1 text."""
    runs, sources_of = {}, {}
    for _path, doc in files.items():
        if WHO_LICENCE not in doc["file_licence"]["text_licence"]:
            continue
        for item in doc["items"]:
            sources_of[item["id"]] = item["source"]["source_ids"]
            versions = [item]
            if v1_cards and item["id"] in v1_cards:
                versions.append({"content": v1_content(v1_cards[item["id"]])})
            for version in versions:
                for _p, _lang, text in user_texts(version):
                    toks = VO.tokens(text)
                    for i in range(len(toks) - LICENCE_SCAN_RUN + 1):
                        runs.setdefault(tuple(toks[i:i + LICENCE_SCAN_RUN]), set()).add(item["id"])
    return runs, sources_of


def licence_scan(paths=None, files=None, sources_by_id=None, v1_cards=None):
    """(errors, report) for every file that holds CC BY-NC-SA card text without its licence notice.

    A file passes when it contains the licence URI, the IRIS link of every source whose card text it holds,
    'WHO' and the adaptation notice ('not responsible' or 'tidak bertanggung jawab'). Binary files (a NUL byte
    in the first 8 KiB, e.g. compressed dataset packs) and files over 32 MB cannot be scanned and are counted.
    """
    if files is None or sources_by_id is None:
        _schema, _set, sources, files = load_library()
        sources_by_id = {s["id"]: s for s in sources["sources"]}
    if v1_cards is None:
        v1_cards = load_v1_cards()
    runs, sources_of = cc_card_runs(files, v1_cards)
    errors, report = [], {"scanned": 0, "binary_or_large": [], "with_card_text": {}}
    for path in (repository_files() if paths is None else paths):
        path = Path(path)
        try:
            if path.stat().st_size > LICENCE_SCAN_MAX_BYTES:
                report["binary_or_large"].append(str(path))
                continue
            raw = path.read_bytes()
        except OSError:
            continue
        if b"\0" in raw[:8192]:
            report["binary_or_large"].append(str(path))
            continue
        report["scanned"] += 1
        text = _decoded(raw)
        toks = VO.tokens(text)
        cards = set()
        for i in range(len(toks) - LICENCE_SCAN_RUN + 1):
            hit = runs.get(tuple(toks[i:i + LICENCE_SCAN_RUN]))
            if hit:
                cards |= hit
        if not cards:
            continue
        try:
            rel = str(path.resolve().relative_to(REPO))
        except ValueError:
            rel = str(path)
        report["with_card_text"][rel] = sorted(cards)
        missing = []
        if not _contains_url(text, WHO_LICENCE_URL):
            missing.append(f"the licence URI {WHO_LICENCE_URL}")
        links = sorted({source_link(sources_by_id[sid]) for cid in cards for sid in sources_of.get(cid, [])
                        if sid in sources_by_id})
        absent = [link for link in links if not _contains_url(text, link)]
        if absent:
            missing.append("the source link(s) " + ", ".join(absent))
        if "WHO" not in text or not any(marker in text for marker in NOTICE_MARKERS):
            missing.append("the adaptation notice (adapted by the Dengar.in and Sela team; WHO is not responsible)")
        if missing:
            errors.append(f"{rel}: holds CC BY-NC-SA 3.0 IGO card text from {len(cards)} WHO card(s) "
                          f"(e.g. {', '.join(sorted(cards)[:3])}) without {'; '.join(missing)}. Add the attribution and "
                          "the notice (data/content/reading/LICENSE.md) or stop shipping the text")
    return errors, report


def licence_scan_errors():
    return licence_scan()[0]


def help_catalogue_by_id():
    if not HELP_CATALOGUE_PATH.exists():
        return None
    doc = load_json(HELP_CATALOGUE_PATH)
    return {entry["id"]: entry for entry in doc.get("help", [])}


def library_errors():
    errors = []
    schema, set_doc, sources, files = load_library()
    errors += [f"set.json {m}" for m in validate_schema(set_doc, schema["$defs"]["setFile"], schema)]
    errors += [f"sources.json {m}" for m in validate_schema(sources, schema["$defs"]["sourcesFile"], schema)]
    for path, doc in files.items():
        errors += [f"{path} {m}" for m in validate_schema(doc, schema, schema)]
    for path in [SET_PATH, SOURCES_PATH, *[CONTENT / p for p in files]]:
        raw = path.read_text(encoding="utf-8")
        if raw != dump_json(json.loads(raw)) or "\r" in raw:
            errors.append(f"{path.name}: not in canonical form (2-space JSON, LF, trailing newline; run --update)")
    if errors:
        return errors  # semantic checks assume a schema-valid library

    sources_by_id = {}
    for s in sources["sources"]:
        if s["id"] in sources_by_id:
            errors.append(f"sources.json: duplicate id {s['id']}")
        sources_by_id[s["id"]] = s
        if s["reverify_by"] <= s["checked_on"]:
            errors.append(f"sources.json:{s['id']}: reverify_by must be after checked_on")
        for key in ("url", "persistent_url"):
            if key in s and _host(s[key]) not in APPROVED_HOSTS:
                errors.append(f"sources.json:{s['id']}: {key} host {_host(s[key])!r} is not an approved official host")
        if s["url"] == WHO_404_URL or s.get("persistent_url") == WHO_404_URL:
            errors.append(f"sources.json:{s['id']}: uses the 404 URL {WHO_404_URL}")
        if s["licence"].startswith(WHO_LICENCE) and s["kind"] == "who_publication" and s["use"].startswith("adapted"):
            if not s.get("suggested_citation") or not s.get("translation_disclaimer_verbatim"):
                errors.append(f"sources.json:{s['id']}: adapted WHO sources need the suggested citation and the translation disclaimer")
            if s.get("commercial_use") != "blocked" or s.get("blocker") != COMMERCIAL_BLOCKER:
                errors.append(f"sources.json:{s['id']}: adapted WHO sources are blocked for commercial use ({COMMERCIAL_BLOCKER})")
            if s.get("licence_url") != WHO_LICENCE_URL or not s.get("persistent_url"):
                errors.append(f"sources.json:{s['id']}: adapted WHO sources need licence_url {WHO_LICENCE_URL} and a persistent_url")

    if not LICENCE_NOTICE_PATH.exists():
        errors.append(f"{LICENCE_NOTICE_PATH.relative_to(REPO)}: missing (the WHO card text is CC BY-NC-SA 3.0 IGO, not MIT)")
    else:
        errors += licence_notice_errors(LICENCE_NOTICE_PATH.read_text(encoding="utf-8"), sources)
    if set_doc["version"] not in [c["version"] for c in set_doc["changes"]]:
        errors.append(f"set.json: changes has no entry for version {set_doc['version']} (a content set is never changed silently)")

    gate_sets = load_gate_sets()
    errors += gate_examples_errors(gate_sets)
    v1_cards = load_v1_cards()
    help_catalogue = help_catalogue_by_id()

    listed = {f["path"]: f["item_ids"] for f in set_doc["files"]}
    seen = {}
    items = []
    for path, doc in files.items():
        ids = [i["id"] for i in doc["items"]]
        if ids != listed[path]:
            errors.append(f"set.json: item_ids for {path} do not match the file")
        for item in doc["items"]:
            if item["id"] in seen:
                errors.append(f"{path}: duplicate id {item['id']} (also in {seen[item['id']]})")
            seen[item["id"]] = path
            items.append(item)
            errors += item_errors(item, file_path=path, file_doc=doc, sources_by_id=sources_by_id,
                                  gate_sets=gate_sets, v1_cards=v1_cards, help_catalogue=help_catalogue)

    errors += verbatim_check_errors(sources, files)
    if set_doc["counts"] != compute_counts(items):
        errors.append("set.json: counts are stale (run --update)")
    if set_doc["fingerprint"] != fingerprint(items):
        errors.append("set.json: fingerprint is stale (run --update)")
    if v1_cards is not None:
        base = set_doc["v1_baseline"]
        if base["sha256"] != sha256(V1_CARDS_PATH.read_bytes()):
            errors.append("set.json: v1_baseline.sha256 does not match packages/config/src/selfHelpCards.v1.json")
        if base["card_count"] != len(v1_cards):
            errors.append("set.json: v1_baseline.card_count does not match the v1 file")
        migrated = {i["id"] for i in items if i["origin"]["kind"] == "migrated_v1"}
        missing = sorted(set(v1_cards) - migrated)
        if missing:
            errors.append(f"v1 cards not migrated: {missing}")
    return errors


# ---------------------------------------------------------------- update and staleness

def update_files():
    """Recompute every machine-check field in place. Review fields are never changed."""
    schema, set_doc, sources, files = load_library()
    gate_sets = load_gate_sets()
    items = []
    for path, doc in files.items():
        for item in doc["items"]:
            item["text_sha256"] = text_sha256(item)
            item["reading_time_minutes"] = reading_time(item)
            item["checks"]["plain_language"] = plain_language(item)
            item["checks"]["claims"] = claims_check(item, gate_sets, item["checks"]["claims"])
            items.append(item)
        (CONTENT / path).write_text(dump_json(doc), encoding="utf-8")
    set_doc["files"] = [{"path": p, "item_ids": [i["id"] for i in files[p]["items"]]} for p in files]
    set_doc["counts"] = compute_counts(items)
    set_doc["fingerprint"] = fingerprint(items)
    if V1_CARDS_PATH.exists():
        set_doc["v1_baseline"]["sha256"] = sha256(V1_CARDS_PATH.read_bytes())
    SET_PATH.write_text(dump_json(set_doc), encoding="utf-8")
    SOURCES_PATH.write_text(dump_json(sources), encoding="utf-8")


def stale_entries(today):
    _schema, _set, sources, files = load_library()
    out = [f"source {s['id']}: reverify_by {s['reverify_by']}" for s in sources["sources"] if s["reverify_by"] < today]
    for s in sources["sources"]:
        vc = s.get("verbatim_check")
        if vc and vc["status"] == "not_run" and vc.get("due_by", "9999-12-31") < today:
            out.append(f"source {s['id']}: verbatim_check not run, due_by {vc['due_by']}")
    for _path, item in all_items(files):
        for contact in item["contacts"]:
            if contact["reverify_by"] < today:
                out.append(f"item {item['id']}: contact {contact['catalogue_id']} reverify_by {contact['reverify_by']}")
    return out


def main(argv=None):
    ap = argparse.ArgumentParser(description=__doc__.split("\n")[0])
    ap.add_argument("--update", action="store_true", help="recompute machine-check fields and rewrite files")
    ap.add_argument("--check-stale", action="store_true", help="list entries past their re-verify-by date")
    ap.add_argument("--today", default=_dt.date.today().isoformat(), help="date for --check-stale (YYYY-MM-DD)")
    args = ap.parse_args(argv)
    if args.check_stale:
        stale = stale_entries(args.today)
        print("\n".join(stale) if stale else f"no source, contact or verbatim check is past its re-verify-by or due date on {args.today}")
        return 1 if stale else 0
    if args.update:
        update_files()
    errors = library_errors()
    if errors:
        print("\n".join(errors))
        print(f"{len(errors)} error(s)")
        return 1
    _schema, set_doc, _sources, _files = load_library()
    counts = set_doc["counts"]
    print(f"library ok: {counts['items']} items, fingerprint {set_doc['fingerprint'][:16]}, "
          f"{counts['review:machine_drafted_unreviewed']} machine_drafted_unreviewed, {counts['publishable']} publishable")
    scan_errors, report = licence_scan()
    print(f"licence scan: {report['scanned']} text files scanned, {len(report['binary_or_large'])} binary or large "
          f"files not scanned; CC BY-NC-SA card text found in: {', '.join(sorted(report['with_card_text'])) or 'none'}")
    if scan_errors:
        print("\n".join(scan_errors))
        print(f"{len(scan_errors)} licence error(s)")
        return 1
    return 0


if __name__ == "__main__":
    sys.exit(main())
