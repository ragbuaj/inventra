import 'package:dio/dio.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../../core/api/dio_provider.dart';
import '../../../core/api/error_mapper.dart';

/// Aksi tulis FR-M7 dari Detail Aset. Peminjaman (Staf) memakai
/// `POST /assignments/borrow` — membuat pengajuan `assignment` via maker-checker
/// (peminjam di-resolve server dari JWT, bukan dari body). Check-out/Check-in
/// (Manager) menyusul di fase berikutnya.
class AssetActionRepository {
  AssetActionRepository(this._dio);

  final Dio _dio;

  /// Mengajukan peminjaman aset. [dueDate] "2006-01-02", [notes] opsional.
  /// Melempar AppFailure via toAppFailure() saat DioException.
  Future<void> borrow({
    required String assetId,
    String? dueDate,
    String? notes,
  }) async {
    final String? trimmedNotes = notes?.trim();
    try {
      await _dio.post<Map<String, dynamic>>(
        '/assignments/borrow',
        data: <String, dynamic>{
          'asset_id': assetId,
          'due_date': ?dueDate,
          if (trimmedNotes != null && trimmedNotes.isNotEmpty)
            'notes': trimmedNotes,
        },
      );
    } on DioException catch (err) {
      throw err.toAppFailure();
    }
  }
}

final Provider<AssetActionRepository> assetActionRepositoryProvider =
    Provider<AssetActionRepository>(
      (Ref ref) => AssetActionRepository(ref.watch(dioProvider)),
    );
