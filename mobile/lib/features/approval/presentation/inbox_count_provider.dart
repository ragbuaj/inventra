import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../../core/auth/auth_controller.dart';
import '../data/approval_repository.dart';

/// Jumlah pengajuan yang menunggu keputusan pemanggil
/// (`GET /requests/inbox/count`) — sumber badge tab Approval di shell.
///
/// Auto-retry dimatikan: 403 (peran tanpa `request.decide`) tidak akan
/// berubah bila diulang; nilai diperbarui lewat invalidate setelah keputusan
/// atau refresh inbox.
///
/// Watch [authControllerProvider] supaya badge di-fetch ulang saat sesi
/// berubah (logout ATAU sesi mati lalu user lain login di perangkat yang
/// sama) — mencegah angka milik user sebelumnya bocor ke user berikutnya.
final FutureProvider<int> approvalInboxCountProvider = FutureProvider<int>((
  Ref ref,
) {
  ref.watch(authControllerProvider);
  return ref.watch(approvalRepositoryProvider).inboxCount();
}, retry: (int retryCount, Object error) => null);

/// Nilai badge siap pakai untuk shell: 0 selama loading ATAU saat gagal
/// (offline / 403 peran tanpa izin memutus) — badge tidak boleh menggagalkan
/// shell (panggilan suplementer selalu non-fatal).
final Provider<int> approvalPendingBadgeProvider = Provider<int>(
  (Ref ref) => ref.watch(approvalInboxCountProvider).value ?? 0,
);
