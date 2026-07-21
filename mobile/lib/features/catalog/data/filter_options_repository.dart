import 'package:dio/dio.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../../core/api/dio_provider.dart';
import '../../../core/api/error_mapper.dart';

/// Opsi filter (id + nama) untuk picker Kategori/Kantor katalog.
class FilterOption {
  const FilterOption(this.id, this.name);

  final String id;
  final String name;
}

/// Mengambil daftar opsi filter dari endpoint list master data yang sudah ada
/// (`GET /categories`, `GET /offices` — auth-only, di-scope backend). Hanya
/// id + name yang dipakai; entri tanpa keduanya dilewati.
class FilterOptionsRepository {
  FilterOptionsRepository(this._dio);

  final Dio _dio;

  Future<List<FilterOption>> categories() => _list('/categories');

  Future<List<FilterOption>> offices() => _list('/offices');

  Future<List<FilterOption>> _list(String path) async {
    try {
      final Response<Map<String, dynamic>> response = await _dio
          .get<Map<String, dynamic>>(
            path,
            queryParameters: <String, dynamic>{'limit': 100, 'offset': 0},
          );
      final Object? data = response.data?['data'];
      if (data is! List) {
        return const <FilterOption>[];
      }
      return data
          .whereType<Map<String, dynamic>>()
          .map((Map<String, dynamic> m) {
            final Object? id = m['id'];
            final Object? name = m['name'];
            if (id is String && name is String && name.isNotEmpty) {
              return FilterOption(id, name);
            }
            return null;
          })
          .whereType<FilterOption>()
          .toList();
    } on DioException catch (err) {
      throw err.toAppFailure();
    }
  }
}

final Provider<FilterOptionsRepository> filterOptionsRepositoryProvider =
    Provider<FilterOptionsRepository>(
      (Ref ref) => FilterOptionsRepository(ref.watch(dioProvider)),
    );

/// Opsi kategori untuk picker filter (di-cache sesi — daftar kecil, dipakai
/// ulang tiap membuka sheet).
final FutureProvider<List<FilterOption>> catalogCategoryOptionsProvider =
    FutureProvider<List<FilterOption>>(
      (Ref ref) => ref.watch(filterOptionsRepositoryProvider).categories(),
    );

/// Opsi kantor untuk picker filter (dalam data scope pemanggil).
final FutureProvider<List<FilterOption>> catalogOfficeOptionsProvider =
    FutureProvider<List<FilterOption>>(
      (Ref ref) => ref.watch(filterOptionsRepositoryProvider).offices(),
    );
