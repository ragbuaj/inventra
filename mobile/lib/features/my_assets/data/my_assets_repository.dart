import 'package:dio/dio.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../../core/api/dio_provider.dart';
import '../../../core/api/error_mapper.dart';

/// Satu penugasan aktif yang dipegang pengguna (subset `Assignment` enriched
/// dari `GET /assignments/mine`). `asset_name`/`asset_tag` selalu dikirim
/// backend (FK aset NOT NULL); `due_date` opsional.
class MyAssignmentDto {
  const MyAssignmentDto({
    required this.assetName,
    required this.assetTag,
    this.status,
    this.checkoutDate,
    this.dueDate,
  });

  final String assetName;
  final String assetTag;
  final String? status;
  final String? checkoutDate; // ISO timestamp
  final String? dueDate; // "2006-01-02"

  factory MyAssignmentDto.fromJson(Map<String, dynamic> json) {
    return MyAssignmentDto(
      assetName: (json['asset_name'] as String?) ?? '',
      assetTag: (json['asset_tag'] as String?) ?? '',
      status: json['status'] as String?,
      checkoutDate: json['checkout_date'] as String?,
      dueDate: json['due_date'] as String?,
    );
  }
}

/// Repository "Aset Saya" (`GET /assignments/mine`): aset yang sedang dipegang
/// pengguna. Employee di-resolve server dari JWT (bukan dari request), jadi
/// respons hanya berisi baris milik pemanggil. Endpoint digate `request.create`.
class MyAssetsRepository {
  MyAssetsRepository(this._dio);

  final Dio _dio;

  /// Penugasan aktif (`status=active`) milik pengguna. Respons `{data: [...]}`
  /// (tanpa paginasi; server membatasi 100). Melempar AppFailure via
  /// toAppFailure() saat DioException.
  Future<List<MyAssignmentDto>> list() async {
    try {
      final Response<Map<String, dynamic>> response = await _dio
          .get<Map<String, dynamic>>(
            '/assignments/mine',
            queryParameters: <String, dynamic>{'status': 'active'},
          );
      final Object? data = response.data?['data'];
      if (data is! List) {
        return const <MyAssignmentDto>[];
      }
      return data
          .whereType<Map<String, dynamic>>()
          .map(MyAssignmentDto.fromJson)
          .toList();
    } on DioException catch (err) {
      throw err.toAppFailure();
    }
  }
}

final Provider<MyAssetsRepository> myAssetsRepositoryProvider =
    Provider<MyAssetsRepository>(
      (Ref ref) => MyAssetsRepository(ref.watch(dioProvider)),
    );

/// Daftar aset yang dipegang pengguna. autoDispose: dimuat saat layar dibuka;
/// pull-to-refresh lewat `ref.invalidate`. Auto-retry dimatikan (tombol retry).
final myAssetsProvider = FutureProvider.autoDispose<List<MyAssignmentDto>>(
  (Ref ref) => ref.watch(myAssetsRepositoryProvider).list(),
  retry: (int retryCount, Object error) => null,
);
