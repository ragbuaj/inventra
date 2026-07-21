import 'package:dio/dio.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../../core/api/dio_provider.dart';
import '../../../core/api/error_mapper.dart';
import 'asset_list_dto.dart';

/// Repository katalog aset (kontrak `GET /assets`): daftar aset dalam data
/// scope pemanggil, dengan pencarian (`search` mencocokkan nama/tag/serial di
/// server) dan paginasi limit/offset. Read-only — field sensitif dihapus
/// field-permission masking backend, jadi klien tidak pernah membocorkannya.
class CatalogRepository {
  CatalogRepository(this._dio);

  final Dio _dio;

  /// Satu halaman katalog. [search] yang null/kosong tidak dikirim (semua aset
  /// dalam scope). Melempar AppFailure lewat toAppFailure() saat DioException.
  Future<AssetListDto> list({
    String? search,
    int offset = 0,
    int limit = 20,
  }) async {
    final String? term = search?.trim();
    try {
      final Response<Map<String, dynamic>> response = await _dio
          .get<Map<String, dynamic>>(
            '/assets',
            queryParameters: <String, dynamic>{
              if (term != null && term.isNotEmpty) 'search': term,
              'limit': limit,
              'offset': offset,
            },
          );
      return AssetListDto.fromJson(response.data!);
    } on DioException catch (err) {
      throw err.toAppFailure();
    }
  }
}

final Provider<CatalogRepository> catalogRepositoryProvider =
    Provider<CatalogRepository>(
      (Ref ref) => CatalogRepository(ref.watch(dioProvider)),
    );
