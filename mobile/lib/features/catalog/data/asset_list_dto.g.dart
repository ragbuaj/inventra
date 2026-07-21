// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'asset_list_dto.dart';

// **************************************************************************
// JsonSerializableGenerator
// **************************************************************************

_AssetListDto _$AssetListDtoFromJson(Map<String, dynamic> json) =>
    _AssetListDto(
      data:
          (json['data'] as List<dynamic>?)
              ?.map((e) => AssetDto.fromJson(e as Map<String, dynamic>))
              .toList() ??
          const <AssetDto>[],
      total: (json['total'] as num).toInt(),
      limit: (json['limit'] as num).toInt(),
      offset: (json['offset'] as num).toInt(),
    );

Map<String, dynamic> _$AssetListDtoToJson(_AssetListDto instance) =>
    <String, dynamic>{
      'data': instance.data,
      'total': instance.total,
      'limit': instance.limit,
      'offset': instance.offset,
    };
