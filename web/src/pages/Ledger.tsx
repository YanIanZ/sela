import { useState } from 'react';
import { useI18n, type MessageKey } from '../i18n';
import { useRouter } from '../lib/router';
import { useSession } from '../lib/session';
import { request } from '../lib/api';
import { useOperation } from '../lib/operation';
import { KINDS, type Item, type Kind, type OpRecord, type SessionView } from '../lib/types';
import { Badge, Button, Card, ErrorNotice, Notice, PageHeader, ProvenanceBadge, QualifierChips, Spinner } from '../components/ui';

function KindSelect({ id, value, onChange, label }: { id: string; value: Kind; onChange: (k: Kind) => void; label: string }) {
  const { t } = useI18n();
  return (
    <div className="field inline">
      <label htmlFor={id}>{label}</label>
      <select id={id} value={value} onChange={(e) => onChange(e.target.value as Kind)}>
        {KINDS.map((k) => (
          <option key={k} value={k}>
            {t(`ledger.kind.${k}` as MessageKey)}
          </option>
        ))}
      </select>
    </div>
  );
}

function ItemCard({ item, segmentIndex }: { item: Item; segmentIndex: number }) {
  const { t, has } = useI18n();
  const { run, setSession } = useSession();
  const [editing, setEditing] = useState(false);
  const [wording, setWording] = useState(item.wording);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<unknown>(null);
  const act = async (body: Record<string, unknown>) => {
    setBusy(true);
    setError(null);
    try {
      const v = await run((version) => request<SessionView>(`/ledger/items/${item.id}`, { method: 'PATCH', body: { expectedVersion: version, ...body } }));
      setSession(v);
      setEditing(false);
    } catch (err) {
      setError(err);
    } finally {
      setBusy(false);
    }
  };
  const originKey = `ledger.item.wording.${item.wordingOrigin}` as MessageKey;
  return (
    <li className={`ledger-item status-${item.status}`}>
      <div className="item-head">
        <Badge>{t(`ledger.kind.${item.kind}` as MessageKey)}</Badge>
        <Badge tone={item.status === 'confirmed' ? 'ok' : 'warn'}>{t(`ledger.item.status.${item.status}` as MessageKey)}</Badge>
        <ProvenanceBadge provenance={item.provenance} />
      </div>
      <div className="quote-block">
        <p className="label">{t('ledger.item.quote')}</p>
        <blockquote>{item.quote}</blockquote>
        <p className="muted small">{t('ledger.item.source', { segment: segmentIndex })}</p>
      </div>
      {item.wordingOrigin !== 'quote' || item.notes?.length ? (
        <div className={`wording-block origin-${item.wordingOrigin}`}>
          <p className="label">{t(originKey)}</p>
          {item.wordingOrigin !== 'quote' ? <p className="wording">{item.wording}</p> : null}
        </div>
      ) : (
        <p className="muted small">{t('ledger.item.wording.quote')}</p>
      )}
      <QualifierChips q={item.qualifiers} />
      {item.notes?.length ? (
        <ul className="notes">
          {item.notes.map((n) => {
            const key = `ledger.note.${n}`;
            return <li key={n}>{has(key) ? t(key) : n}</li>;
          })}
        </ul>
      ) : null}
      <ErrorNotice error={error} />
      {editing ? (
        <div className="stack-sm">
          <div className="field">
            <label htmlFor={`w-${item.id}`}>{t('ledger.item.editLabel')}</label>
            <textarea id={`w-${item.id}`} rows={3} maxLength={400} value={wording} onChange={(e) => setWording(e.target.value)} />
          </div>
          <div className="row">
            <Button busy={busy} disabled={wording.trim().length === 0} onClick={() => act({ action: 'edit', wording })}>
              {t('app.save')}
            </Button>
            <Button variant="quiet" onClick={() => setEditing(false)}>
              {t('app.cancel')}
            </Button>
          </div>
        </div>
      ) : (
        <div className="row actions">
          {item.status !== 'confirmed' ? (
            <Button busy={busy} onClick={() => act({ action: 'confirm' })}>
              {t('ledger.item.confirm')}
            </Button>
          ) : null}
          {item.wordingOrigin !== 'quote' ? (
            <Button variant="secondary" busy={busy} onClick={() => act({ action: 'use_quote' })}>
              {t('ledger.item.useQuote')}
            </Button>
          ) : null}
          <Button variant="secondary" onClick={() => { setWording(item.wording); setEditing(true); }}>
            {t('ledger.item.edit')}
          </Button>
          <KindSelect id={`k-${item.id}`} value={item.kind} label={t('ledger.item.kindLabel')} onChange={(k) => act({ action: 'set_kind', kind: k })} />
          <Button variant="quiet" busy={busy} onClick={() => act({ action: 'remove' })}>
            {t('ledger.item.remove')}
          </Button>
        </div>
      )}
    </li>
  );
}

function AIDraft({ session }: { session: SessionView }) {
  const { t } = useI18n();
  const { run, setSession, refresh } = useSession();
  const running = session.ops.find((o) => o.kind === 'ledger_draft' && (o.state === 'queued' || o.state === 'running'));
  const [opId, setOpId] = useState<string | null>(running?.id ?? null);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<unknown>(null);
  const op = useOperation(opId, () => {
    void refresh();
  });
  const consent = session.consents.ai_processing?.granted === true;
  const available = session.ai.available;
  const start = async () => {
    setBusy(true);
    setError(null);
    try {
      const res = await run((version) => request<{ operation: OpRecord; session: SessionView }>('/ledger/draft', { method: 'POST', body: { expectedVersion: version } }));
      setSession(res.session);
      setOpId(res.operation.id);
    } catch (err) {
      setError(err);
    } finally {
      setBusy(false);
    }
  };
  const cancel = async () => {
    if (!opId) return;
    try {
      const v = await run(() => request<SessionView>(`/operations/${opId}`, { method: 'DELETE' }));
      setSession(v);
    } catch (err) {
      setError(err);
    }
  };
  const active = op.phase === 'queued' || op.phase === 'warming' || op.phase === 'running' || op.phase === 'validating';
  if (!available) return <Notice>{t('ledger.ai.disabled')}</Notice>;
  if (!consent) return <Notice>{t('ledger.ai.noConsent')}</Notice>;
  const rejected = op.done?.rejected as Record<string, number> | undefined;
  const rejectedCount = rejected ? Object.values(rejected).reduce((a, b) => a + b, 0) : 0;
  return (
    <div className="stack-sm">
      <p className="muted">{t('ledger.ai.hint')}</p>
      {session.ledger && session.ledger.items.length > 0 && !active ? <p className="muted small">{t('ledger.ai.replaceWarning')}</p> : null}
      <ErrorNotice error={error} />
      {active ? (
        <div className="op-panel" aria-live="polite">
          <Spinner
            label={
              op.phase === 'warming'
                ? t('ledger.op.warming', { minutes: Math.max(1, Math.round((op.coldStartMs ?? 180000) / 60000)) })
                : op.phase === 'validating'
                  ? t('ledger.op.validating')
                  : op.phase === 'queued'
                    ? t('ledger.op.queued')
                    : t('ledger.op.running')
            }
          />
          {op.previews.length > 0 ? (
            <div>
              <p className="label">{t('ledger.op.previewTitle')}</p>
              <ul className="preview-list">
                {op.previews.map((p, i) => (
                  <li key={i}>«{String(p.quote)}»</li>
                ))}
              </ul>
            </div>
          ) : null}
          <Button variant="secondary" onClick={cancel}>
            {t('ledger.op.cancel')}
          </Button>
        </div>
      ) : (
        <Button onClick={start} busy={busy}>
          {op.phase === 'error' ? t('app.retry') : t('ledger.ai.start')}
        </Button>
      )}
      {op.phase === 'done' && op.done ? (
        <Notice tone="success">
          {t('ledger.op.done', { accepted: Number(op.done.accepted ?? 0) })}
          {rejectedCount > 0 ? ` ${t('ledger.op.rejected', { count: rejectedCount })}` : ''}
        </Notice>
      ) : null}
      {op.phase === 'error' ? <ErrorNotice code={op.errorCode ?? 'ai_unavailable'} /> : null}
    </div>
  );
}

function ManualBuilder({ session }: { session: SessionView }) {
  const { t } = useI18n();
  const { run, setSession } = useSession();
  const [kinds, setKinds] = useState<Record<string, Kind>>({});
  const [busy, setBusy] = useState<string | null>(null);
  const [error, setError] = useState<unknown>(null);
  const segments = session.story?.segments ?? [];
  const inLedger = new Set((session.ledger?.items ?? []).filter((i) => i.provenance === 'manual').map((i) => i.segmentId));
  const add = async (segmentId: string) => {
    setBusy(segmentId);
    setError(null);
    try {
      const v = await run((version) => request<SessionView>('/ledger/items', { method: 'POST', body: { expectedVersion: version, segmentId, kind: kinds[segmentId] ?? 'experience' } }));
      setSession(v);
    } catch (err) {
      setError(err);
    } finally {
      setBusy(null);
    }
  };
  return (
    <div className="stack-sm">
      <p className="muted">{t('ledger.manual.hint')}</p>
      <ErrorNotice error={error} />
      <ol className="segment-list">
        {segments.map((s) => (
          <li key={s.id}>
            <p>{s.text}</p>
            <div className="row">
              <KindSelect id={`mk-${s.id}`} value={kinds[s.id] ?? 'experience'} label={t('ledger.manual.kindLabel')} onChange={(k) => setKinds({ ...kinds, [s.id]: k })} />
              {inLedger.has(s.id) ? (
                <Badge tone="ok">{t('ledger.manual.added')}</Badge>
              ) : (
                <Button variant="secondary" busy={busy === s.id} onClick={() => add(s.id)}>
                  {t('ledger.manual.add')}
                </Button>
              )}
            </div>
          </li>
        ))}
      </ol>
    </div>
  );
}

function Clarify({ session }: { session: SessionView }) {
  const { t, tx } = useI18n();
  const { catalog, run, setSession } = useSession();
  const [answer, setAnswer] = useState('');
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<unknown>(null);
  const field = session.ledger?.clarification;
  if (!field || !catalog) return null;
  const send = async (body: Record<string, unknown>) => {
    setBusy(true);
    setError(null);
    try {
      const v = await run((version) => request<SessionView>('/ledger/clarifications', { method: 'POST', body: { expectedVersion: version, field, ...body } }));
      setSession(v);
      setAnswer('');
    } catch (err) {
      setError(err);
    } finally {
      setBusy(false);
    }
  };
  return (
    <div className="clarify stack-sm">
      <h3>{t('ledger.clarify.title')}</h3>
      <p>{tx(catalog.clarifications[field])}</p>
      <ErrorNotice error={error} />
      <div className="field">
        <label htmlFor="clarify-answer">{t('ledger.clarify.answer')}</label>
        <textarea id="clarify-answer" rows={2} maxLength={400} value={answer} onChange={(e) => setAnswer(e.target.value)} />
      </div>
      <div className="row">
        <Button busy={busy} disabled={answer.trim().length < 2} onClick={() => send({ answer })}>
          {t('ledger.clarify.submit')}
        </Button>
        <Button variant="quiet" busy={busy} onClick={() => send({ skip: true })}>
          {t('ledger.clarify.skip')}
        </Button>
      </div>
    </div>
  );
}

export function LedgerPage() {
  const { t } = useI18n();
  const { navigate } = useRouter();
  const { session, run, setSession } = useSession();
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<unknown>(null);
  if (!session) return null;
  if (!session.story || !session.ledger) {
    return (
      <div className="stack">
        <PageHeader title={t('ledger.title')} />
        <Notice>{t('ledger.noStory')}</Notice>
        <Button onClick={() => navigate('/story')}>{t('app.nav.story')}</Button>
      </div>
    );
  }
  const ledger = session.ledger;
  const segIndex = (id: string) => session.story!.segments.findIndex((s) => s.id === id) + 1;
  const total = ledger.items.length;
  const approvedCurrent = ledger.approved && ledger.approved.version === ledger.version;
  const approve = async () => {
    setBusy(true);
    setError(null);
    try {
      const v = await run((version) => request<SessionView>('/ledger/approval', { method: 'POST', body: { expectedVersion: version, ledgerVersion: ledger.version } }));
      setSession(v);
    } catch (err) {
      setError(err);
    } finally {
      setBusy(false);
    }
  };
  return (
    <div className="stack">
      <PageHeader title={t('ledger.title')} lead={t('ledger.lead')} />
      <Card labelledBy="ai-draft">
        <h2 id="ai-draft">{t('ledger.ai.start')}</h2>
        <AIDraft session={session} />
      </Card>
      <Card labelledBy="manual">
        <details>
          <summary>
            <h2 id="manual" className="inline-heading">
              {t('ledger.manual.title')}
            </h2>
          </summary>
          <ManualBuilder session={session} />
        </details>
      </Card>
      <Card labelledBy="items">
        <div className="item-head">
          <h2 id="items">{t('ledger.items.title')}</h2>
          <Badge>{t('ledger.version', { version: ledger.version })}</Badge>
        </div>
        {total === 0 ? <p className="muted">{t('ledger.items.empty')}</p> : null}
        <ul className="ledger-list">
          {ledger.items.map((it) => (
            <ItemCard key={`${it.id}-${it.status}-${it.wording}`} item={it} segmentIndex={segIndex(it.segmentId)} />
          ))}
        </ul>
      </Card>
      <Card labelledBy="unknowns">
        <h2 id="unknowns">{t('ledger.unknowns.title')}</h2>
        <p className="muted">{t('ledger.unknowns.hint')}</p>
        {ledger.unknowns.length > 0 ? (
          <ul>
            {ledger.unknowns.map((u) => (
              <li key={u}>
                {t(`ledger.unknown.${u}` as MessageKey)}
                {ledger.skipped.includes(u) ? ` — ${t('ledger.clarify.skip')}` : ''}
              </li>
            ))}
          </ul>
        ) : null}
        <Clarify session={session} />
      </Card>
      <Card labelledBy="approve">
        <h2 id="approve" className="visually-hidden">
          {t('ledger.approve', { version: ledger.version })}
        </h2>
        <p aria-live="polite">{t('ledger.review.progress', { reviewed: ledger.reviewed, total })}</p>
        <ErrorNotice error={error} />
        {approvedCurrent ? (
          <>
            <Notice tone="success">{t('ledger.approved', { version: ledger.version })}</Notice>
            <p className="muted">{t('ledger.approved.hint')}</p>
            <Button onClick={() => navigate('/next')}>{t('ledger.next')}</Button>
          </>
        ) : (
          <>
            {total === 0 || ledger.reviewed < total ? <p className="muted">{t('ledger.approve.needReview')}</p> : null}
            <Button onClick={approve} busy={busy} disabled={total === 0 || ledger.reviewed < total}>
              {t('ledger.approve', { version: ledger.version })}
            </Button>
          </>
        )}
      </Card>
    </div>
  );
}
