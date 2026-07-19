import 'package:dio/dio.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:inventra_mobile/core/api/app_failure.dart';
import 'package:inventra_mobile/core/auth/data/auth_repository.dart';
import 'package:inventra_mobile/core/auth/data/token_response_dto.dart';
import 'package:inventra_mobile/core/auth/data/user_dto.dart';
import 'package:mocktail/mocktail.dart';

class _MockDio extends Mock implements Dio {}

const Map<String, dynamic> _tokenJson = <String, dynamic>{
  'access_token': 'access-1',
  'token_type': 'Bearer',
  'expires_in': 900,
  'refresh_token': 'rt-1',
};

Response<Map<String, dynamic>> _jsonResponse(
  String path,
  Map<String, dynamic> data,
) {
  return Response<Map<String, dynamic>>(
    requestOptions: RequestOptions(path: path),
    statusCode: 200,
    data: data,
  );
}

DioException _mappedError(String path, AppFailure failure) {
  return DioException(
    requestOptions: RequestOptions(path: path),
    error: failure,
  );
}

void main() {
  late _MockDio dio;
  late AuthRepository repository;

  setUp(() {
    dio = _MockDio();
    repository = AuthRepository(dio);
  });

  group('login', () {
    test(
      'POST /auth/login dengan body email+password, parse TokenResponse',
      () async {
        when(
          () => dio.post<Map<String, dynamic>>(
            '/auth/login',
            data: any(named: 'data'),
          ),
        ).thenAnswer((_) async => _jsonResponse('/auth/login', _tokenJson));

        final TokenResponseDto tokens = await repository.login(
          email: 'admin@inventra.local',
          password: 'admin12345',
        );

        expect(tokens.accessToken, 'access-1');
        expect(tokens.refreshToken, 'rt-1');
        final Map<String, dynamic> body =
            verify(
                  () => dio.post<Map<String, dynamic>>(
                    '/auth/login',
                    data: captureAny(named: 'data'),
                  ),
                ).captured.single
                as Map<String, dynamic>;
        expect(body, <String, String>{
          'email': 'admin@inventra.local',
          'password': 'admin12345',
        });
      },
    );

    test('kredensial salah: melempar UnauthorizedFailure', () async {
      when(
        () => dio.post<Map<String, dynamic>>(
          '/auth/login',
          data: any(named: 'data'),
        ),
      ).thenThrow(_mappedError('/auth/login', const UnauthorizedFailure()));

      expect(
        () => repository.login(email: 'a@b.c', password: 'salah'),
        throwsA(isA<UnauthorizedFailure>()),
      );
    });

    test(
      'DioException yang belum terpetakan tetap menjadi AppFailure',
      () async {
        final RequestOptions options = RequestOptions(path: '/auth/login');
        when(
          () => dio.post<Map<String, dynamic>>(
            '/auth/login',
            data: any(named: 'data'),
          ),
        ).thenThrow(
          DioException(
            requestOptions: options,
            type: DioExceptionType.badResponse,
            response: Response<dynamic>(
              requestOptions: options,
              statusCode: 403,
            ),
          ),
        );

        expect(
          () => repository.login(email: 'a@b.c', password: 'x'),
          throwsA(isA<ForbiddenFailure>()),
        );
      },
    );
  });

  group('refresh', () {
    test('POST /auth/refresh dengan body refresh_token', () async {
      when(
        () => dio.post<Map<String, dynamic>>(
          '/auth/refresh',
          data: any(named: 'data'),
        ),
      ).thenAnswer((_) async => _jsonResponse('/auth/refresh', _tokenJson));

      final TokenResponseDto tokens = await repository.refresh(
        refreshToken: 'rt-lama',
      );

      expect(tokens.accessToken, 'access-1');
      final Map<String, dynamic> body =
          verify(
                () => dio.post<Map<String, dynamic>>(
                  '/auth/refresh',
                  data: captureAny(named: 'data'),
                ),
              ).captured.single
              as Map<String, dynamic>;
      expect(body, <String, String>{'refresh_token': 'rt-lama'});
    });

    test('refresh token ditolak: melempar UnauthorizedFailure', () async {
      when(
        () => dio.post<Map<String, dynamic>>(
          '/auth/refresh',
          data: any(named: 'data'),
        ),
      ).thenThrow(_mappedError('/auth/refresh', const UnauthorizedFailure()));

      expect(
        () => repository.refresh(refreshToken: 'rt-dicabut'),
        throwsA(isA<UnauthorizedFailure>()),
      );
    });
  });

  group('logout', () {
    test('POST /auth/logout dengan body refresh_token', () async {
      when(
        () => dio.post<Map<String, dynamic>>(
          '/auth/logout',
          data: any(named: 'data'),
        ),
      ).thenAnswer(
        (_) async => _jsonResponse('/auth/logout', <String, dynamic>{}),
      );

      await repository.logout(refreshToken: 'rt-1');

      final Map<String, dynamic> body =
          verify(
                () => dio.post<Map<String, dynamic>>(
                  '/auth/logout',
                  data: captureAny(named: 'data'),
                ),
              ).captured.single
              as Map<String, dynamic>;
      expect(body, <String, String>{'refresh_token': 'rt-1'});
    });

    test('offline: melempar NetworkFailure', () async {
      when(
        () => dio.post<Map<String, dynamic>>(
          '/auth/logout',
          data: any(named: 'data'),
        ),
      ).thenThrow(_mappedError('/auth/logout', const NetworkFailure()));

      expect(
        () => repository.logout(refreshToken: 'rt-1'),
        throwsA(isA<NetworkFailure>()),
      );
    });
  });

  group('me', () {
    test('GET /auth/me parse UserDto', () async {
      when(() => dio.get<Map<String, dynamic>>('/auth/me')).thenAnswer(
        (_) async => _jsonResponse('/auth/me', <String, dynamic>{
          'id': 'user-1',
          'name': 'Ragil',
          'email': 'ragil@inventra.local',
          'role_id': 'role-1',
          'office_id': null,
          'employee_id': null,
          'status': 'active',
          'has_avatar': true,
          'google_linked': false,
        }),
      );

      final UserDto user = await repository.me();

      expect(user.id, 'user-1');
      expect(user.roleId, 'role-1');
      expect(user.hasAvatar, isTrue);
      expect(user.officeId, isNull);
    });

    test('sesi tidak valid: melempar UnauthorizedFailure', () async {
      when(
        () => dio.get<Map<String, dynamic>>('/auth/me'),
      ).thenThrow(_mappedError('/auth/me', const UnauthorizedFailure()));

      expect(() => repository.me(), throwsA(isA<UnauthorizedFailure>()));
    });
  });
}
