import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../../core/auth/auth_controller.dart';
import '../data/notifications_repository.dart';

/// Jumlah notifikasi belum dibaca (`GET /notifications/unread-count`) —
/// sumber badge tab Notif di shell dan lonceng header Beranda.
///
/// Auto-retry dimatikan: nilai diperbarui lewat invalidate setelah tandai
/// dibaca / tandai semua / refresh feed.
///
/// Watch [authControllerProvider] supaya badge di-fetch ulang saat sesi
/// berubah (logout ATAU sesi mati lalu user lain login di perangkat yang
/// sama) — mencegah angka milik user sebelumnya bocor ke user berikutnya.
final FutureProvider<int> notificationsUnreadCountProvider =
    FutureProvider<int>((Ref ref) {
      ref.watch(authControllerProvider);
      return ref.watch(notificationsRepositoryProvider).unreadCount();
    }, retry: (int retryCount, Object error) => null);

/// Nilai badge siap pakai untuk shell/beranda: 0 selama loading ATAU saat
/// gagal (offline dsb.) — badge tidak boleh menggagalkan shell (panggilan
/// suplementer selalu non-fatal). Kontrak provider ini tidak berubah sejak
/// placeholder Task 7 — shell hanya membaca angka.
final Provider<int> unreadNotificationCountProvider = Provider<int>(
  (Ref ref) => ref.watch(notificationsUnreadCountProvider).value ?? 0,
);
