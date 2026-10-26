# Produk dan Sela Relay

> Research/proposal reference. Authority: [SPEC.md](../../../SPEC.md); traceability: [package index](README.md#authority-and-spec-traceability). No runtime implementation or requirement change is implied.


Status: usulan. Tidak ada klaim kebaruan ilmiah atau hasil klinis.

## Masalah dan pengguna awal

Fokus awal yang realistis untuk solo builder adalah dewasa muda Indonesia yang mempertimbangkan konsultasi pertama dan menghadapi kebingungan menjelaskan pengalaman, biaya, waktu, privasi atau ketidakpastian langkah berikutnya. Kasus Inggris menunjukkan desain bilingual; ekspansi pasar lintas negara bukan scope demo.

Asumsi ini perlu wawancara. Mahasiswa tidak homogen: orang yang tahu ingin ke profesional memiliki kebutuhan berbeda dari orang yang hanya ingin menulis. Sela harus memungkinkan pengguna berhenti tanpa dipaksa mengungkap lebih banyak atau memesan layanan.

Job to be done: “Ketika aku merasa perlu bantuan tetapi bingung memulai, bantu aku menyiapkan apa yang ingin kusampaikan dan langkah yang bisa kulakukan, tanpa kehilangan kendali atas ceritaku.”

## Satu kapabilitas utama: Sela Relay

Relay adalah alur dengan empat mekanisme berikut. Demo sebaiknya memperlihatkannya sebagai satu perjalanan, bukan empat aplikasi terpisah.

| Mekanisme | Hasil untuk pengguna | Bukti yang bisa ditampilkan |
|---|---|---|
| Story Ledger | Ringkasan terhubung ke potongan cerita dan boleh dikoreksi | Klik satu klaim → lihat sumber, waktu, negasi dan versi |
| Next Step | Pilihan praktis sesuai bahasa, biaya, waktu dan format | Pengguna dapat menjelaskan mengapa opsi ditampilkan; empty state jujur |
| Opening Practice | Draf kalimat pembuka yang boleh diedit atau dilewati | Pengguna berlatih menjelaskan tujuan konsultasi tanpa AI berperan sebagai terapis |
| Handoff Receipt | Status permintaan dan respons penerima terlihat | Terkirim, diterima, butuh klarifikasi, kedaluwarsa dan dicabut merupakan state berbeda |

**Tambahan yang membedakan dari Care Passport lama:** menghubungkan review sumber, kesiapan praktis, konfirmasi penerima dan tindak lanjut dalam kontrak state yang dapat dibuktikan. Source grounding dan pembuatan kalimat pembuka sudah ada pada implementasi lama, termasuk `services/orchestrator/src/openingLinesPipeline.ts`; keduanya merupakan fondasi yang diwarisi dan perlu diadaptasi. Opening Practice adalah penyempurnaan alur, bukan klaim penemuan kemampuan baru.

### Story Ledger: makna tidak berubah diam-diam

Setiap item memuat sumber asli, parafrase yang diusulkan, rentang waktu, subjek, label ketidakpastian dan status user-confirmed. “Aku khawatir akan gagal” adalah kekhawatiran, bukan fakta bahwa pengguna gagal. “Kata dokter tahun lalu...” dipertahankan sebagai laporan pengguna mengenai dokter, bukan diagnosis Sela.

Model boleh mengusulkan satu pertanyaan klarifikasi yang berguna. Pengguna boleh memilih “belum tahu” atau melewati. Jangan mengejar kelengkapan dengan menginterogasi atau menambahkan detail.

Saat bahasa diganti, kutipan asli tetap terlihat. Terjemahan ditandai dan memerlukan persetujuan versi baru bila makna catatan yang dibagikan berubah. Kesamaan back-translation bukan bukti tunggal akurasi.

### Next Step: kurangi hambatan konkret

Tanyakan maksimal satu hambatan utama dari pilihan sederhana: biaya, jadwal, cara memulai, privasi, atau belum siap. Ini preferensi pengguna, bukan skor kesiapan klinis. Cabangkan langkah yang relevan; jangan menginfer kondisi mental dari pilihan tersebut.

Discovery menggunakan katalog dan aturan deterministik. Model boleh menjelaskan opsi yang ditemukan berdasarkan preferensi eksplisit. Ia tidak menentukan psikolog versus psikiater berdasarkan diagnosis, menjamin kecocokan klinis, mengarang slot atau menyatakan fasilitas menerima asuransi tanpa verifikasi.

### Opening Practice: bantuan berbicara

Contoh ID: “Saya ingin membahas rasa kewalahan yang mulai mengganggu kuliah. Saya belum yakin cara menjelaskannya, jadi saya membawa catatan ini.”

Contoh EN: “I would like to talk about feeling overwhelmed and how it affects my studies. I brought a note because I am unsure where to start.”

Kalimat hanya memuat detail dari fixture/catatan disetujui. Latihan ini tidak meniru konsultasi, memberi penilaian kesiapan, atau menjanjikan pengguna akan merasa lebih baik.

### Handoff Receipt: sampai ada respons

Permintaan yang dikirim belum berarti bantuan diterima. Penerima bisa acknowledge, meminta satu klarifikasi nonmedis, menolak karena kapasitas, atau belum merespons. Jika gagal, tawarkan kembali opsi praktis menggunakan catatan yang sudah disetujui, tanpa mengirim ulang ke penerima baru sebelum consent baru.

Professional view membaca snapshot yang dibagikan. Ia tidak melihat diary privat. Dalam demo, respons profesional dan rencana setelah konsultasi berasal dari fixture. AI tidak menulis terapi atau obat.

## Batas anonimitas

Pengguna boleh memulai tanpa identitas nyata. Jelaskan data apa yang diproses dan oleh siapa; pseudonim tidak identik dengan anonimitas teknis. Pada layanan nyata, penyedia mungkin memerlukan identitas, verifikasi usia dan informasi layanan; kebutuhannya harus ditampilkan sebelum pengguna memilih berbagi.

Untuk demo, semua identitas dan keputusan sintetis. Untuk produksi, perlukan mitra profesional, prosedur penanganan dan tanggung jawab, verifikasi kredensial, tata kelola data, serta review ketentuan yang berlaku. Jangan mengubah demo menjadi layanan klinis hanya dengan menyalakan database.

## Ukuran dampak

Target hipotesis: mengurangi hambatan administratif dan komunikasi pada perjalanan menuju bantuan. Indikatornya adalah keberhasilan tugas, pemahaman consent, kelengkapan catatan yang benar, waktu persiapan, jumlah koreksi dan penyelesaian handoff. Penurunan depresi, pencegahan bunuh diri dan efektivitas terapi bukan hasil yang dapat diklaim dari demo ini.

Riset dasar dan counterevidence: R012, R019, R023, R027, R031, R038, R047, R065, R087, R098–R100 pada [register](09_RESEARCH_REGISTER.md).
