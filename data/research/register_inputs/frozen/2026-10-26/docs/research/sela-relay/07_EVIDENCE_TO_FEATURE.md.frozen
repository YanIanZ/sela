# Peta bukti → keputusan produk

> Research/proposal reference. Authority: [SPEC.md](../../../SPEC.md); traceability: [package index](README.md#authority-and-spec-traceability). No runtime implementation or requirement change is implied.


Interpretasi produk di bawah adalah inferensi desain. Sumber tidak menguji Sela dan tidak membuktikan kombinasi Sela Relay efektif.

| Bukti pada register | Temuan yang relevan | Keputusan Sela | Batas inferensi |
|---|---|---|---|
| R098; R019 | Akses, biaya, stigma dan faktor sosial memengaruhi pencarian bantuan | Filter biaya/waktu, opsi privasi dan langkah praktis | Sebuah app tidak menghapus kekurangan tenaga profesional |
| R012; R038 | Pengguna membutuhkan cara masuk layanan dan informasi yang dapat dipahami | Satu next step, ekspektasi konsultasi dan navigasi jelas | Usability bukan bukti perubahan perilaku jangka panjang |
| R023 | Care navigation berasosiasi dengan penggunaan/retensi layanan lebih baik dalam kohort AS | Handoff sampai ada respons dan recovery bila gagal | Observasional, konteks layanan berbeda; efek kausal belum pasti |
| R027 | Warm handoff tatap muka dikaitkan dengan odds attendance lebih rendah daripada referral internal tradisional pada studi remaja | Uji jalur pengguna; jangan wajibkan warm handoff untuk semua | Temuan konteks tertentu; bukan bukti seluruh handoff buruk |
| R047 | Pilot RCT Indonesia memperbaiki literasi tetapi tidak memperbaiki help-seeking intention atau outcome gejala dibanding kontrol | Jangan berhenti pada edukasi; ukur penyelesaian langkah | Studi trauma mahasiswa, bukan seluruh pengguna Sela |
| R031; R035 | Pemahaman tujuan data dan review kolaboratif penting bagi keterlibatan | Catatan bisa diperiksa, dikoreksi dan dibahas | Belum membuktikan setiap dashboard akan meningkatkan engagement |
| R045; R043 | Transparansi dan consent sharing memerlukan implementasi konkret | Grant per penerima/versi/field; preview dan revoke | Framework consent bukan jaminan kepatuhan Indonesia |
| R057; R058 | Performa dan bias berubah menurut bahasa dan demografi | Eval ID/EN serta code-switching; sumber asli berdampingan | Tidak boleh memindahkan angka English/Korean ke Indonesia |
| R096; R065 | Summarization menjanjikan tetapi dapat menghasilkan error/confabulation | Grounding, pemeriksaan negasi dan human approval | Ringkasan lancar tidak identik dengan benar |
| R069; R068 | Evaluasi kualitas ringkasan perlu struktur dan penilaian klinis | Rubrik completeness, correctness, clarity, utility; cek safety terpisah | Adaptasi rubrik untuk Sela bukan instrumen tervalidasi baru |
| R087 | Trial chatbot waiting-list dihentikan tanpa data evaluable karena engagement/rekrutmen | Sederhanakan perjalanan; jangan memaksa AI atau delapan sesi | Studi remaja Jepang, bukan estimasi dropout Sela |
| R090; R092 | Engagement harus dikaitkan dengan tujuan dan outcomes | Ukur task completion dan koreksi, bukan waktu chat | Lebih banyak klik bukan manfaat |
| R099; R075; R081 | Chatbot dapat gagal dalam respons berisiko dan praktik terapeutik | Batasi model sebagai drafting/navigation assistant | Rule gate dan pembatasan scope bukan sertifikasi keselamatan |
| R097; R100 | Ada sinyal manfaat pada intervensi terstruktur yang diteliti | Tetap evaluasi dan jangan menolak AI tanpa bukti | Efek produk lain tidak dapat diklaim sebagai efikasi Sela |

## Hipotesis yang dapat digugurkan

H1: Story Ledger mengurangi fakta tidak didukung dan mempercepat koreksi dibanding ringkasan AI biasa. H2: Next Step membantu menyelesaikan tugas pencarian opsi dibanding direktori tanpa panduan. H3: Opening Practice meningkatkan kejelasan tujuan konsultasi yang dinilai dengan rubrik tugas. H4: Handoff Receipt meningkatkan pemahaman status dan penyelesaian langkah berikutnya pada simulasi.

Jika H3 menambah beban atau membuat pengguna merasa dipaksa, jadikan opsional atau hilangkan. Jika H1 tidak mengalahkan form manual, jangan menyembunyikan hasil; pertimbangkan form sebagai jalur utama. Jika H4 hanya memperbaiki visual status tanpa membantu penyelesaian tugas, kecilkan klaimnya.

## Rancangan evaluasi engineering dan usability

Bandingkan tiga kondisi pada fixture yang sama: A form manual, B ringkasan Meta tanpa ledger, C Meta + ledger + consent/handoff. A vs B menguji drafting; B vs C mencampur beberapa mekanisme sehingga lakukan ablation tambahan jika ingin mengatribusikan dampak ke satu mekanisme. Randomisasikan urutan dan cocokkan tugas agar efek belajar tidak mendominasi.

Gunakan set development dan held-out terpisah. Laporkan model/version, prompt version, bahasa, fixture dan denominator. Ground truth berupa fakta sumber yang ditulis/ditelaah manusia, bukan penilaian model atas jawabannya sendiri. Untuk readiness/usability, gunakan peserta dewasa sehat dengan tugas sintetis dan consent riset yang sesuai; jangan mengumpulkan cerita klinis mereka.

Metrik yang disarankan:

- Unsupported claim rate = klaim faktual yang tidak didukung / seluruh klaim faktual yang dihasilkan. Pisahkan angka raw sebelum validator dan residual sesudah validator.
- Coverage = fakta sumber penting yang tetap benar / fakta penting pada reference set. Periksa negasi, subjek, waktu dan uncertainty secara terpisah.
- Correction burden = jumlah edit bermakna dan waktu review sampai brief disetujui.
- Consent comprehension = jawaban benar tentang penerima, field, versi dan pencabutan / seluruh pertanyaan consent.
- Journey task success = peserta yang menyelesaikan tugas tanpa bantuan / seluruh peserta yang memulai tugas; laporkan dropout.
- Authorization violation = pembacaan atau mutasi yang berhasil di luar scope; target acceptance nol pada kasus teruji, bukan klaim nol risiko global.
- Latency/cost = end-to-end serta inference terpisah, termasuk timeout/failure; semua biaya aktual, bukan tebakan.

Ukuran sampel demo/usability kecil hanya deskriptif. Tentukan jumlah peserta berdasarkan feasibility dan jelaskan ketidakpastian; tidak ada klaim signifikansi atau power yang dibuat tanpa desain dan analisis yang sesuai.
