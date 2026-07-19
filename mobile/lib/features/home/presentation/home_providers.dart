import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../../core/auth/auth_controller.dart';
import '../../../core/auth/auth_session.dart';
import '../../../core/masterdata/reference_lookup_repository.dart';
import '../../stock_opname/data/stock_opname_session_dto.dart';
import '../../stock_opname/presentation/opname_sessions_provider.dart';

/// Sesi opname berjalan yang ditampilkan kartu "Sesi Opname Aktif" Beranda:
/// sesi non-closed pertama dari daftar (reuse [opnameSessionsProvider] —
/// termasuk KPI progress dari fetch detailnya). null berarti tidak ada sesi
/// berjalan; error diteruskan supaya kartu merender cabang errornya sendiri
/// (kartu lain tidak terpengaruh — semua panggilan ringkasan independen).
final FutureProvider<StockOpnameSessionDto?> homeActiveOpnameSessionProvider =
    FutureProvider.autoDispose<StockOpnameSessionDto?>((Ref ref) async {
      final List<StockOpnameSessionDto> sessions = await ref.watch(
        opnameSessionsProvider.future,
      );
      for (final StockOpnameSessionDto session in sessions) {
        if (session.status != 'closed') {
          return session;
        }
      }
      return null;
    }, retry: (int retryCount, Object error) => null);

/// Nama kantor pengguna untuk subjudul header Beranda, di-resolve non-fatal
/// via [ReferenceLookupRepository] (`GET /offices/{id}` — lookup gagal berarti
/// null dan header merender tanpa nama kantor, tidak pernah memblokir layar).
final FutureProvider<String?> homeOfficeNameProvider =
    FutureProvider.autoDispose<String?>((Ref ref) async {
      final AuthSession? session = ref.watch(authControllerProvider).value;
      if (session is! Authenticated) {
        return null;
      }
      final String? officeId = session.user.officeId;
      if (officeId == null || officeId.isEmpty) {
        return null;
      }
      return ref.watch(referenceLookupRepositoryProvider).officeName(officeId);
    }, retry: (int retryCount, Object error) => null);
