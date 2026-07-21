import 'package:dio/dio.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../../core/api/dio_provider.dart';
import '../../../core/api/error_mapper.dart';

/// Registrasi aset baru: `POST /requests` type `asset_create` (pengajuan via
/// maker-checker). Payload mengikuti `AssetCreatePayload` backend; `amount`
/// request WAJIB sama dengan `payload.purchase_cost` (nol bila harga kosong) —
/// server menolak bila beda (cegah understate band approval).
class AssetRegisterRepository {
  AssetRegisterRepository(this._dio);

  final Dio _dio;

  Future<void> register({
    required String name,
    required String categoryId,
    required String officeId,
    required String assetClass,
    String? purchaseCost,
    String? purchaseDate,
    String? serialNumber,
    String? notes,
  }) async {
    final String? cost = _clean(purchaseCost);
    final String? serial = _clean(serialNumber);
    final String? note = _clean(notes);

    final Map<String, dynamic> payload = <String, dynamic>{
      'name': name.trim(),
      'category_id': categoryId,
      'office_id': officeId,
      'asset_class': assetClass,
      'purchase_cost': ?cost,
      'purchase_date': ?purchaseDate,
      'serial_number': ?serial,
      'notes': ?note,
    };

    try {
      await _dio.post<Map<String, dynamic>>(
        '/requests',
        data: <String, dynamic>{
          'type': 'asset_create',
          // amount == purchase_cost (kontrak); '0' bila harga tidak diisi.
          'amount': cost ?? '0',
          'office_id': officeId,
          'payload': payload,
        },
      );
    } on DioException catch (err) {
      throw err.toAppFailure();
    }
  }

  static String? _clean(String? v) {
    final String t = v?.trim() ?? '';
    return t.isEmpty ? null : t;
  }
}

final Provider<AssetRegisterRepository> assetRegisterRepositoryProvider =
    Provider<AssetRegisterRepository>(
      (Ref ref) => AssetRegisterRepository(ref.watch(dioProvider)),
    );
