import { useWords } from "../shared";
import { LinkButton, Notice, PageHeader } from "../ui";

export function ReadingPage() {
  const w = useWords();
  return (
    <>
      <PageHeader
        back={{ to: "/", label: w("Pengenalan", "Introduction") }}
        title={w("Bacaan & latihan kecil", "Reading & small exercises")}
        lead={w(
          "Bacaan singkat dan latihan kecil yang bisa kamu coba kapan saja. Semuanya pilihan, tanpa hitungan hari.",
          "Short reads and small exercises you can try any time. All optional, no day counts.",
        )}
      />
      <div className="s-narrow">
        <Notice tone="info" title={w("Belum bisa dibuka", "Can't open yet")}>
          <p>
            {w(
              "Bacaan belum ditampilkan. Setiap item menunggu tinjauan editor manusia, termasuk pemeriksaan sumber dan lisensi, sebelum bisa dibaca di Sela.",
              "Reading isn't shown yet. Every item awaits a human editor's review, including its source and licence, before it can be read in Sela.",
            )}
          </p>
          <p className="s-small s-muted">
            {w(
              "Sela tidak menampilkan teks yang belum ditinjau sebagai bacaan siap pakai. Tidak ada item contoh yang dipalsukan di sini.",
              "Sela doesn't present unreviewed text as ready-to-use reading. No example item is faked here.",
            )}
          </p>
        </Notice>
        <h2 style={{ marginTop: "var(--space-5)" }}>{w("Sementara itu", "In the meantime")}</h2>
        <p>
          {w(
            "Kamu tetap bisa menulis catatan, bercerita, atau membuka layanan bantuan yang sudah diverifikasi.",
            "You can still write a note, talk, or open the verified support services.",
          )}
        </p>
        <div className="s-actions">
          <LinkButton to="/notes/write" variant="primary">
            {w("Tulis catatan", "Write a note")}
          </LinkButton>
          <LinkButton to="/help" variant="help" icon="help">
            {w("Layanan bantuan", "Support services")}
          </LinkButton>
          <LinkButton to="/" variant="quiet">
            {w("Ke pengenalan", "Go to Introduction")}
          </LinkButton>
        </div>
      </div>
    </>
  );
}

export function RecipientUnavailablePage() {
  const w = useWords();
  return (
    <div className="s-narrow">
      <PageHeader
        title={w("Halaman penerima", "Recipient page")}
        lead={w(
          "Halaman ini untuk orang yang diundang membaca ringkasan. Versi Sela v3 belum bisa membuat atau membuka undangan.",
          "This page is for someone invited to read a summary. This build of Sela v3 can't make or open an invitation yet.",
        )}
      />
      <Notice tone="info" title={w("Belum tersedia", "Not available yet")}>
        <p>
          {w(
            "Tautan dan kode akses belum bisa dibuat karena layanan berbagi belum tersambung. Halaman penerima versi lama tetap tersedia hanya di demo sintetis.",
            "Links and access codes can't be made because the sharing service isn't connected. The earlier recipient page remains available only in the synthetic demo.",
          )}
        </p>
      </Notice>
      <div className="s-actions">
        <LinkButton to="/help" variant="help" icon="help">
          {w("Bantuan sekarang", "Help now")}
        </LinkButton>
        <LinkButton to="/" variant="quiet">
          {w("Ke pengenalan", "Go to Introduction")}
        </LinkButton>
      </div>
    </div>
  );
}
