import 'package:dio/dio.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:inventra_mobile/core/api/app_failure.dart';
import 'package:inventra_mobile/features/asset_detail/data/asset_action_repository.dart';
import 'package:mocktail/mocktail.dart';

class _MockDio extends Mock implements Dio {}

Response<Map<String, dynamic>> _ok() {
  return Response<Map<String, dynamic>>(
    requestOptions: RequestOptions(path: '/assignments/borrow'),
    statusCode: 201,
    data: <String, dynamic>{'id': 'req-1'},
  );
}

void main() {
  late _MockDio dio;
  late AssetActionRepository repository;

  setUp(() {
    dio = _MockDio();
    repository = AssetActionRepository(dio);
  });

  Map<String, dynamic> capturedBody() {
    return verify(
          () => dio.post<Map<String, dynamic>>(
            '/assignments/borrow',
            data: captureAny(named: 'data'),
          ),
        ).captured.single
        as Map<String, dynamic>;
  }

  test('borrow: body asset_id + due_date + notes (di-trim)', () async {
    when(
      () => dio.post<Map<String, dynamic>>(
        '/assignments/borrow',
        data: any(named: 'data'),
      ),
    ).thenAnswer((_) async => _ok());

    await repository.borrow(
      assetId: 'asset-1',
      dueDate: '2026-08-01',
      notes: '  presentasi  ',
    );

    expect(capturedBody(), <String, dynamic>{
      'asset_id': 'asset-1',
      'due_date': '2026-08-01',
      'notes': 'presentasi',
    });
  });

  test('borrow tanpa due_date/notes: hanya asset_id', () async {
    when(
      () => dio.post<Map<String, dynamic>>(
        '/assignments/borrow',
        data: any(named: 'data'),
      ),
    ).thenAnswer((_) async => _ok());

    await repository.borrow(assetId: 'asset-1');

    expect(capturedBody(), <String, dynamic>{'asset_id': 'asset-1'});
  });

  test('offline: NetworkFailure', () async {
    when(
      () => dio.post<Map<String, dynamic>>(
        '/assignments/borrow',
        data: any(named: 'data'),
      ),
    ).thenThrow(
      DioException(
        requestOptions: RequestOptions(path: '/assignments/borrow'),
        type: DioExceptionType.connectionError,
      ),
    );

    expect(
      () => repository.borrow(assetId: 'asset-1'),
      throwsA(isA<NetworkFailure>()),
    );
  });

  test('checkout: body asset_id/employee_id/checkout_date/due_date/condition', () async {
    when(
      () => dio.post<Map<String, dynamic>>(
        '/assignments',
        data: any(named: 'data'),
      ),
    ).thenAnswer(
      (_) async => Response<Map<String, dynamic>>(
        requestOptions: RequestOptions(path: '/assignments'),
        statusCode: 201,
        data: <String, dynamic>{'id': 'as-1'},
      ),
    );

    await repository.checkout(
      assetId: 'asset-1',
      employeeId: 'emp-1',
      checkoutDate: '2026-07-21',
      dueDate: '2026-08-21',
      conditionOut: '  baik  ',
    );

    final Map<String, dynamic> body =
        verify(
              () => dio.post<Map<String, dynamic>>(
                '/assignments',
                data: captureAny(named: 'data'),
              ),
            ).captured.single
            as Map<String, dynamic>;
    expect(body, <String, dynamic>{
      'asset_id': 'asset-1',
      'employee_id': 'emp-1',
      'checkout_date': '2026-07-21',
      'due_date': '2026-08-21',
      'condition_out': 'baik',
    });
  });

  test('checkin: POST /assignments/:id/checkin body needs_maintenance', () async {
    when(
      () => dio.post<Map<String, dynamic>>(
        '/assignments/as-1/checkin',
        data: any(named: 'data'),
      ),
    ).thenAnswer(
      (_) async => Response<Map<String, dynamic>>(
        requestOptions: RequestOptions(path: '/assignments/as-1/checkin'),
        statusCode: 200,
        data: <String, dynamic>{'id': 'as-1'},
      ),
    );

    await repository.checkin(assignmentId: 'as-1', needsMaintenance: true);

    final Map<String, dynamic> body =
        verify(
              () => dio.post<Map<String, dynamic>>(
                '/assignments/as-1/checkin',
                data: captureAny(named: 'data'),
              ),
            ).captured.single
            as Map<String, dynamic>;
    expect(body, <String, dynamic>{'needs_maintenance': true});
  });

  group('activeAssignment', () {
    void stubList(List<Map<String, dynamic>> rows) {
      when(
        () => dio.get<Map<String, dynamic>>('/assets/asset-1/assignments'),
      ).thenAnswer(
        (_) async => Response<Map<String, dynamic>>(
          requestOptions: RequestOptions(path: '/assets/asset-1/assignments'),
          statusCode: 200,
          data: <String, dynamic>{'data': rows},
        ),
      );
    }

    test('memilih baris status active + nama pemegang', () async {
      stubList(<Map<String, dynamic>>[
        <String, dynamic>{
          'id': 'as-0',
          'status': 'returned',
          'employee_name': 'Lama',
        },
        <String, dynamic>{
          'id': 'as-1',
          'status': 'active',
          'employee_name': 'Budi',
        },
      ]);

      final active = await repository.activeAssignment('asset-1');
      expect(active?.id, 'as-1');
      expect(active?.holderName, 'Budi');
    });

    test('tanpa baris active: null', () async {
      stubList(<Map<String, dynamic>>[
        <String, dynamic>{'id': 'as-0', 'status': 'returned'},
      ]);
      expect(await repository.activeAssignment('asset-1'), isNull);
    });
  });

  test('searchEmployees: query search/limit + parse', () async {
    when(
      () => dio.get<Map<String, dynamic>>(
        '/employees',
        queryParameters: any(named: 'queryParameters'),
      ),
    ).thenAnswer(
      (_) async => Response<Map<String, dynamic>>(
        requestOptions: RequestOptions(path: '/employees'),
        statusCode: 200,
        data: <String, dynamic>{
          'data': <Map<String, dynamic>>[
            <String, dynamic>{'id': 'emp-1', 'name': 'Budi Santoso'},
          ],
        },
      ),
    );

    final result = await repository.searchEmployees('budi');
    expect(result.single.id, 'emp-1');
    expect(result.single.name, 'Budi Santoso');

    final Map<String, dynamic> query =
        verify(
              () => dio.get<Map<String, dynamic>>(
                '/employees',
                queryParameters: captureAny(named: 'queryParameters'),
              ),
            ).captured.single
            as Map<String, dynamic>;
    expect(query['search'], 'budi');
  });
}
