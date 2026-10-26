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
"""
from __future__ import annotations

import datetime as dt
import hashlib
import json
import os
import re
import sys
import time
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

ENDPOINTS = {
    "europepmc": EPMC_SEARCH,
    "crossref": CROSSREF_WORK,
    "openalex": OPENALEX_WORK,
    "doi_handle": DOI_HANDLE,
    "fulltext": EPMC_FULLTEXT,
}
MIN_INTERVAL = {"europepmc": 0.5, "crossref": 0.5, "openalex": 0.5, "doi_handle": 0.5, "fulltext": 1.0}
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

    def _pace(self, service):
        interval = MIN_INTERVAL[service]
        if service == "crossref" and self.mailto is None:
            interval = max(interval, 0.5)  # public pool: <= 2 req/s, well under its 5 req/s limit
        wait = self.last.get(service, 0) + interval - time.monotonic()
        if wait > 0:
            self.sleep(wait)
        self.last[service] = time.monotonic()

    def _redact(self, text):
        if self.mailto:
            text = text.replace(self.mailto, "<redacted>").replace(quote(self.mailto, safe=""), "<redacted>")
        return text

    def get(self, service, url, accept="application/json"):
        """Return (http_status, body_bytes, error). 404 is a definitive answer, not an error."""
        request_url = url
        if service == "crossref" and self.mailto:
            request_url = url + ("&" if "?" in url else "?") + urlencode({"mailto": self.mailto})
        req = Request(request_url, headers={"User-Agent": USER_AGENT, "Accept": accept})
        for attempt in range(self.attempts):
            self._pace(service)
            self.requests += 1
            try:
                with self.opener(req, timeout=self.timeout) as resp:
                    return resp.status, resp.read(), None
            except HTTPError as e:
                if e.code == 404:
                    try:
                        body = e.read()
                    except Exception:  # noqa: BLE001 - body is optional diagnostics
                        body = b""
                    return 404, body, None
                if e.code not in RETRY_STATUS or attempt == self.attempts - 1:
                    return e.code, b"", self._redact(f"HTTP {e.code}")
                retry_after = (e.headers or {}).get("Retry-After", "") if e.headers else ""
                try:
                    delay = min(60.0, max(1.0, float(retry_after)))
                except ValueError:
                    delay = float(2 ** attempt)
                self.log(f"  {service}: HTTP {e.code}, retry in {delay:.0f}s")
                self.sleep(delay)
            except (URLError, TimeoutError, ConnectionError, OSError) as e:
                if attempt == self.attempts - 1:
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
