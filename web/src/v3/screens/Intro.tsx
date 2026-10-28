import { useEffect, useState } from "react";
import { Link } from "../../lib/router";
import { usePrefs } from "../prefs";
import { useSession } from "../session";
import { useAnnounce, useWords } from "../shared";
import { Icon } from "../icons";
import {
  LinkButton,
  MoodArt,
  Notice,
  PageHeader,
} from "../ui";
import { LanguageSwitch } from "../shell";

export function IntroPage() {
  const w = useWords();
  const { prefs, set } = usePrefs();
  const { announce } = useAnnounce();
  const { options } = useSession();

  return (
    <div className="n-intro">
      <section className="n-welcome" aria-labelledby="page-title">
        <div className="n-welcome-copy">
          <h1 id="page-title">
            {w("Tidak harus rapi.", "It doesn't have to be tidy.")}{" "}
            <span className="n-title-accent">
              {w("Mulai dari yang terasa.", "Start with what you feel.")}
            </span>
          </h1>
          <p>
            {w(
              "Sela adalah ruang untuk bercerita, mengenali apa yang terasa, dan menyimpan yang ingin kamu bawa. Tidak semua hal harus selesai dulu sebelum kamu mulai.",
              "Sela is a space to talk, notice what you feel, and keep what you want to carry. Not everything has to be finished before you start.",
            )}
          </p>
          <div className="n-welcome-actions">
            <LinkButton to="/start/age" variant="primary" icon="arrow-right">
              {w("Mulai", "Get started")}
            </LinkButton>
            <p className="s-hint">
              {w("Sudah punya akun? ", "Already have an account? ")}
              <Link to="/signin">{w("Masuk", "Sign in")}</Link>
            </p>
          </div>
        </div>
        <MoodArt className="n-brand-art" priority />
      </section>

      <div className="n-intro-details">
        <section className="n-intro-facts" aria-labelledby="intro-facts-title">
          <h2 id="intro-facts-title">{w("Yang perlu kamu tahu", "What you should know")}</h2>
          <ul className="s-intro-points">
            {[
              w("Sela adalah AI, bukan psikolog atau dokter.", "Sela is an AI, not a psychologist or a doctor."),
              w("Sela tidak memberi diagnosis atau pengobatan.", "Sela gives no diagnosis or treatment."),
              w("Kamu yang memilih apa yang disimpan dan dibagikan.", "You choose what is kept and what is shared."),
              w("Sela tidak pernah menghubungi siapa pun atas namamu.", "Sela never contacts anyone on your behalf."),
            ].map((p) => (
              <li key={p}>
                <Icon name="check" />
                <span>{p}</span>
              </li>
            ))}
          </ul>
          <dl className="s-start-facts">
            <div>
              <dt>{w("Tanpa akun", "No account")}</dt>
              <dd>
                {w(
                  "Mulai sebagai tamu. Data disimpan di server Sela dan dihapus otomatis setelah tidak aktif.",
                  "Start as a guest. Data is stored on Sela's server and deleted automatically after inactivity.",
                )}
              </dd>
            </div>
            <div>
              <dt>{w("Dengan akun", "With an account")}</dt>
              <dd>
                {w(
                  "Nama pengguna dan kata sandi, tanpa email. Bisa dibuka dari perangkat lain.",
                  "Username and password, no email. Open it from another device.",
                )}
              </dd>
            </div>
          </dl>
          {options?.guest.available === false && (
            <p className="s-hint">
              {w(
                "Mode tamu sedang penuh. Kamu masih bisa membuat akun.",
                "Guest mode is full right now. You can still create an account.",
              )}
            </p>
          )}
          <p className="s-small s-muted s-intro-safety">
            {w(
              'Kalau kamu dalam bahaya sekarang, tekan "Butuh bantuan sekarang" di bagian atas layar.',
              'If you\'re in danger right now, press "Need help now" at the top of the screen.',
            )}
          </p>
          <div className="s-intro-links">
            <Link to="/about">{w("Pelajari Sela lebih lanjut", "Learn more about Sela")}</Link>
            <Link to="/reading">{w("Bacaan & latihan kecil", "Reading & small exercises")}</Link>
            <Link to="/policies">{w("Kebijakan & privasi", "Policies & privacy")}</Link>
          </div>
          <p className="s-hint">
            {w("Belum siap? ", "Not ready? ")}
            <Link to="/help">{w("Bantuan sekarang", "Help now")}</Link>
            {w(" bisa dibuka tanpa akun.", " opens without an account.")}
          </p>
        </section>

        <section className="n-intro-controls" aria-labelledby="intro-controls-title">
          <h2 id="intro-controls-title">{w("Tampilan", "Display")}</h2>
          <div className="s-pref-row">
            <p className="s-label">{w("Bahasa", "Language")}</p>
            <LanguageSwitch compact />
          </div>
          <div className="s-pref-row">
            <p className="s-label">{w("Ukuran teks", "Text size")}</p>
            <div className="s-textsize" role="group" aria-label={w("Ukuran teks", "Text size")}>
              {(["100", "115", "130", "150"] as const).map((s) => (
                <button
                  key={s}
                  type="button"
                  aria-pressed={prefs.textScale === s}
                  onClick={() => {
                    set("textScale", s);
                    announce("status", w("Ukuran teks: ", "Text size: ") + s + "%");
                  }}
                >
                  {`Aa ${s}%`}
                </button>
              ))}
            </div>
          </div>
          <Link to="/settings/display" className="s-small">
            {w("Pengaturan tampilan lainnya", "More display settings")}
          </Link>
        </section>
      </div>
    </div>
  );
}

export function AboutPage() {
  const w = useWords();
  const [aiStatus, setAiStatus] = useState<{
    available: boolean;
    provider: { mode?: string; modelId?: string; servedModel?: string };
  } | null>(null);
  const [aiStatusFailed, setAiStatusFailed] = useState(false);

  useEffect(() => {
    let active = true;
    fetch("/api/v2/ai/status", {
      credentials: "same-origin",
      headers: { Accept: "application/json" },
    })
      .then(async (r) => {
        if (!r.ok) throw new Error(String(r.status));
        const body = await r.json();
        if (active) setAiStatus(body.data ?? null);
      })
      .catch(() => {
        if (active) setAiStatusFailed(true);
      });
    return () => {
      active = false;
    };
  }, []);

  return (
    <div className="s-narrow">
      <PageHeader
        back={{ to: "/", label: w("Pengenalan", "Introduction") }}
        title={w("Tentang Sela", "About Sela")}
        lead={w(
          "Sela adalah ruang untuk bercerita, memahami kembali pengalaman, dan membawa konteks yang kamu pilih ketika mencari bantuan.",
          "Sela is a space to tell your story, look back at your experience, and bring the context you choose when you look for help.",
        )}
      />
      <h2>{w("Bagaimana AI dipakai", "How AI is used")}</h2>
      <p>
        {w(
          "Sela memakai model terbuka Meta yang dijalankan di Modal untuk menyusun balasan. Balasan diperiksa oleh aturan tetap sebelum ditampilkan; balasan yang menyebut hal kesehatan juga diperiksa oleh Claude dari Anthropic. DeepSeek hanya menjadi model cadangan jika model utama tidak tersedia, dan hanya jika kamu mengizinkannya secara terpisah. Model cadangan tidak dipakai untuk usia di bawah 18 tahun.",
          "Sela uses Meta's open-weight model, served on Modal, to write replies. Replies pass deterministic guards before they are shown; replies that mention health are also checked by Claude from Anthropic. DeepSeek is only a backup generator when the main model is unavailable, and only if you allow it separately. The backup model isn't used under 18.",
        )}
      </p>
      <p>
        {w(
          "Tidak setiap penyedia aktif di setiap lingkungan. Kalau balasan AI tidak tersedia, Sela mengatakan itu dan kamu tetap bisa menulis.",
          "Not every provider is live in every environment. If AI replies aren't available, Sela says so and you can still write.",
        )}
      </p>
      <p className="s-hint">
        {aiStatusFailed
          ? w(
              "Status penyedia tidak dapat dibaca di lingkungan ini.",
              "The provider status can't be read in this environment.",
            )
          : aiStatus === null
            ? w("Membaca status penyedia…", "Reading provider status…")
            : aiStatus.available
              ? w(
                  `Penyedia yang dikonfigurasi: ${aiStatus.provider.modelId || aiStatus.provider.servedModel || aiStatus.provider.mode || "tidak tercatat"}. Ini bukan bukti kualitas atau ketersediaan langsung.`,
                  `Configured provider: ${aiStatus.provider.modelId || aiStatus.provider.servedModel || aiStatus.provider.mode || "not recorded"}. This is not evidence of quality or a live call.`,
                )
              : w(
                  "Balasan AI sedang mati di lingkungan ini. Menulis dan menyimpan tetap bisa.",
                  "AI replies are off in this environment. Writing and saving still work.",
                )}
      </p>
      <h2 style={{ marginTop: "var(--space-5)" }}>{w("Di mana data diproses", "Where data is processed")}</h2>
      <ul className="s-about-list">
        <li>
          <strong>{"Meta · Modal"}</strong>
          <p className="s-small s-muted">{w("Amerika Serikat. Balasan percakapan.", "United States. Conversation replies.")}</p>
        </li>
        <li>
          <strong>{"Claude · Anthropic"}</strong>
          <p className="s-small s-muted">{w("Amerika Serikat. Pemeriksaan balasan dan ringkasan.", "United States. Checks replies and summaries.")}</p>
        </li>
        <li>
          <strong>{"DeepSeek"}</strong>
          <p className="s-small s-muted">
            {w("Tiongkok. Hanya cadangan ketersediaan, mati kecuali kamu mengizinkan.", "China. Availability fallback only, off unless you allow it.")}
          </p>
        </li>
        <li>
          <strong>{w("Server Sela", "Sela's server")}</strong>
          <p className="s-small s-muted">
            {w("Catatan dan percakapan yang kamu simpan, terenkripsi per orang.", "Notes and conversations you save, encrypted per person.")}
          </p>
        </li>
      </ul>
      <h2 style={{ marginTop: "var(--space-5)" }}>{w("Yang tidak pernah Sela lakukan", "What Sela never does")}</h2>
      <ul>
        <li>{w("Memberi diagnosis, resep, atau terapi.", "Give a diagnosis, prescription or therapy.")}</li>
        <li>{w("Menghubungi siapa pun atas namamu.", "Contact anyone on your behalf.")}</li>
        <li>{w("Menampilkan balasan AI sebagai tenaga profesional.", "Present an AI reply as a professional.")}</li>
        <li>{w("Memakai catatanmu untuk melatih model.", "Use your notes to train models.")}</li>
        <li>{w("Menjual data atau memasang iklan.", "Sell data or show advertising.")}</li>
      </ul>
      <h2 style={{ marginTop: "var(--space-5)" }}>{w("Kredit", "Credits")}</h2>
      <p>
        {w(
          "Sela melanjutkan Dengar.in. Ikon mengikuti Lucide (lisensi ISC). Bacaan yang dikutip dari WHO memakai lisensi CC BY-NC-SA 3.0 IGO dan tidak ditampilkan sebelum ditinjau editor.",
          "Sela continues Dengar.in. Icons follow Lucide (ISC licence). Reading adapted from WHO uses CC BY-NC-SA 3.0 IGO and isn't shown before an editor reviews it.",
        )}
      </p>
      <div className="s-actions">
        <LinkButton to="/policies" variant="secondary">
          {w("Baca kebijakan", "Read the policies")}
        </LinkButton>
        <LinkButton to="/demo/research" variant="quiet">
          {w("Untuk peninjau", "For reviewers")}
        </LinkButton>
      </div>
      <Notice tone="info" title={w("Bukan layanan darurat", "Not an emergency service")}>
        <p>
          {w(
            "Kalau kamu dalam bahaya sekarang, hubungi layanan darurat setempat atau orang terdekat yang bisa menemanimu.",
            "If you're in danger right now, contact local emergency services or someone nearby who can stay with you.",
          )}
        </p>
      </Notice>
    </div>
  );
}
