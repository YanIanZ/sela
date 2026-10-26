import { useEffect, useState } from 'react';
import { useI18n, type MessageKey } from '../i18n';
import { useSession } from '../lib/session';
import { ApiError, request } from '../lib/api';
import type { RecipientHandoff } from '../lib/types';
import { Badge, Button, Card, ErrorNotice, Notice, PageHeader, Select, Spinner } from '../components/ui';
import { ProjectionView } from '../components/Projection';

function readToken(): string | null {
  const m = /[#&]t=([A-Za-z0-9_-]{20,100})/.exec(window.location.hash);
  return m?.[1] ?? null;
}

export function RecipientPage() {
  const { t, tx } = useI18n();
  const { catalog } = useSession();
  const [token] = useState<string | null>(readToken);
  const [state, setState] = useState<'loading' | 'code' | 'view' | 'ended' | 'notoken'>('loading');
  const [handoff, setHandoff] = useState<RecipientHandoff | null>(null);
  const [code, setCode] = useState('');
  const [reason, setReason] = useState('');
  const [plan, setPlan] = useState('');
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<unknown>(null);

  useEffect(() => {
    // Remove the capability token from the address bar and history.
    if (window.location.hash) window.history.replaceState(null, '', '/r');
    request<RecipientHandoff>('/recipient/handoff')
      .then((h) => {
        setHandoff(h);
        setState('view');
      })
      .catch(() => setState(token ? 'code' : 'notoken'));
  }, [token]);

  const signIn = async () => {
    if (!token) return;
    setBusy(true);
    setError(null);
    try {
      const h = await request<RecipientHandoff>('/recipient/sessions', { method: 'POST', body: { token, code } });
      setHandoff(h);
      setState('view');
    } catch (err) {
      setError(err);
    } finally {
      setBusy(false);
    }
  };
  const act = async (action: string, extra: Record<string, string> = {}) => {
    if (!handoff) return;
    setBusy(true);
    setError(null);
    try {
      const h = await request<RecipientHandoff>('/recipient/handoff/actions', { method: 'POST', body: { expectedVersion: handoff.version, action, ...extra } });
      setHandoff(h);
    } catch (err) {
      if (err instanceof ApiError && err.code === 'access_ended') {
        setHandoff(null);
        setState('ended');
      } else if (err instanceof ApiError && err.code === 'stale_version') {
        setHandoff(await request<RecipientHandoff>('/recipient/handoff'));
        setError(err);
      } else setError(err);
    } finally {
      setBusy(false);
    }
  };
  const signOut = async () => {
    await request('/recipient/session', { method: 'DELETE' }).catch(() => undefined);
    setHandoff(null);
    setState(token ? 'code' : 'notoken');
  };

  const prof = catalog?.professionals.find((p) => p.id === handoff?.professionalId);
  return (
    <div className="stack recipient">
      <PageHeader title={t('recipient.title')} />
      <Notice tone="warning">{t('recipient.banner')}</Notice>
      {state === 'loading' ? <Spinner label={t('app.loading')} /> : null}
      {state === 'notoken' ? <Notice>{t('recipient.noToken')}</Notice> : null}
      {state === 'ended' ? <Notice tone="alert">{t('recipient.ended')}</Notice> : null}
      {state === 'code' ? (
        <Card labelledBy="code-title">
          <h2 id="code-title">{t('recipient.code.title')}</h2>
          <p className="muted">{t('recipient.code.hint')}</p>
          <div className="field">
            <label htmlFor="access-code">{t('recipient.code.label')}</label>
            <input id="access-code" className="mono" autoComplete="one-time-code" maxLength={12} value={code} onChange={(e) => setCode(e.target.value)} />
          </div>
          <ErrorNotice error={error} />
          <Button onClick={signIn} busy={busy} disabled={code.trim().length < 8}>
            {t('recipient.code.submit')}
          </Button>
        </Card>
      ) : null}
      {state === 'view' && handoff && catalog ? (
        <>
          <Card labelledBy="req-title">
            <h2 id="req-title">{t('recipient.for', { recipient: prof?.name ?? handoff.professionalId })}</h2>
            <p>
              <Badge>{t(`journey.status.${handoff.status}` as MessageKey)}</Badge>
            </p>
            <ProjectionView p={handoff.projection} />
          </Card>
          <Card labelledBy="actions-title">
            <h2 id="actions-title">{t('recipient.actions')}</h2>
            <ErrorNotice error={error} />
            {handoff.status === 'requested' ? (
              <Button onClick={() => act('acknowledge')} busy={busy}>
                {t('recipient.acknowledge')}
              </Button>
            ) : null}
            {handoff.status === 'acknowledged' ? (
              <div className="stack-sm">
                <Select
                  id="plan"
                  label={t('recipient.plan')}
                  value={plan}
                  onChange={setPlan}
                  options={[{ value: '', label: '—' }, ...handoff.plans.map((id) => ({ value: id, label: tx(catalog.plans.find((p) => p.id === id)?.title) }))]}
                />
                <Button onClick={() => act('complete', { planId: plan })} busy={busy} disabled={!plan}>
                  {t('recipient.complete')}
                </Button>
              </div>
            ) : null}
            {handoff.status === 'requested' || handoff.status === 'acknowledged' ? (
              <div className="grid-2">
                <div className="stack-sm">
                  <Select
                    id="clarify-reason"
                    label={`${t('recipient.clarify')} — ${t('recipient.reason')}`}
                    value={reason}
                    onChange={setReason}
                    options={[{ value: '', label: '—' }, ...catalog.clarifyReasons.map((r) => ({ value: r.id, label: tx(r.label) }))]}
                  />
                  <Button variant="secondary" onClick={() => act('clarify', { reasonId: reason })} busy={busy} disabled={!catalog.clarifyReasons.some((r) => r.id === reason)}>
                    {t('recipient.clarify')}
                  </Button>
                </div>
                <div className="stack-sm">
                  <Select
                    id="decline-reason"
                    label={`${t('recipient.decline')} — ${t('recipient.reason')}`}
                    value={reason}
                    onChange={setReason}
                    options={[{ value: '', label: '—' }, ...catalog.declineReasons.map((r) => ({ value: r.id, label: tx(r.label) }))]}
                  />
                  <Button variant="secondary" onClick={() => act('decline', { reasonId: reason })} busy={busy} disabled={!catalog.declineReasons.some((r) => r.id === reason)}>
                    {t('recipient.decline')}
                  </Button>
                </div>
              </div>
            ) : (
              <p>{t('recipient.done', { status: t(`journey.status.${handoff.status}` as MessageKey) })}</p>
            )}
          </Card>
          <Button variant="quiet" onClick={signOut}>
            {t('recipient.signOut')}
          </Button>
        </>
      ) : null}
    </div>
  );
}
