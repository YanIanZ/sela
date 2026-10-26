"""Step 4 and step 7 date logic: parsing, reconciliation, rolling cutoff."""
from __future__ import annotations

import datetime as dt
import re

# Sources whose DAY-precision date may become the canonical publication date.
PRIMARY_SOURCES = (
    "crossref.published_online",
    "europepmc.first_publication_date",
    "europepmc.electronic_publication_date",
)
# Stored for transparency but never compared (deposit timestamp, not publication).
STORE_ONLY_SOURCES = ("crossref.created",)
DISCREPANCY_DAYS = 31

_PARTIAL = re.compile(r"^(\d{4})(?:-(\d{2})(?:-(\d{2}))?)?$")
_MONTHS = {m: i for i, m in enumerate(
    ["jan", "feb", "mar", "apr", "may", "jun", "jul", "aug", "sep", "oct", "nov", "dec"], start=1)}


def parse_iso(value):
    return dt.date.fromisoformat(value)


def parse_partial(value):
    """'2025', '2025-01' or '2025-01-17' -> (normalised string, precision)."""
    m = _PARTIAL.match(str(value or "").strip()[:10])
    if not m:
        return None, None
    y, mo, d = m.groups()
    try:
        if d:
            dt.date(int(y), int(mo), int(d))
            return f"{y}-{mo}-{d}", "day"
        if mo:
            if not 1 <= int(mo) <= 12:
                return None, None
            return f"{y}-{mo}", "month"
    except ValueError:
        return None, None
    return y, "year"


def from_date_parts(obj):
    """Crossref {'date-parts': [[2025, 1, 17]]} -> (string, precision)."""
    try:
        parts = [p for p in (obj or {}).get("date-parts", [[]])[0] if p is not None]
    except (AttributeError, IndexError, TypeError):
        return None, None
    if not parts:
        return None, None
    try:
        if len(parts) >= 3:
            return dt.date(int(parts[0]), int(parts[1]), int(parts[2])).isoformat(), "day"
        if len(parts) == 2:
            if not 1 <= int(parts[1]) <= 12:
                return None, None
            return f"{int(parts[0]):04d}-{int(parts[1]):02d}", "month"
        return f"{int(parts[0]):04d}", "year"
    except (TypeError, ValueError):
        return None, None


def truncate(value, precision):
    if value is None:
        return None
    return {"day": value[:10], "month": value[:7], "year": value[:4]}.get(precision, value)


def epmc_print_precision(date_of_publication):
    """Precision of Europe PMC journalInfo.dateOfPublication ('2025 Jan 17' / '2025 Jan' / '2025 Jan-Dec')."""
    s = " ".join(str(date_of_publication or "").split())
    if re.match(r"^\d{4} [A-Za-z]{3} \d{1,2}$", s):
        return "day"
    m = re.match(r"^\d{4} ([A-Za-z]{3})$", s)
    if m and m.group(1).lower() in _MONTHS:
        return "month"
    return "year"


def collect_dates(epmc=None, crossref=None, openalex=None, register=None):
    """Return (dates, precision) maps keyed by '<source>.<field>'.

    Values are ISO strings truncated to their real precision. Known padding is
    removed: OpenAlex publication_date is month-padded so only its year is used
    for comparison; Europe PMC print dates are truncated to the precision of
    journalInfo.dateOfPublication.
    """
    dates, precision = {}, {}

    def put(key, value, prec):
        if value:
            dates[key] = truncate(value, prec)
            precision[key] = prec

    if epmc:
        ji = epmc.get("journalInfo") or {}
        print_prec = epmc_print_precision(ji.get("dateOfPublication"))
        print_raw, print_raw_p = parse_partial(ji.get("printPublicationDate"))
        if print_raw:
            put("europepmc.print_publication_date", print_raw, min_precision(print_prec, print_raw_p))
        epub, epub_p = parse_partial(epmc.get("electronicPublicationDate"))
        put("europepmc.electronic_publication_date", epub, epub_p)
        first, first_p = parse_partial(epmc.get("firstPublicationDate"))
        if first and first_p == "day" and not epub and print_raw and first == print_raw[:10] and print_prec != "day":
            # firstPublicationDate copied from a padded print date: not day precision.
            first_p = print_prec
        put("europepmc.first_publication_date", first, first_p)
        pub_year, _ = parse_partial(epmc.get("pubYear"))
        put("europepmc.pub_year", pub_year, "year")
    if crossref:
        for field, key in (("published-online", "crossref.published_online"),
                           ("published-print", "crossref.published_print"),
                           ("issued", "crossref.issued"),
                           ("created", "crossref.created")):
            value, prec = from_date_parts(crossref.get(field))
            put(key, value, prec)
    if openalex:
        oa_date, _ = parse_partial(openalex.get("publication_date"))
        if oa_date:
            dates["openalex.publication_date"] = oa_date
            precision["openalex.publication_date"] = "padded"
        oa_year = openalex.get("publication_year")
        if isinstance(oa_year, int):
            put("openalex.publication_year", f"{oa_year:04d}", "year")
    if register:
        value, prec = parse_partial(register.get("first_publication_date"))
        put("register.first_publication_date", value, prec)
        value, _ = parse_partial(register.get("publication_year"))
        put("register.publication_year", value, "year")
    return dates, precision


_ORDER = {"year": 0, "month": 1, "day": 2}


def min_precision(a, b):
    return a if _ORDER.get(a, -1) <= _ORDER.get(b, -1) else b


def _kind(source):
    if source in PRIMARY_SOURCES:
        return "primary"
    if source.startswith("register."):
        return "register"
    return "secondary"


def reconcile(dates, precision, freeze_date=None, cutoff_date=None):
    """Choose the canonical date and list every discrepancy.

    canonical = earliest DAY-precision date among PRIMARY_SOURCES. Every other
    source is compared with it: |difference| > 31 days, a different year, or
    (for primary sources) month/year-only precision is recorded. A conflict
    (which forces screening to needs_check) means two primary day dates differ
    by more than 31 days or fall on different sides of the window boundary.
    """
    primary_day = {k: dates[k] for k in PRIMARY_SOURCES
                   if k in dates and precision.get(k) == "day"}
    canonical = min(primary_day.values()) if primary_day else None
    canonical_sources = sorted(k for k, v in primary_day.items() if v == canonical)
    discrepancies = []

    def add(kind_, source, type_, days=None):
        discrepancies.append({
            "type": type_, "source": source, "kind": kind_,
            "value": dates.get(source), "precision": precision.get(source),
            "canonical": canonical, "days": days,
        })

    for k in PRIMARY_SOURCES:
        if k in dates and precision.get(k) != "day":
            add("primary", k, "month_precision" if precision.get(k) == "month" else "year_precision")
    if canonical is None and dates:
        add("primary", "canonical", "missing_primary_day_date")
    if canonical is not None:
        c = parse_iso(canonical)
        for k in sorted(dates):
            if k in canonical_sources or k in STORE_ONLY_SOURCES:
                continue
            prec = precision.get(k)
            if prec == "padded":
                continue  # year compared via openalex.publication_year
            if prec == "day":
                diff = (parse_iso(dates[k]) - c).days
                if abs(diff) > DISCREPANCY_DAYS:
                    add(_kind(k), k, "gt_31_days", diff)
            if dates[k][:4] != canonical[:4]:
                add(_kind(k), k, "year_mismatch")
    conflict_reasons = []
    values = sorted(set(primary_day.values()))
    if len(values) > 1:
        spread = (parse_iso(values[-1]) - parse_iso(values[0])).days
        if spread > DISCREPANCY_DAYS:
            conflict_reasons.append("primary_dates_differ_gt_31_days")
        if freeze_date and cutoff_date:
            inside = {cutoff_date <= v <= freeze_date for v in values}
            if len(inside) > 1:
                conflict_reasons.append("primary_dates_straddle_window")
    discrepancies.sort(key=lambda d: (d["source"], d["type"]))
    return {
        "canonical": canonical,
        "canonical_sources": canonical_sources,
        "discrepancies": discrepancies,
        "conflict_reasons": conflict_reasons,
    }


def subtract_years(day, years):
    """Calendar subtraction; 29 February maps to 28 February in non-leap years."""
    try:
        return day.replace(year=day.year - years)
    except ValueError:
        return day.replace(year=day.year - years, day=28)


def cutoff_for(freeze_date, years=5):
    """Rolling cutoff: freeze date minus five calendar years (ISO strings)."""
    return subtract_years(parse_iso(freeze_date), years).isoformat()


def within_window(canonical, cutoff_date, freeze_date):
    if canonical is None:
        return None
    return cutoff_date <= canonical <= freeze_date


def _bounds(value, precision):
    """Earliest/latest calendar day covered by a partial date."""
    if precision == "day":
        return value, value
    y = int(value[:4])
    if precision == "month":  # 'padded' (OpenAlex) is treated as year precision: the day/month may be filler
        m = int(value[5:7])
        last = (dt.date(y + (m == 12), m % 12 + 1, 1) - dt.timedelta(days=1)).isoformat()
        return f"{y:04d}-{m:02d}-01", last
    return f"{y:04d}-01-01", f"{y:04d}-12-31"


def window_by_any_source(dates, precision, cutoff_date, freeze_date):
    """Informative only: do ALL source dates (any precision) fall inside the window?

    Returns 'all_inside', 'all_outside', 'mixed_or_uncertain' or 'no_dates'.
    crossref.created (deposit time) is ignored. Never used to set screening status.
    """
    verdicts = set()
    for key, value in dates.items():
        if key in STORE_ONLY_SOURCES:
            continue
        lo, hi = _bounds(value, precision.get(key))
        if lo >= cutoff_date and hi <= freeze_date:
            verdicts.add("inside")
        elif hi < cutoff_date or lo > freeze_date:
            verdicts.add("outside")
        else:
            verdicts.add("uncertain")
    if not verdicts:
        return "no_dates"
    if verdicts == {"inside"}:
        return "all_inside"
    if verdicts == {"outside"}:
        return "all_outside"
    return "mixed_or_uncertain"
