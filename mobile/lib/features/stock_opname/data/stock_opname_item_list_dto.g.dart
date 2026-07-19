// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'stock_opname_item_list_dto.dart';

// **************************************************************************
// JsonSerializableGenerator
// **************************************************************************

_StockOpnameItemListDto _$StockOpnameItemListDtoFromJson(
  Map<String, dynamic> json,
) => _StockOpnameItemListDto(
  data: (json['data'] as List<dynamic>)
      .map((e) => StockOpnameItemDto.fromJson(e as Map<String, dynamic>))
      .toList(),
  total: (json['total'] as num).toInt(),
  limit: (json['limit'] as num).toInt(),
  offset: (json['offset'] as num).toInt(),
);

Map<String, dynamic> _$StockOpnameItemListDtoToJson(
  _StockOpnameItemListDto instance,
) => <String, dynamic>{
  'data': instance.data,
  'total': instance.total,
  'limit': instance.limit,
  'offset': instance.offset,
};
