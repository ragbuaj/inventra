import 'package:dio/dio.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../../core/api/dio_provider.dart';
import '../../../core/api/error_mapper.dart';
import 'notification_dto.dart';
import 'notification_list_dto.dart';

/// Repository modul notifikasi (kontrak backend/api/openapi.yaml):
/// `GET /notifications` (filter `read` opsional + limit/offset),
/// `GET /notifications/unread-count` (badge), `POST /notifications/{id}/read`
/// (idempoten; milik pengguna lain berarti 404, bukan 403), dan
/// `POST /notifications/read-all` (idempoten, 204).
///
/// Semua endpoint hanya menyentuh notifikasi milik pemanggil — tidak ada
/// permission key khusus.
class NotificationsRepository {
  NotificationsRepository(this._dio);

  final Dio _dio;

  /// Halaman feed, terbaru lebih dulu. [read] null berarti seluruh feed;
  /// `false` hanya belum dibaca, `true` hanya yang sudah.
  Future<NotificationListDto> list({
    bool? read,
    int offset = 0,
    int limit = 20,
  }) async {
    try {
      final Response<Map<String, dynamic>> response = await _dio
          .get<Map<String, dynamic>>(
            '/notifications',
            queryParameters: <String, dynamic>{
              'read': ?read,
              'limit': limit,
              'offset': offset,
            },
          );
      return NotificationListDto.fromJson(response.data!);
    } on DioException catch (err) {
      throw err.toAppFailure();
    }
  }

  /// Jumlah belum dibaca untuk badge (`GET /notifications/unread-count`).
  /// Melempar [AppFailure] — pemanggil badge memperlakukannya non-fatal.
  Future<int> unreadCount() async {
    try {
      final Response<Map<String, dynamic>> response = await _dio
          .get<Map<String, dynamic>>('/notifications/unread-count');
      return (response.data!['count'] as num).toInt();
    } on DioException catch (err) {
      throw err.toAppFailure();
    }
  }

  /// Menandai satu notifikasi dibaca; mengembalikan notifikasi terbaru
  /// (dengan `read_at` terisi).
  Future<NotificationDto> markRead(String id) async {
    try {
      final Response<Map<String, dynamic>> response = await _dio
          .post<Map<String, dynamic>>(
            '/notifications/${Uri.encodeComponent(id)}/read',
          );
      return NotificationDto.fromJson(response.data!);
    } on DioException catch (err) {
      throw err.toAppFailure();
    }
  }

  /// Menandai seluruh notifikasi dibaca (204, idempoten).
  Future<void> markAllRead() async {
    try {
      await _dio.post<void>('/notifications/read-all');
    } on DioException catch (err) {
      throw err.toAppFailure();
    }
  }
}

final Provider<NotificationsRepository> notificationsRepositoryProvider =
    Provider<NotificationsRepository>(
      (Ref ref) => NotificationsRepository(ref.watch(dioProvider)),
    );
