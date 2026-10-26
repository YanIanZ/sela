import importlib.util
import json
from pathlib import Path
from tempfile import TemporaryDirectory
import unittest
from unittest.mock import patch

SCRIPT = Path(__file__).resolve().parents[2] / 'scripts/research/collect_europepmc.py'
spec = importlib.util.spec_from_file_location('sela_research_collector', SCRIPT)
collector = importlib.util.module_from_spec(spec)
spec.loader.exec_module(collector)


class CollectorTests(unittest.TestCase):
    def test_normalize_doi(self):
        self.assertEqual(collector.normalized_doi('HTTPS://doi.org/10.1234/AbC. '), '10.1234/abc')

    def test_reject_bad_or_future_dates(self):
        sample = {'title': 'Test', 'source': 'MED', 'id': '123', 'firstPublicationDate': '2027-01-01'}
        self.assertIsNone(collector.from_api(sample, 'theme', '2021-10-27', '2026-10-26'))

    def test_fetch_multiple_queries_and_skip_curated(self):
        with TemporaryDirectory() as temp:
            base = Path(temp)
            curated = base / 'curated.json'
            curated.write_text(json.dumps({'papers': [{'doi': '10.5555/curated'}]}))
            def mocked_fetch(query, cursor, size, timeout):
                self.assertEqual(cursor, '*')
                return {
                    'resultList': {'result': [
                        {'title': 'Previously curated', 'doi': '10.5555/CURATED', 'source': 'MED', 'id': '11', 'firstPublicationDate': '2025-01-01'},
                        {'title': 'Relevant new paper', 'doi': '10.5555/new', 'source': 'MED', 'id': '22', 'firstPublicationDate': '2025-02-01'},
                        {'title': 'Outside date', 'doi': '10.5555/future', 'source': 'MED', 'id': '33', 'firstPublicationDate': '2027-02-01'},
                    ]},
                    'nextCursorMark': None,
                }
            args = type('Args', (), {'target': 2, 'page_size': 100, 'pause': .2, 'from_date': '2021-10-27', 'to_date': '2026-10-26', 'curated': curated, 'output': base/'candidates.jsonl', 'manifest': base/'manifest.json', 'per_theme': 1, 'pages_per_theme': 1, 'timeout': 1.0})()
            with patch.object(collector, 'fetch', side_effect=mocked_fetch):
                self.assertEqual(collector.run(args), 0)
            rows = [json.loads(x) for x in args.output.read_text().splitlines()]
            self.assertEqual(len(rows), 1)
            self.assertEqual(rows[0]['doi'], '10.5555/new')
            self.assertEqual(rows[0]['screening_status'], 'candidate_metadata_only_unreviewed')
            self.assertNotIn('abstract', rows[0])
            manifest = json.loads(args.manifest.read_text())
            self.assertEqual(manifest['candidate_count'], 1)
            self.assertGreaterEqual(manifest['duplicate_or_curated_hits'], 1)


if __name__ == '__main__':
    unittest.main()
