import 'package:inventra_mobile/core/api/app_failure.dart';
import 'package:inventra_mobile/features/stock_opname/data/stock_opname_item_dto.dart';
import 'package:inventra_mobile/features/stock_opname/data/stock_opname_item_list_dto.dart';
import 'package:inventra_mobile/features/stock_opname/data/stock_opname_item_result_dto.dart';
import 'package:inventra_mobile/features/stock_opname/data/stock_opname_repository.dart';
import 'package:inventra_mobile/features/stock_opname/data/stock_opname_scan_result_dto.dart';
import 'package:inventra_mobile/features/stock_opname/data/stock_opname_session_dto.dart';
import 'package:inventra_mobile/features/stock_opname/data/stock_opname_session_list_dto.dart';

/// [StockOpnameRepository] palsu berbasis data in-memory untuk widget/golden/
/// router test — tanpa Dio/HTTP. Scan me-lookup [itemsData] by tag; hasil
/// setResult diterapkan ke data sehingga refresh layar melihat nilai baru.
class FakeStockOpnameRepository implements StockOpnameRepository {
  FakeStockOpnameRepository({
    List<StockOpnameSessionDto>? sessionsData,
    List<StockOpnameItemDto>? itemsData,
  }) : sessionsData = sessionsData ?? <StockOpnameSessionDto>[],
       itemsData = List<StockOpnameItemDto>.of(
         itemsData ?? <StockOpnameItemDto>[],
       );

  final List<StockOpnameSessionDto> sessionsData;
  final List<StockOpnameItemDto> itemsData;

  /// Item yang baru terlihat oleh [scan] setelah dipindai (temuan di luar
  /// snapshot): tag -> item. Dipindah ke [itemsData] saat scan pertama.
  final Map<String, StockOpnameItemDto> unexpectedByTag =
      <String, StockOpnameItemDto>{};

  int scanCalls = 0;
  int setResultCalls = 0;

  @override
  Future<StockOpnameSessionListDto> sessions({
    String? status,
    int limit = 20,
    int offset = 0,
  }) async {
    return StockOpnameSessionListDto(
      data: sessionsData,
      total: sessionsData.length,
      limit: limit,
      offset: offset,
    );
  }

  @override
  Future<StockOpnameSessionDto> session(String id) async {
    for (final StockOpnameSessionDto session in sessionsData) {
      if (session.id == id) {
        return session;
      }
    }
    throw const NotFoundFailure();
  }

  @override
  Future<StockOpnameItemListDto> items(
    String sessionId, {
    OpnameItemResult? result,
  }) async {
    final List<StockOpnameItemDto> filtered = itemsData
        .where(
          (StockOpnameItemDto item) =>
              item.sessionId == sessionId &&
              (result == null || item.result == result.wire),
        )
        .toList(growable: false);
    return StockOpnameItemListDto(
      data: filtered,
      total: filtered.length,
      limit: filtered.length,
      offset: 0,
    );
  }

  @override
  Future<StockOpnameScanResultDto> scan(
    String sessionId,
    String assetTag,
  ) async {
    scanCalls += 1;
    StockOpnameItemDto? match;
    for (final StockOpnameItemDto item in itemsData) {
      if (item.sessionId == sessionId && item.assetTag == assetTag) {
        match = item;
        break;
      }
    }
    if (match == null) {
      final StockOpnameItemDto? unexpected = unexpectedByTag.remove(assetTag);
      if (unexpected == null) {
        throw const NotFoundFailure();
      }
      itemsData.add(unexpected);
      match = unexpected;
    }
    return StockOpnameScanResultDto(
      id: match.id,
      sessionId: match.sessionId,
      assetId: match.assetId,
      expected: match.expected,
      result: match.result,
    );
  }

  @override
  Future<StockOpnameItemResultDto> setResult(
    String sessionId,
    String itemId, {
    required OpnameItemResult result,
    String? note,
  }) async {
    setResultCalls += 1;
    final int index = itemsData.indexWhere(
      (StockOpnameItemDto item) =>
          item.sessionId == sessionId && item.id == itemId,
    );
    if (index < 0) {
      throw const NotFoundFailure();
    }
    final StockOpnameItemDto updated = itemsData[index].copyWith(
      result: result.wire,
      note: note,
      countedAt: DateTime.utc(2026, 7, 19, 3),
    );
    itemsData[index] = updated;
    return StockOpnameItemResultDto(
      id: updated.id,
      sessionId: updated.sessionId,
      assetId: updated.assetId,
      expected: updated.expected,
      result: updated.result,
      note: updated.note,
      countedAt: updated.countedAt,
    );
  }

  @override
  Future<OpnameVarianceData> variance(String sessionId) async {
    final StockOpnameItemListDto list = await items(sessionId);
    return OpnameVarianceData.fromItems(list.data);
  }
}
