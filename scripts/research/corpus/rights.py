"""Step 6: licence, access status, processing/redistribution rights, retractions.

Conservative by design: anything that is not an explicit Creative Commons
licence is treated as "not permitted" for text processing and redistribution.
Licence metadata is a signal recorded by the source, not legal advice.
"""
from __future__ import annotations

import re

from .dates import from_date_parts

_CC_URL = re.compile(
    r"creativecommons\.org/(?:licenses|licences)/([a-z\-]+)(?:/(\d(?:\.\d)?))?", re.IGNORECASE)
_CC_ZERO = re.compile(r"creativecommons\.org/publicdomain/zero/(\d(?:\.\d)?)", re.IGNORECASE)
_EPMC_CC = re.compile(r"^cc[\s-]*(0|zero|by(?:[\s-](?:nc|nd|sa))*)$", re.IGNORECASE)
_CC_ELEMENTS = ("NC", "ND", "SA")
REDISTRIBUTABLE_FAMILIES = {"CC0", "CC-BY", "CC-BY-SA"}
RESTRICTION_NOTES = {
    "NC": "NC: non-commercial use only",
    "ND": "ND: no derivatives; do not redistribute adapted text or excerpts",
    "SA": "SA: share-alike applies to adaptations",
}


def _cc_family(code):
    elements = [e.upper() for e in re.split(r"[\s-]+", code) if e]
    if elements[0] == "BY":
        rest = [e for e in _CC_ELEMENTS if e in elements[1:]]
        return "-".join(["CC", "BY"] + rest), rest
    return None, []


def classify_licence_url(url, content_version=None):
    """Classify a licence URL (Crossref license[].URL)."""
    if not url:
        return None
    u = str(url).strip()
    m = _CC_ZERO.search(u)
    if m:
        return {"id": f"CC0-{m.group(1)}", "family": "CC0", "is_cc": True,
                "restrictions": [], "url": u}
    m = _CC_URL.search(u)
    if m:
        family, rest = _cc_family(m.group(1))
        if family:
            version = f"-{m.group(2)}" if m.group(2) else ""
            return {"id": family + version, "family": family, "is_cc": True,
                    "restrictions": rest, "url": u}
    tdm = content_version == "tdm" or "/tdm" in u.lower() or "text-and-data-mining" in u.lower()
    return {"id": "publisher-tdm" if tdm else "publisher-specific",
            "family": "publisher-tdm" if tdm else "publisher-specific",
            "is_cc": False, "restrictions": [], "url": u}


def classify_epmc_licence(value):
    """Classify Europe PMC's short licence string ('cc by', 'cc by-nc-nd', 'cc0')."""
    if not value:
        return None
    v = " ".join(str(value).split()).lower()
    m = _EPMC_CC.match(v)
    if m:
        code = m.group(1)
        if code in ("0", "zero"):
            return {"id": "CC0", "family": "CC0", "is_cc": True, "restrictions": [], "url": None,
                    "raw": value}
        family, rest = _cc_family(code)
        return {"id": family, "family": family, "is_cc": True, "restrictions": rest,
                "url": None, "raw": value}
    return {"id": "unrecognised", "family": "unrecognised", "is_cc": False,
            "restrictions": [], "url": None, "raw": value}


def crossref_licence_candidates(licences, freeze_date):
    """Annotate every Crossref licence; mark which are applicable at freeze_date.

    Applicable = content-version 'vor' or 'unspecified' and start <= freeze date
    (the freeze date stands in for "today" so the build is reproducible).
    """
    out = []
    for lic in licences or []:
        start, _ = from_date_parts(lic.get("start"))
        version = lic.get("content-version")
        cls = classify_licence_url(lic.get("URL"), version) or {}
        reason = None
        if version not in ("vor", "unspecified"):
            reason = f"content-version {version!r} is not vor/unspecified"
        elif start and start > freeze_date:
            reason = "start date after freeze date"
        out.append({
            "source": "crossref", "url": lic.get("URL"), "content_version": version,
            "start": start, "delay_in_days": lic.get("delay-in-days"),
            "id": cls.get("id"), "is_cc": cls.get("is_cc", False),
            "applicable": reason is None, "not_applicable_reason": reason,
        })
    return sorted(out, key=lambda c: (c["url"] or "", c["content_version"] or "", c["start"] or ""))


def _latest(cands):
    return sorted(cands, key=lambda c: (c["start"] or "", c["content_version"] == "vor", c["is_cc"], c["url"] or ""),
                  reverse=True)[0]


def select_crossref_licence(candidates, freeze_date=None):
    """Latest-starting applicable licence; prefer vor, then CC, then URL order.

    If no vor/unspecified licence applies but a text-and-data-mining ('tdm')
    licence has started, report it as a TDM-only licence: it is never treated
    as permission to process or redistribute text here.
    """
    usable = [c for c in candidates if c["applicable"]]
    if usable:
        best = _latest(usable)
        return classify_licence_url(best["url"], best["content_version"])
    tdm = [c for c in candidates if c["content_version"] == "tdm" and c["url"]
           and (freeze_date is None or not c["start"] or c["start"] <= freeze_date)]
    if not tdm:
        return None
    best = _latest(tdm)
    cls = classify_licence_url(best["url"], "tdm")
    if cls["is_cc"]:
        return {"id": cls["id"] + "+tdm-only", "family": "cc-tdm-only", "is_cc": False,
                "restrictions": [], "url": cls["url"], "underlying_family": cls["family"]}
    return {"id": "publisher-tdm", "family": "publisher-tdm", "is_cc": False, "restrictions": [],
            "url": cls["url"]}


def _restriction_rank(lic):
    if lic is None:
        return -1
    if not lic["is_cc"]:
        return 99
    return len(lic["restrictions"])


def combine_licences(crossref_lic, epmc_lic):
    """Merge Crossref and Europe PMC licence signals; disagreement => more restrictive."""
    chosen, source, disagreement = None, None, False
    if crossref_lic and epmc_lic and crossref_lic.get("underlying_family") and \
            crossref_lic["underlying_family"] == epmc_lic["family"]:
        # Crossref states the same CC licence for the TDM version only; Europe PMC states it for the article.
        return epmc_lic, "europepmc", False
    if crossref_lic and epmc_lic:
        if crossref_lic["family"] == epmc_lic["family"]:
            chosen, source = crossref_lic, "crossref"
        else:
            disagreement = True
            if _restriction_rank(epmc_lic) > _restriction_rank(crossref_lic):
                chosen, source = epmc_lic, "europepmc"
            else:
                chosen, source = crossref_lic, "crossref"
    elif crossref_lic:
        chosen, source = crossref_lic, "crossref"
    elif epmc_lic:
        chosen, source = epmc_lic, "europepmc"
    return chosen, source, disagreement


def licence_detail(crossref_msg, epmc, freeze_date):
    candidates = crossref_licence_candidates((crossref_msg or {}).get("license"), freeze_date)
    cr = select_crossref_licence(candidates, freeze_date)
    ep = classify_epmc_licence((epmc or {}).get("license"))
    if ep:
        candidates.append({"source": "europepmc", "value": ep.get("raw"), "id": ep["id"],
                           "is_cc": ep["is_cc"], "applicable": True, "not_applicable_reason": None})
    chosen, source, disagreement = combine_licences(cr, ep)
    is_cc = bool(chosen and chosen["is_cc"])
    restrictions = list(chosen["restrictions"]) if chosen else []
    text_ok = is_cc
    redistribute = bool(is_cc and chosen["family"] in REDISTRIBUTABLE_FAMILIES and not disagreement)
    notes = [RESTRICTION_NOTES[r] for r in restrictions]
    if disagreement:
        notes.append("Crossref and Europe PMC licences disagree; the more restrictive is used")
    if chosen and chosen["family"] in ("publisher-tdm", "cc-tdm-only"):
        notes.append("only a text-and-data-mining (tdm) licence is declared; not used as permission here")
    if chosen and not is_cc:
        notes.append("not an explicit Creative Commons licence: no text processing or redistribution")
    if chosen is None:
        notes.append("no licence metadata found")
    return {
        "id": chosen["id"] if chosen else None,
        "url": chosen.get("url") if chosen else None,
        "family": chosen["family"] if chosen else None,
        "is_creative_commons": is_cc,
        "restrictions": restrictions,
        "source": source,
        "sources_disagree": disagreement,
        "candidates": candidates,
        "notes": notes,
        "text_processing_allowed": text_ok,
        "redistribution_allowed": redistribute,
    }


def fulltext_in_europepmc(epmc):
    """Open-access full text served by Europe PMC (needed for fullTextXML)."""
    if not epmc:
        return False
    return bool(epmc.get("pmcid")) and epmc.get("inEPMC") == "Y" and epmc.get("isOpenAccess") == "Y"


def access_status(lic, epmc, openalex):
    if lic["is_creative_commons"] and fulltext_in_europepmc(epmc):
        return "open_access_licensed"
    signals = []
    if lic["is_creative_commons"]:
        signals.append("cc_licence_without_europepmc_full_text")
    if epmc and epmc.get("isOpenAccess") == "Y":
        signals.append("europepmc_isOpenAccess")
    if epmc and epmc.get("inPMC") == "Y":
        signals.append("in_pmc")
    if openalex and (openalex.get("open_access") or {}).get("is_oa"):
        signals.append("openalex_is_oa")
    return "open_access_unclear" if signals else "closed_or_unknown"


_RETRACTED = {"retraction", "withdrawal", "removal"}
_PARTIAL = {"partial_retraction", "partial-retraction"}
_CONCERN = {"expression_of_concern", "expression-of-concern"}
_CORRECTED = {"correction", "erratum", "corrigendum", "addendum", "clarification"}
_EPMC_COMMENT = {
    "retraction in": "retracted",
    "partial retraction in": "partially_retracted",
    "expression of concern in": "expression_of_concern",
    "erratum in": "corrected",
    "corrected and republished in": "corrected",
    "retraction of": "is_update_notice",
    "erratum for": "is_update_notice",
    "partial retraction of": "is_update_notice",
    "expression of concern for": "is_update_notice",
}
_EPMC_PUBTYPE = {
    "retracted publication": "retracted",
    "retraction of publication": "is_update_notice",
    "published erratum": "is_update_notice",
    "expression of concern": "is_update_notice",
}
STATUS_PRIORITY = ("retracted", "partially_retracted", "is_update_notice",
                   "expression_of_concern", "corrected", "none_found")


def _update_status(update_type):
    t = (update_type or "").lower()
    if t in _RETRACTED:
        return "retracted"
    if t in _PARTIAL:
        return "partially_retracted"
    if t in _CONCERN:
        return "expression_of_concern"
    if t in _CORRECTED:
        return "corrected"
    return None


def retraction_detail(crossref_msg, openalex, epmc):
    """Combine Crossref updated-by/update-to/relation, OpenAlex and Europe PMC signals."""
    evidence = []
    consulted = []
    if crossref_msg is not None:
        consulted.append("crossref")
        for item in crossref_msg.get("updated-by") or []:
            status = _update_status(item.get("type"))
            if status:
                evidence.append({"source": "crossref.updated-by", "type": item.get("type"),
                                 "doi": item.get("DOI"), "origin": item.get("source"),
                                 "status": status})
        for item in crossref_msg.get("update-to") or []:
            evidence.append({"source": "crossref.update-to", "type": item.get("type"),
                             "doi": item.get("DOI"), "origin": item.get("source"),
                             "status": "is_update_notice"})
        for rel_type, items in sorted((crossref_msg.get("relation") or {}).items()):
            rt = rel_type.lower()
            status = ("retracted" if "retract" in rt else
                      "corrected" if "correct" in rt or "errat" in rt else None)
            if status and rt.startswith("is-"):
                for item in items or []:
                    evidence.append({"source": "crossref.relation", "type": rel_type,
                                     "doi": item.get("id"), "origin": None, "status": status})
    if openalex is not None:
        consulted.append("openalex")
        if openalex.get("is_retracted") is True:
            evidence.append({"source": "openalex.is_retracted", "type": "is_retracted",
                             "doi": None, "origin": None, "status": "retracted"})
    if epmc is not None:
        consulted.append("europepmc")
        for pt in ((epmc.get("pubTypeList") or {}).get("pubType") or []):
            status = _EPMC_PUBTYPE.get(str(pt).lower())
            if status:
                evidence.append({"source": "europepmc.pubTypeList", "type": pt,
                                 "doi": None, "origin": None, "status": status})
        for cc in ((epmc.get("commentCorrectionList") or {}).get("commentCorrection") or []):
            status = _EPMC_COMMENT.get(str(cc.get("type") or "").lower())
            if status:
                evidence.append({"source": "europepmc.commentCorrectionList", "type": cc.get("type"),
                                 "doi": None, "origin": cc.get("id"), "status": status})
    if not consulted:
        status = "unknown"
    else:
        found = {e["status"] for e in evidence}
        status = next((s for s in STATUS_PRIORITY if s in found), "none_found")
    evidence.sort(key=lambda e: (e["source"], str(e["type"]), str(e["doi"]), str(e["origin"])))
    return {"status": status, "evidence": evidence, "sources_consulted": sorted(consulted)}
