import { useState } from 'react';
import { useI18n, type MessageKey } from '../i18n';
import { Link } from '../lib/router';
import { useSession } from '../lib/session';
import { request } from '../lib/api';
import type { Handoff, SessionView } from '../lib/types';
import { Badge, Button, Card, ConfirmButton, ErrorNotice, Notice, PageHeader } from '../components/ui';

function HandoffCard({ h }: { h: Handoff }) {
  const { t, tx, dateTime, has } = useI18n();
  const { session, catalog, run, setSession } = useSession();
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<unknown>(null);
  if (!session || !catalog) return null;
  const prof = catalog.professionals.find((p) => p.id === h.professionalId);
  const plan = h.planId ? catalog.plans.find((p) => p.id === h.planId) : undefined;
  const clarify = h.clarifyReason ? catalog.clarifyReasons.find((r) => r.id === h.clarifyReason) : undefined;
  const decline = h.declineReason ? catalog.declineReasons.find((r) => r.id === h.declineReason) : undefined;
  const call = async (fn: (version: number) => Promise<SessionView>) => {
    setBusy(true);
    setError(null);
    try {
      setSession(await run(fn));
    } catch (err) {
      setError(err);
    } finally {
      setBusy(false);
    }
  };
  const revoke = () => call((version) => request<SessionView>(`/handoffs/${h.id}/share`, { method: 'DELETE', body: { expectedVersion: version } }));
  const checkIn = (optionId: string) =>
    call((version) => request<SessionView>('/follow-up/check-ins', { method: 'POST', body: { expectedVersion: version, handoffId: h.id, optionId } }));
  const done = new Set(session.checkIns.filter((c) => c.handoffId === h.id).map((c) => c.optionId));
  return (
    <article className="card handoff" aria-labelledby={`h-${h.id}`}>
      <h2 id={`h-${h.id}`}>{t('journey.to', { recipient: prof?.name ?? h.professionalId })}</h2>
      <p className="row">
        <Badge tone={h.status === 'completed' ? 'ok' : 'neutral'}>{t(`journey.status.${h.status}` as MessageKey)}</Badge>
        <Badge tone={h.share === 'active' ? 'ok' : 'warn'}>{t(`journey.share.${h.share}` as MessageKey)}</Badge>
        <Badge>{t('journey.version', { version: h.briefVersion })}</Badge>
      </p>
      <p>{h.opened ? t('journey.opened') : t('journey.notOpened')}</p>
      {h.share === 'active' ? <p className="muted">{t('journey.expires', { time: dateTime(h.expiresAt) })}</p> : null}
      <Notice>{t('journey.careNote')}</Notice>
      {h.status === 'requested' && h.share === 'active' ? <p className="muted">{t('journey.waiting.hint')}</p> : null}
      {h.status === 'clarification_requested' ? (
        <Notice tone="warning">
          {clarify ? tx(clarify.label) : null} {t('journey.clarify.hint')} <Link to="/ledger">{t('journey.clarify.action')}</Link>
        </Notice>
      ) : null}
      {h.status === 'declined' ? (
        <Notice tone="warning">
          {decline ? tx(decline.label) : null} <Link to="/next">{t('journey.declined.action')}</Link>
        </Notice>
      ) : null}
      {plan && h.status === 'completed' && !session.safety.interrupted ? (
        <section className="plan" aria-labelledby={`plan-${h.id}`}>
          <h3 id={`plan-${h.id}`}>{t('journey.plan.title')}</h3>
          <p className="muted">{tx(plan.title)}</p>
          <ol>
            {plan.steps.map((s, i) => (
              <li key={i}>{tx(s)}</li>
            ))}
          </ol>
          <p className="muted small">{tx(plan.attribution)}</p>
          <>
              <h3>{t('journey.checkin.title')}</h3>
              <div className="row">
                {catalog.checkIns.map((o) =>
                  done.has(o.id) ? (
                    <Badge key={o.id} tone="ok">
                      {tx(o.label)} — {t('journey.checkin.done')}
                    </Badge>
                  ) : (
                    <Button key={o.id} variant="secondary" busy={busy} onClick={() => checkIn(o.id)}>
                      {tx(o.label)}
                    </Button>
                  ),
                )}
              </div>
            </>
        </section>
      ) : null}
      <details>
        <summary>{t('journey.timeline')}</summary>
        <ol className="timeline">
          {h.timeline.map((e, i) => {
            const key = `journey.event.${e.kind}`;
            return (
              <li key={i}>
                {has(key) ? t(key) : e.kind} — {dateTime(e.at)}
              </li>
            );
          })}
        </ol>
      </details>
      <ErrorNotice error={error} />
      {h.share === 'active' ? <ConfirmButton label={t('journey.revoke')} confirmText={t('journey.revokeConfirm')} onConfirm={revoke} busy={busy} /> : null}
    </article>
  );
}

export function JourneyPage() {
  const { t } = useI18n();
  const { session } = useSession();
  if (!session) return null;
  const list = [...session.handoffs].reverse();
  return (
    <div className="stack">
      <PageHeader title={t('journey.title')} lead={t('journey.lead')} />
      {list.length === 0 ? (
        <Card>
          <p>{t('journey.empty')}</p>
          <Link to="/next">{t('app.nav.next')}</Link>
        </Card>
      ) : null}
      {list.map((h) => (
        <HandoffCard key={`${h.id}-${h.version}`} h={h} />
      ))}
    </div>
  );
}
