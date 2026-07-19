import 'dart:io';

import 'package:dio/dio.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:inventra_mobile/core/api/app_failure.dart';
import 'package:inventra_mobile/core/api/error_mapper.dart';

import '../../helpers/fakes.dart';

void main() {
  final RequestOptions options = RequestOptions(path: '/anything');

  DioException badResponse(int statusCode, [Object? body]) {
    return DioException(
      requestOptions: options,
      type: DioExceptionType.badResponse,
      response: Response<dynamic>(
        requestOptions: options,
        statusCode: statusCode,
        data: body,
      ),
    );
  }

  group('mapDioException - jenis koneksi', () {
    for (final DioExceptionType type in <DioExceptionType>[
      DioExceptionType.connectionTimeout,
      DioExceptionType.sendTimeout,
      DioExceptionType.receiveTimeout,
      DioExceptionType.connectionError,
    ]) {
      test('$type menjadi NetworkFailure', () {
        final DioException err = DioException(
          requestOptions: options,
          type: type,
        );
        expect(mapDioException(err), isA<NetworkFailure>());
      });
    }

    test('unknown dengan SocketException menjadi NetworkFailure', () {
      final DioException err = DioException(
        requestOptions: options,
        type: DioExceptionType.unknown,
        error: const SocketException('connection refused'),
      );
      expect(mapDioException(err), isA<NetworkFailure>());
    });

    test('cancel menjadi UnknownFailure', () {
      final DioException err = DioException(
        requestOptions: options,
        type: DioExceptionType.cancel,
      );
      expect(mapDioException(err), isA<UnknownFailure>());
    });
  });

  group('mapDioException - status HTTP', () {
    test('400 menjadi ValidationFailure dengan pesan backend', () {
      final AppFailure failure = mapDioException(
        badResponse(400, <String, dynamic>{'error': 'email is required'}),
      );
      expect(failure, isA<ValidationFailure>());
      expect((failure as ValidationFailure).message, 'email is required');
    });

    test('422 menjadi ValidationFailure', () {
      expect(mapDioException(badResponse(422)), isA<ValidationFailure>());
    });

    test(
      '400 tanpa body error tetap ValidationFailure dengan pesan kosong',
      () {
        final AppFailure failure = mapDioException(badResponse(400, 'oops'));
        expect((failure as ValidationFailure).message, isEmpty);
      },
    );

    test('401 menjadi UnauthorizedFailure', () {
      expect(mapDioException(badResponse(401)), isA<UnauthorizedFailure>());
    });

    test('403 menjadi ForbiddenFailure', () {
      expect(mapDioException(badResponse(403)), isA<ForbiddenFailure>());
    });

    test('404 menjadi NotFoundFailure', () {
      expect(mapDioException(badResponse(404)), isA<NotFoundFailure>());
    });

    test('409 menjadi ConflictFailure', () {
      expect(mapDioException(badResponse(409)), isA<ConflictFailure>());
    });

    test('429 menjadi RateLimitedFailure', () {
      expect(mapDioException(badResponse(429)), isA<RateLimitedFailure>());
    });

    test('500 dan 503 menjadi ServerFailure', () {
      expect(mapDioException(badResponse(500)), isA<ServerFailure>());
      expect(mapDioException(badResponse(503)), isA<ServerFailure>());
    });

    test('status tak terpetakan menjadi UnknownFailure', () {
      expect(mapDioException(badResponse(418)), isA<UnknownFailure>());
    });

    test('cause UnknownFailure tidak membawa refresh token / body request', () {
      // DioException /auth/refresh dengan refresh token di body — persis yang
      // dulu bocor karena menyimpan DioException mentah sebagai cause.
      final RequestOptions authOptions = RequestOptions(
        path: '/auth/refresh',
        method: 'POST',
        data: <String, dynamic>{'refresh_token': 'super-secret-refresh-token'},
        headers: <String, dynamic>{
          'Authorization': 'Bearer super-secret-access-token',
        },
      );
      final DioException err = DioException(
        requestOptions: authOptions,
        type: DioExceptionType.badResponse,
        response: Response<dynamic>(
          requestOptions: authOptions,
          statusCode: 418,
        ),
      );

      final AppFailure failure = mapDioException(err);
      expect(failure, isA<UnknownFailure>());
      final String cause = '${(failure as UnknownFailure).cause}';
      expect(cause, isNot(contains('super-secret-refresh-token')));
      expect(cause, isNot(contains('super-secret-access-token')));
      // Metadata aman tetap tersedia untuk diagnosis.
      expect(cause, contains('/auth/refresh'));
      expect(cause, contains('418'));
    });

    test('cause UnknownFailure jenis non-response juga aman', () {
      final RequestOptions authOptions = RequestOptions(
        path: '/auth/logout',
        method: 'POST',
        data: <String, dynamic>{'refresh_token': 'secret-logout-token'},
      );
      // Tipe cancel tanpa err.error -> fallback ke ringkasan aman (bukan
      // DioException mentah yang membawa body).
      final DioException err = DioException(
        requestOptions: authOptions,
        type: DioExceptionType.cancel,
      );

      final AppFailure failure = mapDioException(err);
      final String cause = '${(failure as UnknownFailure).cause}';
      expect(cause, isNot(contains('secret-logout-token')));
      expect(cause, contains('/auth/logout'));
    });
  });

  group('toAppFailure', () {
    test('memakai AppFailure hasil interceptor bila sudah ada', () {
      final DioException err = DioException(
        requestOptions: options,
        error: const ConflictFailure(),
      );
      expect(err.toAppFailure(), isA<ConflictFailure>());
    });

    test('memetakan sendiri bila belum lewat interceptor', () {
      expect(badResponse(404).toAppFailure(), isA<NotFoundFailure>());
    });
  });

  group('ErrorMapperInterceptor lewat Dio', () {
    test('membungkus AppFailure ke DioException.error', () async {
      final Dio dio = Dio(BaseOptions(baseUrl: 'http://api.test/api/v1'))
        ..interceptors.add(const ErrorMapperInterceptor());
      dio.httpClientAdapter = RoutingHttpClientAdapter(
        (RequestOptions options) async =>
            jsonResponseBody(429, <String, dynamic>{'error': 'slow down'}),
      );

      try {
        await dio.get<dynamic>('/assets');
        fail('harus melempar DioException');
      } on DioException catch (err) {
        expect(err.error, isA<RateLimitedFailure>());
      }
    });
  });
}
