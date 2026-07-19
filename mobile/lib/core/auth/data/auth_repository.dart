import 'package:dio/dio.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../api/dio_provider.dart';
import '../../api/error_mapper.dart';
import 'token_response_dto.dart';
import 'user_dto.dart';

/// Repository `/auth/*` (kontrak backend/api/openapi.yaml). Mengembalikan DTO
/// atau melempar `AppFailure` — tidak tahu UI (ARCHITECTURE bagian 1).
class AuthRepository {
  AuthRepository(this._dio);

  final Dio _dio;

  /// `POST /auth/login`. Header `X-Client-Type: mobile` (dipasang
  /// interceptor) membuat backend mengirim `refresh_token` di body tanpa
  /// cookie. 401 kredensial salah, 403 nonaktif, 429 rate limit.
  Future<TokenResponseDto> login({
    required String email,
    required String password,
  }) async {
    try {
      final Response<Map<String, dynamic>> response = await _dio
          .post<Map<String, dynamic>>(
            '/auth/login',
            data: <String, String>{'email': email, 'password': password},
          );
      return TokenResponseDto.fromJson(response.data!);
    } on DioException catch (err) {
      throw err.toAppFailure();
    }
  }

  /// `POST /auth/refresh` dengan refresh token di body; respons memuat
  /// refresh token BARU (rotasi) yang wajib disimpan menggantikan yang lama.
  Future<TokenResponseDto> refresh({required String refreshToken}) async {
    try {
      final Response<Map<String, dynamic>> response = await _dio
          .post<Map<String, dynamic>>(
            '/auth/refresh',
            data: <String, String>{'refresh_token': refreshToken},
          );
      return TokenResponseDto.fromJson(response.data!);
    } on DioException catch (err) {
      throw err.toAppFailure();
    }
  }

  /// `POST /auth/logout` (Bearer) dengan refresh token di body.
  Future<void> logout({required String refreshToken}) async {
    try {
      await _dio.post<Map<String, dynamic>>(
        '/auth/logout',
        data: <String, String>{'refresh_token': refreshToken},
      );
    } on DioException catch (err) {
      throw err.toAppFailure();
    }
  }

  /// `GET /auth/me` (Bearer).
  Future<UserDto> me() async {
    try {
      final Response<Map<String, dynamic>> response = await _dio
          .get<Map<String, dynamic>>('/auth/me');
      return UserDto.fromJson(response.data!);
    } on DioException catch (err) {
      throw err.toAppFailure();
    }
  }
}

final Provider<AuthRepository> authRepositoryProvider =
    Provider<AuthRepository>(
      (Ref ref) => AuthRepository(ref.watch(dioProvider)),
    );
