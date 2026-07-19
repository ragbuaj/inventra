// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'stock_opname_scan_result_dto.dart';

// **************************************************************************
// JsonSerializableGenerator
// **************************************************************************

_StockOpnameScanResultDto _$StockOpnameScanResultDtoFromJson(
  Map<String, dynamic> json,
) => _StockOpnameScanResultDto(
  id: json['id'] as String,
  sessionId: json['session_id'] as String,
  assetId: json['asset_id'] as String,
  expected: json['expected'] as bool,
  result: json['result'] as String,
);

Map<String, dynamic> _$StockOpnameScanResultDtoToJson(
  _StockOpnameScanResultDto instance,
) => <String, dynamic>{
  'id': instance.id,
  'session_id': instance.sessionId,
  'asset_id': instance.assetId,
  'expected': instance.expected,
  'result': instance.result,
};
