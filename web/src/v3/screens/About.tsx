import { useEffect, useState } from "react";
import { useWords } from "../shared";
import { LinkButton, Notice, PageHeader } from "../ui";

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
          "Sela memakai model Meta melalui Meta Model API atau model terbuka Meta yang dijalankan di Modal untuk menyusun balasan. Balasan diperiksa oleh aturan tetap sebelum ditampilkan; balasan yang menyebut hal kesehatan juga diperiksa oleh model Meta lain, bukan model yang menulisnya. Tidak ada model cadangan dari penyedia ketiga.",
          "Sela uses Meta's model through the Meta Model API or the open-weight model served on Modal to write replies. Replies pass deterministic guards before they are shown; replies that mention health are also checked by a different Meta model, never the one that wrote them. There is no third-party backup model.",
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
      <h2 className="s-mt-5">{w("Di mana data diproses", "Where data is processed")}</h2>
      <ul className="s-about-list">
        <li>
          <strong>{"Meta · Model API / Modal"}</strong>
          <p className="s-small s-muted">{w("Amerika Serikat. Balasan percakapan.", "United States. Conversation replies.")}</p>
        </li>
        <li>
          <strong>{w("Model Meta pemeriksa", "Meta verifier model")}</strong>
          <p className="s-small s-muted">{w("Amerika Serikat. Model Meta lain memeriksa balasan dan ringkasan.", "United States. A different Meta model checks replies and summaries.")}</p>
        </li>
        <li>
          <strong>{w("Server Sela", "Sela's server")}</strong>
          <p className="s-small s-muted">
            {w("Catatan dan percakapan yang kamu simpan, terenkripsi per orang.", "Notes and conversations you save, encrypted per person.")}
          </p>
        </li>
      </ul>
      <h2 className="s-mt-5">{w("Yang tidak pernah Sela lakukan", "What Sela never does")}</h2>
      <ul>
        <li>{w("Memberi diagnosis, resep, atau terapi.", "Give a diagnosis, prescription or therapy.")}</li>
        <li>{w("Menghubungi siapa pun atas namamu.", "Contact anyone on your behalf.")}</li>
        <li>{w("Menampilkan balasan AI sebagai tenaga profesional.", "Present an AI reply as a professional.")}</li>
        <li>{w("Memakai catatanmu untuk melatih model.", "Use your notes to train models.")}</li>
        <li>{w("Menjual data atau memasang iklan.", "Sell data or show advertising.")}</li>
      </ul>
      <h2 className="s-mt-5">{w("Kredit", "Credits")}</h2>
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
