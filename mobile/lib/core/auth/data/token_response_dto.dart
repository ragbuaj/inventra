import 'package:freezed_annotation/freezed_annotation.dart';

part 'token_response_dto.freezed.dart';
part 'token_response_dto.g.dart';

/// `TokenResponse` openapi.yaml — respons `POST /auth/login` dan
/// `POST /auth/refresh`. Klien mobile (`X-Client-Type: mobile`) menerima
/// `refresh_token` di body; rotasi selalu mengirim token baru (ADR-0017).
@freezed
abstract class TokenResponseDto with _$TokenResponseDto {
  const TokenResponseDto._();

  const factory TokenResponseDto({
    @JsonKey(name: 'access_token') required String accessToken,
    @JsonKey(name: 'token_type') required String tokenType,
    @JsonKey(name: 'expires_in') required int expiresIn,
    @JsonKey(name: 'refresh_token') String? refreshToken,
  }) = _TokenResponseDto;

  factory TokenResponseDto.fromJson(Map<String, dynamic> json) =>
      _$TokenResponseDtoFromJson(json);

  /// Override toString supaya `access_token`/`refresh_token` mentah tidak
  /// pernah masuk log atau crash reporter. Nilai token diredaksi (hanya
  /// keberadaannya yang dilaporkan), token_type & expires_in aman ditampilkan.
  @override
  String toString() =>
      'TokenResponseDto(accessToken: [redacted], tokenType: $tokenType, '
      'expiresIn: $expiresIn, '
      'refreshToken: ${refreshToken == null ? 'null' : '[redacted]'})';
}
