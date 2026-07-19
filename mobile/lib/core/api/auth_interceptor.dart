import 'package:dio/dio.dart';

import '../auth/refresh_outcome.dart';
import 'app_failure.dart';

/// Membaca access token dari memori (null bila belum login).
typedef AccessTokenReader = String? Function();

/// Melakukan refresh sesi (lihat [RefreshOutcome]).
///
/// Implementasinya (SessionManager) wajib single-flight: pemanggilan bersamaan
/// menunggu satu proses refresh yang sama.
typedef SessionRefresher = Future<RefreshOutcome> Function();

/// Dipanggil saat refresh ditolak definitif — sesi dinyatakan mati (auth
/// controller me-logout dan router mengarahkan ke login).
typedef SessionExpiredCallback = void Function();

/// Interceptor pertama pada Dio tunggal (ARCHITECTURE bagian 4):
/// menempelkan `Authorization: Bearer` dari memori + header
/// `X-Client-Type: mobile` (ADR-0017), dan menangani 401 dengan refresh
/// single-flight lalu mengulang request.
class AuthInterceptor extends Interceptor {
  AuthInterceptor({
    required this._dio,
    required this.readAccessToken,
    required this.refreshSession,
    required this.onSessionExpired,
  });

  /// Dio yang sama dengan pemilik interceptor ini — dipakai untuk mengulang
  /// request setelah refresh (header Bearer baru dipasang oleh [onRequest]).
  final Dio _dio;

  final AccessTokenReader readAccessToken;
  final SessionRefresher refreshSession;
  final SessionExpiredCallback onSessionExpired;

  /// Penanda di `RequestOptions.extra` supaya request hasil retry tidak
  /// memicu refresh kedua (mencegah loop bila 401 berulang).
  static const String retriedExtraKey = 'auth_interceptor_retried';

  /// Endpoint auth berbasis kredensial/refresh token: 401 dari sini berarti
  /// kredensial/refresh token-nya yang salah — refresh tidak akan menolong
  /// dan justru berisiko loop. `/auth/me` sengaja tidak masuk daftar.
  static const Set<String> _refreshExemptPaths = <String>{
    '/auth/login',
    '/auth/refresh',
    '/auth/logout',
  };

  @override
  void onRequest(RequestOptions options, RequestInterceptorHandler handler) {
    options.headers['X-Client-Type'] = 'mobile';
    final String? token = readAccessToken();
    if (token != null && token.isNotEmpty) {
      options.headers['Authorization'] = 'Bearer $token';
    }
    handler.next(options);
  }

  @override
  Future<void> onError(
    DioException err,
    ErrorInterceptorHandler handler,
  ) async {
    final RequestOptions options = err.requestOptions;
    if (err.response?.statusCode != 401 ||
        _isRefreshExempt(options) ||
        options.extra[retriedExtraKey] == true) {
      handler.next(err);
      return;
    }

    final RefreshOutcome outcome = await refreshSession();
    if (outcome == RefreshOutcome.rejected) {
      // Token ditolak definitif: sesi mati, error asal (401) diteruskan.
      onSessionExpired();
      handler.next(err);
      return;
    }
    if (outcome == RefreshOutcome.networkFailed) {
      // Offline/timeout saat refresh: sesi TIDAK dimatikan (401 berikutnya
      // boleh mencoba refresh lagi); request asal gagal sebagai failure
      // network, bukan unauthorized.
      handler.next(
        DioException(
          requestOptions: options,
          type: DioExceptionType.connectionError,
          error: const NetworkFailure(),
          message: 'Sesi tidak dapat diperbarui: kegagalan jaringan.',
        ),
      );
      return;
    }

    try {
      options.extra[retriedExtraKey] = true;
      final Response<dynamic> response = await _dio.fetch<dynamic>(options);
      handler.resolve(response);
    } on DioException catch (retryErr) {
      handler.next(retryErr);
    }
  }

  bool _isRefreshExempt(RequestOptions options) {
    final String path = options.uri.path;
    return _refreshExemptPaths.any(path.endsWith);
  }
}
