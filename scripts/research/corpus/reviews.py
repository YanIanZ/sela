"""Human reviewer decisions for the 100-record register (input of the v3 build).

The file ``data/research/register_inputs/reviewer_decisions.json`` is written
by people only. The pipeline copies a validated entry into the record's
reviewer fields and never derives one. No entry exists yet, so every reviewer
field in the committed outputs is ``null`` and every ``reviewer_status`` is
``not_reviewed`` (blocker B-4).

Entry schema (``sela-register-review-v2``), one entry per R-id::

    {"id": "R032", "reviewer_name": "Full Name", "human_attestation": true,
     "review_date": "2026-11-02",
     "review_level": "title_abstract" | "full_text",
     "decision": "include" | "exclude" | "uncertain",
     "appraisal_tool": null | "RoB 2" | "ROBINS-I" | "AMSTAR 2" | "CASP" | ...,   # required for full_text
     "reviewer_2": null | {"reviewer_name": "...", "human_attestation": true,
                           "review_date": "YYYY-MM-DD", "decision": "include" | "exclude" | "uncertain"},
     "adjudication": null | {"adjudicated_by": "...", "human_attestation": true,
                             "adjudication_date": "YYYY-MM-DD", "decision": "...", "notes": null | "..."},
     "study_type_override": null | "<label>",
     "publication_date_override": null | {"date": "YYYY-MM-DD", "source_url": "https://..."},
     "country_promoted": null | ["<label>", ...],
     "findings": null | "<reviewer's own words; full_text level only>",
     "notes": null | "<text>"}

Dual review (research audit R-5): ``reviewer_2`` is a second, independent
person. When the two decisions differ, an ``adjudication`` by a third person is
required and its decision becomes the final ``review_decision``; when they
agree, no adjudication is recorded. A single review stays allowed and is
marked as such (``review_disagreement`` stays ``null``).

A reviewer is a person. Every person block needs ``human_attestation: true``
(the named person read the source and wrote the entry; no AI tool wrote it).
Names of AI tools, models or automation, and template placeholders, are
refused; ordinary given names are not (for example the Sundanese given name
"Ai" or the French given name "Claude" in a full name).
"""
from __future__ import annotations

import datetime as dt
import json
import re

SCHEMA = "sela-register-review-v2"
REVIEW_LEVELS = ("title_abstract", "full_text")
DECISIONS = ("include", "exclude", "uncertain")
OPTIONAL = ("appraisal_tool", "reviewer_2", "adjudication", "study_type_override", "publication_date_override",
            "country_promoted", "findings", "notes")
REQUIRED = ("id", "reviewer_name", "human_attestation", "review_date", "review_level", "decision")
REVIEWER_2_KEYS = ("reviewer_name", "human_attestation", "review_date", "decision")
ADJUDICATION_KEYS = ("adjudicated_by", "human_attestation", "adjudication_date", "decision", "notes")

# Explicit names of AI tools and models (a product name, optionally with a model variant), generic
# AI/automation words, and template placeholders. A bare given name such as "Ai" or "Claude" inside a
# full personal name is NOT refused; "Claude" or "Gemini" as the whole name, or with a model variant
# ("Claude Opus 4", "gemini-1.5-pro"), is. The same rule covers names that are also given names
# ("Gemma", "Kimi", "Sela"): refused alone or with a model version ("Gemma 2", "Kimi K2"), accepted in a
# full personal name ("Gemma Collins", "Kimi Lindqvist").
TOOL_OR_MODEL_NAME = re.compile(
    r"\bclaude(?:[\s_-]+(?:opus|sonnet|haiku|instant|code|ai|\d[\w.]*))+\b|^\s*claude\s*$|"
    r"\bgemini(?:[\s_-]+(?:pro|flash|ultra|nano|advanced|\d[\w.]*))+\b|^\s*gemini\s*$|"
    r"\b(?:chat\s*gpt|gpt(?:[\s_-]?\d[\w.-]*|[\s_-]?o\d\w*)?|openai|anthropic|deepseek\w*|copilot|perplexity|"
    r"mistral|mixtral|qwen\w*|grok(?:[\s_-]?\d[\w.]*)?|google\s+bard|meta\s+ai|llama[\s_-]?\d[\w.]*)\b|"
    # this project's own generators and verifier (D-030: Muse Spark is the primary generator) and other
    # model families: Meta Llama, bare model tiers with a version, Gemma, Phi, Kimi, Codex, Cohere Command R
    r"\bmuse(?:[\s_-]+(?:glimmer|spark|open|\d[\w.]*))+\b|^\s*muse\s*$|"
    r"\bmeta[\s_-]+llama\b|^\s*llama\s*$|"
    r"\b(?:sonnet|opus|haiku)[\s_-]*\d[\w.]*|"
    r"\bgemma(?:[\s_-]*\d[\w.]*)+|^\s*gemma\s*$|"
    r"\bphi[\s_-]?\d[\w.]*|"
    r"\bkimi(?:[\s_-]+k?\d[\w.]*)+|^\s*kimi\s*$|"
    r"\b(?:codex|cohere)\b|\bcommand[\s_-]?r\b|"
    r"\bsela[\s_-]+(?:verifier|ai|assistant|bot|agent|model|generator)\b|^\s*sela\s*$", re.I)
AUTOMATION_WORDS = re.compile(
    r"\b(?:llms?|chatbots?|large\s+language\s+models?|language\s+models?|"
    r"ai\s+(?:agents?|assistants?|models?|reviewers?|systems?|tools?|bots?)|artificial\s+intelligence|"
    r"(?:research\s+)?agent\s*\d+|automated|automatic|machine|pipeline|script|robot|algorithm)\b", re.I)
PLACEHOLDER_NAME = re.compile(
    r"^\s*(?:full\s+name|your\s+name|name|unknown|anonymous|n/?a|none|null|tbd|todo|test(?:\s+user)?|"
    r"reviewer(?:\s*[-_#]?\s*\d+)?|nobody|x+|"
    # "Reviewer A", "Reviewer two", "Anonymous Reviewer", "Human Reviewer", the template's "Second Person",
    # "Jane Doe" / "Dr. John Q. Doe", "Reviewer Name"
    r"reviewer\s*[-_#]?\s*(?:[a-z]|one|two|three)|reviewer\s+name|adjudicator(?:\s+name)?|"
    r"(?:anonymous|human|external|independent|first|second|third|lead|primary|secondary)\s+"
    r"(?:reviewer|screener|adjudicator|person)|"
    r"(?:(?:dr|prof|mr|mrs|ms)\.?\s+)?(?:jane|john)\s+(?:[a-z]\.?\s+)?doe)\s*$", re.I)
_RID = re.compile(r"^R\d{3}$")
_URL = re.compile(r"^https://\S+$")
MAX_TEXT = 2000


class ReviewError(ValueError):
    pass


def _date(value, where):
    try:
        return dt.date.fromisoformat(str(value)).isoformat()
    except ValueError:
        raise ReviewError(f"{where}: {value!r} is not an ISO date (YYYY-MM-DD)") from None


def human_name(value, where, field="reviewer_name"):
    """Normalised personal name, or ReviewError for a tool/model name, automation word or placeholder."""
    if not isinstance(value, str) or not value.strip():
        raise ReviewError(f"{where}: {field} is required")
    name = " ".join(value.split())
    if len(name) < 3 or TOOL_OR_MODEL_NAME.search(name) or AUTOMATION_WORDS.search(name) \
            or PLACEHOLDER_NAME.match(name):
        raise ReviewError(f"{where}: {field} must name the human reviewer (no tool, model or placeholder)")
    return name


def _attested(block, where):
    if block.get("human_attestation") is not True:
        raise ReviewError(f"{where}: human_attestation must be true (the named person read the source and wrote "
                          "this entry; no AI tool wrote it)")


def _decision(value, where):
    if value not in DECISIONS:
        raise ReviewError(f"{where}: decision must be one of {DECISIONS}")
    return value


def _person_block(block, keys, name_key, date_key, where):
    if not isinstance(block, dict):
        raise ReviewError(f"{where}: must be an object or null")
    unknown = set(block) - set(keys)
    if unknown:
        raise ReviewError(f"{where}: unknown keys {sorted(unknown)}")
    _attested(block, where)
    out = {"name": human_name(block.get(name_key), where, name_key),
           "date": _date(block.get(date_key), where),
           "decision": _decision(block.get("decision"), where)}
    if "notes" in keys:
        notes = block.get("notes")
        if notes is not None and (not isinstance(notes, str) or len(notes) > MAX_TEXT):
            raise ReviewError(f"{where}: notes must be text of at most {MAX_TEXT} characters or null")
        out["notes"] = notes
    return out


def validate_entry(entry, known_ids):
    where = f"review {entry.get('id', '?')}"
    unknown = set(entry) - set(REQUIRED) - set(OPTIONAL)
    if unknown:
        raise ReviewError(f"{where}: unknown keys {sorted(unknown)}")
    for key in ("id", "review_date", "review_level", "decision"):
        if not isinstance(entry.get(key), str) or not entry[key].strip():
            raise ReviewError(f"{where}: {key} is required")
    if not _RID.match(entry["id"]) or entry["id"] not in known_ids:
        raise ReviewError(f"{where}: id is not a register R-id")
    out = {k: entry.get(k) for k in REQUIRED + OPTIONAL}
    out["reviewer_name"] = human_name(entry.get("reviewer_name"), where)
    _attested(entry, where)
    out["review_date"] = _date(entry["review_date"], where)
    if out["review_level"] not in REVIEW_LEVELS:
        raise ReviewError(f"{where}: review_level must be one of {REVIEW_LEVELS}")
    _decision(out["decision"], where)
    for key in ("appraisal_tool", "study_type_override", "notes", "findings"):
        v = out[key]
        if v is not None and (not isinstance(v, str) or not v.strip() or len(v) > MAX_TEXT):
            raise ReviewError(f"{where}: {key} must be non-empty text of at most {MAX_TEXT} characters or null")
    if out["review_level"] == "full_text" and out["appraisal_tool"] is None:
        raise ReviewError(f"{where}: a full_text review must name its appraisal_tool (RoB 2, ROBINS-I, AMSTAR 2, "
                          "CASP, ...)")
    if out["findings"] is not None and out["review_level"] != "full_text":
        raise ReviewError(f"{where}: findings may only be recorded with a full-text review")
    pdo = out["publication_date_override"]
    if pdo is not None:
        if not isinstance(pdo, dict) or set(pdo) != {"date", "source_url"} or not _URL.match(str(pdo["source_url"])):
            raise ReviewError(f"{where}: publication_date_override needs {{date, source_url (https)}}")
        out["publication_date_override"] = {"date": _date(pdo["date"], where), "source_url": pdo["source_url"]}
    cp = out["country_promoted"]
    if cp is not None and (not isinstance(cp, list) or not cp or not all(isinstance(c, str) and c for c in cp)):
        raise ReviewError(f"{where}: country_promoted must be a non-empty list of labels or null")
    # dual review and adjudication
    r2 = None
    if out["reviewer_2"] is not None:
        r2 = _person_block(out["reviewer_2"], REVIEWER_2_KEYS, "reviewer_name", "review_date", f"{where} reviewer_2")
        if r2["name"].casefold() == out["reviewer_name"].casefold():
            raise ReviewError(f"{where}: reviewer_2 must be a different person")
    adj = None
    if out["adjudication"] is not None:
        adj = _person_block(out["adjudication"], ADJUDICATION_KEYS, "adjudicated_by", "adjudication_date",
                            f"{where} adjudication")
        names = {out["reviewer_name"].casefold()} | ({r2["name"].casefold()} if r2 else set())
        if adj["name"].casefold() in names:
            raise ReviewError(f"{where}: the adjudicator must be a third person, not one of the two reviewers")
    disagreement = None if r2 is None else r2["decision"] != out["decision"]
    if disagreement and adj is None:
        raise ReviewError(f"{where}: the two reviewers disagree ({out['decision']} vs {r2['decision']}); an "
                          "adjudication by a third person is required")
    if adj is not None and not disagreement:
        raise ReviewError(f"{where}: adjudication is recorded only when two reviewers disagree")
    out["reviewer_2"] = r2
    out["adjudication"] = adj
    out["review_disagreement"] = disagreement
    out["final_decision"] = adj["decision"] if adj else out["decision"]
    return out


def load_reviews(raw_bytes, known_ids):
    """Parse and validate the reviewer file; returns {R-id: entry}."""
    doc = json.loads(raw_bytes.decode("utf-8"))
    if doc.get("schema") != SCHEMA:
        raise ReviewError(f"reviewer file schema must be {SCHEMA!r}")
    reviews = doc.get("reviews")
    if not isinstance(reviews, list):
        raise ReviewError("reviewer file: 'reviews' must be a list")
    out = {}
    for entry in reviews:
        if not isinstance(entry, dict):
            raise ReviewError("reviewer file: every review must be an object")
        e = validate_entry(entry, set(known_ids))
        if e["id"] in out:
            raise ReviewError(f"review {e['id']}: more than one entry (record a second reviewer in reviewer_2)")
        out[e["id"]] = e
    return out
