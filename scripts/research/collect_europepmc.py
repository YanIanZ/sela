#!/usr/bin/env python3
"""Fetch candidate *metadata* for a Sela literature evidence map from Europe PMC.

Stdlib-only, bounded/rate-limited API requests. Never downloads copyrighted PDFs,
full-text XML or article abstracts for redistribution. A candidate is NOT reviewed evidence.
"""
import argparse
import datetime as dt
import hashlib
import json
import os
from pathlib import Path
import re
import sys
import time
from urllib.error import HTTPError, URLError
from urllib.parse import urlencode
from urllib.request import Request, urlopen

API = 'https://www.ebi.ac.uk/europepmc/webservices/rest/search'
QUERIES = {
    'mental_health_help_seeking': '"mental health" AND ("help seeking" OR "help-seeking" OR "treatment seeking")',
    'stigma_barriers': '"mental health" AND (stigma OR barriers OR access OR affordability)',
    'care_navigation': '"mental health" AND ("care navigation" OR referral OR "patient navigation")',
    'continuity_handoff': '"mental health" AND ("continuity of care" OR "warm handoff" OR "care coordination")',
    'shared_decision': '"mental health" AND ("shared decision making" OR "patient engagement")',
    'digital_mental_health': '"digital mental health" OR "mobile mental health"',
    'ai_mental_health': '"mental health" AND ("artificial intelligence" OR chatbot OR "large language model")',
    'ai_safety': '(chatbot OR "large language model") AND ("mental health" OR psychiatry) AND (safety OR harm OR bias)',
    'clinical_summarization': '("clinical summarization" OR "clinical documentation") AND ("language model" OR AI)',
    'health_privacy': '("mental health" OR psychotherapy) AND (privacy OR consent OR confidentiality)',
    'bilingual_health': '("mental health" OR psychotherapy) AND (multilingual OR translation OR "language barrier")',
    'cultural_equity': '"mental health" AND (cultural OR equity OR disparities OR underserved)',
    'indonesia_asean': '(Indonesia OR Indonesian OR ASEAN) AND ("mental health" OR psychotherapy)',
    'youth_students': '(university OR college OR students OR "young adults") AND "mental health"',
    'digital_engagement': '("mental health app" OR "digital mental health") AND (engagement OR dropout OR adherence)',
    'psychological_services': '(psychological OR psychiatric) AND ("health services" OR "service utilization")',
    'primary_care_integration': '("mental health" OR depression) AND ("primary care" OR "integrated care")',
    'user_experience': '("mental health" OR psychiatry) AND (usability OR "user experience" OR acceptability)',
}


def normalized_doi(value):
    doi = (value or '').strip().lower()
    doi = re.sub(r'^https?://(?:dx\.)?doi\.org/', '', doi)
    return doi.split('?')[0].rstrip('.,; )')


def normalized_title(value):
    return re.sub(r'[^a-z0-9]+', '', (value or '').lower())


def primary_key(row):
    doi = normalized_doi(row.get('doi'))
    if doi:
        return 'doi:' + doi
    pmid = str(row.get('pmid') or '').strip()
    if pmid:
        return 'pmid:' + pmid
    title = normalized_title(row.get('title'))
    if title:
        return 'title:' + hashlib.sha256((title + '|' + (row.get('first_publication_date') or '')[:4]).encode()).hexdigest()[:24]
    return ''


def from_api(raw, theme, start, end):
    date = str(raw.get('firstPublicationDate') or raw.get('firstIndexDate') or raw.get('firstPublicationDate') or '')
    if len(date) < 10 or date[:10] < start or date[:10] > end:
        return None
    doi = normalized_doi(raw.get('doi'))
    pubid = str(raw.get('id') or '')
    title = ' '.join(str(raw.get('title') or '').split())
    if not title:
        return None
    source = str(raw.get('source') or '').upper()
    if source != 'MED':
        return None
    return {
        'title': title,
        'doi': doi or None,
        'pmid': pubid if pubid.isdigit() else None,
        'first_publication_date': date[:10],
        'authors': str(raw.get('authorString') or '')[:4000],
        'journal': str(raw.get('journalTitle') or ''),
        'publication_type': str(raw.get('pubType') or ''),
        'europepmc_url': f'https://europepmc.org/article/MED/{pubid}',
        'themes': [theme],
        'screening_status': 'candidate_metadata_only_unreviewed',
    }


def fetch(query, cursor, size, timeout, attempts=4):
    params = urlencode({'query': query, 'format': 'json', 'resultType': 'lite', 'pageSize': size, 'cursorMark': cursor})
    req = Request(API + '?' + params, headers={'User-Agent': 'Sela-Research-Metadata-Collector/1.0 (research only; contact via GitHub YanIanZ/sela)', 'Accept': 'application/json'})
    for tries in range(attempts):
        try:
            with urlopen(req, timeout=timeout) as response:
                return json.load(response)
        except HTTPError as e:
            if e.code not in (429, 500, 502, 503, 504) or tries == attempts - 1:
                raise
            retry_after = e.headers.get('Retry-After', '')
            try:
                delay = min(60.0, max(1.0, float(retry_after)))
            except ValueError:
                delay = 2 ** tries
            time.sleep(delay)
        except (URLError, TimeoutError):
            if tries == attempts - 1:
                raise
            time.sleep(2 ** tries)
    raise RuntimeError('unreachable')


def load_curated(path):
    if not path.exists():
        raise FileNotFoundError(f'Existing research_100.json missing: {path}')
    obj = json.loads(path.read_text(encoding='utf-8'))
    rows = obj.get('papers', [])
    return {normalized_doi(p.get('doi')) for p in rows if normalized_doi(p.get('doi'))}


def atomic_write(path, data):
    path.parent.mkdir(parents=True, exist_ok=True)
    tmp = path.with_name(path.name + '.tmp')
    tmp.write_text(data, encoding='utf-8')
    os.replace(tmp, path)


def run(args):
    if args.target < 1 or args.page_size < 1 or args.page_size > 1000 or args.pause < .2:
        raise ValueError('target must be positive, page-size 1..1000, and pause >= 0.2 seconds')
    if args.from_date > args.to_date:
        raise ValueError('from-date must precede to-date')
    known_doi = load_curated(args.curated)
    by_id = {}
    title_index = {}
    duplicate_hits = 0
    fetched = 0
    counts = {}
    errors = {}
    date_filter = f'FIRST_PDATE:[{args.from_date} TO {args.to_date}] AND HAS_ABSTRACT:Y AND SRC:MED'
    for theme, phrase in QUERIES.items():
        if len(by_id) >= args.target:
            break
        query = f'({phrase}) AND {date_filter}'
        cursor = '*'
        seen_cursors = set()
        accepted_this_theme = 0
        pages = 0
        while pages < args.pages_per_theme and accepted_this_theme < args.per_theme and len(by_id) < args.target:
            if cursor in seen_cursors:
                break
            seen_cursors.add(cursor)
            try:
                payload = fetch(query, cursor, args.page_size, args.timeout)
            except (HTTPError, URLError, TimeoutError, ValueError) as exc:
                errors[theme] = str(exc)
                print(f'ERROR [{theme}]: {exc}', file=sys.stderr)
                break
            items = payload.get('resultList', {}).get('result', [])
            fetched += len(items)
            pages += 1
            if not items:
                break
            for item in items:
                row = from_api(item, theme, args.from_date, args.to_date)
                if row is None:
                    continue
                if row.get('doi') in known_doi:
                    duplicate_hits += 1
                    continue
                key = primary_key(row)
                title_fallback = (normalized_title(row['title']), row['first_publication_date'][:4])
                if key in by_id or title_fallback in title_index:
                    existing = by_id.get(key) or by_id[title_index[title_fallback]]
                    if theme not in existing['themes']:
                        existing['themes'].append(theme)
                    duplicate_hits += 1
                    continue
                if not key:
                    continue
                by_id[key] = row
                title_index[title_fallback] = key
                accepted_this_theme += 1
                if len(by_id) >= args.target or accepted_this_theme >= args.per_theme:
                    break
            new_cursor = payload.get('nextCursorMark')
            if not new_cursor or new_cursor == cursor:
                break
            cursor = new_cursor
            if args.pause:
                time.sleep(args.pause)
        counts[theme] = {'accepted': accepted_this_theme, 'pages': pages}
        print(f'{theme}: {accepted_this_theme} new, {len(by_id)} total')
    candidates = sorted(by_id.values(), key=lambda r: (r['first_publication_date'], r['title']), reverse=True)
    output = ''.join(json.dumps(r, ensure_ascii=False, sort_keys=True) + '\n' for r in candidates)
    manifest = {
        'generated_utc': dt.datetime.now(dt.timezone.utc).isoformat(),
        'source': API,
        'query_filters': date_filter,
        'target_unique_candidates': args.target,
        'candidate_count': len(candidates),
        'existing_curated_doi_count': len(known_doi),
        'raw_api_records_seen': fetched,
        'duplicate_or_curated_hits': duplicate_hits,
        'request_errors': errors,
        'thematic_counts': counts,
        'content': 'Bibliographic metadata only. No abstracts, PDFs or full texts. Not clinically reviewed.',
        'limits': 'Search coverage, licenses, screening relevance and study quality unverified; not a systematic review.',
    }
    atomic_write(args.output, output)
    atomic_write(args.manifest, json.dumps(manifest, indent=2, ensure_ascii=False) + '\n')
    print(f'Wrote {len(candidates)} UNREVIEWED candidate records to {args.output}')
    if errors:
        print('Incomplete due to network/API errors; see manifest.', file=sys.stderr)
    return 1 if errors else 0


def main():
    p = argparse.ArgumentParser(description=__doc__)
    p.add_argument('--target', type=int, default=5000)
    p.add_argument('--per-theme', type=int, default=750)
    p.add_argument('--pages-per-theme', type=int, default=15)
    p.add_argument('--page-size', type=int, default=100)
    p.add_argument('--pause', type=float, default=0.5)
    p.add_argument('--timeout', type=float, default=30.0)
    p.add_argument('--from-date', default='2021-10-27')
    p.add_argument('--to-date', default='2026-10-26')
    default_curated = Path(__file__).resolve().parents[2] / 'docs/research/sela-relay/research_100.json'
    p.add_argument('--curated', type=Path, default=default_curated)
    p.add_argument('--output', type=Path, default=Path('data/research/sela_candidates.jsonl'))
    p.add_argument('--manifest', type=Path, default=Path('data/research/sela_candidates_manifest.json'))
    args = p.parse_args()
    try:
        return run(args)
    except (ValueError, FileNotFoundError) as exc:
        p.error(str(exc))


if __name__ == '__main__':
    sys.exit(main())
