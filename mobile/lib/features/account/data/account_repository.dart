import 'dart:typed_data';

import 'package:dio/dio.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../../core/api/app_failure.dart';
import '../../../core/api/dio_provider.dart';
import '../../../core/api/error_mapper.dart';
import 'profile_dto.dart';
import 'session_dto.dart';

/// Repository layar Profil (kontrak backend/api/openapi.yaml):
/// `GET /auth/sessions` (daftar sesi device aktif, sesi ini ber-flag
/// `current`), `DELETE /auth/sessions/{id}` (cabut satu sesi — id milik
/// pengguna lain berarti 404, bukan 403), `POST /auth/sessions/revoke-others`
/// (cabut semua kecuali sesi ini), dan `GET /auth/avatar` (bytes foto profil;
/// endpoint ber-auth sehingga tidak bisa dipakai sebagai URL gambar polos).
///
/// Data profil sendiri datang dari sesi auth (`GET /auth/me` — UserDto core);
/// M0 hanya MENAMPILKAN avatar, upload tetap dari aplikasi web.
class AccountRepository {
  AccountRepository(this._dio);

  final Dio _dio;

  /// Profil lengkap pemanggil (`GET /auth/profile`): metadata akun + detail
  /// pegawai tertaut. Melempar AppFailure via toAppFailure() saat DioException.
  Future<ProfileDto> getProfile() async {
    try {
      final Response<Map<String, dynamic>> response = await _dio
          .get<Map<String, dynamic>>('/auth/profile');
      return ProfileDto.fromJson(response.data!);
    } on DioException catch (err) {
      throw err.toAppFailure();
    }
  }

  /// Menyunting data diri sendiri (`PUT /auth/profile` — hanya `name` wajib +
  /// `phone` opsional yang boleh diedit). Mengembalikan profil terbaru.
  Future<ProfileDto> updateProfile({
    required String name,
    String? phone,
  }) async {
    try {
      final Response<Map<String, dynamic>> response = await _dio
          .put<Map<String, dynamic>>(
            '/auth/profile',
            data: <String, dynamic>{
              'name': name.trim(),
              'phone': phone?.trim() ?? '',
            },
          );
      return ProfileDto.fromJson(response.data!);
    } on DioException catch (err) {
      throw err.toAppFailure();
    }
  }

  /// Seluruh sesi aktif milik pemanggil (`GET /auth/sessions`).
  Future<List<SessionDto>> sessions() async {
    try {
      final Response<Map<String, dynamic>> response = await _dio
          .get<Map<String, dynamic>>('/auth/sessions');
      final List<dynamic> data =
          (response.data!['data'] as List<dynamic>?) ?? <dynamic>[];
      return data
          .map(
            (dynamic item) => SessionDto.fromJson(item as Map<String, dynamic>),
          )
          .toList(growable: false);
    } on DioException catch (err) {
      throw err.toAppFailure();
    }
  }

  /// Mencabut satu sesi (`DELETE /auth/sessions/{id}`). Perangkat yang dicabut
  /// gagal pada request berikutnya (refresh token + record sesinya hilang).
  Future<void> revokeSession(String id) async {
    try {
      await _dio.delete<Map<String, dynamic>>(
        '/auth/sessions/${Uri.encodeComponent(id)}',
      );
    } on DioException catch (err) {
      throw err.toAppFailure();
    }
  }

  /// Mencabut semua sesi lain (`POST /auth/sessions/revoke-others`);
  /// mengembalikan jumlah sesi yang dicabut.
  Future<int> revokeOtherSessions() async {
    try {
      final Response<Map<String, dynamic>> response = await _dio
          .post<Map<String, dynamic>>('/auth/sessions/revoke-others');
      return ((response.data?['revoked'] as num?) ?? 0).toInt();
    } on DioException catch (err) {
      throw err.toAppFailure();
    }
  }

  /// Mengunggah foto profil (`POST /auth/avatar`, multipart field `file`, JPG/PNG;
  /// server memotong persegi + re-encode JPEG, buang EXIF). Mengganti foto lama.
  Future<void> uploadAvatar(List<int> bytes, {required String filename}) async {
    try {
      await _dio.post<Map<String, dynamic>>(
        '/auth/avatar',
        data: FormData.fromMap(<String, dynamic>{
          'file': MultipartFile.fromBytes(bytes, filename: filename),
        }),
      );
    } on DioException catch (err) {
      throw err.toAppFailure();
    }
  }

  /// Menghapus foto profil (`DELETE /auth/avatar`).
  Future<void> deleteAvatar() async {
    try {
      await _dio.delete<Map<String, dynamic>>('/auth/avatar');
    } on DioException catch (err) {
      throw err.toAppFailure();
    }
  }

  /// Bytes foto profil (`GET /auth/avatar`, image/jpeg). Null saat belum ada
  /// avatar (404) — bukan error; kegagalan lain tetap [AppFailure] (pemanggil
  /// memperlakukannya non-fatal, jatuh ke inisial).
  Future<Uint8List?> avatar() async {
    try {
      final Response<List<int>> response = await _dio.get<List<int>>(
        '/auth/avatar',
        options: Options(responseType: ResponseType.bytes),
      );
      final List<int>? data = response.data;
      if (data == null || data.isEmpty) {
        return null;
      }
      return Uint8List.fromList(data);
    } on DioException catch (err) {
      final AppFailure failure = err.toAppFailure();
      if (failure is NotFoundFailure) {
        return null;
      }
      throw failure;
    }
  }
}

final Provider<AccountRepository> accountRepositoryProvider =
    Provider<AccountRepository>(
      (Ref ref) => AccountRepository(ref.watch(dioProvider)),
    );
