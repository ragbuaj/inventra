import 'package:freezed_annotation/freezed_annotation.dart';

part 'session_dto.freezed.dart';
part 'session_dto.g.dart';

/// `SessionView` openapi.yaml — item `GET /auth/sessions`.
///
/// browser/os/device_type diturunkan server dari user-agent tersimpan;
/// `location` adalah kota/negara GeoIP best-effort dan bisa kosong (klien
/// jatuh ke IP). `current` menandai sesi yang sedang memanggil. Tidak pernah
/// memuat refresh token/JTI mentah (kontrak).
@freezed
abstract class SessionDto with _$SessionDto {
  const factory SessionDto({
    required String id,
    required String browser,
    required String os,
    @JsonKey(name: 'device_type') required String deviceType,
    @JsonKey(name: 'ip_address') required String ipAddress,
    required String location,
    @JsonKey(name: 'created_at') required DateTime createdAt,
    @JsonKey(name: 'last_seen_at') required DateTime lastSeenAt,
    required bool current,
  }) = _SessionDto;

  factory SessionDto.fromJson(Map<String, dynamic> json) =>
      _$SessionDtoFromJson(json);
}
