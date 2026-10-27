"""Network fetchers behind a local cache (the only module that does I/O to APIs).

Etiquette (checked 2026-10-26):
- Sequential requests only, one service at a time, fixed minimum interval per
  service, retries with exponential backoff that honours Retry-After on
  429/5xx, descriptive User-Agent.
- Crossref public pool: rate 5 req/s, concurrency 1; polite pool via the
  ``mailto`` parameter (rate 10, concurrency 3).
  https://crossref.org/documentation/retrieve-metadata/rest-api/access-and-authentication/
  The contact address is read ONLY from env SELA_RESEARCH_MAILTO, sent only to
  Crossref, and never written to the cache, logs or outputs. Without it we use
  the public pool at <= 2 req/s.
- OpenAlex: singleton /works/doi:{doi} lookups are free and unlimited; list
  and filter queries spend the keyless daily budget and are NOT used.
  https://help.openalex.org/access/example-costs/ ,
  https://help.openalex.org/how-to-use-the-api/rate-limits-and-authentication
- DOI resolution check: https://doi.org/api/handles/{doi} (responseCode 1 =
  found, 100 = handle not found).
  https://www.doi.org/the-identifier/resources/factsheets/doi-resolution-documentation
- Europe PMC REST search (resultType=core) and
  https://www.ebi.ac.uk/europepmc/webservices/rest/{PMCID}/fullTextXML for
  open-access full text. Documented at https://europepmc.org/RestfulWebService
  (that page returned a Cloudflare 403 to this container; the endpoint
  behaviour was verified directly: HTTP 200 application/xml for an OA PMCID).
- v3 (pipeline 1.2.0), only for records whose screening is needs_check:
  * NCBI E-utilities efetch (db=pubmed, retmode=xml) for the public PubMed
    date history; only dates, publication model and language are cached
    (the abstract is dropped while parsing). <= 3 requests/s without an API
    key: https://www.ncbi.nlm.nih.gov/books/NBK25497/
  * the publisher landing page reached through https://doi.org/{doi}; only the
    citation_*/dc/prism date meta tags and a tdm-reservation tag are cached,
    never the page. A challenge page (HTTP 403/503) is recorded as blocked and
    is NOT retried or bypassed; an HTML meta-refresh is never followed.
"""
from __future__ import annotations

import datetime as dt
import hashlib
import html.parser
import json
import os
import re
import sys
import time
import xml.etree.ElementTree as ET
from pathlib import Path
from urllib.error import HTTPError, URLError
from urllib.parse import quote, urlencode
from urllib.request import Request, urlopen

from . import PIPELINE_VERSION, USER_AGENT

EPMC_SEARCH = "https://www.ebi.ac.uk/europepmc/webservices/rest/search"
EPMC_FULLTEXT = "https://www.ebi.ac.uk/europepmc/webservices/rest/{pmcid}/fullTextXML"
CROSSREF_WORK = "https://api.crossref.org/works/{doi}"
OPENALEX_WORK = "https://api.openalex.org/works/doi:{doi}"
DOI_HANDLE = "https://doi.org/api/handles/{doi}"
PUBMED_EFETCH = "https://eutils.ncbi.nlm.nih.gov/entrez/eutils/efetch.fcgi"
LANDING = "https://doi.org/{doi}"

ENDPOINTS = {
    "europepmc": EPMC_SEARCH,
    "crossref": CROSSREF_WORK,
    "openalex": OPENALEX_WORK,
    "doi_handle": DOI_HANDLE,
    "fulltext": EPMC_FULLTEXT,
    "pubmed_history": PUBMED_EFETCH,
    "landing_meta": LANDING + " (publisher landing page, date meta tags only)",
    "crossref_search": "https://api.crossref.org/works (query + filter search)",
    "arxiv": "https://export.arxiv.org/api/query (id_list lookup)",
}
MIN_INTERVAL = {"europepmc": 0.5, "crossref": 0.5, "openalex": 0.5, "doi_handle": 0.5, "fulltext": 1.0,
                "pubmed_history": 0.4, "landing_meta": 2.0, "crossref_search": 1.0, "arxiv": 3.0}
RETRY_STATUS = {429, 500, 502, 503, 504}
# Fields dropped before caching (data minimisation: abstracts and reference
# lists are not needed by any pipeline step).
DROP_FIELDS = {
    "europepmc": {"abstractText"},
    "crossref": {"abstract", "reference"},
    "openalex": {"abstract_inverted_index"},
}
_EMAIL = re.compile(r"^[^@\s]+@[^@\s]+\.[^@\s]+$")


def utc_now():
    return dt.datetime.now(dt.timezone.utc).replace(microsecond=0).isoformat().replace("+00:00", "Z")


def safe_name(key):
    slug = re.sub(r"[^a-z0-9._-]+", "_", key.lower()).strip("_")[:80]
    return f"{slug}__{hashlib.sha256(key.encode('utf-8')).hexdigest()[:10]}"


class Cache:
    """JSON envelopes under <dir>/<service>/<name>.json; full text as .xml beside it."""

    def __init__(self, root):
        self.root = Path(root)

    def path(self, service, key, suffix=".json"):
        return self.root / service / (safe_name(key) + suffix)

    def get(self, service, key):
        p = self.path(service, key)
        if not p.exists():
            return None
        return json.loads(p.read_text(encoding="utf-8"))

    def get_text(self, service, key, suffix=".xml"):
        p = self.path(service, key, suffix)
        return p.read_bytes() if p.exists() else None

    def put(self, service, key, envelope, raw_bytes=None, suffix=".xml"):
        p = self.path(service, key)
        p.parent.mkdir(parents=True, exist_ok=True)
        if raw_bytes is not None:
            rp = self.path(service, key, suffix)
            tmp = rp.with_name(rp.name + ".tmp")
            tmp.write_bytes(raw_bytes)
            os.replace(tmp, rp)
        tmp = p.with_name(p.name + ".tmp")
        tmp.write_text(json.dumps(envelope, ensure_ascii=False, sort_keys=True, indent=1) + "\n", encoding="utf-8")
        os.replace(tmp, p)

    def entries(self):
        for p in sorted(self.root.glob("*/*.json")):
            yield p, json.loads(p.read_text(encoding="utf-8"))


def envelope(service, key, public_url, fetched_at, http_status, status, body=None, error=None, extra=None):
    env = {
        "service": service, "key": key, "endpoint": ENDPOINTS[service], "url": public_url,
        "fetched_at": fetched_at, "http_status": http_status, "status": status,
        "pipeline_version": PIPELINE_VERSION, "error": error, "body": body,
    }
    if extra:
        env.update(extra)
    return env


def trim(service, body):
    if isinstance(body, dict):
        drop = DROP_FIELDS.get(service, set())
        if service == "europepmc":
            results = (body.get("resultList") or {}).get("result") or []
            for r in results:
                for f in drop:
                    r.pop(f, None)
            return body
        if service == "crossref" and isinstance(body.get("message"), dict):
            for f in drop:
                body["message"].pop(f, None)
            return body
        for f in drop:
            body.pop(f, None)
    return body


class Client:
    """Sequential HTTP client with per-service pacing and bounded retries."""

    def __init__(self, timeout=30.0, attempts=4, sleep=time.sleep, opener=urlopen, mailto=None, log=None):
        self.timeout = timeout
        self.attempts = attempts
        self.sleep = sleep
        self.opener = opener
        self.mailto = mailto if (mailto and _EMAIL.match(mailto)) else None
        self.last = {}
        self.log = log or (lambda msg: print(msg, file=sys.stderr))
        self.requests = 0
        self.last_url = None

    def _pace(self, service):
        interval = MIN_INTERVAL[service]
        if service in ("crossref", "crossref_search") and self.mailto is None:
            interval = max(interval, 0.5)  # public pool: <= 2 req/s, well under its 5 req/s limit
        wait = self.last.get(service, 0) + interval - time.monotonic()
        if wait > 0:
            self.sleep(wait)
        self.last[service] = time.monotonic()

    def _redact(self, text):
        if self.mailto:
            text = text.replace(self.mailto, "<redacted>").replace(quote(self.mailto, safe=""), "<redacted>")
        return text

    def get(self, service, url, accept="application/json", attempts=None):
        """Return (http_status, body_bytes, error). 404 is a definitive answer, not an error."""
        attempts = attempts or self.attempts
        request_url = url
        if service in ("crossref", "crossref_search") and self.mailto:
            request_url = url + ("&" if "?" in url else "?") + urlencode({"mailto": self.mailto})
        req = Request(request_url, headers={"User-Agent": USER_AGENT, "Accept": accept})
        for attempt in range(attempts):
            self._pace(service)
            self.requests += 1
            try:
                with self.opener(req, timeout=self.timeout) as resp:
                    self.last_url = resp.geturl() if hasattr(resp, "geturl") else request_url
                    return resp.status, resp.read(), None
            except HTTPError as e:
                if e.code == 404:
                    try:
                        body = e.read()
                    except Exception:  # noqa: BLE001 - body is optional diagnostics
                        body = b""
                    return 404, body, None
                if e.code not in RETRY_STATUS or attempt == attempts - 1:
                    return e.code, b"", self._redact(f"HTTP {e.code}")
                retry_after = (e.headers or {}).get("Retry-After", "") if e.headers else ""
                try:
                    delay = min(60.0, max(1.0, float(retry_after)))
                except ValueError:
                    delay = float(2 ** attempt)
                self.log(f"  {service}: HTTP {e.code}, retry in {delay:.0f}s")
                self.sleep(delay)
            except (URLError, TimeoutError, ConnectionError, OSError) as e:
                if attempt == attempts - 1:
                    return None, b"", self._redact(f"{type(e).__name__}: {getattr(e, 'reason', e)}")
                self.log(f"  {service}: {type(e).__name__}, retry in {2 ** attempt}s")
                self.sleep(float(2 ** attempt))
        return None, b"", "exhausted retries"


# --- request builders (public URLs never contain the contact address) -----------

def epmc_doi_request(doi):
    query = f'DOI:"{doi}"'
    url = EPMC_SEARCH + "?" + urlencode({"query": query, "format": "json", "resultType": "core", "pageSize": 25})
    return "doi:" + doi, url


def epmc_pmid_request(pmid):
    query = f"EXT_ID:{pmid} AND SRC:MED"
    url = EPMC_SEARCH + "?" + urlencode({"query": query, "format": "json", "resultType": "core", "pageSize": 25})
    return "pmid:" + pmid, url


def crossref_request(doi):
    return doi, CROSSREF_WORK.format(doi=quote(doi, safe=""))


def openalex_request(doi):
    return doi, OPENALEX_WORK.format(doi=quote(doi, safe="/"))


def doi_handle_request(doi):
    return doi, DOI_HANDLE.format(doi=quote(doi, safe="/")) + "?" + urlencode({"type": "URL"})


def fulltext_request(pmcid):
    return pmcid, EPMC_FULLTEXT.format(pmcid=pmcid)


def pubmed_history_request(pmid):
    return "pmid:" + pmid, PUBMED_EFETCH + "?" + urlencode({"db": "pubmed", "id": pmid, "retmode": "xml",
                                                              "tool": "sela-corpus-pipeline"})


def landing_request(doi):
    return doi, LANDING.format(doi=quote(doi, safe="/"))


def _pm_date(el):
    if el is None:
        return None
    y, m, d = el.findtext("Year"), el.findtext("Month"), el.findtext("Day")
    if not y:
        return el.findtext("MedlineDate")
    months = {n: i for i, n in enumerate(("jan", "feb", "mar", "apr", "may", "jun", "jul", "aug", "sep", "oct",
                                           "nov", "dec"), start=1)}
    if m and not m.isdigit():
        m = str(months.get(m[:3].lower(), "")) or None
    parts = [y] + ([f"{int(m):02d}"] if m else []) + ([f"{int(d):02d}"] if m and d else [])
    return "-".join(parts)


def parse_pubmed_history(xml_bytes, pmid):
    """Dates only (abstract and author data are dropped here and never cached)."""
    root = ET.fromstring(xml_bytes)
    art = next((a for a in root.findall("PubmedArticle") if a.findtext("MedlineCitation/PMID") == pmid), None)
    if art is None:
        return None
    article = art.find("MedlineCitation/Article")
    return {
        "pmid": pmid,
        "article_dates": [{"type": d.get("DateType"), "date": _pm_date(d)}
                          for d in (article.findall("ArticleDate") if article is not None else [])],
        "journal_issue_date": _pm_date(article.find("Journal/JournalIssue/PubDate")) if article is not None else None,
        "history": [{"status": d.get("PubStatus"), "date": _pm_date(d)}
                    for d in art.findall("PubmedData/History/PubMedPubDate")],
        "pub_model": article.get("PubModel") if article is not None else None,
        "languages": [x.text for x in article.findall("Language")] if article is not None else [],
    }


def fetch_pubmed_history(client, cache, pmid, refresh=False):
    key, url = pubmed_history_request(pmid)
    cached = cache.get("pubmed_history", key)
    if cached is not None and not refresh:
        return cached, False
    status, raw, error = client.get("pubmed_history", url, accept="application/xml")
    fetched_at = utc_now()
    if status == 200:
        try:
            body = parse_pubmed_history(raw, pmid)
        except ET.ParseError as exc:
            return {"fetch_error": f"invalid XML: {exc}"}, True
        env = envelope("pubmed_history", key, url, fetched_at, 200, "ok" if body else "not_found", body=body)
    elif status == 404:
        env = envelope("pubmed_history", key, url, fetched_at, 404, "not_found")
    else:
        return {"fetch_error": error or f"HTTP {status}"}, True
    cache.put("pubmed_history", key, env)
    return env, True


LANDING_META_NAMES = {"citation_online_date", "citation_publication_date", "citation_date", "dc.date",
                      "prism.publicationdate", "article:published_time"}


class _MetaParser(html.parser.HTMLParser):
    def __init__(self):
        super().__init__(convert_charrefs=True)
        self.meta, self.tdm = {}, None

    def handle_starttag(self, tag, attrs):
        if tag.lower() != "meta":
            return
        a = {k.lower(): (v or "") for k, v in attrs}
        name = (a.get("name") or a.get("property") or "").strip().lower()
        if name == "tdm-reservation":
            self.tdm = a.get("content", "").strip()
        elif name in LANDING_META_NAMES and a.get("content") and name not in self.meta:
            self.meta[name] = a["content"].strip()[:40]


def parse_landing_meta(html_bytes):
    p = _MetaParser()
    p.feed(html_bytes[:1_000_000].decode("utf-8", errors="replace"))
    return dict(sorted(p.meta.items())), p.tdm


def fetch_landing_meta(client, cache, doi, refresh=False):
    """Date meta tags of the publisher landing page; blocked pages are recorded, never bypassed."""
    key, url = landing_request(doi)
    cached = cache.get("landing_meta", key)
    if cached is not None and not refresh:
        return cached, False
    status, raw, error = client.get("landing_meta", url, accept="text/html", attempts=1)  # a challenge is final
    fetched_at = utc_now()
    final = client.last_url if status == 200 else None
    if status == 200:
        meta, tdm = parse_landing_meta(raw)
        note = None
        if tdm == "1":
            note = "tdm-reservation=1: rights reserved; only this page's own meta tags were read, nothing followed"
        if not meta:
            note = (note + "; " if note else "") + "no date meta tags on the page reached via doi.org"
        env = envelope("landing_meta", key, url, fetched_at, 200, "ok",
                       body={"final_url": final, "meta": meta, "tdm_reservation": tdm, "note": note})
    elif status in (401, 403, 429, 503):
        env = envelope("landing_meta", key, url, fetched_at, status, "blocked",
                       body={"final_url": None, "meta": {}, "tdm_reservation": None,
                             "note": f"HTTP {status} (bot challenge or access control); not bypassed"})
    elif status == 404:
        env = envelope("landing_meta", key, url, fetched_at, 404, "not_found")
    else:
        return {"fetch_error": error or f"HTTP {status}"}, True
    cache.put("landing_meta", key, env)
    return env, True


def fetch_json(client, cache, service, key, url, refresh=False, extra=None):
    """Fetch one JSON resource into the cache.

    Returns (envelope, fetched_now). On a transient failure the envelope is
    ``{"fetch_error": ...}`` and nothing is cached, so a later run retries it.
    """
    cached = cache.get(service, key)
    if cached is not None and not refresh:
        return cached, False
    status, raw, error = client.get(service, url)
    fetched_at = utc_now()
    if status == 200:
        try:
            body = trim(service, json.loads(raw.decode("utf-8")))
        except (ValueError, UnicodeDecodeError) as exc:
            return {"fetch_error": f"invalid JSON: {exc}"}, True
        env = envelope(service, key, url, fetched_at, 200, "ok", body=body, extra=extra)
    elif status == 404:
        body = None
        try:
            body = json.loads(raw.decode("utf-8")) if raw else None
        except (ValueError, UnicodeDecodeError):
            body = None
        env = envelope(service, key, url, fetched_at, 404, "not_found", body=body, extra=extra)
    else:
        return {"fetch_error": error or f"HTTP {status}"}, True
    cache.put(service, key, env)
    return env, True


def fetch_fulltext(client, cache, pmcid, refresh=False):
    key, url = fulltext_request(pmcid)
    cached = cache.get("fulltext", key)
    if cached is not None and not refresh and (cached["status"] != "ok" or cache.get_text("fulltext", key)):
        return cached, False
    status, raw, error = client.get("fulltext", url, accept="application/xml")
    fetched_at = utc_now()
    if status == 200 and raw.lstrip().startswith(b"<"):
        env = envelope("fulltext", key, url, fetched_at, 200, "ok", body=None,
                       extra={"sha256": hashlib.sha256(raw).hexdigest(), "bytes": len(raw)})
        cache.put("fulltext", key, env, raw_bytes=raw)
        return env, True
    if status == 404:
        env = envelope("fulltext", key, url, fetched_at, 404, "not_found")
        cache.put("fulltext", key, env)
        return env, True
    return {"fetch_error": error or f"HTTP {status} (non-XML body)"}, True
