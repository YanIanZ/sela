import { useEffect } from 'react';
import { useI18n } from '../i18n';
import { Link, useRouter } from '../lib/router';
import { useSession } from '../lib/session';
import { useCatalog } from '../lib/catalog';
import { Button, ErrorNotice, Notice, PageHeader } from '../components/ui';

export function NotFound() {
  const { t } = useI18n();
  return (
    <div className="stack">
      <PageHeader title={t('app.notFound.title')} />
      <Link to="/">{t('app.notFound.home')}</Link>
    </div>
  );
}

export function SessionEnded() {
  const { t } = useI18n();
  const { navigate } = useRouter();
  const { endedReason } = useSession();
  // The control that ended the session (e.g. "Delete all data now") is gone:
  // move focus to the page that says what happened.
  useEffect(() => {
    document.getElementById('main')?.focus();
  }, []);
  if (endedReason === 'deleted') {
    return (
      <div className="stack">
        <PageHeader title={t('settings.delete.title')} />
        <Notice tone="success">{t('settings.delete.done')}</Notice>
        <Button onClick={() => { window.location.assign('/start'); navigate('/start', { replace: true }); }}>{t('app.sessionEnded.restart')}</Button>
      </div>
    );
  }
  return (
    <div className="stack">
      <PageHeader title={t('app.sessionEnded.title')} />
      <Notice>{t('app.sessionEnded.body')}</Notice>
      <Button onClick={() => { window.location.assign('/start'); navigate('/start', { replace: true }); }}>{t('app.sessionEnded.restart')}</Button>
    </div>
  );
}

/** CatalogUnavailable replaces a page that needs the fixture catalog while it
 * cannot be loaded (it is retried in the background). Help stays reachable. */
export function CatalogUnavailable() {
  const { t } = useI18n();
  const { error, retry } = useCatalog();
  return (
    <div className="stack">
      <PageHeader title={t('app.catalogError.title')} />
      <ErrorNotice code={error} />
      <p>{t('app.catalogError.body')}</p>
      <div className="row">
        <Button onClick={retry}>{t('app.retry')}</Button>
        <Link to="/help" className="btn btn-secondary">
          {t('app.nav.help')}
        </Link>
      </div>
    </div>
  );
}

/** Interrupted replaces every journey page once the session took the crisis
 * route: no journey control is offered, only help and the way to delete. */
export function Interrupted() {
  const { t } = useI18n();
  return (
    <div className="stack interrupted">
      <PageHeader title={t('interrupted.title')} lead={t('interrupted.body')} />
      <div className="row">
        <Link to="/help" className="btn btn-primary">
          {t('app.nav.help')}
        </Link>
        <Link to="/settings" className="btn btn-secondary">
          {t('interrupted.settings')}
        </Link>
      </div>
    </div>
  );
}
