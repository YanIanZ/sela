import { useEffect, useMemo, useState } from 'react';
import { useI18n, type MessageKey } from '../i18n';
import { Link, useRouter } from '../lib/router';
import { useSession } from '../lib/session';
import { newIdempotencyKey, request } from '../lib/api';
import type { HandoffCreated, Projection } from '../lib/types';
import { Button, Card, CopyField, ErrorNotice, Notice, PageHeader } from '../components/ui';
import { ProjectionView } from '../components/Projection';

const OPTIONAL = ['quotes', 'unknowns', 'opening', 'preferences', 'display_name', 'age_range', 'language'] as const;

export function SharePage() {
  const { t, tx, dateTime } = useI18n();
  const { query } = useRouter();
  const { session, catalog, run, setSession } = useSession();
  const proId = query.get('pro') ?? '';
  const slotId = query.get('slot') ?? '';
  const [fields, setFields] = useState<string[]>(['items']);
  const [duration, setDuration] = useState(24);
  const [consent, setConsent] = useState(false);
  const [preview, setPreview] = useState<Projection | null>(null);
  const [previewError, setPreviewError] = useState<unknown>(null);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<unknown>(null);
  const [created, setCreated] = useState<HandoffCreated | null>(null);
  const payloadKey = JSON.stringify({ proId, slotId, fields, duration, v: session?.ledger?.version });
  // One idempotency key per exact payload: retries of the same click reuse it.
  const idemKey = useMemo(() => newIdempotencyKey(), [payloadKey]);

  useEffect(() => {
    setConsent(false);
    if (!proId || !slotId) return;
    request<Projection>('/handoffs/preview', { method: 'POST', body: { professionalId: proId, slotId, fields } })
      .then((p) => {
        setPreview(p);
        setPreviewError(null);
      })
      .catch((e: unknown) => {
        setPreview(null);
        setPreviewError(e);
      });
  }, [proId, slotId, payloadKey]);

  if (!session || !catalog) return null;
  const prof = catalog.professionals.find((p) => p.id === proId);
  const slot = prof?.slots.find((s) => s.id === slotId);
  const ledger = session.ledger;
  const approved = !!ledger?.approved && ledger.approved.version === ledger.version;
  if (created) {
    const link = `${window.location.origin}/r#t=${created.token}`;
    return (
      <div className="stack">
        <PageHeader title={t('share.created.title')} lead={t('share.created.body')} />
        {created.replay ? <Notice>{t('share.replay')}</Notice> : null}
        <Card>
          <CopyField label={t('share.link')} value={link} mono />
          <CopyField label={t('share.code')} value={created.code} mono />
          <p className="muted">{t('share.demoDelivery')}</p>
          <div className="row">
            <a className="btn btn-secondary" href={`/r#t=${created.token}`} target="_blank" rel="noreferrer noopener">
              {t('share.openAsRecipient')}
            </a>
            <Link to="/journey" className="btn btn-primary">
              {t('share.toJourney')}
            </Link>
          </div>
          <Notice tone="warning">{t('share.notCare')}</Notice>
        </Card>
      </div>
    );
  }
  if (!prof || !slot) {
    return (
      <div className="stack">
        <PageHeader title={t('share.title')} />
        <Notice>{t('share.choose')}</Notice>
        <Link to="/next">{t('app.nav.next')}</Link>
      </div>
    );
  }
  if (!approved || !ledger) {
    return (
      <div className="stack">
        <PageHeader title={t('share.title')} />
        <Notice tone="warning">{t('share.needApproval')}</Notice>
        <Link to="/ledger">{t('app.nav.ledger')}</Link>
      </div>
    );
  }
  const toggle = (f: string) => setFields(fields.includes(f) ? fields.filter((x) => x !== f) : [...fields, f]);
  const optionDisabled = (f: string) => (f === 'opening' && !(session.opening && session.opening.briefVersion === ledger.version)) || (f === 'preferences' && !session.preferences);
  const submit = async () => {
    setBusy(true);
    setError(null);
    try {
      const res = await run((version) =>
        request<HandoffCreated>('/handoffs', {
          method: 'POST',
          headers: { 'Idempotency-Key': idemKey },
          body: { expectedVersion: version, ledgerVersion: ledger.version, professionalId: prof.id, slotId: slot.id, fields, durationHours: duration, shareConsent: consent, sharingPolicy: catalog.policies.sharing.version },
        }),
      );
      setSession(res.session);
      setCreated(res);
    } catch (err) {
      setError(err);
    } finally {
      setBusy(false);
    }
  };
  const durationLabel = t(`share.duration.${duration}` as MessageKey);
  return (
    <div className="stack">
      <PageHeader title={t('share.title')} lead={t('share.lead')} />
      <Card>
        <p>
          <strong>{t('share.recipient')}:</strong> {prof.name} — {tx(prof.role)}
        </p>
        <p>
          <strong>{t('share.slot')}:</strong> {dateTime(slot.start)} ({t(`next.slot.${slot.format}` as MessageKey)})
        </p>
      </Card>
      <Card labelledBy="fields-title">
        <fieldset>
          <legend id="fields-title">{t('share.fields')}</legend>
          <div className="checkbox">
            <input id="f-items" type="checkbox" checked disabled />
            <label htmlFor="f-items">{t('share.field.items')}</label>
          </div>
          {OPTIONAL.map((f) => (
            <div className="checkbox" key={f}>
              <input id={`f-${f}`} type="checkbox" checked={fields.includes(f)} disabled={optionDisabled(f)} onChange={() => toggle(f)} />
              <label htmlFor={`f-${f}`}>{t(`share.field.${f}` as MessageKey)}</label>
            </div>
          ))}
        </fieldset>
        <div className="field">
          <label htmlFor="duration">{t('share.duration')}</label>
          <select id="duration" value={duration} onChange={(e) => setDuration(Number(e.target.value))}>
            {[1, 24, 72].map((d) => (
              <option key={d} value={d}>
                {t(`share.duration.${d}` as MessageKey)}
              </option>
            ))}
          </select>
          <p className="muted small">{t('share.durationCap')}</p>
        </div>
      </Card>
      <Card labelledBy="preview-title">
        <h2 id="preview-title">{t('share.preview')}</h2>
        <ErrorNotice error={previewError} />
        {preview ? <ProjectionView p={preview} /> : null}
      </Card>
      <Card>
        <Notice tone="warning">{t('share.cannotRecall')}</Notice>
        <p className="muted">{t('share.notCare')}</p>
        <div className="checkbox">
          <input id="share-consent" type="checkbox" checked={consent} onChange={(e) => setConsent(e.target.checked)} />
          <label htmlFor="share-consent">{t('share.consent', { version: ledger.version, recipient: prof.name, duration: durationLabel })}</label>
        </div>
        <ErrorNotice error={error} />
        <Button onClick={submit} busy={busy} disabled={!consent || !preview}>
          {t('share.submit')}
        </Button>
      </Card>
    </div>
  );
}
