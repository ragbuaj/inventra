import 'package:inventra_mobile/core/api/app_failure.dart';
import 'package:inventra_mobile/features/notifications/data/notification_dto.dart';
import 'package:inventra_mobile/features/notifications/data/notification_list_dto.dart';
import 'package:inventra_mobile/features/notifications/data/notifications_repository.dart';

/// [NotificationsRepository] palsu berbasis data in-memory untuk widget/
/// golden/router test — tanpa Dio/HTTP. markRead/markAllRead diterapkan ke
/// [feed] sehingga refresh melihat nilai baru; kegagalan bisa diskrip.
class FakeNotificationsRepository implements NotificationsRepository {
  FakeNotificationsRepository({
    List<NotificationDto>? feed,
    this.failMarkRead = false,
    this.failMarkAllRead = false,
  }) : feed = List<NotificationDto>.of(feed ?? <NotificationDto>[]);

  final List<NotificationDto> feed;
  final bool failMarkRead;
  final bool failMarkAllRead;

  final List<String> markReadCalls = <String>[];
  int markAllReadCalls = 0;

  /// `read_at` yang dituliskan markRead/markAllRead palsu (deterministik).
  static final DateTime readAtStamp = DateTime.utc(2026, 7, 19, 3);

  @override
  Future<NotificationListDto> list({
    bool? read,
    int offset = 0,
    int limit = 20,
  }) async {
    final List<NotificationDto> filtered = feed
        .where(
          (NotificationDto item) =>
              read == null || (item.readAt != null) == read,
        )
        .toList(growable: false);
    final List<NotificationDto> page = filtered
        .skip(offset)
        .take(limit)
        .toList(growable: false);
    return NotificationListDto(
      data: page,
      total: filtered.length,
      limit: limit,
      offset: offset,
    );
  }

  @override
  Future<int> unreadCount() async =>
      feed.where((NotificationDto item) => item.readAt == null).length;

  @override
  Future<NotificationDto> markRead(String id) async {
    markReadCalls.add(id);
    if (failMarkRead) {
      throw const NetworkFailure();
    }
    final int index = feed.indexWhere((NotificationDto item) => item.id == id);
    if (index < 0) {
      throw const NotFoundFailure();
    }
    final NotificationDto updated = feed[index].readAt == null
        ? feed[index].copyWith(readAt: readAtStamp)
        : feed[index];
    feed[index] = updated;
    return updated;
  }

  @override
  Future<void> markAllRead() async {
    markAllReadCalls += 1;
    if (failMarkAllRead) {
      throw const NetworkFailure();
    }
    for (int i = 0; i < feed.length; i++) {
      if (feed[i].readAt == null) {
        feed[i] = feed[i].copyWith(readAt: readAtStamp);
      }
    }
  }
}
