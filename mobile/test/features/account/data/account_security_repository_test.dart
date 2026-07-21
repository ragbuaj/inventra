import 'package:dio/dio.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:inventra_mobile/core/api/app_failure.dart';
import 'package:inventra_mobile/features/account/data/account_security_repository.dart';
import 'package:mocktail/mocktail.dart';

class _MockDio extends Mock implements Dio {}

Response<Map<String, dynamic>> _ok(String path) {
  return Response<Map<String, dynamic>>(
    requestOptions: RequestOptions(path: path),
    statusCode: 200,
    data: <String, dynamic>{'status': 'ok'},
  );
}

DioException _statusError(String path, int code) {
  final RequestOptions options = RequestOptions(path: path);
  return DioException(
    requestOptions: options,
    type: DioExceptionType.badResponse,
    response: Response<dynamic>(
      requestOptions: options,
      statusCode: code,
      data: <String, dynamic>{'error': 'server message'},
    ),
  );
}

void main() {
  late _MockDio dio;
  late AccountSecurityRepository repository;

  setUp(() {
    dio = _MockDio();
    repository = AccountSecurityRepository(dio);
  });

  group('requestPasswordChange', () {
    test('POST body current_password', () async {
      when(
        () => dio.post<Map<String, dynamic>>(
          '/auth/password/change-request',
          data: any(named: 'data'),
        ),
      ).thenAnswer((_) async => _ok('/auth/password/change-request'));

      await repository.requestPasswordChange('rahasia123');

      final Map<String, dynamic> body =
          verify(
                () => dio.post<Map<String, dynamic>>(
                  '/auth/password/change-request',
                  data: captureAny(named: 'data'),
                ),
              ).captured.single
              as Map<String, dynamic>;
      expect(body, <String, dynamic>{'current_password': 'rahasia123'});
    });

    test('400 password salah: ValidationFailure (bukan Unauthorized)', () async {
      when(
        () => dio.post<Map<String, dynamic>>(
          '/auth/password/change-request',
          data: any(named: 'data'),
        ),
      ).thenThrow(_statusError('/auth/password/change-request', 400));

      expect(
        () => repository.requestPasswordChange('x'),
        throwsA(isA<ValidationFailure>()),
      );
    });
  });

  group('requestEmailChange', () {
    test('POST body new_email + current_password (trim email)', () async {
      when(
        () => dio.post<Map<String, dynamic>>(
          '/auth/email/change-request',
          data: any(named: 'data'),
        ),
      ).thenAnswer((_) async => _ok('/auth/email/change-request'));

      await repository.requestEmailChange(
        newEmail: '  baru@x.local  ',
        currentPassword: 'p',
      );

      final Map<String, dynamic> body =
          verify(
                () => dio.post<Map<String, dynamic>>(
                  '/auth/email/change-request',
                  data: captureAny(named: 'data'),
                ),
              ).captured.single
              as Map<String, dynamic>;
      expect(body, <String, dynamic>{
        'new_email': 'baru@x.local',
        'current_password': 'p',
      });
    });

    test('409 email dipakai: ConflictFailure', () async {
      when(
        () => dio.post<Map<String, dynamic>>(
          '/auth/email/change-request',
          data: any(named: 'data'),
        ),
      ).thenThrow(_statusError('/auth/email/change-request', 409));

      expect(
        () => repository.requestEmailChange(newEmail: 'a@b.c', currentPassword: 'p'),
        throwsA(isA<ConflictFailure>()),
      );
    });
  });
}
