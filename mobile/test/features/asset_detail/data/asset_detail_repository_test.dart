import 'package:dio/dio.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:inventra_mobile/core/api/app_failure.dart';
import 'package:inventra_mobile/features/asset_detail/data/asset_detail_repository.dart';
import 'package:mocktail/mocktail.dart';

import 'asset_dto_test.dart' show fullAssetJson;

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

DioException _mappedError(String path, AppFailure failure) {
  return DioException(
    requestOptions: RequestOptions(path: path),
    error: failure,
  );
}

void main() {
  late _MockDio dio;
  late AssetDetailRepository repository;

  setUp(() {
    dio = _MockDio();
    repository = AssetDetailRepository(dio);
  });

  group('getByTag', () {
    test('GET /assets/by-tag/{tag} parse DTO, tanpa field dimask', () async {
      when(
        () => dio.get<Map<String, dynamic>>(
          '/assets/by-tag/JKT01-ELK-2026-00001',
        ),
      ).thenAnswer(
        (_) async =>
            _jsonResponse('/assets/by-tag/JKT01-ELK-2026-00001', fullAssetJson),
      );

      final AssetDetailData data = await repository.getByTag(
        'JKT01-ELK-2026-00001',
      );

      expect(data.asset.name, 'Laptop Dell Latitude 5440');
      expect(data.asset.purchaseCost, '18750000.00');
      expect(data.maskedFields, isEmpty);
      expect(data.isMasked('purchase_cost'), isFalse);
    });

    test('tag di-URL-encode pada path', () async {
      when(
        () => dio.get<Map<String, dynamic>>('/assets/by-tag/AB%2001%2F2'),
      ).thenAnswer(
        (_) async => _jsonResponse('/assets/by-tag/AB%2001%2F2', fullAssetJson),
      );

      await repository.getByTag('AB 01/2');

      verify(
        () => dio.get<Map<String, dynamic>>('/assets/by-tag/AB%2001%2F2'),
      ).called(1);
    });

    test('field permission: kunci absen menjadi DTO null + masuk maskedFields, '
        'kunci bernilai null TIDAK dianggap dimask', () async {
      final Map<String, dynamic> maskedJson =
          Map<String, dynamic>.of(fullAssetJson)
            ..remove('purchase_cost')
            ..remove('book_value')
            ..remove('accumulated_depreciation');
      when(
        () => dio.get<Map<String, dynamic>>('/assets/by-tag/TAG-1'),
      ).thenAnswer(
        (_) async => _jsonResponse('/assets/by-tag/TAG-1', maskedJson),
      );

      final AssetDetailData data = await repository.getByTag('TAG-1');

      expect(data.asset.purchaseCost, isNull);
      expect(data.asset.bookValue, isNull);
      expect(data.maskedFields, <String>{
        'purchase_cost',
        'book_value',
        'accumulated_depreciation',
      });
      // unit_id dikirim null (bukan dimask) — tidak masuk maskedFields.
      expect(data.asset.unitId, isNull);
      expect(data.isMasked('unit_id'), isFalse);
    });

    test('404 (tag tak dikenal / di luar scope): NotFoundFailure', () async {
      when(
        () => dio.get<Map<String, dynamic>>('/assets/by-tag/INV-XX-90412'),
      ).thenThrow(
        _mappedError('/assets/by-tag/INV-XX-90412', const NotFoundFailure()),
      );

      expect(
        () => repository.getByTag('INV-XX-90412'),
        throwsA(isA<NotFoundFailure>()),
      );
    });

    test('403 (tanpa izin asset.view): ForbiddenFailure', () async {
      final RequestOptions options = RequestOptions(
        path: '/assets/by-tag/TAG-1',
      );
      // DioException mentah (belum lewat interceptor) tetap terpetakan.
      when(
        () => dio.get<Map<String, dynamic>>('/assets/by-tag/TAG-1'),
      ).thenThrow(
        DioException(
          requestOptions: options,
          type: DioExceptionType.badResponse,
          response: Response<dynamic>(requestOptions: options, statusCode: 403),
        ),
      );

      expect(
        () => repository.getByTag('TAG-1'),
        throwsA(isA<ForbiddenFailure>()),
      );
    });

    test('offline: NetworkFailure', () async {
      when(
        () => dio.get<Map<String, dynamic>>('/assets/by-tag/TAG-1'),
      ).thenThrow(_mappedError('/assets/by-tag/TAG-1', const NetworkFailure()));

      expect(
        () => repository.getByTag('TAG-1'),
        throwsA(isA<NetworkFailure>()),
      );
    });
  });
}
