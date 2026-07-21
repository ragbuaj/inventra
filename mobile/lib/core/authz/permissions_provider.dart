import 'package:dio/dio.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../api/dio_provider.dart';
import '../api/error_mapper.dart';

/// Mengambil kunci permission peran pemanggil (`GET /auth/permissions` ->
/// `{permissions: [..keys]}`). Dipakai untuk menampilkan/menyembunyikan aksi di
/// klien — otorisasi sesungguhnya tetap ditegakkan server pada tiap endpoint.
class PermissionsRepository {
  PermissionsRepository(this._dio);

  final Dio _dio;

  Future<Set<String>> list() async {
    try {
      final Response<Map<String, dynamic>> response = await _dio
          .get<Map<String, dynamic>>('/auth/permissions');
      final Object? raw = response.data?['permissions'];
      if (raw is! List) {
        return const <String>{};
      }
      return raw.whereType<String>().toSet();
    } on DioException catch (err) {
      throw err.toAppFailure();
    }
  }
}

final Provider<PermissionsRepository> permissionsRepositoryProvider =
    Provider<PermissionsRepository>(
      (Ref ref) => PermissionsRepository(ref.watch(dioProvider)),
    );

/// Himpunan permission pemanggil. autoDispose: diambil ulang saat sebuah layar
/// yang butuh izin dibuka — tidak bertahan lintas sesi (hindari izin user lama
/// bocor ke user berikutnya di perangkat yang sama). Gagal ambil = sisi
/// pemanggil memperlakukannya sebagai "tanpa aksi" (default aman).
final permissionsProvider = FutureProvider.autoDispose<Set<String>>(
  (Ref ref) => ref.watch(permissionsRepositoryProvider).list(),
);
