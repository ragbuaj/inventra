import 'package:dio/dio.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../../core/api/dio_provider.dart';
import '../../../core/api/error_mapper.dart';
import 'asset_dto.dart';

/// Hasil lookup aset by-tag: DTO + himpunan field yang TIDAK dikirim backend.
///
/// [maskedFields] berisi kunci skema `Asset` yang absen dari JSON respons —
/// dihapus oleh field-permission masking per peran. UI merender field ini
/// sebagai em-dash dengan penanda "dibatasi"; field yang dikirim bernilai null
/// (mis. aset tanpa pemegang) TIDAK masuk himpunan ini.
class AssetDetailData {
  const AssetDetailData({required this.asset, required this.maskedFields});

  final AssetDto asset;
  final Set<String> maskedFields;

  /// true bila [field] (kunci snake_case openapi) tidak dikirim backend.
  bool isMasked(String field) => maskedFields.contains(field);
}

/// Repository `GET /assets/by-tag/{tag}` (kontrak backend/api/openapi.yaml).
///
/// 404 berarti tag tidak terdaftar ATAU aset di luar scope kantor pemanggil
/// (backend sengaja tidak membedakan untuk mencegah enumerasi tag); 403 hanya
/// muncul bila peran tidak punya permission `asset.view` sama sekali.
class AssetDetailRepository {
  AssetDetailRepository(this._dio);

  final Dio _dio;

  Future<AssetDetailData> getByTag(String tag) async {
    try {
      final Response<Map<String, dynamic>> response = await _dio
          .get<Map<String, dynamic>>(
            '/assets/by-tag/${Uri.encodeComponent(tag)}',
          );
      final Map<String, dynamic> json = response.data!;
      return AssetDetailData(
        asset: AssetDto.fromJson(json),
        maskedFields: assetSchemaKeys
            .where((String key) => !json.containsKey(key))
            .toSet(),
      );
    } on DioException catch (err) {
      throw err.toAppFailure();
    }
  }
}

final Provider<AssetDetailRepository> assetDetailRepositoryProvider =
    Provider<AssetDetailRepository>(
      (Ref ref) => AssetDetailRepository(ref.watch(dioProvider)),
    );
