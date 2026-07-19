// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'stock_opname_item_result_dto.dart';

// **************************************************************************
// JsonSerializableGenerator
// **************************************************************************

_StockOpnameItemResultDto _$StockOpnameItemResultDtoFromJson(
  Map<String, dynamic> json,
) => _StockOpnameItemResultDto(
  id: json['id'] as String,
  sessionId: json['session_id'] as String,
  assetId: json['asset_id'] as String,
  expected: json['expected'] as bool,
  result: json['result'] as String,
  note: json['note'] as String?,
  countedAt: json['counted_at'] == null
      ? null
      : DateTime.parse(json['counted_at'] as String),
);

Map<String, dynamic> _$StockOpnameItemResultDtoToJson(
  _StockOpnameItemResultDto instance,
) => <String, dynamic>{
  'id': instance.id,
  'session_id': instance.sessionId,
  'asset_id': instance.assetId,
  'expected': instance.expected,
  'result': instance.result,
  'note': instance.note,
  'counted_at': instance.countedAt?.toIso8601String(),
};
