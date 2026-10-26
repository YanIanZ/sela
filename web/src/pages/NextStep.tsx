import { useEffect, useState } from 'react';
import { useI18n, type MessageKey } from '../i18n';
import { Link, useRouter } from '../lib/router';
import { useSession } from '../lib/session';
import { request } from '../lib/api';
import type { NextSteps, ProfessionalOption, SessionView } from '../lib/types';
import { Badge, Button, Card, ErrorNotice, Notice, PageHeader, Select } from '../components/ui';

const FIELDS = {
  language: ['id', 'en', 'either'],
  format: ['online', 'in_person', 'either'],
  budget: ['free_only', 'low_or_free', 'any'],
  availability: ['weekday_morning', 'weekday_daytime', 'weekday_evening', 'weekend', 'any'],
  barrier: ['none', 'cost', 'schedule', 'how_to_start', 'privacy', 'not_ready'],
} as const;
type PrefKey = keyof typeof FIELDS;

function OptionCard({ opt, approved }: { opt: ProfessionalOption; approved: boolean }) {
  const { t, tx, dateTime } = useI18n();
  const { catalog } = useSession();
  const { navigate } = useRouter();
  const prof = catalog?.professionals.find((p) => p.id === opt.professionalId);
  if (!prof) return null;
  return (
    <article className="card option" aria-labelledby={`p-${prof.id}`}>
      <h3 id={`p-${prof.id}`}>{prof.name}</h3>
      <p className="muted">
        {tx(prof.role)} · {tx(prof.area)} · {tx(prof.costLabel)}
      </p>
      <p>{tx(prof.bio)}</p>
      <ul className="reasons">
        {opt.reasons.map((r) => (
          <li key={r.criterion} className={r.match ? 'ok' : 'miss'}>
            <span className="visually-hidden">{r.match ? t('next.reason.match') : t('next.reason.miss')}: </span>
            <span aria-hidden="true">{r.match ? '✓' : '✗'}</span> {t(`next.reason.${r.criterion}` as MessageKey)}
          </li>
        ))}
      </ul>
      <h4>{t('next.slots')}</h4>
      {opt.slots.length === 0 ? (
        <p className="muted">{t('next.slots.none')}</p>
      ) : (
        <div className="row">
          {opt.slots.map((s) => (
            <Button
              key={s.id}
              variant="secondary"
              disabled={!approved}
              onClick={() => navigate(`/share?pro=${encodeURIComponent(prof.id)}&slot=${encodeURIComponent(s.id)}`)}
            >
              {t('next.slot.choose', { time: `${dateTime(s.start)} (${t(`next.slot.${s.format}` as MessageKey)})` })}
            </Button>
          ))}
        </div>
      )}
    </article>
  );
}

export function NextStepPage() {
  const { t, tx, date } = useI18n();
  const { session, catalog, run, setSession } = useSession();
  const [prefs, setPrefs] = useState<Record<PrefKey, string>>({
    language: session?.preferences?.language ?? session?.language ?? 'id',
    format: session?.preferences?.format ?? 'either',
    budget: session?.preferences?.budget ?? 'any',
    availability: session?.preferences?.availability ?? 'any',
    barrier: session?.preferences?.barrier ?? 'none',
  });
  const [result, setResult] = useState<NextSteps | null>(null);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<unknown>(null);

  useEffect(() => {
    if (session?.preferences && !result) {
      request<NextSteps>('/next-steps').then(setResult).catch(() => undefined);
    }
  }, [session?.preferences, result]);

  if (!session || !catalog) return null;
  const approved = !!session.ledger?.approved && session.ledger.approved.version === session.ledger.version;
  const submit = async () => {
    setBusy(true);
    setError(null);
    try {
      const v = await run((version) => request<SessionView>('/preferences', { method: 'PUT', body: { expectedVersion: version, ...prefs } }));
      setSession(v);
      setResult(await request<NextSteps>('/next-steps'));
    } catch (err) {
      setError(err);
    } finally {
      setBusy(false);
    }
  };
  const steps = (result?.steps ?? []).map((id) => catalog.nextSteps.find((s) => s.id === id)).filter((s) => s !== undefined);
  const stepLink = (action: string) => (action === 'practice' ? '/practice' : action === 'trusted_person' ? '/practice?recipient=trusted_person' : action === 'help' ? '/help' : null);
  const stepsBlock = (
    <section className="card" aria-labelledby="steps-title">
      <h2 id="steps-title">{t('next.results.steps')}</h2>
      <ul className="steps">
        {steps.map((s) => {
          const to = stepLink(s.action);
          return (
            <li key={s.id}>
              <strong>{tx(s.title)}</strong>
              <p>{tx(s.description)}</p>
              {to ? <Link to={to}>{t('next.step.open')}</Link> : null}
            </li>
          );
        })}
      </ul>
    </section>
  );
  const options = result ? (
    <section className="stack" aria-labelledby="exact-title">
      <h2 id="exact-title">{result.handoffFirst ? t('next.results.exact') : t('next.results.whenReady')}</h2>
      {result.exact.length === 0 ? <Notice>{t('next.results.noExact')}</Notice> : null}
      <div className="grid-2">
        {result.exact.map((o) => (
          <OptionCard key={o.professionalId} opt={o} approved={approved} />
        ))}
      </div>
      {result.near.length > 0 ? (
        <>
          <h2>{t('next.results.near')}</h2>
          <div className="grid-2">
            {result.near.map((o) => (
              <OptionCard key={o.professionalId} opt={o} approved={approved} />
            ))}
          </div>
        </>
      ) : null}
    </section>
  ) : null;
  return (
    <div className="stack">
      <PageHeader title={t('next.title')} lead={t('next.lead')} />
      <Card labelledBy="prefs-title">
        <h2 id="prefs-title">{t('next.prefs.title')}</h2>
        <div className="grid-2">
          {(Object.keys(FIELDS) as PrefKey[]).map((k) => (
            <Select
              key={k}
              id={`pref-${k}`}
              label={t(`next.prefs.${k}` as MessageKey)}
              value={prefs[k]}
              onChange={(v) => setPrefs({ ...prefs, [k]: v })}
              options={FIELDS[k].map((v) => ({ value: v, label: t(`next.opt.${k}.${v}` as MessageKey) }))}
            />
          ))}
        </div>
        <ErrorNotice error={error} />
        <Button onClick={submit} busy={busy}>
          {t('next.prefs.submit')}
        </Button>
        <p className="muted small">{t('next.categoryNote')}</p>
        <p className="muted small">{t('next.demoClock', { date: date(catalog.demoClock) })}</p>
      </Card>
      {!approved ? (
        <Notice tone="warning">
          {t('next.needApproval')} <Link to="/ledger">{t('app.nav.ledger')}</Link>
        </Notice>
      ) : null}
      {result && !result.handoffFirst ? <Notice>{t('next.results.notReady')}</Notice> : null}
      {result && !result.handoffFirst ? stepsBlock : null}
      {options}
      {result && result.handoffFirst ? stepsBlock : null}
      <p>
        <Link to="/practice">{t('next.practice')}</Link>
      </p>
      <p className="muted small">
        <Badge tone="demo">{t('app.demoBadge')}</Badge>
      </p>
    </div>
  );
}
