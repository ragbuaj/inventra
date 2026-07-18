/// Hasil percobaan refresh sesi — dibedakan supaya kegagalan jaringan tidak
/// diperlakukan seperti penolakan token (aplikasi lapangan sering offline).
enum RefreshOutcome {
  /// Sesi hidup kembali: access token baru di memori, refresh token rotasi
  /// tersimpan.
  success,

  /// Penolakan definitif (token tidak ada, atau ditolak otentik oleh server
  /// lewat 401): refresh token dihapus dan sesi dinyatakan mati.
  rejected,

  /// Gagal sementara (jaringan/timeout, server 5xx, rate limit, atau
  /// kegagalan tak terpetakan lain): refresh token DIPERTAHANKAN; percobaan
  /// berikutnya (401 berikutnya atau launch berikutnya) boleh mencoba lagi —
  /// keraguan tidak boleh menghancurkan sesi.
  networkFailed,
}
