// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'stock_opname_session_list_dto.dart';

// **************************************************************************
// JsonSerializableGenerator
// **************************************************************************

_StockOpnameSessionListDto _$StockOpnameSessionListDtoFromJson(
  Map<String, dynamic> json,
) => _StockOpnameSessionListDto(
  data: (json['data'] as List<dynamic>)
      .map((e) => StockOpnameSessionDto.fromJson(e as Map<String, dynamic>))
      .toList(),
  total: (json['total'] as num).toInt(),
  limit: (json['limit'] as num).toInt(),
  offset: (json['offset'] as num).toInt(),
);

Map<String, dynamic> _$StockOpnameSessionListDtoToJson(
  _StockOpnameSessionListDto instance,
) => <String, dynamic>{
  'data': instance.data,
  'total': instance.total,
  'limit': instance.limit,
  'offset': instance.offset,
};
