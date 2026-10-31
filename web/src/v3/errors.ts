import type { Lang } from "./api";

/*
 * Every API error is shown from its stable code, never from a server string
 * that could leak implementation detail. The server's message is the fallback
 * when a code is newer than this client.
 */
const copy: Record<string, { id: string; en: string }> = {
  invalid_input: {
    id: "Ada isian yang belum bisa dipakai. Periksa lagi bagian yang ditandai.",
    en: "Something in this form can't be used yet. Check the part that is marked.",
  },
  not_found: {
    id: "Halaman atau isi ini tidak ditemukan.",
    en: "This page or item wasn't found.",
  },
  method_not_allowed: {
    id: "Tindakan ini tidak tersedia di sini.",
    en: "That action isn't available here.",
  },
  stale_version: {
    id: "Isi ini berubah di tab lain. Muat ulang untuk melihat versi terbaru.",
    en: "This changed in another tab. Reload to see the latest version.",
  },
  invalid_transition: {
    id: "Langkah ini belum bisa dilakukan sekarang.",
    en: "This step can't be done right now.",
  },
  policy_outdated: {
    id: "Kebijakan sudah diperbarui. Baca versi terbaru lalu pilih lagi.",
    en: "The policy has changed. Read the latest version, then choose again.",
  },
  too_large: {
    id: "Tulisannya terlalu panjang untuk dikirim sekaligus.",
    en: "This is too long to send at once.",
  },
  unsupported_media: {
    id: "Permintaan ini tidak bisa diproses.",
    en: "This request couldn't be processed.",
  },
  cross_origin: {
    id: "Permintaan ini ditolak untuk keamanan. Muat ulang halaman lalu coba lagi.",
    en: "This request was refused for security. Reload the page and try again.",
  },
  rate_limited: {
    id: "Terlalu banyak percobaan. Coba lagi sebentar lagi.",
    en: "Too many attempts. Try again shortly.",
  },
  internal: {
    id: "Ada yang tidak berjalan seperti seharusnya. Coba lagi.",
    en: "Something went wrong. Try again.",
  },
  timeout: {
    id: "Koneksi terlalu lama menjawab. Tulisanmu tetap di sini; coba lagi.",
    en: "The connection took too long. Your writing is still here; try again.",
  },
  invalid_share: {
    id: "Tautan atau kode tidak cocok, atau sudah tidak berlaku.",
    en: "The link or code doesn't match, or it is no longer valid.",
  },
  name_taken: {
    id: "Nama ini sudah ada di daftarmu.",
    en: "This name is already on your list.",
  },
  name_limit_reached: {
    id: "Daftar nama berisi paling banyak 20 nama. Hapus satu dulu untuk menambah yang lain.",
    en: "The name list holds at most 20 names. Remove one before adding another.",
  },
  consent_required: {
    id: "Izin yang diperlukan belum diberikan.",
    en: "The required permission hasn't been given.",
  },
  not_signed_in: {
    id: "Kamu belum masuk atau sesimu sudah berakhir. Silakan masuk lagi.",
    en: "You aren't signed in, or your session has ended. Please sign in again.",
  },
  invalid_credentials: {
    id: "Nama pengguna atau kata sandi tidak cocok.",
    en: "The username or password does not match.",
  },
  recovery_failed: {
    id: "Nama pengguna atau kode pemulihan tidak cocok, atau kode itu sudah pernah dipakai.",
    en: "The username or recovery code does not match, or that code was already used.",
  },
  current_password_incorrect: {
    id: "Kata sandi saat ini tidak cocok.",
    en: "The current password does not match.",
  },
  reauth_required: {
    id: "Untuk melindungi datamu, masukkan kata sandimu sekali lagi.",
    en: "To protect your data, enter your password once more.",
  },
  account_required: {
    id: "Langkah ini memerlukan akun. Simpan data tamumu ke akun terlebih dahulu.",
    en: "This step needs an account. Save your guest data to an account first.",
  },
  password_login_disabled: {
    id: "Masuk dengan kata sandi dinonaktifkan sementara karena terlalu banyak percobaan. Pakai kode pemulihan untuk membuat kata sandi baru.",
    en: "Password sign-in is disabled after too many attempts. Use a recovery code to set a new password.",
  },
  age_not_supported: {
    id: "Sela saat ini dibuat untuk usia 16 tahun ke atas.",
    en: "Sela is currently made for people aged 16 and over.",
  },
  age_band_not_enabled: {
    id: "Sela belum tersedia untuk kelompok usia ini.",
    en: "Sela isn't available for this age group yet.",
  },
  not_guest: {
    id: "Akun ini sudah terdaftar.",
    en: "This account is already registered.",
  },
  required_consent: {
    id: "Persetujuan ini diperlukan untuk memakai Sela. Kalau tidak lagi setuju, kamu bisa mengekspor lalu menghapus akunmu.",
    en: "Sela can't be used without this consent. If you no longer agree, you can export and then delete your account.",
  },
  username_invalid: {
    id: "Nama pengguna harus 3–32 karakter: huruf a–z, angka, titik, garis bawah, atau tanda hubung. Jangan memakai nama asli, email, atau nomor telepon.",
    en: "Usernames are 3–32 characters: letters a–z, digits, dot, underscore or hyphen. Don't use your real name, email or phone number.",
  },
  username_taken: {
    id: "Nama pengguna ini sudah dipakai. Pilih yang lain.",
    en: "This username is taken. Choose another.",
  },
  password_too_short: {
    id: "Kata sandi minimal 15 karakter. Coba gabungan beberapa kata yang mudah kamu ingat.",
    en: "Passwords need at least 15 characters. Try a few words you can remember.",
  },
  password_too_long: {
    id: "Kata sandi maksimal 256 karakter.",
    en: "Passwords can have at most 256 characters.",
  },
  password_weak: {
    id: "Kata sandi ini terlalu umum atau mudah ditebak. Pilih yang lain.",
    en: "This password is too common or easy to guess. Choose another.",
  },
  store_unavailable: {
    id: "Penyimpanan sedang tidak tersedia. Coba lagi sebentar lagi. Bantuan sekarang tetap bisa dibuka.",
    en: "Storage is unavailable right now. Try again shortly. Help now still works.",
  },
  server_busy: {
    id: "Server sedang sibuk. Coba lagi sebentar lagi.",
    en: "The server is busy. Try again shortly.",
  },
  storage_required: {
    id: "Pilih dulu apakah tulisan ini disimpan.",
    en: "Choose first whether this is saved.",
  },
  capacity: {
    id: "Mode tamu sedang penuh. Coba lagi sebentar lagi, atau buat akun.",
    en: "Guest mode is full right now. Try again shortly, or create an account.",
  },
  limit_reached: {
    id: "Percakapan ini sudah mencapai batas panjang. Akhiri dan mulai percakapan baru.",
    en: "This conversation has reached its length limit. End it and start a new one.",
  },
  unsupported: {
    id: "Fitur ini belum tersedia di versi Sela ini.",
    en: "This feature isn't available in this build of Sela yet.",
  },
};

export function errorCopy(code: string, lang: Lang, serverMessage?: string): string {
  const entry = copy[code];
  if (entry) return entry[lang];
  return serverMessage ?? code;
}

export function isBlocking(code: string): boolean {
  return [
    "store_unavailable",
    "server_busy",
    "not_signed_in",
    "rate_limited",
    "timeout",
  ].includes(code);
}
