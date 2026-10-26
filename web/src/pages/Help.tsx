import { useI18n, type MessageKey } from '../i18n';
import { useRouter } from '../lib/router';
import { useSession } from '../lib/session';
import { request } from '../lib/api';
import { Button, Notice, PageHeader } from '../components/ui';

export function HelpPage() {
  const { t, tx, date } = useI18n();
  const { navigate } = useRouter();
  const { session, catalog, markEnded } = useSession();
  if (!catalog) return null;
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
      <Notice tone={crisis ? 'alert' : 'info'}>{tx(catalog.helpNotice)}</Notice>
      <ul className="help-list">
        {catalog.help.map((h) => (
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
