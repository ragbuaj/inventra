import 'package:flutter/foundation.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../../core/api/app_failure.dart';
import '../../../core/utils/clock.dart';
import '../data/notification_dto.dart';
import '../data/notification_list_dto.dart';
import '../data/notifications_repository.dart';
import 'unread_count_provider.dart';

/// State feed notifikasi: halaman-halaman yang sudah dimuat + status
/// muat-berikutnya untuk infinite scroll (limit/offset kontrak) — pola yang
/// sama dengan inbox approval.
@immutable
class NotificationsState {
  const NotificationsState({
    required this.items,
    required this.total,
    this.isLoadingMore = false,
    this.loadMoreFailed = false,
  });

  final List<NotificationDto> items;

  /// Total baris feed di server (`NotificationList.total`).
  final int total;

  final bool isLoadingMore;
  final bool loadMoreFailed;

  bool get hasMore => items.length < total;

  bool get hasUnread =>
      items.any((NotificationDto item) => item.readAt == null);

  NotificationsState copyWith({
    List<NotificationDto>? items,
    int? total,
    bool? isLoadingMore,
    bool? loadMoreFailed,
  }) {
    return NotificationsState(
      items: items ?? this.items,
      total: total ?? this.total,
      isLoadingMore: isLoadingMore ?? this.isLoadingMore,
      loadMoreFailed: loadMoreFailed ?? this.loadMoreFailed,
    );
  }
}

/// Feed notifikasi. autoDispose: state dibuang saat layar ditutup; refresh
/// lewat `ref.refresh(...future)` (pull-to-refresh). Auto-retry Riverpod
/// dimatikan — pengguna punya tombol "Coba lagi".
final notificationsFeedProvider =
    AsyncNotifierProvider.autoDispose<
      NotificationsController,
      NotificationsState
    >(
      NotificationsController.new,
      retry: (int retryCount, Object error) => null,
    );

class NotificationsController extends AsyncNotifier<NotificationsState> {
  static const int pageSize = 20;

  @override
  Future<NotificationsState> build() async {
    final NotificationListDto page = await ref
        .watch(notificationsRepositoryProvider)
        .list(offset: 0, limit: pageSize);
    return NotificationsState(items: page.data, total: page.total);
  }

  /// Memuat halaman berikutnya (offset = jumlah item termuat). Kegagalan
  /// TIDAK menjatuhkan seluruh feed — hanya menandai
  /// [NotificationsState.loadMoreFailed] supaya baris retry tampil di kaki.
  Future<void> loadMore() async {
    final NotificationsState? current = state.value;
    if (current == null || current.isLoadingMore || !current.hasMore) {
      return;
    }
    state = AsyncData<NotificationsState>(
      current.copyWith(isLoadingMore: true, loadMoreFailed: false),
    );
    try {
      final NotificationListDto page = await ref
          .read(notificationsRepositoryProvider)
          .list(offset: current.items.length, limit: pageSize);
      state = AsyncData<NotificationsState>(
        current.copyWith(
          items: List<NotificationDto>.unmodifiable(<NotificationDto>[
            ...current.items,
            ...page.data,
          ]),
          total: page.total,
          isLoadingMore: false,
        ),
      );
    } on Object {
      state = AsyncData<NotificationsState>(
        current.copyWith(isLoadingMore: false, loadMoreFailed: true),
      );
    }
  }

  /// Menandai satu notifikasi dibaca: optimistis di UI (penanda unread hilang
  /// seketika saat di-tap), lalu POST ke server. Kegagalan mengembalikan item
  /// ke belum dibaca — non-fatal, tanpa menjatuhkan feed.
  Future<void> markRead(String id) async {
    final NotificationDto? original = _itemById(id);
    if (original == null || original.readAt != null) {
      return;
    }
    _replaceById(id, original.copyWith(readAt: ref.read(clockProvider)()));
    try {
      final NotificationDto updated = await ref
          .read(notificationsRepositoryProvider)
          .markRead(id);
      _replaceById(id, updated);
      ref.invalidate(notificationsUnreadCountProvider);
    } on AppFailure {
      _replaceById(id, original);
    }
  }

  /// Menandai semua dibaca (`POST /notifications/read-all`). Mengembalikan
  /// false bila server menolak — layar menampilkan pemberitahuan, feed utuh.
  Future<bool> markAllRead() async {
    try {
      await ref.read(notificationsRepositoryProvider).markAllRead();
    } on AppFailure {
      return false;
    }
    final NotificationsState? current = state.value;
    if (current != null) {
      final DateTime now = ref.read(clockProvider)();
      state = AsyncData<NotificationsState>(
        current.copyWith(
          items: List<NotificationDto>.unmodifiable(
            current.items.map(
              (NotificationDto item) =>
                  item.readAt == null ? item.copyWith(readAt: now) : item,
            ),
          ),
        ),
      );
    }
    ref.invalidate(notificationsUnreadCountProvider);
    return true;
  }

  NotificationDto? _itemById(String id) {
    final NotificationsState? current = state.value;
    if (current == null) {
      return null;
    }
    for (final NotificationDto item in current.items) {
      if (item.id == id) {
        return item;
      }
    }
    return null;
  }

  /// Mengganti item ber-id [id] pada state TERKINI (bukan snapshot lama —
  /// state bisa berubah selama menunggu server, mis. loadMore menambah item).
  void _replaceById(String id, NotificationDto replacement) {
    final NotificationsState? current = state.value;
    if (current == null) {
      return;
    }
    state = AsyncData<NotificationsState>(
      current.copyWith(
        items: List<NotificationDto>.unmodifiable(
          current.items.map(
            (NotificationDto item) => item.id == id ? replacement : item,
          ),
        ),
      ),
    );
  }
}
