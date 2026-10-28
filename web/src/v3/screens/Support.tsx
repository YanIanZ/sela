import { useWords } from "../shared";
import { LinkButton, PageHeader, UnavailableFeature } from "../ui";

export function SupportPage() {
  const w = useWords();
  return (
    <>
      <PageHeader
        eyebrow={w("DUKUNGAN", "SUPPORT")}
        title={w("Dukungan", "Support")}
        lead={w(
          "Hal-hal yang bisa membantu di luar percakapan. Semuanya pilihan.",
          "Things that can help beyond talking. All of it is optional.",
        )}
      />
      <div className="s-support-grid">
        <section className="s-support-card" aria-labelledby="summary-title">
          <h2 id="summary-title">{w("Ringkasan untuk konsultasi", "Summary for a consultation")}</h2>
          <p>
            {w(
              "Kalau kamu ingin bicara dengan profesional, misalnya di Puskesmas atau dengan psikolog, Sela bisa membantumu menyiapkan ringkasan dari yang kamu pilih di Catatanku.",
              "If you want to talk with a professional, for example at a Puskesmas or with a psychologist, Sela can help you prepare a summary from what you choose in My notes.",
            )}
          </p>
          <p className="s-status">
            <strong>{w("Belum tersedia.", "Not available yet.")}</strong>{" "}
            {w(
              "Penyusunan, persetujuan, berbagi, dan tindak lanjut belum tersambung. Tidak ada ringkasan contoh atau tautan berbagi yang ditampilkan.",
              "Building, approving, sharing and follow-up aren't connected. No example summary or share link is shown.",
            )}
          </p>
          <div className="s-actions">
            <LinkButton to="/support/summaries/new" variant="secondary">
              {w("Lihat status fitur ringkasan", "See the summary feature status")}
            </LinkButton>
          </div>
        </section>

        <section className="s-support-card" aria-labelledby="reading-title">
          <h2 id="reading-title">{w("Bacaan & latihan kecil", "Reading & small exercises")}</h2>
          <p>
            {w(
              "Bacaan singkat dan latihan kecil yang bisa kamu coba kapan saja. Tanpa hitungan hari dan tanpa nilai.",
              "Short reads and small exercises you can try any time. No day counts and no scores.",
            )}
          </p>
          <p className="s-status">
            <strong>{w("Belum bisa dibuka.", "Can't open yet.")}</strong>{" "}
            {w(
              "Setiap bacaan menunggu tinjauan editor manusia sebelum ditampilkan.",
              "Every reading item awaits a human editor's review before it is shown.",
            )}
          </p>
          <LinkButton to="/reading" variant="quiet">
            {w("Tentang bacaan & latihan kecil", "About reading & small exercises")}
          </LinkButton>
        </section>

        <section className="s-support-card" aria-labelledby="services-title">
          <h2 id="services-title">{w("Layanan bantuan", "Support services")}</h2>
          <p>
            {w(
              "Daftar layanan yang sudah diverifikasi, dengan sumber dan tanggal pemeriksaan. Bisa dibuka tanpa akun dan tanpa jaringan.",
              "A verified list of services, with sources and verification dates. It opens without an account or a network.",
            )}
          </p>
          <div className="s-actions">
            <LinkButton to="/help" variant="help" icon="help">
              {w("Lihat layanan bantuan", "See support services")}
            </LinkButton>
          </div>
        </section>

        <section className="s-support-card" aria-labelledby="prepare-title">
          <h2 id="prepare-title">{w("Persiapan & setelah konsultasi", "Preparation & after a consultation")}</h2>
          <p>
            {w(
              "Persiapan konsultasi dan catatan setelah konsultasi akan mengikuti ringkasan yang kamu buat. Keduanya belum tersedia.",
              "Preparing for a consultation and writing afterwards will follow the summary you make. Neither is available yet.",
            )}
          </p>
          <div className="s-actions">
            <LinkButton to="/support/summaries/new?step=prepare" variant="quiet">
              {w("Status persiapan", "Preparation status")}
            </LinkButton>
            <LinkButton to="/support/summaries/new?step=after" variant="quiet">
              {w("Status setelah konsultasi", "After-consultation status")}
            </LinkButton>
          </div>
        </section>
      </div>
      <p className="s-hint" style={{ marginTop: "var(--space-5)" }}>
        {w(
          "Mencari bantuan profesional selalu pilihanmu. Sela tidak menghubungi siapa pun dan tidak menilai apakah kamu perlu bantuan.",
          "Seeking professional help is always your choice. Sela contacts no one and doesn't judge whether you need help.",
        )}
      </p>
    </>
  );
}

export function SupportFeaturePage({ feature }: { feature: string }) {
  const w = useWords();
  const copy: Record<string, { id: string; en: string; bodyId: string; bodyEn: string }> = {
    summary: {
      id: "Ringkasan untuk konsultasi",
      en: "Summary for a consultation",
      bodyId:
        "Ringkasan akan memuat hanya hal yang kamu pilih, dengan kata-kata aslimu dan label siapa yang menulis setiap kalimat. Persetujuan akan terikat pada satu versi.",
      bodyEn:
        "A summary will contain only what you choose, with your own words and a label for who wrote each sentence. Approval will be tied to one version.",
    },
    prepare: {
      id: "Persiapan konsultasi",
      en: "Preparing for a consultation",
      bodyId:
        "Persiapan akan berisi tujuan, daftar yang perlu dibawa, kalimat pembuka, dan latihan tanpa penilaian. Sela tidak mengklaim kerja sama atau pemesanan.",
      bodyEn:
        "Preparation will hold the destination, what to bring, opening lines and practice without scoring. Sela claims no partnership or booking.",
    },
    share: {
      id: "Bagikan ringkasan",
      en: "Share a summary",
      bodyId:
        "Berbagi akan memerlukan pratinjau persis, penerima yang diberi label tidak terverifikasi, tujuan, jangka waktu, dan persetujuan baru. Penerima membutuhkan tautan dan kode akses.",
      bodyEn:
        "Sharing will require an exact preview, an unverified recipient label, a purpose, a duration and a fresh consent. A recipient needs both the link and an access code.",
    },
    status: {
      id: "Status berbagi",
      en: "Sharing status",
      bodyId:
        "Halaman status akan menampilkan hanya kejadian yang tercatat, riwayat akses tanpa isi, dan tombol untuk menghentikan akses.",
      bodyEn:
        "The status page will show only recorded events, a content-free access history and a control to stop access.",
    },
    after: {
      id: "Setelah konsultasi",
      en: "After the consultation",
      bodyId:
        "Catatan setelah konsultasi akan ditulis dengan kata-katamu sendiri, disimpan sebagai catatan biasa, dan tidak pernah disebut catatan resmi profesional.",
      bodyEn:
        "Notes after a consultation will be written in your own words, saved as an ordinary note, and never described as an official professional note.",
    },
  };
  const c = copy[feature] ?? copy.summary!;
  return (
    <div className="s-narrow">
      <PageHeader
        back={{ to: "/support", label: w("Dukungan", "Support") }}
        title={w(c.id, c.en)}
      />
      <UnavailableFeature
        title={w("Belum tersedia", "Not available yet")}
        body={w(c.bodyId, c.bodyEn)}
        dependency={w(
          "API ringkasan dan berbagi belum ada di /api/v3. Tidak ada data contoh atau tautan palsu.",
          "The summary and sharing APIs aren't part of /api/v3 yet. No sample data or fake link is shown.",
        )}
      />
      <div className="s-actions">
        <LinkButton to="/notes" variant="secondary">
          {w("Buka Catatanku", "Open My notes")}
        </LinkButton>
        <LinkButton to="/help" variant="help" icon="help">
          {w("Bantuan sekarang", "Help now")}
        </LinkButton>
      </div>
      <p className="s-hint" style={{ marginTop: "var(--space-4)" }}>
        {w(
          "Kamu tetap bisa menyiapkan dirimu dengan menulis catatan dan membawanya sendiri. Tidak wajib membuat ringkasan.",
          "You can still prepare by writing a note and bringing it yourself. A summary is never required.",
        )}
      </p>
    </div>
  );
}
