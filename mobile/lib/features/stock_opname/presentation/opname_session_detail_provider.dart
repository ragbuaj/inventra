import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../data/stock_opname_item_dto.dart';
import '../data/stock_opname_item_list_dto.dart';
import '../data/stock_opname_repository.dart';
import '../data/stock_opname_session_dto.dart';

/// Detail sesi + seluruh itemnya — dipakai layar Counting dan Variance.
class OpnameSessionDetail {
  const OpnameSessionDetail({required this.session, required this.items});

  final StockOpnameSessionDto session;
  final List<StockOpnameItemDto> items;

  /// Item yang sudah dihitung, terbaru dulu — daftar "Baru saja dipindai".
  List<StockOpnameItemDto> get counted {
    final List<StockOpnameItemDto> result = items
        .where((StockOpnameItemDto item) => item.countedAt != null)
        .toList(growable: false);
    result.sort(
      (StockOpnameItemDto a, StockOpnameItemDto b) =>
          b.countedAt!.compareTo(a.countedAt!),
    );
    return result;
  }

  int countOf(String result) =>
      items.where((StockOpnameItemDto item) => item.result == result).length;
}

/// `GET /sessions/{id}` + `GET /sessions/{id}/items` paralel. autoDispose:
/// state dibuang saat layar ditutup; refresh via `ref.invalidate`.
final opnameSessionDetailProvider = FutureProvider.autoDispose
    .family<OpnameSessionDetail, String>((Ref ref, String sessionId) async {
      final StockOpnameRepository repository = ref.watch(
        stockOpnameRepositoryProvider,
      );
      // Future.wait eagerError: kegagalan pertama dilempar apa adanya
      // (AppFailure), bukan dibungkus error paralel.
      final List<Object> results = await Future.wait<Object>(<Future<Object>>[
        repository.session(sessionId),
        repository.items(sessionId),
      ], eagerError: true);
      return OpnameSessionDetail(
        session: results[0] as StockOpnameSessionDto,
        items: (results[1] as StockOpnameItemListDto).data,
      );
    }, retry: (int retryCount, Object error) => null);
