import 'package:flutter_riverpod/flutter_riverpod.dart';

/// Jumlah notifikasi belum dibaca untuk badge bottom-nav.
///
/// Placeholder Task 7: selalu 0 sampai feed notifikasi dibangun (Task 11 plan
/// M0) dan provider ini disambungkan ke datanya. Shell hanya membaca angka —
/// kontraknya tidak berubah saat sumber nyata masuk.
final Provider<int> unreadNotificationCountProvider = Provider<int>(
  (Ref ref) => 0,
);
