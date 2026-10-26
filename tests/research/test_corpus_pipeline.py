"""Offline tests for the Sela research corpus pipeline (no network).

Fixtures under tests/research/fixtures/corpus/: trimmed real metadata responses
(no abstracts, no author lists) and SYNTHETIC full-text XML written for these
tests. Synthetic DOIs use the 10.5555 test prefix.
"""
import hashlib
import io
import json
import shutil
import sys
import tempfile
import time
import unittest
from contextlib import redirect_stdout
from pathlib import Path
from urllib.error import HTTPError

HERE = Path(__file__).resolve().parent
REPO = HERE.parents[1]
sys.path.insert(0, str(REPO / "scripts" / "research"))

import corpus_pipeline as P  # noqa: E402
from corpus import claims as C  # noqa: E402
from corpus import dates as D  # noqa: E402
from corpus import evidence as EV  # noqa: E402
from corpus import export as E  # noqa: E402
from corpus import extract as X  # noqa: E402
from corpus import fetch as F  # noqa: E402
from corpus import ids  # noqa: E402
from corpus import ingest as IN  # noqa: E402
from corpus import records as R  # noqa: E402
from corpus import rights as RT  # noqa: E402
from corpus import verify as V  # noqa: E402

FIX = HERE / "fixtures" / "corpus"
MINI = FIX / "repo"
FETCHED = "2026-10-26T00:00:00Z"
FREEZE = "2026-10-26"


def load(name):
    return json.loads((FIX / name).read_text(encoding="utf-8"))


def put(cache, service, key, url, status, body=None, http=200, **extra):
    cache.put(service, key, F.envelope(service, key, url, FETCHED, http, status, body=body, extra=extra or None))


EMPTY_EPMC = {"hitCount": 0, "resultList": {"result": []}}


def make_cache(cache_dir):
    """Cache for the 5-record mini register (see fixtures/corpus/repo)."""
    cache = F.Cache(cache_dir)

    def meta(doi, epmc, crossref, openalex, handle):
        for service, req, payload in (("europepmc", F.epmc_doi_request(doi), epmc),
                                      ("crossref", F.crossref_request(doi), crossref),
                                      ("openalex", F.openalex_request(doi), openalex),
                                      ("doi_handle", F.doi_handle_request(doi), handle)):
            key, url = req
            if payload is None:
                put(cache, service, key, url, "not_found", None, http=404)
            else:
                put(cache, service, key, url, "ok", payload)

    ok_handle = {"responseCode": 1}
    meta("10.1177/20552076241313401", load("europepmc_R001.json"), load("crossref_R001.json"),
         load("openalex_R001.json"), ok_handle)
    xml = (FIX / "fulltext_single.xml").read_bytes()
    key, url = F.fulltext_request("PMC11748072")
    cache.put("fulltext", key, F.envelope("fulltext", key, url, FETCHED, 200, "ok",
                                          extra={"sha256": hashlib.sha256(xml).hexdigest(), "bytes": len(xml)}),
              raw_bytes=xml)
    meta("10.1177/1758835920922055", load("europepmc_retracted.json"), load("crossref_retracted_original.json"),
         load("openalex_retracted.json"), ok_handle)
    key, url = F.fulltext_request("PMC7222246")
    put(cache, "fulltext", key, url, "not_found", http=404)
    cc_by = [{"URL": "https://creativecommons.org/licenses/by/4.0/", "content-version": "vor",
              "start": {"date-parts": [[2019, 5, 1]]}}]
    meta("10.5555/sela-test-old", EMPTY_EPMC,
         {"message": {"DOI": "10.5555/sela-test-old", "type": "journal-article",
                      "published-online": {"date-parts": [[2019, 5, 1]]}, "license": cc_by}},
         None, ok_handle)
    nc_nd = [{"URL": "http://creativecommons.org/licenses/by-nc-nd/4.0/", "content-version": "vor",
              "start": {"date-parts": [[2024, 3, 1]]}}]
    meta("10.5555/sela-test-month", EMPTY_EPMC,
         {"message": {"DOI": "10.5555/sela-test-month", "type": "journal-article",
                      "published-online": {"date-parts": [[2024, 3]]}, "license": nc_nd}},
         {"publication_date": "2024-03-01", "publication_year": 2024, "is_retracted": False}, None)
    meta("10.5555/sela-test-unresolvable", EMPTY_EPMC, None, None, None)
    return cache


class BuildCase(unittest.TestCase):
    def setUp(self):
        self.tmp = Path(tempfile.mkdtemp())
        self.cache_dir = self.tmp / "cache"
        make_cache(self.cache_dir)
        self.inputs = P.Inputs(MINI)

    def tearDown(self):
        shutil.rmtree(self.tmp, ignore_errors=True)

    def build(self, out_name="out", freeze=FREEZE, inputs=None):
        return P.run_build(inputs or self.inputs, self.cache_dir, self.tmp / out_name, freeze)

    def records(self, out_name="out"):
        lines = (self.tmp / out_name / "records.jsonl").read_text(encoding="utf-8").splitlines()
        return {r["id"]: r for r in map(json.loads, lines)}


class IdentifierTests(unittest.TestCase):
    def test_doi_normalisation(self):
        self.assertEqual(ids.normalize_doi("HTTPS://doi.org/10.1234/AbC. "), "10.1234/abc")
        self.assertEqual(ids.normalize_doi("doi: https://dx.doi.org/10.1/X"), "10.1/x")
        self.assertEqual(ids.normalize_doi(" 10.1016/S0140-6736(20)30183-5 "), "10.1016/s0140-6736(20)30183-5")
        self.assertEqual(ids.normalize_doi("10.1/abc)"), "10.1/abc")
        self.assertIsNone(ids.normalize_doi("  "))
        self.assertIsNone(ids.normalize_doi(None))
        self.assertTrue(ids.is_valid_doi("10.1056/aioa2400802"))
        self.assertFalse(ids.is_valid_doi("not-a-doi"))

    def test_pmid_pmcid(self):
        self.assertEqual(ids.pmid_from_url("https://pubmed.ncbi.nlm.nih.gov/39839954/"), "39839954")
        self.assertIsNone(ids.pmid_from_url("https://arxiv.org/abs/2504.18412"))
        self.assertEqual(ids.normalize_pmcid("pmc123"), "PMC123")
        self.assertEqual(ids.normalize_pmcid("123"), "PMC123")
        self.assertIsNone(ids.normalize_pmcid("PMCX"))

    def test_dedupe(self):
        rows = [
            {"id": "R1", "doi": "https://doi.org/10.1/ABC", "title": "Alpha study", "year": "2025"},
            {"id": "R2", "doi": "10.1/abc", "title": "Different", "year": "2025"},
            {"id": "R3", "doi": "10.1/x", "pmid": "42", "title": "Beta", "year": "2024"},
            {"id": "R4", "doi": "10.1/y", "pmid": "42", "title": "Gamma", "year": "2024"},
            {"id": "R5", "doi": "10.1/z", "title": "Délta: study!", "year": "2023"},
            {"id": "R6", "doi": "10.1/w", "title": "delta study", "year": "2023"},
            {"id": "R7", "doi": "10.1/v", "title": "delta study", "year": "2022"},
        ]
        found = {(d["key_type"], tuple(d["ids"])) for d in ids.find_duplicates(rows)}
        self.assertEqual(found, {("doi", ("R1", "R2")), ("pmid", ("R3", "R4")), ("title_year", ("R5", "R6"))})
        self.assertEqual(ids.find_duplicates(rows[:1] + rows[2:3]), [])

    def test_real_register_has_no_duplicates_and_copies_agree(self):
        inputs = P.Inputs(REPO)
        papers = P.load_papers(inputs)
        self.assertEqual(len(papers), 100)
        dups = ids.find_duplicates([{"id": p["id"], "doi": p["doi"], "pmid": ids.pmid_from_url(p["index_url"]),
                                     "title": p["title"], "year": p["publication_year"]} for p in papers])
        self.assertEqual(dups, [])
        bib = IN.parse_bib(inputs.bib.read_text(encoding="utf-8"))
        md = IN.parse_register_md(inputs.register_md.read_text(encoding="utf-8"))
        self.assertEqual(len(bib), 100)
        self.assertEqual(len(md), 100)
        issues = IN.cross_check(papers, bib, md)
        self.assertTrue(all(i["field"] == "author_format" for i in issues), issues[:3])


class DateTests(unittest.TestCase):
    def test_canonical_is_earliest_primary_day_date(self):
        epmc = {"firstPublicationDate": "2025-01-17", "electronicPublicationDate": "2025-01-17", "pubYear": "2025",
                "journalInfo": {"printPublicationDate": "2025-01-01", "dateOfPublication": "2025 Jan-Dec"}}
        cr = {"published-online": {"date-parts": [[2025, 1, 20]]}, "published-print": {"date-parts": [[2025, 1]]}}
        oa = {"publication_date": "2025-01-01", "publication_year": 2025}
        dmap, prec = D.collect_dates(epmc, cr, oa, {"first_publication_date": "2025-01-17", "publication_year": "2025"})
        self.assertEqual(prec["europepmc.print_publication_date"], "year")
        self.assertEqual(dmap["europepmc.print_publication_date"], "2025")
        self.assertEqual(prec["openalex.publication_date"], "padded")
        rec = D.reconcile(dmap, prec, FREEZE, "2021-10-26")
        self.assertEqual(rec["canonical"], "2025-01-17")
        self.assertEqual(rec["canonical_sources"],
                         ["europepmc.electronic_publication_date", "europepmc.first_publication_date"])
        self.assertEqual(rec["discrepancies"], [])
        self.assertEqual(rec["conflict_reasons"], [])

    def test_gt_31_days_and_year_mismatch(self):
        cr = {"published-online": {"date-parts": [[2025, 3, 5]]}, "published-print": {"date-parts": [[2026, 4]]}}
        epmc = {"firstPublicationDate": "2025-01-01"}
        dmap, prec = D.collect_dates(epmc, cr, None, {"first_publication_date": "2025-01-01", "publication_year": "2026"})
        rec = D.reconcile(dmap, prec, FREEZE, "2021-10-26")
        self.assertEqual(rec["canonical"], "2025-01-01")
        types = {(d["source"], d["type"], d["kind"]) for d in rec["discrepancies"]}
        self.assertIn(("crossref.published_online", "gt_31_days", "primary"), types)
        self.assertIn(("crossref.published_print", "year_mismatch", "secondary"), types)
        self.assertIn(("register.publication_year", "year_mismatch", "register"), types)
        self.assertEqual(rec["conflict_reasons"], ["primary_dates_differ_gt_31_days"])
        gt = next(d for d in rec["discrepancies"] if d["type"] == "gt_31_days")
        self.assertEqual(gt["days"], 63)

    def test_month_precision_gives_no_canonical(self):
        cr = {"published-online": {"date-parts": [[2024, 3]]}}
        dmap, prec = D.collect_dates(None, cr, None, None)
        rec = D.reconcile(dmap, prec, FREEZE, "2021-10-26")
        self.assertIsNone(rec["canonical"])
        self.assertEqual({d["type"] for d in rec["discrepancies"]}, {"month_precision", "missing_primary_day_date"})

    def test_epmc_first_date_copied_from_padded_print_date_is_not_day_precision(self):
        epmc = {"firstPublicationDate": "2024-03-01",
                "journalInfo": {"printPublicationDate": "2024-03-01", "dateOfPublication": "2024 Mar"}}
        dmap, prec = D.collect_dates(epmc, None, None, None)
        self.assertEqual(prec["europepmc.first_publication_date"], "month")
        self.assertEqual(dmap["europepmc.first_publication_date"], "2024-03")
        self.assertIsNone(D.reconcile(dmap, prec)["canonical"])

    def test_primary_dates_straddling_window_conflict(self):
        dmap = {"europepmc.first_publication_date": "2021-10-20", "crossref.published_online": "2021-11-05"}
        prec = {k: "day" for k in dmap}
        rec = D.reconcile(dmap, prec, FREEZE, "2021-10-26")
        self.assertEqual(rec["canonical"], "2021-10-20")
        self.assertEqual(rec["conflict_reasons"], ["primary_dates_straddle_window"])

    def test_cutoff_arithmetic_including_leap_day(self):
        self.assertEqual(D.cutoff_for("2026-10-26"), "2021-10-26")
        self.assertEqual(D.cutoff_for("2028-02-29"), "2023-02-28")
        self.assertEqual(D.cutoff_for("2024-02-29"), "2019-02-28")
        self.assertEqual(D.cutoff_for("2028-02-28"), "2023-02-28")
        cutoff = D.cutoff_for("2028-02-29")
        self.assertTrue(D.within_window("2023-02-28", cutoff, "2028-02-29"))
        self.assertFalse(D.within_window("2023-02-27", cutoff, "2028-02-29"))
        self.assertTrue(D.within_window("2028-02-29", cutoff, "2028-02-29"))
        self.assertFalse(D.within_window("2028-03-01", cutoff, "2028-02-29"))
        self.assertTrue(D.within_window("2021-10-26", "2021-10-26", FREEZE))
        self.assertFalse(D.within_window("2021-10-25", "2021-10-26", FREEZE))
        self.assertIsNone(D.within_window(None, "2021-10-26", FREEZE))

    def test_window_by_any_source_is_informative(self):
        cut = "2021-10-26"
        self.assertEqual(D.window_by_any_source({"crossref.published_print": "2025-08"}, {"crossref.published_print": "month"},
                                                cut, FREEZE), "all_inside")
        self.assertEqual(D.window_by_any_source({"x": "2021"}, {"x": "year"}, cut, FREEZE), "mixed_or_uncertain")
        self.assertEqual(D.window_by_any_source({"x": "2019-01-02"}, {"x": "day"}, cut, FREEZE), "all_outside")
        self.assertEqual(D.window_by_any_source({"crossref.created": "2020-01-01"}, {"crossref.created": "day"},
                                                cut, FREEZE), "no_dates")
        self.assertEqual(D.window_by_any_source({"openalex.publication_date": "2021-12-01"},
                                                {"openalex.publication_date": "padded"}, cut, FREEZE),
                         "mixed_or_uncertain")


class LicenceTests(unittest.TestCase):
    def test_cc_by_4(self):
        lic = RT.licence_detail(load("crossref_R001.json")["message"],
                                load("europepmc_R001.json")["resultList"]["result"][0], FREEZE)
        self.assertEqual(lic["id"], "CC-BY-4.0")
        self.assertTrue(lic["text_processing_allowed"])
        self.assertTrue(lic["redistribution_allowed"])
        self.assertFalse(lic["sources_disagree"])

    def test_cc_by_nc_nd(self):
        cr = {"license": [{"URL": "http://creativecommons.org/licenses/by-nc-nd/4.0/", "content-version": "vor",
                           "start": {"date-parts": [[2024, 1, 1]]}}]}
        lic = RT.licence_detail(cr, None, FREEZE)
        self.assertEqual(lic["id"], "CC-BY-NC-ND-4.0")
        self.assertEqual(lic["restrictions"], ["NC", "ND"])
        self.assertTrue(lic["text_processing_allowed"])
        self.assertFalse(lic["redistribution_allowed"])
        self.assertTrue(any(n.startswith("ND") for n in lic["notes"]))

    def test_publisher_tdm_only_url(self):
        lic = RT.licence_detail(load("crossref_tdm_only.json")["message"], None, FREEZE)
        self.assertEqual(lic["id"], "publisher-tdm")
        self.assertFalse(lic["is_creative_commons"])
        self.assertFalse(lic["text_processing_allowed"])
        self.assertFalse(lic["redistribution_allowed"])
        self.assertEqual(RT.access_status(lic, None, None), "closed_or_unknown")

    def test_no_licence(self):
        lic = RT.licence_detail({"license": []}, {"license": None}, FREEZE)
        self.assertIsNone(lic["id"])
        self.assertFalse(lic["text_processing_allowed"])
        self.assertEqual(RT.access_status(lic, None, {"open_access": {"is_oa": True}}), "open_access_unclear")
        self.assertEqual(RT.access_status(lic, None, None), "closed_or_unknown")

    def test_future_start_and_accepted_manuscript_are_not_applicable(self):
        cr = {"license": [
            {"URL": "https://creativecommons.org/licenses/by/4.0/", "content-version": "vor",
             "start": {"date-parts": [[2027, 1, 1]]}},
            {"URL": "https://creativecommons.org/licenses/by/4.0/", "content-version": "am",
             "start": {"date-parts": [[2025, 1, 1]]}},
            {"URL": "https://www.example-publisher.org/terms", "content-version": "vor",
             "start": {"date-parts": [[2025, 1, 1]]}}]}
        lic = RT.licence_detail(cr, None, FREEZE)
        self.assertEqual(lic["id"], "publisher-specific")
        self.assertFalse(lic["text_processing_allowed"])

    def test_sources_disagree_uses_more_restrictive(self):
        cr = {"license": [{"URL": "https://creativecommons.org/licenses/by/4.0/", "content-version": "vor"}]}
        lic = RT.licence_detail(cr, {"license": "cc by-nc"}, FREEZE)
        self.assertEqual(lic["id"], "CC-BY-NC")
        self.assertTrue(lic["sources_disagree"])
        self.assertFalse(lic["redistribution_allowed"])

    def test_cc_declared_for_tdm_version_only(self):
        cr = {"license": [{"URL": "http://creativecommons.org/licenses/by-nc/4.0/", "content-version": "tdm",
                           "start": {"date-parts": [[2025, 10, 1]]}}]}
        alone = RT.licence_detail(cr, None, FREEZE)
        self.assertEqual(alone["id"], "CC-BY-NC-4.0+tdm-only")
        self.assertFalse(alone["text_processing_allowed"])
        agreed = RT.licence_detail(cr, {"license": "cc by-nc"}, FREEZE)
        self.assertEqual((agreed["id"], agreed["source"], agreed["sources_disagree"]), ("CC-BY-NC", "europepmc", False))
        self.assertTrue(agreed["text_processing_allowed"])
        self.assertFalse(agreed["redistribution_allowed"])

    def test_open_access_licensed_needs_epmc_full_text(self):
        epmc = load("europepmc_R001.json")["resultList"]["result"][0]
        lic = RT.licence_detail(load("crossref_R001.json")["message"], epmc, FREEZE)
        self.assertEqual(RT.access_status(lic, epmc, None), "open_access_licensed")
        self.assertEqual(RT.access_status(lic, dict(epmc, inEPMC="N"), None), "open_access_unclear")


class RetractionTests(unittest.TestCase):
    def test_crossref_updated_by_openalex_and_epmc(self):
        d = RT.retraction_detail(load("crossref_retracted_original.json")["message"],
                                 load("openalex_retracted.json"),
                                 load("europepmc_retracted.json")["resultList"]["result"][0])
        self.assertEqual(d["status"], "retracted")
        sources = {e["source"] for e in d["evidence"]}
        self.assertEqual(sources, {"crossref.updated-by", "openalex.is_retracted", "europepmc.pubTypeList",
                                   "europepmc.commentCorrectionList"})
        self.assertEqual(RT.retraction_detail(load("crossref_retracted_original.json")["message"], None, None)["status"],
                         "retracted")

    def test_notice_none_and_unknown(self):
        notice = RT.retraction_detail(load("crossref_retraction_notice.json")["message"], None, None)
        self.assertEqual(notice["status"], "is_update_notice")
        clean = RT.retraction_detail(load("crossref_R001.json")["message"], load("openalex_R001.json"), None)
        self.assertEqual(clean["status"], "none_found")
        self.assertEqual(RT.retraction_detail(None, None, None)["status"], "unknown")
        corrected = RT.retraction_detail({"updated-by": [{"type": "correction", "DOI": "10.1/c"}]}, None, None)
        self.assertEqual(corrected["status"], "corrected")


class ExtractionTests(unittest.TestCase):
    def parsed(self, name):
        return X.parse_fulltext((FIX / name).read_bytes())

    def test_single_sample_size_with_locators(self):
        ft = self.parsed("fulltext_single.xml")
        self.assertTrue(ft["licence"]["is_cc"])
        detail, candidates = X.extract_sample_size(ft["paragraphs"], "https://example.org/ft")
        self.assertEqual(detail["value"], 312)
        voting = [c for c in candidates if "excluded_reason" not in c]
        self.assertEqual({c["value"] for c in voting}, {312})
        self.assertNotIn(1050, {c["value"] for c in candidates})  # measures section is out of scope
        self.assertNotIn(999, {c["value"] for c in candidates})   # reference list never parsed
        self.assertNotIn(7, {c["value"] for c in candidates})     # citation <xref> removed
        for c in candidates:
            self.assertLessEqual(len(c["snippet"]), 240)
            self.assertIn(c["locator"]["part"], ("abstract", "body"))
            self.assertEqual(c["status"], "machine_extracted_unreviewed")
            self.assertEqual(c["snippet_sha256"], hashlib.sha256(c["snippet"].encode()).hexdigest())
        loc = {(c["locator"]["section"], c["locator"]["paragraph"]) for c in voting}
        self.assertEqual(loc, {("Methods", 1), ("Methods > Participants", 1)})

    def test_conflicting_sample_sizes_left_null(self):
        detail, candidates = X.extract_sample_size(self.parsed("fulltext_conflicting.xml")["paragraphs"], "u")
        self.assertIsNone(detail["value"])
        self.assertEqual(detail["candidate_values"], [98, 120])
        self.assertIn("conflicting", detail["note"])
        self.assertGreaterEqual(len(candidates), 3)

    def test_no_sample_size_no_guess(self):
        detail, candidates = X.extract_sample_size(self.parsed("fulltext_none.xml")["paragraphs"], "u")
        self.assertIsNone(detail["value"])
        self.assertEqual(candidates, [])

    def test_subgroup_excluded_and_review_left_null(self):
        para = [{"part": "abstract", "section": "Methods", "paragraph": 1,
                 "text": "We surveyed 200 students; missing data (n = 11, 3%) were excluded."}]
        detail, candidates = X.extract_sample_size(para, "u")
        self.assertEqual(detail["value"], 200)
        self.assertEqual([c["value"] for c in candidates if "excluded_reason" in c], [11])
        review, _ = X.extract_sample_size(para, "u", is_review=True)
        self.assertIsNone(review["value"])

    def test_labelled_sections_design_and_country(self):
        ft = self.parsed("fulltext_single.xml")
        labelled = X.extract_labelled_sections(ft["paragraphs"], "u")
        self.assertEqual(set(labelled), {"outcomes", "limitations"})
        self.assertEqual(labelled["outcomes"]["locator"]["section"], "Methods > Outcomes")
        designs = {e["value"] for e in X.extract_design_phrases(ft["paragraphs"], "u")}
        self.assertIn("cross-sectional", designs)
        self.assertEqual({e["value"] for e in X.extract_countries(ft["paragraphs"], "u")}, {"Indonesia"})

    def test_xml_without_cc_licence(self):
        xml = b"<article><front><article-meta><permissions><license><license-p>All rights reserved.</license-p>" \
              b"</license></permissions></article-meta></front><body/></article>"
        self.assertFalse(X.parse_fulltext(xml)["licence"]["is_cc"])

    def test_study_type_basis(self):
        st = X.study_type(["Systematic Review", "Journal Article"], "journal-article",
                          "A scoping review protocol", ["Review"])
        self.assertEqual(st["primary"], "protocol")
        self.assertEqual(set(st["labels"]), {"systematic_review", "journal_article", "review", "scoping_review",
                                             "protocol"})
        self.assertTrue(all("source" in b and "term" in b for b in st["basis"]))
        self.assertEqual(X.study_type([], None, "Untitled")["primary"], None)


class SampleRuleRegressionTests(unittest.TestCase):
    """Rules added after auditing real extractions (sentences here are invented)."""

    def votes(self, text, section="Methods", part="body"):
        detail, cands = X.extract_sample_size([{"part": part, "section": section, "paragraph": 1, "text": text}], "u")
        return detail["value"], sorted({c["value"] for c in cands if "excluded_reason" not in c}), cands

    def test_no_sentence_boundary_crossing(self):
        self.assertEqual(self.votes("Sessions lasted 32 to 56 minutes. Participants received a voucher.")[1], [])

    def test_approximate_and_pre_enrolment_counts_do_not_vote(self):
        value, votes, cands = self.votes("The clinics serve more than 700,000 individuals. Of the 96 eligible "
                                         "individuals, 40 participants enrolled.")
        self.assertEqual((value, votes), (40, [40]))
        reasons = {c["value"]: c["excluded_reason"] for c in cands if "excluded_reason" in c}
        self.assertEqual(reasons, {700000: "approximate or bounded quantity",
                                   96: "pre-enrolment count (eligible/invited/screened)"})

    def test_research_team_counts_do_not_vote(self):
        for text in ("Testing was conducted by 2 researchers and 2 medical students.",
                     "Then 3 qualified physicians independently annotated the outputs.",
                     "A panel of 8 physicians (A.B., C.D., and E.F.) developed the rubric.",
                     "Two raters (XY and ZW) scored them; 4 clinicians (AB and CD) reviewed disagreements."):
            self.assertEqual(self.votes(text)[1], [], text)

    def test_n_of_non_person_units_does_not_vote(self):
        self.assertEqual(self.votes("Raters needed 22 minutes per app (n=10) on average.")[1], [])

    def test_scope_excludes_results_setting_and_coder_sections(self):
        self.assertEqual(self.votes("We interviewed 30 participants.", section="Results > Participants")[1], [])
        self.assertEqual(self.votes("We interviewed 30 participants.", section="Methods > Setting")[1], [])
        self.assertEqual(self.votes("We interviewed 30 participants.", section="Methods > Coders")[1], [])
        self.assertEqual(self.votes("We interviewed 30 participants.", section="Methods > Settings and Participants")[1],
                         [30])
        self.assertEqual(self.votes("We interviewed 30 participants.", section="Results", part="abstract")[1], [30])
        self.assertEqual(self.votes("We recruited 30 participants.", section="Methods and analysis")[1], [30])
        self.assertEqual(self.votes("Data from 30 participants were used.", section="Statistical analysis")[1], [])

    def test_adjectives_between_number_and_people(self):
        self.assertEqual(self.votes("We enrolled 120 older adults.")[0], 120)
        self.assertEqual(self.votes("A survey of 1,234 U.S. university students.")[0], 1234)
        self.assertEqual(self.votes("People aged 18 or older adults were eligible.")[1], [])

    def test_licence_named_in_text_or_nested_link(self):
        nested = (b'<article xmlns:xlink="http://www.w3.org/1999/xlink"><front><article-meta><permissions><license>'
                  b'<license-p>Distributed under a <ext-link xlink:href="https://creativecommons.org/licenses/by/4.0/">'
                  b'BY 4.0</ext-link> licence.</license-p></license></permissions></article-meta></front></article>')
        lic = X.parse_fulltext(nested)["licence"]
        self.assertTrue(lic["is_cc"])
        self.assertEqual(lic["href"], "https://creativecommons.org/licenses/by/4.0/")
        named = (b"<article><front><article-meta><permissions><license><license-p>Open access under the CC-BY "
                 b"License.</license-p></license></permissions></article-meta></front></article>")
        self.assertEqual(X.parse_fulltext(named)["licence"]["basis"], "licence text names Creative Commons")

    def test_abstract_types(self):
        xml = (b"<article><front><article-meta>"
               b"<abstract abstract-type='teaser'><p>Teaser with 5 participants.</p></abstract>"
               b"<abstract abstract-type='key-points'><sec><title>Findings</title><p>In 40 adults.</p></sec></abstract>"
               b"<abstract><sec><title>Methods</title><p>We surveyed 40 adults.</p></sec></abstract>"
               b"</article-meta></front></article>")
        sections = [p["section"] for p in X.parse_fulltext(xml)["paragraphs"]]
        self.assertEqual(sections, ["[key-points] > Findings", "Methods"])


class PopulationTests(unittest.TestCase):
    def test_explicit_terms_with_matched_term(self):
        r = X.match_terms({"title": "Attitudes in a sample of U.S. college students.",
                           "note": "Pemakaian chatbot oleh mahasiswa AS relatif jarang."}, X.POPULATION_TERMS)
        self.assertEqual(r["labels"], ["college or university students"])
        self.assertIn({"field": "title", "term": "college students", "label": "college or university students"},
                      r["matches"])
        c = X.match_terms({"title": "Attitudes in a sample of U.S. college students.",
                           "note": "mahasiswa AS"}, X.COUNTRY_TERMS)
        self.assertEqual(c["labels"], ["United States"])
        self.assertEqual({m["term"] for m in c["matches"]}, {"U.S.", "AS"})

    def test_adolescents_indonesia_and_no_term(self):
        p = X.match_terms({"title": "Help-seeking among Indonesian adolescents", "note": None}, X.POPULATION_TERMS)
        self.assertEqual(p["labels"], ["adolescents"])
        c = X.match_terms({"title": "Help-seeking among Indonesian adolescents", "note": None}, X.COUNTRY_TERMS)
        self.assertEqual(c["labels"], ["Indonesia"])
        self.assertIsNone(X.match_terms({"title": "Large language models and summaries", "note": ""},
                                        X.POPULATION_TERMS))
        sa = X.match_terms({"title": "Clinics in South Africa", "note": ""}, X.COUNTRY_TERMS)
        self.assertEqual(sa["labels"], ["South Africa"])
        sa_id = X.match_terms({"title": "", "note": "Studi di Indonesia dan Afrika Selatan."}, X.COUNTRY_TERMS)
        self.assertEqual(sa_id["labels"], ["Indonesia", "South Africa"])
        # 'as' in English prose is not the Indonesian abbreviation for the United States
        self.assertIsNone(X.match_terms({"title": "Chatbots as companions", "note": ""}, X.COUNTRY_TERMS))

    def test_negated_or_transfer_note_clauses_are_skipped(self):
        c = X.match_country("Regulation of apps in South Africa",
                            "Analisis regulasi Afrika Selatan; bukan panduan hukum Indonesia.")
        self.assertEqual(c["labels"], ["South Africa"])
        self.assertEqual([(s["term"], s["reason"]) for s in c["skipped"]],
                         [("Indonesia", "note clause has negation marker 'bukan'")])
        self.assertEqual(c["skipped"][0]["clause"], "bukan panduan hukum Indonesia")
        p = X.match_population("Mental health among adolescents with HIV",
                               "Studi remaja; relevansi target dewasa bersifat tidak langsung.")
        self.assertEqual(p["labels"], ["adolescents"])
        self.assertEqual([s["term"] for s in p["skipped"]], ["dewasa"])
        # transfer and Sela-context clauses without a negation word are skipped too
        for note, kind in (("Populasinya berbeda dari populasi Indonesia.", "transfer_or_difference"),
                           ("Hasil ini berlaku untuk mahasiswa Indonesia.", "transfer_or_difference"),
                           ("Konteks awal Sela adalah Indonesia.", "sela_context"),
                           ("Jika diterapkan di Indonesia, perlu uji lokal.", "transfer_or_difference"),
                           ("Seandainya diterapkan di Indonesia.", "conditional")):
            got = X.match_country("", note)
            self.assertEqual(got["labels"], [], note)
            self.assertIn(f"has {kind} marker", got["skipped"][0]["reason"], note)
        # an affirmative clause in the same note still counts; a contrastive conjunction starts a new clause
        r = X.match_country("", "Pilot RCT Indonesia meningkatkan literasi tetapi tidak mengungguli kontrol di India.")
        self.assertEqual(r["labels"], ["Indonesia"])
        self.assertEqual([s["term"] for s in r["skipped"]], ["India"])
        only_skipped = X.match_country("", "bukan Indonesia")
        self.assertEqual(only_skipped["labels"], [])  # record field becomes null, the skip stays visible

    def test_compounds_are_not_labels(self):
        p = X.match_population("Comparing Physician- and Model-Generated Responses", "relasi dokter-pasien")
        self.assertEqual(p["labels"], [])
        self.assertEqual(sorted(s["term"] for s in p["skipped"]), ["Physician", "pasien"])
        self.assertEqual(X.match_population("Patient-reported outcomes in college students", "")["labels"],
                         ["college or university students"])
        c = X.match_country("Non-English Records and English Translations", "")
        self.assertEqual(c["labels"], ["English (language)"])
        self.assertEqual([s["reason"] for s in c["skipped"]], ["negated by prefix 'non-'"])
        self.assertEqual(X.match_country("Support for Chinese-Speaking students", "")["labels"],
                         ["Chinese (language/nationality)"])  # language modifiers stay labels

    def test_real_register_regressions(self):
        """Reviewer-reported records, using the real register titles and annotations."""
        papers = {p["id"]: p for p in P.load_papers(P.Inputs(REPO))}

        def labels(rid):
            p = papers[rid]
            title = " ".join(p["title"].split())
            return ((X.match_country(title, p["abstract_based_note_id"]) or {}).get("labels") or None,
                    (X.match_population(title, p["abstract_based_note_id"]) or {}).get("labels") or None)

        self.assertEqual(labels("R041")[0], ["South Africa"])
        self.assertEqual(labels("R044")[0], ["India"])
        self.assertEqual(labels("R057")[0], ["English (language)", "Korean (language/nationality)"])
        self.assertEqual(labels("R054"), (["Indonesia"], ["adolescents"]))
        self.assertEqual(labels("R011")[1], ["international students"])
        self.assertIsNone(labels("R065")[1])
        indonesian = sorted(rid for rid in papers if "Indonesia" in (labels(rid)[0] or []))
        self.assertEqual(indonesian, ["R047", "R048", "R049", "R050", "R051", "R052", "R053", "R054", "R098"])

    def test_population_flag_follows_filtered_labels(self):
        def flags(country):
            ctx = {"register_review_status": "title_abstract_screened", "study_type": {"labels": []},
                   "country": country, "population": None, "discrepancies": [],
                   "retraction": {"status": "none"}, "doi_resolvable": True, "epmc": {},
                   "licence": {"id": "CC-BY-4.0", "redistribution_allowed": True, "sources_disagree": False},
                   "fulltext_processed": False, "sample": None, "pmid_mismatch": False, "within": True}
            return {f["id"] for f in R.assign_flags(ctx)}
        note = "Review aplikasi pasar India; hasil satu negara tidak otomatis berlaku untuk katalog Indonesia."
        self.assertIn("population_not_indonesian", flags(X.match_country("Apps for Indian Users", note)))
        self.assertIn("population_country_not_stated", flags(X.match_country("", "bukan Indonesia")))
        self.assertNotIn("population_not_indonesian", flags(X.match_country("Studi di Indonesia", "")))


class ExportAndVerifyTests(BuildCase):
    def test_build_statuses_and_schema(self):
        m = self.build()
        recs = self.records()
        self.assertEqual(m["counts"]["screening_status"], {"excluded_out_of_window": 1, "excluded_retracted": 1,
                                                           "included_title_abstract_v0": 1, "needs_check": 2})
        self.assertEqual(recs["R001"]["screening_status"], "included_title_abstract_v0")
        self.assertEqual(recs["R001"]["sample_size"], 312)
        self.assertEqual(recs["R001"]["license"], "CC-BY-4.0")
        self.assertEqual(recs["R001"]["access_status"], "open_access_licensed")
        self.assertEqual(recs["R001"]["extraction_source"], "europepmc_oa_fulltext_xml")
        self.assertIsNotNone(recs["R001"]["limitations"]["snippet"])  # CC BY: snippet may be redistributed
        self.assertEqual(recs["R002"]["screening_status"], "excluded_retracted")
        self.assertEqual(recs["R002"]["doi"], "10.1177/1758835920922055")
        self.assertIn("eligible but no cached", recs["R002"]["fulltext_check"]["skipped"])
        self.assertEqual(recs["R003"]["screening_status"], "excluded_out_of_window")
        self.assertFalse(recs["R003"]["within_cutoff"])
        self.assertEqual(recs["R004"]["screening_reasons"], ["no_primary_day_precision_date"])
        self.assertIn("doi_not_resolvable", recs["R005"]["screening_reasons"])
        self.assertFalse(recs["R005"]["doi_resolvable"])
        for r in recs.values():
            self.assertEqual(r["reviewer_status"], "not_reviewed")
            self.assertEqual(V.validate_record(r, FREEZE, m["cutoff_date"]), [])
        self.assertEqual(m["duplicates"]["count"], 0)
        y = m["year_distribution_check"]
        self.assertTrue(y["vs_json_publication_year"]["matches_claim"])
        self.assertFalse(y["vs_verified_canonical_dates"]["matches_claim"])
        violations, _ = V.verify_outputs(self.tmp / "out")
        self.assertEqual(violations, [])

    def test_deterministic_export(self):
        self.build("a")
        self.build("b")
        files_a = sorted(p.relative_to(self.tmp / "a") for p in (self.tmp / "a").rglob("*") if p.is_file())
        files_b = sorted(p.relative_to(self.tmp / "b") for p in (self.tmp / "b").rglob("*") if p.is_file())
        self.assertEqual(files_a, files_b)
        for rel in files_a:
            self.assertEqual((self.tmp / "a" / rel).read_bytes(), (self.tmp / "b" / rel).read_bytes(), str(rel))
        raw = (self.tmp / "a" / "records.jsonl").read_bytes()
        self.assertTrue(raw.endswith(b"\n"))
        first = json.loads(raw.splitlines()[0])
        self.assertEqual(list(first), sorted(first))
        m = json.loads((self.tmp / "a" / "manifest.json").read_text(encoding="utf-8"))
        kb = [json.loads(x) for x in (self.tmp / "a" / "knowledge_base.jsonl").read_text().splitlines()]
        self.assertEqual(m["index_version"], E.index_version_of(kb))
        self.assertEqual(len(m["index_version"]), 16)
        self.assertEqual(set(kb[0]), set(E.KB_FIELDS))

    def test_snapshot_immutability(self):
        self.build()
        snap = self.tmp / "out" / "snapshots" / FREEZE / "records.jsonl"
        before = snap.read_bytes()
        self.build()  # identical content: allowed
        changed = self.tmp / "changed"
        shutil.copytree(MINI, changed)
        reg = changed / "docs/research/sela-relay/research_100.json"
        obj = json.loads(reg.read_text(encoding="utf-8"))
        obj["papers"][0]["theme"] = "NAVIGATION"
        reg.write_text(json.dumps(obj, ensure_ascii=False, indent=2), encoding="utf-8")
        records_before = (self.tmp / "out" / "records.jsonl").read_bytes()
        with self.assertRaises(E.SnapshotConflict):
            self.build(inputs=P.Inputs(changed))
        self.assertEqual(snap.read_bytes(), before)
        self.assertEqual((self.tmp / "out" / "records.jsonl").read_bytes(), records_before)
        self.build(inputs=P.Inputs(changed), freeze="2026-10-27")  # a new freeze date is a new snapshot
        self.assertTrue((self.tmp / "out" / "snapshots" / "2026-10-27" / "manifest.json").exists())

    def test_rights_redaction_in_committed_outputs(self):
        self.build()
        recs = self.records()
        rec = dict(recs["R001"])
        rec["redistribution_allowed"] = False
        self.assertTrue(any("snippet" in e for e in V.validate_record(rec)))
        public = E.public_record(dict(recs["R001"], redistribution_allowed=False))
        self.assertTrue(all(e["snippet"] is None and e["snippet_withheld"] for e in public["evidence"]))
        self.assertIsNone(public["limitations"]["snippet"])

    def test_schema_violations_and_tamper_detection(self):
        self.build()
        recs = self.records()
        bad = dict(recs["R001"])
        del bad["license"]
        self.assertTrue(V.validate_record(bad))
        self.assertTrue(V.validate_record(dict(recs["R001"], reviewer_status="reviewed")))
        self.assertTrue(V.validate_record(dict(recs["R001"], within_cutoff=False), FREEZE, "2021-10-26"))
        self.assertTrue(V.validate_record(dict(recs["R003"], screening_status="included_title_abstract_v0")))
        self.assertTrue(V.validate_record(dict(recs["R001"], title="x" * 2001)))
        path = self.tmp / "out" / "knowledge_base.jsonl"
        path.write_bytes(path.read_bytes().replace(b"AI_SUPPORT", b"AI_SUPPORX"))
        violations, _ = V.verify_outputs(self.tmp / "out")
        self.assertTrue(any("sha256" in v for v in violations))
        (self.tmp / "out" / "leak.xml").write_text("<article/>")
        violations, _ = V.verify_outputs(self.tmp / "out")
        self.assertTrue(any("leak.xml" in v for v in violations))

    def test_missing_cache_fails_build(self):
        shutil.rmtree(self.cache_dir / "crossref")
        with self.assertRaises(P.MissingCache):
            self.build()
        self.assertEqual(P.run_fetch(self.inputs, self.cache_dir, FREEZE, offline=True, log=lambda m: None), 1)


class EvidenceMapTests(unittest.TestCase):
    def test_rid_validation_and_ranges(self):
        doc07 = ("| Bukti pada register | Temuan | Keputusan Sela |\n|---|---|---|\n"
                 "| R001; R999 | x | Fitur A |\n| R002–R004 | y | Fitur B |\n")
        doc10 = ("| Feature or claim | Registry starting points |\n| --- | --- |\n| Gate | R003 |\n"
                 "**First detailed appraisal batch:** R001 and R004–R005. More text.\n")
        emap = EV.build_evidence_map("07", doc07, "10", doc10, ["R001", "R002", "R003", "R004"],
                                     {"R001": "included_title_abstract_v0", "R003": "needs_check"})
        self.assertEqual(emap["missing_r_ids"], ["R999"])
        self.assertFalse(emap["all_r_ids_present"])
        self.assertEqual(emap["entries"][1]["r_ids"], ["R002", "R003", "R004"])
        self.assertEqual(emap["entries"][2]["relies_on_non_included"], ["R003"])
        self.assertEqual(emap["first_appraisal_batch"]["r_ids"], ["R001", "R004", "R005"])
        self.assertEqual(emap["first_appraisal_batch"]["missing_r_ids"], ["R005"])
        self.assertEqual(emap["features_by_r_id"]["R003"], ["Fitur B", "Gate"])

    def test_real_docs_reference_only_existing_rids(self):
        inputs = P.Inputs(REPO)
        rids = [p["id"] for p in P.load_papers(inputs)]
        emap = EV.build_evidence_map("07", inputs.doc07.read_text(encoding="utf-8"),
                                     "10", inputs.doc10.read_text(encoding="utf-8"), rids, {})
        self.assertGreater(len(emap["entries"]), 10)
        self.assertEqual(emap["missing_r_ids"], [])

    def test_year_claim_parse(self):
        counts, raw = EV.parse_year_claim("| First-publication year | 2023: 2; 2024: 3; 2025: 86; 2026: 9 |")
        self.assertEqual(counts, {"2023": 2, "2024": 3, "2025": 86, "2026": 9})
        cmp = EV.compare_distribution(counts, {"2023": 2, "2024": 3, "2025": 94, "2026": 1})
        self.assertFalse(cmp["matches_claim"])
        self.assertEqual(cmp["difference_observed_minus_claimed"], {"2025": 8, "2026": -8})


class ClaimGateTests(unittest.TestCase):
    def test_patterns_compile_and_examples(self):
        doc = C.export_document()
        self.assertEqual(V.check_claim_gates(json.loads(json.dumps(doc))), [])
        ids_ = [g["id"] for g in doc["gates"]]
        self.assertEqual(len(ids_), len(set(ids_)))
        for required in ("clinical_validation", "efficacy", "reduces_symptoms", "suicide_prevention",
                         "safe_or_certified", "diagnosis"):
            self.assertIn(required, ids_)
        for gate in doc["gates"]:
            self.assertTrue(gate["sources"])
            for ex in gate["forbidden_examples"]:
                self.assertIn(gate["id"], {h["gate"] for h in C.scan_text(ex)}, ex)
            for ex in gate["allowed_examples"]:
                self.assertEqual(C.scan_text(ex), [], ex)

    def test_bilingual_and_case_insensitive(self):
        self.assertTrue(C.scan_text("SELA TERBUKTI SECARA KLINIS"))
        self.assertTrue(C.scan_text("Sela Reduces Depression"))
        self.assertEqual(C.scan_text("Sela bukan terapis, tidak mendiagnosis, dan tidak menilai risiko."), [])

    # Reviewer-reported false positives: disclaimers, including real product copy.
    DISCLAIMERS = (
        "A sent request does not mean you have received care.",
        "Sent or acknowledged does not mean you have received care. All of this is simulated.",
        "Permintaan yang terkirim belum berarti kamu sudah mendapat perawatan.",
        "Terkirim atau diterima belum berarti kamu sudah mendapat perawatan. Semua ini simulasi.",
        "Sela is a bridge to professional support, not an AI therapist.",
        "Sela is not a diagnostic tool.",
        "Sela is not a substitute for a therapist.",
        "Sela doesn't replace your psychologist.",
        "Sela doesn’t replace your psychologist.",
        "Sela cannot assess your suicide risk.",
        "We make no claim that Sela is clinically validated.",
        "Kami tidak mengklaim Sela terbukti efektif.",
        "Sela does not diagnose, treat or prescribe medication.",
        "Sela is a non-diagnostic tool.",
    )
    # Reviewer-reported false negatives plus scope cases that must still hit.
    CLAIMS = (
        ("Sela adalah terapis", "therapy_or_counselling"),
        ("Sela adalah psikolog AI", "therapy_or_counselling"),
        ("Sela membantu mengatasi depresi", "reduces_symptoms"),
        ("Sela is an evidence-based therapist", "therapy_or_counselling"),
        ("Sela is an evidence-based therapist", "evidence_count_overclaim"),
        ("Sela is not a chatbot but an AI therapist.", "therapy_or_counselling"),       # scope breaker
        ("Sela bukan aplikasi biasa, melainkan terapis AI.", "therapy_or_counselling"),
        ("No waiting list. Sela provides therapy.", "therapy_or_counselling"),            # clause boundary
        ("Don't wait — get your diagnosis today.", "diagnosis"),
        ("Don't wait - get your diagnosis today.", "diagnosis"),
        ("No sign-up needed and the app also provides therapy.", "therapy_or_counselling"),  # cue > 6 tokens back
        ("Sela is 'clinically validated'.", "clinical_validation"),                       # quoted claim
    )

    def test_disclaimers_pass_and_affirmative_claims_hit(self):
        for text in self.DISCLAIMERS:
            self.assertEqual(C.scan_text(text), [], text)
        for text, gate in self.CLAIMS:
            self.assertIn(gate, {h["gate"] for h in C.scan_text(text)}, text)
        self.assertEqual(C.scan_text("Sela adalah jembatan menuju psikolog."), [])   # navigation, not a claim
        self.assertEqual(C.scan_text("Share this note with your therapist."), [])

    def test_apostrophe_cannot_split_a_negation(self):
        # 'doesn' + 't' would evade the cue if tokens could end inside a word
        self.assertEqual(C.scan_text("Sela doesn't provide therapy."), [])
        self.assertEqual(C.scan_text("It isn’t a diagnostic tool."), [])

    def test_hit_reports_claim_span_and_negated_mentions(self):
        hits = C.scan_text("Intro text. Sela provides therapy.")
        self.assertEqual([(h["match"], h["start"]) for h in hits], [("provides therapy", 17)])
        neg = C.scan_text("Sela does not provide therapy.", include_negated=True)
        self.assertEqual([(h["gate"], h["match"], h["negated"]) for h in neg],
                         [("therapy_or_counselling", "provide therapy", True)])

    def test_patterns_are_portable_and_linear(self):
        doc = C.export_document()
        for gate in doc["gates"]:
            self.assertEqual(len(gate["patterns"]), len(gate["claim_patterns"]))
            for p in gate["patterns"]:
                for python_only in ("(?P", "(?#", "(?>", "(?i", "\\A", "\\Z", "\\w", "\\W"):
                    self.assertNotIn(python_only, p)  # Python-only syntax, or \\w (Unicode in Python, ASCII in JS)
                self.assertTrue(p.startswith(C.NEGATION_PREFIX))
        start = time.perf_counter()
        for line in ("| --- | --- |" * 300, "-" * 4000, "a-" * 1500 + " provides therapy"):
            C.scan_text(line)
        self.assertLess(time.perf_counter() - start, 10.0)  # no exponential backtracking on separator runs

    def test_scan_claims_cli_counts_only_hits(self):
        with tempfile.TemporaryDirectory() as td:
            copy = Path(td) / "copy.txt"
            copy.write_text("Sela is not an AI therapist.\nKamu sudah mendapat perawatan.\n", encoding="utf-8")
            out = io.StringIO()
            with redirect_stdout(out):
                code = P.main(["scan-claims", "--show-negated", str(copy)])
            self.assertEqual(code, 1)
            self.assertIn("scan-claims: 1 hit(s); 1 negated mention(s) not counted", out.getvalue())

    @unittest.skipUnless((REPO / "web" / "src" / "i18n" / "en.ts").exists(), "web/ i18n copy not present")
    def test_product_copy_has_no_forbidden_claims(self):
        """The real UI copy (web/src/i18n/*.ts) must give 0 hits, line by line as scan-claims reads it."""
        problems = []
        for path in sorted((REPO / "web" / "src" / "i18n").glob("*.ts")):
            for lineno, line in enumerate(path.read_text(encoding="utf-8").splitlines(), start=1):
                problems.extend(f"{path.name}:{lineno} [{h['gate']}] {line.strip()}" for h in C.scan_text(line))
        self.assertEqual(problems, [])


class FetchClientTests(unittest.TestCase):
    class FakeResponse(io.BytesIO):
        status = 200

        def __enter__(self):
            return self

        def __exit__(self, *a):
            return False

    def test_retry_after_404_and_contact_never_cached(self):
        seen, sleeps = [], []
        responses = [HTTPError("u", 429, "slow down", {"Retry-After": "3"}, None),
                     self.FakeResponse(b'{"message": {"DOI": "10.1/x"}}')]

        def opener(req, timeout):
            seen.append(req.full_url)
            r = responses.pop(0)
            if isinstance(r, Exception):
                raise r
            return r

        client = F.Client(opener=opener, sleep=sleeps.append, mailto="someone@example.org", log=lambda m: None)
        with tempfile.TemporaryDirectory() as td:
            cache = F.Cache(td)
            key, url = F.crossref_request("10.1/x")
            env, fetched = F.fetch_json(client, cache, "crossref", key, url, extra={"pool": "polite"})
            self.assertTrue(fetched)
            self.assertEqual(env["status"], "ok")
            self.assertIn(3.0, sleeps)
            self.assertTrue(all("mailto=someone%40example.org" in u for u in seen))
            stored = "".join(p.read_text() for p in Path(td).rglob("*.json"))
            self.assertNotIn("someone", stored)
            self.assertNotIn("mailto", stored)
            again, fetched_again = F.fetch_json(client, cache, "crossref", key, url)
            self.assertFalse(fetched_again)

        def not_found(req, timeout):
            raise HTTPError("u", 404, "nf", {}, io.BytesIO(b'{"responseCode": 100}'))
        client = F.Client(opener=not_found, sleep=lambda s: None, log=lambda m: None)
        with tempfile.TemporaryDirectory() as td:
            key, url = F.doi_handle_request("10.5555/none")
            env, _ = F.fetch_json(client, F.Cache(td), "doi_handle", key, url)
            self.assertEqual((env["status"], env["body"]), ("not_found", {"responseCode": 100}))

    def test_transient_error_is_not_cached(self):
        def boom(req, timeout):
            raise HTTPError("u", 503, "unavailable", {}, None)
        client = F.Client(opener=boom, sleep=lambda s: None, attempts=2, log=lambda m: None)
        with tempfile.TemporaryDirectory() as td:
            key, url = F.openalex_request("10.1/x")
            env, _ = F.fetch_json(client, F.Cache(td), "openalex", key, url)
            self.assertIn("fetch_error", env)
            self.assertEqual(list(Path(td).rglob("*.json")), [])

    def test_abstracts_trimmed_before_caching(self):
        body = {"resultList": {"result": [{"id": "1", "abstractText": "secret"}]}}
        self.assertNotIn("abstractText", F.trim("europepmc", body)["resultList"]["result"][0])
        self.assertNotIn("abstract", F.trim("crossref", {"message": {"abstract": "x", "DOI": "d"}})["message"])


class RepositoryHygieneTests(unittest.TestCase):
    def test_cache_is_gitignored(self):
        text = (REPO / ".gitignore").read_text(encoding="utf-8")
        self.assertIn("/data/research/corpus/.cache/", text)


if __name__ == "__main__":
    unittest.main()
