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
      case "invalid_input":
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
};

export function ReadingPage({
  admin,
  onSignOut,
  onExpired,
}: {
  admin: Admin;
  onSignOut: () => void;
  onExpired: () => void;
}) {
  const w = useWords();
  const { lang } = useI18n();
  const allowed = canReviewReading(admin);
  const [items, setItems] = useState<ReadingAdminItem[] | null>(null);
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
    const [rows, events] = await Promise.all([
      adminApi.reading(),
      adminApi.readingAudit(),
    ]);
    setItems(rows);
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
    setBusy(`approve:${slug}`);
    setError("");
    setNotice("");
    try {
      await adminApi.approveReading(slug, {
        ...fields,
        verifierStatus: "supported",
        contentHash: item.contentHash,
        confirm: true,
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
      handleError(err);
      return false;
    } finally {
      setBusy("");
    }
  };

  const revoke = async (item: ReadingAdminItem, reason: string) => {
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
    } catch (err) {
      handleError(err);
    } finally {
      setBusy("");
    }
  };

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
        <div className="op-intro">
          <div>
            <h1 ref={titleRef} tabIndex={-1}>
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

        {error && (
          <p className="op-alert" role="alert">
            {error}
          </p>
        )}
        {notice && (
          <p className="op-notice" role="status">
            {notice}
          </p>
        )}

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
        ) : items === null ? (
          <section className="op-section" aria-labelledby="op-reading-loading">
            <h2 id="op-reading-loading">{w("Bacaan", "Reading")}</h2>
            <p className="op-hint">
              {w("Memuat pustaka…", "Loading the library…")}
            </p>
          </section>
        ) : (
          <ReadingWorkspace
            items={items}
            busy={busy}
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
  busy,
  onApprove,
  onRevoke,
}: {
  items: ReadingAdminItem[];
  busy: string;
  onApprove: (item: ReadingAdminItem, fields: ApproveFields) => Promise<boolean>;
  onRevoke: (item: ReadingAdminItem, reason: string) => Promise<void>;
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
          busy={busy}
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
  busy,
  onApprove,
  onRevoke,
}: {
  item: ReadingAdminItem;
  busy: string;
  onApprove: (item: ReadingAdminItem, fields: ApproveFields) => Promise<boolean>;
  onRevoke: (item: ReadingAdminItem, reason: string) => Promise<void>;
}) {
  const w = useWords();
  const { lang } = useI18n();
  const detail = item.detail;
  const [reviewerName, setReviewerName] = useState("");
  const [reviewedOn, setReviewedOn] = useState("");
  const [note, setNote] = useState("");
  const [confirm, setConfirm] = useState(false);
  const [revoking, setRevoking] = useState(false);
  const [reason, setReason] = useState("");
  const nameId = useId();
  const dateId = useId();
  const noteId = useId();
  const confirmId = useId();
  const reasonId = useId();

  useEffect(() => {
    setReviewerName("");
    setReviewedOn("");
    setNote("");
    setConfirm(false);
    setRevoking(false);
    setReason("");
  }, [detail.id, item.contentHash]);

  const currentApproval =
    item.approval && !item.approval.revokedAt ? item.approval : undefined;
  const approveBusy = busy === `approve:${detail.id}`;
  const revokeBusy = busy === `revoke:${detail.id}`;
  const canSubmit =
    reviewerName.trim() !== "" && reviewedOn !== "" && confirm && !approveBusy;

  const submit = (e: FormEvent) => {
    e.preventDefault();
    if (!canSubmit) return;
    setConfirm(false);
    void onApprove(item, {
      reviewerName: reviewerName.trim(),
      reviewedOn,
      note: note.trim(),
    }).then((ok) => {
      if (ok) {
        setReviewerName("");
        setReviewedOn("");
        setNote("");
      }
    });
  };

  return (
    <div className="op-reading-preview" id="op-reading-preview">
      <div className="op-reading-preview-head">
        <div>
          <h3>
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
                        void onRevoke(item, reason.trim());
                      }}
                    >
                      {revokeBusy
                        ? w("Mengakhiri…", "Ending…")
                        : w("Ya, akhiri publikasi", "Yes, end publication")}
                    </button>
                    <button
                      type="button"
                      className="op-btn op-btn--quiet"
                      onClick={() => setRevoking(false)}
                    >
                      {w("Batal", "Cancel")}
                    </button>
                  </div>
                </div>
              ) : (
                <button
                  type="button"
                  className="op-btn op-btn--quiet op-btn--danger-text"
                  onClick={() => setRevoking(true)}
                >
                  {w("Akhiri publikasi", "End publication")}
                </button>
              )}
            </div>
          )}
        </div>
      )}

      <form className="op-reading-form" onSubmit={submit}>
        <h4>{w("Setujui teks ini", "Approve this text")}</h4>
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
        <div className="op-actions">
          <button
            type="submit"
            className="op-btn op-btn--primary"
            disabled={!canSubmit}
            aria-busy={approveBusy || undefined}
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
