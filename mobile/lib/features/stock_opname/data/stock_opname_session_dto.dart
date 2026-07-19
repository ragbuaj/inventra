import 'package:freezed_annotation/freezed_annotation.dart';

part 'stock_opname_session_dto.freezed.dart';
part 'stock_opname_session_dto.g.dart';

/// `StockOpnameSession` openapi.yaml — item `GET /stock-opname/sessions` dan
/// respons `GET /stock-opname/sessions/{id}`.
///
/// KPI hasil item (`total`/`found`/`pending`/`variance`) hanya dikirim pada
/// respons single-session (get by id) — null pada daftar. `period`
/// dinormalisasi backend ke tanggal 1 tiap bulan (format `YYYY-MM-DD`).
@freezed
abstract class StockOpnameSessionDto with _$StockOpnameSessionDto {
  const factory StockOpnameSessionDto({
    required String id,
    @JsonKey(name: 'office_id') required String officeId,
    String? name,
    DateTime? period,
    required String status,
    @JsonKey(name: 'started_by_id') required String startedById,
    @JsonKey(name: 'started_at') DateTime? startedAt,
    @JsonKey(name: 'closed_by_id') String? closedById,
    @JsonKey(name: 'closed_at') DateTime? closedAt,
    @JsonKey(name: 'office_name') String? officeName,
    @JsonKey(name: 'started_by_name') String? startedByName,
    @JsonKey(name: 'closed_by_name') String? closedByName,
    int? total,
    int? found,
    int? pending,
    int? variance,
    @JsonKey(name: 'created_at') DateTime? createdAt,
    @JsonKey(name: 'updated_at') DateTime? updatedAt,
  }) = _StockOpnameSessionDto;

  factory StockOpnameSessionDto.fromJson(Map<String, dynamic> json) =>
      _$StockOpnameSessionDtoFromJson(json);
}
