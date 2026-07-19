// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'stock_opname_item_dto.dart';

// **************************************************************************
// JsonSerializableGenerator
// **************************************************************************

_StockOpnameItemDto _$StockOpnameItemDtoFromJson(Map<String, dynamic> json) =>
    _StockOpnameItemDto(
      id: json['id'] as String,
      sessionId: json['session_id'] as String,
      assetId: json['asset_id'] as String,
      assetName: json['asset_name'] as String?,
      assetTag: json['asset_tag'] as String?,
      officeName: json['office_name'] as String?,
      roomName: json['room_name'] as String?,
      floorName: json['floor_name'] as String?,
      expected: json['expected'] as bool,
      result: json['result'] as String,
      note: json['note'] as String?,
      countedByName: json['counted_by_name'] as String?,
      countedAt: json['counted_at'] == null
          ? null
          : DateTime.parse(json['counted_at'] as String),
      followupRequestId: json['followup_request_id'] as String?,
      followupRecordId: json['followup_record_id'] as String?,
    );

Map<String, dynamic> _$StockOpnameItemDtoToJson(_StockOpnameItemDto instance) =>
    <String, dynamic>{
      'id': instance.id,
      'session_id': instance.sessionId,
      'asset_id': instance.assetId,
      'asset_name': instance.assetName,
      'asset_tag': instance.assetTag,
      'office_name': instance.officeName,
      'room_name': instance.roomName,
      'floor_name': instance.floorName,
      'expected': instance.expected,
      'result': instance.result,
      'note': instance.note,
      'counted_by_name': instance.countedByName,
      'counted_at': instance.countedAt?.toIso8601String(),
      'followup_request_id': instance.followupRequestId,
      'followup_record_id': instance.followupRecordId,
    };
