import {
  useEffect,
  useId,
  useRef,
  useState,
  type ButtonHTMLAttributes,
  type InputHTMLAttributes,
  type ReactNode,
  type SelectHTMLAttributes,
  type TextareaHTMLAttributes,
} from "react";
import { Link } from "../lib/router";
import { useI18n } from "../i18n";
import { V3Error } from "./api";
import { errorCopy } from "./errors";
import { Icon, Mark, type IconName } from "./icons";
import { useWords } from "./shared";

/* ------------------------------------------------------------------ */
/* Headings, links and status                                          */
/* ------------------------------------------------------------------ */

export function PageHeader({
  title,
  lead,
  back,
  actions,
}: {
  title: string;
  lead?: string;
  back?: { to: string; label: string };
  actions?: ReactNode;
}) {
  return (
    <header className="s-page-header">
      {back && (
        <Link to={back.to} className="s-back">
          <Icon name="arrow-left" size={18} />
          {back.label}
        </Link>
      )}
      <h1 id="page-title">{title}</h1>
      {lead && <p className="s-lead">{lead}</p>}
      {actions && <div className="s-page-actions">{actions}</div>}
    </header>
  );
}

export function StatusLine({
  children,
  tone = "neutral",
  id,
}: {
  children: ReactNode;
  tone?: "neutral" | "success" | "warning";
  id?: string;
}) {
  return (
    <p id={id} className={`s-status s-status--${tone}`} role="status">
      {children}
    </p>
  );
}

/* ------------------------------------------------------------------ */
/* Buttons                                                             */
/* ------------------------------------------------------------------ */

export type ButtonVariant =
  | "primary"
  | "secondary"
  | "quiet"
  | "danger"
  | "confirm"
  | "help"
  | "help-quiet"
  | "icon";

export function buttonClass(variant: ButtonVariant = "secondary", extra = ""): string {
  return `s-btn s-btn--${variant}${extra ? " " + extra : ""}`;
}

type ButtonProps = ButtonHTMLAttributes<HTMLButtonElement> & {
  variant?: ButtonVariant;
  icon?: IconName;
  busy?: boolean;
};

export function Button({
  variant = "secondary",
  icon,
  busy,
  children,
  className,
  type = "button",
  ...rest
}: ButtonProps) {
  return (
    <button
      {...rest}
      type={type}
      className={buttonClass(variant, className ?? "")}
      aria-busy={busy || undefined}
    >
      {icon && <Icon name={icon} />}
      <span>{children}</span>
    </button>
  );
}

export function LinkButton({
  to,
  variant = "secondary",
  icon,
  children,
  className,
  ...rest
}: {
  to: string;
  variant?: ButtonVariant;
  icon?: IconName;
  children: ReactNode;
  className?: string;
  "aria-current"?: "page" | undefined;
}) {
  return (
    <Link to={to} className={buttonClass(variant, className ?? "")} {...rest}>
      {icon && <Icon name={icon} />}
      <span>{children}</span>
    </Link>
  );
}

/* ------------------------------------------------------------------ */
/* Notices, errors and states                                          */
/* ------------------------------------------------------------------ */

export type NoticeTone = "info" | "success" | "warning" | "danger" | "help" | "demo" | "test";

const noticeIcon: Record<NoticeTone, IconName> = {
  info: "info",
  success: "check",
  warning: "alert",
  danger: "alert",
  help: "help",
  demo: "info",
  test: "info",
};

export function Notice({
  tone = "info",
  title,
  children,
  actions,
  id,
}: {
  tone?: NoticeTone;
  title?: string;
  children?: ReactNode;
  actions?: ReactNode;
  id?: string;
}) {
  return (
    <section className={`s-notice s-notice--${tone}`} id={id}>
      <div className="s-notice-head">
        <Icon name={noticeIcon[tone]} />
        <div>
          {title && <h2 className="s-notice-title">{title}</h2>}
          {children}
        </div>
      </div>
      {actions && <div className="s-actions">{actions}</div>}
    </section>
  );
}

export function ErrorNotice({
  error,
  title,
  onRetry,
  retryLabel,
  id,
}: {
  error?: unknown;
  title?: string;
  onRetry?: () => void;
  retryLabel?: string;
  id?: string;
}) {
  const w = useWords();
  const { lang } = useI18n();
  if (!error) return null;
  let text: string;
  let code = "";
  if (error instanceof V3Error) {
    text = errorCopy(error.code, lang, error.message);
    code = error.code;
  } else if (error instanceof Error) {
    text = error.message;
  } else {
    text = String(error);
  }
  return (
    <section className="s-notice s-notice--danger" role="alert" id={id}>
      <div className="s-notice-head">
        <Icon name="alert" />
        <div>
          <h2 className="s-notice-title">
            {title ?? w("Perlu diperbaiki", "Needs fixing")}
          </h2>
          <p>{text}</p>
          {code === "store_unavailable" && (
            <p className="s-muted">
              {w(
                "Bantuan sekarang tetap bisa dibuka tanpa penyimpanan.",
                "Help now still works without storage.",
              )}
            </p>
          )}
        </div>
      </div>
      {onRetry && (
        <div className="s-actions">
          <Button variant="secondary" onClick={onRetry}>
            {retryLabel ?? w("Coba lagi", "Try again")}
          </Button>
        </div>
      )}
    </section>
  );
}

export function Loading({ label }: { label: string }) {
  const [show, setShow] = useState(false);
  useEffect(() => {
    const t = window.setTimeout(() => setShow(true), 400);
    return () => window.clearTimeout(t);
  }, []);
  if (!show) return <div className="s-loading-space" aria-hidden="true" />;
  return (
    <p className="s-loading" role="status">
      {label}
    </p>
  );
}

export function EmptyState({
  title,
  body,
  actions,
  mark,
}: {
  title: string;
  body?: string;
  actions?: ReactNode;
  mark?: boolean;
}) {
  return (
    <div className="s-empty">
      {mark && (
        <span className="s-empty-mark" aria-hidden="true">
          <Mark size={30} />
        </span>
      )}
      <h2>{title}</h2>
      {body && <p>{body}</p>}
      {actions && <div className="s-actions">{actions}</div>}
    </div>
  );
}

export function UnavailableFeature({
  title,
  body,
  dependency,
}: {
  title: string;
  body: string;
  dependency?: string;
}) {
  const w = useWords();
  return (
    <Notice tone="info" title={title}>
      <p>{body}</p>
      {dependency && (
        <p className="s-muted s-small">
          {w("Ketergantungan teknis: ", "Technical dependency: ")}
          {dependency}
        </p>
      )}
      <p className="s-muted s-small">
        {w(
          "Tidak ada data palsu yang ditampilkan di sini. Fitur ini akan aktif saat layanannya tersedia.",
          "No fake data is shown here. This feature will become available when its service exists.",
        )}
      </p>
    </Notice>
  );
}

/* ------------------------------------------------------------------ */
/* Form fields                                                         */
/* ------------------------------------------------------------------ */

function joinIds(...ids: Array<string | false | undefined>): string | undefined {
  const list = ids.filter(Boolean) as string[];
  return list.length ? list.join(" ") : undefined;
}

const errorPrefix = (lang: string) => (lang === "en" ? "Needs fixing: " : "Perlu diperbaiki: ");

export function FieldError({ id, children }: { id: string; children: ReactNode }) {
  const w = useWords();
  return (
    <p className="s-field-error" id={id}>
      <Icon name="alert" size={18} />
      <span>
        <span className="s-sr-only">{w("Perlu diperbaiki: ", "Needs fixing: ")}</span>
        {children}
      </span>
    </p>
  );
}

type BaseField = {
  label: string;
  hint?: string;
  error?: string;
  id?: string;
  optional?: boolean;
};

export function TextField({
  label,
  hint,
  error,
  id,
  optional,
  type = "text",
  className,
  ...rest
}: BaseField & InputHTMLAttributes<HTMLInputElement>) {
  const auto = useId();
  const fieldId = id ?? auto;
  const hintId = hint ? `${fieldId}-hint` : undefined;
  const errorId = error ? `${fieldId}-error` : undefined;
  const w = useWords();
  return (
    <div className={`s-field ${className ?? ""}`}>
      <label htmlFor={fieldId} className="s-label">
        {label}
        {optional && (
          <span className="s-optional"> {w("(opsional)", "(optional)")}</span>
        )}
      </label>
      {hint && (
        <p className="s-hint" id={hintId}>
          {hint}
        </p>
      )}
      <input
        {...rest}
        id={fieldId}
        type={type}
        className="s-input"
        aria-invalid={error ? true : undefined}
        aria-describedby={joinIds(hintId, errorId)}
      />
      {error && <FieldError id={errorId!}>{error}</FieldError>}
    </div>
  );
}

export function PasswordField({
  label,
  hint,
  error,
  id,
  optional,
  autoComplete,
  ...rest
}: BaseField & InputHTMLAttributes<HTMLInputElement>) {
  const auto = useId();
  const fieldId = id ?? auto;
  const hintId = hint ? `${fieldId}-hint` : undefined;
  const errorId = error ? `${fieldId}-error` : undefined;
  const [show, setShow] = useState(false);
  const w = useWords();
  return (
    <div className="s-field">
      <label htmlFor={fieldId} className="s-label">
        {label}
        {optional && (
          <span className="s-optional"> {w("(opsional)", "(optional)")}</span>
        )}
      </label>
      {hint && (
        <p className="s-hint" id={hintId}>
          {hint}
        </p>
      )}
      <div className="s-password-row">
        <input
          {...rest}
          id={fieldId}
          type={show ? "text" : "password"}
          autoComplete={autoComplete}
          className="s-input"
          aria-invalid={error ? true : undefined}
          aria-describedby={joinIds(hintId, errorId)}
        />
        <button
          type="button"
          className={buttonClass("quiet", "s-show-password")}
          aria-pressed={show}
          onClick={() => setShow((s) => !s)}
        >
          {show ? w("Sembunyikan", "Hide") : w("Tampilkan", "Show")}
        </button>
      </div>
      {error && <FieldError id={errorId!}>{error}</FieldError>}
    </div>
  );
}

export function TextArea({
  label,
  hint,
  error,
  id,
  optional,
  hideLabel,
  describedBy,
  rows = 6,
  className,
  ...rest
}: BaseField & TextareaHTMLAttributes<HTMLTextAreaElement> & {
  hideLabel?: boolean;
  describedBy?: string;
}) {
  const auto = useId();
  const fieldId = id ?? auto;
  const hintId = hint ? `${fieldId}-hint` : undefined;
  const errorId = error ? `${fieldId}-error` : undefined;
  const w = useWords();
  return (
    <div className={`s-field ${className ?? ""}`}>
      <label htmlFor={fieldId} className={hideLabel ? "sr-only" : "s-label"}>
        {label}
        {optional && (
          <span className="s-optional"> {w("(opsional)", "(optional)")}</span>
        )}
      </label>
      {hint && (
        <p className="s-hint" id={hintId}>
          {hint}
        </p>
      )}
      <textarea
        {...rest}
        id={fieldId}
        rows={rows}
        className="s-textarea"
        aria-invalid={error ? true : undefined}
        aria-describedby={joinIds(describedBy, hintId, errorId)}
      />
      {error && <FieldError id={errorId!}>{error}</FieldError>}
    </div>
  );
}

export function SelectField({
  label,
  hint,
  error,
  id,
  optional,
  children,
  ...rest
}: BaseField & SelectHTMLAttributes<HTMLSelectElement>) {
  const auto = useId();
  const fieldId = id ?? auto;
  const hintId = hint ? `${fieldId}-hint` : undefined;
  const errorId = error ? `${fieldId}-error` : undefined;
  const w = useWords();
  return (
    <div className="s-field">
      <label htmlFor={fieldId} className="s-label">
        {label}
        {optional && (
          <span className="s-optional"> {w("(opsional)", "(optional)")}</span>
        )}
      </label>
      {hint && (
        <p className="s-hint" id={hintId}>
          {hint}
        </p>
      )}
      <select
        {...rest}
        id={fieldId}
        className="s-select"
        aria-invalid={error ? true : undefined}
        aria-describedby={joinIds(hintId, errorId)}
      >
        {children}
      </select>
      {error && <FieldError id={errorId!}>{error}</FieldError>}
    </div>
  );
}

export function CheckboxRow({
  checked,
  onChange,
  children,
  hint,
  error,
  id,
  disabled,
}: {
  checked: boolean;
  onChange: (checked: boolean) => void;
  children: ReactNode;
  hint?: string;
  error?: string;
  id?: string;
  disabled?: boolean;
}) {
  const auto = useId();
  const fieldId = id ?? auto;
  const hintId = hint ? `${fieldId}-hint` : undefined;
  const errorId = error ? `${fieldId}-error` : undefined;
  return (
    <div className="s-choice-row">
      <label className="s-check" htmlFor={fieldId}>
        <input
          id={fieldId}
          type="checkbox"
          checked={checked}
          disabled={disabled}
          onChange={(e) => onChange(e.target.checked)}
          aria-invalid={error ? true : undefined}
          aria-describedby={joinIds(hintId, errorId)}
        />
        <span className="s-check-box" aria-hidden="true">
          {checked && <Icon name="check" size={16} />}
        </span>
        <span className="s-choice-copy">
          <span>{children}</span>
          {hint && (
            <span className="s-hint" id={hintId}>
              {hint}
            </span>
          )}
        </span>
      </label>
      {error && <FieldError id={errorId!}>{error}</FieldError>}
    </div>
  );
}

/** A group of radio rows or chips with the group error in the legend. */
export function ChoiceGroup<T extends string>({
  legend,
  name,
  value,
  onChange,
  options,
  hint,
  error,
  variant = "rows",
  optional,
  columns,
}: {
  legend: string;
  name: string;
  value: T | null;
  onChange: (v: T) => void;
  options: Array<{
    value: T;
    label: string;
    hint?: string;
    disabled?: boolean;
  }>;
  hint?: string;
  error?: string;
  variant?: "rows" | "chips" | "tiles";
  optional?: boolean;
  columns?: number;
}) {
  const w = useWords();
  const { lang } = useI18n();
  const hintId = hint ? `${name}-hint` : undefined;
  const errorId = error ? `${name}-error` : undefined;
  const firstRef = useRef<HTMLInputElement>(null);
  useEffect(() => {
    if (error) firstRef.current?.focus();
  }, [error]);
  return (
    <fieldset
      className={`s-fieldset s-choice-group s-choice-group--${variant}`}
      role="radiogroup"
      aria-invalid={error ? true : undefined}
      aria-describedby={joinIds(hintId, errorId)}
      style={columns ? { ["--choice-columns" as string]: columns } : undefined}
    >
      <legend className="s-legend">
        {legend}
        {optional && (
          <span className="s-optional"> {w("(boleh dilewati)", "(optional)")}</span>
        )}
        {error && (
          <span className="s-legend-error" id={errorId}>
            <Icon name="alert" size={18} />
            {errorPrefix(lang)}
            {error}
          </span>
        )}
      </legend>
      {hint && (
        <p className="s-hint" id={hintId}>
          {hint}
        </p>
      )}
      <div className="s-choice-items">
        {options.map((o, i) => (
          <label
            key={o.value}
            className={`s-choice s-choice--${variant}${value === o.value ? " is-selected" : ""}`}
          >
            <input
              ref={i === 0 ? firstRef : undefined}
              type="radio"
              name={name}
              value={o.value}
              checked={value === o.value}
              disabled={o.disabled}
              onChange={() => onChange(o.value)}
            />
            {variant === "chips" && value === o.value && (
              <Icon name="check" size={16} />
            )}
            <span className="s-choice-text">
              <span>{o.label}</span>
              {o.hint && <span className="s-hint">{o.hint}</span>}
            </span>
          </label>
        ))}
      </div>
    </fieldset>
  );
}

export function Switch({
  checked,
  onChange,
  label,
  hint,
}: {
  checked: boolean;
  onChange: (v: boolean) => void;
  label: string;
  hint?: string;
}) {
  const id = useId();
  const w = useWords();
  return (
    <div className="s-switch-row">
      <button
        type="button"
        role="switch"
        id={id}
        aria-checked={checked}
        className="s-switch"
        onClick={() => onChange(!checked)}
      >
        <span className="s-switch-track" aria-hidden="true">
          <span className="s-switch-thumb" />
        </span>
        <span className="s-switch-label">{label}</span>
        <span className="s-switch-state" aria-hidden="true">
          {checked ? w("Aktif", "On") : w("Mati", "Off")}
        </span>
      </button>
      {hint && <p className="s-hint">{hint}</p>}
    </div>
  );
}

/* ------------------------------------------------------------------ */
/* Dialogs, sheets and confirmation                                     */
/* ------------------------------------------------------------------ */

export function Dialog({
  open,
  onClose,
  title,
  children,
  className,
  sheet = false,
}: {
  open: boolean;
  onClose: () => void;
  title: string;
  children: ReactNode;
  className?: string;
  sheet?: boolean;
}) {
  const w = useWords();
  const ref = useRef<HTMLDialogElement>(null);
  const heading = useRef<HTMLHeadingElement>(null);
  const returnTo = useRef<HTMLElement | null>(null);

  useEffect(() => {
    const el = ref.current;
    if (!el) return;
    if (open && !el.open) {
      returnTo.current =
        document.activeElement instanceof HTMLElement ? document.activeElement : null;
      el.showModal();
      requestAnimationFrame(() => {
        if (sheet) document.getElementById("dialog-help")?.focus();
        else heading.current?.focus();
      });
    } else if (!open && el.open) {
      el.close();
      returnTo.current?.focus();
    }
  }, [open, sheet]);

  return (
    <dialog
      ref={ref}
      className={`s-dialog ${className ?? ""}`}
      aria-labelledby="s-dialog-title"
      onCancel={(e) => {
        e.preventDefault();
        onClose();
      }}
      onClose={() => {
        if (open) onClose();
      }}
      onClick={(e) => {
        if (e.target === ref.current) onClose();
      }}
    >
      <div className="s-dialog-inner">
        {sheet && (
          <div className="s-menu-safety">
            <Link to="/help" id="dialog-help" className={buttonClass("help-quiet")} onClick={onClose}>
              <Icon name="help" />
              <span>{w("Butuh bantuan sekarang", "Need help now")}</span>
            </Link>
            <button
              type="button"
              className={buttonClass("quiet")}
              onClick={() => window.dispatchEvent(new Event("sela:request-quick-exit"))}
            >
              <Icon name="quick-exit" />
              <span>{w("Keluar cepat", "Quick exit")}</span>
            </button>
            <button type="button" className={buttonClass("quiet")} onClick={onClose}>
              <Icon name="close" />
              <span>{w("Tutup", "Close")}</span>
            </button>
          </div>
        )}
        <div className="s-dialog-head">
          <h2 id="s-dialog-title" ref={heading} tabIndex={-1}>
            {title}
          </h2>
          <button
            type="button"
            className={buttonClass("quiet", "s-dialog-close")}
            onClick={onClose}
            aria-label={w("Tutup", "Close")}
          >
            <Icon name="close" />
          </button>
        </div>
        {children}
      </div>
    </dialog>
  );
}

export function InlineConfirm({
  label,
  question,
  confirmLabel,
  cancelLabel,
  onConfirm,
  icon = "trash",
  variant = "danger",
  secondaryLabel,
  onSecondary,
  busy,
}: {
  label: string;
  question: string;
  confirmLabel: string;
  cancelLabel?: string;
  onConfirm: () => void;
  icon?: IconName;
  variant?: "danger" | "confirm";
  secondaryLabel?: string;
  onSecondary?: () => void;
  busy?: boolean;
}) {
  const w = useWords();
  const [open, setOpen] = useState(false);
  const q = useRef<HTMLParagraphElement>(null);
  const trigger = useRef<HTMLButtonElement>(null);
  useEffect(() => {
    if (open) requestAnimationFrame(() => q.current?.focus());
  }, [open]);
  if (!open) {
    return (
      <button
        ref={trigger}
        type="button"
        className={buttonClass(variant)}
        onClick={() => setOpen(true)}
      >
        <Icon name={icon} />
        <span>{label}</span>
      </button>
    );
  }
  return (
    <div className="s-confirm" role="group" aria-label={label}>
      <p ref={q} tabIndex={-1} className="s-confirm-question">
        {question}
      </p>
      <div className="s-actions">
        <Button variant="confirm" icon={icon} onClick={onConfirm} disabled={busy}>
          {confirmLabel}
        </Button>
        {secondaryLabel && onSecondary && (
          <Button variant="confirm" icon={icon} onClick={onSecondary} disabled={busy}>
            {secondaryLabel}
          </Button>
        )}
        <Button
          variant="quiet"
          onClick={() => {
            setOpen(false);
            requestAnimationFrame(() => trigger.current?.focus());
          }}
        >
          {cancelLabel ?? w("Batal", "Cancel")}
        </Button>
      </div>
    </div>
  );
}

/* ------------------------------------------------------------------ */
/* Provenance                                                          */
/* ------------------------------------------------------------------ */

export type ProvenanceClass =
  | "user_original"
  | "ai_summary"
  | "user_correction"
  | "unconfirmed_interpretation"
  | "uploaded_document"
  | "verified_professional_note";

const provenanceCopy: Record<
  ProvenanceClass,
  { id: string; en: string; icon: IconName }
> = {
  user_original: { id: "Kata-katamu", en: "Your words", icon: "quote" },
  ai_summary: { id: "Tulisan AI", en: "AI wording", icon: "ai" },
  user_correction: { id: "Koreksimu", en: "Your correction", icon: "pencil" },
  unconfirmed_interpretation: {
    id: "Tafsiran Sela",
    en: "Sela's interpretation",
    icon: "interpretation",
  },
  uploaded_document: {
    id: "Dokumen unggahan",
    en: "Uploaded document",
    icon: "copy",
  },
  verified_professional_note: {
    id: "Catatan profesional terverifikasi",
    en: "Verified professional note",
    icon: "shield",
  },
};

export function ProvenanceBadge({ value }: { value: string }) {
  const w = useWords();
  if (value === "verified_professional_note" || value === "uploaded_document") {
    // Reserved classes are never rendered in the pilot.
    return null;
  }
  const c = provenanceCopy[value as ProvenanceClass] ?? provenanceCopy.user_original;
  return (
    <span className={`s-badge s-badge--${value}`}>
      <Icon name={c.icon} size={16} />
      {w(c.id, c.en)}
    </span>
  );
}

export function ReviewStatusChip({ value }: { value: string }) {
  const w = useWords();
  const map: Record<string, [string, string, IconName]> = {
    unreviewed: ["Belum ditinjau", "Not reviewed", "interpretation"],
    confirmed: ["Dikonfirmasi", "Confirmed", "check"],
    corrected: ["Dikoreksi", "Corrected", "pencil"],
    rejected: ["Ditolak", "Rejected", "ban"],
  };
  const entry = map[value];
  if (!entry) return null;
  return (
    <span className={`s-status-chip s-status-chip--${value}`}>
      <Icon name={entry[2]} size={16} />
      {w(entry[0], entry[1])}
    </span>
  );
}

export function GeneratorLine({
  provider,
  checked,
}: {
  provider?: string;
  checked?: boolean;
}) {
  const w = useWords();
  const name =
    provider === "deepseek"
      ? w("DeepSeek, cadangan", "DeepSeek, backup")
      : provider === "test_double"
        ? w("program uji (bukan AI)", "a test program (not AI)")
        : "Meta";
  return (
    <p className="s-generator">
      {provider === "test_double"
        ? w("Ditulis oleh program uji (bukan AI)", "Written by a test program (not AI)")
        : w("Ditulis dengan bantuan AI", "Written with AI help") +
          ` (${name})`}
      {checked &&
        w(" · diperiksa oleh Claude", " · checked by Claude")}
    </p>
  );
}

/* ------------------------------------------------------------------ */
/* Tabs, lists and misc                                               */
/* ------------------------------------------------------------------ */

export function Subnav({
  label,
  items,
  current,
}: {
  label: string;
  items: Array<{ to: string; label: string }>;
  current: string;
}) {
  return (
    <nav className="s-subnav" aria-label={label}>
      {items.map((i) => (
        <Link
          key={i.to}
          to={i.to}
          className="s-subnav-link"
          aria-current={i.to === current ? "page" : undefined}
        >
          {i.label}
        </Link>
      ))}
    </nav>
  );
}

export function DefinitionList({
  items,
}: {
  items: Array<{ term: string; detail: ReactNode }>;
}) {
  return (
    <dl className="s-dl">
      {items.map((i) => (
        <div key={i.term}>
          <dt>{i.term}</dt>
          <dd>{i.detail}</dd>
        </div>
      ))}
    </dl>
  );
}

export function Divider() {
  return <hr className="s-divider" />;
}

export function ExternalLink({
  href,
  children,
}: {
  href: string;
  children: ReactNode;
}) {
  return (
    <a href={href} target="_blank" rel="noreferrer noopener">
      {children}
      <Icon name="external" size={16} />
    </a>
  );
}
