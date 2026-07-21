import 'package:dio/dio.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../../core/api/dio_provider.dart';
import '../../../core/api/error_mapper.dart';

/// Opsi pegawai untuk picker custodian check-out (id + nama).
class EmployeeOption {
  const EmployeeOption(this.id, this.name);

  final String id;
  final String name;
}

/// Penugasan aktif satu aset (untuk check-in): id penugasan + nama pemegang.
class ActiveAssignment {
  const ActiveAssignment({required this.id, this.holderName});

  final String id;
  final String? holderName;
}

/// Kategori masalah untuk Lapor Kerusakan (id + nama).
class ProblemCategory {
  const ProblemCategory(this.id, this.name);

  final String id;
  final String name;
}

/// Aksi tulis FR-M7 dari Detail Aset. Peminjaman (Staf) via
/// `POST /assignments/borrow` (pengajuan approval); Check-out & Check-in
/// (Manager) langsung via `POST /assignments` dan `POST /assignments/:id/checkin`.
class AssetActionRepository {
  AssetActionRepository(this._dio);

  final Dio _dio;

  /// Mengajukan peminjaman aset (Staf). [dueDate] "2006-01-02".
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

  /// Check-out langsung (Manager): menugaskan aset ke pegawai. [checkoutDate] &
  /// [dueDate] "2006-01-02". Aset menjadi `assigned`.
  Future<void> checkout({
    required String assetId,
    required String employeeId,
    required String checkoutDate,
    String? dueDate,
    String? conditionOut,
  }) async {
    final String? cond = conditionOut?.trim();
    try {
      await _dio.post<Map<String, dynamic>>(
        '/assignments',
        data: <String, dynamic>{
          'asset_id': assetId,
          'employee_id': employeeId,
          'checkout_date': checkoutDate,
          'due_date': ?dueDate,
          if (cond != null && cond.isNotEmpty) 'condition_out': cond,
        },
      );
    } on DioException catch (err) {
      throw err.toAppFailure();
    }
  }

  /// Check-in (Manager): mengembalikan aset. [needsMaintenance] true menandai
  /// aset perlu servis (jadi `under_maintenance`), selain itu `available`.
  Future<void> checkin({
    required String assignmentId,
    String? conditionIn,
    required bool needsMaintenance,
  }) async {
    final String? cond = conditionIn?.trim();
    try {
      await _dio.post<Map<String, dynamic>>(
        '/assignments/${Uri.encodeComponent(assignmentId)}/checkin',
        data: <String, dynamic>{
          if (cond != null && cond.isNotEmpty) 'condition_in': cond,
          'needs_maintenance': needsMaintenance,
        },
      );
    } on DioException catch (err) {
      throw err.toAppFailure();
    }
  }

  /// Penugasan aktif aset untuk check-in: `GET /assets/:id/assignments` lalu
  /// pilih baris `status == 'active'` (belum di-check-in). null bila tak ada.
  Future<ActiveAssignment?> activeAssignment(String assetId) async {
    try {
      final Response<Map<String, dynamic>> response = await _dio
          .get<Map<String, dynamic>>(
            '/assets/${Uri.encodeComponent(assetId)}/assignments',
          );
      final Object? data = response.data?['data'];
      if (data is! List) {
        return null;
      }
      for (final Object? row in data) {
        if (row is Map<String, dynamic> && row['status'] == 'active') {
          final Object? id = row['id'];
          if (id is String && id.isNotEmpty) {
            final Object? holder = row['employee_name'];
            return ActiveAssignment(
              id: id,
              holderName: holder is String ? holder : null,
            );
          }
        }
      }
      return null;
    } on DioException catch (err) {
      throw err.toAppFailure();
    }
  }

  /// Cari pegawai untuk picker custodian (`GET /employees?search=`, di-scope
  /// backend). Mengembalikan id + nama; entri tanpa keduanya dilewati.
  Future<List<EmployeeOption>> searchEmployees(String query) async {
    final String term = query.trim();
    try {
      final Response<Map<String, dynamic>> response = await _dio
          .get<Map<String, dynamic>>(
            '/employees',
            queryParameters: <String, dynamic>{
              if (term.isNotEmpty) 'search': term,
              'limit': 20,
              'offset': 0,
            },
          );
      final Object? data = response.data?['data'];
      if (data is! List) {
        return const <EmployeeOption>[];
      }
      return data
          .whereType<Map<String, dynamic>>()
          .map((Map<String, dynamic> m) {
            final Object? id = m['id'];
            final Object? name = m['name'];
            if (id is String && name is String && name.isNotEmpty) {
              return EmployeeOption(id, name);
            }
            return null;
          })
          .whereType<EmployeeOption>()
          .toList();
    } on DioException catch (err) {
      throw err.toAppFailure();
    }
  }

  /// Daftar kategori masalah untuk Lapor Kerusakan (`GET /problem-categories`).
  Future<List<ProblemCategory>> problemCategories() async {
    try {
      final Response<Map<String, dynamic>> response = await _dio
          .get<Map<String, dynamic>>(
            '/problem-categories',
            queryParameters: <String, dynamic>{'limit': 100, 'offset': 0},
          );
      final Object? data = response.data?['data'];
      if (data is! List) {
        return const <ProblemCategory>[];
      }
      return data
          .whereType<Map<String, dynamic>>()
          .map((Map<String, dynamic> m) {
            final Object? id = m['id'];
            final Object? name = m['name'];
            if (id is String && name is String && name.isNotEmpty) {
              return ProblemCategory(id, name);
            }
            return null;
          })
          .whereType<ProblemCategory>()
          .toList();
    } on DioException catch (err) {
      throw err.toAppFailure();
    }
  }

  /// Lapor kerusakan aset (`POST /maintenance/reports`, multipart). Membuat
  /// pengajuan `maintenance` via approval. [problemCategoryId] wajib; [photoBytes]
  /// (field `photo`) opsional.
  Future<void> reportDamage({
    required String assetId,
    required String problemCategoryId,
    String? description,
    List<int>? photoBytes,
    String? photoFilename,
  }) async {
    final String? desc = description?.trim();
    try {
      await _dio.post<Map<String, dynamic>>(
        '/maintenance/reports',
        data: FormData.fromMap(<String, dynamic>{
          'asset_id': assetId,
          'problem_category_id': problemCategoryId,
          if (desc != null && desc.isNotEmpty) 'description': desc,
          if (photoBytes != null)
            'photo': MultipartFile.fromBytes(
              photoBytes,
              filename: photoFilename ?? 'photo.jpg',
            ),
        }),
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
