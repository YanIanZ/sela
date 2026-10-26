import { useI18n, type MessageKey } from '../i18n';
import { useRouter } from '../lib/router';
import { useSession } from '../lib/session';
import { useCatalog } from '../lib/catalog';
import { request } from '../lib/api';
import type { HelpResource, Text } from '../lib/types';
import { Button, Notice, PageHeader } from '../components/ui';
import fallbackJSON from '../data/help.fallback.json';

/** The verified help list bundled with the page (a byte-identical copy of
 * internal/fixtures/data/help.json, checked by help.test.ts), shown whenever
 * the catalog cannot be loaded: Immediate help never depends on that call. */
export const helpFallback = fallbackJSON as unknown as { helpNotice: Text; help: HelpResource[] };

export function HelpPage() {
  const { t, tx, date } = useI18n();
  const { navigate } = useRouter();
  const { session, markEnded } = useSession();
  const { catalog, retry } = useCatalog();
  const help = catalog ?? helpFallback;
  const crisis = session?.safety.interrupted;
  const restart = async () => {
    try {
      await request('/session', { method: 'DELETE' });
    } catch {
      // the session may already be gone
    }
    markEnded();
    navigate('/start');
  };
  return (
    <div className="stack">
      <PageHeader title={t('help.title')} lead={crisis ? t('help.crisisLead') : t('help.lead')} />
      <Notice tone={crisis ? 'alert' : 'info'}>{tx(help.helpNotice)}</Notice>
      {!catalog ? (
        <p className="muted small">
          {t('help.fallback')}{' '}
          <Button variant="quiet" onClick={retry}>
            {t('app.retry')}
          </Button>
        </p>
      ) : null}
      <ul className="help-list">
        {help.help.map((h) => (
          <li key={h.id} className="card">
            <h2>{h.name}</h2>
            <p className="muted">{t('help.region', { region: tx(h.regionName) })}</p>
            <p>{tx(h.description)}</p>
            <ul className="channels">
              {h.channels.map((c) => (
                <li key={c.kind + c.value}>
                  <strong>{t(`help.channel.${c.kind}` as MessageKey)}:</strong>{' '}
                  {c.kind === 'web' ? (
                    <a href={c.value} target="_blank" rel="noreferrer noopener">
                      {c.value}
                    </a>
                  ) : c.kind === 'phone' ? (
                    <a href={`tel:${c.value.replace(/[^0-9+]/g, '')}`}>{c.value}</a>
                  ) : (
                    <span>{c.value}</span>
                  )}
                </li>
              ))}
            </ul>
            <p>
              <strong>{t('help.hours')}:</strong> {tx(h.hours)} · <strong>{t('help.cost')}:</strong> {tx(h.cost)}
            </p>
            <details>
              <summary>{t('help.sources')}</summary>
              <ul>
                {h.sources.map((s) => (
                  <li key={s.url}>
                    <a href={s.url} target="_blank" rel="noreferrer noopener">
                      {s.label}
                    </a>{' '}
                    ({date(s.accessedOn)})
                  </li>
                ))}
              </ul>
              <p className="muted small">{t('help.verified', { date: date(h.lastVerified), by: h.verifiedBy, reverify: date(h.reverifyBy) })}</p>
            </details>
          </li>
        ))}
      </ul>
      {crisis ? <Button variant="secondary" onClick={restart}>{t('help.newSession')}</Button> : null}
    </div>
  );
}
