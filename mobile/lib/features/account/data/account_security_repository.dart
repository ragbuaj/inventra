import 'package:dio/dio.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../../core/api/dio_provider.dart';
import '../../../core/api/error_mapper.dart';

/// Keamanan akun berbasis link email (FR-M6.3). Klien hanya MEMULAI alur;
/// penetapan/konfirmasi diselesaikan di halaman web via link.
/// - Ganti password: verifikasi password lama lalu kirim link reset
///   (`POST /auth/password/change-request`). Password lama salah -> 400
///   (`ValidationFailure`), BUKAN 401 (agar interceptor tidak auto-logout).
/// - Ganti email: verifikasi password lama + email baru lalu kirim link
///   verifikasi (`POST /auth/email/change-request`). Email sudah dipakai -> 409.
class AccountSecurityRepository {
  AccountSecurityRepository(this._dio);

  final Dio _dio;

  Future<void> requestPasswordChange(String currentPassword) async {
    try {
      await _dio.post<Map<String, dynamic>>(
        '/auth/password/change-request',
        data: <String, dynamic>{'current_password': currentPassword},
      );
    } on DioException catch (err) {
      throw err.toAppFailure();
    }
  }

  Future<void> requestEmailChange({
    required String newEmail,
    required String currentPassword,
  }) async {
    try {
      await _dio.post<Map<String, dynamic>>(
        '/auth/email/change-request',
        data: <String, dynamic>{
          'new_email': newEmail.trim(),
          'current_password': currentPassword,
        },
      );
    } on DioException catch (err) {
      throw err.toAppFailure();
    }
  }
}

final Provider<AccountSecurityRepository> accountSecurityRepositoryProvider =
    Provider<AccountSecurityRepository>(
      (Ref ref) => AccountSecurityRepository(ref.watch(dioProvider)),
    );
