// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'stock_opname_session_dto.dart';

// **************************************************************************
// JsonSerializableGenerator
// **************************************************************************

_StockOpnameSessionDto _$StockOpnameSessionDtoFromJson(
  Map<String, dynamic> json,
) => _StockOpnameSessionDto(
  id: json['id'] as String,
  officeId: json['office_id'] as String,
  name: json['name'] as String?,
  period: json['period'] == null
      ? null
      : DateTime.parse(json['period'] as String),
  status: json['status'] as String,
  startedById: json['started_by_id'] as String,
  startedAt: json['started_at'] == null
      ? null
      : DateTime.parse(json['started_at'] as String),
  closedById: json['closed_by_id'] as String?,
  closedAt: json['closed_at'] == null
      ? null
      : DateTime.parse(json['closed_at'] as String),
  officeName: json['office_name'] as String?,
  startedByName: json['started_by_name'] as String?,
  closedByName: json['closed_by_name'] as String?,
  total: (json['total'] as num?)?.toInt(),
  found: (json['found'] as num?)?.toInt(),
  pending: (json['pending'] as num?)?.toInt(),
  variance: (json['variance'] as num?)?.toInt(),
  createdAt: json['created_at'] == null
      ? null
      : DateTime.parse(json['created_at'] as String),
  updatedAt: json['updated_at'] == null
      ? null
      : DateTime.parse(json['updated_at'] as String),
);

Map<String, dynamic> _$StockOpnameSessionDtoToJson(
  _StockOpnameSessionDto instance,
) => <String, dynamic>{
  'id': instance.id,
  'office_id': instance.officeId,
  'name': instance.name,
  'period': instance.period?.toIso8601String(),
  'status': instance.status,
  'started_by_id': instance.startedById,
  'started_at': instance.startedAt?.toIso8601String(),
  'closed_by_id': instance.closedById,
  'closed_at': instance.closedAt?.toIso8601String(),
  'office_name': instance.officeName,
  'started_by_name': instance.startedByName,
  'closed_by_name': instance.closedByName,
  'total': instance.total,
  'found': instance.found,
  'pending': instance.pending,
  'variance': instance.variance,
  'created_at': instance.createdAt?.toIso8601String(),
  'updated_at': instance.updatedAt?.toIso8601String(),
};
