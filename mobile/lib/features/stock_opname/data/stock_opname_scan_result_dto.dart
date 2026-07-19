import 'package:freezed_annotation/freezed_annotation.dart';

part 'stock_opname_scan_result_dto.freezed.dart';
part 'stock_opname_scan_result_dto.g.dart';

/// `StockOpnameScanResult` openapi.yaml — respons
/// `POST /stock-opname/sessions/{id}/scan`: item yang ter-resolve (match
/// snapshot, atau baris baru `expected: false` untuk temuan di luar catatan).
@freezed
abstract class StockOpnameScanResultDto with _$StockOpnameScanResultDto {
  const factory StockOpnameScanResultDto({
    required String id,
    @JsonKey(name: 'session_id') required String sessionId,
    @JsonKey(name: 'asset_id') required String assetId,
    required bool expected,
    required String result,
  }) = _StockOpnameScanResultDto;

  factory StockOpnameScanResultDto.fromJson(Map<String, dynamic> json) =>
      _$StockOpnameScanResultDtoFromJson(json);
}
