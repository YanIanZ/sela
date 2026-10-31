import { useWords } from "../shared";
import { LinkButton, PageHeader, UnavailableFeature } from "../ui";

export function SupportPage() {
  const w = useWords();
  return <div className="c-support"><PageHeader title={w("Dukungan, dengan caramu", "Support, your way")} lead={w("Untuk saat kamu ingin membawa cerita lebih jauh. Pilih yang berguna bagimu.", "For when you want to take your story further. Choose what is useful to you.")} />
    <section className="c-support-intro"><div><h2>{w("Kamu yang menentukan ceritanya.", "You decide what to share.")}</h2><p>{w("Pilih bagian dari Catatanku, susun dengan kata-katamu, dan periksa sebelum kamu membawanya ke percakapan dengan orang lain.", "Choose parts of My notes, arrange them in your words, and review them before taking them into a conversation with someone else.")}</p><div className="s-actions"><LinkButton to="/support/summaries/new" variant="primary">{w("Susun ringkasan", "Build a summary")}</LinkButton><LinkButton to="/support/summaries" variant="quiet">{w("Lihat ringkasanku", "My summaries")}</LinkButton></div></div></section>
    <div className="c-support-paths"><section><h2>{w("Sebelum dan setelahnya", "Before and afterwards")}</h2><p>{w("Simpan tujuan, pertanyaan, dan hal yang ingin dibawa bersama ringkasanmu. Sesudahnya, catat yang penting dengan kata-katamu sendiri.", "Keep your goals, questions, and things to bring with your summary. Afterwards, note what matters in your own words.")}</p><LinkButton to="/support/summaries" variant="secondary">{w("Buka persiapanku", "Open my preparation")}</LinkButton></section>
    <section><h2>{w("Jeda untuk membaca", "A pause to read")}</h2><p>{w("Temukan panduan dari WHO dan NHS langsung di sumber resminya. Kartu latihan Sela akan tersedia setelah tinjauan editor.", "Find WHO and NHS guides at their official sources. Sela exercise cards will be available after editorial review.")}</p><LinkButton to="/reading" variant="secondary">{w("Jelajahi sumber bacaan", "Explore reading sources")}</LinkButton></section>
    <section><h2>{w("Bantuan terencana", "Planned support")}</h2><p>{w("Lihat peta kondisimu dari catatan yang kamu simpan, lalu temukan bantuan dan tenaga profesional yang cocok. Kamu yang menghubungi; tidak ada permintaan otomatis.", "See your condition map from the entries you saved, then find help and professionals that fit. You make contact; there is no automatic request.")}</p><LinkButton to="/support/directory" variant="secondary">{w("Cari bantuan terencana", "Find planned support")}</LinkButton></section>
    <section><h2>{w("Ruang berbagi", "Sharing space")}</h2><p>{w("Bagikan cerita secara anonim dan saling menguatkan dengan sesama. Setiap tulisan diperiksa dulu; perundungan langsung disembunyikan.", "Share stories anonymously and support each other. Everything is checked first; bullying is hidden at once.")}</p><LinkButton to="/support/community" variant="secondary">{w("Buka ruang berbagi", "Open the sharing space")}</LinkButton></section>
    <section><h2>{w("Bantuan manusia", "Human support")}</h2><p>{w("Layanan bantuan dengan sumber dan tanggal verifikasi. Bisa dibuka tanpa akun.", "Support services with sources and verification dates. Available without an account.")}</p><LinkButton to="/help" variant="help" icon="help">{w("Lihat layanan bantuan", "See support services")}</LinkButton></section></div>
    <section className="c-support-share" aria-labelledby="support-share-title"><div><h2 id="support-share-title">{w("Berbagi ringkasan", "Sharing a summary")}</h2><p>{w("Sela tidak menghubungi siapa pun. Mencari bantuan profesional selalu pilihanmu. Kamu dapat membagikan ringkasan yang sudah disetujui melalui tautan dan kode akses, lalu mencabutnya kapan saja.", "Sela contacts no one. Seeking professional help is always your choice. You can share an approved summary with a link and access code, then revoke it at any time.")}</p></div><LinkButton to="/support/shares" variant="secondary" icon="share">{w("Kelola tautan berbagi", "Manage share links")}</LinkButton></section>
  </div>;
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
          "Tautan berbagi, kode akses penerima, dan pencabutan akses belum tersedia. Ringkasan manual bisa disetujui dan diunduh dari halaman Ringkasan.",
          "Share links, recipient access codes, and access revocation are not available yet. Manual summaries can be approved and downloaded from Summaries.",
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
