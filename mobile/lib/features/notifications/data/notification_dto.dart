import 'package:freezed_annotation/freezed_annotation.dart';

part 'notification_dto.freezed.dart';
part 'notification_dto.g.dart';

/// `Notification` openapi.yaml — item `GET /notifications` dan respons
/// `POST /notifications/{id}/read`.
///
/// `params` adalah nilai interpolasi bebas untuk kalimat yang dirender KLIEN
/// via i18n (ADR-0014: server tidak pernah mengirim kalimat jadi). `type` di
/// luar enum kontrak tetap diparse apa adanya — presentasi punya fallback.
/// `read_at` null berarti belum dibaca.
@freezed
abstract class NotificationDto with _$NotificationDto {
  const factory NotificationDto({
    required String id,
    required String type,
    @Default(<String, dynamic>{}) Map<String, dynamic> params,
    @JsonKey(name: 'entity_type') String? entityType,
    @JsonKey(name: 'entity_id') String? entityId,
    @JsonKey(name: 'read_at') DateTime? readAt,
    @JsonKey(name: 'created_at') required DateTime createdAt,
  }) = _NotificationDto;

  factory NotificationDto.fromJson(Map<String, dynamic> json) =>
      _$NotificationDtoFromJson(json);
}
