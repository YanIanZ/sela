import { useI18n } from '../i18n';
import { Link, useRouter } from '../lib/router';
import { useSession } from '../lib/session';
import { Button, Notice, PageHeader } from '../components/ui';

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
