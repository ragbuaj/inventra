import 'package:freezed_annotation/freezed_annotation.dart';

import '../../asset_detail/data/asset_dto.dart';

part 'asset_list_dto.freezed.dart';
part 'asset_list_dto.g.dart';

/// Halaman `GET /assets` (kontrak list `{data, total, limit, offset}`,
/// clamp limit 1-100 di server). Item memakai skema `Asset` yang sama dengan
/// detail ([AssetDto]) — field bisa dihapus field-permission masking backend.
@freezed
abstract class AssetListDto with _$AssetListDto {
  const factory AssetListDto({
    @Default(<AssetDto>[]) List<AssetDto> data,
    required int total,
    required int limit,
    required int offset,
  }) = _AssetListDto;

  factory AssetListDto.fromJson(Map<String, dynamic> json) =>
      _$AssetListDtoFromJson(json);
}
