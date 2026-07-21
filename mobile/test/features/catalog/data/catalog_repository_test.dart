import 'package:dio/dio.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:inventra_mobile/core/api/app_failure.dart';
import 'package:inventra_mobile/features/catalog/data/asset_list_dto.dart';
import 'package:inventra_mobile/features/catalog/data/catalog_repository.dart';
import 'package:mocktail/mocktail.dart';

class _MockDio extends Mock implements Dio {}

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

Map<String, dynamic> _listJson({int total = 1, int offset = 0}) {
  return <String, dynamic>{
    'data': <Map<String, dynamic>>[
      <String, dynamic>{
        'id': 'a1',
        'asset_tag': 'JKT01-ELK-2026-00001',
        'name': 'Laptop Dell Latitude 5440',
        'status': 'available',
      },
    ],
    'total': total,
    'limit': 20,
    'offset': offset,
  };
}

void main() {
  late _MockDio dio;
  late CatalogRepository repository;

  setUp(() {
    dio = _MockDio();
    repository = CatalogRepository(dio);
  });

  Map<String, dynamic> capturedQuery() {
    return verify(
          () => dio.get<Map<String, dynamic>>(
            '/assets',
            queryParameters: captureAny(named: 'queryParameters'),
          ),
        ).captured.single
        as Map<String, dynamic>;
  }

  void stubOk() {
    when(
      () => dio.get<Map<String, dynamic>>(
        '/assets',
        queryParameters: any(named: 'queryParameters'),
      ),
    ).thenAnswer((_) async => _jsonResponse('/assets', _listJson()));
  }

  group('list', () {
    test('tanpa search: query hanya limit/offset', () async {
      stubOk();

      final AssetListDto page = await repository.list();

      expect(page.data.single.assetTag, 'JKT01-ELK-2026-00001');
      final Map<String, dynamic> query = capturedQuery();
      expect(query, <String, dynamic>{'limit': 20, 'offset': 0});
      expect(query.containsKey('search'), isFalse);
    });

    test('search di-trim dan dikirim', () async {
      stubOk();

      await repository.list(search: '  laptop  ');

      expect(capturedQuery()['search'], 'laptop');
    });

    test('search kosong/whitespace: parameter tidak dikirim', () async {
      stubOk();

      await repository.list(search: '   ');

      expect(capturedQuery().containsKey('search'), isFalse);
    });

    test('pagination: offset diteruskan', () async {
      when(
        () => dio.get<Map<String, dynamic>>(
          '/assets',
          queryParameters: any(named: 'queryParameters'),
        ),
      ).thenAnswer(
        (_) async => _jsonResponse('/assets', _listJson(total: 45, offset: 20)),
      );

      final AssetListDto page = await repository.list(offset: 20);

      expect(page.total, 45);
      expect(capturedQuery()['offset'], 20);
    });

    test('offline: NetworkFailure', () async {
      when(
        () => dio.get<Map<String, dynamic>>(
          '/assets',
          queryParameters: any(named: 'queryParameters'),
        ),
      ).thenThrow(
        DioException(
          requestOptions: RequestOptions(path: '/assets'),
          type: DioExceptionType.connectionError,
        ),
      );

      expect(() => repository.list(), throwsA(isA<NetworkFailure>()));
    });

    test('403: ForbiddenFailure', () async {
      when(
        () => dio.get<Map<String, dynamic>>(
          '/assets',
          queryParameters: any(named: 'queryParameters'),
        ),
      ).thenThrow(_statusError('/assets', 403));

      expect(() => repository.list(), throwsA(isA<ForbiddenFailure>()));
    });
  });
}
