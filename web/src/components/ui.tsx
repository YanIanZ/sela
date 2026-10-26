import { useEffect, useId, useRef, useState, type ButtonHTMLAttributes, type ReactNode, type Ref } from 'react';
import { useI18n, type MessageKey } from '../i18n';
import type { Provenance, Qualifiers } from '../lib/types';
import { ApiError } from '../lib/api';

type ButtonProps = ButtonHTMLAttributes<HTMLButtonElement> & { variant?: 'primary' | 'secondary' | 'quiet' | 'danger'; busy?: boolean; ref?: Ref<HTMLButtonElement> };

export function Button({ variant = 'primary', busy, children, disabled, className, ref, ...rest }: ButtonProps) {
  return (
    <button
      ref={ref}
      type="button"
      className={`btn btn-${variant}${className ? ` ${className}` : ''}`}
      disabled={disabled || busy}
      aria-busy={busy || undefined}
      {...rest}
    >
      {children}
    </button>
  );
}

export function Card({ children, className, as: As = 'section', labelledBy }: { children: ReactNode; className?: string; as?: 'section' | 'article' | 'div'; labelledBy?: string }) {
  return (
    <As className={`card${className ? ` ${className}` : ''}`} aria-labelledby={labelledBy}>
      {children}
    </As>
  );
}

/** Notice is a polite status message; tone "alert" interrupts. */
export function Notice({ tone = 'info', children }: { tone?: 'info' | 'success' | 'warning' | 'alert'; children: ReactNode }) {
  return (
    <div className={`notice notice-${tone}`} role={tone === 'alert' ? 'alert' : 'status'}>
      {children}
    </div>
  );
}

export function errorCode(err: unknown): string {
  if (err instanceof ApiError) return err.code;
  return 'internal';
}

/** ErrorNotice localizes a stable server error code. */
export function ErrorNotice({ error, code }: { error?: unknown; code?: string | null }) {
  const { t, has } = useI18n();
  const c = code ?? (error !== undefined && error !== null ? errorCode(error) : null);
  if (!c) return null;
  const key = `error.${c}`;
  return <Notice tone="alert">{has(key) ? t(key) : t('error.internal')}</Notice>;
}

export function Badge({ children, tone = 'neutral' }: { children: ReactNode; tone?: 'neutral' | 'demo' | 'ok' | 'warn' | 'ai' | 'test' }) {
  return <span className={`badge badge-${tone}`}>{children}</span>;
}

export function ProvenanceBadge({ provenance }: { provenance: Provenance }) {
  const { t } = useI18n();
  const tone = provenance === 'test_double' ? 'test' : provenance === 'manual' ? 'neutral' : 'ai';
  return <Badge tone={tone}>{t(`provenance.${provenance}` as MessageKey)}</Badge>;
}

export function QualifierChips({ q }: { q: Qualifiers }) {
  const { t } = useI18n();
  const chips: MessageKey[] = [];
  if (q.negated) chips.push('ledger.q.negated');
  if (q.subject === 'other') chips.push('ledger.q.other');
  if (q.time === 'past') chips.push('ledger.q.past');
  if (q.time === 'future') chips.push('ledger.q.future');
  if (q.certainty === 'uncertain') chips.push('ledger.q.uncertain');
  if (q.certainty === 'hypothetical') chips.push('ledger.q.hypothetical');
  if (chips.length === 0) return null;
  return (
    <ul className="chips" aria-label={t('ledger.q.label')}>
      {chips.map((c) => (
        <li key={c} className="chip">
          {t(c)}
        </li>
      ))}
    </ul>
  );
}

export function CopyField({ label, value, mono }: { label: string; value: string; mono?: boolean }) {
  const { t } = useI18n();
  const id = useId();
  const [copied, setCopied] = useState(false);
  const copy = async () => {
    try {
      await navigator.clipboard.writeText(value);
      setCopied(true);
      window.setTimeout(() => setCopied(false), 2000);
    } catch {
      // Clipboard may be blocked; the field remains selectable.
    }
  };
  return (
    <div className="copy-field">
      <label htmlFor={id}>{label}</label>
      <div className="copy-row">
        <input id={id} readOnly value={value} className={mono ? 'mono' : undefined} onFocus={(e) => e.currentTarget.select()} />
        <Button variant="secondary" onClick={copy} aria-live="polite">
          {copied ? t('app.copied') : t('app.copy')}
        </Button>
      </div>
    </div>
  );
}

/** ConfirmButton asks for an inline second confirmation for destructive
 * actions. Focus never drops to the page: Cancel returns it to the trigger,
 * and so does confirming while the trigger is still shown (the caller moves
 * it to the result when the action removes the button). */
export function ConfirmButton({ label, confirmText, onConfirm, busy }: { label: string; confirmText: string; onConfirm: () => void; busy?: boolean }) {
  const { t } = useI18n();
  const [asking, setAsking] = useState(false);
  const trigger = useRef<HTMLButtonElement>(null);
  const refocus = useRef(false);
  useEffect(() => {
    if (!asking && refocus.current) {
      refocus.current = false;
      trigger.current?.focus();
    }
  }, [asking]);
  const close = () => {
    refocus.current = true;
    setAsking(false);
  };
  if (!asking) {
    return (
      // aria-disabled rather than disabled while busy: a disabled button
      // cannot hold the focus returned to it.
      <Button ref={trigger} variant="danger" onClick={() => { if (!busy) setAsking(true); }} aria-disabled={busy || undefined} aria-busy={busy || undefined}>
        {label}
      </Button>
    );
  }
  return (
    <div className="confirm" role="group" aria-label={label}>
      <p>{confirmText}</p>
      <div className="row">
        <Button variant="danger" onClick={() => { close(); onConfirm(); }} autoFocus>
          {label}
        </Button>
        <Button variant="quiet" onClick={close}>
          {t('app.cancel')}
        </Button>
      </div>
    </div>
  );
}

export function PageHeader({ title, lead }: { title: string; lead?: string }) {
  return (
    <header className="page-header">
      <h1>{title}</h1>
      {lead ? <p className="lead">{lead}</p> : null}
    </header>
  );
}

export function Spinner({ label }: { label: string }) {
  return (
    <p className="spinner" role="status">
      <span className="spinner-dot" aria-hidden="true" />
      {label}
    </p>
  );
}

type RadioOption = { value: string; label: string; hint?: string };

export function RadioGroup({ legend, name, value, options, onChange }: { legend: string; name: string; value: string; options: RadioOption[]; onChange: (v: string) => void }) {
  return (
    <fieldset className="radio-group">
      <legend>{legend}</legend>
      {options.map((o) => {
        const id = `${name}-${o.value}`;
        return (
          <div key={o.value} className="radio">
            <input type="radio" id={id} name={name} value={o.value} checked={value === o.value} onChange={() => onChange(o.value)} />
            <label htmlFor={id}>
              {o.label}
              {o.hint ? <span className="hint">{o.hint}</span> : null}
            </label>
          </div>
        );
      })}
    </fieldset>
  );
}

export function Select({ label, value, options, onChange, id }: { label: string; value: string; options: RadioOption[]; onChange: (v: string) => void; id: string }) {
  return (
    <div className="field">
      <label htmlFor={id}>{label}</label>
      <select id={id} value={value} onChange={(e) => onChange(e.target.value)}>
        {options.map((o) => (
          <option key={o.value} value={o.value}>
            {o.label}
          </option>
        ))}
      </select>
    </div>
  );
}
