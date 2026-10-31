import { useEffect, useState, type FormEvent } from "react";
import { api, type Entry } from "../api";
import { useRouter } from "../../lib/router";
import { useSession } from "../session";
import { useLocale, useWords } from "../shared";
import {
  Button,
  CheckboxRow,
  ErrorNotice,
  LinkButton,
  Loading,
  Notice,
  PageHeader,
  TextField,
} from "../ui";

type Share = {
  id: string;
  summaryId: string;
  summaryVersion: number;
  label: string;
  purpose: string;
  state: "active" | "revoked" | "expired";
  version: number;
  expiresAt: string;
  createdAt: string;
  acknowledged: boolean;
  openedCount: number;
  acknowledgedCount: number;
  lastOpenedAt: string | null;
};

type ShareEvent = {
  kind: "created" | "opened" | "acknowledged" | "revoked" | string;
  at: string;
};

type HistoryState = {
  loading: boolean;
  events?: ShareEvent[];
  error?: boolean;
};

type Projection = {
  shareId: string;
  label: string;
  purpose: string;
  summaryVersion: number;
  title: string;
  text: string;
  prepared?: { goal: string; questions: string; bring: string };
  sources: string[];
  createdAt: string;
  expiresAt: string;
  acknowledged: boolean;
};

const purposes = [
  { value: "consultation", id: "Untuk konsultasi", en: "For a consultation" },
  { value: "second_opinion", id: "Pendapat kedua", en: "A second opinion" },
  { value: "family", id: "Keluarga atau orang tepercaya", en: "Family or someone trusted" },
  { value: "other", id: "Lainnya", en: "Other" },
];

export function SharesPage() {
  const w = useWords();
  const loc = useLocale();
  const { policies } = useSession();
  const [shares, setShares] = useState<Share[] | null>(null);
  const [summaries, setSummaries] = useState<Entry[]>([]);
  const [error, setError] = useState<unknown>(null);
  const [busy, setBusy] = useState(false);
  const [consent, setConsent] = useState(false);
  const [form, setForm] = useState({ summaryId: "", label: "", purpose: "consultation", hours: 24 });
  const [created, setCreated] = useState<{ token: string; code: string; share: Share } | null>(null);
  const [expanded, setExpanded] = useState<Record<string, boolean>>({});
  const [history, setHistory] = useState<Record<string, HistoryState>>({});

  const load = () => {
    setError(null);
    Promise.all([
      api<{ shares: Share[] }>("/shares"),
      api<{ summaries: Entry[] }>("/consultation-summaries"),
    ])
      .then(([s, c]) => {
        setShares(s.shares);
        setSummaries(c.summaries.filter((x) => x.summaryState === "approved"));
      })
      .catch(setError);
  };
  useEffect(load, []);

  const create = async (e: FormEvent) => {
    e.preventDefault();
    setError(null);
    if (!consent) {
      setError(new Error(w("Centang persetujuan berbagi dulu.", "Tick the sharing consent first.")));
      return;
    }
    setBusy(true);
    try {
      const version = policies.sharing?.version;
      if (version) {
        await api("/me/consents/sharing", { granted: true, policyVersion: version }, "PUT").catch(() => undefined);
      }
      const res = await api<{ share: Share; token: string; code: string }>("/shares", {
        summaryId: form.summaryId,
        label: form.label,
        purpose: form.purpose,
        durationHours: form.hours,
      });
      setCreated(res);
      setConsent(false);
      setForm({ summaryId: "", label: "", purpose: "consultation", hours: 24 });
      load();
    } catch (err) {
      setError(err);
    } finally {
      setBusy(false);
    }
  };

  const revoke = async (share: Share) => {
    setBusy(true);
    setError(null);
    try {
      await api(`/shares/${share.id}/revoke`, { expectedVersion: share.version }, "POST");
      setHistory((h) => {
        const next = { ...h };
        delete next[share.id];
        return next;
      });
      load();
      if (expanded[share.id]) loadHistory(share.id);
    } catch (err) {
      setError(err);
    } finally {
      setBusy(false);
    }
  };

  const loadHistory = (id: string) => {
    setHistory((h) => ({ ...h, [id]: { loading: true } }));
    api<{ events: ShareEvent[] }>(`/shares/${id}/events`)
      .then((r) => setHistory((h) => ({ ...h, [id]: { loading: false, events: r.events } })))
      .catch(() => setHistory((h) => ({ ...h, [id]: { loading: false, error: true } })));
  };

  const toggleHistory = (id: string) => {
    const next = !expanded[id];
    setExpanded((x) => ({ ...x, [id]: next }));
    if (next && !history[id]) loadHistory(id);
  };

  const eventLabel = (kind: string) => {
    switch (kind) {
      case "created":
        return w("Tautan dibuat", "Link created");
      case "opened":
        return w("Dibuka dengan kode akses", "Opened with the access code");
      case "acknowledged":
        return w("Ditandai sudah dibaca", "Marked as read");
      case "revoked":
        return w("Akses dicabut", "Access revoked");
      default:
        return kind;
    }
  };

  if (!shares) {
    return <Loading label={w("Memuat tautan berbagi…", "Loading share links…")} />;
  }
  return (
    <div className="s-narrow">
      <PageHeader
        back={{ to: "/support", label: w("Dukungan", "Support") }}
        title={w("Bagikan ringkasan", "Share a summary")}
        lead={w(
          "Hanya ringkasan yang sudah disetujui. Penerima tidak diverifikasi identitasnya; kamu bisa mencabut akses kapan saja.",
          "Only approved summaries. The recipient isn't identity-verified; you can revoke access at any time.",
        )}
      />
      <ErrorNotice error={error} />

      {created && (
        <Notice tone="success" title={w("Tautan dibuat — salin sekarang", "Link created — copy it now")}>
          <p>{w("Tautan dan kode hanya ditampilkan sekali. Berikan keduanya kepada penerima.", "The link and code are shown once. Give both to the recipient.")}</p>
          <p>
            <strong>{w("Tautan", "Link")}:</strong>{" "}
            <code>{`${window.location.origin}/recipient?token=${created.token}`}</code>
          </p>
          <p>
            <strong>{w("Kode akses", "Access code")}:</strong> <code>{created.code}</code>
          </p>
        </Notice>
      )}

      <section>
        <h2>{w("Tautan aktif", "Your links")}</h2>
        {shares.length === 0 && <p className="s-hint">{w("Belum ada tautan.", "No links yet.")}</p>}
        {shares.map((share) => {
          const state = history[share.id];
          const open = expanded[share.id] === true;
          return (
            <article className="s-consent" key={share.id}>
              <h3>{share.label}</h3>
              <p className="s-hint">
                {(loc.lang === "id" ? purposes.find((p) => p.value === share.purpose)?.id : purposes.find((p) => p.value === share.purpose)?.en) ?? share.purpose + " · "}
                {share.state === "active"
                  ? w("Aktif sampai ", "Active until ") + loc.dateTime(share.expiresAt)
                  : share.state === "revoked"
                    ? w("Dicabut", "Revoked")
                    : w("Kedaluwarsa", "Expired")}
              </p>
              <p className="s-hint">
                {share.openedCount === 0
                  ? w("Belum pernah dibuka", "Not opened yet")
                  : w(`Dibuka ${share.openedCount} kali`, `Opened ${share.openedCount} times`)}
                {" · "}
                {share.acknowledgedCount === 0
                  ? w("Belum ada konfirmasi dibaca", "No read confirmation yet")
                  : w(
                      `Dikonfirmasi dibaca ${share.acknowledgedCount} kali`,
                      `Marked read ${share.acknowledgedCount} times`,
                    )}
              </p>
              {share.lastOpenedAt && (
                <p className="s-hint">
                  {w("Terakhir dibuka ", "Last opened ") + loc.dateTime(share.lastOpenedAt)}
                </p>
              )}
              <div className="s-actions">
                {share.state === "active" && (
                  <Button variant="quiet" disabled={busy} onClick={() => void revoke(share)}>
                    {w("Cabut akses", "Revoke access")}
                  </Button>
                )}
                <Button
                  variant="quiet"
                  aria-expanded={open}
                  aria-controls={`share-history-${share.id}`}
                  onClick={() => toggleHistory(share.id)}
                >
                  {open ? w("Tutup riwayat akses", "Hide access history") : w("Riwayat akses", "Access history")}
                </Button>
              </div>
              <section id={`share-history-${share.id}`} hidden={!open}>
                {state?.loading && (
                  <p className="s-hint" role="status">
                    {w("Memuat riwayat akses…", "Loading access history…")}
                  </p>
                )}
                {state?.error && (
                  <>
                    <p className="s-hint" role="alert">
                      {w("Riwayat akses tidak bisa dimuat.", "Access history could not be loaded.")}
                    </p>
                    <Button variant="quiet" onClick={() => loadHistory(share.id)}>
                      {w("Coba lagi", "Try again")}
                    </Button>
                  </>
                )}
                {state?.events && state.events.length === 0 && (
                  <p className="s-hint" role="status">
                    {w("Belum ada riwayat.", "No history yet.")}
                  </p>
                )}
                {state?.events && state.events.length > 0 && (
                  <ol>
                    {state.events.map((event, i) => (
                      <li key={`${event.kind}-${event.at}-${i}`}>
                        <span>{eventLabel(event.kind)}</span>{" "}
                        <time className="s-hint" dateTime={event.at}>
                          {loc.dateTime(event.at)}
                        </time>
                      </li>
                    ))}
                  </ol>
                )}
                <p className="s-hint">
                  {w(
                    "Riwayat ini mencatat tautan dan kode akses, bukan identitas penerima.",
                    "This history records the link and access code, not the recipient's identity.",
                  )}
                </p>
              </section>
            </article>
          );
        })}
      </section>

      <form onSubmit={(e) => void create(e)}>
        <h2>{w("Buat tautan baru", "Create a new link")}</h2>
        {summaries.length === 0 ? (
          <Notice tone="info" title={w("Belum ada ringkasan disetujui", "No approved summary yet")}>
            <p>{w("Setujui ringkasan di halaman Ringkasan terlebih dahulu.", "Approve a summary on the Summaries page first.")}</p>
            <LinkButton to="/support/summaries" variant="secondary">
              {w("Buka ringkasan", "Open summaries")}
            </LinkButton>
          </Notice>
        ) : (
          <>
            <label className="s-field">
              <span>{w("Ringkasan", "Summary")}</span>
              <select value={form.summaryId} onChange={(e) => setForm({ ...form, summaryId: e.target.value })} required>
                <option value="">{w("Pilih ringkasan", "Choose a summary")}</option>
                {summaries.map((s) => (
                  <option key={s.id} value={s.id}>
                    {s.title || w("Ringkasan pribadi", "Personal summary")} · v{s.version}
                  </option>
                ))}
              </select>
            </label>
            <TextField
              label={w("Label penerima (tidak diverifikasi)", "Recipient label (not verified)")}
              value={form.label}
              onChange={(e) => setForm({ ...form, label: e.target.value })}
              maxLength={80}
              required
            />
            <label className="s-field">
              <span>{w("Tujuan", "Purpose")}</span>
              <select value={form.purpose} onChange={(e) => setForm({ ...form, purpose: e.target.value })}>
                {purposes.map((p) => (
                  <option key={p.value} value={p.value}>
                    {w(p.id, p.en)}
                  </option>
                ))}
              </select>
            </label>
            <label className="s-field">
              <span>{w("Berlaku (jam)", "Valid for (hours)")}</span>
              <input type="number" min={1} max={168} value={form.hours} onChange={(e) => setForm({ ...form, hours: Number(e.target.value) })} />
            </label>
            {policies.sharing && (
              <div className="s-hint">
                {policies.sharing.body.slice(0, 2).map((p, i) => (
                  <p key={i}>{p[loc.lang]}</p>
                ))}
              </div>
            )}
            <CheckboxRow checked={consent} onChange={setConsent}>
              {w("Saya setuju membagikan ringkasan ini kepada penerima berlabel di atas.", "I agree to share this summary with the labelled recipient above.")}
            </CheckboxRow>
            <div className="s-actions">
              <Button variant="primary" type="submit" disabled={busy || !form.summaryId || form.label.trim() === ""}>
                {w("Buat tautan", "Create link")}
              </Button>
            </div>
          </>
        )}
      </form>
    </div>
  );
}

export function RecipientPage() {
  const w = useWords();
  const loc = useLocale();
  const { query } = useRouter();
  const [token, setToken] = useState(query.get("token") ?? "");
  const [code, setCode] = useState("");
  const [share, setShare] = useState<Projection | null>(null);
  const [error, setError] = useState<unknown>(null);
  const [busy, setBusy] = useState(false);

  useEffect(() => {
    api<{ share: Projection }>("/recipient/share")
      .then((r) => setShare(r.share))
      .catch(() => undefined);
  }, []);

  const open = async (e: FormEvent) => {
    e.preventDefault();
    setBusy(true);
    setError(null);
    try {
      const r = await api<{ share: Projection }>("/recipient/session", { token: token.trim(), code: code.trim() });
      setShare(r.share);
    } catch (err) {
      setError(err);
    } finally {
      setBusy(false);
    }
  };

  const acknowledge = async () => {
    setBusy(true);
    setError(null);
    try {
      await api("/recipient/acknowledge", {});
      setShare((s) => (s ? { ...s, acknowledged: true } : s));
    } catch (err) {
      setError(err);
    } finally {
      setBusy(false);
    }
  };

  if (!share) {
    return (
      <div className="s-narrow">
        <PageHeader
          title={w("Ringkasan untukmu", "A summary for you")}
          lead={w(
            "Seseorang membagikan ringkasan pribadi melalui Sela. Masukkan tautan dan kode akses yang kamu terima.",
            "Someone shared a personal summary through Sela. Enter the link and access code you received.",
          )}
        />
        <ErrorNotice error={error} />
        <form onSubmit={(e) => void open(e)}>
          <TextField label={w("Tautan atau token", "Link or token")} value={token} onChange={(e) => setToken(e.target.value)} required />
          <TextField label={w("Kode akses", "Access code")} value={code} onChange={(e) => setCode(e.target.value)} maxLength={8} required />
          <div className="s-actions">
            <Button variant="primary" type="submit" disabled={busy}>
              {w("Buka ringkasan", "Open summary")}
            </Button>
          </div>
        </form>
        <p className="s-hint">
          {w("Penerima tidak diverifikasi. Tautan bisa dicabut pemiliknya kapan saja.", "The recipient isn't identity-verified. The owner can revoke the link at any time.")}
        </p>
      </div>
    );
  }

  return (
    <div className="s-narrow">
      <PageHeader
        title={share.title || w("Ringkasan pribadi", "Personal summary")}
        lead={w("Dibagikan oleh pemiliknya untuk: ", "Shared by its owner for: ") + share.label}
      />
      <ErrorNotice error={error} />
      <Notice tone="info" title={w("Bukan catatan resmi", "Not an official record")}>
        <p>{w("Ringkasan ini ditulis dan disetujui pemiliknya. Ini bukan diagnosis atau catatan tenaga profesional.", "This summary was written and approved by its owner. It is not a diagnosis or a professional record.")}</p>
      </Notice>
      <article className="s-consent">
        <p>{share.text}</p>
        {share.prepared?.goal && (
          <>
            <h2>{w("Yang ingin diceritakan", "What they want to discuss")}</h2>
            <p>{share.prepared.goal}</p>
          </>
        )}
        {share.prepared?.questions && (
          <>
            <h2>{w("Pertanyaan", "Questions")}</h2>
            <p>{share.prepared.questions}</p>
          </>
        )}
        {share.prepared?.bring && (
          <>
            <h2>{w("Yang dibawa", "What they will bring")}</h2>
            <p>{share.prepared.bring}</p>
          </>
        )}
        {share.sources.length > 0 && (
          <>
            <h2>{w("Sumber pilihan", "Chosen sources")}</h2>
            <ul>
              {share.sources.map((s, i) => (
                <li key={i}>{s}</li>
              ))}
            </ul>
          </>
        )}
        <p className="s-hint">{w("Berlaku sampai ", "Valid until ") + loc.dateTime(share.expiresAt)}</p>
      </article>
      <div className="s-actions">
        <Button variant="primary" disabled={busy || share.acknowledged} onClick={() => void acknowledge()}>
          {share.acknowledged ? w("Sudah dikonfirmasi", "Acknowledged") : w("Saya sudah membaca", "I have read this")}
        </Button>
        <LinkButton to="/help" variant="help" icon="help">
          {w("Bantuan sekarang", "Help now")}
        </LinkButton>
      </div>
      <p className="s-hint">
        {w("Salinan yang kamu unduh atau cetak tidak bisa ditarik kembali.", "Copies you download or print cannot be recalled.")}
      </p>
    </div>
  );
}
