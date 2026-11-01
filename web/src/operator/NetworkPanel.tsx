import { useCallback, useEffect, useState, type FormEvent } from "react";
import { useWords } from "../v3/shared";
import { AdminError, adminApi, type CareOrg, type CareOrgMember } from "./api";
import { roleLabel } from "./chrome";
import "./care-panels.css";
import "./network.css";

type Words = (id: string, en: string) => string;

export const ORG_KIND: Record<string, [string, string]> = {
  clinic: ["Klinik", "Clinic"],
  hospital: ["Rumah sakit", "Hospital"],
  puskesmas: ["Puskesmas", "Community health centre"],
  practice: ["Praktik mandiri", "Private practice"],
  organization: ["Organisasi", "Organization"],
};
const ORG_STATUS: Record<string, [string, string]> = {
  pending: ["Menunggu verifikasi", "Pending verification"],
  verified: ["Terverifikasi", "Verified"],
  suspended: ["Ditangguhkan", "Suspended"],
};
const STAGES: [string, string, string][] = [
  ["everyday", "Sehari-hari", "Everyday"],
  ["attention", "Perlu perhatian", "Needs attention"],
  ["support", "Butuh dukungan", "Needs support"],
  ["urgent", "Butuh bantuan segera", "Needs help now"],
];
export const PROFESSIONAL_TYPE: Record<string, [string, string]> = {
  psychologist: ["Psikolog", "Psychologist"],
  psychiatrist: ["Psikiater", "Psychiatrist"],
  doctor: ["Dokter", "Doctor"],
  counsellor: ["Konselor", "Counsellor"],
  nurse: ["Perawat", "Nurse"],
  social_worker: ["Pekerja sosial", "Social worker"],
};

export function mapLabel(map: Record<string, [string, string]>, key: string | undefined, w: Words): string {
  if (!key) return "";
  const v = map[key];
  return v ? w(v[0], v[1]) : key;
}

function errorText(err: unknown, w: Words): string {
  if (err instanceof AdminError) {
    if (err.status === 403) return w("Tindakan ini tidak diizinkan untuk peranmu, atau pembuat tidak boleh memverifikasi organisasi nyata miliknya sendiri.", "Your role may not do this, or the creator may not verify a real organisation they created.");
    if (err.status === 409) return w("Status sudah berubah. Muat ulang lalu coba lagi.", "The status already changed. Refresh and try again.");
    if (err.status === 400) return w("Periksa isian formulir.", "Check the form fields.");
  }
  return w("Permintaan gagal. Coba lagi.", "The request failed. Try again.");
}

/**
 * NetworkPanel is the internal view of the professional care network
 * (D-076): partner organisations, their accounts and licence checks.
 * Accounts are provisioned out of band with the CLI, never from here.
 */
export function NetworkPanel({
  canManage,
  canVerify,
  onExpired,
}: {
  canManage: boolean;
  canVerify: boolean;
  onExpired: () => void;
}) {
  const w = useWords();
  const [orgs, setOrgs] = useState<CareOrg[] | null>(null);
  const [realPartners, setRealPartners] = useState(false);
  const [error, setError] = useState("");
  const [notice, setNotice] = useState("");
  const [busy, setBusy] = useState("");
  const [form, setForm] = useState(false);
  const [open, setOpen] = useState("");
  const [members, setMembers] = useState<Record<string, CareOrgMember[]>>({});

  const fail = useCallback(
    (err: unknown) => {
      if (err instanceof AdminError && err.status === 401) onExpired();
      else setError(errorText(err, w));
    },
    [onExpired, w],
  );

  const load = useCallback(async () => {
    try {
      const r = await adminApi.networkOrgs();
      setOrgs(r.orgs);
      setRealPartners(r.realPartners);
    } catch (err) {
      fail(err);
    }
  }, [fail]);
  useEffect(() => {
    void load();
  }, [load]);

  const run = async (key: string, job: () => Promise<void>, done: string) => {
    if (busy) return;
    setBusy(key);
    setError("");
    setNotice("");
    try {
      await job();
      setNotice(done);
    } catch (err) {
      fail(err);
    } finally {
      setBusy("");
    }
  };

  const loadMembers = async (id: string) => {
    try {
      const list = await adminApi.networkMembers(id);
      setMembers((m) => ({ ...m, [id]: list }));
    } catch (err) {
      fail(err);
    }
  };

  const toggle = (id: string) => {
    const next = open === id ? "" : id;
    setOpen(next);
    if (next && !members[next]) void loadMembers(next);
  };

  return (
    <section className="op-section" aria-labelledby="op-network">
      <div className="op-section-head">
        <div>
          <h2 id="op-network">{w("Jaringan profesional", "Professional network")}</h2>
          <p className="op-hint">
            {w(
              "Klinik, praktik, rumah sakit, dan puskesmas mitra beserta akun profesionalnya. Akun mitra hanya melihat permintaan bantuan organisasinya sendiri. Pembuat organisasi nyata tidak boleh memverifikasinya sendiri.",
              "Partner clinics, practices, hospitals and community health centres with their professional accounts. A partner account only sees its own organisation's care requests. The creator of a real organisation may not verify it.",
            )}
          </p>
          {!realPartners && (
            <p className="op-hint op-network-sim">
              {w(
                "Deployment ini hanya mengizinkan organisasi simulasi sampai owner memberi persetujuan SELA_CARE_NETWORK_REAL_ACK (B-10 masih terbuka).",
                "This deployment allows simulated organisations only until the owner sets SELA_CARE_NETWORK_REAL_ACK (B-10 is still open).",
              )}
            </p>
          )}
        </div>
        {canManage && (
          <button type="button" className="op-btn op-btn--secondary" onClick={() => setForm((v) => !v)}>
            {form ? w("Tutup formulir", "Close form") : w("Tambah organisasi", "Add organisation")}
          </button>
        )}
      </div>

      {error && <p className="op-alert" role="alert">{error}</p>}
      {notice && <p className="op-notice" role="status">{notice}</p>}

      {form && canManage && (
        <OrgForm
          realPartners={realPartners}
          busy={busy === "create"}
          onSubmit={(input) =>
            void run(
              "create",
              async () => {
                await adminApi.createNetworkOrg(input);
                setForm(false);
                await load();
              },
              w("Organisasi dibuat dan menunggu verifikasi.", "Organisation created and waiting for verification."),
            )
          }
        />
      )}

      {orgs === null ? (
        <p className="op-hint">{w("Memuat jaringan…", "Loading the network…")}</p>
      ) : orgs.length === 0 ? (
        <p className="op-hint">{w("Belum ada organisasi mitra.", "No partner organisations yet.")}</p>
      ) : (
        <ul className="op-care-list">
          {orgs.map((o) => (
            <li key={o.id}>
              <div className="op-care-main">
                <strong>{o.name}</strong>
                <span className="op-care-tags">
                  <span className="op-badge op-badge--soft">{mapLabel(ORG_KIND, o.kind, w)}</span>
                  <span className={`op-care-status op-care-status--${o.status === "suspended" ? "revoked" : o.status}`}>
                    {mapLabel(ORG_STATUS, o.status, w)}
                  </span>
                  {o.synthetic && <span className="op-badge op-badge--demo">{w("Simulasi", "Simulation")}</span>}
                  {o.city && <span className="op-hint">{o.city}</span>}
                  <span className="op-hint">
                    {o.members} {w("akun", "accounts")}
                  </span>
                </span>
                {o.description && <p className="op-care-desc">{o.description}</p>}
                <button
                  type="button"
                  className="op-btn op-btn--quiet op-network-toggle"
                  aria-expanded={open === o.id}
                  onClick={() => toggle(o.id)}
                >
                  {open === o.id ? w("Sembunyikan akun", "Hide accounts") : w("Lihat akun", "Show accounts")}
                </button>
                {open === o.id && (
                  <Members
                    org={o}
                    members={members[o.id]}
                    canVerify={canVerify}
                    busy={busy}
                    onVerify={(username) =>
                      void run(
                        `license:${username}`,
                        async () => {
                          await adminApi.verifyLicense(username);
                          await loadMembers(o.id);
                        },
                        w(`Lisensi ${username} ditandai terverifikasi.`, `${username}'s licence is marked verified.`),
                      )
                    }
                  />
                )}
              </div>
              {canVerify && (
                <div className="op-care-actions">
                  {o.status !== "verified" && (
                    <button
                      type="button"
                      className="op-btn op-btn--secondary"
                      disabled={busy !== ""}
                      onClick={() =>
                        void run(
                          `status:${o.id}`,
                          async () => {
                            await adminApi.setNetworkOrgStatus(o.id, "verified");
                            await load();
                          },
                          w("Organisasi terverifikasi dan bisa menerima permintaan.", "Organisation verified and can receive requests."),
                        )
                      }
                    >
                      {w("Verifikasi", "Verify")}
                    </button>
                  )}
                  {o.status !== "suspended" && (
                    <button
                      type="button"
                      className="op-btn op-btn--quiet op-btn--danger-text"
                      disabled={busy !== ""}
                      onClick={() =>
                        void run(
                          `status:${o.id}`,
                          async () => {
                            await adminApi.setNetworkOrgStatus(o.id, "suspended");
                            await load();
                          },
                          w("Organisasi ditangguhkan. Permintaan baru tidak bisa dikirim.", "Organisation suspended. No new requests can be sent."),
                        )
                      }
                    >
                      {w("Tangguhkan", "Suspend")}
                    </button>
                  )}
                </div>
              )}
            </li>
          ))}
        </ul>
      )}
    </section>
  );
}

function OrgForm({
  realPartners,
  busy,
  onSubmit,
}: {
  realPartners: boolean;
  busy: boolean;
  onSubmit: (input: Record<string, unknown>) => void;
}) {
  const w = useWords();
  const [kind, setKind] = useState("clinic");
  const [name, setName] = useState("");
  const [city, setCity] = useState("");
  const [description, setDescription] = useState("");
  const [modes, setModes] = useState<string[]>(["online"]);
  const [stages, setStages] = useState<string[]>([]);
  const [synthetic, setSynthetic] = useState(true);
  const flip = (list: string[], v: string) => (list.includes(v) ? list.filter((x) => x !== v) : [...list, v]);

  const submit = (e: FormEvent) => {
    e.preventDefault();
    onSubmit({ kind, name, city, description, modes, stages, synthetic: realPartners ? synthetic : true });
  };

  return (
    <form className="op-care-form" onSubmit={submit} aria-label={w("Organisasi baru", "New organisation")}>
      <label className="op-field">
        <span>{w("Nama organisasi", "Organisation name")}</span>
        <input className="op-input" required minLength={2} maxLength={160} value={name} onChange={(e) => setName(e.target.value)} />
      </label>
      <label className="op-field">
        <span>{w("Jenis", "Kind")}</span>
        <select className="op-input" value={kind} onChange={(e) => setKind(e.target.value)}>
          {Object.entries(ORG_KIND).map(([k, v]) => (
            <option key={k} value={k}>
              {w(v[0], v[1])}
            </option>
          ))}
        </select>
      </label>
      <label className="op-field">
        <span>{w("Kota", "City")}</span>
        <input className="op-input" maxLength={120} value={city} onChange={(e) => setCity(e.target.value)} />
      </label>
      <label className="op-field op-field--wide">
        <span>{w("Deskripsi untuk pengguna", "Description for people")}</span>
        <textarea className="op-input" rows={2} maxLength={1000} value={description} onChange={(e) => setDescription(e.target.value)} />
      </label>
      <fieldset className="op-field op-network-set">
        <legend>{w("Cara layanan", "Service modes")}</legend>
        {(
          [
            ["online", w("Daring", "Online")],
            ["in_person", w("Tatap muka", "In person")],
          ] as const
        ).map(([v, l]) => (
          <label key={v} className="op-check">
            <input type="checkbox" checked={modes.includes(v)} onChange={() => setModes((m) => flip(m, v))} />
            <span>{l}</span>
          </label>
        ))}
      </fieldset>
      <fieldset className="op-field op-network-set">
        <legend>{w("Cocok untuk tahap", "Fits stages")}</legend>
        {STAGES.map(([v, id, en]) => (
          <label key={v} className="op-check">
            <input type="checkbox" checked={stages.includes(v)} onChange={() => setStages((s) => flip(s, v))} />
            <span>{w(id, en)}</span>
          </label>
        ))}
      </fieldset>
      <div className="op-field op-field--wide">
        <label className="op-check">
          <input
            type="checkbox"
            checked={realPartners ? synthetic : true}
            disabled={!realPartners}
            onChange={(e) => setSynthetic(e.target.checked)}
          />
          <span>{w("Organisasi simulasi (berlabel Simulasi untuk pengguna)", "Simulated organisation (labelled Simulation for people)")}</span>
        </label>
        {!realPartners && (
          <p className="op-hint">
            {w("Wajib simulasi: organisasi nyata belum diizinkan di deployment ini.", "Simulation is required: real organisations are not allowed on this deployment yet.")}
          </p>
        )}
      </div>
      <div className="op-actions">
        <button className="op-btn op-btn--primary" type="submit" disabled={busy || modes.length === 0}>
          {busy ? w("Menyimpan…", "Saving…") : w("Buat organisasi", "Create organisation")}
        </button>
      </div>
    </form>
  );
}

function Members({
  org,
  members,
  canVerify,
  busy,
  onVerify,
}: {
  org: CareOrg;
  members?: CareOrgMember[];
  canVerify: boolean;
  busy: string;
  onVerify: (username: string) => void;
}) {
  const w = useWords();
  const [copied, setCopied] = useState(false);
  const cli = `sela admin create-partner --username … --role practitioner --org ${org.id} --display-name … --professional-type psychologist --license …`;
  const copy = async () => {
    try {
      await navigator.clipboard.writeText(cli);
      setCopied(true);
    } catch {
      setCopied(false);
    }
  };
  return (
    <div className="op-network-members">
      {members === undefined ? (
        <p className="op-hint">{w("Memuat akun…", "Loading accounts…")}</p>
      ) : members.length === 0 ? (
        <p className="op-hint">{w("Belum ada akun untuk organisasi ini.", "No accounts for this organisation yet.")}</p>
      ) : (
        <ul className="op-network-member-list">
          {members.map((m) => (
            <li key={m.username}>
              <div className="op-care-main">
                <strong>{m.displayName || m.username}</strong>
                <span className="op-care-tags">
                  <span className="op-badge op-badge--soft">{roleLabel(m.role, w)}</span>
                  {m.professionalType && <span className="op-hint">{mapLabel(PROFESSIONAL_TYPE, m.professionalType, w)}</span>}
                  <span className="op-hint">@{m.username}</span>
                  {m.status !== "active" && <span className="op-care-status op-care-status--revoked">{w("Dicabut", "Revoked")}</span>}
                </span>
                {m.role === "practitioner" && (
                  <p className="op-hint">
                    {w("Lisensi", "Licence")}: {m.licenseNo || w("belum diisi", "not provided")} ·{" "}
                    {m.licenseVerified ? w("terverifikasi", "verified") : w("belum diverifikasi", "not verified")}
                  </p>
                )}
              </div>
              {canVerify && m.role === "practitioner" && m.licenseNo && !m.licenseVerified && (
                <button
                  type="button"
                  className="op-btn op-btn--secondary"
                  disabled={busy !== ""}
                  onClick={() => onVerify(m.username)}
                >
                  {w("Verifikasi lisensi", "Verify licence")}
                </button>
              )}
            </li>
          ))}
        </ul>
      )}
      {canVerify && (
        <p className="op-hint">
          {w(
            "Verifikasi lisensi berarti kamu sudah mencocokkan nomor STR/SIP dengan register resmi (KKI, dinas kesehatan). Kamu tidak bisa memverifikasi lisensimu sendiri.",
            "Verifying a licence means you checked the STR/SIP number against the official register (KKI, health office). You cannot verify your own.",
          )}
        </p>
      )}
      <div className="op-network-cli">
        <p className="op-hint">
          {w(
            "Akun dibuat di luar konsol setelah identitas dan lisensinya dicek. Jalankan di server (kata sandi dibaca dari stdin; rahasia TOTP ditampilkan sekali):",
            "Accounts are created outside the console after identity and licence checks. Run on the server (the password is read from stdin; the TOTP secret is shown once):",
          )}
        </p>
        <code>{cli}</code>
        <button type="button" className="op-btn op-btn--quiet" onClick={() => void copy()}>
          {copied ? w("Tersalin", "Copied") : w("Salin perintah", "Copy command")}
        </button>
      </div>
    </div>
  );
}
