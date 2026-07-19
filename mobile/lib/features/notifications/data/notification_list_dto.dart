import 'package:freezed_annotation/freezed_annotation.dart';

import 'notification_dto.dart';

part 'notification_list_dto.freezed.dart';
part 'notification_list_dto.g.dart';

/// `NotificationList` openapi.yaml — halaman `GET /notifications`
/// (limit/offset, terbaru lebih dulu).
@freezed
abstract class NotificationListDto with _$NotificationListDto {
  const factory NotificationListDto({
    @Default(<NotificationDto>[]) List<NotificationDto> data,
    required int total,
    required int limit,
    required int offset,
  }) = _NotificationListDto;

  factory NotificationListDto.fromJson(Map<String, dynamic> json) =>
      _$NotificationListDtoFromJson(json);
}
