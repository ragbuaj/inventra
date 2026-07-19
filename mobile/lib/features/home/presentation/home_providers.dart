import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../../core/api/app_failure.dart';
import '../../../core/auth/auth_controller.dart';
import '../../../core/auth/auth_session.dart';
import '../../../core/masterdata/reference_lookup_repository.dart';
import '../../stock_opname/data/stock_opname_repository.dart';
import '../../stock_opname/data/stock_opname_session_dto.dart';
import '../../stock_opname/presentation/opname_sessions_provider.dart';

/// Sesi opname berjalan yang ditampilkan kartu "Sesi Opname Aktif" Beranda:
/// sesi non-closed PERTAMA. Beranda hanya butuh satu sesi, jadi detail
/// (KPI progress) di-fetch HANYA untuk sesi itu — bukan seluruh daftar —
/// supaya tidak memicu N+1 request per sesi dan rawan 429 (rate limiter
/// per-user ADR-0017). null berarti tidak ada sesi berjalan.
///
/// Detail satu sesi non-fatal: bila `session(id)` gagal, kartu memakai data
/// daftar tanpa KPI (baris progress dilewati) alih-alih menggagalkan kartu.
/// Kegagalan mengambil DAFTAR tetap diteruskan sebagai error kartu (kartu lain
/// tidak terpengaruh — semua panggilan ringkasan independen).
final FutureProvider<StockOpnameSessionDto?> homeActiveOpnameSessionProvider =
    FutureProvider.autoDispose<StockOpnameSessionDto?>((Ref ref) async {
      final StockOpnameRepository repository = ref.watch(
        stockOpnameRepositoryProvider,
      );
      final List<StockOpnameSessionDto> sessions = (await repository.sessions(
        limit: opnameSessionsFetchLimit,
      )).data;
      StockOpnameSessionDto? running;
      for (final StockOpnameSessionDto session in sessions) {
        if (session.status != 'closed') {
          running = session;
          break;
        }
      }
      if (running == null) {
        return null;
      }
      try {
        return await repository.session(running.id);
      } on AppFailure {
        return running;
      }
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
