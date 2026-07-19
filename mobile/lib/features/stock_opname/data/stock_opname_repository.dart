import 'package:dio/dio.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../../core/api/dio_provider.dart';
import '../../../core/api/error_mapper.dart';
import 'stock_opname_item_dto.dart';
import 'stock_opname_item_list_dto.dart';
import 'stock_opname_item_result_dto.dart';
import 'stock_opname_scan_result_dto.dart';
import 'stock_opname_session_dto.dart';
import 'stock_opname_session_list_dto.dart';

/// Nilai enum `result` item opname (kontrak `StockOpnameItem.result`).
enum OpnameItemResult {
  found('found'),
  notFound('not_found'),
  damaged('damaged'),
  misplaced('misplaced'),
  pending('pending');

  const OpnameItemResult(this.wire);

  /// Nilai kawat persis openapi (`found|not_found|damaged|misplaced|pending`).
  final String wire;

  /// null untuk nilai di luar kontrak (defensif terhadap server lebih baru).
  static OpnameItemResult? tryParse(String? wire) {
    for (final OpnameItemResult value in OpnameItemResult.values) {
      if (value.wire == wire) {
        return value;
      }
    }
    return null;
  }
}

/// Data layar Variance yang dihitung klien dari satu panggilan
/// `GET .../items`: ringkasan + kelompok per kategori selisih. Tidak ada
/// endpoint variance khusus di kontrak — selisih adalah turunan hasil item.
class OpnameVarianceData {
  const OpnameVarianceData({
    required this.notFound,
    required this.damaged,
    required this.misplaced,
    required this.unexpected,
  });

  /// Item snapshot dengan hasil `not_found`.
  final List<StockOpnameItemDto> notFound;

  /// Item dengan hasil `damaged`.
  final List<StockOpnameItemDto> damaged;

  /// Item dengan hasil `misplaced`.
  final List<StockOpnameItemDto> misplaced;

  /// Temuan di luar catatan (`expected: false`), apa pun hasilnya.
  final List<StockOpnameItemDto> unexpected;

  bool get isEmpty =>
      notFound.isEmpty &&
      damaged.isEmpty &&
      misplaced.isEmpty &&
      unexpected.isEmpty;

  /// Mengelompokkan [items]: kategori hasil hanya diisi item snapshot
  /// (`expected: true`) supaya temuan di luar catatan tidak terhitung ganda
  /// dengan kelompok "Di Luar Catatan".
  factory OpnameVarianceData.fromItems(List<StockOpnameItemDto> items) {
    List<StockOpnameItemDto> byResult(OpnameItemResult result) =>
        List<StockOpnameItemDto>.unmodifiable(
          items.where(
            (StockOpnameItemDto item) =>
                item.expected && item.result == result.wire,
          ),
        );

    return OpnameVarianceData(
      notFound: byResult(OpnameItemResult.notFound),
      damaged: byResult(OpnameItemResult.damaged),
      misplaced: byResult(OpnameItemResult.misplaced),
      unexpected: List<StockOpnameItemDto>.unmodifiable(
        items.where((StockOpnameItemDto item) => !item.expected),
      ),
    );
  }
}

/// Repository modul stock opname (kontrak backend/api/openapi.yaml):
/// `GET /stock-opname/sessions` (list, filter status + limit/offset),
/// `GET /stock-opname/sessions/{id}` (detail + KPI counter),
/// `GET /stock-opname/sessions/{id}/items` (filter result opsional),
/// `POST /stock-opname/sessions/{id}/scan` (lookup by asset_tag; auto-add
/// temuan di luar snapshot), dan
/// `PATCH /stock-opname/sessions/{id}/items/{itemId}` (catat hasil hitung).
///
/// State machine sesi ditegakkan SERVER: scan/catat hasil di luar tahap
/// `counting` mendapat 409 ([ConflictFailure]) — klien tidak menduplikasi
/// aturannya. Fase M0 online-only: TANPA antrean offline (drift menyusul M5).
class StockOpnameRepository {
  StockOpnameRepository(this._dio);

  final Dio _dio;

  Future<StockOpnameSessionListDto> sessions({
    String? status,
    int limit = 20,
    int offset = 0,
  }) async {
    try {
      final Response<Map<String, dynamic>> response = await _dio
          .get<Map<String, dynamic>>(
            '/stock-opname/sessions',
            queryParameters: <String, dynamic>{
              'status': ?status,
              'limit': limit,
              'offset': offset,
            },
          );
      return StockOpnameSessionListDto.fromJson(response.data!);
    } on DioException catch (err) {
      throw err.toAppFailure();
    }
  }

  /// Detail satu sesi, diperkaya nama kantor/petugas + KPI
  /// `total`/`found`/`pending`/`variance`.
  Future<StockOpnameSessionDto> session(String id) async {
    try {
      final Response<Map<String, dynamic>> response = await _dio
          .get<Map<String, dynamic>>(
            '/stock-opname/sessions/${Uri.encodeComponent(id)}',
          );
      return StockOpnameSessionDto.fromJson(response.data!);
    } on DioException catch (err) {
      throw err.toAppFailure();
    }
  }

  Future<StockOpnameItemListDto> items(
    String sessionId, {
    OpnameItemResult? result,
  }) async {
    try {
      final Response<Map<String, dynamic>> response = await _dio
          .get<Map<String, dynamic>>(
            '/stock-opname/sessions/${Uri.encodeComponent(sessionId)}/items',
            queryParameters: <String, dynamic>{'result': ?result?.wire},
          );
      return StockOpnameItemListDto.fromJson(response.data!);
    } on DioException catch (err) {
      throw err.toAppFailure();
    }
  }

  /// Resolve tag hasil pindai/ketik terhadap sesi `counting`. 404 berarti tag
  /// tidak dikenal ([NotFoundFailure]); 409 berarti sesi bukan tahap
  /// menghitung ([ConflictFailure]).
  Future<StockOpnameScanResultDto> scan(
    String sessionId,
    String assetTag,
  ) async {
    try {
      final Response<Map<String, dynamic>> response = await _dio
          .post<Map<String, dynamic>>(
            '/stock-opname/sessions/${Uri.encodeComponent(sessionId)}/scan',
            data: <String, dynamic>{'asset_tag': assetTag},
          );
      return StockOpnameScanResultDto.fromJson(response.data!);
    } on DioException catch (err) {
      throw err.toAppFailure();
    }
  }

  /// Catat hasil hitung satu item (`StockOpnameSetResultRequest`); `note`
  /// hanya dikirim bila diisi.
  Future<StockOpnameItemResultDto> setResult(
    String sessionId,
    String itemId, {
    required OpnameItemResult result,
    String? note,
  }) async {
    final String? trimmed = note?.trim();
    try {
      final Response<Map<String, dynamic>> response = await _dio
          .patch<Map<String, dynamic>>(
            '/stock-opname/sessions/${Uri.encodeComponent(sessionId)}'
            '/items/${Uri.encodeComponent(itemId)}',
            data: <String, dynamic>{
              'result': result.wire,
              if (trimmed != null && trimmed.isNotEmpty) 'note': trimmed,
            },
          );
      return StockOpnameItemResultDto.fromJson(response.data!);
    } on DioException catch (err) {
      throw err.toAppFailure();
    }
  }

  /// Data layar Variance: seluruh item sesi dikelompokkan klien — lihat
  /// [OpnameVarianceData].
  Future<OpnameVarianceData> variance(String sessionId) async {
    final StockOpnameItemListDto list = await items(sessionId);
    return OpnameVarianceData.fromItems(list.data);
  }
}

final Provider<StockOpnameRepository> stockOpnameRepositoryProvider =
    Provider<StockOpnameRepository>(
      (Ref ref) => StockOpnameRepository(ref.watch(dioProvider)),
    );
