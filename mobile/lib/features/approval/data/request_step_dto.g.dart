// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'request_step_dto.dart';

// **************************************************************************
// JsonSerializableGenerator
// **************************************************************************

_RequestStepDto _$RequestStepDtoFromJson(Map<String, dynamic> json) =>
    _RequestStepDto(
      stepOrder: (json['step_order'] as num?)?.toInt(),
      requiredLevel: json['required_level'] as String?,
      approverId: json['approver_id'] as String?,
      approverName: json['approver_name'] as String?,
      decision: json['decision'] as String?,
      note: json['note'] as String?,
      decidedAt: json['decided_at'] == null
          ? null
          : DateTime.parse(json['decided_at'] as String),
    );

Map<String, dynamic> _$RequestStepDtoToJson(_RequestStepDto instance) =>
    <String, dynamic>{
      'step_order': instance.stepOrder,
      'required_level': instance.requiredLevel,
      'approver_id': instance.approverId,
      'approver_name': instance.approverName,
      'decision': instance.decision,
      'note': instance.note,
      'decided_at': instance.decidedAt?.toIso8601String(),
    };
