import 'package:dio/dio.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:inventra_mobile/core/api/app_failure.dart';
import 'package:inventra_mobile/features/stock_opname/data/stock_opname_item_list_dto.dart';
import 'package:inventra_mobile/features/stock_opname/data/stock_opname_item_result_dto.dart';
import 'package:inventra_mobile/features/stock_opname/data/stock_opname_repository.dart';
import 'package:inventra_mobile/features/stock_opname/data/stock_opname_scan_result_dto.dart';
import 'package:inventra_mobile/features/stock_opname/data/stock_opname_session_dto.dart';
import 'package:inventra_mobile/features/stock_opname/data/stock_opname_session_list_dto.dart';
import 'package:mocktail/mocktail.dart';

import 'stock_opname_dto_test.dart'
    show detailSessionJson, fullItemJson, listSessionJson;

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

DioException _connectionError(String path) {
  return DioException(
    requestOptions: RequestOptions(path: path),
    type: DioExceptionType.connectionError,
  );
}

Map<String, dynamic> _itemListJson(List<Map<String, dynamic>> data) {
  return <String, dynamic>{
    'data': data,
    'total': data.length,
    'limit': data.length,
    'offset': 0,
  };
}

void main() {
  late _MockDio dio;
  late StockOpnameRepository repository;

  setUp(() {
    dio = _MockDio();
    repository = StockOpnameRepository(dio);
  });

  group('sessions', () {
    test(
      'tanpa filter: TANPA parameter status + limit/offset default',
      () async {
        when(
          () => dio.get<Map<String, dynamic>>(
            '/stock-opname/sessions',
            queryParameters: any(named: 'queryParameters'),
          ),
        ).thenAnswer(
          (_) async =>
              _jsonResponse('/stock-opname/sessions', <String, dynamic>{
                'data': <Map<String, dynamic>>[listSessionJson],
                'total': 1,
                'limit': 20,
                'offset': 0,
              }),
        );

        final StockOpnameSessionListDto page = await repository.sessions();

        expect(page.data.single.id, 'op-1');
        expect(page.total, 1);
        final Map<String, dynamic> query =
            verify(
                  () => dio.get<Map<String, dynamic>>(
                    '/stock-opname/sessions',
                    queryParameters: captureAny(named: 'queryParameters'),
                  ),
                ).captured.single
                as Map<String, dynamic>;
        expect(query, <String, dynamic>{'limit': 20, 'offset': 0});
      },
    );

    test('filter status + limit/offset diteruskan sebagai query', () async {
      when(
        () => dio.get<Map<String, dynamic>>(
          '/stock-opname/sessions',
          queryParameters: any(named: 'queryParameters'),
        ),
      ).thenAnswer(
        (_) async => _jsonResponse('/stock-opname/sessions', <String, dynamic>{
          'data': <Map<String, dynamic>>[],
          'total': 0,
          'limit': 100,
          'offset': 40,
        }),
      );

      await repository.sessions(status: 'closed', limit: 100, offset: 40);

      final Map<String, dynamic> query =
          verify(
                () => dio.get<Map<String, dynamic>>(
                  '/stock-opname/sessions',
                  queryParameters: captureAny(named: 'queryParameters'),
                ),
              ).captured.single
              as Map<String, dynamic>;
      expect(query, <String, dynamic>{
        'status': 'closed',
        'limit': 100,
        'offset': 40,
      });
    });

    test('403 dipetakan ForbiddenFailure', () async {
      when(
        () => dio.get<Map<String, dynamic>>(
          '/stock-opname/sessions',
          queryParameters: any(named: 'queryParameters'),
        ),
      ).thenThrow(_statusError('/stock-opname/sessions', 403));

      expect(repository.sessions(), throwsA(isA<ForbiddenFailure>()));
    });

    test('gangguan koneksi dipetakan NetworkFailure', () async {
      when(
        () => dio.get<Map<String, dynamic>>(
          '/stock-opname/sessions',
          queryParameters: any(named: 'queryParameters'),
        ),
      ).thenThrow(_connectionError('/stock-opname/sessions'));

      expect(repository.sessions(), throwsA(isA<NetworkFailure>()));
    });
  });

  group('session (detail by id)', () {
    test('mengembalikan sesi dengan KPI counter', () async {
      when(
        () => dio.get<Map<String, dynamic>>('/stock-opname/sessions/op-1'),
      ).thenAnswer(
        (_) async =>
            _jsonResponse('/stock-opname/sessions/op-1', detailSessionJson),
      );

      final StockOpnameSessionDto session = await repository.session('op-1');

      expect(session.id, 'op-1');
      expect(session.total, 150);
      expect(session.pending, 22);
    });

    test('404 dipetakan NotFoundFailure', () async {
      when(
        () => dio.get<Map<String, dynamic>>('/stock-opname/sessions/op-x'),
      ).thenThrow(_statusError('/stock-opname/sessions/op-x', 404));

      expect(repository.session('op-x'), throwsA(isA<NotFoundFailure>()));
    });
  });

  group('items', () {
    test('tanpa filter: TANPA parameter result', () async {
      when(
        () => dio.get<Map<String, dynamic>>(
          '/stock-opname/sessions/op-1/items',
          queryParameters: any(named: 'queryParameters'),
        ),
      ).thenAnswer(
        (_) async => _jsonResponse(
          '/stock-opname/sessions/op-1/items',
          _itemListJson(<Map<String, dynamic>>[fullItemJson]),
        ),
      );

      final StockOpnameItemListDto list = await repository.items('op-1');

      expect(list.data.single.id, 'item-1');
      final Map<String, dynamic> query =
          verify(
                () => dio.get<Map<String, dynamic>>(
                  '/stock-opname/sessions/op-1/items',
                  queryParameters: captureAny(named: 'queryParameters'),
                ),
              ).captured.single
              as Map<String, dynamic>;
      expect(query.containsKey('result'), isFalse);
    });

    test('filter result memakai nilai kawat kontrak (not_found)', () async {
      when(
        () => dio.get<Map<String, dynamic>>(
          '/stock-opname/sessions/op-1/items',
          queryParameters: any(named: 'queryParameters'),
        ),
      ).thenAnswer(
        (_) async => _jsonResponse(
          '/stock-opname/sessions/op-1/items',
          _itemListJson(<Map<String, dynamic>>[]),
        ),
      );

      await repository.items('op-1', result: OpnameItemResult.notFound);

      final Map<String, dynamic> query =
          verify(
                () => dio.get<Map<String, dynamic>>(
                  '/stock-opname/sessions/op-1/items',
                  queryParameters: captureAny(named: 'queryParameters'),
                ),
              ).captured.single
              as Map<String, dynamic>;
      expect(query, <String, dynamic>{'result': 'not_found'});
    });

    test('404 (sesi tidak ada) dipetakan NotFoundFailure', () async {
      when(
        () => dio.get<Map<String, dynamic>>(
          '/stock-opname/sessions/op-x/items',
          queryParameters: any(named: 'queryParameters'),
        ),
      ).thenThrow(_statusError('/stock-opname/sessions/op-x/items', 404));

      expect(repository.items('op-x'), throwsA(isA<NotFoundFailure>()));
    });
  });

  group('scan', () {
    test('POST asset_tag; item ter-resolve dikembalikan', () async {
      when(
        () => dio.post<Map<String, dynamic>>(
          '/stock-opname/sessions/op-1/scan',
          data: any(named: 'data'),
        ),
      ).thenAnswer(
        (_) async =>
            _jsonResponse('/stock-opname/sessions/op-1/scan', <String, dynamic>{
              'id': 'item-9',
              'session_id': 'op-1',
              'asset_id': 'asset-9',
              'expected': false,
              'result': 'pending',
            }),
      );

      final StockOpnameScanResultDto result = await repository.scan(
        'op-1',
        'JKT01-ELK-2026-00099',
      );

      expect(result.id, 'item-9');
      expect(result.expected, isFalse);
      final Map<String, dynamic> body =
          verify(
                () => dio.post<Map<String, dynamic>>(
                  '/stock-opname/sessions/op-1/scan',
                  data: captureAny(named: 'data'),
                ),
              ).captured.single
              as Map<String, dynamic>;
      expect(body, <String, dynamic>{'asset_tag': 'JKT01-ELK-2026-00099'});
    });

    test('404 (tag tidak dikenal) dipetakan NotFoundFailure', () async {
      when(
        () => dio.post<Map<String, dynamic>>(
          '/stock-opname/sessions/op-1/scan',
          data: any(named: 'data'),
        ),
      ).thenThrow(_statusError('/stock-opname/sessions/op-1/scan', 404));

      expect(
        repository.scan('op-1', 'TAG-ASING'),
        throwsA(isA<NotFoundFailure>()),
      );
    });

    test('409 (sesi bukan counting) dipetakan ConflictFailure', () async {
      when(
        () => dio.post<Map<String, dynamic>>(
          '/stock-opname/sessions/op-1/scan',
          data: any(named: 'data'),
        ),
      ).thenThrow(_statusError('/stock-opname/sessions/op-1/scan', 409));

      expect(
        repository.scan('op-1', 'JKT01-ELK-2026-00001'),
        throwsA(isA<ConflictFailure>()),
      );
    });
  });

  group('setResult', () {
    Map<String, dynamic> patchResponse() => <String, dynamic>{
      'id': 'item-1',
      'session_id': 'op-1',
      'asset_id': 'asset-1',
      'expected': true,
      'result': 'damaged',
      'note': 'Engsel patah',
      'counted_at': '2026-07-19T03:00:00Z',
    };

    test('PATCH result + note terkirim (note di-trim)', () async {
      when(
        () => dio.patch<Map<String, dynamic>>(
          '/stock-opname/sessions/op-1/items/item-1',
          data: any(named: 'data'),
        ),
      ).thenAnswer(
        (_) async => _jsonResponse(
          '/stock-opname/sessions/op-1/items/item-1',
          patchResponse(),
        ),
      );

      final StockOpnameItemResultDto result = await repository.setResult(
        'op-1',
        'item-1',
        result: OpnameItemResult.damaged,
        note: '  Engsel patah  ',
      );

      expect(result.result, 'damaged');
      final Map<String, dynamic> body =
          verify(
                () => dio.patch<Map<String, dynamic>>(
                  '/stock-opname/sessions/op-1/items/item-1',
                  data: captureAny(named: 'data'),
                ),
              ).captured.single
              as Map<String, dynamic>;
      expect(body, <String, dynamic>{
        'result': 'damaged',
        'note': 'Engsel patah',
      });
    });

    test('note kosong TIDAK dikirim', () async {
      when(
        () => dio.patch<Map<String, dynamic>>(
          '/stock-opname/sessions/op-1/items/item-1',
          data: any(named: 'data'),
        ),
      ).thenAnswer(
        (_) async => _jsonResponse(
          '/stock-opname/sessions/op-1/items/item-1',
          patchResponse(),
        ),
      );

      await repository.setResult(
        'op-1',
        'item-1',
        result: OpnameItemResult.found,
        note: '   ',
      );

      final Map<String, dynamic> body =
          verify(
                () => dio.patch<Map<String, dynamic>>(
                  '/stock-opname/sessions/op-1/items/item-1',
                  data: captureAny(named: 'data'),
                ),
              ).captured.single
              as Map<String, dynamic>;
      expect(body, <String, dynamic>{'result': 'found'});
    });

    test('409 (sesi bukan counting) dipetakan ConflictFailure', () async {
      when(
        () => dio.patch<Map<String, dynamic>>(
          '/stock-opname/sessions/op-1/items/item-1',
          data: any(named: 'data'),
        ),
      ).thenThrow(
        _statusError('/stock-opname/sessions/op-1/items/item-1', 409),
      );

      expect(
        repository.setResult('op-1', 'item-1', result: OpnameItemResult.found),
        throwsA(isA<ConflictFailure>()),
      );
    });
  });

  group('variance', () {
    test(
      'mengelompokkan item per kategori; found/pending bukan selisih',
      () async {
        Map<String, dynamic> item(
          String id,
          String result, {
          bool expected = true,
        }) => <String, dynamic>{
          ...fullItemJson,
          'id': id,
          'result': result,
          'expected': expected,
        };

        when(
          () => dio.get<Map<String, dynamic>>(
            '/stock-opname/sessions/op-1/items',
            queryParameters: any(named: 'queryParameters'),
          ),
        ).thenAnswer(
          (_) async => _jsonResponse(
            '/stock-opname/sessions/op-1/items',
            _itemListJson(<Map<String, dynamic>>[
              item('i-1', 'found'),
              item('i-2', 'not_found'),
              item('i-3', 'damaged'),
              item('i-4', 'misplaced'),
              item('i-5', 'pending'),
              // Temuan di luar snapshot TIDAK ikut kelompok hasil (anti dobel).
              item('i-6', 'found', expected: false),
            ]),
          ),
        );

        final OpnameVarianceData data = await repository.variance('op-1');

        expect(data.notFound.single.id, 'i-2');
        expect(data.damaged.single.id, 'i-3');
        expect(data.misplaced.single.id, 'i-4');
        expect(data.unexpected.single.id, 'i-6');
        expect(data.isEmpty, isFalse);
      },
    );

    test('semua tercocokkan berarti isEmpty', () async {
      when(
        () => dio.get<Map<String, dynamic>>(
          '/stock-opname/sessions/op-1/items',
          queryParameters: any(named: 'queryParameters'),
        ),
      ).thenAnswer(
        (_) async => _jsonResponse(
          '/stock-opname/sessions/op-1/items',
          _itemListJson(<Map<String, dynamic>>[fullItemJson]),
        ),
      );

      final OpnameVarianceData data = await repository.variance('op-1');

      expect(data.isEmpty, isTrue);
    });
  });

  group('OpnameItemResult', () {
    test('tryParse nilai kontrak', () {
      expect(OpnameItemResult.tryParse('found'), OpnameItemResult.found);
      expect(OpnameItemResult.tryParse('not_found'), OpnameItemResult.notFound);
      expect(OpnameItemResult.tryParse('damaged'), OpnameItemResult.damaged);
      expect(
        OpnameItemResult.tryParse('misplaced'),
        OpnameItemResult.misplaced,
      );
      expect(OpnameItemResult.tryParse('pending'), OpnameItemResult.pending);
    });

    test('tryParse nilai asing/null mengembalikan null', () {
      expect(OpnameItemResult.tryParse('unknown'), isNull);
      expect(OpnameItemResult.tryParse(null), isNull);
    });
  });
}
