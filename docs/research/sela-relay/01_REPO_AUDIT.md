# Audit terarah repo Sela

> Research/proposal reference. Authority: [SPEC.md](../../../SPEC.md); traceability: [package index](README.md#authority-and-spec-traceability). No runtime implementation or requirement change is implied.


Tanggal 26 Oktober 2026; baseline `c51807ccdc5c3bc0df308b5f40d3d68ae8a6e1fd`. Audit membaca struktur recursive, README, SPEC, PRODUCT, ARCHITECTURE, META_INTEGRATION, META_EVENT_NOTES, ROADMAP, SAFETY, VALIDATION, SELA_PROVENANCE serta bagian implementasi pipeline dan validator Care Passport. Ini bukan audit keamanan seluruh codebase atau verifikasi runtime.

## Kondisi yang teramati

| Area | Bukti repo | Implikasi |
|---|---|---|
| Identitas | README sudah memakai Sela dan tagline | Rebranding dokumen sudah dimulai |
| Backend | SPEC menetapkan Go; tree belum memuat runtime Go | Refaktor implementasi masih harus dikerjakan |
| Frontend | `apps/web` berisi Next.js/TypeScript; target React/Vite statis | UI baru perlu dipisahkan dari API Next lama |
| AMD | `services/orchestrator/src/carePassportPipeline.ts` masih memakai default AMD provider | Mengganti logo tidak mengganti jalur inference |
| Care Passport | Pipeline consent/crisis/clarification; validator mengaitkan kutipan ke segmen; opening-lines pipeline sudah tersedia | Grounding dan approval bukan ide baru khusus Meta |
| Bahasa | Ada modul i18n ID/EN | Terjemahan lama bisa menjadi inventaris, tetap perlu audit makna |
| Kesinambungan | Dokumen v2 sudah mendesain discovery, brief, handoff, konsultasi dan follow-up sintetis | Perkuat kontrak antar tahap; hindari mendesain ulang konsep sama dengan nama baru |
| Infrastruktur | PostgreSQL/Redis dan berbagai service masih ada di v1; v2 merencanakan memory store | Infrastruktur lama tidak otomatis wajib bagi demo Go |
| Bukti mutu | VALIDATION membedakan pemeriksaan v1/docs dari v2 | Badge lama tidak membuktikan implementasi baru |
| Kontribusi | Provenance menyebut asal Dengar.in dan mempertahankan MIT | Presentasikan karya baru solo sambil menjaga kredit karya tim |

## Pertahankan, rancang ulang, tunda

**Pertahankan perilaku:** consent AI terpisah, pemeriksaan sumber, unknown fields, jalur manual, pembatasan keluaran medis, isolasi sesi, provenance dan perlindungan data. Port perilakunya ke Go; jangan menganggap kode TypeScript dapat dipindahkan tanpa perubahan.

**Rancang ulang:** navigasi berpusat pada satu perjalanan, state backend, scope-sharing per versi, penerimaan oleh penerima, visualisasi perubahan catatan, dan pemulihan ketika handoff gagal.

**Tunda dari demo:** forum, gamifikasi, marketplace profesional, pembayaran, diagnosis otomatis, model training dan penambahan banyak provider. Ini keputusan fokus implementasi sesuai SPEC saat ini, bukan penilaian bahwa seluruh fitur tersebut tidak berguna.

## Risiko desain yang perlu ditutup

1. Kutipan yang cocok secara tekstual belum membuktikan kesimpulan model benar. “Tidak susah tidur” tidak mendukung “susah tidur”. Periksa negasi, waktu, subjek, ketidakpastian dan siapa yang membuat pernyataan.
2. Keyword crisis gate memberi hasil deterministik pada kasus yang diketahui; ia tidak membuktikan sensitivitas klinis pada bahasa alami. Demo seeded dan input bebas produksi memerlukan penilaian berbeda.
3. Memory store memudahkan demo, tetapi tidak menyediakan kontinuitas produksi setelah restart. UI harus mengatakan sesi telah berakhir, bukan menggambarkan data aman tersimpan.
4. Pergantian role pada demo bukan autentikasi profesional. Semua professional view dan konsultasi tetap fiktif.
5. Revoke hanya dapat memblokir akses berikutnya di sistem; ia tidak menghapus informasi yang sudah dilihat, disalin atau sah disimpan penerima.

## Batas perubahan

Usulan paket ini tidak mengganti constraint user-confirmed pada SPEC. Perubahan produk yang disetujui perlu ditulis sebagai amendment SPEC sebelum implementasi, terutama untuk free-text input, data nyata, deployment persisten atau layanan klinis nyata.

Sumber dapat dibuka pada [repo baseline](https://github.com/YanIanZ/sela/tree/c51807ccdc5c3bc0df308b5f40d3d68ae8a6e1fd).
