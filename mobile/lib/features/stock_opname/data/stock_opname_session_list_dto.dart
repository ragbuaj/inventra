import 'package:freezed_annotation/freezed_annotation.dart';

import 'stock_opname_session_dto.dart';

part 'stock_opname_session_list_dto.freezed.dart';
part 'stock_opname_session_list_dto.g.dart';

/// `StockOpnameSessionList` openapi.yaml — halaman `GET /stock-opname/sessions`.
@freezed
abstract class StockOpnameSessionListDto with _$StockOpnameSessionListDto {
  const factory StockOpnameSessionListDto({
    required List<StockOpnameSessionDto> data,
    required int total,
    required int limit,
    required int offset,
  }) = _StockOpnameSessionListDto;

  factory StockOpnameSessionListDto.fromJson(Map<String, dynamic> json) =>
      _$StockOpnameSessionListDtoFromJson(json);
}
