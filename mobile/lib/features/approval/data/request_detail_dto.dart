import 'package:freezed_annotation/freezed_annotation.dart';

import 'request_step_dto.dart';

part 'request_detail_dto.freezed.dart';
part 'request_detail_dto.g.dart';

/// `RequestDetail` openapi.yaml (allOf `Request` + `payload` + `steps`) —
/// respons `GET /requests/{id}`.
///
/// `payload` adalah objek bebas sesuai jenis pengajuan (AssetCreatePayload /
/// DisposalPayload / TransferPayload); absen bila dimask field permission.
@freezed
abstract class RequestDetailDto with _$RequestDetailDto {
  const factory RequestDetailDto({
    required String id,
    required String type,
    required String status,
    String? amount,
    @JsonKey(name: 'current_step') required int currentStep,
    @JsonKey(name: 'office_id') String? officeId,
    @JsonKey(name: 'target_id') String? targetId,
    @JsonKey(name: 'target_entity') String? targetEntity,
    String? reason,
    @JsonKey(name: 'requested_by_id') required String requestedById,
    @JsonKey(name: 'requested_by_name') String? requestedByName,
    @JsonKey(name: 'requested_by_role') String? requestedByRole,
    @JsonKey(name: 'office_name') String? officeName,
    @JsonKey(name: 'decided_by_id') String? decidedById,
    @JsonKey(name: 'decision_note') String? decisionNote,
    @JsonKey(name: 'created_at') DateTime? createdAt,
    Map<String, dynamic>? payload,
    @Default(<RequestStepDto>[]) List<RequestStepDto> steps,
  }) = _RequestDetailDto;

  factory RequestDetailDto.fromJson(Map<String, dynamic> json) =>
      _$RequestDetailDtoFromJson(json);
}
