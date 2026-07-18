// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'user_dto.dart';

// **************************************************************************
// JsonSerializableGenerator
// **************************************************************************

_UserDto _$UserDtoFromJson(Map<String, dynamic> json) => _UserDto(
  id: json['id'] as String,
  name: json['name'] as String,
  email: json['email'] as String,
  roleId: json['role_id'] as String,
  officeId: json['office_id'] as String?,
  employeeId: json['employee_id'] as String?,
  status: json['status'] as String,
  hasAvatar: json['has_avatar'] as bool? ?? false,
  googleLinked: json['google_linked'] as bool,
  createdAt: json['created_at'] == null
      ? null
      : DateTime.parse(json['created_at'] as String),
  updatedAt: json['updated_at'] == null
      ? null
      : DateTime.parse(json['updated_at'] as String),
);

Map<String, dynamic> _$UserDtoToJson(_UserDto instance) => <String, dynamic>{
  'id': instance.id,
  'name': instance.name,
  'email': instance.email,
  'role_id': instance.roleId,
  'office_id': instance.officeId,
  'employee_id': instance.employeeId,
  'status': instance.status,
  'has_avatar': instance.hasAvatar,
  'google_linked': instance.googleLinked,
  'created_at': instance.createdAt?.toIso8601String(),
  'updated_at': instance.updatedAt?.toIso8601String(),
};
