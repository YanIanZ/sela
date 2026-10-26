import { useEffect, useState } from 'react';
import { useI18n } from '../i18n';
import { ApiError, request } from '../lib/api';
import { Badge, Button, Card, ErrorNotice, Notice, PageHeader } from '../components/ui';

type Locator = { part?: string; section?: string; paragraph?: number };
type Evidence = { field: string; value: unknown; snippet?: string; locator?: Locator | null; source_url?: string; status?: string };
type KBRecord = {
  id: string;
  title: string;
  authors?: string[];
  year?: number | null;
  publication_date?: string | null;
  journal?: string | null;
  doi?: string | null;
  url?: string | null;
  theme?: string | null;
  note_id?: string | null;
  transfer_limitation_id?: string | null;
  study_type?: string | null;
  population?: string[] | null;
  country_or_language?: string[] | null;
  sample_size?: number | null;
  license?: string | null;
  access_status?: string | null;
  screening_status?: string | null;
  reviewer_status?: string | null;
  within_cutoff?: boolean | null;
  flags?: string[];
  evidence?: Evidence[];
};
type SearchResult = { total: number; results: { record: KBRecord; score: number }[]; indexVersion: string; themes: string[]; statuses: string[] };
type Manifest = { index_version?: string; freeze_date?: string; cutoff_date?: string; counts?: Record<string, unknown> };

export function ResearchPage() {
  const { t } = useI18n();
  const [q, setQ] = useState('');
  const [theme, setTheme] = useState('');
  const [status, setStatus] = useState('');
  const [manifest, setManifest] = useState<Manifest | null>(null);
  const [result, setResult] = useState<SearchResult | null>(null);
  const [error, setError] = useState<unknown>(null);
  const [unavailable, setUnavailable] = useState(false);
  const search = async () => {
    setError(null);
    try {
      const params = new URLSearchParams({ q, theme, status, limit: '30' });
      setResult(await request<SearchResult>(`/research/search?${params.toString()}`));
    } catch (err) {
      if (err instanceof ApiError && err.code === 'research_unavailable') setUnavailable(true);
      else setError(err);
    }
  };
  useEffect(() => {
    request<Manifest>('/research/manifest')
      .then(setManifest)
      .catch((err: unknown) => {
        if (err instanceof ApiError && err.code === 'research_unavailable') setUnavailable(true);
      });
    void search();
  }, []);
  const val = (v: unknown) =>
    v === null || v === undefined || v === '' || (Array.isArray(v) && v.length === 0) ? t('research.unknown') : Array.isArray(v) ? v.join(', ') : String(v);
  const where = (l?: Locator | null) => (l ? [l.part, l.section, l.paragraph !== undefined ? `¶${l.paragraph}` : undefined].filter(Boolean).join(' · ') : '');
  return (
    <div className="stack">
      <PageHeader title={t('research.title')} lead={t('research.lead')} />
      <Notice>{t('research.noSessionData')}</Notice>
      {unavailable ? <Notice tone="warning">{t('research.unavailable')}</Notice> : null}
      {manifest ? <p className="muted">{t('research.manifest', { index: val(manifest.index_version), freeze: val(manifest.freeze_date), cutoff: val(manifest.cutoff_date) })}</p> : null}
      <Card>
        <form
          className="stack-sm"
          role="search"
          onSubmit={(e) => {
            e.preventDefault();
            void search();
          }}
        >
          <div className="field">
            <label htmlFor="rq">{t('research.search')}</label>
            <input id="rq" type="search" maxLength={200} value={q} onChange={(e) => setQ(e.target.value)} />
          </div>
          <div className="grid-2">
            <div className="field">
              <label htmlFor="rtheme">{t('research.theme')}</label>
              <select id="rtheme" value={theme} onChange={(e) => setTheme(e.target.value)}>
                <option value="">{t('research.theme.all')}</option>
                {(result?.themes ?? []).map((x) => (
                  <option key={x} value={x}>
                    {x}
                  </option>
                ))}
              </select>
            </div>
            <div className="field">
              <label htmlFor="rstatus">{t('research.status')}</label>
              <select id="rstatus" value={status} onChange={(e) => setStatus(e.target.value)}>
                <option value="">{t('research.status.all')}</option>
                {(result?.statuses ?? []).map((x) => (
                  <option key={x} value={x}>
                    {x}
                  </option>
                ))}
              </select>
            </div>
          </div>
          <Button type="submit">{t('research.searchButton')}</Button>
        </form>
      </Card>
      <ErrorNotice error={error} />
      {result ? <p aria-live="polite">{t('research.results', { count: result.total })}</p> : null}
      <ul className="research-list">
        {(result?.results ?? []).map(({ record: r }) => (
          <li key={r.id} className="card">
            <h2 className="h3">
              {r.id} — {r.title}
            </h2>
            <p className="muted small">
              {(r.authors ?? []).slice(0, 4).join(', ')}
              {(r.authors?.length ?? 0) > 4 ? ' …' : ''} · {val(r.journal)} · {val(r.publication_date)}
            </p>
            <p className="row">
              {r.theme ? <Badge>{r.theme}</Badge> : null}
              <Badge tone={r.within_cutoff ? 'ok' : 'warn'}>{val(r.screening_status)}</Badge>
              <Badge>{val(r.access_status)}</Badge>
            </p>
            <dl className="facts">
              <dt>{t('research.field.studyType')}</dt>
              <dd>{val(r.study_type)}</dd>
              <dt>{t('research.field.population')}</dt>
              <dd>{val(r.population)}</dd>
              <dt>{t('research.field.country')}</dt>
              <dd>{val(r.country_or_language)}</dd>
              <dt>{t('research.field.sample')}</dt>
              <dd>{val(r.sample_size)}</dd>
              <dt>{t('research.field.license')}</dt>
              <dd>{val(r.license)}</dd>
              <dt>{t('research.field.reviewer')}</dt>
              <dd>{val(r.reviewer_status)}</dd>
            </dl>
            {r.note_id ? (
              <p lang="id">
                <strong>{t('research.field.note')}:</strong> {r.note_id}
              </p>
            ) : null}
            {r.transfer_limitation_id ? (
              <p lang="id" className="muted">
                <strong>{t('research.field.limit')}:</strong> {r.transfer_limitation_id}
              </p>
            ) : null}
            {r.evidence && r.evidence.length > 0 ? (
              <details>
                <summary>{t('research.field.evidence')}</summary>
                <ul>
                  {r.evidence.map((e, i) => (
                    <li key={i}>
                      {e.field}: {val(e.value)} {e.locator ? `(${where(e.locator)})` : ''} {e.snippet ? `«${e.snippet}»` : ''} {e.status ? <Badge>{e.status}</Badge> : null}
                    </li>
                  ))}
                </ul>
              </details>
            ) : null}
            {r.doi ? (
              <a href={`https://doi.org/${r.doi}`} target="_blank" rel="noreferrer noopener">
                doi:{r.doi}
              </a>
            ) : null}
          </li>
        ))}
      </ul>
    </div>
  );
}
