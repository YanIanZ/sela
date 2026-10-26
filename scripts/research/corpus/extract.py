"""Steps 7-8: deterministic term matching and traceable full-text extraction.

Rules only match explicit words. Nothing is inferred, paraphrased or guessed:
when a rule does not fire the value stays null. Every full-text extraction is
labelled 'machine_extracted_unreviewed' and carries a locator.
"""
from __future__ import annotations

import hashlib
import re
import xml.etree.ElementTree as ET

EXTRACTION_STATUS = "machine_extracted_unreviewed"
SNIPPET_LIMIT = 240

I = re.IGNORECASE

# --- study type -------------------------------------------------------------

PUBTYPE_LABELS = {
    "systematic review": "systematic_review", "systematic-review": "systematic_review",
    "meta-analysis": "meta_analysis", "scoping review": "scoping_review",
    "review": "review", "review-article": "review",
    "randomized controlled trial": "randomized_controlled_trial",
    "clinical trial": "clinical_trial", "controlled clinical trial": "clinical_trial",
    "pragmatic clinical trial": "clinical_trial", "clinical trial protocol": "protocol",
    "observational study": "observational_study", "comparative study": "comparative_study",
    "validation study": "validation_study", "multicenter study": "multicenter_study",
    "evaluation study": "evaluation_study", "case reports": "case_report",
    "editorial": "editorial", "comment": "comment", "letter": "letter", "news": "news",
    "preprint": "preprint", "brief-report": "brief_report", "practice guideline": "guideline",
    "guideline": "guideline", "published erratum": "erratum",
    "research-article": "journal_article", "journal article": "journal_article",
    "conference paper": "conference_paper",
}
CROSSREF_TYPE_LABELS = {
    "journal-article": "journal_article", "proceedings-article": "conference_paper",
    "posted-content": "preprint", "book-chapter": "book_chapter", "report": "report",
    "dissertation": "dissertation", "peer-review": "peer_review",
}
TITLE_CUES = (
    ("systematic_review", r"\bsystematic(?:\s+literature)?\s+review\b"),
    ("meta_analysis", r"\bmeta[- ]analy(?:sis|ses|tic)\b"),
    ("scoping_review", r"\bscoping\s+review\b"),
    ("randomized_trial", r"\brandomi[sz]ed\b"),
    ("qualitative", r"\bqualitative\b|\bthematic\s+analysis\b|\bfocus\s+groups?\b|\binterview\s+study\b"),
    ("cross_sectional", r"\bcross[- ]sectional\b"),
    ("cohort", r"\bcohort\b"),
    ("protocol", r"\bprotocol\b"),
    ("mixed_methods", r"\bmixed[- ]methods?\b"),
    ("pilot", r"\bpilot\b"),
    ("feasibility", r"\bfeasibility\b"),
    ("case_series", r"\bcase\s+series\b"),
    ("longitudinal", r"\blongitudinal\b"),
)
TITLE_CUE_PATTERNS = tuple((label, re.compile(rx, I)) for label, rx in TITLE_CUES)
PRIMARY_PRECEDENCE = (
    "protocol", "meta_analysis", "systematic_review", "scoping_review", "review",
    "randomized_controlled_trial", "randomized_trial", "clinical_trial", "cohort",
    "longitudinal", "cross_sectional", "observational_study", "mixed_methods", "qualitative",
    "case_series", "case_report", "pilot", "feasibility", "validation_study",
    "evaluation_study", "comparative_study", "multicenter_study", "guideline",
    "conference_paper", "brief_report", "preprint", "editorial", "comment", "letter",
    "news", "erratum", "book_chapter", "report", "journal_article",
)
REVIEW_LABELS = {"meta_analysis", "systematic_review", "scoping_review", "review"}
OBSERVATIONAL_LABELS = {"observational_study", "cross_sectional", "cohort", "longitudinal"}


def study_type(epmc_pubtypes, crossref_type, title, register_types=None):
    basis = []
    for pt in epmc_pubtypes or []:
        label = PUBTYPE_LABELS.get(str(pt).strip().lower())
        if label:
            basis.append({"source": "europepmc.pubTypeList", "term": pt, "label": label})
    if crossref_type:
        label = CROSSREF_TYPE_LABELS.get(crossref_type)
        if label:
            basis.append({"source": "crossref.type", "term": crossref_type, "label": label})
    for pt in register_types or []:
        label = PUBTYPE_LABELS.get(str(pt).strip().lower())
        if label:
            basis.append({"source": "register.publication_types_from_index", "term": pt, "label": label})
    for label, pattern in TITLE_CUE_PATTERNS:
        m = pattern.search(title or "")
        if m:
            basis.append({"source": "title", "term": m.group(0), "label": label})
    labels = sorted({b["label"] for b in basis})
    specific = [l for l in labels if l != "journal_article"]
    primary = next((l for l in PRIMARY_PRECEDENCE if l in (specific or labels)), None)
    basis.sort(key=lambda b: (b["source"], b["label"], b["term"]))
    return {"primary": primary, "labels": labels, "basis": basis,
            "rule": "pubtype/crossref-type/title-cue lexicon; primary by fixed precedence",
            "status": "machine_assigned_unreviewed"}


# --- population and country/language terms ------------------------------------

def _p(label, *patterns, case_sensitive=False, fields=("title", "note")):
    flags = 0 if case_sensitive else I
    return [(label, re.compile(p, flags), fields) for p in patterns]


# Order matters: specific phrases first; a later match overlapping an earlier
# span in the same field is skipped (so "college students" is not also "students").
POPULATION_TERMS = (
    _p("international students", r"\binternational students?\b", r"\bmahasiswa internasional\b")
    + _p("college or university students", r"\bcollege students?\b", r"\buniversity students?\b",
         r"\bundergraduate students?\b", r"\bundergraduates?\b", r"\bmahasiswa\b")
    + _p("young adults", r"\byoung adults?\b", r"\bemerging adults?\b", r"\bdewasa muda\b")
    + _p("older adults", r"\bolder adults?\b", r"\belderly\b", r"\blate[- ]life\b", r"\bgeriatric\b",
         r"\blansia\b", r"\blanjut usia\b")
    + _p("young people or youth", r"\byoung people\b", r"\byouths?\b", r"\banak muda\b", r"\borang muda\b")
    + _p("adolescents", r"\badolescen(?:ts?|ce)\b", r"\bteen(?:agers?|s)?\b", r"\bremaja\b")
    + _p("children", r"\bchildren\b", r"\bchild\b", r"\banak(?:-anak)?\b")
    + _p("students", r"\bstudents?\b", r"\bpelajar\b", r"\bsiswa\b")
    + _p("people with mental disorders", r"\bpeople with (?:serious |severe )?mental (?:disorders?|illness(?:es)?)\b",
         r"\bserious mental illness\b", r"\borang dengan gangguan jiwa\b")
    + _p("clinicians or health professionals", r"\bclinicians?\b", r"\bphysicians?\b", r"\bpsychiatrists?\b",
         r"\bpsychologists?\b", r"\btherapists?\b", r"\bnurses?\b",
         r"\bhealth(?:care)? (?:professionals|workers|providers)\b",
         r"\bmental health (?:professionals|providers|practitioners|clinicians)\b",
         r"\bklinisi\b", r"\bperawat\b", r"\bpsikolog\b", r"\bpsikiater\b", r"\btenaga kesehatan\b")
    + _p("patients", r"\bpatients?\b", r"\bpasien\b")
    + _p("parents or caregivers", r"\bparents?\b", r"\bcaregivers?\b", r"\borang tua\b", r"\bpengasuh\b")
    + _p("veterans", r"\bveterans?\b")
    + _p("women or perinatal", r"\bwomen\b", r"\bperinatal\b", r"\bpostpartum\b", r"\bperempuan\b")
    + _p("employees or workers", r"\bemployees?\b", r"\bworkers?\b", r"\bkaryawan\b", r"\bpekerja\b")
    + _p("sexual and gender minorities", r"\blgbtq?\+?(?=\W|$)", r"\bsexual (?:and gender )?minorit(?:y|ies)\b")
    + _p("refugees or migrants", r"\brefugees?\b", r"\bmigrants?\b", r"\bpengungsi\b")
    + _p("adults", r"\badults?\b", r"\borang dewasa\b", r"\bdewasa\b")
)

COUNTRY_TERMS = (
    _p("Indonesia", r"\bindonesia(?:n)?\b", r"\bjakarta\b", r"\bsurabaya\b", r"\byogyakarta\b",
       r"\bbandung\b")
    + _p("ASEAN / Southeast Asia (region)", r"\basean\b", r"\bsoutheast asia(?:n)?\b", r"\basia tenggara\b")
    + _p("United States", r"\bunited states\b", r"\bamerika serikat\b")
    + _p("United States", r"\bU\.S\.(?=\W|$)", r"\bUSA\b", r"\bUS\b", case_sensitive=True)
    + _p("United States", r"\bAS\b", case_sensitive=True, fields=("note",))
    + _p("United Kingdom", r"\bunited kingdom\b", r"\bengland\b", r"\bscotland\b")
    + _p("United Kingdom", r"\bUK\b", case_sensitive=True)
    + _p("English (language)", r"\benglish\b", r"\bbahasa inggris\b")
    + _p("English (language) or United Kingdom (ambiguous)", r"\binggris\b")
    + _p("multilingual (languages unspecified)", r"\bmultilingual\b", r"\bmultibahasa\b",
         r"\bcross[- ]lingual\b", r"\bbilingual\b", r"\bdwibahasa\b")
    + _p("low- and middle-income countries (group)", r"\blow[- ] and middle[- ]income countr(?:y|ies)\b",
         r"\bnegara berpendapatan rendah dan menengah\b")
    + _p("low- and middle-income countries (group)", r"\bLMICs?\b", case_sensitive=True)
    + _p("Europe (region)", r"\beurope(?:an)?\b", r"\beropa\b")
    + _p("Africa (region)", r"\bsub-saharan africa\b", r"\bafrica(?:n)?\b", r"\bafrika\b")
    + _p("India", r"\bindia\b")
    + _p("China", r"\bchina\b", r"\btiongkok\b")
    + _p("Chinese (language/nationality)", r"\bchinese\b")
    + _p("Hong Kong", r"\bhong kong\b")
    + _p("Taiwan", r"\btaiwan(?:ese)?\b")
    + _p("Japan", r"\bjapan(?:ese)?\b", r"\bjepang\b")
    + _p("Korea", r"\b(?:south )?korea\b", r"\bkorea selatan\b")
    + _p("Korean (language/nationality)", r"\bkorean\b")
    + _p("Malaysia", r"\bmalaysia(?:n)?\b")
    + _p("Singapore", r"\bsingapore(?:an)?\b")
    + _p("Thailand", r"\bthailand\b", r"\bthai\b")
    + _p("Philippines", r"\bphilippines\b", r"\bfilipin[oa]s?\b", r"\bfilipina\b")
    + _p("Vietnam", r"\bvietnam(?:ese)?\b", r"\bviet nam\b")
    + _p("Australia", r"\baustralia(?:n)?\b")
    + _p("New Zealand", r"\bnew zealand\b", r"\bselandia baru\b")
    + _p("Canada", r"\bcanada\b", r"\bcanadian\b", r"\bkanada\b")
    + _p("Germany", r"\bgermany\b", r"\bgerman\b", r"\bjerman\b")
    + _p("Netherlands", r"\bnetherlands\b", r"\bdutch\b", r"\bbelanda\b")
    + _p("Sweden", r"\bsweden\b", r"\bswedish\b", r"\bswedia\b")
    + _p("Norway", r"\bnorway\b", r"\bnorwegian\b")
    + _p("Denmark", r"\bdenmark\b", r"\bdanish\b")
    + _p("Finland", r"\bfinland\b", r"\bfinnish\b")
    + _p("France", r"\bfrance\b", r"\bfrench\b", r"\bprancis\b")
    + _p("Spain or Spanish (language)", r"\bspain\b", r"\bspanish\b", r"\bspanyol\b")
    + _p("Italy", r"\bitaly\b", r"\bitalian\b")
    + _p("Ireland", r"\bireland\b", r"\birish\b")
    + _p("Switzerland", r"\bswitzerland\b", r"\bswiss\b")
    + _p("Brazil", r"\bbrazil(?:ian)?\b")
    + _p("Mexico", r"\bmexic(?:o|an)\b")
    + _p("Nigeria", r"\bnigeria(?:n)?\b")
    + _p("Kenya", r"\bkenya(?:n)?\b")
    + _p("South Africa", r"\bsouth africa(?:n)?\b", r"\bafrika selatan\b")
    + _p("Pakistan", r"\bpakistan(?:i)?\b")
    + _p("Bangladesh", r"\bbangladesh(?:i)?\b")
    + _p("Iran", r"\biran(?:ian)?\b")
    + _p("Saudi Arabia", r"\bsaudi arabia\b", r"\bsaudi\b")
    + _p("Turkey", r"\bt(?:u|ü)rkiye\b", r"\bturkey\b", r"\bturkish\b")
    + _p("Israel", r"\bisrael(?:i)?\b")
)
# 'South Africa' must win over 'Africa (region)': put country-specific first.
COUNTRY_TERMS = tuple(sorted(COUNTRY_TERMS, key=lambda t: 0 if t[0] == "South Africa" else 1))


# The register annotation ('note') is Sela's own transfer commentary. It often
# names a population or country only to say the study does NOT carry over
# ("bukan panduan hukum Indonesia", "relevansi target dewasa bersifat tidak
# langsung"). Notes are therefore split into clauses, and a clause carrying a
# negation, transfer/difference, Sela-context or conditional marker is not used
# as a source; its term matches are kept under 'skipped' with the reason.
NOTE_CLAUSE_BOUNDARY = re.compile(
    r"[;.!?:](?=\s|$)|\s[—–-]\s|\b(?:tetapi|tapi|namun|melainkan|sedangkan|padahal|but|however|whereas)\b", I)
NOTE_SKIP_MARKERS = (
    ("negation", re.compile(r"\b(?:bukan(?:lah)?|tidak|tak|belum|tanpa|jangan|not|no|never|nor)\b", I)),
    ("transfer_or_difference", re.compile(
        r"\b(?:berbeda|beda|relevansi|relevan|berlaku|generalisasi|digeneralisasi|dibandingkan|dibanding|"
        r"transfer|perlu)\b", I)),
    ("sela_context", re.compile(r"\b(?:sela|target)\b", I)),
    ("conditional", re.compile(r"\b(?:jika|bila|apabila|seandainya|if)\b", I)),
)
_NON_PREFIX = re.compile(r"\bnon[-\s]?$", I)


def note_clauses(text):
    """[(start, end, skip_reason_or_None, marker_or_None)] covering ``text``."""
    cuts = [0] + [m.end() for m in NOTE_CLAUSE_BOUNDARY.finditer(text)] + [len(text)]
    out = []
    for s, e in zip(cuts, cuts[1:]):
        if e <= s:
            continue
        clause = text[s:e]
        hit = next(((kind, m.group(0)) for kind, rx in NOTE_SKIP_MARKERS for m in [rx.search(clause)] if m), None)
        out.append((s, e, hit[0] if hit else None, hit[1].lower() if hit else None))
    return out


def _compound_reason(text, start, end, compound):
    """Why a term inside a compound is not a population/country label (None = usable)."""
    if compound == "population":
        joined_before = start >= 2 and text[start - 1] == "-" and text[start - 2].isalnum()
        joined_after = end < len(text) and text[end] == "-"
        if joined_before or joined_after:
            return "hyphenated compound (modifier or relationship, e.g. 'dokter-pasien', 'Physician-generated')"
    if compound == "country" and _NON_PREFIX.search(text[max(0, start - 4):start]):
        return "negated by prefix 'non-'"
    return None


def match_terms(texts, lexicon, compound=None):
    """Explicit-term matching. ``texts`` maps field name -> text ('title', 'note').

    Title matches count unless the term sits in a negated or hyphenated
    compound (``compound`` = 'population' or 'country' selects the rule).
    Note matches count only in clauses without a skip marker. Returns None when
    nothing matched at all, else ``{"labels": [...], "matches": [...],
    "skipped": [...], ...}``; ``labels`` may be empty when every match was skipped.
    """
    matches, skipped = [], []
    for field in sorted(texts):
        text = texts[field] or ""
        clauses = note_clauses(text) if field == "note" else []
        taken = []
        for label, pattern, fields in lexicon:
            if field not in fields:
                continue
            for m in pattern.finditer(text):
                span = (m.start(), m.end())
                if any(span[0] < e and s < span[1] for s, e in taken):
                    continue
                taken.append(span)  # a skipped match still claims its span ('college students' is not also 'students')
                entry = {"field": field, "term": m.group(0), "label": label}
                reason = _compound_reason(text, m.start(), m.end(), compound)
                if reason:
                    skipped.append(dict(entry, reason=reason))
                    continue
                clause = next((c for c in clauses if c[0] <= m.start() < c[1]), None)
                if clause and clause[2]:
                    skipped.append(dict(entry, reason=f"note clause has {clause[2]} marker '{clause[3]}'",
                                        clause=text[clause[0]:clause[1]].strip(" ;.,")))
                    continue
                matches.append(entry)
    if not matches and not skipped:
        return None
    unique = sorted({(m["field"], m["term"], m["label"]) for m in matches})
    skipped_unique = {json_key(s): s for s in skipped}
    return {
        "labels": sorted({m[2] for m in unique}),
        "matches": [{"field": f, "term": t, "label": l} for f, t, l in unique],
        "skipped": [skipped_unique[k] for k in sorted(skipped_unique)],
        "rule": ("explicit term lexicon over the title and the affirmative clauses of the register annotation; "
                 "annotation clauses with negation/transfer/Sela-context/conditional markers, hyphenated "
                 "population compounds and 'non-' prefixed countries are skipped (listed under 'skipped'); "
                 "not a verified study population"),
        "status": "machine_extracted_unreviewed",
    }


def json_key(entry):
    return tuple(sorted((k, str(v)) for k, v in entry.items()))


def match_population(title, note):
    return match_terms({"title": title, "note": note}, POPULATION_TERMS, compound="population")


def match_country(title, note):
    return match_terms({"title": title, "note": note}, COUNTRY_TERMS, compound="country")


# --- full-text XML ----------------------------------------------------------

_XLINK = "{http://www.w3.org/1999/xlink}href"
_ALI = "{http://www.niso.org/schemas/ali/1.0/}license_ref"
_SKIP_ABSTRACT_TYPES = {"graphical", "teaser", "toc", "author-highlights", "precis"}
_NESTED_P_CONTAINERS = {"list", "list-item", "disp-quote", "boxed-text", "statement", "def-list", "def"}


_CITATION_XREF_TYPES = {"bibr", "fn", "table-fn"}


def _gather(el, out):
    if el.text:
        out.append(el.text)
    for child in el:
        tag = child.tag if isinstance(child.tag, str) else ""
        citation = (tag == "xref" and (child.get("ref-type") or "bibr") in _CITATION_XREF_TYPES) or (
            tag == "sup" and re.fullmatch(r"[\d,\s\u2013\-]+", "".join(child.itertext()) or ""))
        if not citation:
            _gather(child, out)
        if child.tail:
            out.append(child.tail)


def _text(el):
    """Element text with citation markers (<xref ref-type='bibr'>, numeric <sup>) removed."""
    out = []
    _gather(el, out)
    return " ".join("".join(out).split())


_CC_NAME = re.compile(r"\bcreative\s+commons\b|\bCC[- ]?BY\b|\bCC0\b", I)


def fulltext_licence(root):
    """Licence statement in the article XML (JATS <permissions><license>).

    Explicit CC = a creativecommons.org URL (license/@xlink:href, ali:license_ref
    or any descendant ext-link/@xlink:href) or the licence text naming
    "Creative Commons" / "CC BY" / "CC0".
    """
    licences = root.findall(".//permissions/license") or root.findall(".//license")
    if not licences:
        return {"present": False, "href": None, "license_type": None, "is_cc": False, "basis": None}
    hrefs = []
    texts = []
    for lic in licences:
        for el in lic.iter():
            href = el.get(_XLINK)
            if href:
                hrefs.append(href.strip())
        for ali in lic.iter(_ALI):
            if ali.text:
                hrefs.append(ali.text.strip())
        texts.append(_text(lic))
    for ali in root.iter(_ALI):
        if ali.text and ali.text.strip() not in hrefs:
            hrefs.append(ali.text.strip())
    text = " ".join(texts)
    if not any("creativecommons.org/" in h.lower() for h in hrefs):
        m = re.search(r"https?://creativecommons\.org/[^\s)\"']+", text)
        if m:
            hrefs.append(m.group(0))
    cc_href = next((h for h in hrefs if "creativecommons.org/" in h.lower()), None)
    named = bool(_CC_NAME.search(text))
    basis = "creativecommons.org URL" if cc_href else "licence text names Creative Commons" if named else None
    return {"present": True, "href": cc_href or (hrefs[0] if hrefs else None),
            "license_type": licences[0].get("license-type"), "is_cc": bool(cc_href or named), "basis": basis}


def parse_fulltext(xml_bytes):
    """Return {'licence', 'paragraphs': [{'part','section','paragraph','text'}]}.

    Only <abstract> and <body> are walked; front matter (affiliations),
    references, tables and figure captions are skipped. ElementTree does not
    resolve external entities; the input is treated as untrusted data.
    """
    root = ET.fromstring(xml_bytes)
    paragraphs = []

    def add(part, path, counter, el):
        key = tuple(path)
        counter[key] = counter.get(key, 0) + 1
        text = _text(el)
        if text:
            paragraphs.append({"part": part, "section": " > ".join(path) or "(no section title)",
                               "paragraph": counter[key], "text": text})

    def walk(el, path, part, counter):
        for child in el:
            tag = child.tag if isinstance(child.tag, str) else ""
            if tag == "sec":
                title_el = child.find("title")
                title = _text(title_el) if title_el is not None else ""
                walk(child, path + [title or "(untitled section)"], part, counter)
            elif tag == "p":
                add(part, path, counter, child)
            elif tag in _NESTED_P_CONTAINERS:
                for p in child.iter("p"):
                    add(part, path, counter, p)

    front = root.find("front")
    article_meta = front.find("article-meta") if front is not None else None
    abstracts = article_meta.findall("abstract") if article_meta is not None else root.findall(".//abstract")
    untyped = 0
    for abstract in abstracts:
        kind = (abstract.get("abstract-type") or "").strip().lower()
        if kind in _SKIP_ABSTRACT_TYPES:
            continue
        if kind:
            prefix = [f"[{kind}]"]
        else:
            untyped += 1
            prefix = [] if untyped == 1 else [f"[abstract {untyped}]"]
        walk(abstract, prefix, "abstract", {})
    body = root.find("body")
    if body is not None:
        walk(body, [], "body", {})
    return {"licence": fulltext_licence(root), "paragraphs": paragraphs}


_METHODS_SECTION = re.compile(
    r"\b(?:methods?|methodology|materials|participants?|subjects?|sample|sampling|recruitment|"
    r"design|procedures?|setting|population|data collection|eligibility|study selection|"
    r"search strategy|inclusion|measures)\b", I)


_NOT_SAMPLE_SECTION = re.compile(
    r"\b(?:measures?|instruments?|materials$|questionnaires?|scales?|statistical|analys[ie]s|analytic|"
    r"positionality|reflexivity|coders?|coding|raters?|rating|scoring|annotat\w*|ethics?|ethical|"
    r"consent|reliability)\b|^(?:study\s+)?settings?$", I)


def _in_scope(paragraph):
    """Abstract (all of it) plus body sections whose title path is methods-like."""
    return paragraph["part"] == "abstract" or bool(_METHODS_SECTION.search(paragraph["section"]))


def _in_sample_scope(paragraph):
    """Sample-size scope: the abstract, plus body sections whose TOP-LEVEL title
    is methods-like, excluding measures/instrument, analysis, setting,
    positionality, coder/rater and ethics subsections (these describe other
    samples, catchment populations, missing data or the research team)."""
    if paragraph["part"] == "abstract":
        return True
    if not _top_level_methods(paragraph):
        return False
    path = [t.strip() for t in paragraph["section"].split(" > ")]
    if _NOT_SAMPLE_SECTION.search(path[0]) and not _CORE_METHODS.search(path[0]):
        return False  # e.g. a top-level "Statistical analysis" or "Setting" heading
    return not any(_NOT_SAMPLE_SECTION.search(t) for t in path[1:])


_CORE_METHODS = re.compile(r"\b(?:methods?|methodology|participants?|sample|subjects?|recruitment|population)\b", I)


def _top_level_methods(paragraph):
    return paragraph["part"] == "body" and bool(_METHODS_SECTION.search(paragraph["section"].split(" > ")[0]))


def snippet_around(text, start, end, limit=SNIPPET_LIMIT):
    """Verbatim (whitespace-normalised) window of at most ``limit`` characters."""
    if len(text) <= limit:
        return text
    pad = max(0, (limit - (end - start)) // 2)
    s = max(0, start - pad)
    e = min(len(text), s + limit)
    s = max(0, e - limit)
    return text[s:e]


def _locator(paragraph):
    return {"part": paragraph["part"], "section": paragraph["section"], "paragraph": paragraph["paragraph"]}


def evidence_item(field, value, paragraph, start, end, source_url, pattern):
    snip = snippet_around(paragraph["text"], start, end)
    return {
        "field": field, "value": value, "pattern": pattern,
        "snippet": snip, "snippet_sha256": hashlib.sha256(snip.encode("utf-8")).hexdigest(),
        "locator": _locator(paragraph), "source_url": source_url, "status": EXTRACTION_STATUS,
    }


_NUM = r"(\d{1,3}(?:,\d{3})+|\d+)"
_PEOPLE = (r"(?:participants|respondents|students|undergraduates|adults|adolescents|youths?|"
           r"young\s+(?:people|adults)|children|parents|caregivers|patients|individuals|people|"
           r"persons|users|clinicians|physicians|psychiatrists|psychologists|therapists|"
           r"counsell?ors|nurses|veterans|women|men|employees|workers|volunteers|subjects|"
           r"interviewees|informants|professionals)")
# Intervening words (at most two) may be adjectives such as "eligible" or "U.S.";
# function words, time units and non-person count nouns break the phrase.
_STOP_WORDS = (
    "years?|months?|weeks?|days?|hours?|minutes?|seconds?|times?|or|and|to|aged?|of|per|in|"
    "from|with|for|at|on|by|as|into|about|among|between|than|who|that|which|were|was|are|is|be|been|had|"
    "have|has|the|a|an|each|all|both|these|those|their|our|his|her|its|not|no|outcomes?|items?|studies|"
    "study|sessions?|interviews?|questions?|workshops?|groups?|articles?|papers?|trials?|conversations?|"
    "transcripts?|chatbots?|responses?|messages?|prompts?|apps?|scenarios?|vignettes?|countries|sites?|"
    "centers?|centres?|schools?|hospitals?|clinics?|services?|records?|posts?|reviews?|percent")
_WORD = r"(?!(?:" + _STOP_WORDS + r")\b)(?:(?:[A-Za-z]\.){2,}|[A-Za-z][A-Za-z\-]*)"
_GAP = r"\s+(?:" + _WORD + r"\s+){0,2}?"
SAMPLE_PATTERNS = (
    ("n_equals", re.compile(r"(?<![A-Za-z0-9])[nN]\s*=\s*" + _NUM + r"(?![\d]|[.,]\d)")),
    ("total_of", re.compile(r"\btotal\s+of\s+" + _NUM + _GAP + _PEOPLE + r"\b", I)),
    ("count_noun", re.compile(r"(?<![\d.,\-–/])\b" + _NUM + _GAP + _PEOPLE + r"\b", I)),
)
_APPROX_BEFORE = re.compile(
    r"(?:more\s+than|over|approximately|approx\.|about|nearly|almost|around|up\s+to|at\s+least|"
    r"fewer\s+than|less\s+than|roughly|some|estimated|[<>≥≤~])\s*$", I)
_SUBGROUP_AFTER = re.compile(r"^\s*[,;]\s*\d+(?:\.\d+)?\s*%")
_SUBGROUP_BEFORE = re.compile(r"\d+(?:\.\d+)?\s*%\s*[,;]?\s*\(?\s*$")


_NON_PERSON_UNIT_BEFORE = re.compile(
    r"\b(?:apps?|applications?|studies|articles|trials|items|chatbots?|conversations?|responses?|queries|"
    r"transcripts?|messages?|posts?|sessions?|minutes?|prompts?|vignettes?|scenarios?|records?|documents?|"
    r"notes?|reports?|websites?|videos?|models?)\b[^.;]{0,25}$", I)
_PRE_ENROLMENT = re.compile(r"\b(?:eligible|potential(?:ly)?|invited|approached|screened|contacted|prospective)\b", I)
_STAFF_AGENT_BEFORE = re.compile(
    r"\b(?:conducted|performed|rated|coded|reviewed|annotated|scored|evaluated|assessed|compiled|translated|"
    r"screened|extracted|developed|collected|moderated|facilitated|led|validated)\s+by\b", I)
_STAFF_VERB_AFTER = re.compile(
    r"^\s*(?:\([^)]{0,40}\)\s*)?(?:(?:independently|separately|jointly|also)\s+)?(?:compiled|annotat\w*|rated|"
    r"coded|reviewed|scored|evaluated|screened|extracted|translated|developed|moderated|facilitated|"
    r"transcribed|analy[sz]ed)\b", I)


_STAFF_GROUP_BEFORE = re.compile(r"\b(?:panel|team|committee)\s+of\s*$", I)
_INITIALS_AFTER = re.compile(r"^\s*\(\s*(?:[A-Z]\.?){2,3}(?:\s*,\s*|\s+and\s+)")


def _exclusion(text, m, pattern_name):
    """Deterministic reasons a match is not the study sample (None = it votes)."""
    if pattern_name == "n_equals":
        if _is_subgroup(text, m):
            return "subgroup count (adjacent percentage)"
        if _NON_PERSON_UNIT_BEFORE.search(text[max(0, m.start() - 40):m.start()]):
            return "n refers to a non-person unit (apps, studies, sessions, ...)"
    if _APPROX_BEFORE.search(text[max(0, m.start(1) - 20):m.start(1)]):
        return "approximate or bounded quantity"
    if pattern_name != "n_equals" and _PRE_ENROLMENT.search(m.group(0)):
        return "pre-enrolment count (eligible/invited/screened)"
    sentence_start = max(text.rfind(". ", 0, m.start()), text.rfind("; ", 0, m.start()))
    after = text[m.end():m.end() + 60]
    if (_STAFF_AGENT_BEFORE.search(text[sentence_start + 1:m.start()]) or _STAFF_VERB_AFTER.search(after)
            or _STAFF_GROUP_BEFORE.search(text[max(0, m.start() - 20):m.start()]) or _INITIALS_AFTER.match(after)):
        return "research team member count, not participants"
    return None


def _is_subgroup(text, m):
    """'(n = 11, 3%)' or '45% (n = 20)' describes a subgroup, not the sample."""
    return bool(_SUBGROUP_AFTER.match(text[m.end():m.end() + 16]) or
                _SUBGROUP_BEFORE.search(text[max(0, m.start() - 16):m.start()]))


def extract_sample_size(paragraphs, source_url, is_review=False):
    """Collect every explicit sample-size candidate; keep a value only if all agree.

    Returns (detail, candidates). Candidates that are subgroup counts
    ('n = 11, 3%') are kept with ``excluded_reason`` but do not vote.
    """
    candidates = []
    seen = set()
    for para in paragraphs:
        if not _in_sample_scope(para):
            continue
        for name, pattern in SAMPLE_PATTERNS:
            for m in pattern.finditer(para["text"]):
                value = int(m.group(1).replace(",", ""))
                if value <= 0:
                    continue
                key = (value, para["part"], para["section"], para["paragraph"], m.start(1))
                if key in seen:
                    continue  # 'total of N participants' is also a count_noun match
                seen.add(key)
                item = evidence_item("sample_size", value, para, m.start(), m.end(), source_url, name)
                reason = _exclusion(para["text"], m, name)
                if reason:
                    item["excluded_reason"] = reason
                candidates.append(item)
    values = sorted({c["value"] for c in candidates if "excluded_reason" not in c})
    if is_review:
        note = "review-type record: counts are pooled or included-study figures, not one study's sample size"
        value = None
    elif not values:
        note = "no explicit sample-size pattern found in abstract or methods sections"
        value = None
    elif len(values) > 1:
        note = "conflicting candidates " + ", ".join(str(v) for v in values) + "; left null (no guessing)"
        value = None
    else:
        note = "single consistent explicit value"
        value = values[0]
    return {"value": value, "candidate_values": values, "note": note,
            "rule": ("n = N | total of N <people> | N <people> in the abstract and top-level methods "
                     "sections (not measures/analysis/setting/coder/rater/ethics subsections); excluded "
                     "(kept, non-voting): subgroup 'n = N, x%', approximate ('more than N'), pre-enrolment "
                     "('N eligible'), research-team counts ('rated by N', 'N physicians annotated') and n of "
                     "non-person units; value only if all remaining candidates agree"),
            "status": EXTRACTION_STATUS}, candidates


DESIGN_PHRASES = (
    ("randomized controlled trial", r"\brandomi[sz]ed[- ]controlled[- ]trials?\b"),
    ("randomized", r"\brandomi[sz](?:ed|ation)\b"),
    ("pilot", r"\bpilot\b"),
    ("feasibility", r"\bfeasibility\b"),
    ("cross-sectional", r"\bcross[- ]sectional\b"),
    ("cohort", r"\bcohort\b"),
    ("longitudinal", r"\blongitudinal\b"),
    ("prospective", r"\bprospective\b"),
    ("retrospective", r"\bretrospective\b"),
    ("qualitative", r"\bqualitative\b"),
    ("semi-structured interviews", r"\bsemi[- ]structured\s+interviews?\b"),
    ("focus groups", r"\bfocus\s+groups?\b"),
    ("thematic analysis", r"\bthematic\s+analysis\b"),
    ("mixed methods", r"\bmixed[- ]methods?\b"),
    ("systematic review", r"\bsystematic\s+review\b"),
    ("meta-analysis", r"\bmeta[- ]analys[ie]s\b"),
    ("scoping review", r"\bscoping\s+review\b"),
    ("survey", r"\bsurveys?\b"),
    ("case series", r"\bcase\s+series\b"),
    ("diary study", r"\bdiary\s+study\b"),
    ("co-design", r"\bco[- ]?design\b"),
    ("delphi", r"\bdelphi\b"),
    ("observational", r"\bobservational\b"),
    ("quasi-experimental", r"\bquasi[- ]experimental\b"),
    ("waitlist control", r"\bwait[- ]?list(?:ed)?\b"),
    ("single-arm", r"\bsingle[- ]arm\b"),
)
DESIGN_PATTERNS = tuple((label, re.compile(rx, I)) for label, rx in DESIGN_PHRASES)


def _first_occurrences(paragraphs, lexicon, field, source_url, pattern_name):
    """One evidence item per label at its first in-scope occurrence (+ count)."""
    found = {}
    for para in paragraphs:
        if not _in_scope(para):
            continue
        for entry in lexicon:
            label, pattern = entry[0], entry[1]
            for m in pattern.finditer(para["text"]):
                if label in found:
                    found[label]["occurrences"] += 1
                    continue
                item = evidence_item(field, label, para, m.start(), m.end(), source_url, pattern_name)
                item["matched_text"] = m.group(0)
                item["occurrences"] = 1
                found[label] = item
    return [found[k] for k in sorted(found)]


def extract_design_phrases(paragraphs, source_url):
    return _first_occurrences(paragraphs, DESIGN_PATTERNS, "study_design_phrase", source_url, "design_lexicon")


def extract_countries(paragraphs, source_url):
    lexicon = [(label, pattern) for label, pattern, fields in COUNTRY_TERMS if "title" in fields]
    return _first_occurrences(paragraphs, lexicon, "country", source_url, "country_lexicon")


LABELLED_SECTIONS = {
    "intervention_or_exposure": re.compile(r"^(?:interventions?|exposures?|intervention and comparators?)$", I),
    "comparator": re.compile(r"^(?:comparators?|comparisons?|control (?:group|condition)s?)$", I),
    "outcomes": re.compile(r"^(?:(?:main|primary|secondary)\s+)?outcomes?(?:\s+(?:and\s+)?measures?)?$|"
                           r"^outcome measures?$", I),
    "limitations": re.compile(r"^(?:study\s+)?limitations?(?:\s+of\s+(?:the|this)\s+study)?$|"
                              r"^strengths and limitations$|^limitations and future (?:research|directions)$", I),
}


def extract_labelled_sections(paragraphs, source_url):
    """First paragraph of an explicitly titled section, verbatim, with locator.

    Intervention/exposure, comparator and outcomes are taken only from the
    abstract or from sections under a top-level Methods heading. Limitations are taken from a section
    explicitly titled "Limitations" (or "Strengths and limitations") wherever it
    appears, because journals usually place it inside the Discussion.
    """
    out = {}
    for field, pattern in LABELLED_SECTIONS.items():
        for para in paragraphs:
            if field != "limitations" and not (para["part"] == "abstract" or _top_level_methods(para)):
                continue
            last = para["section"].split(" > ")[-1].strip().rstrip(":.")
            if pattern.match(last):
                item = evidence_item(field, last, para, 0, 0, source_url, "labelled_section_title")
                item["section_title"] = last
                out[field] = item
                break
    return out
