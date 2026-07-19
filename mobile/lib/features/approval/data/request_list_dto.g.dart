// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'request_list_dto.dart';

// **************************************************************************
// JsonSerializableGenerator
// **************************************************************************

_RequestListDto _$RequestListDtoFromJson(Map<String, dynamic> json) =>
    _RequestListDto(
      data:
          (json['data'] as List<dynamic>?)
              ?.map((e) => RequestDto.fromJson(e as Map<String, dynamic>))
              .toList() ??
          const <RequestDto>[],
      total: (json['total'] as num).toInt(),
      limit: (json['limit'] as num).toInt(),
      offset: (json['offset'] as num).toInt(),
    );

Map<String, dynamic> _$RequestListDtoToJson(_RequestListDto instance) =>
    <String, dynamic>{
      'data': instance.data,
      'total': instance.total,
      'limit': instance.limit,
      'offset': instance.offset,
    };
