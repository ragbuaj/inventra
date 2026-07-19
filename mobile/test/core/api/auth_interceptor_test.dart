import 'package:dio/dio.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:inventra_mobile/core/api/app_failure.dart';
import 'package:inventra_mobile/core/api/auth_interceptor.dart';
import 'package:inventra_mobile/core/api/error_mapper.dart';
import 'package:inventra_mobile/core/auth/session_manager.dart';

import '../../helpers/fakes.dart';

/// Harness: Dio nyata + adapter palsu + SessionManager nyata (single-flight
/// asli) dengan storage in-memory dan executor refresh yang bisa diskrip.
class _Harness {
  _Harness({
    String? storedRefreshToken,
    String? accessToken,
    Future<SessionTokens> Function(String refreshToken)? refreshExecutor,
  }) {
    storage = InMemoryTokenStorage(storedRefreshToken);
    session = SessionManager(
      tokenStorage: storage,
      refreshExecutor: (String refreshToken) {
        refreshCalls += 1;
        return (refreshExecutor ?? _defaultExecutor)(refreshToken);
      },
    );
    session.accessToken = accessToken;
    dio = Dio(BaseOptions(baseUrl: 'http://api.test/api/v1'));
    dio.interceptors.addAll(<Interceptor>[
      AuthInterceptor(
        dio: dio,
        readAccessToken: () => session.accessToken,
        refreshSession: session.refresh,
        onSessionExpired: () => sessionExpiredCalls += 1,
      ),
      const ErrorMapperInterceptor(),
    ]);
  }

  late final InMemoryTokenStorage storage;
  late final SessionManager session;
  late final Dio dio;
  int refreshCalls = 0;
  int sessionExpiredCalls = 0;

  static Future<SessionTokens> _defaultExecutor(String refreshToken) async {
    // Delay kecil supaya request bersamaan benar-benar tumpang tindih dengan
    // refresh yang sedang berjalan (menguji single-flight).
    await Future<void>.delayed(const Duration(milliseconds: 20));
    return (accessToken: 'new-access', refreshToken: 'rt-rotated');
  }
}

void main() {
  group('onRequest', () {
    test('menempelkan X-Client-Type: mobile dan Bearer dari memori', () async {
      final _Harness h = _Harness(accessToken: 'access-1');
      final RoutingHttpClientAdapter adapter = RoutingHttpClientAdapter(
        (RequestOptions options) async =>
            jsonResponseBody(200, <String, dynamic>{'ok': true}),
      );
      h.dio.httpClientAdapter = adapter;

      await h.dio.get<dynamic>('/assets');

      final RequestOptions sent = adapter.requests.single;
      expect(sent.headers['X-Client-Type'], 'mobile');
      expect(sent.headers['Authorization'], 'Bearer access-1');
    });

    test(
      'tanpa access token: X-Client-Type tetap, Authorization absen',
      () async {
        final _Harness h = _Harness();
        final RoutingHttpClientAdapter adapter = RoutingHttpClientAdapter(
          (RequestOptions options) async =>
              jsonResponseBody(200, <String, dynamic>{'ok': true}),
        );
        h.dio.httpClientAdapter = adapter;

        await h.dio.get<dynamic>('/assets');

        final RequestOptions sent = adapter.requests.single;
        expect(sent.headers['X-Client-Type'], 'mobile');
        expect(sent.headers.containsKey('Authorization'), isFalse);
      },
    );
  });

  group('401 dan refresh single-flight', () {
    ResponseBody protectedRoute(RequestOptions options) {
      final Object? auth = options.headers['Authorization'];
      if (auth == 'Bearer new-access') {
        return jsonResponseBody(200, <String, dynamic>{'ok': true});
      }
      return jsonResponseBody(401, <String, dynamic>{'error': 'expired'});
    }

    test(
      '401 memicu satu refresh lalu mengulang request sampai sukses',
      () async {
        final _Harness h = _Harness(
          storedRefreshToken: 'rt-1',
          accessToken: 'stale-access',
        );
        final RoutingHttpClientAdapter adapter = RoutingHttpClientAdapter(
          (RequestOptions options) async => protectedRoute(options),
        );
        h.dio.httpClientAdapter = adapter;

        final Response<dynamic> response = await h.dio.get<dynamic>('/assets');

        expect(response.statusCode, 200);
        expect(h.refreshCalls, 1);
        expect(h.sessionExpiredCalls, 0);
        // Request kedua (retry) memakai Bearer hasil refresh.
        expect(adapter.requests, hasLength(2));
        expect(
          adapter.requests.last.headers['Authorization'],
          'Bearer new-access',
        );
        // Rotasi: refresh token baru tersimpan menggantikan yang lama.
        expect(h.storage.refreshToken, 'rt-rotated');
        expect(h.session.accessToken, 'new-access');
      },
    );

    test(
      'N request bersamaan yang 401 menunggu SATU refresh yang sama',
      () async {
        final _Harness h = _Harness(
          storedRefreshToken: 'rt-1',
          accessToken: 'stale-access',
        );
        final RoutingHttpClientAdapter adapter = RoutingHttpClientAdapter(
          (RequestOptions options) async => protectedRoute(options),
        );
        h.dio.httpClientAdapter = adapter;

        final List<Response<dynamic>> responses =
            await Future.wait(<Future<Response<dynamic>>>[
              h.dio.get<dynamic>('/assets'),
              h.dio.get<dynamic>('/requests'),
              h.dio.get<dynamic>('/notifications'),
            ]);

        expect(
          responses.map((Response<dynamic> r) => r.statusCode),
          everyElement(200),
        );
        expect(h.refreshCalls, 1);
        expect(h.sessionExpiredCalls, 0);
      },
    );

    test(
      'refresh ditolak definitif: sesi-mati, token terhapus, unauthorized',
      () async {
        final _Harness h = _Harness(
          storedRefreshToken: 'rt-1',
          accessToken: 'stale-access',
          refreshExecutor: (String refreshToken) async =>
              throw const UnauthorizedFailure(),
        );
        final RoutingHttpClientAdapter adapter = RoutingHttpClientAdapter(
          (RequestOptions options) async => protectedRoute(options),
        );
        h.dio.httpClientAdapter = adapter;

        try {
          await h.dio.get<dynamic>('/assets');
          fail('harus melempar DioException');
        } on DioException catch (err) {
          expect(err.toAppFailure(), isA<UnauthorizedFailure>());
        }
        expect(h.refreshCalls, 1);
        expect(h.sessionExpiredCalls, 1);
        // Penolakan definitif: refresh token ikut terhapus.
        expect(h.storage.refreshToken, isNull);
        // Tidak ada retry: request hanya terkirim sekali.
        expect(adapter.requests, hasLength(1));
      },
    );

    // Konservatif: kegagalan refresh selain penolakan 401 (jaringan, 500,
    // 429) bersifat sementara — sesi tidak mati, token dipertahankan.
    for (final AppFailure transientFailure in <AppFailure>[
      const NetworkFailure(),
      const ServerFailure(),
      const RateLimitedFailure(),
    ]) {
      test('refresh gagal sementara (${transientFailure.runtimeType}): sesi '
          'TIDAK mati, token dipertahankan, boleh dicoba lagi di 401 '
          'berikutnya', () async {
        final _Harness h = _Harness(
          storedRefreshToken: 'rt-1',
          accessToken: 'stale-access',
          refreshExecutor: (String refreshToken) async =>
              throw transientFailure,
        );
        final RoutingHttpClientAdapter adapter = RoutingHttpClientAdapter(
          (RequestOptions options) async => protectedRoute(options),
        );
        h.dio.httpClientAdapter = adapter;

        // Request pertama: gagal sebagai failure network (retryable),
        // bukan unauthorized.
        try {
          await h.dio.get<dynamic>('/assets');
          fail('harus melempar DioException');
        } on DioException catch (err) {
          expect(err.toAppFailure(), isA<NetworkFailure>());
        }
        expect(h.refreshCalls, 1);
        expect(h.sessionExpiredCalls, 0);
        expect(h.storage.refreshToken, 'rt-1');

        // 401 berikutnya: refresh dicoba lagi (tidak diblokir sesi-mati).
        await expectLater(
          h.dio.get<dynamic>('/assets'),
          throwsA(isA<DioException>()),
        );
        expect(h.refreshCalls, 2);
        expect(h.sessionExpiredCalls, 0);
        expect(h.storage.refreshToken, 'rt-1');
      });
    }

    test(
      'tanpa refresh token tersimpan: sesi-mati tanpa memanggil executor',
      () async {
        final _Harness h = _Harness(accessToken: 'stale-access');
        final RoutingHttpClientAdapter adapter = RoutingHttpClientAdapter(
          (RequestOptions options) async => protectedRoute(options),
        );
        h.dio.httpClientAdapter = adapter;

        await expectLater(
          h.dio.get<dynamic>('/assets'),
          throwsA(isA<DioException>()),
        );
        expect(h.refreshCalls, 0);
        expect(h.sessionExpiredCalls, 1);
      },
    );

    test(
      '401 berulang setelah retry tidak memicu refresh kedua (tanpa loop)',
      () async {
        final _Harness h = _Harness(
          storedRefreshToken: 'rt-1',
          accessToken: 'stale-access',
        );
        final RoutingHttpClientAdapter adapter = RoutingHttpClientAdapter(
          (RequestOptions options) async =>
              jsonResponseBody(401, <String, dynamic>{'error': 'expired'}),
        );
        h.dio.httpClientAdapter = adapter;

        await expectLater(
          h.dio.get<dynamic>('/assets'),
          throwsA(isA<DioException>()),
        );
        expect(h.refreshCalls, 1);
        // Satu request awal + satu retry, lalu berhenti.
        expect(adapter.requests, hasLength(2));
      },
    );
  });

  group('endpoint auth tidak memicu refresh', () {
    for (final String path in <String>[
      '/auth/login',
      '/auth/refresh',
      '/auth/logout',
    ]) {
      test('401 dari $path diteruskan tanpa refresh', () async {
        final _Harness h = _Harness(
          storedRefreshToken: 'rt-1',
          accessToken: 'stale-access',
        );
        final RoutingHttpClientAdapter adapter = RoutingHttpClientAdapter(
          (RequestOptions options) async => jsonResponseBody(
            401,
            <String, dynamic>{'error': 'invalid credentials'},
          ),
        );
        h.dio.httpClientAdapter = adapter;

        await expectLater(
          h.dio.post<dynamic>(path),
          throwsA(isA<DioException>()),
        );
        expect(h.refreshCalls, 0);
        expect(h.sessionExpiredCalls, 0);
        expect(adapter.requests, hasLength(1));
      });
    }
  });
}
