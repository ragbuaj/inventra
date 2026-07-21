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
}
