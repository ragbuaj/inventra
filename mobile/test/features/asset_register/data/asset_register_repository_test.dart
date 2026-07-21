import 'package:dio/dio.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:inventra_mobile/core/api/app_failure.dart';
import 'package:inventra_mobile/features/asset_register/data/asset_register_repository.dart';
import 'package:mocktail/mocktail.dart';

class _MockDio extends Mock implements Dio {}

Response<Map<String, dynamic>> _ok() {
  return Response<Map<String, dynamic>>(
    requestOptions: RequestOptions(path: '/requests'),
    statusCode: 201,
    data: <String, dynamic>{'id': 'req-1'},
  );
}

void main() {
  late _MockDio dio;
  late AssetRegisterRepository repository;

  setUp(() {
    dio = _MockDio();
    repository = AssetRegisterRepository(dio);
  });

  Map<String, dynamic> capturedBody() {
    return verify(
          () => dio.post<Map<String, dynamic>>(
            '/requests',
            data: captureAny(named: 'data'),
          ),
        ).captured.single
        as Map<String, dynamic>;
  }

  void stubOk() {
    when(
      () => dio.post<Map<String, dynamic>>('/requests', data: any(named: 'data')),
    ).thenAnswer((_) async => _ok());
  }

  test('dengan harga: amount == purchase_cost, payload lengkap', () async {
    stubOk();

    await repository.register(
      name: '  Laptop Dell  ',
      categoryId: 'cat-1',
      officeId: 'off-1',
      assetClass: 'tangible',
      purchaseCost: '15000000',
      purchaseDate: '2026-07-01',
      serialNumber: '  SN-9  ',
      notes: '  baru  ',
    );

    final Map<String, dynamic> body = capturedBody();
    expect(body['type'], 'asset_create');
    expect(body['amount'], '15000000');
    expect(body['office_id'], 'off-1');
    expect(body['payload'], <String, dynamic>{
      'name': 'Laptop Dell',
      'category_id': 'cat-1',
      'office_id': 'off-1',
      'asset_class': 'tangible',
      'purchase_cost': '15000000',
      'purchase_date': '2026-07-01',
      'serial_number': 'SN-9',
      'notes': 'baru',
    });
  });

  test('tanpa harga: amount "0", payload tanpa purchase_cost', () async {
    stubOk();

    await repository.register(
      name: 'Kursi',
      categoryId: 'cat-2',
      officeId: 'off-1',
      assetClass: 'tangible',
    );

    final Map<String, dynamic> body = capturedBody();
    expect(body['amount'], '0');
    final Map<String, dynamic> payload = body['payload'] as Map<String, dynamic>;
    expect(payload.containsKey('purchase_cost'), isFalse);
    expect(payload['name'], 'Kursi');
  });

  test('offline: NetworkFailure', () async {
    when(
      () => dio.post<Map<String, dynamic>>('/requests', data: any(named: 'data')),
    ).thenThrow(
      DioException(
        requestOptions: RequestOptions(path: '/requests'),
        type: DioExceptionType.connectionError,
      ),
    );

    expect(
      () => repository.register(
        name: 'X',
        categoryId: 'c',
        officeId: 'o',
        assetClass: 'tangible',
      ),
      throwsA(isA<NetworkFailure>()),
    );
  });
}
