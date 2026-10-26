import { useState } from 'react';
import { useI18n } from '../i18n';
import { useSession } from '../lib/session';
import { request } from '../lib/api';
import type { SessionView } from '../lib/types';
import { Button, Card, ConfirmButton, ErrorNotice, PageHeader, RadioGroup } from '../components/ui';
import { ConsentControls } from './Consent';

export function SettingsPage() {
  const { t, dateTime } = useI18n();
  const { session, run, setSession, markEnded } = useSession();
  const [mode, setMode] = useState<'anonymous' | 'pseudonym'>(session?.identity.mode ?? 'anonymous');
  const [pseudonym, setPseudonym] = useState(session?.identity.pseudonym ?? '');
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<unknown>(null);
  if (!session) return null;
  const saveIdentity = async () => {
    setBusy(true);
    setError(null);
    try {
      const v = await run((version) =>
        request<SessionView>('/session', { method: 'PATCH', body: { expectedVersion: version, identity: mode === 'pseudonym' ? { mode, pseudonym } : { mode } } }),
      );
      setSession(v);
    } catch (err) {
      setError(err);
    } finally {
      setBusy(false);
    }
  };
  const deleteAll = async () => {
    setBusy(true);
    try {
      await request('/session', { method: 'DELETE' });
      markEnded('deleted');
    } catch (err) {
      setError(err);
    } finally {
      setBusy(false);
    }
  };
  return (
    <div className="stack">
      <PageHeader title={t('settings.title')} />
      <Card>
        <h2>{t('settings.language')}</h2>
        <p className="muted">{t('settings.languageNote')}</p>
      </Card>
      <Card>
        <h2>{t('settings.identity')}</h2>
        <RadioGroup
          legend={t('start.identity.title')}
          name="settings-identity"
          value={mode}
          onChange={(v) => setMode(v as 'anonymous' | 'pseudonym')}
          options={[
            { value: 'anonymous', label: t('start.identity.anonymous'), hint: t('start.identity.anonymousHint') },
            { value: 'pseudonym', label: t('start.identity.pseudonym'), hint: t('start.identity.pseudonymHint') },
          ]}
        />
        {mode === 'pseudonym' ? (
          <div className="field">
            <label htmlFor="settings-pseudonym">{t('start.identity.pseudonymLabel')}</label>
            <input id="settings-pseudonym" value={pseudonym} maxLength={24} autoComplete="off" onChange={(e) => setPseudonym(e.target.value)} />
          </div>
        ) : null}
        <Button variant="secondary" onClick={saveIdentity} busy={busy}>
          {t('app.save')}
        </Button>
      </Card>
      <section aria-labelledby="consents-title" className="stack">
        <h2 id="consents-title">{t('settings.consents')}</h2>
        <ConsentControls />
      </section>
      <Card>
        <h2>{t('settings.retention')}</h2>
        <p>{session.retention === 'day' ? t('settings.retention.day') : t('settings.retention.session')}</p>
        <p className="muted">{t('settings.retention.expires', { time: dateTime(session.expiresAt < session.idleExpiresAt ? session.expiresAt : session.idleExpiresAt) })}</p>
      </Card>
      <Card>
        <h2>{t('settings.delete.title')}</h2>
        <p>{t('settings.delete.body')}</p>
        <ErrorNotice error={error} />
        <ConfirmButton label={t('settings.delete.button')} confirmText={t('settings.delete.confirm')} onConfirm={deleteAll} busy={busy} />
      </Card>
    </div>
  );
}
