import 'package:dio/dio.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:inventra_mobile/core/api/app_failure.dart';
import 'package:inventra_mobile/features/approval/data/approval_repository.dart'
    show ApprovalStatusFilter;
import 'package:inventra_mobile/features/approval/data/request_dto.dart';
import 'package:inventra_mobile/features/approval/data/request_list_dto.dart';
import 'package:inventra_mobile/features/my_requests/data/my_requests_repository.dart';
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

Map<String, dynamic> _requestJson({String status = 'pending'}) {
  return <String, dynamic>{
    'id': 'req-1',
    'type': 'assignment',
    'status': status,
    'current_step': 1,
    'requested_by_id': 'user-me',
    'reason': 'Peminjaman Proyektor',
    'created_at': '2026-07-20T09:00:00Z',
  };
}

Map<String, dynamic> _listJson() {
  return <String, dynamic>{
    'data': <Map<String, dynamic>>[_requestJson()],
    'total': 1,
    'limit': 20,
    'offset': 0,
  };
}

void main() {
  late _MockDio dio;
  late MyRequestsRepository repository;

  setUp(() {
    dio = _MockDio();
    repository = MyRequestsRepository(dio);
  });

  group('list', () {
    test('mine=true + status pending + limit/offset', () async {
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
      expect(query['mine'], 'true');
      expect(query['status'], 'pending');
    });

    test('filter semua: mine=true tanpa status', () async {
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
      expect(query['mine'], 'true');
      expect(query.containsKey('status'), isFalse);
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

  group('cancel', () {
    test('sukses: POST cancel mengembalikan pengajuan dibatalkan', () async {
      when(
        () => dio.post<Map<String, dynamic>>('/requests/req-1/cancel'),
      ).thenAnswer(
        (_) async => _jsonResponse(
          '/requests/req-1/cancel',
          _requestJson(status: 'cancelled'),
        ),
      );

      final RequestDto out = await repository.cancel('req-1');

      expect(out.status, 'cancelled');
    });

    test('409 (bukan pending): ConflictFailure', () async {
      when(
        () => dio.post<Map<String, dynamic>>('/requests/req-1/cancel'),
      ).thenThrow(_statusError('/requests/req-1/cancel', 409));

      expect(
        () => repository.cancel('req-1'),
        throwsA(isA<ConflictFailure>()),
      );
    });

    test('403 (bukan milik): ForbiddenFailure', () async {
      when(
        () => dio.post<Map<String, dynamic>>('/requests/req-1/cancel'),
      ).thenThrow(_statusError('/requests/req-1/cancel', 403));

      expect(
        () => repository.cancel('req-1'),
        throwsA(isA<ForbiddenFailure>()),
      );
    });
  });
}
