import 'package:dio/dio.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:inventra_mobile/core/api/app_failure.dart';
import 'package:inventra_mobile/core/authz/permissions_provider.dart';
import 'package:mocktail/mocktail.dart';

class _MockDio extends Mock implements Dio {}

Response<Map<String, dynamic>> _resp(Map<String, dynamic> data) {
  return Response<Map<String, dynamic>>(
    requestOptions: RequestOptions(path: '/auth/permissions'),
    statusCode: 200,
    data: data,
  );
}

void main() {
  late _MockDio dio;
  late PermissionsRepository repository;

  setUp(() {
    dio = _MockDio();
    repository = PermissionsRepository(dio);
  });

  test('parse daftar permission jadi Set', () async {
    when(() => dio.get<Map<String, dynamic>>('/auth/permissions')).thenAnswer(
      (_) async => _resp(<String, dynamic>{
        'permissions': <String>['request.create', 'assignment.manage'],
      }),
    );

    expect(await repository.list(), <String>{
      'request.create',
      'assignment.manage',
    });
  });

  test('permissions bukan list: Set kosong', () async {
    when(
      () => dio.get<Map<String, dynamic>>('/auth/permissions'),
    ).thenAnswer((_) async => _resp(<String, dynamic>{'permissions': null}));

    expect(await repository.list(), isEmpty);
  });

  test('offline: NetworkFailure', () async {
    when(() => dio.get<Map<String, dynamic>>('/auth/permissions')).thenThrow(
      DioException(
        requestOptions: RequestOptions(path: '/auth/permissions'),
        type: DioExceptionType.connectionError,
      ),
    );

    expect(() => repository.list(), throwsA(isA<NetworkFailure>()));
  });
}
