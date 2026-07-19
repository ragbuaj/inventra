import 'package:freezed_annotation/freezed_annotation.dart';

part 'asset_dto.freezed.dart';
part 'asset_dto.g.dart';

/// Seluruh kunci properti skema `Asset` openapi.yaml. Dipakai repository untuk
/// mendeteksi field yang TIDAK dikirim backend (dihapus field-permission
/// masking) — berbeda dari field yang dikirim bernilai null.
const List<String> assetSchemaKeys = <String>[
  'id',
  'asset_tag',
  'name',
  'category_id',
  'office_id',
  'brand_id',
  'model_id',
  'room_id',
  'unit_id',
  'vendor_id',
  'current_holder_employee_id',
  'created_by_id',
  'status',
  'asset_class',
  'serial_number',
  'purchase_date',
  'purchase_cost',
  'book_value',
  'accumulated_depreciation',
  'salvage_value',
  'impairment_loss',
  'po_number',
  'funding_source',
  'warranty_expiry',
  'capitalized',
  'depreciation_method',
  'useful_life_months',
  'fiscal_group',
  'fiscal_life_months',
  'acquisition_bast_no',
  'excluded_from_valuation',
  'valuation_exclusion_reason',
  'notes',
  'created_at',
  'updated_at',
];

/// `Asset` openapi.yaml — respons `GET /assets/by-tag/{tag}`.
///
/// SEMUA field nullable: field-permission masking backend bisa menghapus kunci
/// apa pun dari respons per peran (authz.FilterEntity), jadi klien tidak boleh
/// mengasumsikan kehadiran field mana pun. Nilai finansial adalah string
/// desimal (IDR) sesuai kontrak.
@freezed
abstract class AssetDto with _$AssetDto {
  const factory AssetDto({
    String? id,
    @JsonKey(name: 'asset_tag') String? assetTag,
    String? name,
    @JsonKey(name: 'category_id') String? categoryId,
    @JsonKey(name: 'office_id') String? officeId,
    @JsonKey(name: 'brand_id') String? brandId,
    @JsonKey(name: 'model_id') String? modelId,
    @JsonKey(name: 'room_id') String? roomId,
    @JsonKey(name: 'unit_id') String? unitId,
    @JsonKey(name: 'vendor_id') String? vendorId,
    @JsonKey(name: 'current_holder_employee_id')
    String? currentHolderEmployeeId,
    @JsonKey(name: 'created_by_id') String? createdById,
    String? status,
    @JsonKey(name: 'asset_class') String? assetClass,
    @JsonKey(name: 'serial_number') String? serialNumber,
    @JsonKey(name: 'purchase_date') String? purchaseDate,
    @JsonKey(name: 'purchase_cost') String? purchaseCost,
    @JsonKey(name: 'book_value') String? bookValue,
    @JsonKey(name: 'accumulated_depreciation') String? accumulatedDepreciation,
    @JsonKey(name: 'salvage_value') String? salvageValue,
    @JsonKey(name: 'impairment_loss') String? impairmentLoss,
    @JsonKey(name: 'po_number') String? poNumber,
    @JsonKey(name: 'funding_source') String? fundingSource,
    @JsonKey(name: 'warranty_expiry') String? warrantyExpiry,
    bool? capitalized,
    @JsonKey(name: 'depreciation_method') String? depreciationMethod,
    @JsonKey(name: 'useful_life_months') int? usefulLifeMonths,
    @JsonKey(name: 'fiscal_group') String? fiscalGroup,
    @JsonKey(name: 'fiscal_life_months') int? fiscalLifeMonths,
    @JsonKey(name: 'acquisition_bast_no') String? acquisitionBastNo,
    @JsonKey(name: 'excluded_from_valuation') bool? excludedFromValuation,
    @JsonKey(name: 'valuation_exclusion_reason')
    String? valuationExclusionReason,
    String? notes,
    @JsonKey(name: 'created_at') DateTime? createdAt,
    @JsonKey(name: 'updated_at') DateTime? updatedAt,
  }) = _AssetDto;

  factory AssetDto.fromJson(Map<String, dynamic> json) =>
      _$AssetDtoFromJson(json);
}
