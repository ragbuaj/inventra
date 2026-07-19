import 'package:dio/dio.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../auth/session_manager.dart';
import 'auth_interceptor.dart';
import 'error_mapper.dart';

/// Base URL backend via `--dart-define=API_BASE_URL=...`; default dev
/// menunjuk backend lokal (paritas compose dev).
const String apiBaseUrl = String.fromEnvironment(
  'API_BASE_URL',
  defaultValue: 'http://localhost:8080',
);

/// Dipisah sebagai provider supaya test/flavor bisa meng-override tanpa
/// menyentuh [dioProvider].
final Provider<String> apiBaseUrlProvider = Provider<String>(
  (Ref ref) => apiBaseUrl,
);

/// Dio tunggal aplikasi (ARCHITECTURE bagian 4). Interceptor berurutan:
/// (1) auth — Bearer dari memori, `X-Client-Type: mobile`, 401 refresh
/// single-flight; (2) error mapper — DioException menjadi `AppFailure`.
/// Tanpa cookie jar: klien mobile tidak memakai cookie sama sekali (ADR-0017).
final Provider<Dio> dioProvider = Provider<Dio>((Ref ref) {
  final Dio dio = Dio(
    BaseOptions(
      baseUrl: '${ref.watch(apiBaseUrlProvider)}/api/v1',
      connectTimeout: const Duration(seconds: 10),
      sendTimeout: const Duration(seconds: 10),
      receiveTimeout: const Duration(seconds: 20),
    ),
  );
  dio.interceptors.addAll(<Interceptor>[
    AuthInterceptor(
      dio: dio,
      // ref.read lazy di dalam closure: SessionManager baru disentuh saat
      // request berjalan, bukan saat dioProvider dibangun (hindari siklus).
      readAccessToken: () => ref.read(sessionManagerProvider).accessToken,
      refreshSession: () => ref.read(sessionManagerProvider).refresh(),
      onSessionExpired: () =>
          ref.read(sessionManagerProvider).notifySessionExpired(),
    ),
    const ErrorMapperInterceptor(),
  ]);
  return dio;
});
