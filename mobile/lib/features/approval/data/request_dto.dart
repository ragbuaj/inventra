import 'package:freezed_annotation/freezed_annotation.dart';

part 'request_dto.freezed.dart';
part 'request_dto.g.dart';

/// Kunci skema `Request`/`RequestDetail` openapi.yaml yang BISA dihapus
/// field-permission masking per peran (catatan kontrak: "Fields
/// `amount`/`payload`/`reason` may be omitted per role field-permissions").
/// Dipakai repository untuk membedakan field dimask dari field bernilai null.
const List<String> requestMaskableKeys = <String>[
  'amount',
  'payload',
  'reason',
];

/// `Request` openapi.yaml — item `GET /requests` dan respons approve/reject.
///
/// Field wajib kontrak (id/type/status/current_step/requested_by_id) non-null;
/// sisanya nullable — termasuk `amount`/`reason` yang bisa dihapus
/// field-permission masking. Nilai finansial adalah string desimal (IDR).
@freezed
abstract class RequestDto with _$RequestDto {
  const factory RequestDto({
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
  }) = _RequestDto;

  factory RequestDto.fromJson(Map<String, dynamic> json) =>
      _$RequestDtoFromJson(json);
}
