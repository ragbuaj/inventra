// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'request_dto.dart';

// **************************************************************************
// JsonSerializableGenerator
// **************************************************************************

_RequestDto _$RequestDtoFromJson(Map<String, dynamic> json) => _RequestDto(
  id: json['id'] as String,
  type: json['type'] as String,
  status: json['status'] as String,
  amount: json['amount'] as String?,
  currentStep: (json['current_step'] as num).toInt(),
  officeId: json['office_id'] as String?,
  targetId: json['target_id'] as String?,
  targetEntity: json['target_entity'] as String?,
  reason: json['reason'] as String?,
  requestedById: json['requested_by_id'] as String,
  requestedByName: json['requested_by_name'] as String?,
  requestedByRole: json['requested_by_role'] as String?,
  officeName: json['office_name'] as String?,
  decidedById: json['decided_by_id'] as String?,
  decisionNote: json['decision_note'] as String?,
  createdAt: json['created_at'] == null
      ? null
      : DateTime.parse(json['created_at'] as String),
);

Map<String, dynamic> _$RequestDtoToJson(_RequestDto instance) =>
    <String, dynamic>{
      'id': instance.id,
      'type': instance.type,
      'status': instance.status,
      'amount': instance.amount,
      'current_step': instance.currentStep,
      'office_id': instance.officeId,
      'target_id': instance.targetId,
      'target_entity': instance.targetEntity,
      'reason': instance.reason,
      'requested_by_id': instance.requestedById,
      'requested_by_name': instance.requestedByName,
      'requested_by_role': instance.requestedByRole,
      'office_name': instance.officeName,
      'decided_by_id': instance.decidedById,
      'decision_note': instance.decisionNote,
      'created_at': instance.createdAt?.toIso8601String(),
    };
