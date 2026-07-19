import 'package:freezed_annotation/freezed_annotation.dart';

part 'request_step_dto.freezed.dart';
part 'request_step_dto.g.dart';

/// `RequestStep` openapi.yaml — satu tahap jenjang persetujuan pada
/// `GET /requests/{id}`.
@freezed
abstract class RequestStepDto with _$RequestStepDto {
  const factory RequestStepDto({
    @JsonKey(name: 'step_order') int? stepOrder,
    @JsonKey(name: 'required_level') String? requiredLevel,
    @JsonKey(name: 'approver_id') String? approverId,
    @JsonKey(name: 'approver_name') String? approverName,
    String? decision,
    String? note,
    @JsonKey(name: 'decided_at') DateTime? decidedAt,
  }) = _RequestStepDto;

  factory RequestStepDto.fromJson(Map<String, dynamic> json) =>
      _$RequestStepDtoFromJson(json);
}
