# Roadmap solo dan validasi owner

> Research/proposal reference. Authority: [SPEC.md](../../../SPEC.md); traceability: [package index](README.md#authority-and-spec-traceability). No runtime implementation or requirement change is implied.


Status: rencana; seluruh checkbox adalah pekerjaan yang belum dibuktikan. Roadmap mengikuti task T2–T9 repo dan tidak menandai dokumen sebagai implementasi.

## Prioritas

P0: satu alur ID/EN dari fixture hingga shared brief, professional acknowledgement dan fixture follow-up; Go state, Meta drafting, consent dan manual path. P1: Opening Practice dan recovery ketika penerima menolak/tidak tersedia. P2: audio sintetis, dashboard evaluasi lebih luas dan integrasi nyata. Jangan mengerjakan P2 sebelum P0 tahan demo.

| Hari usulan | Hasil yang dapat diperiksa |
|---|---|
| 26 Okt | Konfirmasi acceptance, aturan, akses Meta; freeze baseline; amendment SPEC bila usulan diadopsi |
| 27 Okt | Skeleton Go/static web dan isolated sessions; fixture ID/EN |
| 28 Okt | Domain rules consent/version/revoke dan kontrak API |
| 29 Okt | Meta text + schema/source validation; unavailable/manual states |
| 30 Okt | Story Ledger dan discovery deterministik |
| 31 Okt | Shared snapshot dan professional acknowledgement |
| 1 Nov | Konsultasi scripted dan follow-up fixture; flow ID/EN lengkap |
| 2 Nov | Edge cases, usability, aksesibilitas dan evaluator sumber; potong fitur bila perlu |
| 3 Nov | Dry run deployment, security boundaries, rollback dan rekaman cadangan berlabel |
| 4 Nov | Feature freeze, video, provenance dan submission; sisakan buffer sebelum deadline |

Waktu efektif tergantung jam kickoff dan notifikasi acceptance. Ini rencana kerja, bukan komitmen bahwa semua implementasi selesai pada tanggal tersebut.

## Kartu uji owner

| ID | Tindakan | Hasil yang harus terlihat |
|---|---|---|
| O01 | Mulai sesi baru ID lalu ulangi EN | Seluruh langkah dan failure copy tersedia; data sintetis jelas |
| O02 | Tolak consent AI | Tidak ada inference; manual flow tetap mencapai handoff simulasi |
| O03 | Generate dari fixture dengan negasi | Tidak ada perubahan negasi/subjek/waktu; klik klaim menampilkan sumber |
| O04 | Model mengembalikan sumber palsu/JSON rusak | Draft ditolak; tidak ada auto-approval; manual tersedia |
| O05 | Edit brief yang sudah disetujui | Version bertambah, approval hilang; stale share ditolak |
| O06 | Share satu field kepada penerima A | Professional view A hanya melihat snapshot terpilih |
| O07 | Buka dengan sesi lain atau penerima B | Tidak bisa membaca/mengubah handoff A |
| O08 | Cabut saat inference/acknowledge berlangsung | Respons terlambat tidak memulihkan akses atau state lama |
| O09 | Klik share dua kali | Satu request/receipt; perubahan payload dengan key lama gagal eksplisit |
| O10 | Penerima belum acknowledge | UI belum menyebut bantuan diterima/konsultasi selesai |
| O11 | Penerima menolak/slot kosong | Empty state; opsi lain; consent baru sebelum berbagi ulang |
| O12 | Selesaikan konsultasi scripted | Rencana hanya dari fixture; tidak ada obat/terapi dari model |
| O13 | Jalankan crisis fixture | Tidak ada model call, normal booking atau completed consultation |
| O14 | Timeout dan API unavailable | Status jujur, operation berhenti, manual/recorded diberi label |
| O15 | Reset/expiry/restart | State lama tidak bisa diakses; pengguna tahu progres demo berakhir |
| O16 | Pakai keyboard, layar mobile dan reduced motion | Fokus jelas; semua aksi/status bisa dipahami dan dioperasikan |
| O17 | Periksa browser bundle dan log | Tidak ada credential, prompt, catatan privat atau identitas di operational logs |
| O18 | Ganti bahasa setelah approval | Tidak ada perubahan makna tersembunyi; changed brief perlu reapproval |

Owner dapat menjalankan walkthrough manual. Implementasi tetap memerlukan bukti engineering yang sesuai, terutama state transitions, authorization dan concurrency. Dokumen ini tidak memerintahkan menghapus tes atau mengklaim seluruh tes sudah lulus.

## Definition of demo ready

Alur P0 bekerja pada deployment yang akan ditonton, model live terverifikasi, failure path jujur, tidak ada bypass consent yang diketahui, ID/EN lengkap dan sumber/versi dapat ditunjukkan. Ada bukti commit/version-specific dan daftar limitations. Jika live access belum ada, labeli sebagai demo recorded/manual; jangan mengklaim kriteria tooling sudah terpenuhi.

## Sesudah hackathon

Lakukan discovery dengan calon pengguna dan profesional. Prioritaskan satu mitra layanan dan satu daerah sebelum katalog luas. Uji alur administratif yang diawasi, tetapkan SLA/respons yang benar, tanggung jawab krisis, credential verification, retensi dan persetujuan data. Penanganan profesional nyata berarti proses layanan yang disepakati dan dijalankan orang berwenang, bukan sekadar tombol di UI.
