import 'package:freezed_annotation/freezed_annotation.dart';

import 'stock_opname_item_dto.dart';

part 'stock_opname_item_list_dto.freezed.dart';
part 'stock_opname_item_list_dto.g.dart';

/// `StockOpnameItemList` openapi.yaml — respons
/// `GET /stock-opname/sessions/{id}/items` (tidak dipaginasi; `limit`/`offset`
/// mencerminkan jumlah hasil penuh).
@freezed
abstract class StockOpnameItemListDto with _$StockOpnameItemListDto {
  const factory StockOpnameItemListDto({
    required List<StockOpnameItemDto> data,
    required int total,
    required int limit,
    required int offset,
  }) = _StockOpnameItemListDto;

  factory StockOpnameItemListDto.fromJson(Map<String, dynamic> json) =>
      _$StockOpnameItemListDtoFromJson(json);
}
