import { useState } from 'react';
import { useI18n } from '../i18n';
import { useRouter } from '../lib/router';
import { useSession } from '../lib/session';
import { request } from '../lib/api';
import type { SessionView } from '../lib/types';
import { Button, Card, ErrorNotice, PageHeader } from '../components/ui';
import { PolicyText } from './Privacy';

export function ConsentControls() {
  const { t } = useI18n();
  const { session, catalog, run, setSession } = useSession();
  const [busy, setBusy] = useState<string | null>(null);
  const [error, setError] = useState<unknown>(null);
  if (!session || !catalog) return null;
  const decide = async (kind: 'ai_processing' | 'storage', granted: boolean) => {
    setBusy(kind);
    setError(null);
    try {
      const v = await run((version) =>
        request<SessionView>(`/consents/${kind}`, { method: 'PUT', body: { expectedVersion: version, granted, policyVersion: catalog.policies[kind].version } }),
      );
      setSession(v);
    } catch (err) {
      setError(err);
    } finally {
      setBusy(null);
    }
  };
  const ai = session.consents.ai_processing;
  const storage = session.consents.storage;
  return (
    <div className="stack">
      <ErrorNotice error={error} />
      <Card labelledBy="ai-title">
        <h2 id="ai-title">{t('consent.ai.title')}</h2>
        <PolicyText policy={catalog.policies.ai_processing} hideTitle />
        <p className="status-line" aria-live="polite">
          {ai ? (ai.granted ? t('consent.ai.granted') : t('consent.ai.declined')) : t('consent.undecided')}
        </p>
        <div className="row">
          <Button variant={ai?.granted ? 'primary' : 'secondary'} aria-pressed={ai?.granted === true} busy={busy === 'ai_processing'} onClick={() => decide('ai_processing', true)}>
            {t('consent.ai.allow')}
          </Button>
          <Button variant={ai && !ai.granted ? 'primary' : 'secondary'} aria-pressed={ai?.granted === false} busy={busy === 'ai_processing'} onClick={() => decide('ai_processing', false)}>
            {t('consent.ai.decline')}
          </Button>
        </div>
      </Card>
      <Card labelledBy="storage-title">
        <h2 id="storage-title">{t('consent.storage.title')}</h2>
        <PolicyText policy={catalog.policies.storage} hideTitle />
        <p className="status-line" aria-live="polite">
          {storage ? (storage.granted ? t('consent.storage.granted') : t('consent.storage.declined')) : t('consent.storage.declined')}
        </p>
        <div className="row">
          <Button variant={storage?.granted ? 'primary' : 'secondary'} aria-pressed={storage?.granted === true} busy={busy === 'storage'} onClick={() => decide('storage', true)}>
            {t('consent.storage.keep')}
          </Button>
          <Button variant={!storage?.granted ? 'primary' : 'secondary'} aria-pressed={!storage?.granted} busy={busy === 'storage'} onClick={() => decide('storage', false)}>
            {t('consent.storage.session')}
          </Button>
        </div>
      </Card>
    </div>
  );
}

export function ConsentPage() {
  const { t } = useI18n();
  const { session } = useSession();
  const { navigate } = useRouter();
  return (
    <div className="stack">
      <PageHeader title={t('consent.title')} lead={t('consent.lead')} />
      <ConsentControls />
      <p className="muted">{t('consent.later')}</p>
      <div className="row">
        <Button onClick={() => navigate('/story')} disabled={!session?.consents.ai_processing}>
          {t('consent.continue')}
        </Button>
      </div>
    </div>
  );
}
