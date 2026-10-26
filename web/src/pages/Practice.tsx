import { useState } from 'react';
import { useI18n } from '../i18n';
import { Link, useRouter } from '../lib/router';
import { useSession } from '../lib/session';
import { request } from '../lib/api';
import { useOperation } from '../lib/operation';
import type { OpRecord, SessionView } from '../lib/types';
import { Button, Card, ErrorNotice, Notice, PageHeader, ProvenanceBadge, RadioGroup, Spinner } from '../components/ui';

type Coverage = { mentioned: string[]; notMentioned: string[] };

export function PracticePage() {
  const { t } = useI18n();
  const { query } = useRouter();
  const { session, run, setSession, refresh } = useSession();
  const [recipient, setRecipient] = useState(query.get('recipient') === 'trusted_person' ? 'trusted_person' : 'professional');
  const [opId, setOpId] = useState<string | null>(null);
  const [busy, setBusy] = useState<string | null>(null);
  const [error, setError] = useState<unknown>(null);
  const [editing, setEditing] = useState(false);
  const [lines, setLines] = useState<string[]>([]);
  const [attempt, setAttempt] = useState('');
  const [coverage, setCoverage] = useState<Coverage | null>(null);
  const op = useOperation(opId, () => void refresh());
  if (!session) return null;
  const approved = !!session.ledger?.approved && session.ledger.approved.version === session.ledger.version;
  if (!approved) {
    return (
      <div className="stack">
        <PageHeader title={t('practice.title')} lead={t('practice.lead')} />
        <Notice tone="warning">
          {t('next.needApproval')} <Link to="/ledger">{t('app.nav.ledger')}</Link>
        </Notice>
      </div>
    );
  }
  const items = session.ledger?.items ?? [];
  const label = (id: string) => (id === 'user' ? t('practice.basedOn.user') : id === 'intent' ? t('practice.basedOn.intent') : items.find((i) => i.id === id)?.wording ?? id);
  const call = async <T,>(key: string, fn: (version: number) => Promise<T>) => {
    setBusy(key);
    setError(null);
    try {
      return await run(fn);
    } catch (err) {
      setError(err);
      return null;
    } finally {
      setBusy(null);
    }
  };
  const template = async () => {
    const v = await call('template', (version) => request<SessionView>('/opening/template', { method: 'POST', body: { expectedVersion: version, recipient } }));
    if (v) setSession(v);
  };
  const draft = async () => {
    const res = await call('draft', (version) => request<{ operation: OpRecord; session: SessionView }>('/opening/draft', { method: 'POST', body: { expectedVersion: version, recipient } }));
    if (res) {
      setSession(res.session);
      setOpId(res.operation.id);
    }
  };
  const save = async () => {
    const v = await call('save', (version) => request<SessionView>('/opening', { method: 'PUT', body: { expectedVersion: version, lines: lines.filter((l) => l.trim()) } }));
    if (v) {
      setSession(v);
      setEditing(false);
    }
  };
  const practice = async () => {
    const res = await call('practice', () => request<{ coverage: Coverage; safety: string }>('/opening/practice', { method: 'POST', body: { attempt } }));
    if (res) {
      if (res.safety === 'crisis') await refresh();
      setCoverage(res.coverage);
    }
  };
  const opening = session.opening;
  const active = op.phase === 'queued' || op.phase === 'warming' || op.phase === 'running' || op.phase === 'validating';
  const aiOk = session.ai.available && session.consents.ai_processing?.granted;
  return (
    <div className="stack">
      <PageHeader title={t('practice.title')} lead={t('practice.lead')} />
      <Card>
        <RadioGroup
          legend={t('practice.recipient')}
          name="recipient"
          value={recipient}
          onChange={setRecipient}
          options={[
            { value: 'professional', label: t('practice.recipient.professional') },
            { value: 'trusted_person', label: t('practice.recipient.trusted_person') },
          ]}
        />
        <div className="row">
          {aiOk ? (
            <Button onClick={draft} busy={busy === 'draft' || active}>
              {t('practice.ai')}
            </Button>
          ) : null}
          <Button variant="secondary" onClick={template} busy={busy === 'template'}>
            {t('practice.template')}
          </Button>
        </div>
        {active ? <Spinner label={op.phase === 'warming' ? t('ledger.op.warming', { minutes: Math.max(1, Math.round((op.coldStartMs ?? 180000) / 60000)) }) : t('ledger.op.running')} /> : null}
        {op.phase === 'error' ? <ErrorNotice code={op.errorCode ?? 'ai_unavailable'} /> : null}
        <ErrorNotice error={error} />
      </Card>
      <Card labelledBy="lines-title">
        <h2 id="lines-title">{t('practice.lines')}</h2>
        {!opening ? <p className="muted">{t('practice.empty')}</p> : null}
        {opening && !editing ? (
          <>
            <ProvenanceBadge provenance={opening.provenance} />
            <ol className="lines">
              {opening.lines.map((l, i) => (
                <li key={i}>
                  <p className="line">{l.text}</p>
                  <p className="muted small">{t('practice.basedOn', { items: l.basedOn.map(label).join('; ') })}</p>
                </li>
              ))}
            </ol>
            <Button variant="secondary" onClick={() => { setLines(opening.lines.map((l) => l.text)); setEditing(true); }}>
              {t('practice.edit')}
            </Button>
          </>
        ) : null}
        {editing ? (
          <div className="stack-sm">
            {[0, 1, 2].map((i) => (
              <div className="field" key={i}>
                <label htmlFor={`line-${i}`}>{t('practice.editLabel', { n: i + 1 })}</label>
                <input id={`line-${i}`} maxLength={240} value={lines[i] ?? ''} onChange={(e) => { const next = [...lines]; next[i] = e.target.value; setLines(next); }} />
              </div>
            ))}
            <div className="row">
              <Button onClick={save} busy={busy === 'save'} disabled={!lines.some((l) => l && l.trim())}>
                {t('app.save')}
              </Button>
              <Button variant="quiet" onClick={() => setEditing(false)}>
                {t('app.cancel')}
              </Button>
            </div>
          </div>
        ) : null}
      </Card>
      <Card labelledBy="try-title">
        <h2 id="try-title">{t('practice.try.title')}</h2>
        <div className="field">
          <label htmlFor="attempt">{t('practice.try.label')}</label>
          <textarea id="attempt" rows={3} maxLength={1200} value={attempt} onChange={(e) => setAttempt(e.target.value)} />
        </div>
        <Button variant="secondary" onClick={practice} busy={busy === 'practice'} disabled={!attempt.trim()}>
          {t('practice.try.submit')}
        </Button>
        {coverage ? (
          <div className="grid-2" aria-live="polite">
            <div>
              <h3>{t('practice.try.mentioned')}</h3>
              <ul>{coverage.mentioned.map((id) => <li key={id}>{label(id)}</li>)}</ul>
            </div>
            <div>
              <h3>{t('practice.try.notMentioned')}</h3>
              <ul>{coverage.notMentioned.map((id) => <li key={id}>{label(id)}</li>)}</ul>
            </div>
          </div>
        ) : null}
        <p className="muted small">{t('practice.try.note')}</p>
      </Card>
      <p>
        <Link to="/next">{t('practice.toShare')}</Link>
      </p>
    </div>
  );
}
