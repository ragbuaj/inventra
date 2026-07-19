// GENERATED CODE - DO NOT MODIFY BY HAND
// coverage:ignore-file
// ignore_for_file: type=lint
// ignore_for_file: unused_element, deprecated_member_use, deprecated_member_use_from_same_package, use_function_type_syntax_for_parameters, unnecessary_const, avoid_init_to_null, invalid_override_different_default_values_named, prefer_expression_function_bodies, annotate_overrides, invalid_annotation_target, unnecessary_question_mark

part of 'asset_dto.dart';

// **************************************************************************
// FreezedGenerator
// **************************************************************************

// dart format off
T _$identity<T>(T value) => value;

/// @nodoc
mixin _$AssetDto {

 String? get id;@JsonKey(name: 'asset_tag') String? get assetTag; String? get name;@JsonKey(name: 'category_id') String? get categoryId;@JsonKey(name: 'office_id') String? get officeId;@JsonKey(name: 'brand_id') String? get brandId;@JsonKey(name: 'model_id') String? get modelId;@JsonKey(name: 'room_id') String? get roomId;@JsonKey(name: 'unit_id') String? get unitId;@JsonKey(name: 'vendor_id') String? get vendorId;@JsonKey(name: 'current_holder_employee_id') String? get currentHolderEmployeeId;@JsonKey(name: 'created_by_id') String? get createdById; String? get status;@JsonKey(name: 'asset_class') String? get assetClass;@JsonKey(name: 'serial_number') String? get serialNumber;@JsonKey(name: 'purchase_date') String? get purchaseDate;@JsonKey(name: 'purchase_cost') String? get purchaseCost;@JsonKey(name: 'book_value') String? get bookValue;@JsonKey(name: 'accumulated_depreciation') String? get accumulatedDepreciation;@JsonKey(name: 'salvage_value') String? get salvageValue;@JsonKey(name: 'impairment_loss') String? get impairmentLoss;@JsonKey(name: 'po_number') String? get poNumber;@JsonKey(name: 'funding_source') String? get fundingSource;@JsonKey(name: 'warranty_expiry') String? get warrantyExpiry; bool? get capitalized;@JsonKey(name: 'depreciation_method') String? get depreciationMethod;@JsonKey(name: 'useful_life_months') int? get usefulLifeMonths;@JsonKey(name: 'fiscal_group') String? get fiscalGroup;@JsonKey(name: 'fiscal_life_months') int? get fiscalLifeMonths;@JsonKey(name: 'acquisition_bast_no') String? get acquisitionBastNo;@JsonKey(name: 'excluded_from_valuation') bool? get excludedFromValuation;@JsonKey(name: 'valuation_exclusion_reason') String? get valuationExclusionReason; String? get notes;@JsonKey(name: 'created_at') DateTime? get createdAt;@JsonKey(name: 'updated_at') DateTime? get updatedAt;
/// Create a copy of AssetDto
/// with the given fields replaced by the non-null parameter values.
@JsonKey(includeFromJson: false, includeToJson: false)
@pragma('vm:prefer-inline')
$AssetDtoCopyWith<AssetDto> get copyWith => _$AssetDtoCopyWithImpl<AssetDto>(this as AssetDto, _$identity);

  /// Serializes this AssetDto to a JSON map.
  Map<String, dynamic> toJson();


@override
bool operator ==(Object other) {
  return identical(this, other) || (other.runtimeType == runtimeType&&other is AssetDto&&(identical(other.id, id) || other.id == id)&&(identical(other.assetTag, assetTag) || other.assetTag == assetTag)&&(identical(other.name, name) || other.name == name)&&(identical(other.categoryId, categoryId) || other.categoryId == categoryId)&&(identical(other.officeId, officeId) || other.officeId == officeId)&&(identical(other.brandId, brandId) || other.brandId == brandId)&&(identical(other.modelId, modelId) || other.modelId == modelId)&&(identical(other.roomId, roomId) || other.roomId == roomId)&&(identical(other.unitId, unitId) || other.unitId == unitId)&&(identical(other.vendorId, vendorId) || other.vendorId == vendorId)&&(identical(other.currentHolderEmployeeId, currentHolderEmployeeId) || other.currentHolderEmployeeId == currentHolderEmployeeId)&&(identical(other.createdById, createdById) || other.createdById == createdById)&&(identical(other.status, status) || other.status == status)&&(identical(other.assetClass, assetClass) || other.assetClass == assetClass)&&(identical(other.serialNumber, serialNumber) || other.serialNumber == serialNumber)&&(identical(other.purchaseDate, purchaseDate) || other.purchaseDate == purchaseDate)&&(identical(other.purchaseCost, purchaseCost) || other.purchaseCost == purchaseCost)&&(identical(other.bookValue, bookValue) || other.bookValue == bookValue)&&(identical(other.accumulatedDepreciation, accumulatedDepreciation) || other.accumulatedDepreciation == accumulatedDepreciation)&&(identical(other.salvageValue, salvageValue) || other.salvageValue == salvageValue)&&(identical(other.impairmentLoss, impairmentLoss) || other.impairmentLoss == impairmentLoss)&&(identical(other.poNumber, poNumber) || other.poNumber == poNumber)&&(identical(other.fundingSource, fundingSource) || other.fundingSource == fundingSource)&&(identical(other.warrantyExpiry, warrantyExpiry) || other.warrantyExpiry == warrantyExpiry)&&(identical(other.capitalized, capitalized) || other.capitalized == capitalized)&&(identical(other.depreciationMethod, depreciationMethod) || other.depreciationMethod == depreciationMethod)&&(identical(other.usefulLifeMonths, usefulLifeMonths) || other.usefulLifeMonths == usefulLifeMonths)&&(identical(other.fiscalGroup, fiscalGroup) || other.fiscalGroup == fiscalGroup)&&(identical(other.fiscalLifeMonths, fiscalLifeMonths) || other.fiscalLifeMonths == fiscalLifeMonths)&&(identical(other.acquisitionBastNo, acquisitionBastNo) || other.acquisitionBastNo == acquisitionBastNo)&&(identical(other.excludedFromValuation, excludedFromValuation) || other.excludedFromValuation == excludedFromValuation)&&(identical(other.valuationExclusionReason, valuationExclusionReason) || other.valuationExclusionReason == valuationExclusionReason)&&(identical(other.notes, notes) || other.notes == notes)&&(identical(other.createdAt, createdAt) || other.createdAt == createdAt)&&(identical(other.updatedAt, updatedAt) || other.updatedAt == updatedAt));
}

@JsonKey(includeFromJson: false, includeToJson: false)
@override
int get hashCode => Object.hashAll([runtimeType,id,assetTag,name,categoryId,officeId,brandId,modelId,roomId,unitId,vendorId,currentHolderEmployeeId,createdById,status,assetClass,serialNumber,purchaseDate,purchaseCost,bookValue,accumulatedDepreciation,salvageValue,impairmentLoss,poNumber,fundingSource,warrantyExpiry,capitalized,depreciationMethod,usefulLifeMonths,fiscalGroup,fiscalLifeMonths,acquisitionBastNo,excludedFromValuation,valuationExclusionReason,notes,createdAt,updatedAt]);

@override
String toString() {
  return 'AssetDto(id: $id, assetTag: $assetTag, name: $name, categoryId: $categoryId, officeId: $officeId, brandId: $brandId, modelId: $modelId, roomId: $roomId, unitId: $unitId, vendorId: $vendorId, currentHolderEmployeeId: $currentHolderEmployeeId, createdById: $createdById, status: $status, assetClass: $assetClass, serialNumber: $serialNumber, purchaseDate: $purchaseDate, purchaseCost: $purchaseCost, bookValue: $bookValue, accumulatedDepreciation: $accumulatedDepreciation, salvageValue: $salvageValue, impairmentLoss: $impairmentLoss, poNumber: $poNumber, fundingSource: $fundingSource, warrantyExpiry: $warrantyExpiry, capitalized: $capitalized, depreciationMethod: $depreciationMethod, usefulLifeMonths: $usefulLifeMonths, fiscalGroup: $fiscalGroup, fiscalLifeMonths: $fiscalLifeMonths, acquisitionBastNo: $acquisitionBastNo, excludedFromValuation: $excludedFromValuation, valuationExclusionReason: $valuationExclusionReason, notes: $notes, createdAt: $createdAt, updatedAt: $updatedAt)';
}


}

/// @nodoc
abstract mixin class $AssetDtoCopyWith<$Res>  {
  factory $AssetDtoCopyWith(AssetDto value, $Res Function(AssetDto) _then) = _$AssetDtoCopyWithImpl;
@useResult
$Res call({
 String? id,@JsonKey(name: 'asset_tag') String? assetTag, String? name,@JsonKey(name: 'category_id') String? categoryId,@JsonKey(name: 'office_id') String? officeId,@JsonKey(name: 'brand_id') String? brandId,@JsonKey(name: 'model_id') String? modelId,@JsonKey(name: 'room_id') String? roomId,@JsonKey(name: 'unit_id') String? unitId,@JsonKey(name: 'vendor_id') String? vendorId,@JsonKey(name: 'current_holder_employee_id') String? currentHolderEmployeeId,@JsonKey(name: 'created_by_id') String? createdById, String? status,@JsonKey(name: 'asset_class') String? assetClass,@JsonKey(name: 'serial_number') String? serialNumber,@JsonKey(name: 'purchase_date') String? purchaseDate,@JsonKey(name: 'purchase_cost') String? purchaseCost,@JsonKey(name: 'book_value') String? bookValue,@JsonKey(name: 'accumulated_depreciation') String? accumulatedDepreciation,@JsonKey(name: 'salvage_value') String? salvageValue,@JsonKey(name: 'impairment_loss') String? impairmentLoss,@JsonKey(name: 'po_number') String? poNumber,@JsonKey(name: 'funding_source') String? fundingSource,@JsonKey(name: 'warranty_expiry') String? warrantyExpiry, bool? capitalized,@JsonKey(name: 'depreciation_method') String? depreciationMethod,@JsonKey(name: 'useful_life_months') int? usefulLifeMonths,@JsonKey(name: 'fiscal_group') String? fiscalGroup,@JsonKey(name: 'fiscal_life_months') int? fiscalLifeMonths,@JsonKey(name: 'acquisition_bast_no') String? acquisitionBastNo,@JsonKey(name: 'excluded_from_valuation') bool? excludedFromValuation,@JsonKey(name: 'valuation_exclusion_reason') String? valuationExclusionReason, String? notes,@JsonKey(name: 'created_at') DateTime? createdAt,@JsonKey(name: 'updated_at') DateTime? updatedAt
});




}
/// @nodoc
class _$AssetDtoCopyWithImpl<$Res>
    implements $AssetDtoCopyWith<$Res> {
  _$AssetDtoCopyWithImpl(this._self, this._then);

  final AssetDto _self;
  final $Res Function(AssetDto) _then;

/// Create a copy of AssetDto
/// with the given fields replaced by the non-null parameter values.
@pragma('vm:prefer-inline') @override $Res call({Object? id = freezed,Object? assetTag = freezed,Object? name = freezed,Object? categoryId = freezed,Object? officeId = freezed,Object? brandId = freezed,Object? modelId = freezed,Object? roomId = freezed,Object? unitId = freezed,Object? vendorId = freezed,Object? currentHolderEmployeeId = freezed,Object? createdById = freezed,Object? status = freezed,Object? assetClass = freezed,Object? serialNumber = freezed,Object? purchaseDate = freezed,Object? purchaseCost = freezed,Object? bookValue = freezed,Object? accumulatedDepreciation = freezed,Object? salvageValue = freezed,Object? impairmentLoss = freezed,Object? poNumber = freezed,Object? fundingSource = freezed,Object? warrantyExpiry = freezed,Object? capitalized = freezed,Object? depreciationMethod = freezed,Object? usefulLifeMonths = freezed,Object? fiscalGroup = freezed,Object? fiscalLifeMonths = freezed,Object? acquisitionBastNo = freezed,Object? excludedFromValuation = freezed,Object? valuationExclusionReason = freezed,Object? notes = freezed,Object? createdAt = freezed,Object? updatedAt = freezed,}) {
  return _then(_self.copyWith(
id: freezed == id ? _self.id : id // ignore: cast_nullable_to_non_nullable
as String?,assetTag: freezed == assetTag ? _self.assetTag : assetTag // ignore: cast_nullable_to_non_nullable
as String?,name: freezed == name ? _self.name : name // ignore: cast_nullable_to_non_nullable
as String?,categoryId: freezed == categoryId ? _self.categoryId : categoryId // ignore: cast_nullable_to_non_nullable
as String?,officeId: freezed == officeId ? _self.officeId : officeId // ignore: cast_nullable_to_non_nullable
as String?,brandId: freezed == brandId ? _self.brandId : brandId // ignore: cast_nullable_to_non_nullable
as String?,modelId: freezed == modelId ? _self.modelId : modelId // ignore: cast_nullable_to_non_nullable
as String?,roomId: freezed == roomId ? _self.roomId : roomId // ignore: cast_nullable_to_non_nullable
as String?,unitId: freezed == unitId ? _self.unitId : unitId // ignore: cast_nullable_to_non_nullable
as String?,vendorId: freezed == vendorId ? _self.vendorId : vendorId // ignore: cast_nullable_to_non_nullable
as String?,currentHolderEmployeeId: freezed == currentHolderEmployeeId ? _self.currentHolderEmployeeId : currentHolderEmployeeId // ignore: cast_nullable_to_non_nullable
as String?,createdById: freezed == createdById ? _self.createdById : createdById // ignore: cast_nullable_to_non_nullable
as String?,status: freezed == status ? _self.status : status // ignore: cast_nullable_to_non_nullable
as String?,assetClass: freezed == assetClass ? _self.assetClass : assetClass // ignore: cast_nullable_to_non_nullable
as String?,serialNumber: freezed == serialNumber ? _self.serialNumber : serialNumber // ignore: cast_nullable_to_non_nullable
as String?,purchaseDate: freezed == purchaseDate ? _self.purchaseDate : purchaseDate // ignore: cast_nullable_to_non_nullable
as String?,purchaseCost: freezed == purchaseCost ? _self.purchaseCost : purchaseCost // ignore: cast_nullable_to_non_nullable
as String?,bookValue: freezed == bookValue ? _self.bookValue : bookValue // ignore: cast_nullable_to_non_nullable
as String?,accumulatedDepreciation: freezed == accumulatedDepreciation ? _self.accumulatedDepreciation : accumulatedDepreciation // ignore: cast_nullable_to_non_nullable
as String?,salvageValue: freezed == salvageValue ? _self.salvageValue : salvageValue // ignore: cast_nullable_to_non_nullable
as String?,impairmentLoss: freezed == impairmentLoss ? _self.impairmentLoss : impairmentLoss // ignore: cast_nullable_to_non_nullable
as String?,poNumber: freezed == poNumber ? _self.poNumber : poNumber // ignore: cast_nullable_to_non_nullable
as String?,fundingSource: freezed == fundingSource ? _self.fundingSource : fundingSource // ignore: cast_nullable_to_non_nullable
as String?,warrantyExpiry: freezed == warrantyExpiry ? _self.warrantyExpiry : warrantyExpiry // ignore: cast_nullable_to_non_nullable
as String?,capitalized: freezed == capitalized ? _self.capitalized : capitalized // ignore: cast_nullable_to_non_nullable
as bool?,depreciationMethod: freezed == depreciationMethod ? _self.depreciationMethod : depreciationMethod // ignore: cast_nullable_to_non_nullable
as String?,usefulLifeMonths: freezed == usefulLifeMonths ? _self.usefulLifeMonths : usefulLifeMonths // ignore: cast_nullable_to_non_nullable
as int?,fiscalGroup: freezed == fiscalGroup ? _self.fiscalGroup : fiscalGroup // ignore: cast_nullable_to_non_nullable
as String?,fiscalLifeMonths: freezed == fiscalLifeMonths ? _self.fiscalLifeMonths : fiscalLifeMonths // ignore: cast_nullable_to_non_nullable
as int?,acquisitionBastNo: freezed == acquisitionBastNo ? _self.acquisitionBastNo : acquisitionBastNo // ignore: cast_nullable_to_non_nullable
as String?,excludedFromValuation: freezed == excludedFromValuation ? _self.excludedFromValuation : excludedFromValuation // ignore: cast_nullable_to_non_nullable
as bool?,valuationExclusionReason: freezed == valuationExclusionReason ? _self.valuationExclusionReason : valuationExclusionReason // ignore: cast_nullable_to_non_nullable
as String?,notes: freezed == notes ? _self.notes : notes // ignore: cast_nullable_to_non_nullable
as String?,createdAt: freezed == createdAt ? _self.createdAt : createdAt // ignore: cast_nullable_to_non_nullable
as DateTime?,updatedAt: freezed == updatedAt ? _self.updatedAt : updatedAt // ignore: cast_nullable_to_non_nullable
as DateTime?,
  ));
}

}


/// Adds pattern-matching-related methods to [AssetDto].
extension AssetDtoPatterns on AssetDto {
/// A variant of `map` that fallback to returning `orElse`.
///
/// It is equivalent to doing:
/// ```dart
/// switch (sealedClass) {
///   case final Subclass value:
///     return ...;
///   case _:
///     return orElse();
/// }
/// ```

@optionalTypeArgs TResult maybeMap<TResult extends Object?>(TResult Function( _AssetDto value)?  $default,{required TResult orElse(),}){
final _that = this;
switch (_that) {
case _AssetDto() when $default != null:
return $default(_that);case _:
  return orElse();

}
}
/// A `switch`-like method, using callbacks.
///
/// Callbacks receives the raw object, upcasted.
/// It is equivalent to doing:
/// ```dart
/// switch (sealedClass) {
///   case final Subclass value:
///     return ...;
///   case final Subclass2 value:
///     return ...;
/// }
/// ```

@optionalTypeArgs TResult map<TResult extends Object?>(TResult Function( _AssetDto value)  $default,){
final _that = this;
switch (_that) {
case _AssetDto():
return $default(_that);case _:
  throw StateError('Unexpected subclass');

}
}
/// A variant of `map` that fallback to returning `null`.
///
/// It is equivalent to doing:
/// ```dart
/// switch (sealedClass) {
///   case final Subclass value:
///     return ...;
///   case _:
///     return null;
/// }
/// ```

@optionalTypeArgs TResult? mapOrNull<TResult extends Object?>(TResult? Function( _AssetDto value)?  $default,){
final _that = this;
switch (_that) {
case _AssetDto() when $default != null:
return $default(_that);case _:
  return null;

}
}
/// A variant of `when` that fallback to an `orElse` callback.
///
/// It is equivalent to doing:
/// ```dart
/// switch (sealedClass) {
///   case Subclass(:final field):
///     return ...;
///   case _:
///     return orElse();
/// }
/// ```

@optionalTypeArgs TResult maybeWhen<TResult extends Object?>(TResult Function( String? id, @JsonKey(name: 'asset_tag')  String? assetTag,  String? name, @JsonKey(name: 'category_id')  String? categoryId, @JsonKey(name: 'office_id')  String? officeId, @JsonKey(name: 'brand_id')  String? brandId, @JsonKey(name: 'model_id')  String? modelId, @JsonKey(name: 'room_id')  String? roomId, @JsonKey(name: 'unit_id')  String? unitId, @JsonKey(name: 'vendor_id')  String? vendorId, @JsonKey(name: 'current_holder_employee_id')  String? currentHolderEmployeeId, @JsonKey(name: 'created_by_id')  String? createdById,  String? status, @JsonKey(name: 'asset_class')  String? assetClass, @JsonKey(name: 'serial_number')  String? serialNumber, @JsonKey(name: 'purchase_date')  String? purchaseDate, @JsonKey(name: 'purchase_cost')  String? purchaseCost, @JsonKey(name: 'book_value')  String? bookValue, @JsonKey(name: 'accumulated_depreciation')  String? accumulatedDepreciation, @JsonKey(name: 'salvage_value')  String? salvageValue, @JsonKey(name: 'impairment_loss')  String? impairmentLoss, @JsonKey(name: 'po_number')  String? poNumber, @JsonKey(name: 'funding_source')  String? fundingSource, @JsonKey(name: 'warranty_expiry')  String? warrantyExpiry,  bool? capitalized, @JsonKey(name: 'depreciation_method')  String? depreciationMethod, @JsonKey(name: 'useful_life_months')  int? usefulLifeMonths, @JsonKey(name: 'fiscal_group')  String? fiscalGroup, @JsonKey(name: 'fiscal_life_months')  int? fiscalLifeMonths, @JsonKey(name: 'acquisition_bast_no')  String? acquisitionBastNo, @JsonKey(name: 'excluded_from_valuation')  bool? excludedFromValuation, @JsonKey(name: 'valuation_exclusion_reason')  String? valuationExclusionReason,  String? notes, @JsonKey(name: 'created_at')  DateTime? createdAt, @JsonKey(name: 'updated_at')  DateTime? updatedAt)?  $default,{required TResult orElse(),}) {final _that = this;
switch (_that) {
case _AssetDto() when $default != null:
return $default(_that.id,_that.assetTag,_that.name,_that.categoryId,_that.officeId,_that.brandId,_that.modelId,_that.roomId,_that.unitId,_that.vendorId,_that.currentHolderEmployeeId,_that.createdById,_that.status,_that.assetClass,_that.serialNumber,_that.purchaseDate,_that.purchaseCost,_that.bookValue,_that.accumulatedDepreciation,_that.salvageValue,_that.impairmentLoss,_that.poNumber,_that.fundingSource,_that.warrantyExpiry,_that.capitalized,_that.depreciationMethod,_that.usefulLifeMonths,_that.fiscalGroup,_that.fiscalLifeMonths,_that.acquisitionBastNo,_that.excludedFromValuation,_that.valuationExclusionReason,_that.notes,_that.createdAt,_that.updatedAt);case _:
  return orElse();

}
}
/// A `switch`-like method, using callbacks.
///
/// As opposed to `map`, this offers destructuring.
/// It is equivalent to doing:
/// ```dart
/// switch (sealedClass) {
///   case Subclass(:final field):
///     return ...;
///   case Subclass2(:final field2):
///     return ...;
/// }
/// ```

@optionalTypeArgs TResult when<TResult extends Object?>(TResult Function( String? id, @JsonKey(name: 'asset_tag')  String? assetTag,  String? name, @JsonKey(name: 'category_id')  String? categoryId, @JsonKey(name: 'office_id')  String? officeId, @JsonKey(name: 'brand_id')  String? brandId, @JsonKey(name: 'model_id')  String? modelId, @JsonKey(name: 'room_id')  String? roomId, @JsonKey(name: 'unit_id')  String? unitId, @JsonKey(name: 'vendor_id')  String? vendorId, @JsonKey(name: 'current_holder_employee_id')  String? currentHolderEmployeeId, @JsonKey(name: 'created_by_id')  String? createdById,  String? status, @JsonKey(name: 'asset_class')  String? assetClass, @JsonKey(name: 'serial_number')  String? serialNumber, @JsonKey(name: 'purchase_date')  String? purchaseDate, @JsonKey(name: 'purchase_cost')  String? purchaseCost, @JsonKey(name: 'book_value')  String? bookValue, @JsonKey(name: 'accumulated_depreciation')  String? accumulatedDepreciation, @JsonKey(name: 'salvage_value')  String? salvageValue, @JsonKey(name: 'impairment_loss')  String? impairmentLoss, @JsonKey(name: 'po_number')  String? poNumber, @JsonKey(name: 'funding_source')  String? fundingSource, @JsonKey(name: 'warranty_expiry')  String? warrantyExpiry,  bool? capitalized, @JsonKey(name: 'depreciation_method')  String? depreciationMethod, @JsonKey(name: 'useful_life_months')  int? usefulLifeMonths, @JsonKey(name: 'fiscal_group')  String? fiscalGroup, @JsonKey(name: 'fiscal_life_months')  int? fiscalLifeMonths, @JsonKey(name: 'acquisition_bast_no')  String? acquisitionBastNo, @JsonKey(name: 'excluded_from_valuation')  bool? excludedFromValuation, @JsonKey(name: 'valuation_exclusion_reason')  String? valuationExclusionReason,  String? notes, @JsonKey(name: 'created_at')  DateTime? createdAt, @JsonKey(name: 'updated_at')  DateTime? updatedAt)  $default,) {final _that = this;
switch (_that) {
case _AssetDto():
return $default(_that.id,_that.assetTag,_that.name,_that.categoryId,_that.officeId,_that.brandId,_that.modelId,_that.roomId,_that.unitId,_that.vendorId,_that.currentHolderEmployeeId,_that.createdById,_that.status,_that.assetClass,_that.serialNumber,_that.purchaseDate,_that.purchaseCost,_that.bookValue,_that.accumulatedDepreciation,_that.salvageValue,_that.impairmentLoss,_that.poNumber,_that.fundingSource,_that.warrantyExpiry,_that.capitalized,_that.depreciationMethod,_that.usefulLifeMonths,_that.fiscalGroup,_that.fiscalLifeMonths,_that.acquisitionBastNo,_that.excludedFromValuation,_that.valuationExclusionReason,_that.notes,_that.createdAt,_that.updatedAt);case _:
  throw StateError('Unexpected subclass');

}
}
/// A variant of `when` that fallback to returning `null`
///
/// It is equivalent to doing:
/// ```dart
/// switch (sealedClass) {
///   case Subclass(:final field):
///     return ...;
///   case _:
///     return null;
/// }
/// ```

@optionalTypeArgs TResult? whenOrNull<TResult extends Object?>(TResult? Function( String? id, @JsonKey(name: 'asset_tag')  String? assetTag,  String? name, @JsonKey(name: 'category_id')  String? categoryId, @JsonKey(name: 'office_id')  String? officeId, @JsonKey(name: 'brand_id')  String? brandId, @JsonKey(name: 'model_id')  String? modelId, @JsonKey(name: 'room_id')  String? roomId, @JsonKey(name: 'unit_id')  String? unitId, @JsonKey(name: 'vendor_id')  String? vendorId, @JsonKey(name: 'current_holder_employee_id')  String? currentHolderEmployeeId, @JsonKey(name: 'created_by_id')  String? createdById,  String? status, @JsonKey(name: 'asset_class')  String? assetClass, @JsonKey(name: 'serial_number')  String? serialNumber, @JsonKey(name: 'purchase_date')  String? purchaseDate, @JsonKey(name: 'purchase_cost')  String? purchaseCost, @JsonKey(name: 'book_value')  String? bookValue, @JsonKey(name: 'accumulated_depreciation')  String? accumulatedDepreciation, @JsonKey(name: 'salvage_value')  String? salvageValue, @JsonKey(name: 'impairment_loss')  String? impairmentLoss, @JsonKey(name: 'po_number')  String? poNumber, @JsonKey(name: 'funding_source')  String? fundingSource, @JsonKey(name: 'warranty_expiry')  String? warrantyExpiry,  bool? capitalized, @JsonKey(name: 'depreciation_method')  String? depreciationMethod, @JsonKey(name: 'useful_life_months')  int? usefulLifeMonths, @JsonKey(name: 'fiscal_group')  String? fiscalGroup, @JsonKey(name: 'fiscal_life_months')  int? fiscalLifeMonths, @JsonKey(name: 'acquisition_bast_no')  String? acquisitionBastNo, @JsonKey(name: 'excluded_from_valuation')  bool? excludedFromValuation, @JsonKey(name: 'valuation_exclusion_reason')  String? valuationExclusionReason,  String? notes, @JsonKey(name: 'created_at')  DateTime? createdAt, @JsonKey(name: 'updated_at')  DateTime? updatedAt)?  $default,) {final _that = this;
switch (_that) {
case _AssetDto() when $default != null:
return $default(_that.id,_that.assetTag,_that.name,_that.categoryId,_that.officeId,_that.brandId,_that.modelId,_that.roomId,_that.unitId,_that.vendorId,_that.currentHolderEmployeeId,_that.createdById,_that.status,_that.assetClass,_that.serialNumber,_that.purchaseDate,_that.purchaseCost,_that.bookValue,_that.accumulatedDepreciation,_that.salvageValue,_that.impairmentLoss,_that.poNumber,_that.fundingSource,_that.warrantyExpiry,_that.capitalized,_that.depreciationMethod,_that.usefulLifeMonths,_that.fiscalGroup,_that.fiscalLifeMonths,_that.acquisitionBastNo,_that.excludedFromValuation,_that.valuationExclusionReason,_that.notes,_that.createdAt,_that.updatedAt);case _:
  return null;

}
}

}

/// @nodoc
@JsonSerializable()

class _AssetDto implements AssetDto {
  const _AssetDto({this.id, @JsonKey(name: 'asset_tag') this.assetTag, this.name, @JsonKey(name: 'category_id') this.categoryId, @JsonKey(name: 'office_id') this.officeId, @JsonKey(name: 'brand_id') this.brandId, @JsonKey(name: 'model_id') this.modelId, @JsonKey(name: 'room_id') this.roomId, @JsonKey(name: 'unit_id') this.unitId, @JsonKey(name: 'vendor_id') this.vendorId, @JsonKey(name: 'current_holder_employee_id') this.currentHolderEmployeeId, @JsonKey(name: 'created_by_id') this.createdById, this.status, @JsonKey(name: 'asset_class') this.assetClass, @JsonKey(name: 'serial_number') this.serialNumber, @JsonKey(name: 'purchase_date') this.purchaseDate, @JsonKey(name: 'purchase_cost') this.purchaseCost, @JsonKey(name: 'book_value') this.bookValue, @JsonKey(name: 'accumulated_depreciation') this.accumulatedDepreciation, @JsonKey(name: 'salvage_value') this.salvageValue, @JsonKey(name: 'impairment_loss') this.impairmentLoss, @JsonKey(name: 'po_number') this.poNumber, @JsonKey(name: 'funding_source') this.fundingSource, @JsonKey(name: 'warranty_expiry') this.warrantyExpiry, this.capitalized, @JsonKey(name: 'depreciation_method') this.depreciationMethod, @JsonKey(name: 'useful_life_months') this.usefulLifeMonths, @JsonKey(name: 'fiscal_group') this.fiscalGroup, @JsonKey(name: 'fiscal_life_months') this.fiscalLifeMonths, @JsonKey(name: 'acquisition_bast_no') this.acquisitionBastNo, @JsonKey(name: 'excluded_from_valuation') this.excludedFromValuation, @JsonKey(name: 'valuation_exclusion_reason') this.valuationExclusionReason, this.notes, @JsonKey(name: 'created_at') this.createdAt, @JsonKey(name: 'updated_at') this.updatedAt});
  factory _AssetDto.fromJson(Map<String, dynamic> json) => _$AssetDtoFromJson(json);

@override final  String? id;
@override@JsonKey(name: 'asset_tag') final  String? assetTag;
@override final  String? name;
@override@JsonKey(name: 'category_id') final  String? categoryId;
@override@JsonKey(name: 'office_id') final  String? officeId;
@override@JsonKey(name: 'brand_id') final  String? brandId;
@override@JsonKey(name: 'model_id') final  String? modelId;
@override@JsonKey(name: 'room_id') final  String? roomId;
@override@JsonKey(name: 'unit_id') final  String? unitId;
@override@JsonKey(name: 'vendor_id') final  String? vendorId;
@override@JsonKey(name: 'current_holder_employee_id') final  String? currentHolderEmployeeId;
@override@JsonKey(name: 'created_by_id') final  String? createdById;
@override final  String? status;
@override@JsonKey(name: 'asset_class') final  String? assetClass;
@override@JsonKey(name: 'serial_number') final  String? serialNumber;
@override@JsonKey(name: 'purchase_date') final  String? purchaseDate;
@override@JsonKey(name: 'purchase_cost') final  String? purchaseCost;
@override@JsonKey(name: 'book_value') final  String? bookValue;
@override@JsonKey(name: 'accumulated_depreciation') final  String? accumulatedDepreciation;
@override@JsonKey(name: 'salvage_value') final  String? salvageValue;
@override@JsonKey(name: 'impairment_loss') final  String? impairmentLoss;
@override@JsonKey(name: 'po_number') final  String? poNumber;
@override@JsonKey(name: 'funding_source') final  String? fundingSource;
@override@JsonKey(name: 'warranty_expiry') final  String? warrantyExpiry;
@override final  bool? capitalized;
@override@JsonKey(name: 'depreciation_method') final  String? depreciationMethod;
@override@JsonKey(name: 'useful_life_months') final  int? usefulLifeMonths;
@override@JsonKey(name: 'fiscal_group') final  String? fiscalGroup;
@override@JsonKey(name: 'fiscal_life_months') final  int? fiscalLifeMonths;
@override@JsonKey(name: 'acquisition_bast_no') final  String? acquisitionBastNo;
@override@JsonKey(name: 'excluded_from_valuation') final  bool? excludedFromValuation;
@override@JsonKey(name: 'valuation_exclusion_reason') final  String? valuationExclusionReason;
@override final  String? notes;
@override@JsonKey(name: 'created_at') final  DateTime? createdAt;
@override@JsonKey(name: 'updated_at') final  DateTime? updatedAt;

/// Create a copy of AssetDto
/// with the given fields replaced by the non-null parameter values.
@override @JsonKey(includeFromJson: false, includeToJson: false)
@pragma('vm:prefer-inline')
_$AssetDtoCopyWith<_AssetDto> get copyWith => __$AssetDtoCopyWithImpl<_AssetDto>(this, _$identity);

@override
Map<String, dynamic> toJson() {
  return _$AssetDtoToJson(this, );
}

@override
bool operator ==(Object other) {
  return identical(this, other) || (other.runtimeType == runtimeType&&other is _AssetDto&&(identical(other.id, id) || other.id == id)&&(identical(other.assetTag, assetTag) || other.assetTag == assetTag)&&(identical(other.name, name) || other.name == name)&&(identical(other.categoryId, categoryId) || other.categoryId == categoryId)&&(identical(other.officeId, officeId) || other.officeId == officeId)&&(identical(other.brandId, brandId) || other.brandId == brandId)&&(identical(other.modelId, modelId) || other.modelId == modelId)&&(identical(other.roomId, roomId) || other.roomId == roomId)&&(identical(other.unitId, unitId) || other.unitId == unitId)&&(identical(other.vendorId, vendorId) || other.vendorId == vendorId)&&(identical(other.currentHolderEmployeeId, currentHolderEmployeeId) || other.currentHolderEmployeeId == currentHolderEmployeeId)&&(identical(other.createdById, createdById) || other.createdById == createdById)&&(identical(other.status, status) || other.status == status)&&(identical(other.assetClass, assetClass) || other.assetClass == assetClass)&&(identical(other.serialNumber, serialNumber) || other.serialNumber == serialNumber)&&(identical(other.purchaseDate, purchaseDate) || other.purchaseDate == purchaseDate)&&(identical(other.purchaseCost, purchaseCost) || other.purchaseCost == purchaseCost)&&(identical(other.bookValue, bookValue) || other.bookValue == bookValue)&&(identical(other.accumulatedDepreciation, accumulatedDepreciation) || other.accumulatedDepreciation == accumulatedDepreciation)&&(identical(other.salvageValue, salvageValue) || other.salvageValue == salvageValue)&&(identical(other.impairmentLoss, impairmentLoss) || other.impairmentLoss == impairmentLoss)&&(identical(other.poNumber, poNumber) || other.poNumber == poNumber)&&(identical(other.fundingSource, fundingSource) || other.fundingSource == fundingSource)&&(identical(other.warrantyExpiry, warrantyExpiry) || other.warrantyExpiry == warrantyExpiry)&&(identical(other.capitalized, capitalized) || other.capitalized == capitalized)&&(identical(other.depreciationMethod, depreciationMethod) || other.depreciationMethod == depreciationMethod)&&(identical(other.usefulLifeMonths, usefulLifeMonths) || other.usefulLifeMonths == usefulLifeMonths)&&(identical(other.fiscalGroup, fiscalGroup) || other.fiscalGroup == fiscalGroup)&&(identical(other.fiscalLifeMonths, fiscalLifeMonths) || other.fiscalLifeMonths == fiscalLifeMonths)&&(identical(other.acquisitionBastNo, acquisitionBastNo) || other.acquisitionBastNo == acquisitionBastNo)&&(identical(other.excludedFromValuation, excludedFromValuation) || other.excludedFromValuation == excludedFromValuation)&&(identical(other.valuationExclusionReason, valuationExclusionReason) || other.valuationExclusionReason == valuationExclusionReason)&&(identical(other.notes, notes) || other.notes == notes)&&(identical(other.createdAt, createdAt) || other.createdAt == createdAt)&&(identical(other.updatedAt, updatedAt) || other.updatedAt == updatedAt));
}

@JsonKey(includeFromJson: false, includeToJson: false)
@override
int get hashCode => Object.hashAll([runtimeType,id,assetTag,name,categoryId,officeId,brandId,modelId,roomId,unitId,vendorId,currentHolderEmployeeId,createdById,status,assetClass,serialNumber,purchaseDate,purchaseCost,bookValue,accumulatedDepreciation,salvageValue,impairmentLoss,poNumber,fundingSource,warrantyExpiry,capitalized,depreciationMethod,usefulLifeMonths,fiscalGroup,fiscalLifeMonths,acquisitionBastNo,excludedFromValuation,valuationExclusionReason,notes,createdAt,updatedAt]);

@override
String toString() {
  return 'AssetDto(id: $id, assetTag: $assetTag, name: $name, categoryId: $categoryId, officeId: $officeId, brandId: $brandId, modelId: $modelId, roomId: $roomId, unitId: $unitId, vendorId: $vendorId, currentHolderEmployeeId: $currentHolderEmployeeId, createdById: $createdById, status: $status, assetClass: $assetClass, serialNumber: $serialNumber, purchaseDate: $purchaseDate, purchaseCost: $purchaseCost, bookValue: $bookValue, accumulatedDepreciation: $accumulatedDepreciation, salvageValue: $salvageValue, impairmentLoss: $impairmentLoss, poNumber: $poNumber, fundingSource: $fundingSource, warrantyExpiry: $warrantyExpiry, capitalized: $capitalized, depreciationMethod: $depreciationMethod, usefulLifeMonths: $usefulLifeMonths, fiscalGroup: $fiscalGroup, fiscalLifeMonths: $fiscalLifeMonths, acquisitionBastNo: $acquisitionBastNo, excludedFromValuation: $excludedFromValuation, valuationExclusionReason: $valuationExclusionReason, notes: $notes, createdAt: $createdAt, updatedAt: $updatedAt)';
}


}

/// @nodoc
abstract mixin class _$AssetDtoCopyWith<$Res> implements $AssetDtoCopyWith<$Res> {
  factory _$AssetDtoCopyWith(_AssetDto value, $Res Function(_AssetDto) _then) = __$AssetDtoCopyWithImpl;
@override @useResult
$Res call({
 String? id,@JsonKey(name: 'asset_tag') String? assetTag, String? name,@JsonKey(name: 'category_id') String? categoryId,@JsonKey(name: 'office_id') String? officeId,@JsonKey(name: 'brand_id') String? brandId,@JsonKey(name: 'model_id') String? modelId,@JsonKey(name: 'room_id') String? roomId,@JsonKey(name: 'unit_id') String? unitId,@JsonKey(name: 'vendor_id') String? vendorId,@JsonKey(name: 'current_holder_employee_id') String? currentHolderEmployeeId,@JsonKey(name: 'created_by_id') String? createdById, String? status,@JsonKey(name: 'asset_class') String? assetClass,@JsonKey(name: 'serial_number') String? serialNumber,@JsonKey(name: 'purchase_date') String? purchaseDate,@JsonKey(name: 'purchase_cost') String? purchaseCost,@JsonKey(name: 'book_value') String? bookValue,@JsonKey(name: 'accumulated_depreciation') String? accumulatedDepreciation,@JsonKey(name: 'salvage_value') String? salvageValue,@JsonKey(name: 'impairment_loss') String? impairmentLoss,@JsonKey(name: 'po_number') String? poNumber,@JsonKey(name: 'funding_source') String? fundingSource,@JsonKey(name: 'warranty_expiry') String? warrantyExpiry, bool? capitalized,@JsonKey(name: 'depreciation_method') String? depreciationMethod,@JsonKey(name: 'useful_life_months') int? usefulLifeMonths,@JsonKey(name: 'fiscal_group') String? fiscalGroup,@JsonKey(name: 'fiscal_life_months') int? fiscalLifeMonths,@JsonKey(name: 'acquisition_bast_no') String? acquisitionBastNo,@JsonKey(name: 'excluded_from_valuation') bool? excludedFromValuation,@JsonKey(name: 'valuation_exclusion_reason') String? valuationExclusionReason, String? notes,@JsonKey(name: 'created_at') DateTime? createdAt,@JsonKey(name: 'updated_at') DateTime? updatedAt
});




}
/// @nodoc
class __$AssetDtoCopyWithImpl<$Res>
    implements _$AssetDtoCopyWith<$Res> {
  __$AssetDtoCopyWithImpl(this._self, this._then);

  final _AssetDto _self;
  final $Res Function(_AssetDto) _then;

/// Create a copy of AssetDto
/// with the given fields replaced by the non-null parameter values.
@override @pragma('vm:prefer-inline') $Res call({Object? id = freezed,Object? assetTag = freezed,Object? name = freezed,Object? categoryId = freezed,Object? officeId = freezed,Object? brandId = freezed,Object? modelId = freezed,Object? roomId = freezed,Object? unitId = freezed,Object? vendorId = freezed,Object? currentHolderEmployeeId = freezed,Object? createdById = freezed,Object? status = freezed,Object? assetClass = freezed,Object? serialNumber = freezed,Object? purchaseDate = freezed,Object? purchaseCost = freezed,Object? bookValue = freezed,Object? accumulatedDepreciation = freezed,Object? salvageValue = freezed,Object? impairmentLoss = freezed,Object? poNumber = freezed,Object? fundingSource = freezed,Object? warrantyExpiry = freezed,Object? capitalized = freezed,Object? depreciationMethod = freezed,Object? usefulLifeMonths = freezed,Object? fiscalGroup = freezed,Object? fiscalLifeMonths = freezed,Object? acquisitionBastNo = freezed,Object? excludedFromValuation = freezed,Object? valuationExclusionReason = freezed,Object? notes = freezed,Object? createdAt = freezed,Object? updatedAt = freezed,}) {
  return _then(_AssetDto(
id: freezed == id ? _self.id : id // ignore: cast_nullable_to_non_nullable
as String?,assetTag: freezed == assetTag ? _self.assetTag : assetTag // ignore: cast_nullable_to_non_nullable
as String?,name: freezed == name ? _self.name : name // ignore: cast_nullable_to_non_nullable
as String?,categoryId: freezed == categoryId ? _self.categoryId : categoryId // ignore: cast_nullable_to_non_nullable
as String?,officeId: freezed == officeId ? _self.officeId : officeId // ignore: cast_nullable_to_non_nullable
as String?,brandId: freezed == brandId ? _self.brandId : brandId // ignore: cast_nullable_to_non_nullable
as String?,modelId: freezed == modelId ? _self.modelId : modelId // ignore: cast_nullable_to_non_nullable
as String?,roomId: freezed == roomId ? _self.roomId : roomId // ignore: cast_nullable_to_non_nullable
as String?,unitId: freezed == unitId ? _self.unitId : unitId // ignore: cast_nullable_to_non_nullable
as String?,vendorId: freezed == vendorId ? _self.vendorId : vendorId // ignore: cast_nullable_to_non_nullable
as String?,currentHolderEmployeeId: freezed == currentHolderEmployeeId ? _self.currentHolderEmployeeId : currentHolderEmployeeId // ignore: cast_nullable_to_non_nullable
as String?,createdById: freezed == createdById ? _self.createdById : createdById // ignore: cast_nullable_to_non_nullable
as String?,status: freezed == status ? _self.status : status // ignore: cast_nullable_to_non_nullable
as String?,assetClass: freezed == assetClass ? _self.assetClass : assetClass // ignore: cast_nullable_to_non_nullable
as String?,serialNumber: freezed == serialNumber ? _self.serialNumber : serialNumber // ignore: cast_nullable_to_non_nullable
as String?,purchaseDate: freezed == purchaseDate ? _self.purchaseDate : purchaseDate // ignore: cast_nullable_to_non_nullable
as String?,purchaseCost: freezed == purchaseCost ? _self.purchaseCost : purchaseCost // ignore: cast_nullable_to_non_nullable
as String?,bookValue: freezed == bookValue ? _self.bookValue : bookValue // ignore: cast_nullable_to_non_nullable
as String?,accumulatedDepreciation: freezed == accumulatedDepreciation ? _self.accumulatedDepreciation : accumulatedDepreciation // ignore: cast_nullable_to_non_nullable
as String?,salvageValue: freezed == salvageValue ? _self.salvageValue : salvageValue // ignore: cast_nullable_to_non_nullable
as String?,impairmentLoss: freezed == impairmentLoss ? _self.impairmentLoss : impairmentLoss // ignore: cast_nullable_to_non_nullable
as String?,poNumber: freezed == poNumber ? _self.poNumber : poNumber // ignore: cast_nullable_to_non_nullable
as String?,fundingSource: freezed == fundingSource ? _self.fundingSource : fundingSource // ignore: cast_nullable_to_non_nullable
as String?,warrantyExpiry: freezed == warrantyExpiry ? _self.warrantyExpiry : warrantyExpiry // ignore: cast_nullable_to_non_nullable
as String?,capitalized: freezed == capitalized ? _self.capitalized : capitalized // ignore: cast_nullable_to_non_nullable
as bool?,depreciationMethod: freezed == depreciationMethod ? _self.depreciationMethod : depreciationMethod // ignore: cast_nullable_to_non_nullable
as String?,usefulLifeMonths: freezed == usefulLifeMonths ? _self.usefulLifeMonths : usefulLifeMonths // ignore: cast_nullable_to_non_nullable
as int?,fiscalGroup: freezed == fiscalGroup ? _self.fiscalGroup : fiscalGroup // ignore: cast_nullable_to_non_nullable
as String?,fiscalLifeMonths: freezed == fiscalLifeMonths ? _self.fiscalLifeMonths : fiscalLifeMonths // ignore: cast_nullable_to_non_nullable
as int?,acquisitionBastNo: freezed == acquisitionBastNo ? _self.acquisitionBastNo : acquisitionBastNo // ignore: cast_nullable_to_non_nullable
as String?,excludedFromValuation: freezed == excludedFromValuation ? _self.excludedFromValuation : excludedFromValuation // ignore: cast_nullable_to_non_nullable
as bool?,valuationExclusionReason: freezed == valuationExclusionReason ? _self.valuationExclusionReason : valuationExclusionReason // ignore: cast_nullable_to_non_nullable
as String?,notes: freezed == notes ? _self.notes : notes // ignore: cast_nullable_to_non_nullable
as String?,createdAt: freezed == createdAt ? _self.createdAt : createdAt // ignore: cast_nullable_to_non_nullable
as DateTime?,updatedAt: freezed == updatedAt ? _self.updatedAt : updatedAt // ignore: cast_nullable_to_non_nullable
as DateTime?,
  ));
}


}

// dart format on
