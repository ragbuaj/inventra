import 'package:dio/dio.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:inventra_mobile/core/api/app_failure.dart';
import 'package:inventra_mobile/features/my_assets/data/my_assets_repository.dart';
import 'package:mocktail/mocktail.dart';

class _MockDio extends Mock implements Dio {}

Response<Map<String, dynamic>> _jsonResponse(Map<String, dynamic> data) {
  return Response<Map<String, dynamic>>(
    requestOptions: RequestOptions(path: '/assignments/mine'),
    statusCode: 200,
    data: data,
  );
}

DioException _statusError(int statusCode) {
  final RequestOptions options = RequestOptions(path: '/assignments/mine');
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

void main() {
  late _MockDio dio;
  late MyAssetsRepository repository;

  setUp(() {
    dio = _MockDio();
    repository = MyAssetsRepository(dio);
  });

  void stub(Map<String, dynamic> data) {
    when(
      () => dio.get<Map<String, dynamic>>(
        '/assignments/mine',
        queryParameters: any(named: 'queryParameters'),
      ),
    ).thenAnswer((_) async => _jsonResponse(data));
  }

  test('parse item + query status=active', () async {
    stub(<String, dynamic>{
      'data': <Map<String, dynamic>>[
        <String, dynamic>{
          'asset_name': 'Laptop Dell Latitude 5440',
          'asset_tag': 'JKT01-ELK-2026-00001',
          'status': 'active',
          'checkout_date': '2026-07-01T00:00:00Z',
          'due_date': '2026-08-01',
        },
      ],
    });

    final List<MyAssignmentDto> items = await repository.list();

    expect(items, hasLength(1));
    expect(items.single.assetName, 'Laptop Dell Latitude 5440');
    expect(items.single.assetTag, 'JKT01-ELK-2026-00001');
    expect(items.single.dueDate, '2026-08-01');

    final Map<String, dynamic> query =
        verify(
              () => dio.get<Map<String, dynamic>>(
                '/assignments/mine',
                queryParameters: captureAny(named: 'queryParameters'),
              ),
            ).captured.single
            as Map<String, dynamic>;
    expect(query['status'], 'active');
  });

  test('data kosong: daftar kosong', () async {
    stub(<String, dynamic>{'data': <Map<String, dynamic>>[]});
    expect(await repository.list(), isEmpty);
  });

  test('offline: NetworkFailure', () async {
    when(
      () => dio.get<Map<String, dynamic>>(
        '/assignments/mine',
        queryParameters: any(named: 'queryParameters'),
      ),
    ).thenThrow(
      DioException(
        requestOptions: RequestOptions(path: '/assignments/mine'),
        type: DioExceptionType.connectionError,
      ),
    );
    expect(() => repository.list(), throwsA(isA<NetworkFailure>()));
  });

  test('403: ForbiddenFailure', () async {
    when(
      () => dio.get<Map<String, dynamic>>(
        '/assignments/mine',
        queryParameters: any(named: 'queryParameters'),
      ),
    ).thenThrow(_statusError(403));
    expect(() => repository.list(), throwsA(isA<ForbiddenFailure>()));
  });
}
