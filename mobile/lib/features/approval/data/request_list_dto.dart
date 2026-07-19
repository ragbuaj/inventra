import 'package:freezed_annotation/freezed_annotation.dart';

import 'request_dto.dart';

part 'request_list_dto.freezed.dart';
part 'request_list_dto.g.dart';

/// `RequestList` openapi.yaml — halaman `GET /requests` (limit/offset).
@freezed
abstract class RequestListDto with _$RequestListDto {
  const factory RequestListDto({
    @Default(<RequestDto>[]) List<RequestDto> data,
    required int total,
    required int limit,
    required int offset,
  }) = _RequestListDto;

  factory RequestListDto.fromJson(Map<String, dynamic> json) =>
      _$RequestListDtoFromJson(json);
}
