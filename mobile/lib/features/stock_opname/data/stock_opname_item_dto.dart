import 'package:freezed_annotation/freezed_annotation.dart';

part 'stock_opname_item_dto.freezed.dart';
part 'stock_opname_item_dto.g.dart';

/// `StockOpnameItem` openapi.yaml — satu aset dalam snapshot/hitungan sesi.
///
/// `expected: false` berarti item disisipkan oleh scan di luar snapshot
/// ("unexpected find"); `followup_record_id` terisi (alih-alih
/// `followup_request_id`) bila tindak lanjutnya record maintenance item
/// `damaged`.
@freezed
abstract class StockOpnameItemDto with _$StockOpnameItemDto {
  const factory StockOpnameItemDto({
    required String id,
    @JsonKey(name: 'session_id') required String sessionId,
    @JsonKey(name: 'asset_id') required String assetId,
    @JsonKey(name: 'asset_name') String? assetName,
    @JsonKey(name: 'asset_tag') String? assetTag,
    @JsonKey(name: 'office_name') String? officeName,
    @JsonKey(name: 'room_name') String? roomName,
    @JsonKey(name: 'floor_name') String? floorName,
    required bool expected,
    required String result,
    String? note,
    @JsonKey(name: 'counted_by_name') String? countedByName,
    @JsonKey(name: 'counted_at') DateTime? countedAt,
    @JsonKey(name: 'followup_request_id') String? followupRequestId,
    @JsonKey(name: 'followup_record_id') String? followupRecordId,
  }) = _StockOpnameItemDto;

  factory StockOpnameItemDto.fromJson(Map<String, dynamic> json) =>
      _$StockOpnameItemDtoFromJson(json);
}
