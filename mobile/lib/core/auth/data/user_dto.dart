import 'package:freezed_annotation/freezed_annotation.dart';

part 'user_dto.freezed.dart';
part 'user_dto.g.dart';

/// `User` openapi.yaml — respons `GET /auth/me`.
@freezed
abstract class UserDto with _$UserDto {
  const factory UserDto({
    required String id,
    required String name,
    required String email,
    @JsonKey(name: 'role_id') required String roleId,
    @JsonKey(name: 'office_id') String? officeId,
    @JsonKey(name: 'employee_id') String? employeeId,
    required String status,
    @JsonKey(name: 'has_avatar') @Default(false) bool hasAvatar,
    @JsonKey(name: 'google_linked') required bool googleLinked,
    @JsonKey(name: 'created_at') DateTime? createdAt,
    @JsonKey(name: 'updated_at') DateTime? updatedAt,
  }) = _UserDto;

  factory UserDto.fromJson(Map<String, dynamic> json) =>
      _$UserDtoFromJson(json);
}
