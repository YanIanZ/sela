# SELA — Demo Produk 3 Menit · Naskah dubbing (ID) + subtitle (EN)

**File video:** `out/SELA-Product-Demo-3min-1080p-EN-subs.mp4` (subtitle Inggris sudah tertanam).
**Sumber waktu:** `src/demo/vo.json` → `node scripts/demo-plan.mjs` → `subtitles/demo.id.srt` dan `subtitles/demo.en.srt`.

## Cara bikin dubbing

1. **Rekam sendiri (paling bagus).** Rekam tiap baris jadi satu file: `01.wav`, `02.wav`, dan seterusnya, sesuai nomor di tabel. Jaga durasi tiap file di bawah kolom "Maks". Lalu jalankan:
   ```sh
   python3 scripts/dub.py --lines ~/rekaman/sela-demo
   ```
2. **Pakai suara TTS** (butuh internet di laptopmu):
   ```sh
   pip install edge-tts
   python3 scripts/dub.py --tts edge --voice id-ID-GadisNeural   # atau id-ID-ArdiNeural
   ```

`dub.py` menaruh tiap suara tepat di waktunya, mempercepat sedikit (maks 1,18×) kalau kepanjangan, menurunkan musik sekitar 9 dB saat ada suara, menormalkan ke −16 LUFS, lalu menyimpan `out/SELA-Product-Demo-3min-1080p-ID-dub-EN-subs.mp4`. Video tidak di-encode ulang. Laporan di akhir menandai baris yang masih kepanjangan.

## Arah suara

- Satu suara dewasa, tenang, jelas. Bukan suara iklan.
- Baris yang **tidak boleh dipotong**: 04 (program uji, bukan Meta), 08 (kesalahan sengaja dibuat), 16 (yang sudah dibaca tidak bisa ditarik), 24 (bukan paspor medis), 29 (layanan simulasi), 31 (tidak ada janji temu sungguhan), 33 (salinan tidak bisa ditarik).

## Naskah

| # | Mulai | Maks | Dubbing (ID) | Subtitle (EN) |
| --- | --- | --- | --- | --- |
| 01 | 0:00.5 | 3.5 s | Kadang, yang paling sulit bukan mencari jawaban. | Some days, the hardest part isn't finding an answer. |
| 02 | 0:04.1 | 4.1 s | Tapi menjelaskan apa yang sebenarnya kita rasakan. | It's explaining what we really feel. |
| 03 | 0:08.4 | 4.1 s | Ini Sela. Nadia, tokoh fiktif, mulai bercerita. | This is Sela. Nadia, a fictional adult, starts to write. |
| 04 | 0:13.4 | 4.6 s | Balasan di demo ini dari program uji, bukan dari Meta. Labelnya selalu terlihat. | Replies here come from a labelled test program, not from Meta. |
| 05 | 0:18.1 | 3.9 s | Sela menangkap maksudnya. Ini cuma tafsiran, bukan fakta. | Sela notices what she meant: an interpretation, not a fact. |
| 06 | 0:22.1 | 6.1 s | Pemeriksa makna membandingkannya dengan kata-kata Nadia. Nadia yang memutuskan. | A meaning check compares it with her words. Nadia decides. |
| 07 | 0:30.5 | 4.1 s | Bagaimana kalau tafsiran yang salah sempat tersimpan? | But what if a wrong interpretation was already saved? |
| 08 | 0:34.7 | 5.0 s | Ini kesalahan yang sengaja dibuat untuk pengujian: "kadang" jadi "selalu". | This is a scripted test mistake: "sometimes" became "always". |
| 09 | 0:39.8 | 3.9 s | Pemeriksa makna menandainya sebagai perubahan penting. | The meaning check flags it as an important change. |
| 10 | 0:45.5 | 3.7 s | Nadia memperbaikinya dengan kata-katanya sendiri. | Nadia fixes it in her own words. |
| 11 | 0:50.3 | 5.5 s | Sebelum disimpan, Sela menunjukkan apa saja yang ikut berubah. | Before saving, Sela shows everything this will affect. |
| 12 | 0:57.9 | 4.2 s | Satu koreksi diteruskan ke semua yang bergantung padanya. | One correction reaches everything that depends on it. |
| 13 | 1:02.2 | 5.9 s | Meaning Integrity Engine menelusuri ringkasan, tautan, dan permintaan dalam satu langkah. | The Meaning Integrity Engine traces summaries, links and requests in one step. |
| 14 | 1:08.5 | 3.7 s | Ringkasan lama perlu ditinjau ulang. | The old summary needs review again. |
| 15 | 1:12.7 | 3.3 s | Tautan berbaginya dihentikan. | Its share link is stopped. |
| 16 | 1:16.3 | 6.9 s | Permintaan ke layanan simulasi dibatalkan. Yang sudah dibaca orang lain tidak bisa ditarik kembali. | The request to the simulated service is cancelled. What others already read can't be recalled. |
| 17 | 1:23.7 | 4.3 s | Perjalanan ceritaku menampilkan catatan Nadia dari waktu ke waktu. | "My story over time" shows Nadia's notes across the days. |
| 18 | 1:28.2 | 5.1 s | Kata-katanya sendiri, dengan versi sumbernya. Tanpa AI, tanpa skor, tanpa diagnosis. | Her own words, with source versions. No AI, no scores, no diagnosis. |
| 19 | 1:33.4 | 3.9 s | Dia memilih dua catatan untuk dibawa ke ringkasan. | She picks two notes to bring to a summary. |
| 20 | 1:39.8 | 3.7 s | Kalau mau, Nadia memilih satu langkah kecil. | If she wants, Nadia picks one small step. |
| 21 | 1:44.4 | 4.1 s | Dia menulisnya sendiri: dua pertanyaan untuk konsultasi. | She writes it herself: two questions for a consultation. |
| 22 | 1:50.4 | 3.7 s | Tidak ada pengingat, tidak ada hitungan hari. Boleh dilewati. | No reminders, no day counts. She can skip it. |
| 23 | 1:54.2 | 1.9 s | Lalu ringkasan untuk dibawa. | Next, a sheet to bring along. |
| 24 | 1:56.2 | 5.1 s | Sela menulisnya dengan jelas: ini bukan paspor medis, bukan rekam medis, bukan diagnosis. | Sela says it clearly: not a medical passport, record or diagnosis. |
| 25 | 2:02.8 | 4.5 s | Setiap kalimat menunjuk ke kutipan asli dan versi sumbernya. | Each statement points to an original quote and its source version. |
| 26 | 2:08.3 | 4.5 s | Pratinjau ini persis sama dengan yang akan diterima penerima. | This preview is exactly what a recipient would receive. |
| 27 | 2:13.8 | 5.3 s | Nadia menyetujui versi yang persis ini. Hanya versi yang disetujui yang bisa dibagikan. | Nadia approves this exact version. Only approved versions can be shared. |
| 28 | 2:21.1 | 3.9 s | Saat siap, Nadia bisa mengirim ringkasan ke layanan. | When she's ready, Nadia can send the sheet to a service. |
| 29 | 2:25.5 | 5.3 s | Di demo ini penerimanya layanan simulasi, bukan klinik atau praktisi sungguhan. | Here the recipient is a simulated service, not a real clinic or practitioner. |
| 30 | 2:31.9 | 3.5 s | Layanan mengusulkan waktu. Nadia yang memilih. | The service proposes times. Nadia chooses. |
| 31 | 2:36.7 | 3.3 s | Tidak ada janji temu sungguhan di demo ini. | No real appointment is made in this demo. |
| 32 | 2:40.1 | 3.5 s | Nadia bisa mencabut akses kapan saja. | Nadia can revoke access at any time. |
| 33 | 2:46.1 | 4.3 s | Akses berikutnya diblokir. Salinan yang sudah dilihat tidak bisa ditarik. | Future access is blocked. Copies already viewed can't be recalled. |
| 34 | 2:50.5 | 4.3 s | AI seharusnya membantu kita menjelaskan cerita, bukan mengambil alih maknanya. | AI should help us explain our story, not take over its meaning. |
| 35 | 2:54.9 | 3.2 s | Ceritamu. Maknamu. Langkahmu berikutnya. | Your story. Your meaning. Your next step. |
| 36 | 2:58.2 | 1.7 s | Sela. Lebih dari sekadar didengar. | Sela. Beyond being heard. |
