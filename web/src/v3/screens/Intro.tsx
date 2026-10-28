import { useEffect, useState } from "react";
import { Link, useRouter } from "../../lib/router";
import { usePrefs } from "../prefs";
import { useSession } from "../session";
import { useAnnounce, useWords } from "../shared";
import { Mark, Icon } from "../icons";
import {
  LinkButton,
  Notice,
  PageHeader,
  buttonClass,
} from "../ui";
import { LanguageSwitch } from "../shell";

export function IntroPage() {
  const w = useWords();
  const { prefs, set } = usePrefs();
  const { announce } = useAnnounce();
  const { options } = useSession();
  const { navigate } = useRouter();

  return (
    <div className="s-intro">
      <section className="s-intro-copy">
        <div style={{ color: "var(--c-accent)", marginBottom: "var(--space-3)" }}>
          <Mark size={44} />
        </div>
        <p className="s-overline">{w("SEJENAK, UNTUK DIRIMU", "A MOMENT, JUST FOR YOU")}</p>
        <h1 id="page-title">
          {w("Ruang sejenak untuk didengar.", "A little space to feel heard.")}
        </h1>
        <p className="s-intro-lead">
          {w(
            "Ceritakan apa yang sedang kamu rasakan. Sela mendengarkan, membantumu melihat kembali ceritamu, dan kalau kamu mau, membantumu bersiap bicara dengan profesional.",
            "Talk about what's on your mind. Sela listens, helps you look back at what you said and, if you want, helps you get ready to talk with a professional.",
          )}
        </p>
        <div className="s-intro-preferences">
          <div>
            <p className="s-label">{w("Bahasa", "Language")}</p>
            <LanguageSwitch />
          </div>
          <div>
            <p className="s-label">{w("Ukuran teks", "Text size")}</p>
            <div className="s-textsize" role="group" aria-label={w("Ukuran teks", "Text size")}>
              {(["100", "115", "130", "150"] as const).map((s) => (
                <button
                  key={s}
                  type="button"
                  aria-pressed={prefs.textScale === s}
                  onClick={() => {
                    set("textScale", s);
                    announce(
                      "status",
                      w("Ukuran teks: ", "Text size: ") + s + "%",
                    );
                  }}
                >
                  {`Aa ${s}%`}
                </button>
              ))}
            </div>
          </div>
          <LinkButton to="/settings/display" variant="quiet">
            {w("Pengaturan tampilan lainnya", "More display settings")}
          </LinkButton>
        </div>
        <div className="s-intro-actions">
          <LinkButton to="/start/age" variant="primary" icon="arrow-right">
            {w("Mulai", "Get started")}
          </LinkButton>
          <LinkButton to="/signin" variant="secondary">
            {w("Masuk", "Sign in")}
          </LinkButton>
          <LinkButton to="/about" variant="quiet">
            {w("Pelajari Sela lebih lanjut", "Learn more about Sela")}
          </LinkButton>
          <LinkButton to="/reading" variant="quiet">
            {w("Bacaan & latihan kecil", "Reading & small exercises")}
          </LinkButton>
        </div>
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
        <p className="s-small s-muted" style={{ marginTop: "var(--space-4)" }}>
          {w(
            "Kalau kamu dalam bahaya sekarang, tekan \"Butuh bantuan sekarang\" di bagian atas layar.",
            'If you\'re in danger right now, press "Need help now" at the top of the screen.',
          )}
        </p>
      </section>
      <aside className="s-start-panel" aria-labelledby="start-panel-title">
        <p className="s-overline">{w("MULAI DARI SINI", "START HERE")}</p>
        <h2 id="start-panel-title">{w("Kamu tidak harus punya jawaban.", "You don't need the answers.")}</h2>
        <p>
          {w(
            "Sela tidak meminta profil panjang. Bahasa, usia, dan pilihan penyimpanan sudah cukup untuk membuka ruang cerita.",
            "Sela doesn't ask for a long profile. Language, age and a storage choice are enough to open the talking space.",
          )}
        </p>
        <ul className="s-about-list">
          <li>
            <strong>{w("Mulai sebagai tamu", "Start as a guest")}</strong>
            <p className="s-small s-muted">
              {w(
                "Tanpa akun. Data di server Sela, dihapus otomatis setelah tidak aktif.",
                "No account. Data on Sela's server, deleted automatically after inactivity.",
              )}
            </p>
          </li>
          <li>
            <strong>{w("Atau buat akun", "Or create an account")}</strong>
            <p className="s-small s-muted">
              {w(
                "Nama pengguna dan kata sandi, tanpa email. Bisa dibuka dari perangkat lain.",
                "Username and password, no email. Open it from another device.",
              )}
            </p>
          </li>
        </ul>
        <button
          type="button"
          className={buttonClass("primary")}
          onClick={() => navigate("/start/age")}
        >
          {w("Mulai", "Get started")}
          <Icon name="arrow-right" />
        </button>
        {!options?.guest.available && (
          <p className="s-hint">
            {w(
              "Mode tamu sedang penuh. Kamu masih bisa membuat akun.",
              "Guest mode is full right now. You can still create an account.",
            )}
          </p>
        )}
        <p className="s-hint">
          {w(
            "Belum siap? Bantuan sekarang bisa dibuka tanpa akun.",
            "Not ready? Help now opens without an account.",
          )}{" "}
          <Link to="/help">{w("Bantuan sekarang", "Help now")}</Link>
        </p>
      </aside>
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
    <>
      <PageHeader
        back={{ to: "/", label: w("Pengenalan", "Introduction") }}
        title={w("Tentang Sela", "About Sela")}
        lead={w(
          "Sela adalah ruang untuk bercerita, memahami kembali pengalaman, dan membawa konteks yang kamu pilih ketika mencari bantuan.",
          "Sela is a space to tell your story, look back at your experience, and bring the context you choose when you look for help.",
        )}
      />
      <div className="s-narrow">
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
    </>
  );
}
