"""Assemble one corpus record from the register row and cached API responses (pure)."""
from __future__ import annotations

from . import PIPELINE_VERSION, RULES_VERSION
from . import dates as D
from . import extract as X
from . import ids
from . import rights as RT
from .ingest import split_authors

SCREENING_STATUSES = ("included_title_abstract_v0", "excluded_out_of_window",
                      "excluded_retracted", "needs_check")
ACCESS_STATUSES = ("open_access_licensed", "open_access_unclear", "closed_or_unknown")
REVIEWER_STATUS = "not_reviewed"
_SOURCE_RANK = {"MED": 0, "PMC": 1, "PPR": 2}


def body_if_ok(env):
    return env["body"] if env and env.get("status") == "ok" else None


def select_epmc(envs, doi, pmid):
    """Pick the Europe PMC result for this DOI (or register PMID); MED first.

    Returns (result, envelope_it_came_from, other_matching_ids).
    """
    results = []
    for order, env in enumerate(envs):
        body = body_if_ok(env)
        if body:
            results.extend((order, env, r) for r in (body.get("resultList") or {}).get("result") or [])
    matches = [t for t in results
               if ids.normalize_doi(t[2].get("doi")) == doi or (pmid and str(t[2].get("pmid") or "") == pmid)]
    matches.sort(key=lambda t: (ids.normalize_doi(t[2].get("doi")) != doi,
                                _SOURCE_RANK.get(str(t[2].get("source")), 9), str(t[2].get("id")), t[0]))
    if not matches:
        return None, None, []
    chosen = matches[0]
    others = sorted({f"{t[2].get('source')}:{t[2].get('id')}" for t in matches[1:]} -
                    {f"{chosen[2].get('source')}:{chosen[2].get('id')}"})
    return chosen[2], chosen[1], others


def doi_resolution(handle_env, crossref_env):
    """True if doi.org resolves the handle (or Crossref has the work); False only if both say no."""
    if handle_env and handle_env.get("status") == "ok" and (handle_env.get("body") or {}).get("responseCode") == 1:
        return True, "doi.org handle API responseCode 1"
    if crossref_env and crossref_env.get("status") == "ok":
        return True, "Crossref /works record exists"
    handle_missing = handle_env and handle_env.get("status") == "not_found"
    crossref_missing = crossref_env and crossref_env.get("status") == "not_found"
    if handle_missing and crossref_missing:
        return False, "doi.org handle not found (responseCode 100) and no Crossref record"
    return None, "resolution not confirmed (fetch missing or inconclusive)"


def _src(env):
    if not env:
        return None
    return {"endpoint": env.get("endpoint"), "url": env.get("url"), "retrieved_at": env.get("fetched_at"),
            "http_status": env.get("http_status"), "status": env.get("status")}


def _prov(source, env=None, rule=None, **extra):
    p = {"source": source}
    if env:
        p.update({"endpoint": env.get("endpoint"), "retrieved_at": env.get("fetched_at")})
    if rule:
        p["rule"] = rule
    p.update(extra)
    return p


def assign_flags(ctx):
    """Design-based caution labels. All machine-assigned; none is a quality grade."""
    flags = {}

    def add(fid, rule):
        flags[fid] = {"id": fid, "assigned_by": "machine", "rule": rule, "status": "machine_assigned_unreviewed"}

    if "abstract" in (ctx["register_review_status"] or ""):
        add("abstract_only_screening", "register review_status says title/abstract screening only; no full-text appraisal")
    labels = set(ctx["study_type"]["labels"])
    if labels & X.REVIEW_LABELS:
        add("review_overlap_possible", "review-type label: included primary studies may overlap other records")
    if labels & X.OBSERVATIONAL_LABELS:
        add("observational", "observational design label: associations, not causal effects")
    if "qualitative" in labels:
        add("qualitative_design", "qualitative label: no effect estimate")
    if "protocol" in labels:
        add("protocol_not_outcomes", "protocol label: reports plans, not outcomes")
    if labels & {"randomized_controlled_trial", "randomized_trial", "clinical_trial"}:
        add("trial_design_unverified", "trial label from index/title; comparator, blinding and risk of bias unverified")
    if labels & {"conference_paper", "preprint"}:
        add("conference_or_preprint", "conference paper or preprint type")
    countries = set((ctx["country"] or {}).get("labels", []))
    if not countries:
        add("population_country_not_stated", "no explicit country/language term in title or register annotation")
    elif "Indonesia" not in countries:
        add("population_not_indonesian", "explicit country/language terms found and none is Indonesia")
    pops = set((ctx["population"] or {}).get("labels", []))
    if pops & {"adolescents", "children", "young people or youth"}:
        add("population_includes_minors_or_youth", "explicit youth/adolescent/child term; Sela demo is adult-only")
    kinds = {(d["kind"], d["source"].split(".")[0], d["type"]) for d in ctx["discrepancies"]}
    if any(k[0] == "primary" for k in kinds):
        add("date_discrepancy_primary", "primary date sources disagree or lack day precision")
    if any(k[0] == "register" for k in kinds):
        add("register_date_differs", "register date/year differs from the verified canonical date")
    if any(d["type"] == "year_mismatch" and d["source"] in ("crossref.published_print", "europepmc.print_publication_date")
           for d in ctx["discrepancies"]):
        add("print_year_differs", "print/issue year differs from online-first year")
    status = ctx["retraction"]["status"]
    if status in ("retracted", "partially_retracted", "expression_of_concern", "corrected", "is_update_notice"):
        add(status, f"retraction/correction signal: {status}")
    if ctx["doi_resolvable"] is False:
        add("doi_not_resolvable", "doi.org and Crossref do not resolve the DOI")
    elif ctx["doi_resolvable"] is None:
        add("doi_resolution_unknown", "DOI resolution not confirmed")
    if ctx["epmc"] is None:
        add("not_in_europepmc", "no Europe PMC record matched the DOI/PMID")
    lic = ctx["licence"]
    if lic["id"] is None:
        add("licence_unknown", "no licence metadata from Crossref or Europe PMC")
    elif not lic["redistribution_allowed"]:
        add("licence_restricts_redistribution", "licence is not CC BY / BY-SA / CC0 (or sources disagree)")
    if lic["sources_disagree"]:
        add("licence_sources_disagree", "Crossref and Europe PMC licences differ")
    if ctx["fulltext_processed"]:
        add("full_text_machine_extracted_unreviewed", "fields extracted by deterministic patterns; nobody reviewed them")
    if ctx["sample"] and ctx["sample"]["note"].startswith("conflicting"):
        add("sample_size_conflicting_candidates", "several different explicit sample sizes; value left null")
    if ctx["pmid_mismatch"]:
        add("pmid_mismatch", "register PMID differs from Europe PMC PMID")
    if ctx["within"] is False:
        add("out_of_window", "verified date outside the rolling five-year window")
    return [flags[k] for k in sorted(flags)]


def screening(retraction_status, canonical, conflict_reasons, doi_resolvable, within):
    reasons = []
    if retraction_status == "retracted":
        return "excluded_retracted", ["retraction signal (Crossref updated-by / OpenAlex / Europe PMC)"]
    if canonical is None:
        reasons.append("no_primary_day_precision_date")
    reasons.extend(conflict_reasons)
    if doi_resolvable is False:
        reasons.append("doi_not_resolvable")
    elif doi_resolvable is None:
        reasons.append("doi_resolution_unknown")
    if retraction_status in ("partially_retracted", "is_update_notice", "unknown"):
        reasons.append("retraction_status_" + retraction_status)
    if reasons:
        return "needs_check", reasons
    if not within:
        return "excluded_out_of_window", ["verified canonical date outside cutoff..freeze window"]
    return "included_title_abstract_v0", ["inherited title/abstract screening; verified date inside window"]


def _labelled_field(item):
    if not item:
        return None
    return {k: item[k] for k in ("section_title", "locator", "snippet", "snippet_sha256", "source_url", "status")}


def build_record(paper, envs, freeze_date, cutoff_date, features_by_rid, fulltext_xml=None):
    """``envs`` maps 'europepmc_doi', 'europepmc_pmid', 'crossref', 'openalex',
    'doi_handle', 'fulltext' to cache envelopes (or None)."""
    rid = paper["id"]
    doi = ids.normalize_doi(paper.get("doi"))
    reg_pmid = ids.pmid_from_url(paper.get("index_url"))
    epmc, epmc_env, epmc_others = select_epmc([envs.get("europepmc_doi"), envs.get("europepmc_pmid")],
                                               doi, reg_pmid)
    cr_env, oa_env, handle_env = envs.get("crossref"), envs.get("openalex"), envs.get("doi_handle")
    cr = (body_if_ok(cr_env) or {}).get("message") if body_if_ok(cr_env) else None
    oa = body_if_ok(oa_env)

    epmc_pmid = ids.normalize_pmid((epmc or {}).get("pmid"))
    pmid = epmc_pmid or reg_pmid
    pmcid = ids.normalize_pmcid((epmc or {}).get("pmcid"))
    title = " ".join(str(paper.get("title") or "").split())
    note = paper.get("abstract_based_note_id")

    dmap, dprec = D.collect_dates(epmc, cr, oa, paper)
    rec = D.reconcile(dmap, dprec, freeze_date, cutoff_date)
    canonical = rec["canonical"]
    within = D.within_window(canonical, cutoff_date, freeze_date)
    lic = RT.licence_detail(cr, epmc, freeze_date)
    access = RT.access_status(lic, epmc, oa)
    retraction = RT.retraction_detail(cr, oa, epmc)
    resolvable, resolvable_basis = doi_resolution(handle_env, cr_env)
    stype = X.study_type(((epmc or {}).get("pubTypeList") or {}).get("pubType"), (cr or {}).get("type"),
                         title, paper.get("publication_types_from_index"))
    population = X.match_terms({"title": title, "note": note}, X.POPULATION_TERMS)
    country = X.match_terms({"title": title, "note": note}, X.COUNTRY_TERMS)

    ft_env = envs.get("fulltext")
    fulltext_processed = False
    fulltext_check = None
    evidence = []
    sample = None
    labelled = {}
    if lic["text_processing_allowed"] and pmcid:
        if ft_env and ft_env.get("status") == "ok" and fulltext_xml:
            parsed = X.parse_fulltext(fulltext_xml)
            fulltext_check = {"xml_licence": parsed["licence"], "url": ft_env.get("url"),
                              "retrieved_at": ft_env.get("fetched_at"), "sha256": ft_env.get("sha256")}
            if parsed["licence"]["is_cc"]:
                fulltext_processed = True
                url = ft_env.get("url")
                sample, candidates = X.extract_sample_size(parsed["paragraphs"], url,
                                                          is_review=bool(set(stype["labels"]) & X.REVIEW_LABELS))
                labelled = X.extract_labelled_sections(parsed["paragraphs"], url)
                evidence = (candidates + X.extract_design_phrases(parsed["paragraphs"], url)
                            + X.extract_countries(parsed["paragraphs"], url) + list(labelled.values()))
                fulltext_check["paragraphs_scanned"] = len(parsed["paragraphs"])
            else:
                fulltext_check["skipped"] = "XML licence statement is not an explicit Creative Commons licence"
        else:
            fulltext_check = {"skipped": "eligible but no cached open-access full text "
                                         f"({(ft_env or {}).get('status', 'not fetched')})"}
    if sample is None:
        sample = {"value": None, "candidate_values": [], "status": "not_extracted",
                  "note": "no licensed full text processed" if not fulltext_processed else "",
                  "rule": None}
    evidence.sort(key=lambda e: (e["field"], str(e["value"]), e["locator"]["part"], e["locator"]["section"],
                                 e["locator"]["paragraph"], e.get("pattern", "")))

    status, reasons = screening(retraction["status"], canonical, rec["conflict_reasons"], resolvable, within)
    pmid_mismatch = bool(reg_pmid and epmc_pmid and reg_pmid != epmc_pmid)
    flags = assign_flags({
        "register_review_status": paper.get("review_status"), "study_type": stype, "country": country,
        "population": population, "discrepancies": rec["discrepancies"], "retraction": retraction,
        "doi_resolvable": resolvable, "epmc": epmc, "licence": lic, "fulltext_processed": fulltext_processed,
        "sample": sample if fulltext_processed else None, "pmid_mismatch": pmid_mismatch, "within": within,
    })
    used = [e for e in (epmc_env, cr_env, oa_env, handle_env, ft_env if fulltext_processed else None)
            if e and e.get("fetched_at")]
    retrieval_date = max(e["fetched_at"] for e in used)[:10] if used else None
    authors, truncated = split_authors(paper.get("authors"))
    canonical_env = None
    if rec["canonical_sources"]:
        first = rec["canonical_sources"][0]
        canonical_env = cr_env if first.startswith("crossref") else epmc_env
    reg = "register:docs/research/sela-relay/research_100.json"

    provenance = {
        "id": _prov(reg), "title": _prov(reg), "authors": _prov(reg), "journal": _prov(reg),
        "theme": _prov(reg), "relevance_to_sela": _prov(reg, rule="register annotation + docs 07/10 feature tables"),
        "doi": _prov(reg, rule="normalised: lowercase, resolver prefix stripped",
                     verified_by=resolvable_basis),
        "pmid": (_prov("europepmc", epmc_env) if epmc_pmid else
                 _prov(reg, rule="parsed from index_url") if reg_pmid else _prov(None)),
        "pmcid": _prov("europepmc", epmc_env) if pmcid else _prov(None),
        "publication_date": _prov(",".join(rec["canonical_sources"]) or None, canonical_env,
                                  rule="earliest DAY-precision date among crossref.published_online, "
                                       "europepmc.first_publication_date, europepmc.electronic_publication_date"),
        "license": _prov(lic["source"], cr_env if lic["source"] == "crossref" else epmc_env if lic["source"] else None,
                         rule="Crossref vor/unspecified licence with start <= freeze date, cross-checked with Europe PMC"),
        "access_status": _prov("derived", rule="CC licence + Europe PMC OA full text => open_access_licensed"),
        "retraction_status": _prov(",".join(retraction["sources_consulted"]) or None,
                                   rule="Crossref updated-by/update-to/relation, OpenAlex is_retracted, Europe PMC"),
        "study_type": _prov("derived", rule=stype["rule"]),
        "population": _prov("derived", rule="explicit-term lexicon (title + register note)"),
        "country_or_language": _prov("derived", rule="explicit-term lexicon (title + register note)"),
        "sample_size": (_prov("europepmc_fulltext", ft_env, rule=sample.get("rule")) if fulltext_processed
                        else _prov(None, rule="not extracted")),
        "screening_status": _prov("derived", rule=RULES_VERSION),
        "within_cutoff": _prov("derived", rule=f"{cutoff_date} <= publication_date <= {freeze_date}"),
    }
    for field in ("intervention_or_exposure", "comparator", "outcomes", "limitations"):
        provenance[field] = (_prov("europepmc_fulltext", ft_env, rule="explicitly titled section")
                             if labelled.get(field) else _prov(None, rule="null unless explicitly labelled section"))

    return {
        "id": rid,
        "title": title,
        "authors": authors,
        "authors_truncated": truncated,
        "publication_date": canonical,
        "year": int(canonical[:4]) if canonical else None,
        "doi": doi,
        "pmid": pmid,
        "pmcid": pmcid,
        "journal": paper.get("journal"),
        "theme": paper.get("theme"),
        "source_url": paper.get("url") or (f"https://doi.org/{doi}" if doi else None),
        "index_url": paper.get("index_url"),
        "study_type": stype["primary"],
        "study_type_detail": stype,
        "population": population["labels"] if population else None,
        "population_detail": population,
        "country_or_language": country["labels"] if country else None,
        "country_or_language_detail": country,
        "sample_size": sample["value"],
        "sample_size_detail": sample,
        "intervention_or_exposure": _labelled_field(labelled.get("intervention_or_exposure")),
        "comparator": _labelled_field(labelled.get("comparator")),
        "outcomes": _labelled_field(labelled.get("outcomes")),
        "limitations": _labelled_field(labelled.get("limitations")),
        "relevance_to_sela": {
            "theme": paper.get("theme"),
            "note_id": note,
            "transfer_limitation_id": paper.get("transfer_limitation_id"),
            "features": features_by_rid.get(rid, []),
            "status": "register_annotation_abstract_based_not_full_text_appraisal",
        },
        "license": lic["id"],
        "license_detail": {k: lic[k] for k in ("id", "url", "family", "is_creative_commons", "restrictions",
                                               "source", "sources_disagree", "candidates", "notes")},
        "access_status": access,
        "text_processing_allowed": lic["text_processing_allowed"],
        "redistribution_allowed": lic["redistribution_allowed"],
        "screening_status": status,
        "screening_reasons": reasons,
        "within_cutoff": within,
        "window_by_any_source": D.window_by_any_source(dmap, dprec, cutoff_date, freeze_date),
        "dates": dmap,
        "date_precision": dprec,
        "date_discrepancies": rec["discrepancies"],
        "retraction_status": retraction["status"],
        "retraction_detail": retraction,
        "doi_resolvable": resolvable,
        "europepmc_id": f"{epmc.get('source')}:{epmc.get('id')}" if epmc else None,
        "europepmc_other_matches": epmc_others,
        "register_pmid": reg_pmid,
        "register_publication_year": paper.get("publication_year"),
        "register_review_status": paper.get("review_status"),
        "extraction_source": "europepmc_oa_fulltext_xml" if fulltext_processed else "metadata_only",
        "fulltext_check": fulltext_check,
        "evidence": evidence,
        "flags": flags,
        "sources": {"europepmc": _src(epmc_env), "crossref": _src(cr_env), "openalex": _src(oa_env),
                    "doi_handle": _src(handle_env), "fulltext": _src(ft_env) if fulltext_processed else None},
        "provenance": provenance,
        "retrieval_date": retrieval_date,
        "reviewer_status": REVIEWER_STATUS,
        "pipeline_version": PIPELINE_VERSION,
    }
