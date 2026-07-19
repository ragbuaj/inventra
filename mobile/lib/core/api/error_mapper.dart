import 'dart:io';

import 'package:dio/dio.dart';

import 'app_failure.dart';

/// Memetakan [DioException] menjadi [AppFailure] (ARCHITECTURE bagian 4).
///
/// Jenis timeout/koneksi menjadi [NetworkFailure]; respons HTTP dipetakan per
/// status; sisanya [UnknownFailure] dengan penyebab asli untuk crash reporter.
AppFailure mapDioException(DioException err) {
  return switch (err.type) {
    DioExceptionType.connectionTimeout ||
    DioExceptionType.sendTimeout ||
    DioExceptionType.receiveTimeout ||
    DioExceptionType.connectionError => const NetworkFailure(),
    DioExceptionType.badResponse => _mapStatusCode(err),
    _ when err.error is SocketException => const NetworkFailure(),
    _ => UnknownFailure(err.error ?? _safeCause(err)),
  };
}

/// Ringkasan aman [DioException] untuk crash reporter — HANYA metadata request
/// (type, status, method, path). SENGAJA tidak memuat `requestOptions.data`
/// maupun `headers`: body `/auth/refresh` & `/auth/logout` membawa refresh
/// token, dan header membawa access token — keduanya tak boleh bocor ke log.
String _safeCause(DioException err) {
  final RequestOptions options = err.requestOptions;
  return 'DioException(type: ${err.type}, '
      'status: ${err.response?.statusCode}, '
      'method: ${options.method}, path: ${options.path})';
}

AppFailure _mapStatusCode(DioException err) {
  return switch (err.response?.statusCode) {
    400 || 422 => ValidationFailure(_backendErrorMessage(err) ?? ''),
    401 => const UnauthorizedFailure(),
    403 => const ForbiddenFailure(),
    404 => const NotFoundFailure(),
    409 => const ConflictFailure(),
    429 => const RateLimitedFailure(),
    final int status when status >= 500 => const ServerFailure(),
    _ => UnknownFailure(_safeCause(err)),
  };
}

/// Bentuk error backend adalah `{"error": "<pesan>"}` (openapi.yaml).
String? _backendErrorMessage(DioException err) {
  final Object? data = err.response?.data;
  if (data is Map<String, dynamic>) {
    final Object? message = data['error'];
    if (message is String) return message;
  }
  return null;
}

extension DioExceptionToAppFailure on DioException {
  /// [AppFailure] milik exception ini: hasil [ErrorMapperInterceptor] bila
  /// sudah lewat interceptor, atau dipetakan langsung bila belum.
  AppFailure toAppFailure() {
    final Object? cause = error;
    return cause is AppFailure ? cause : mapDioException(this);
  }
}

/// Interceptor kedua pada Dio tunggal: membungkus [AppFailure] hasil pemetaan
/// ke `DioException.error` supaya repository cukup memanggil [DioExceptionToAppFailure.toAppFailure].
class ErrorMapperInterceptor extends Interceptor {
  const ErrorMapperInterceptor();

  @override
  void onError(DioException err, ErrorInterceptorHandler handler) {
    if (err.error is AppFailure) {
      // Sudah dipetakan (mis. error hasil retry yang melewati chain dua kali).
      handler.next(err);
      return;
    }
    handler.next(err.copyWith(error: mapDioException(err)));
  }
}
