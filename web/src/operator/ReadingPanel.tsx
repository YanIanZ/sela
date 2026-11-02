import {
  useCallback,
  useEffect,
  useId,
  useMemo,
  useRef,
  useState,
  type FormEvent,
  type ReactNode,
} from "react";
import { useI18n } from "../i18n";
import { useWords } from "../v3/shared";
import {
  AdminError,
  adminApi,
  type Admin,
  type ReadingA11yCheck,
  type ReadingA11yFinding,
  type ReadingA11yMeta,
  type ReadingA11yReport,
  type ReadingAdminItem,
  type ReadingAudit,
  type ReadingLocalized,
} from "./api";
import { canReviewReading, OperatorHeader } from "./chrome";
import "./reading.css";

type Words = (id: string, en: string) => string;

const EMBEDDED_STATUS: Record<string, [string, string]> = {
  human_approved: ["Disetujui manusia", "Human approved"],
  human_changes_requested: ["Perubahan diminta", "Changes requested"],
  machine_drafted_unreviewed: ["Draf mesin, belum ditinjau", "Machine draft, unreviewed"],
  not_reviewed: ["Belum ditinjau", "Not reviewed"],
};

const ACTION_LABEL: Record<string, [string, string]> = {
  "reading.approve": ["Bacaan disetujui", "Reading approved"],
  "reading.revoke": ["Publikasi bacaan diakhiri", "Reading publication ended"],
  "reading.a11y_override": [
    "Pengecualian keterbacaan dicatat",
    "Readability exception recorded",
  ],
};

function label(map: Record<string, [string, string]>, key: string, w: Words): string {
  const value = map[key];
  return value ? w(value[0], value[1]) : key;
}

function readingError(err: unknown, w: Words): string {
  if (err instanceof AdminError) {
    switch (err.code) {
      case "admin_forbidden":
      case "reading_forbidden":
        return w(
          "Peranmu tidak memiliki izin untuk meninjau bacaan.",
          "Your role isn't allowed to review reading content.",
        );
      case "reading_content_changed":
        return w(
          "Teks berubah sejak pratinjau dibuka. Muat ulang lalu tinjau teks terbaru; tidak ada yang disetujui.",
          "The text changed since this preview was opened. Reload and review the current text; nothing was approved.",
        );
      case "confirmation_required":
        return w(
          "Centang konfirmasi bahwa kamu membaca teks persis ini.",
          "Tick the confirmation that you read this exact text.",
        );
      case "reading_not_approved":
        return w(
          "Bacaan ini tidak punya persetujuan aktif untuk diakhiri.",
          "This item has no current approval to end.",
        );
      case "accessibility_checks_failed":
        return w(
          "Pemeriksaan aksesibilitas wajib gagal. Teks perlu diperbaiki di berkas konten; tidak ada yang disetujui.",
          "A required accessibility check failed. The text must be fixed in the content file; nothing was approved.",
        );
      case "accessibility_checklist_incomplete":
        return w(
          "Centang setiap butir daftar periksa aksesibilitas manual.",
          "Tick every item of the manual accessibility checklist.",
        );
      case "accessibility_override_required":
        return w(
          "Tulis alasan pengecualian keterbacaan, 20 sampai 300 karakter.",
          "Give a reason for the readability exception, 20 to 300 characters.",
        );
      case "accessibility_checks_changed":
        return w(
          "Aturan pemeriksaan aksesibilitas berubah sejak halaman dimuat. Muat ulang lalu tinjau hasil terbaru; tidak ada yang disetujui.",
          "The accessibility checks changed since this page loaded. Reload and review the current results; nothing was approved.",
        );
      case "invalid_input":
        if (err.field === "accessibility.overrideReason") {
          return w(
            "Semua pemeriksaan otomatis lulus, jadi alasan pengecualian tidak dipakai. Kosongkan kolom alasan.",
            "Every automatic check passes, so no exception reason is used. Clear the reason field.",
          );
        }
        return w(
          "Periksa nama peninjau dan tanggal tinjauan (bukan tanggal yang akan datang).",
          "Check the reviewer name and review date (it cannot be in the future).",
        );
      case "store_unavailable":
        return w(
          "Penyimpanan tinjauan tidak tersedia di server ini.",
          "The review store isn't available on this server.",
        );
      case "network":
        return w(
          "Tidak dapat menghubungi server. Periksa koneksimu.",
          "Can't reach the server. Check your connection.",
        );
    }
  }
  return w(
    "Tindakan tinjauan bacaan gagal. Coba lagi.",
    "The reading review action failed. Try again.",
  );
}

function useDateFormatter() {
  const { lang } = useI18n();
  return useMemo(() => {
    const locale = lang === "en" ? "en-GB" : "id-ID";
    return (iso?: string) => {
      if (!iso) return "";
      const d = new Date(iso);
      return Number.isNaN(d.getTime())
        ? iso
        : new Intl.DateTimeFormat(locale, {
            dateStyle: "medium",
            timeStyle: "short",
          }).format(d);
    };
  }, [lang]);
}

function shortHash(hash: string): string {
  return hash.length > 16 ? `${hash.slice(0, 16)}…` : hash;
}

type ApproveFields = {
  reviewerName: string;
  reviewedOn: string;
  note: string;
  manualChecks: Record<string, boolean>;
  overrideReason: string;
};

/** An approve refusal shown inside the form, next to the fields it names. */
type FormError = { slug: string; message: string; field: string };

export function ReadingPage({
  admin,
  onSignOut,
  onExpired,
  embedded = false,
}: {
  admin: Admin;
  onSignOut: () => void;
  onExpired: () => void;
  /** embedded renders the page body inside the operator shell. */
  embedded?: boolean;
}) {
  const w = useWords();
  const { lang } = useI18n();
  const allowed = canReviewReading(admin);
  const [items, setItems] = useState<ReadingAdminItem[] | null>(null);
  const [meta, setMeta] = useState<ReadingA11yMeta | null>(null);
  const [formError, setFormError] = useState<FormError | null>(null);
  const [audit, setAudit] = useState<ReadingAudit[] | null>(null);
  const [loading, setLoading] = useState(allowed);
  const [busy, setBusy] = useState("");
  const [error, setError] = useState("");
  const [notice, setNotice] = useState("");
  const [signingOut, setSigningOut] = useState(false);
  const titleRef = useRef<HTMLHeadingElement>(null);

  useEffect(() => {
    document.title =
      lang === "en" ? "Sela Reading reviews" : "Tinjauan bacaan Sela";
  }, [lang]);

  useEffect(() => {
    titleRef.current?.focus();
  }, []);

  const handleError = useCallback(
    (err: unknown) => {
      if (err instanceof AdminError && err.status === 401) {
        onExpired();
        return;
      }
      setError(readingError(err, w));
    },
    [onExpired, w],
  );

  const reload = useCallback(async () => {
    if (!allowed) return;
    const [library, events] = await Promise.all([
      adminApi.reading(),
      adminApi.readingAudit(),
    ]);
    setItems(library.items);
    setMeta(library.accessibility);
    setAudit(events);
  }, [allowed]);

  const load = useCallback(async () => {
    if (!allowed) return;
    setLoading(true);
    setError("");
    try {
      await reload();
    } catch (err) {
      handleError(err);
    } finally {
      setLoading(false);
    }
  }, [allowed, handleError, reload]);

  useEffect(() => {
    void load();
  }, [load]);

  const signOut = async () => {
    setSigningOut(true);
    try {
      await adminApi.logout();
      onSignOut();
    } catch (err) {
      if (err instanceof AdminError && err.status === 401) {
        onSignOut();
      } else {
        setError(readingError(err, w));
      }
    } finally {
      setSigningOut(false);
    }
  };

  const approve = async (
    item: ReadingAdminItem,
    fields: ApproveFields,
  ): Promise<boolean> => {
    const slug = item.detail.id;
    if (!meta) return false;
    setBusy(`approve:${slug}`);
    setError("");
    setNotice("");
    setFormError(null);
    try {
      await adminApi.approveReading(slug, {
        reviewerName: fields.reviewerName,
        reviewedOn: fields.reviewedOn,
        note: fields.note,
        verifierStatus: "supported",
        contentHash: item.contentHash,
        confirm: true,
        accessibility: {
          checksVersion: meta.checksVersion,
          manualVersion: meta.manualVersion,
          manualChecks: fields.manualChecks,
          overrideReason: fields.overrideReason,
        },
      });
      setNotice(
        w(
          `${slug} disetujui. Publik hanya menyajikan teks dengan hash ini; perubahan teks menarik persetujuan.`,
          `${slug} is approved. The public library serves only text with this hash; a text change withdraws the approval.`,
        ),
      );
      await reload();
      return true;
    } catch (err) {
      if (err instanceof AdminError && err.status !== 401) {
        // Shown in the form, beside the fields it concerns, not at the top
        // of a long page where a keyboard user would never see it.
        setFormError({
          slug,
          message: readingError(err, w),
          field: err.field,
        });
      } else {
        handleError(err);
      }
      return false;
    } finally {
      setBusy("");
    }
  };

  const revoke = async (
    item: ReadingAdminItem,
    reason: string,
  ): Promise<boolean> => {
    const slug = item.detail.id;
    setBusy(`revoke:${slug}`);
    setError("");
    setNotice("");
    try {
      await adminApi.revokeReading(slug, reason);
      setNotice(
        w(
          `${slug} tidak lagi disajikan publik. Tindakan ini dicatat di audit.`,
          `${slug} is no longer publicly served. The action is recorded in the audit.`,
        ),
      );
      await reload();
      return true;
    } catch (err) {
      handleError(err);
      return false;
    } finally {
      setBusy("");
    }
  };

  const body = (
    <>
        <div className="op-intro">
          <div>
            <h1 ref={titleRef} tabIndex={-1} className="op-page-title">
              {w("Tinjauan bacaan", "Reading reviews")}
            </h1>
            <p>
              {w(
                "Pustaka publik hanya menyajikan bacaan yang disetujui manusia untuk teks persis ini, atau yang sudah disetujui di berkas konten. Setiap perubahan teks menarik persetujuan sampai ditinjau ulang.",
                "The public library serves only items a human approved for this exact text, or items already approved in the content file. Any text change withdraws the approval until it is reviewed again.",
              )}
            </p>
          </div>
          {allowed && (
            <button
              type="button"
              className="op-btn op-btn--quiet"
              onClick={() => void load()}
              disabled={loading}
              aria-busy={loading || undefined}
            >
              {loading ? w("Memuat…", "Loading…") : w("Muat ulang", "Refresh")}
            </button>
          )}
        </div>

        {/* The live regions stay in the DOM so screen readers register
            them before a message arrives; an inserted region is often
            not announced. */}
        <p className={error ? "op-alert" : "op-sr-only"} role="alert">
          {error}
        </p>
        <p className={notice ? "op-notice" : "op-sr-only"} role="status">
          {notice}
        </p>

        {!allowed ? (
          <section className="op-section" aria-labelledby="op-reading-denied">
            <h2 id="op-reading-denied">{w("Bacaan", "Reading")}</h2>
            <p className="op-hint">
              {w(
                "Peranmu tidak memiliki izin research/review atau safety-content/review.",
                "Your role doesn't hold research/review or safety-content/review.",
              )}
            </p>
          </section>
        ) : items === null || meta === null ? (
          <section className="op-section" aria-labelledby="op-reading-loading">
            <h2 id="op-reading-loading">{w("Bacaan", "Reading")}</h2>
            <p className="op-hint">
              {w("Memuat pustaka…", "Loading the library…")}
            </p>
          </section>
        ) : (
          <ReadingWorkspace
            items={items}
            meta={meta}
            busy={busy}
            formError={formError}
            onApprove={approve}
            onRevoke={revoke}
          />
        )}

        {allowed && (
          <section className="op-section" aria-labelledby="op-reading-audit">
            <h2 id="op-reading-audit">{w("Audit tinjauan", "Review audit")}</h2>
            <p>
              {w(
                "Hanya tindakan persetujuan dan penghentian publikasi. Tidak ada teks pengguna atau catatan pribadi.",
                "Approve and revoke actions only. No user text or personal notes.",
              )}
            </p>
            {audit === null ? (
              <p className="op-hint">{w("Memuat audit…", "Loading audit…")}</p>
            ) : audit.length === 0 ? (
              <p className="op-hint">
                {w("Belum ada tindakan tinjauan.", "No review actions yet.")}
              </p>
            ) : (
              <ol className="op-events">
                {audit.map((event) => (
                  <li key={event.id}>
                    <div className="op-event-head">
                      <strong>{label(ACTION_LABEL, event.action, w)}</strong>
                      <time dateTime={event.createdAt}>
                        <AuditTime iso={event.createdAt} />
                      </time>
                    </div>
                    <p className="op-event-meta">
                      {event.actor || w("Sistem", "System")}
                      {` · ${event.objectId}`}
                      {event.oldStatus || event.newStatus
                        ? ` · ${[event.oldStatus, event.newStatus].filter(Boolean).join(" → ")}`
                        : ""}
                    </p>
                    {event.reason && (
                      <p className="op-event-reason">{event.reason}</p>
                    )}
                  </li>
                ))}
              </ol>
            )}
          </section>
        )}

        <p className="op-foot">
          {w(
            "Tinjauan bacaan operator. Persetujuan mengikat teks persis yang ditampilkan; tidak ada persetujuan otomatis.",
            "Operator reading review. An approval binds the exact text shown; nothing is approved automatically.",
          )}
        </p>
    </>
  );
  if (embedded) return body;
  return (
    <div className="op">
      <a className="op-skip" href="#op-main">
        {w("Lewati ke konten", "Skip to content")}
      </a>
      <OperatorHeader
        admin={admin}
        active="reading"
        signingOut={signingOut}
        onSignOut={() => void signOut()}
      />
      <main className="op-main" id="op-main">
        {body}
      </main>
    </div>
  );
}

function AuditTime({ iso }: { iso: string }) {
  const date = useDateFormatter();
  return <>{date(iso)}</>;
}

function ReadingWorkspace({
  items,
  meta,
  busy,
  formError,
  onApprove,
  onRevoke,
}: {
  items: ReadingAdminItem[];
  meta: ReadingA11yMeta;
  busy: string;
  formError: FormError | null;
  onApprove: (item: ReadingAdminItem, fields: ApproveFields) => Promise<boolean>;
  onRevoke: (item: ReadingAdminItem, reason: string) => Promise<boolean>;
}) {
  const w = useWords();
  const { lang } = useI18n();
  const [selected, setSelected] = useState("");
  const current = items.find((item) => item.detail.id === selected);

  return (
    <section className="op-section" aria-labelledby="op-reading-library">
      <h2 id="op-reading-library">
        {w("Pustaka bacaan", "Reading library")}{" "}
        <span className="op-hint">
          {w(`${items.length} butir`, `${items.length} items`)}
        </span>
      </h2>
      <p>
        {w(
          "Pilih satu butir untuk membaca teks lengkap, sumber, lisensi, dan catatan tinjauan sebelum menyetujui.",
          "Choose one item to read the full text, source, licence and review record before approving.",
        )}
      </p>
      <ul className="op-reading-list">
        {items.map((item) => {
          const state = readingState(item, w);
          const { id, title, version, kind } = item.detail;
          return (
            <li key={id}>
              <button
                type="button"
                className={`op-reading-row${selected === id ? " op-reading-row--open" : ""}`}
                aria-expanded={selected === id}
                aria-controls="op-reading-preview"
                onClick={() => setSelected(selected === id ? "" : id)}
              >
                <span className="op-reading-row-main">
                  <strong lang={lang}>
                    {lang === "en" ? title.en : title.id}
                  </strong>
                  <span className="op-hint" lang={lang === "en" ? "id" : "en"}>
                    {lang === "en" ? title.id : title.en}
                  </span>
                  <span className="op-hint">
                    {id} · v{version} · {kind}
                  </span>
                </span>
                <span className="op-reading-badges">
                  <span className={`op-reading-state op-reading-state--${state.tone}`}>
                    {state.label}
                  </span>
                  <span className="op-badge op-badge--soft">
                    {label(EMBEDDED_STATUS, item.embeddedReview.status, w)}
                  </span>
                  {item.approval && !item.hashMatches && (
                    <span className="op-reading-state op-reading-state--warning">
                      {w("Hash tidak cocok", "Hash mismatch")}
                    </span>
                  )}
                  <A11yBadge report={item.accessibility} />
                </span>
                <span className="op-hint">
                  {item.approval
                    ? `${item.approval.reviewerName} · ${item.approval.reviewedOn}`
                    : w("Belum ada persetujuan operator", "No operator approval")}
                </span>
              </button>
            </li>
          );
        })}
      </ul>

      {current ? (
        <ReadingPreview
          item={current}
          meta={meta}
          busy={busy}
          formError={formError?.slug === current.detail.id ? formError : null}
          onApprove={onApprove}
          onRevoke={onRevoke}
        />
      ) : (
        <p className="op-hint" id="op-reading-preview">
          {w(
            "Belum ada butir yang dibuka.",
            "No item is open yet.",
          )}
        </p>
      )}
    </section>
  );
}

function readingState(
  item: ReadingAdminItem,
  w: Words,
): { label: string; tone: "ok" | "warning" | "danger" | "muted" } {
  if (item.published) return { label: w("Terbit", "Published"), tone: "ok" };
  if (item.approval?.revokedAt) {
    return { label: w("Dicabut", "Revoked"), tone: "danger" };
  }
  if (item.approval && !item.hashMatches) {
    return {
      label: w("Persetujuan kedaluwarsa", "Approval stale"),
      tone: "warning",
    };
  }
  return { label: w("Tidak terbit", "Not published"), tone: "muted" };
}

function ReadingPreview({
  item,
  meta,
  busy,
  formError,
  onApprove,
  onRevoke,
}: {
  item: ReadingAdminItem;
  meta: ReadingA11yMeta;
  busy: string;
  formError: FormError | null;
  onApprove: (item: ReadingAdminItem, fields: ApproveFields) => Promise<boolean>;
  onRevoke: (item: ReadingAdminItem, reason: string) => Promise<boolean>;
}) {
  const w = useWords();
  const { lang } = useI18n();
  const detail = item.detail;
  const report = item.accessibility;
  const [reviewerName, setReviewerName] = useState("");
  const [reviewedOn, setReviewedOn] = useState("");
  const [note, setNote] = useState("");
  const [confirm, setConfirm] = useState(false);
  const [manual, setManual] = useState<Record<string, boolean>>({});
  const [overrideReason, setOverrideReason] = useState("");
  const [revoking, setRevoking] = useState(false);
  const [reason, setReason] = useState("");
  const nameId = useId();
  const dateId = useId();
  const noteId = useId();
  const confirmId = useId();
  const reasonId = useId();
  const manualId = useId();
  const overrideId = useId();
  const overrideHintId = useId();
  const formErrorId = useId();
  const blockedId = useId();
  const missingId = useId();
  const headingRef = useRef<HTMLHeadingElement>(null);
  const formErrorRef = useRef<HTMLParagraphElement>(null);
  const revokeButtonRef = useRef<HTMLButtonElement>(null);
  const reasonInputRef = useRef<HTMLInputElement>(null);

  useEffect(() => {
    setReviewerName("");
    setReviewedOn("");
    setNote("");
    setConfirm(false);
    setManual({});
    setOverrideReason("");
    setRevoking(false);
    setReason("");
  }, [detail.id, item.contentHash]);

  // Opening an item moves focus to its heading: the preview is rendered
  // after the whole list, so without this a keyboard or screen-reader user
  // would have to walk past every remaining row to reach it.
  useEffect(() => {
    headingRef.current?.focus();
  }, [detail.id]);

  useEffect(() => {
    if (formError) formErrorRef.current?.focus();
  }, [formError]);

  const currentApproval =
    item.approval && !item.approval.revokedAt ? item.approval : undefined;
  const approveBusy = busy === `approve:${detail.id}`;
  const revokeBusy = busy === `revoke:${detail.id}`;
  const blocked = report.result === "blocked";
  const needsOverride = report.result === "needs_override";
  const overrideLength = overrideReason.trim().length;
  const overrideOk =
    !needsOverride ||
    (overrideLength >= meta.overrideReason.min &&
      overrideLength <= meta.overrideReason.max);
  const manualDone = meta.manualChecklist.filter((c) => manual[c.id]).length;
  const manualOk = manualDone === meta.manualChecklist.length;
  const canSubmit =
    !blocked &&
    reviewerName.trim() !== "" &&
    reviewedOn !== "" &&
    manualOk &&
    overrideOk &&
    confirm &&
    !approveBusy;
  const overridable = report.checks.filter(
    (c) => c.status === "fail" && !c.blocking,
  );

  const missing: string[] = [];
  if (reviewerName.trim() === "") missing.push(w("nama peninjau", "reviewer name"));
  if (reviewedOn === "") missing.push(w("tanggal tinjauan", "review date"));
  if (!manualOk) {
    missing.push(
      w(
        `daftar periksa aksesibilitas (${manualDone} dari ${meta.manualChecklist.length})`,
        `accessibility checklist (${manualDone} of ${meta.manualChecklist.length})`,
      ),
    );
  }
  if (!overrideOk) missing.push(w("alasan pengecualian", "exception reason"));
  if (!confirm) missing.push(w("konfirmasi", "confirmation"));

  const fieldError = (field: string) =>
    formError && formError.field === field ? true : undefined;

  const submit = (e: FormEvent) => {
    e.preventDefault();
    // aria-disabled keeps the button focusable; the guard is here.
    if (!canSubmit) return;
    setConfirm(false);
    void onApprove(item, {
      reviewerName: reviewerName.trim(),
      reviewedOn,
      note: note.trim(),
      manualChecks: Object.fromEntries(
        meta.manualChecklist.map((c) => [c.id, Boolean(manual[c.id])]),
      ),
      overrideReason: needsOverride ? overrideReason.trim() : "",
    }).then((ok) => {
      if (ok) {
        setReviewerName("");
        setReviewedOn("");
        setNote("");
        setManual({});
        setOverrideReason("");
        // The form resets and the item now shows its approval; start the
        // reviewer at the item heading.
        headingRef.current?.focus();
      }
    });
  };

  return (
    <div className="op-reading-preview" id="op-reading-preview">
      <div className="op-reading-preview-head">
        <div>
          <h3 ref={headingRef} tabIndex={-1}>
            <span lang={lang}>{lang === "en" ? detail.title.en : detail.title.id}</span>
            <span className="op-hint" lang={lang === "en" ? "id" : "en"}>
              {lang === "en" ? detail.title.id : detail.title.en}
            </span>
          </h3>
          <p className="op-hint">
            {detail.id} · v{detail.version} · {detail.kind} ·{" "}
            {w(
              `${detail.readingTimeMinutes[lang === "en" ? "en" : "id"]} menit baca`,
              `${detail.readingTimeMinutes.en} min read`,
            )}
          </p>
        </div>
        <span className={`op-reading-state op-reading-state--${readingState(item, w).tone}`}>
          {readingState(item, w).label}
        </span>
      </div>

      <div className="op-reading-metadata">
        <Field term={w("Sumber", "Source")}>
          {detail.source.sources.length === 0 ? (
            <span className="op-hint">{w("Tidak ada sumber", "No source")}</span>
          ) : (
            <ul className="op-reading-sources">
              {detail.source.sources.map((source) => (
                <li key={source.id}>
                  <a href={source.url} target="_blank" rel="noreferrer">
                    {source.title}
                    <span className="op-sr-only">
                      {w(" (membuka tab baru)", " (opens in a new tab)")}
                    </span>
                  </a>
                  <span className="op-hint">
                    {` · ${source.publisher} · ${source.licence}`}
                  </span>
                </li>
              ))}
            </ul>
          )}
        </Field>
        <Field term={w("Relasi sumber", "Source relation")}>
          {detail.source.relation}
          {detail.source.locator ? ` · ${detail.source.locator}` : ""}
        </Field>
        <Field term={w("Lisensi teks", "Text licence")}>
          {detail.licence.itemTextLicence}
        </Field>
        <Field term={w("Lisensi sumber", "Source licence")}>
          {detail.licence.sourceLicence}
        </Field>
        <Field term={w("Penggunaan komersial", "Commercial use")}>
          {detail.licence.commercialUse}
        </Field>
        {detail.licence.note && (
          <Field term={w("Catatan lisensi", "Licence note")}>
            {detail.licence.note}
          </Field>
        )}
      </div>

      <div className="op-reading-body">
        <Bilingual label={w("Ringkasan", "Summary")} value={detail.summary} />
        <Bilingual label={w("Atribusi", "Attribution")} value={detail.attribution} />
        {detail.steps && (
          <Steps label={w("Langkah", "Steps")} steps={detail.steps} />
        )}
        {detail.why && <Bilingual label={w("Mengapa", "Why")} value={detail.why} />}
        {detail.seekHelpWhen && (
          <Bilingual
            label={w("Cari bantuan bila", "Seek help when")}
            value={detail.seekHelpWhen}
          />
        )}
        {detail.cautions && (
          <Bilingual label={w("Perhatian", "Cautions")} value={detail.cautions} />
        )}
        {detail.sections?.map((section, index) => (
          <section className="op-reading-section" key={`${section.heading.id}-${index}`}>
            <h4>
              <span lang="id">{section.heading.id}</span>
              <span className="op-hint" lang="en">
                {section.heading.en}
              </span>
            </h4>
            {section.paragraphs.map((paragraph, pIndex) => (
              <Bilingual
                key={`${section.heading.id}-${pIndex}`}
                label=""
                value={paragraph}
              />
            ))}
          </section>
        ))}
        {detail.adaptationNotice && (
          <Bilingual
            label={w("Pemberitahuan adaptasi", "Adaptation notice")}
            value={detail.adaptationNotice}
          />
        )}
        {detail.adaptationNote && (
          <Field term={w("Catatan adaptasi", "Adaptation note")}>
            {detail.adaptationNote}
          </Field>
        )}
        {detail.evidenceNote && (
          <Field term={w("Catatan bukti", "Evidence note")}>
            {detail.evidenceNote}
          </Field>
        )}
      </div>

      <A11yChecks report={report} />

      <div className="op-reading-review">
        <h4>{w("Catatan tinjauan di berkas konten", "Embedded review record")}</h4>
        <dl className="op-reading-dl">
          <dt>{w("Status", "Status")}</dt>
          <dd>{label(EMBEDDED_STATUS, item.embeddedReview.status, w)}</dd>
          <dt>{w("Editor manusia", "Human editor")}</dt>
          <dd>
            {item.embeddedReview.humanEditor || (
              <span className="op-hint">{w("Tidak ada", "None")}</span>
            )}
          </dd>
          <dt>{w("Tanggal tinjauan", "Reviewed on")}</dt>
          <dd>
            {item.embeddedReview.reviewedOn || (
              <span className="op-hint">{w("Tidak ada", "None")}</span>
            )}
          </dd>
          <dt>{w("Verifier", "Verifier")}</dt>
          <dd>
            {item.embeddedReview.verifierStatus}
            {item.embeddedReview.verifierNote
              ? ` · ${item.embeddedReview.verifierNote}`
              : ""}
          </dd>
          <dt>{w("Dapat diterbitkan", "Publishable")}</dt>
          <dd>{item.embeddedReview.publishable ? w("Ya", "Yes") : w("Tidak", "No")}</dd>
          <dt>{w("Tinjauan remaja", "Teen review")}</dt>
          <dd>{item.embeddedReview.teenReview || "—"}</dd>
          {item.embeddedReview.approvedTextSha256 && (
            <>
              <dt>{w("Hash teks disetujui (berkas)", "Approved text hash (file)")}</dt>
              <dd className="op-reading-hash">
                {item.embeddedReview.approvedTextSha256}
              </dd>
            </>
          )}
          <dt>{w("Hash konten saat ini", "Current content hash")}</dt>
          <dd className="op-reading-hash" title={item.contentHash}>
            {shortHash(item.contentHash)}
          </dd>
        </dl>
        <p className="op-hint">
          {w(
            "Persetujuan mengikat hash konten saat ini. Jika teks berubah, butir tidak disajikan sampai disetujui ulang.",
            "The approval binds the current content hash. If the text changes, the item is not served until it is approved again.",
          )}
        </p>
      </div>

      {item.approval && (
        <div className="op-reading-approved">
          <h4>{w("Persetujuan operator", "Operator approval")}</h4>
          <dl className="op-reading-dl">
            <dt>{w("Peninjau", "Reviewer")}</dt>
            <dd>{item.approval.reviewerName}</dd>
            <dt>{w("Tanggal tinjauan", "Reviewed on")}</dt>
            <dd>{item.approval.reviewedOn}</dd>
            <dt>{w("Verifier", "Verifier")}</dt>
            <dd>{item.approval.verifierStatus}</dd>
            <dt>{w("Disetujui", "Approved")}</dt>
            <dd>
              <AuditTime iso={item.approval.approvedAt} />
            </dd>
            <dt>{w("Operator", "Operator")}</dt>
            <dd className="op-reading-hash">{item.approval.approvedBy}</dd>
            <dt>{w("Versi", "Version")}</dt>
            <dd>{item.approval.version}</dd>
            <dt>{w("Status", "Status")}</dt>
            <dd>
              {item.approval.revokedAt ? (
                <span className="op-reading-state op-reading-state--danger">
                  {w("Dicabut", "Revoked")}
                </span>
              ) : item.hashMatches ? (
                <span className="op-reading-state op-reading-state--ok">
                  {w("Aktif untuk teks ini", "Current for this text")}
                </span>
              ) : (
                <span className="op-reading-state op-reading-state--warning">
                  {w("Kedaluwarsa: teks berubah", "Stale: text changed")}
                </span>
              )}
            </dd>
            {item.approval.note && (
              <>
                <dt>{w("Catatan", "Note")}</dt>
                <dd>{item.approval.note}</dd>
              </>
            )}
            <dt>{w("Aksesibilitas", "Accessibility")}</dt>
            <dd>
              {!item.approval.accessibility
                ? w(
                    "Tidak tercatat (disetujui sebelum pemeriksaan aksesibilitas wajib)",
                    "Not recorded (approved before accessibility checks were required)",
                  )
                : item.approval.accessibility.result === "overridden"
                  ? w(
                      `Dikecualikan dengan alasan · ${item.approval.accessibility.checksVersion}`,
                      `Excepted with a reason · ${item.approval.accessibility.checksVersion}`,
                    )
                  : w(
                      `Semua pemeriksaan lulus · ${item.approval.accessibility.checksVersion}`,
                      `All checks passed · ${item.approval.accessibility.checksVersion}`,
                    )}
            </dd>
            {item.approval.accessibility?.overrideReason && (
              <>
                <dt>{w("Alasan pengecualian", "Exception reason")}</dt>
                <dd>{item.approval.accessibility.overrideReason}</dd>
              </>
            )}
          </dl>
          {!item.approval.revokedAt && (
            <div className="op-reading-revoke">
              {revoking ? (
                <div className="op-confirm">
                  <p>
                    {w(
                      "Akhiri publikasi butir ini? Pustaka publik berhenti menyajikannya segera.",
                      "End publication of this item? The public library stops serving it immediately.",
                    )}
                  </p>
                  <label className="op-field" htmlFor={reasonId}>
                    <span>{w("Alasan (opsional)", "Reason (optional)")}</span>
                    <input
                      id={reasonId}
                      ref={reasonInputRef}
                      className="op-input"
                      value={reason}
                      maxLength={300}
                      onChange={(e) => setReason(e.target.value)}
                    />
                  </label>
                  <div className="op-confirm-actions">
                    <button
                      type="button"
                      className="op-btn op-btn--danger"
                      disabled={revokeBusy}
                      onClick={() => {
                        setRevoking(false);
                        void onRevoke(item, reason.trim()).then((ok) => {
                          // The revoke controls are gone once it succeeds;
                          // keep focus inside the preview.
                          if (ok) headingRef.current?.focus();
                          else revokeButtonRef.current?.focus();
                        });
                      }}
                    >
                      {revokeBusy
                        ? w("Mengakhiri…", "Ending…")
                        : w("Ya, akhiri publikasi", "Yes, end publication")}
                    </button>
                    <button
                      type="button"
                      className="op-btn op-btn--quiet"
                      onClick={() => {
                        setRevoking(false);
                        // Return focus to the control that opened the step.
                        requestAnimationFrame(() => revokeButtonRef.current?.focus());
                      }}
                    >
                      {w("Batal", "Cancel")}
                    </button>
                  </div>
                </div>
              ) : (
                <button
                  ref={revokeButtonRef}
                  type="button"
                  className="op-btn op-btn--quiet op-btn--danger-text"
                  onClick={() => {
                    setRevoking(true);
                    // The button is replaced by the confirmation; move focus
                    // into it instead of losing it to the page.
                    requestAnimationFrame(() => reasonInputRef.current?.focus());
                  }}
                >
                  {w("Akhiri publikasi", "End publication")}
                </button>
              )}
            </div>
          )}
        </div>
      )}

      <form className="op-reading-form" onSubmit={submit} noValidate>
        <h4>{w("Setujui teks ini", "Approve this text")}</h4>
        <p
          ref={formErrorRef}
          id={formErrorId}
          tabIndex={-1}
          role="alert"
          className={formError ? "op-alert" : "op-sr-only"}
        >
          {formError?.message ?? ""}
        </p>
        {blocked && (
          <p id={blockedId} className="op-reading-blocked">
            {w(
              "Butir ini tidak bisa disetujui: pemeriksaan aksesibilitas wajib gagal. Perbaiki teks di berkas konten, lalu tinjau ulang. Tidak ada pengecualian untuk pemeriksaan wajib.",
              "This item can't be approved: a required accessibility check failed. Fix the text in the content file, then review it again. Required checks have no exception.",
            )}
          </p>
        )}
        <p className="op-hint">
          {w(
            "Isi nama manusia yang benar-benar membaca teks persis di atas. Tidak ada nama yang diisikan otomatis.",
            "Enter the human who actually read the exact text above. Nothing is filled in automatically.",
          )}
        </p>
        <label className="op-field" htmlFor={nameId}>
          <span>{w("Nama peninjau", "Reviewer name")}</span>
          <input
            id={nameId}
            className="op-input"
            required
            autoComplete="off"
            maxLength={160}
            value={reviewerName}
            aria-invalid={fieldError("reviewerName")}
            aria-describedby={fieldError("reviewerName") && formErrorId}
            onChange={(e) => setReviewerName(e.target.value)}
          />
        </label>
        <label className="op-field" htmlFor={dateId}>
          <span>{w("Tanggal tinjauan", "Review date")}</span>
          <input
            id={dateId}
            className="op-input"
            type="date"
            required
            value={reviewedOn}
            aria-invalid={fieldError("reviewedOn")}
            aria-describedby={fieldError("reviewedOn") && formErrorId}
            onChange={(e) => setReviewedOn(e.target.value)}
          />
        </label>
        <label className="op-field" htmlFor={noteId}>
          <span>{w("Catatan (opsional)", "Note (optional)")}</span>
          <textarea
            id={noteId}
            className="op-input"
            rows={2}
            maxLength={1000}
            value={note}
            onChange={(e) => setNote(e.target.value)}
          />
        </label>
        <fieldset className="op-reading-checklist" aria-describedby={manualId}>
          <legend>
            {w(
              "Daftar periksa aksesibilitas manual (wajib)",
              "Manual accessibility checklist (required)",
            )}
          </legend>
          <p id={manualId} className="op-hint">
            {w(
              "Hal yang tidak bisa dinilai mesin. Centang hanya yang benar-benar kamu periksa pada teks di atas.",
              "Things a machine can't judge. Tick only what you actually checked in the text above.",
            )}
          </p>
          {meta.manualChecklist.map((check) => {
            const id = `${manualId}-${check.id}`;
            const invalid = fieldError(`accessibility.manualChecks.${check.id}`);
            return (
              <div className="op-reading-confirm" key={check.id}>
                <input
                  id={id}
                  type="checkbox"
                  checked={Boolean(manual[check.id])}
                  disabled={blocked}
                  aria-invalid={invalid}
                  aria-describedby={invalid && formErrorId}
                  onChange={(e) =>
                    setManual((prev) => ({ ...prev, [check.id]: e.target.checked }))
                  }
                />
                <label htmlFor={id} lang={lang === "en" ? "en" : "id"}>
                  {lang === "en" ? check.label.en : check.label.id}
                </label>
              </div>
            );
          })}
        </fieldset>
        {needsOverride && (
          <div className="op-field">
            <label htmlFor={overrideId}>
              {w(
                "Alasan pengecualian keterbacaan (wajib)",
                "Readability exception reason (required)",
              )}
            </label>
            <p id={overrideHintId} className="op-hint">
              {w(
                `Pemeriksaan yang tidak terpenuhi: ${overridable.map((c) => checkLabel(c.id, w)).join(", ")}. Jelaskan mengapa teks ini tetap mudah dibaca (${meta.overrideReason.min}–${meta.overrideReason.max} karakter). Alasan dicatat di audit atas nama akunmu.`,
                `Checks not met: ${overridable.map((c) => checkLabel(c.id, w)).join(", ")}. Explain why this text still reads easily (${meta.overrideReason.min}–${meta.overrideReason.max} characters). The reason is recorded in the audit against your account.`,
              )}{" "}
              <span>
                {w(
                  `${overrideLength} dari ${meta.overrideReason.max} karakter`,
                  `${overrideLength} of ${meta.overrideReason.max} characters`,
                )}
              </span>
            </p>
            <textarea
              id={overrideId}
              className="op-input"
              rows={3}
              maxLength={meta.overrideReason.max}
              value={overrideReason}
              aria-invalid={fieldError("accessibility.overrideReason")}
              aria-describedby={
                fieldError("accessibility.overrideReason")
                  ? `${overrideHintId} ${formErrorId}`
                  : overrideHintId
              }
              onChange={(e) => setOverrideReason(e.target.value)}
            />
          </div>
        )}
        <div className="op-reading-verifier">
          <span>{w("Status verifier tercatat", "Recorded verifier status")}</span>
          <strong>{"supported"}</strong>
          <span className="op-hint">
            {w(
              "Persetujuan ini menyatakan teks dan bukti verifier sudah diperiksa manusia.",
              "This approval asserts a human checked the text and the verifier evidence.",
            )}
          </span>
        </div>
        <div className="op-reading-confirm">
          <input
            id={confirmId}
            type="checkbox"
            checked={confirm}
            onChange={(e) => setConfirm(e.target.checked)}
          />
          <label htmlFor={confirmId}>
            {w(
              "Saya sudah membaca teks persis ini dan mengonfirmasi bukti verifier mendukungnya. Saya memahami persetujuan ini dicatat atas nama saya.",
              "I have read this exact text and confirm the verifier evidence supports it. I understand this approval is recorded against my operator account.",
            )}
          </label>
        </div>
        {!blocked && missing.length > 0 && !approveBusy && (
          <p id={missingId} className="op-hint">
            {w(
              `Belum bisa disetujui. Masih perlu: ${missing.join(", ")}.`,
              `Not ready to approve. Still needed: ${missing.join(", ")}.`,
            )}
          </p>
        )}
        <div className="op-actions">
          <button
            type="submit"
            className="op-btn op-btn--primary"
            aria-disabled={!canSubmit || undefined}
            aria-busy={approveBusy || undefined}
            aria-describedby={
              blocked ? blockedId : missing.length > 0 && !approveBusy ? missingId : undefined
            }
          >
            {approveBusy
              ? w("Menyetujui…", "Approving…")
              : currentApproval
                ? w("Setujui ulang teks ini", "Re-approve this text")
                : w("Setujui untuk publikasi", "Approve for publication")}
          </button>
        </div>
      </form>
    </div>
  );
}

function Field({ term, children }: { term: string; children: ReactNode }) {
  return (
    <div className="op-reading-field">
      <span className="op-reading-term">{term}</span>
      <div>{children}</div>
    </div>
  );
}

function Bilingual({
  label,
  value,
}: {
  label: string;
  value: ReadingLocalized;
}) {
  if (!value.id && !value.en) return null;
  return (
    <div className="op-reading-text">
      {label && <h4>{label}</h4>}
      <p lang="id">
        <span className="op-reading-lang" aria-hidden="true">
          {"ID"}
        </span>
        {value.id}
      </p>
      <p lang="en">
        <span className="op-reading-lang" aria-hidden="true">
          {"EN"}
        </span>
        {value.en}
      </p>
    </div>
  );
}

function Steps({
  label,
  steps,
}: {
  label: string;
  steps: NonNullable<ReadingAdminItem["detail"]["steps"]>;
}) {
  return (
    <div className="op-reading-text">
      <h4>{label}</h4>
      <div className="op-reading-steps">
        <div lang="id">
          <span className="op-reading-lang" aria-hidden="true">
            {"ID"}
          </span>
          <ol>
            {steps.id.map((step, index) => (
              <li key={index}>{step}</li>
            ))}
          </ol>
        </div>
        <div lang="en">
          <span className="op-reading-lang" aria-hidden="true">
            {"EN"}
          </span>
          <ol>
            {steps.en.map((step, index) => (
              <li key={index}>{step}</li>
            ))}
          </ol>
        </div>
      </div>
    </div>
  );
}

const CHECK_LABEL: Record<string, [string, string]> = {
  language_parity: ["Dua bahasa lengkap", "Both languages complete"],
  heading_structure: ["Struktur judul", "Heading structure"],
  list_structure: ["Struktur daftar langkah", "Step list structure"],
  link_text: ["Teks tautan dan nama sumber", "Link text and source names"],
  image_alt: ["Teks alternatif gambar", "Image text alternatives"],
  sensory_instructions: [
    "Petunjuk tidak bergantung pada warna, bentuk, atau posisi saja",
    "Instructions don't rely on colour, shape or position alone",
  ],
  sentence_length: [
    "Panjang kalimat (paling banyak 20 kata)",
    "Sentence length (at most 20 words)",
  ],
  readability: [
    "Keterbacaan (rata-rata paling banyak 15 kata per kalimat; tingkat Flesch–Kincaid EN paling tinggi 8)",
    "Readability (average at most 15 words per sentence; English Flesch–Kincaid grade at most 8)",
  ],
  block_length: [
    "Panjang paragraf (paling banyak 80 kata dan 5 kalimat)",
    "Paragraph length (at most 80 words and 5 sentences)",
  ],
};

function checkLabel(id: string, w: Words): string {
  return label(CHECK_LABEL, id, w);
}

function findingLabel(f: ReadingA11yFinding, w: Words): string {
  const v = f.value ?? 0;
  switch (f.code) {
    case "empty_text":
      return w("teks kosong", "empty text");
    case "step_count_differs":
      return w("jumlah langkah ID dan EN berbeda", "ID and EN step counts differ");
    case "title_too_long":
      return w(`judul terlalu panjang (${v} kata)`, `title too long (${v} words)`);
    case "guide_without_sections":
      return w("panduan tanpa bagian berjudul", "guide without headed sections");
    case "heading_too_long":
      return w(
        `judul bagian terlalu panjang (${v} kata)`,
        `section heading too long (${v} words)`,
      );
    case "heading_duplicate":
      return w("judul bagian ganda", "duplicate section heading");
    case "heading_repeats_title":
      return w("judul bagian mengulang judul", "section heading repeats the title");
    case "section_without_text":
      return w("bagian tanpa isi", "section without text");
    case "steps_missing":
      return w("latihan tanpa langkah", "exercise without steps");
    case "too_many_steps":
      return w(`terlalu banyak langkah (${v})`, `too many steps (${v})`);
    case "typed_list_marker":
      return w("nomor atau butir diketik manual", "number or bullet typed into the text");
    case "bare_url":
      return w("alamat web tanpa teks tautan", "web address without link text");
    case "link_text_not_descriptive":
      return w(
        "teks tautan tidak menjelaskan tujuan",
        "link text doesn't describe the target",
      );
    case "generic_link_phrase":
      return w(
        "frasa tautan umum seperti “klik di sini”",
        "generic link phrase such as “click here”",
      );
    case "source_unknown":
      return w("sumber tidak dikenal", "unknown source");
    case "source_title_not_descriptive":
      return w("judul sumber tidak deskriptif", "source title isn't descriptive");
    case "image_without_alt":
      return w("gambar tanpa teks alternatif", "image without a text alternative");
    case "sensory_only_instruction":
      return w(
        "petunjuk bergantung pada warna, bentuk, atau posisi",
        "instruction relies on colour, shape or position",
      );
    case "sentence_over_20_words":
      return w(`kalimat ${v} kata`, `${v}-word sentence`);
    case "avg_words_per_sentence_over_15":
      return w(`rata-rata ${v} kata per kalimat`, `average ${v} words per sentence`);
    case "fk_grade_over_8":
      return w(`tingkat Flesch–Kincaid ${v}`, `Flesch–Kincaid grade ${v}`);
    case "block_over_80_words":
      return w(`paragraf ${v} kata`, `${v}-word paragraph`);
    case "block_over_5_sentences":
      return w(`paragraf ${v} kalimat`, `${v}-sentence paragraph`);
  }
  return f.code;
}

function checkStatus(
  check: ReadingA11yCheck,
  w: Words,
): { text: string; tone: "ok" | "warning" | "danger" | "muted" } {
  if (check.status === "pass") return { text: w("Lulus", "Pass"), tone: "ok" };
  if (check.status === "not_applicable") {
    return { text: w("Tidak berlaku", "Not applicable"), tone: "muted" };
  }
  return check.blocking
    ? { text: w("Gagal · wajib diperbaiki", "Fails · must be fixed"), tone: "danger" }
    : {
        text: w(
          "Gagal · bisa dikecualikan dengan alasan",
          "Fails · can be excepted with a reason",
        ),
        tone: "warning",
      };
}

function a11ySummary(report: ReadingA11yReport, w: Words): string {
  if (report.result === "pass") {
    return w("Aksesibilitas: lulus", "Accessibility: passes");
  }
  if (report.result === "blocked") {
    return w("Aksesibilitas: diblokir", "Accessibility: blocked");
  }
  return w("Aksesibilitas: perlu alasan", "Accessibility: needs a reason");
}

function A11yBadge({ report }: { report: ReadingA11yReport }) {
  const w = useWords();
  const tone =
    report.result === "pass"
      ? "ok"
      : report.result === "blocked"
        ? "danger"
        : "warning";
  return (
    <span className={`op-reading-state op-reading-state--${tone}`}>
      {a11ySummary(report, w)}
    </span>
  );
}

function A11yChecks({ report }: { report: ReadingA11yReport }) {
  const w = useWords();
  const headingId = useId();
  const blocking = report.checks.filter(
    (c) => c.status === "fail" && c.blocking,
  ).length;
  const overridable = report.checks.filter(
    (c) => c.status === "fail" && !c.blocking,
  ).length;
  return (
    <section className="op-reading-a11y" aria-labelledby={headingId}>
      <h4 id={headingId}>
        {w("Pemeriksaan aksesibilitas otomatis", "Automatic accessibility checks")}
      </h4>
      <p>
        <strong>{a11ySummary(report, w)}.</strong>{" "}
        {report.result === "pass"
          ? w(
              "Semua pemeriksaan otomatis lulus untuk teks persis ini. Daftar periksa manual tetap wajib.",
              "Every automatic check passes for this exact text. The manual checklist is still required.",
            )
          : report.result === "blocked"
            ? w(
                `${blocking} pemeriksaan wajib gagal. Teks harus diperbaiki sebelum bisa disetujui.`,
                `${blocking} required check(s) failed. The text must be fixed before it can be approved.`,
              )
            : w(
                `${overridable} pemeriksaan keterbacaan tidak terpenuhi. Persetujuan butuh alasan pengecualian yang dicatat di audit.`,
                `${overridable} readability check(s) not met. Approval needs an exception reason, recorded in the audit.`,
              )}
      </p>
      <ul className="op-a11y-checks">
        {report.checks.map((check) => {
          const status = checkStatus(check, w);
          return (
            <li key={check.id}>
              <span className="op-a11y-check-name">{checkLabel(check.id, w)}</span>
              <span className={`op-reading-state op-reading-state--${status.tone}`}>
                {status.text}
              </span>
              {check.findings.length > 0 && (
                <ul className="op-a11y-findings">
                  {check.findings.map((f, i) => (
                    <li key={`${f.code}-${f.path ?? ""}-${i}`}>
                      {findingLabel(f, w)}
                      {f.path && (
                        <>
                          {" · "}
                          <code>{f.path}</code>
                        </>
                      )}
                    </li>
                  ))}
                </ul>
              )}
            </li>
          );
        })}
      </ul>
      <dl className="op-reading-dl">
        {(["id", "en"] as const).map((code) => {
          const m = report.metrics[code];
          if (!m) return null;
          return (
            <div className="op-a11y-metric" key={code}>
              <dt>
                {code === "id"
                  ? w("Teks Indonesia", "Indonesian text")
                  : w("Teks Inggris", "English text")}
              </dt>
              <dd>
                {w(
                  `${m.sentences} kalimat · rata-rata ${m.avgWordsPerSentence} kata · terpanjang ${m.maxWordsPerSentence} kata`,
                  `${m.sentences} sentences · average ${m.avgWordsPerSentence} words · longest ${m.maxWordsPerSentence} words`,
                )}
                {m.fkGrade !== null && m.fkGrade !== undefined
                  ? w(
                      ` · tingkat Flesch–Kincaid ${m.fkGrade}`,
                      ` · Flesch–Kincaid grade ${m.fkGrade}`,
                    )
                  : ""}
              </dd>
            </div>
          );
        })}
      </dl>
      <p className="op-hint">
        {w(
          `Aturan ${report.version}, dihitung server dari teks persis ini. Bahasa Indonesia belum punya rumus keterbacaan yang tervalidasi, jadi hanya panjang kalimat yang diperiksa (panduan gaya konten 3.2).`,
          `Rules ${report.version}, computed by the server from this exact text. Indonesian has no validated readability formula, so only sentence length is checked (content style guide 3.2).`,
        )}
      </p>
    </section>
  );
}
