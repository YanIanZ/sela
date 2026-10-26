import { useEffect, useState } from 'react';
import { useI18n } from '../i18n';
import { useRouter } from '../lib/router';
import { useSession } from '../lib/session';
import { request } from '../lib/api';
import type { SessionView } from '../lib/types';
import { Button, Card, ErrorNotice, PageHeader, RadioGroup } from '../components/ui';
import { PolicyText } from './Privacy';

export function Start() {
  const { t, lang } = useI18n();
  const { navigate } = useRouter();
  const { catalog, session, setSession } = useSession();
  const [mode, setMode] = useState<'anonymous' | 'pseudonym'>('anonymous');
  const [pseudonym, setPseudonym] = useState('');
  const [adult, setAdult] = useState(false);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<unknown>(null);
  useEffect(() => {
    if (session) navigate(session.stage === 'crisis' ? '/help' : '/story', { replace: true });
  }, [session, navigate]);
  if (!catalog || session) return null;
  const policy = catalog.policies.privacy;
  const submit = async () => {
    setBusy(true);
    setError(null);
    try {
      const v = await request<SessionView>('/sessions', {
        method: 'POST',
        body: { language: lang, identity: mode === 'pseudonym' ? { mode, pseudonym } : { mode }, adultAttested: adult, privacyPolicy: policy.version },
      });
      setSession(v);
      navigate('/consent');
    } catch (err) {
      setError(err);
    } finally {
      setBusy(false);
    }
  };
  return (
    <div className="stack">
      <PageHeader title={t('start.title')} lead={t('start.lead')} />
      <Card>
        <PolicyText policy={policy} />
        <p className="muted">{t('start.policyVersion', { version: policy.version })}</p>
      </Card>
      <Card>
        <RadioGroup
          legend={t('start.identity.title')}
          name="identity"
          value={mode}
          onChange={(v) => setMode(v as 'anonymous' | 'pseudonym')}
          options={[
            { value: 'anonymous', label: t('start.identity.anonymous'), hint: t('start.identity.anonymousHint') },
            { value: 'pseudonym', label: t('start.identity.pseudonym'), hint: t('start.identity.pseudonymHint') },
          ]}
        />
        {mode === 'pseudonym' ? (
          <div className="field">
            <label htmlFor="pseudonym">{t('start.identity.pseudonymLabel')}</label>
            <input id="pseudonym" value={pseudonym} maxLength={24} autoComplete="off" onChange={(e) => setPseudonym(e.target.value)} />
          </div>
        ) : null}
        <div className="checkbox">
          <input id="adult" type="checkbox" checked={adult} onChange={(e) => setAdult(e.target.checked)} />
          <label htmlFor="adult">{t('start.adult')}</label>
        </div>
        <ErrorNotice error={error} />
        <Button onClick={submit} busy={busy} disabled={!adult || (mode === 'pseudonym' && pseudonym.trim().length < 2)}>
          {t('start.submit')}
        </Button>
      </Card>
    </div>
  );
}
