import 'dart:async';

import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../data/stock_opname_repository.dart';
import '../data/stock_opname_session_dto.dart';
import '../data/stock_opname_session_list_dto.dart';

/// Tab filter daftar sesi (mockup Berjalan | Selesai | Semua). Berjalan
/// mencakup open/counting/reconciling — kontrak hanya menyaring SATU status
/// per query, jadi daftar diambil sekali lalu disaring klien.
enum OpnameSessionTab {
  running,
  closed,
  all;

  bool matches(StockOpnameSessionDto session) => switch (this) {
    OpnameSessionTab.running => session.status != 'closed',
    OpnameSessionTab.closed => session.status == 'closed',
    OpnameSessionTab.all => true,
  };
}

/// Batas ambil daftar sesi — maksimum kontrak (sesi per kantor sedikit;
/// tanpa infinite scroll di M0).
const int opnameSessionsFetchLimit = 100;

/// Batas konkurensi fetch detail KPI per sesi. Detail di-fetch berkelompok
/// (bukan sekaligus) supaya burst request tidak memicu 429 rate limiter
/// per-user (ADR-0017) saat daftar berisi banyak sesi.
const int opnameSessionsDetailConcurrency = 6;

/// Daftar sesi dalam lingkup pengguna, diperkaya KPI progress per sesi.
///
/// KPI (`total`/`found`/`pending`/`variance`) hanya ada di respons
/// single-session, TIDAK di daftar — maka setiap sesi di-fetch detailnya.
/// Detail diambil per kelompok berukuran [opnameSessionsDetailConcurrency]
/// agar konkurensi terbatas (hindari burst 429). Kegagalan satu detail tidak
/// menjatuhkan daftar: sesi memakai data daftar tanpa KPI dan kartu merender
/// tanpa baris progress.
final opnameSessionsProvider =
    FutureProvider.autoDispose<List<StockOpnameSessionDto>>((Ref ref) async {
      final StockOpnameRepository repository = ref.watch(
        stockOpnameRepositoryProvider,
      );
      final StockOpnameSessionListDto page = await repository.sessions(
        limit: opnameSessionsFetchLimit,
      );

      final List<StockOpnameSessionDto> enriched = <StockOpnameSessionDto>[];
      for (
        int start = 0;
        start < page.data.length;
        start += opnameSessionsDetailConcurrency
      ) {
        final int end =
            (start + opnameSessionsDetailConcurrency) < page.data.length
            ? start + opnameSessionsDetailConcurrency
            : page.data.length;
        enriched.addAll(
          await Future.wait(
            page.data
                .getRange(start, end)
                .map(
                  (StockOpnameSessionDto session) => repository
                      .session(session.id)
                      .catchError((Object _) => session),
                ),
          ),
        );
      }
      return enriched;
    }, retry: (int retryCount, Object error) => null);
