# Metode evidence map 100 publikasi

> Research/proposal reference. Authority: [SPEC.md](../../../SPEC.md); traceability: [package index](README.md#authority-and-spec-traceability). No runtime implementation or requirement change is implied.


Tanggal pemeriksaan 26 Oktober 2026. Batas usia ditafsirkan secara ketat: publikasi pertama setelah 26 Oktober 2021 dan tidak melewati 26 Oktober 2026. Seluruh 100 item terpilih berada pada 2023–2026 berdasarkan metadata yang diperiksa.

## Apa yang sudah dilakukan

Menelusuri Europe PMC/PubMed pada sepuluh tema: conversational AI, help-seeking, care navigation, continuity/shared decisions, privacy/consent, Indonesia, language equity, clinical summarization, safety dan engagement. Metadata dan abstrak diambil dari API Europe PMC dengan `resultType=core`. Pencarian tambahan menargetkan DOI/PMID karya jangkar. Artikel ACM dan Therabot ditambahkan melalui sumber penerbit/penulis.

Seleksi memeriksa judul, tanggal, DOI, duplikasi, jenis publikasi dan relevansi abstrak. Protokol, bibliometrik dan correction/retraction yang teridentifikasi pada judul tidak dimasukkan. Seratus DOI unik diperiksa secara lokal; keunikan DOI tidak membuktikan tidak ada dataset penelitian yang tumpang tindih.

Register menyediakan 100 anotasi ringkas berbasis abstrak. Studi klinis, studi observasional, tinjauan, kerangka konsep dan artikel perspektif tidak diberi bobot yang sama. Pemanfaatan hasil yang berkaitan dengan diagnosis atau terapi dibatasi pada pelajaran evaluasi, bukan adopsi kemampuan klinis ke Sela.

## Apa yang belum selesai

Ini **bukan systematic review atau meta-analysis baru**. Penelusuran tidak exhaustive, tidak dipreregistrasi, tidak melibatkan dua penilai independen dan belum menghasilkan penilaian risiko bias formal untuk setiap naskah lengkap. Default urutan API dan pembatasan jumlah hasil menimbulkan bias kebaruan; corpus didominasi 2025. Sebagian sumber bersifat konteks tidak langsung atau melibatkan remaja/negara lain.

Abstrak tidak cukup untuk memastikan seluruh comparator, conflict of interest, adverse events, validitas alat ukur, koreksi terbaru atau relevansi praktik Indonesia. Status indeks tidak menjamin mutu jurnal. DOI tercatat telah dicocokkan pada metadata; tidak semua DOI di-resolve ulang satu per satu terhadap halaman penerbit.

> **Catatan pengganti (superseded note, 26 Oktober 2026, riset v3).** Kalimat terakhir di atas sudah tidak berlaku. Sejak 26 Oktober 2026 seluruh 100 DOI berhasil di-resolve melalui doi.org handle API, dan tanggal, lisensi, retraksi serta koreksi seluruh 100 record diverifikasi mesin terhadap Europe PMC, Crossref dan OpenAlex ([CORPUS_PIPELINE.md](../CORPUS_PIPELINE.md)). Ini verifikasi metadata oleh mesin, bukan telaah naskah: `reviewer_status` tetap `not_reviewed`. Bahasa publikasi kini dicatat (`publication_language`; 100 dari 100 berbahasa Inggris). Kalimat "Protokol ... dan correction/retraction ... tidak dimasukkan" berlaku untuk register 100; antrean judul 7.580 baru menyaringnya sejak `sela-title-screen-v1.1` (lihat catatan di [doc 14](14_METADATA_SCREENING_PROTOCOL.md)).

Metadata `firstPublicationDate` digunakan untuk seleksi. Tanggal issue dapat berbeda. Therabot memakai tanggal publikasi penerbit 27 Maret 2025; halaman penulis menampilkan Januari 2025 sehingga perbedaan ini dicatat, bukan disembunyikan.

## Jangan hitung bukti dua kali

Review dan meta-analysis dapat memasukkan trial yang sama. Seratus publikasi bukan seratus intervensi independen, seratus RCT atau seratus pembuktian Sela. Studi abstrak yang melaporkan asosiasi tidak menetapkan sebab-akibat. Penelitian English/Korean atau remaja tidak langsung memvalidasi dewasa muda Indonesia maupun model Meta.

## Langkah appraisal sebelum klaim ilmiah kuat

1. Mulai dari R023, R027, R031, R038, R047, R057, R065, R069, R087 dan R098–R100.
2. Baca full text dan supplement; ekstrak desain, n, comparator, durasi, missingness, outcomes, adverse events, konflik kepentingan dan keterbatasan.
3. Gunakan alat risiko bias yang sesuai desain; jangan memberi label “high quality” hanya karena RCT atau terbit di jurnal ternama.
4. Buat tabel overlap trial antar review sebelum menyebut total evidence.
5. Libatkan penilai bahasa Indonesia/Inggris dan profesional kesehatan mental untuk relevansi dan risiko.
6. Tulis klaim pada tingkat yang didukung: masalah akses, peluang desain atau hasil tugas usability. Validasi klinis Sela tetap membutuhkan studi tersendiri.

## Kriteria kesiapan riset produk

Setiap fitur memiliki bukti relevan, counterevidence, hipotesis manfaat, metrik terukur dan batas klaim. Fitur tidak perlu bergantung pada 100 artikel sekaligus. Lebih berguna memakai beberapa sumber yang langsung relevan dengan benar daripada menempelkan angka 100 pada landing page.

Query yang berhasil dapat direproduksi melalui [search_queries.json](search_queries.json). API sumber: https://www.ebi.ac.uk/europepmc/webservices/rest/search . Parameter tanggal dan `HAS_ABSTRACT:Y AND SRC:MED` dicatat di file tersebut. Pencarian pendahuluan kompleks sempat gagal HTTP 503 dan tidak menjadi dasar hitungan seleksi.
