// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'notification_list_dto.dart';

// **************************************************************************
// JsonSerializableGenerator
// **************************************************************************

_NotificationListDto _$NotificationListDtoFromJson(Map<String, dynamic> json) =>
    _NotificationListDto(
      data:
          (json['data'] as List<dynamic>?)
              ?.map((e) => NotificationDto.fromJson(e as Map<String, dynamic>))
              .toList() ??
          const <NotificationDto>[],
      total: (json['total'] as num).toInt(),
      limit: (json['limit'] as num).toInt(),
      offset: (json['offset'] as num).toInt(),
    );

Map<String, dynamic> _$NotificationListDtoToJson(
  _NotificationListDto instance,
) => <String, dynamic>{
  'data': instance.data,
  'total': instance.total,
  'limit': instance.limit,
  'offset': instance.offset,
};
