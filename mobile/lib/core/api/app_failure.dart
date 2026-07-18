/// Kegagalan seragam untuk seluruh lapisan data (ARCHITECTURE bagian 4).
///
/// Widget menampilkan pesan i18n per jenis — pesan mentah backend tidak pernah
/// dipakai untuk UI. `ValidationFailure.message` hanya untuk logging/diagnosis.
sealed class AppFailure implements Exception {
  const AppFailure();
}

/// Offline, timeout, atau kegagalan koneksi lain sebelum respons diterima.
final class NetworkFailure extends AppFailure {
  const NetworkFailure();
}

/// 401 — sesi tidak valid (dan refresh tidak menolong).
final class UnauthorizedFailure extends AppFailure {
  const UnauthorizedFailure();
}

/// 403 — akses ditolak (permission/SoD/audience).
final class ForbiddenFailure extends AppFailure {
  const ForbiddenFailure();
}

/// 404 — resource tidak ditemukan.
final class NotFoundFailure extends AppFailure {
  const NotFoundFailure();
}

/// 400/422 — input ditolak backend; `message` adalah pesan mentah backend
/// (bentuk `{"error": "..."}`) untuk diagnosis, bukan untuk ditampilkan.
final class ValidationFailure extends AppFailure {
  const ValidationFailure(this.message);

  final String message;
}

/// 409 — konflik state (mis. resource sudah berubah).
final class ConflictFailure extends AppFailure {
  const ConflictFailure();
}

/// 429 — rate limit backend.
final class RateLimitedFailure extends AppFailure {
  const RateLimitedFailure();
}

/// 5xx — kesalahan server.
final class ServerFailure extends AppFailure {
  const ServerFailure();
}

/// Kegagalan yang tidak terpetakan; `cause` disimpan untuk crash reporter.
final class UnknownFailure extends AppFailure {
  const UnknownFailure([this.cause]);

  final Object? cause;
}
