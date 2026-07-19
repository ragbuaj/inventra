import 'package:dio/dio.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:inventra_mobile/core/api/app_failure.dart';
import 'package:inventra_mobile/features/approval/data/approval_repository.dart';
import 'package:inventra_mobile/features/approval/data/request_dto.dart';
import 'package:inventra_mobile/features/approval/data/request_list_dto.dart';
import 'package:mocktail/mocktail.dart';

import 'request_dto_test.dart' show fullRequestDetailJson, fullRequestJson;

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

Map<String, dynamic> _listJson({
  List<Map<String, dynamic>>? data,
  int total = 1,
  int limit = 20,
  int offset = 0,
}) {
  return <String, dynamic>{
    'data': data ?? <Map<String, dynamic>>[fullRequestJson],
    'total': total,
    'limit': limit,
    'offset': offset,
  };
}

void main() {
  late _MockDio dio;
  late ApprovalRepository repository;

  setUp(() {
    dio = _MockDio();
    repository = ApprovalRepository(dio);
  });

  group('list', () {
    test(
      'filter pending: query status=pending + limit/offset default',
      () async {
        when(
          () => dio.get<Map<String, dynamic>>(
            '/requests',
            queryParameters: any(named: 'queryParameters'),
          ),
        ).thenAnswer((_) async => _jsonResponse('/requests', _listJson()));

        final RequestListDto page = await repository.list(
          filter: ApprovalStatusFilter.pending,
        );

        expect(page.data.single.id, 'req-1');
        final Map<String, dynamic> query =
            verify(
                  () => dio.get<Map<String, dynamic>>(
                    '/requests',
                    queryParameters: captureAny(named: 'queryParameters'),
                  ),
                ).captured.single
                as Map<String, dynamic>;
        expect(query, <String, dynamic>{
          'status': 'pending',
          'limit': 20,
          'offset': 0,
        });
      },
    );

    test('filter semua: TANPA parameter status', () async {
      when(
        () => dio.get<Map<String, dynamic>>(
          '/requests',
          queryParameters: any(named: 'queryParameters'),
        ),
      ).thenAnswer((_) async => _jsonResponse('/requests', _listJson()));

      await repository.list(filter: ApprovalStatusFilter.all);

      final Map<String, dynamic> query =
          verify(
                () => dio.get<Map<String, dynamic>>(
                  '/requests',
                  queryParameters: captureAny(named: 'queryParameters'),
                ),
              ).captured.single
              as Map<String, dynamic>;
      expect(query.containsKey('status'), isFalse);
    });

    test('pagination: offset halaman berikutnya diteruskan', () async {
      when(
        () => dio.get<Map<String, dynamic>>(
          '/requests',
          queryParameters: any(named: 'queryParameters'),
        ),
      ).thenAnswer(
        (_) async =>
            _jsonResponse('/requests', _listJson(total: 45, offset: 20)),
      );

      final RequestListDto page = await repository.list(
        filter: ApprovalStatusFilter.approved,
        offset: 20,
      );

      expect(page.total, 45);
      final Map<String, dynamic> query =
          verify(
                () => dio.get<Map<String, dynamic>>(
                  '/requests',
                  queryParameters: captureAny(named: 'queryParameters'),
                ),
              ).captured.single
              as Map<String, dynamic>;
      expect(query['status'], 'approved');
      expect(query['offset'], 20);
    });

    test('offline: NetworkFailure', () async {
      when(
        () => dio.get<Map<String, dynamic>>(
          '/requests',
          queryParameters: any(named: 'queryParameters'),
        ),
      ).thenThrow(
        DioException(
          requestOptions: RequestOptions(path: '/requests'),
          type: DioExceptionType.connectionError,
        ),
      );

      expect(
        () => repository.list(filter: ApprovalStatusFilter.pending),
        throwsA(isA<NetworkFailure>()),
      );
    });
  });

  group('detail', () {
    test('parse DTO + tidak ada field dimask', () async {
      when(() => dio.get<Map<String, dynamic>>('/requests/req-1')).thenAnswer(
        (_) async => _jsonResponse('/requests/req-1', fullRequestDetailJson),
      );

      final ApprovalDetailData data = await repository.detail('req-1');

      expect(data.request.id, 'req-1');
      expect(data.request.steps, hasLength(2));
      expect(data.maskedFields, isEmpty);
      expect(data.isMasked('amount'), isFalse);
    });

    test(
      'field permission: amount/payload/reason absen masuk maskedFields',
      () async {
        final Map<String, dynamic> masked =
            Map<String, dynamic>.of(fullRequestDetailJson)
              ..remove('amount')
              ..remove('payload')
              ..remove('reason');
        when(
          () => dio.get<Map<String, dynamic>>('/requests/req-1'),
        ).thenAnswer((_) async => _jsonResponse('/requests/req-1', masked));

        final ApprovalDetailData data = await repository.detail('req-1');

        expect(data.maskedFields, <String>{'amount', 'payload', 'reason'});
        expect(data.request.amount, isNull);
        expect(data.request.payload, isNull);
      },
    );

    test('404: NotFoundFailure', () async {
      when(
        () => dio.get<Map<String, dynamic>>('/requests/req-x'),
      ).thenThrow(_statusError('/requests/req-x', 404));

      expect(() => repository.detail('req-x'), throwsA(isA<NotFoundFailure>()));
    });
  });

  group('approve / reject', () {
    test('approve tanpa catatan: POST tanpa body', () async {
      when(
        () => dio.post<Map<String, dynamic>>(
          '/requests/req-1/approve',
          data: any<dynamic>(named: 'data'),
        ),
      ).thenAnswer(
        (_) async => _jsonResponse('/requests/req-1/approve', <String, dynamic>{
          ...fullRequestJson,
          'status': 'approved',
          'decided_by_id': 'user-1',
        }),
      );

      final RequestDto updated = await repository.approve('req-1');

      expect(updated.status, 'approved');
      verify(
        () => dio.post<Map<String, dynamic>>(
          '/requests/req-1/approve',
          data: null,
        ),
      ).called(1);
    });

    test('approve dengan catatan: body DecideRequest {note}', () async {
      when(
        () => dio.post<Map<String, dynamic>>(
          '/requests/req-1/approve',
          data: any<dynamic>(named: 'data'),
        ),
      ).thenAnswer(
        (_) async => _jsonResponse('/requests/req-1/approve', fullRequestJson),
      );

      await repository.approve('req-1', note: '  Setuju, dokumentasikan.  ');

      verify(
        () => dio.post<Map<String, dynamic>>(
          '/requests/req-1/approve',
          data: <String, dynamic>{'note': 'Setuju, dokumentasikan.'},
        ),
      ).called(1);
    });

    test('reject dengan catatan: body DecideRequest {note}', () async {
      when(
        () => dio.post<Map<String, dynamic>>(
          '/requests/req-1/reject',
          data: any<dynamic>(named: 'data'),
        ),
      ).thenAnswer(
        (_) async => _jsonResponse('/requests/req-1/reject', <String, dynamic>{
          ...fullRequestJson,
          'status': 'rejected',
        }),
      );

      final RequestDto updated = await repository.reject(
        'req-1',
        note: 'Unit tujuan belum siap menerima',
      );

      expect(updated.status, 'rejected');
      verify(
        () => dio.post<Map<String, dynamic>>(
          '/requests/req-1/reject',
          data: <String, dynamic>{'note': 'Unit tujuan belum siap menerima'},
        ),
      ).called(1);
    });

    test(
      'reject tanpa catatan: kontrak tidak mewajibkan — POST tanpa body',
      () async {
        when(
          () => dio.post<Map<String, dynamic>>(
            '/requests/req-1/reject',
            data: any<dynamic>(named: 'data'),
          ),
        ).thenAnswer(
          (_) async => _jsonResponse('/requests/req-1/reject', fullRequestJson),
        );

        await repository.reject('req-1');

        verify(
          () => dio.post<Map<String, dynamic>>(
            '/requests/req-1/reject',
            data: null,
          ),
        ).called(1);
      },
    );

    test('403 SoD (maker/approver sebelumnya): ForbiddenFailure', () async {
      when(
        () => dio.post<Map<String, dynamic>>(
          '/requests/req-1/approve',
          data: any<dynamic>(named: 'data'),
        ),
      ).thenThrow(_statusError('/requests/req-1/approve', 403));

      expect(
        () => repository.approve('req-1'),
        throwsA(isA<ForbiddenFailure>()),
      );
    });

    test('409 (state tidak mengizinkan): ConflictFailure', () async {
      when(
        () => dio.post<Map<String, dynamic>>(
          '/requests/req-1/reject',
          data: any<dynamic>(named: 'data'),
        ),
      ).thenThrow(_statusError('/requests/req-1/reject', 409));

      expect(
        () => repository.reject('req-1', note: 'terlambat'),
        throwsA(isA<ConflictFailure>()),
      );
    });
  });

  group('inboxCount', () {
    test('sukses: nilai count', () async {
      when(
        () => dio.get<Map<String, dynamic>>('/requests/inbox/count'),
      ).thenAnswer(
        (_) async => _jsonResponse('/requests/inbox/count', <String, dynamic>{
          'count': 17,
        }),
      );

      expect(await repository.inboxCount(), 17);
    });

    test('403 (tanpa request.decide): ForbiddenFailure', () async {
      when(
        () => dio.get<Map<String, dynamic>>('/requests/inbox/count'),
      ).thenThrow(_statusError('/requests/inbox/count', 403));

      expect(() => repository.inboxCount(), throwsA(isA<ForbiddenFailure>()));
    });
  });
}
