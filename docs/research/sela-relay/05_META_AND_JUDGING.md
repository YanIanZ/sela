# Meta, penjurian dan demo

> Research/proposal reference. Authority: [SPEC.md](../../../SPEC.md); traceability: [package index](README.md#authority-and-spec-traceability). No runtime implementation or requirement change is implied.


Sumber resmi diperiksa 26 Oktober 2026. Bukan jaminan penerimaan atau kemenangan.

## Kesesuaian event

Sela cocok secara konseptual dengan Indie Developer dan tema Build for Your World. Halaman event menyebut deadline submission 4 November 2026 pukul 21.00 ET, yaitu 5 November pukul 09.00 WIB. Cek kembali portal karena jadwal dapat berubah.

Waiver yang diambil memuat judul Official Rules, bobot penjurian dan izin komponen open source dengan kepatuhan lisensi serta peningkatan atas produk dasar. Sela perlu mendokumentasikan asal Dengar.in, kontribusi tim, dan pekerjaan baru secara jujur. Kontribusi besar oleh maintainer tidak otomatis membuat seluruh karya tim menjadi milik tunggalnya. Mempertahankan lisensi belum menyelesaikan semua persyaratan hak; gunakan pernyataan ownership yang benar-benar dapat dipenuhi. Detail acceptance, kebijakan model yang relevan dan persyaratan video final belum diverifikasi.

## Pemakaian Meta yang substantif

| Tahap | Peran Meta | Kendali aplikasi |
|---|---|---|
| Structured intake | Usulkan fakta deskriptif, sumber, unknown dan satu klarifikasi | Go memeriksa sumber/negasi dan membatasi scope |
| Read-only tool use | Ambil potongan katalog fiktif atau arti field yang belum tersedia | Allowlist tools, input tervalidasi; matching deterministik |
| Opening practice | Susun kalimat pembuka dari brief yang disetujui | Tidak menambah gejala atau melakukan terapi |
| Bilingual rendering | Parafrase ID/EN dengan kutipan asli berdampingan | Review ulang bila perubahan makna; eval bahasa terpisah |
| Professional brief | Susun tampilan ringkas dari shared snapshot | Tidak mengambil diary privat atau mengarang rencana medis |

Dua panggilan model tidak otomatis memberi kedalaman. Pertahankan jumlah operasi minimum; nilai tambah ada pada fidelity, tool grounding, kontrak state dan penanganan kegagalan. Evaluator dengan model yang sama hanya sinyal tambahan, bukan ground truth independen.

Dokumentasi Meta resmi mendeskripsikan structured output dan function calling. Contoh docs memakai `https://api.meta.ai/v1` dan `muse-spark-1.3`; ini contoh dokumentasi yang diperiksa, **bukan bukti akun Sela memiliki akses**. Pilih endpoint/model setelah probe akses yang sah. Kunci tidak masuk browser atau repo. Structured output menjamin bentuk sesuai kapabilitas provider, bukan kebenaran isi.

Jangan memakai image generation, segmentation atau speech hanya demi daftar fitur. Audio sintetis dapat menjadi stretch goal jika benar-benar meningkatkan aksesibilitas dan kualitas ID/EN terverifikasi. Text-first tetap jalur utama.

## Peta penjurian

| Kriteria | Strategi Sela | Bukti yang perlu ada |
|---|---|---|
| Impact 30% | Selesaikan hambatan konsultasi pertama dan kegagalan handoff | Masalah pengguna, pembanding, metrik task completion; tanpa klaim klinis palsu |
| Meta tooling 25% | Source-linked structured brief, bounded tool calls, bilingual fidelity | Live model trace yang disanitasi, source inspection, baseline tanpa pipeline |
| Technical depth 20% | Go domain rules, version-bound consent, isolation, cancellation dan idempotency | Bukti edit→reapproval, revoke, concurrent request, timeout dan manual fallback |
| Demo 25% | Satu perjalanan emosional yang jelas dari pengguna hingga penerima | Live slice, penanda simulasi, video ringkas dan recovery path |

## Skrip demo usulan 3 menit

Durasi ini rekomendasi internal, bukan ketentuan event yang sudah diverifikasi.

- 0:00–0:20 — Tampilkan fixture dewasa muda: ingin konsultasi tetapi bingung memulai dan khawatir biaya. Jelaskan semua data/konsultasi fiktif.
- 0:20–0:55 — Consent AI, panggilan Meta live dan Story Ledger. Klik satu klaim untuk melihat sumber. Tunjukkan negasi dipertahankan.
- 0:55–1:20 — Tentukan preferensi, pilih profesional fiktif dan lihat kalimat pembuka.
- 1:20–1:50 — Review snapshot, approve versi, share secara eksplisit. Tunjukkan state menunggu penerima.
- 1:50–2:15 — Buka professional demo: hanya data terpilih. Acknowledge dan selesaikan konsultasi scripted.
- 2:15–2:35 — User melihat rencana fixture dan langkah tindak lanjut yang dipilih.
- 2:35–3:00 — Cabut akses, tampilkan denial penerima. Perlihatkan ringkasan bukti engineering dan ID/EN.

Video tambahan singkat dapat menampilkan no-consent, unavailable model, no-match dan crisis fixture. Jangan menampilkan seluruh edge case dalam cerita utama sampai kehilangan alur.

## Kejujuran live demo

Setiap keluaran berlabel `live_meta`, `recorded_demo` atau `manual`. Jika API tidak tersedia, jelaskan dan gunakan manual/recorded berlabel jelas. Video sukses sebelumnya tidak membuktikan live inference saat ini. Tampilan latency model dipisah dari latency aplikasi.

## Referensi resmi

- [Event](https://cerebralvalley.ai/e/meta-ai-developer-hackathon)
- [Official Rules dan waiver](https://cerebralvalley.ai/e/meta-ai-developer-hackathon/waiver)
- [Structured output](https://dev.meta.ai/docs/structured-output)
- [Tool calling](https://dev.meta.ai/docs/tool-calling)
- [Structured output cookbook](https://dev.meta.ai/docs/cookbook/structured-output)
