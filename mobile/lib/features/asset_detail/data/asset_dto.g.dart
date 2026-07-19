// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'asset_dto.dart';

// **************************************************************************
// JsonSerializableGenerator
// **************************************************************************

_AssetDto _$AssetDtoFromJson(Map<String, dynamic> json) => _AssetDto(
  id: json['id'] as String?,
  assetTag: json['asset_tag'] as String?,
  name: json['name'] as String?,
  categoryId: json['category_id'] as String?,
  officeId: json['office_id'] as String?,
  brandId: json['brand_id'] as String?,
  modelId: json['model_id'] as String?,
  roomId: json['room_id'] as String?,
  unitId: json['unit_id'] as String?,
  vendorId: json['vendor_id'] as String?,
  currentHolderEmployeeId: json['current_holder_employee_id'] as String?,
  createdById: json['created_by_id'] as String?,
  status: json['status'] as String?,
  assetClass: json['asset_class'] as String?,
  serialNumber: json['serial_number'] as String?,
  purchaseDate: json['purchase_date'] as String?,
  purchaseCost: json['purchase_cost'] as String?,
  bookValue: json['book_value'] as String?,
  accumulatedDepreciation: json['accumulated_depreciation'] as String?,
  salvageValue: json['salvage_value'] as String?,
  impairmentLoss: json['impairment_loss'] as String?,
  poNumber: json['po_number'] as String?,
  fundingSource: json['funding_source'] as String?,
  warrantyExpiry: json['warranty_expiry'] as String?,
  capitalized: json['capitalized'] as bool?,
  depreciationMethod: json['depreciation_method'] as String?,
  usefulLifeMonths: (json['useful_life_months'] as num?)?.toInt(),
  fiscalGroup: json['fiscal_group'] as String?,
  fiscalLifeMonths: (json['fiscal_life_months'] as num?)?.toInt(),
  acquisitionBastNo: json['acquisition_bast_no'] as String?,
  excludedFromValuation: json['excluded_from_valuation'] as bool?,
  valuationExclusionReason: json['valuation_exclusion_reason'] as String?,
  notes: json['notes'] as String?,
  createdAt: json['created_at'] == null
      ? null
      : DateTime.parse(json['created_at'] as String),
  updatedAt: json['updated_at'] == null
      ? null
      : DateTime.parse(json['updated_at'] as String),
);

Map<String, dynamic> _$AssetDtoToJson(_AssetDto instance) => <String, dynamic>{
  'id': instance.id,
  'asset_tag': instance.assetTag,
  'name': instance.name,
  'category_id': instance.categoryId,
  'office_id': instance.officeId,
  'brand_id': instance.brandId,
  'model_id': instance.modelId,
  'room_id': instance.roomId,
  'unit_id': instance.unitId,
  'vendor_id': instance.vendorId,
  'current_holder_employee_id': instance.currentHolderEmployeeId,
  'created_by_id': instance.createdById,
  'status': instance.status,
  'asset_class': instance.assetClass,
  'serial_number': instance.serialNumber,
  'purchase_date': instance.purchaseDate,
  'purchase_cost': instance.purchaseCost,
  'book_value': instance.bookValue,
  'accumulated_depreciation': instance.accumulatedDepreciation,
  'salvage_value': instance.salvageValue,
  'impairment_loss': instance.impairmentLoss,
  'po_number': instance.poNumber,
  'funding_source': instance.fundingSource,
  'warranty_expiry': instance.warrantyExpiry,
  'capitalized': instance.capitalized,
  'depreciation_method': instance.depreciationMethod,
  'useful_life_months': instance.usefulLifeMonths,
  'fiscal_group': instance.fiscalGroup,
  'fiscal_life_months': instance.fiscalLifeMonths,
  'acquisition_bast_no': instance.acquisitionBastNo,
  'excluded_from_valuation': instance.excludedFromValuation,
  'valuation_exclusion_reason': instance.valuationExclusionReason,
  'notes': instance.notes,
  'created_at': instance.createdAt?.toIso8601String(),
  'updated_at': instance.updatedAt?.toIso8601String(),
};
