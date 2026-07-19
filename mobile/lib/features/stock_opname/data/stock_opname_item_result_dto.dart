import 'package:freezed_annotation/freezed_annotation.dart';

part 'stock_opname_item_result_dto.freezed.dart';
part 'stock_opname_item_result_dto.g.dart';

/// `StockOpnameItemResultResponse` openapi.yaml — respons
/// `PATCH /stock-opname/sessions/{id}/items/{itemId}` setelah hasil hitung
/// satu item dicatat.
@freezed
abstract class StockOpnameItemResultDto with _$StockOpnameItemResultDto {
  const factory StockOpnameItemResultDto({
    required String id,
    @JsonKey(name: 'session_id') required String sessionId,
    @JsonKey(name: 'asset_id') required String assetId,
    required bool expected,
    required String result,
    String? note,
    @JsonKey(name: 'counted_at') DateTime? countedAt,
  }) = _StockOpnameItemResultDto;

  factory StockOpnameItemResultDto.fromJson(Map<String, dynamic> json) =>
      _$StockOpnameItemResultDtoFromJson(json);
}
