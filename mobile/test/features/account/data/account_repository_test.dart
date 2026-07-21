import 'dart:typed_data';

import 'package:dio/dio.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:inventra_mobile/core/api/app_failure.dart';
import 'package:inventra_mobile/features/account/data/account_repository.dart';
import 'package:inventra_mobile/features/account/data/session_dto.dart';
import 'package:mocktail/mocktail.dart';

class _MockDio extends Mock implements Dio {}

/// JSON `SessionView` kontrak lengkap (sesi ini).
const Map<String, dynamic> currentSessionJson = <String, dynamic>{
  'id': 'sess-1',
  'browser': 'Inventra App',
  'os': 'Android',
  'device_type': 'mobile',
  'ip_address': '103.28.11.4',
  'location': 'Jakarta, Indonesia',
  'created_at': '2026-07-01T08:00:00Z',
  'last_seen_at': '2026-07-19T02:41:00Z',
  'current': true,
};

/// Sesi lain (web desktop) dengan lokasi GeoIP kosong (kontrak membolehkan).
const Map<String, dynamic> otherSessionJson = <String, dynamic>{
  'id': 'sess-2',
  'browser': 'Chrome',
  'os': 'Windows',
  'device_type': 'desktop',
  'ip_address': '103.28.11.8',
  'location': '',
  'created_at': '2026-07-10T01:00:00Z',
  'last_seen_at': '2026-07-19T00:41:00Z',
  'current': false,
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

DioException _statusError(String path, int statusCode) {
  final RequestOptions options = RequestOptions(path: path);
  return DioException(
    requestOptions: options,
    type: DioExceptionType.badResponse,
    response: Response<dynamic>(
      requestOptions: options,
      statusCode: statusCode,
      data: <String, dynamic>{'error': 'server message'},
    ),
  );
}

DioException _connectionError(String path) {
  return DioException(
    requestOptions: RequestOptions(path: path),
    type: DioExceptionType.connectionError,
  );
}

void main() {
  late _MockDio dio;
  late AccountRepository repository;

  setUpAll(() {
    registerFallbackValue(Options());
  });

  setUp(() {
    dio = _MockDio();
    repository = AccountRepository(dio);
  });

  group('getProfile', () {
    test('GET /auth/profile: ProfileView terparse', () async {
      when(() => dio.get<Map<String, dynamic>>('/auth/profile')).thenAnswer(
        (_) async => _jsonResponse('/auth/profile', <String, dynamic>{
          'id': 'u1',
          'name': 'Andi Saputra',
          'email': 'andi@x.local',
          'phone': '0812',
          'role_name': 'Asset Manager',
          'office_name': 'Cabang Jakarta Selatan',
          'employee_code': 'EMP-1',
          'employee_status': 'Aktif',
          'department_name': 'GA',
          'position_name': 'Staf Aset',
          'has_avatar': true,
          'google_linked': false,
          'joined_at': '2026-01-15T00:00:00Z',
        }),
      );

      final profile = await repository.getProfile();
      expect(profile.name, 'Andi Saputra');
      expect(profile.employeeCode, 'EMP-1');
      expect(profile.departmentName, 'GA');
      expect(profile.hasEmployee, isTrue);
      expect(profile.hasAvatar, isTrue);
      expect(profile.googleLinked, isFalse);
    });

    test('offline: NetworkFailure', () async {
      when(() => dio.get<Map<String, dynamic>>('/auth/profile')).thenThrow(
        DioException(
          requestOptions: RequestOptions(path: '/auth/profile'),
          type: DioExceptionType.connectionError,
        ),
      );
      expect(() => repository.getProfile(), throwsA(isA<NetworkFailure>()));
    });
  });

  group('avatar', () {
    test('uploadAvatar: multipart field file', () async {
      when(
        () => dio.post<Map<String, dynamic>>(
          '/auth/avatar',
          data: any(named: 'data'),
        ),
      ).thenAnswer(
        (_) async => _jsonResponse('/auth/avatar', <String, dynamic>{
          'has_avatar': true,
        }),
      );

      await repository.uploadAvatar(<int>[1, 2, 3], filename: 'foto.jpg');

      final Object? data =
          verify(
            () => dio.post<Map<String, dynamic>>(
              '/auth/avatar',
              data: captureAny(named: 'data'),
            ),
          ).captured.single;
      expect(data, isA<FormData>());
      expect((data as FormData).files.single.key, 'file');
      expect(data.files.single.value.filename, 'foto.jpg');
    });

    test('deleteAvatar: DELETE /auth/avatar', () async {
      when(
        () => dio.delete<Map<String, dynamic>>('/auth/avatar'),
      ).thenAnswer((_) async => _jsonResponse('/auth/avatar', <String, dynamic>{}));

      await repository.deleteAvatar();

      verify(() => dio.delete<Map<String, dynamic>>('/auth/avatar')).called(1);
    });

    test('uploadAvatar offline: NetworkFailure', () async {
      when(
        () => dio.post<Map<String, dynamic>>(
          '/auth/avatar',
          data: any(named: 'data'),
        ),
      ).thenThrow(
        DioException(
          requestOptions: RequestOptions(path: '/auth/avatar'),
          type: DioExceptionType.connectionError,
        ),
      );
      expect(
        () => repository.uploadAvatar(<int>[1], filename: 'a.jpg'),
        throwsA(isA<NetworkFailure>()),
      );
    });
  });

  group('sessions', () {
    test('GET /auth/sessions: seluruh field SessionView terparse', () async {
      when(() => dio.get<Map<String, dynamic>>('/auth/sessions')).thenAnswer(
        (_) async => _jsonResponse('/auth/sessions', <String, dynamic>{
          'data': <Map<String, dynamic>>[currentSessionJson, otherSessionJson],
        }),
      );

      final List<SessionDto> sessions = await repository.sessions();

      expect(sessions, hasLength(2));
      final SessionDto current = sessions.first;
      expect(current.id, 'sess-1');
      expect(current.browser, 'Inventra App');
      expect(current.os, 'Android');
      expect(current.deviceType, 'mobile');
      expect(current.ipAddress, '103.28.11.4');
      expect(current.location, 'Jakarta, Indonesia');
      expect(current.lastSeenAt, DateTime.utc(2026, 7, 19, 2, 41));
      expect(current.current, isTrue);
      expect(sessions.last.current, isFalse);
      expect(sessions.last.location, isEmpty);
    });

    test('data absen diperlakukan daftar kosong', () async {
      when(() => dio.get<Map<String, dynamic>>('/auth/sessions')).thenAnswer(
        (_) async => _jsonResponse('/auth/sessions', <String, dynamic>{}),
      );

      expect(await repository.sessions(), isEmpty);
    });

    test('offline: NetworkFailure', () async {
      when(
        () => dio.get<Map<String, dynamic>>('/auth/sessions'),
      ).thenThrow(_connectionError('/auth/sessions'));

      expect(() => repository.sessions(), throwsA(isA<NetworkFailure>()));
    });
  });

  group('revokeSession', () {
    test('DELETE /auth/sessions/{id}', () async {
      when(
        () => dio.delete<Map<String, dynamic>>('/auth/sessions/sess-2'),
      ).thenAnswer(
        (_) async => _jsonResponse('/auth/sessions/sess-2', <String, dynamic>{
          'status': 'revoked',
        }),
      );

      await repository.revokeSession('sess-2');

      verify(
        () => dio.delete<Map<String, dynamic>>('/auth/sessions/sess-2'),
      ).called(1);
    });

    test('404 (termasuk id milik pengguna lain): NotFoundFailure', () async {
      when(
        () => dio.delete<Map<String, dynamic>>('/auth/sessions/sess-x'),
      ).thenThrow(_statusError('/auth/sessions/sess-x', 404));

      expect(
        () => repository.revokeSession('sess-x'),
        throwsA(isA<NotFoundFailure>()),
      );
    });

    test('offline: NetworkFailure', () async {
      when(
        () => dio.delete<Map<String, dynamic>>('/auth/sessions/sess-2'),
      ).thenThrow(_connectionError('/auth/sessions/sess-2'));

      expect(
        () => repository.revokeSession('sess-2'),
        throwsA(isA<NetworkFailure>()),
      );
    });
  });

  group('revokeOtherSessions', () {
    test('POST /auth/sessions/revoke-others: jumlah tercabut', () async {
      when(
        () => dio.post<Map<String, dynamic>>('/auth/sessions/revoke-others'),
      ).thenAnswer(
        (_) async => _jsonResponse(
          '/auth/sessions/revoke-others',
          <String, dynamic>{'revoked': 2},
        ),
      );

      expect(await repository.revokeOtherSessions(), 2);
    });

    test('5xx: ServerFailure', () async {
      when(
        () => dio.post<Map<String, dynamic>>('/auth/sessions/revoke-others'),
      ).thenThrow(_statusError('/auth/sessions/revoke-others', 500));

      expect(
        () => repository.revokeOtherSessions(),
        throwsA(isA<ServerFailure>()),
      );
    });
  });

  group('avatar', () {
    test('GET /auth/avatar: bytes gambar', () async {
      when(
        () =>
            dio.get<List<int>>('/auth/avatar', options: any(named: 'options')),
      ).thenAnswer(
        (_) async => Response<List<int>>(
          requestOptions: RequestOptions(path: '/auth/avatar'),
          statusCode: 200,
          data: <int>[1, 2, 3],
        ),
      );

      final Uint8List? bytes = await repository.avatar();

      expect(bytes, Uint8List.fromList(<int>[1, 2, 3]));
    });

    test('404 (belum ada avatar): null, bukan error', () async {
      when(
        () =>
            dio.get<List<int>>('/auth/avatar', options: any(named: 'options')),
      ).thenThrow(_statusError('/auth/avatar', 404));

      expect(await repository.avatar(), isNull);
    });

    test('503 (object storage mati): ServerFailure', () async {
      when(
        () =>
            dio.get<List<int>>('/auth/avatar', options: any(named: 'options')),
      ).thenThrow(_statusError('/auth/avatar', 503));

      expect(() => repository.avatar(), throwsA(isA<ServerFailure>()));
    });

    test('offline: NetworkFailure', () async {
      when(
        () =>
            dio.get<List<int>>('/auth/avatar', options: any(named: 'options')),
      ).thenThrow(_connectionError('/auth/avatar'));

      expect(() => repository.avatar(), throwsA(isA<NetworkFailure>()));
    });
  });
}
