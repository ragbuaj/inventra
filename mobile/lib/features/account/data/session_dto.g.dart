// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'session_dto.dart';

// **************************************************************************
// JsonSerializableGenerator
// **************************************************************************

_SessionDto _$SessionDtoFromJson(Map<String, dynamic> json) => _SessionDto(
  id: json['id'] as String,
  browser: json['browser'] as String,
  os: json['os'] as String,
  deviceType: json['device_type'] as String,
  ipAddress: json['ip_address'] as String,
  location: json['location'] as String,
  createdAt: DateTime.parse(json['created_at'] as String),
  lastSeenAt: DateTime.parse(json['last_seen_at'] as String),
  current: json['current'] as bool,
);

Map<String, dynamic> _$SessionDtoToJson(_SessionDto instance) =>
    <String, dynamic>{
      'id': instance.id,
      'browser': instance.browser,
      'os': instance.os,
      'device_type': instance.deviceType,
      'ip_address': instance.ipAddress,
      'location': instance.location,
      'created_at': instance.createdAt.toIso8601String(),
      'last_seen_at': instance.lastSeenAt.toIso8601String(),
      'current': instance.current,
    };
