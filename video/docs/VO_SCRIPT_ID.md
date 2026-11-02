# SELA — Beyond Being Heard · Naskah voice-over (ID) + subtitle (EN)

**Status:** naskah siap rekam. Belum direkam. Master yang dirender adalah versi **musik + tipografi tanpa narasi**; narasi ini untuk versi kedua.

**Sumber waktu:** `docs/vo_cues.tsv` (satu sumber). `python3 scripts/cues.py` membuat `subtitles/master.id.srt` dan `subtitles/master.en.srt`, dan memberi peringatan bila subtitle Inggris lebih cepat dari 17 karakter/detik.

## Arah suara

- Satu suara dewasa, tenang, dekat ke mikrofon. Bukan suara iklan. Tidak dramatis.
- Tempo pelan dan jelas. Kalau satu baris tidak muat, potong kata lain. Jangan percepat label keamanan.
- Baris yang **wajib ada dan tidak boleh dipotong**:
  - "Kesalahan ini sengaja dibuat untuk pengujian. Bukan keluaran asli dari Meta." (01:45)
  - "Di demo ini, penerimanya layanan simulasi. Bukan klinik atau praktisi sungguhan." (02:33)
  - "Salinan yang sudah dilihat tidak bisa ditarik." (02:39)
  - "Ini lembar persiapan pribadi, bukan rekam medis atau diagnosis." (02:18)
- Musik turun sekitar 6 dB di bawah narasi (sidechain atau volume manual). Hit tipografi tetap terdengar.

## Naskah

| Waktu | Bab | Narasi (ID) | Subtitle (EN) |
| --- | --- | --- | --- |
| 00:00.8 | 01 | Kadang, yang paling sulit bukan mencari jawaban. | Some days, the hardest part isn't finding an answer. |
| 00:04.8 | 01 | Tapi menjelaskan apa yang sebenarnya kita rasakan. | It's explaining what we really feel. |
| 00:10.2 | 01 | Banyak aplikasi bisa mendengar. Tapi siapa yang benar-benar paham? | Many apps can listen. But who really understands? |
| 00:20.6 | 02 | AI bisa mendengar. Belum tentu paham. | AI can listen. That doesn't mean it understands. |
| 00:28.6 | 02 | Ini Nadia, tokoh fiktif. Dia menulis: "Kadang aku butuh waktu sendiri." | This is Nadia, a fictional adult. She writes: "Sometimes I need time alone." |
| 00:33.4 | 02 | Yang Sela tangkap cuma tafsiran. Nadia yang memutuskan: setuju, koreksi, atau tolak. | What Sela notices is only an interpretation. Nadia decides: agree, correct or reject. |
| 00:40.4 | 03 | Satu kata bisa mengubah makna. "Kadang" jadi "selalu". | One word can change the meaning. "Sometimes" becomes "always". |
| 00:45.5 | 03 | **Kesalahan ini sengaja dibuat untuk pengujian. Bukan keluaran asli dari Meta.** | **This mistake is a scripted test fixture, not a real Meta output.** |
| 00:50.4 | 03 | Pemeriksa makna di Sela langsung menandainya. | Sela's meaning check flags it right away. |
| 00:53.4 | 03 | Nadia mengoreksi dengan kata-katanya sendiri. Sebelum disimpan, Sela menunjukkan apa saja yang ikut berubah. | Nadia corrects it in her own words. Before saving, Sela shows what it will affect. |
| 00:58.8 | 03 | Di Sela, koreksi bukan sekadar edit chat. | In Sela, a correction is more than a chat edit. |
| 01:01.8 | 03 | Meaning Integrity Engine menelusuri semua yang bergantung pada makna itu. | The Meaning Integrity Engine traces everything that rests on it. |
| 01:06.0 | 03 | Ringkasan perlu ditinjau ulang. Tautan dihentikan. Permintaan dibatalkan. | The summary needs review. Links stop. The request is cancelled. |
| 01:10.6 | 03 | Yang sudah dibaca orang lain tidak bisa ditarik kembali. | What others already read cannot be recalled. |
| 01:15.4 | 04 | Cerita tidak berhenti di satu momen. | A story doesn't stop at one moment. |
| 01:23.0 | 04 | Nadia bisa melihat catatannya dari waktu ke waktu, dengan kata-katanya sendiri. | Nadia can look back at her notes over time, in her own words. |
| 01:29.4 | 04 | Tanpa skor. Tanpa diagnosis. Halaman ini tidak memakai AI. | No scores. No diagnosis. This page uses no AI. |
| 01:40.4 | 05 | Memahami adalah awal. | Understanding is a beginning. |
| 01:48.0 | 05 | Kalau mau, Nadia memilih satu langkah kecil: menulis dua pertanyaan untuk konsultasi. | If she wants, Nadia picks one small step: writing two questions for a consultation. |
| 01:54.8 | 05 | Tidak ada pengingat, tidak ada target. Boleh dilewati. | No reminders, no targets. She can skip it. |
| 02:00.4 | 06 | Kata-katanya. Pilihannya. Persetujuannya. | Her words. Her choice. Her approval. |
| 02:07.0 | 06 | Nadia memilih kutipan asli dari catatannya. Catatan lain tetap pribadi. | Nadia picks original quotes from her notes. Other notes stay private. |
| 02:14.0 | 06 | Dia menyetujui versi yang persis akan dibagikan. | She approves the exact version that will be shared. |
| 02:18.8 | 06 | **Ini lembar persiapan pribadi, bukan rekam medis atau diagnosis.** | **This is a personal preparation sheet, not a medical record or diagnosis.** |
| 02:25.4 | 07 | Saat Nadia siap, dukungan mengikuti aturannya. | When Nadia is ready, support follows her terms. |
| 02:33.0 | 07 | **Di demo ini, penerimanya layanan simulasi. Bukan klinik atau praktisi sungguhan.** | **In this demo, the recipient is a simulated service. Not a real clinic or practitioner.** |
| 02:39.4 | 07 | Nadia bisa membatalkan kapan saja. **Salinan yang sudah dilihat tidak bisa ditarik.** | Nadia can cancel anytime. **Copies already viewed cannot be recalled.** |
| 02:46.0 | 08 | AI seharusnya membantu kita menjelaskan cerita, bukan mengambil alih maknanya. | AI should help us explain our story, not take over its meaning. |
| 02:50.4 | 08 | Ceritamu. Maknamu. Langkahmu berikutnya. | Your story. Your meaning. Your next step. |
| 02:56.4 | 08 | Sela. Lebih dari sekadar didengar. | Sela. Beyond being heard. |

## Cara menambah narasi ke master

```sh
# narasi.wav = rekaman 180 detik yang sudah dipotong sesuai waktu di atas
ffmpeg -i out/SELA-Beyond-Being-Heard-master-1080p.mp4 -i narasi.wav -filter_complex \
  "[0:a]volume=0.5[m];[m][1:a]amix=inputs=2:duration=first:normalize=0[a]" \
  -map 0:v -map "[a]" -c:v copy -c:a aac -b:a 256k out/SELA-master-vo.mp4
# subtitle Inggris sebagai track terpisah (bisa dimatikan penonton)
ffmpeg -i out/SELA-master-vo.mp4 -i subtitles/master.en.srt -map 0 -map 1 -c copy -c:s mov_text \
  -metadata:s:s:0 language=eng out/SELA-master-vo-sub.mp4
```
