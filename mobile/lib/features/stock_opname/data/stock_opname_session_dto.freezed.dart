// GENERATED CODE - DO NOT MODIFY BY HAND
// coverage:ignore-file
// ignore_for_file: type=lint
// ignore_for_file: unused_element, deprecated_member_use, deprecated_member_use_from_same_package, use_function_type_syntax_for_parameters, unnecessary_const, avoid_init_to_null, invalid_override_different_default_values_named, prefer_expression_function_bodies, annotate_overrides, invalid_annotation_target, unnecessary_question_mark

part of 'stock_opname_session_dto.dart';

// **************************************************************************
// FreezedGenerator
// **************************************************************************

// dart format off
T _$identity<T>(T value) => value;

/// @nodoc
mixin _$StockOpnameSessionDto {

 String get id;@JsonKey(name: 'office_id') String get officeId; String? get name; DateTime? get period; String get status;@JsonKey(name: 'started_by_id') String get startedById;@JsonKey(name: 'started_at') DateTime? get startedAt;@JsonKey(name: 'closed_by_id') String? get closedById;@JsonKey(name: 'closed_at') DateTime? get closedAt;@JsonKey(name: 'office_name') String? get officeName;@JsonKey(name: 'started_by_name') String? get startedByName;@JsonKey(name: 'closed_by_name') String? get closedByName; int? get total; int? get found; int? get pending; int? get variance;@JsonKey(name: 'created_at') DateTime? get createdAt;@JsonKey(name: 'updated_at') DateTime? get updatedAt;
/// Create a copy of StockOpnameSessionDto
/// with the given fields replaced by the non-null parameter values.
@JsonKey(includeFromJson: false, includeToJson: false)
@pragma('vm:prefer-inline')
$StockOpnameSessionDtoCopyWith<StockOpnameSessionDto> get copyWith => _$StockOpnameSessionDtoCopyWithImpl<StockOpnameSessionDto>(this as StockOpnameSessionDto, _$identity);

  /// Serializes this StockOpnameSessionDto to a JSON map.
  Map<String, dynamic> toJson();


@override
bool operator ==(Object other) {
  return identical(this, other) || (other.runtimeType == runtimeType&&other is StockOpnameSessionDto&&(identical(other.id, id) || other.id == id)&&(identical(other.officeId, officeId) || other.officeId == officeId)&&(identical(other.name, name) || other.name == name)&&(identical(other.period, period) || other.period == period)&&(identical(other.status, status) || other.status == status)&&(identical(other.startedById, startedById) || other.startedById == startedById)&&(identical(other.startedAt, startedAt) || other.startedAt == startedAt)&&(identical(other.closedById, closedById) || other.closedById == closedById)&&(identical(other.closedAt, closedAt) || other.closedAt == closedAt)&&(identical(other.officeName, officeName) || other.officeName == officeName)&&(identical(other.startedByName, startedByName) || other.startedByName == startedByName)&&(identical(other.closedByName, closedByName) || other.closedByName == closedByName)&&(identical(other.total, total) || other.total == total)&&(identical(other.found, found) || other.found == found)&&(identical(other.pending, pending) || other.pending == pending)&&(identical(other.variance, variance) || other.variance == variance)&&(identical(other.createdAt, createdAt) || other.createdAt == createdAt)&&(identical(other.updatedAt, updatedAt) || other.updatedAt == updatedAt));
}

@JsonKey(includeFromJson: false, includeToJson: false)
@override
int get hashCode => Object.hash(runtimeType,id,officeId,name,period,status,startedById,startedAt,closedById,closedAt,officeName,startedByName,closedByName,total,found,pending,variance,createdAt,updatedAt);

@override
String toString() {
  return 'StockOpnameSessionDto(id: $id, officeId: $officeId, name: $name, period: $period, status: $status, startedById: $startedById, startedAt: $startedAt, closedById: $closedById, closedAt: $closedAt, officeName: $officeName, startedByName: $startedByName, closedByName: $closedByName, total: $total, found: $found, pending: $pending, variance: $variance, createdAt: $createdAt, updatedAt: $updatedAt)';
}


}

/// @nodoc
abstract mixin class $StockOpnameSessionDtoCopyWith<$Res>  {
  factory $StockOpnameSessionDtoCopyWith(StockOpnameSessionDto value, $Res Function(StockOpnameSessionDto) _then) = _$StockOpnameSessionDtoCopyWithImpl;
@useResult
$Res call({
 String id,@JsonKey(name: 'office_id') String officeId, String? name, DateTime? period, String status,@JsonKey(name: 'started_by_id') String startedById,@JsonKey(name: 'started_at') DateTime? startedAt,@JsonKey(name: 'closed_by_id') String? closedById,@JsonKey(name: 'closed_at') DateTime? closedAt,@JsonKey(name: 'office_name') String? officeName,@JsonKey(name: 'started_by_name') String? startedByName,@JsonKey(name: 'closed_by_name') String? closedByName, int? total, int? found, int? pending, int? variance,@JsonKey(name: 'created_at') DateTime? createdAt,@JsonKey(name: 'updated_at') DateTime? updatedAt
});




}
/// @nodoc
class _$StockOpnameSessionDtoCopyWithImpl<$Res>
    implements $StockOpnameSessionDtoCopyWith<$Res> {
  _$StockOpnameSessionDtoCopyWithImpl(this._self, this._then);

  final StockOpnameSessionDto _self;
  final $Res Function(StockOpnameSessionDto) _then;

/// Create a copy of StockOpnameSessionDto
/// with the given fields replaced by the non-null parameter values.
@pragma('vm:prefer-inline') @override $Res call({Object? id = null,Object? officeId = null,Object? name = freezed,Object? period = freezed,Object? status = null,Object? startedById = null,Object? startedAt = freezed,Object? closedById = freezed,Object? closedAt = freezed,Object? officeName = freezed,Object? startedByName = freezed,Object? closedByName = freezed,Object? total = freezed,Object? found = freezed,Object? pending = freezed,Object? variance = freezed,Object? createdAt = freezed,Object? updatedAt = freezed,}) {
  return _then(_self.copyWith(
id: null == id ? _self.id : id // ignore: cast_nullable_to_non_nullable
as String,officeId: null == officeId ? _self.officeId : officeId // ignore: cast_nullable_to_non_nullable
as String,name: freezed == name ? _self.name : name // ignore: cast_nullable_to_non_nullable
as String?,period: freezed == period ? _self.period : period // ignore: cast_nullable_to_non_nullable
as DateTime?,status: null == status ? _self.status : status // ignore: cast_nullable_to_non_nullable
as String,startedById: null == startedById ? _self.startedById : startedById // ignore: cast_nullable_to_non_nullable
as String,startedAt: freezed == startedAt ? _self.startedAt : startedAt // ignore: cast_nullable_to_non_nullable
as DateTime?,closedById: freezed == closedById ? _self.closedById : closedById // ignore: cast_nullable_to_non_nullable
as String?,closedAt: freezed == closedAt ? _self.closedAt : closedAt // ignore: cast_nullable_to_non_nullable
as DateTime?,officeName: freezed == officeName ? _self.officeName : officeName // ignore: cast_nullable_to_non_nullable
as String?,startedByName: freezed == startedByName ? _self.startedByName : startedByName // ignore: cast_nullable_to_non_nullable
as String?,closedByName: freezed == closedByName ? _self.closedByName : closedByName // ignore: cast_nullable_to_non_nullable
as String?,total: freezed == total ? _self.total : total // ignore: cast_nullable_to_non_nullable
as int?,found: freezed == found ? _self.found : found // ignore: cast_nullable_to_non_nullable
as int?,pending: freezed == pending ? _self.pending : pending // ignore: cast_nullable_to_non_nullable
as int?,variance: freezed == variance ? _self.variance : variance // ignore: cast_nullable_to_non_nullable
as int?,createdAt: freezed == createdAt ? _self.createdAt : createdAt // ignore: cast_nullable_to_non_nullable
as DateTime?,updatedAt: freezed == updatedAt ? _self.updatedAt : updatedAt // ignore: cast_nullable_to_non_nullable
as DateTime?,
  ));
}

}


/// Adds pattern-matching-related methods to [StockOpnameSessionDto].
extension StockOpnameSessionDtoPatterns on StockOpnameSessionDto {
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

@optionalTypeArgs TResult maybeMap<TResult extends Object?>(TResult Function( _StockOpnameSessionDto value)?  $default,{required TResult orElse(),}){
final _that = this;
switch (_that) {
case _StockOpnameSessionDto() when $default != null:
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

@optionalTypeArgs TResult map<TResult extends Object?>(TResult Function( _StockOpnameSessionDto value)  $default,){
final _that = this;
switch (_that) {
case _StockOpnameSessionDto():
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

@optionalTypeArgs TResult? mapOrNull<TResult extends Object?>(TResult? Function( _StockOpnameSessionDto value)?  $default,){
final _that = this;
switch (_that) {
case _StockOpnameSessionDto() when $default != null:
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

@optionalTypeArgs TResult maybeWhen<TResult extends Object?>(TResult Function( String id, @JsonKey(name: 'office_id')  String officeId,  String? name,  DateTime? period,  String status, @JsonKey(name: 'started_by_id')  String startedById, @JsonKey(name: 'started_at')  DateTime? startedAt, @JsonKey(name: 'closed_by_id')  String? closedById, @JsonKey(name: 'closed_at')  DateTime? closedAt, @JsonKey(name: 'office_name')  String? officeName, @JsonKey(name: 'started_by_name')  String? startedByName, @JsonKey(name: 'closed_by_name')  String? closedByName,  int? total,  int? found,  int? pending,  int? variance, @JsonKey(name: 'created_at')  DateTime? createdAt, @JsonKey(name: 'updated_at')  DateTime? updatedAt)?  $default,{required TResult orElse(),}) {final _that = this;
switch (_that) {
case _StockOpnameSessionDto() when $default != null:
return $default(_that.id,_that.officeId,_that.name,_that.period,_that.status,_that.startedById,_that.startedAt,_that.closedById,_that.closedAt,_that.officeName,_that.startedByName,_that.closedByName,_that.total,_that.found,_that.pending,_that.variance,_that.createdAt,_that.updatedAt);case _:
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

@optionalTypeArgs TResult when<TResult extends Object?>(TResult Function( String id, @JsonKey(name: 'office_id')  String officeId,  String? name,  DateTime? period,  String status, @JsonKey(name: 'started_by_id')  String startedById, @JsonKey(name: 'started_at')  DateTime? startedAt, @JsonKey(name: 'closed_by_id')  String? closedById, @JsonKey(name: 'closed_at')  DateTime? closedAt, @JsonKey(name: 'office_name')  String? officeName, @JsonKey(name: 'started_by_name')  String? startedByName, @JsonKey(name: 'closed_by_name')  String? closedByName,  int? total,  int? found,  int? pending,  int? variance, @JsonKey(name: 'created_at')  DateTime? createdAt, @JsonKey(name: 'updated_at')  DateTime? updatedAt)  $default,) {final _that = this;
switch (_that) {
case _StockOpnameSessionDto():
return $default(_that.id,_that.officeId,_that.name,_that.period,_that.status,_that.startedById,_that.startedAt,_that.closedById,_that.closedAt,_that.officeName,_that.startedByName,_that.closedByName,_that.total,_that.found,_that.pending,_that.variance,_that.createdAt,_that.updatedAt);case _:
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

@optionalTypeArgs TResult? whenOrNull<TResult extends Object?>(TResult? Function( String id, @JsonKey(name: 'office_id')  String officeId,  String? name,  DateTime? period,  String status, @JsonKey(name: 'started_by_id')  String startedById, @JsonKey(name: 'started_at')  DateTime? startedAt, @JsonKey(name: 'closed_by_id')  String? closedById, @JsonKey(name: 'closed_at')  DateTime? closedAt, @JsonKey(name: 'office_name')  String? officeName, @JsonKey(name: 'started_by_name')  String? startedByName, @JsonKey(name: 'closed_by_name')  String? closedByName,  int? total,  int? found,  int? pending,  int? variance, @JsonKey(name: 'created_at')  DateTime? createdAt, @JsonKey(name: 'updated_at')  DateTime? updatedAt)?  $default,) {final _that = this;
switch (_that) {
case _StockOpnameSessionDto() when $default != null:
return $default(_that.id,_that.officeId,_that.name,_that.period,_that.status,_that.startedById,_that.startedAt,_that.closedById,_that.closedAt,_that.officeName,_that.startedByName,_that.closedByName,_that.total,_that.found,_that.pending,_that.variance,_that.createdAt,_that.updatedAt);case _:
  return null;

}
}

}

/// @nodoc
@JsonSerializable()

class _StockOpnameSessionDto implements StockOpnameSessionDto {
  const _StockOpnameSessionDto({required this.id, @JsonKey(name: 'office_id') required this.officeId, this.name, this.period, required this.status, @JsonKey(name: 'started_by_id') required this.startedById, @JsonKey(name: 'started_at') this.startedAt, @JsonKey(name: 'closed_by_id') this.closedById, @JsonKey(name: 'closed_at') this.closedAt, @JsonKey(name: 'office_name') this.officeName, @JsonKey(name: 'started_by_name') this.startedByName, @JsonKey(name: 'closed_by_name') this.closedByName, this.total, this.found, this.pending, this.variance, @JsonKey(name: 'created_at') this.createdAt, @JsonKey(name: 'updated_at') this.updatedAt});
  factory _StockOpnameSessionDto.fromJson(Map<String, dynamic> json) => _$StockOpnameSessionDtoFromJson(json);

@override final  String id;
@override@JsonKey(name: 'office_id') final  String officeId;
@override final  String? name;
@override final  DateTime? period;
@override final  String status;
@override@JsonKey(name: 'started_by_id') final  String startedById;
@override@JsonKey(name: 'started_at') final  DateTime? startedAt;
@override@JsonKey(name: 'closed_by_id') final  String? closedById;
@override@JsonKey(name: 'closed_at') final  DateTime? closedAt;
@override@JsonKey(name: 'office_name') final  String? officeName;
@override@JsonKey(name: 'started_by_name') final  String? startedByName;
@override@JsonKey(name: 'closed_by_name') final  String? closedByName;
@override final  int? total;
@override final  int? found;
@override final  int? pending;
@override final  int? variance;
@override@JsonKey(name: 'created_at') final  DateTime? createdAt;
@override@JsonKey(name: 'updated_at') final  DateTime? updatedAt;

/// Create a copy of StockOpnameSessionDto
/// with the given fields replaced by the non-null parameter values.
@override @JsonKey(includeFromJson: false, includeToJson: false)
@pragma('vm:prefer-inline')
_$StockOpnameSessionDtoCopyWith<_StockOpnameSessionDto> get copyWith => __$StockOpnameSessionDtoCopyWithImpl<_StockOpnameSessionDto>(this, _$identity);

@override
Map<String, dynamic> toJson() {
  return _$StockOpnameSessionDtoToJson(this, );
}

@override
bool operator ==(Object other) {
  return identical(this, other) || (other.runtimeType == runtimeType&&other is _StockOpnameSessionDto&&(identical(other.id, id) || other.id == id)&&(identical(other.officeId, officeId) || other.officeId == officeId)&&(identical(other.name, name) || other.name == name)&&(identical(other.period, period) || other.period == period)&&(identical(other.status, status) || other.status == status)&&(identical(other.startedById, startedById) || other.startedById == startedById)&&(identical(other.startedAt, startedAt) || other.startedAt == startedAt)&&(identical(other.closedById, closedById) || other.closedById == closedById)&&(identical(other.closedAt, closedAt) || other.closedAt == closedAt)&&(identical(other.officeName, officeName) || other.officeName == officeName)&&(identical(other.startedByName, startedByName) || other.startedByName == startedByName)&&(identical(other.closedByName, closedByName) || other.closedByName == closedByName)&&(identical(other.total, total) || other.total == total)&&(identical(other.found, found) || other.found == found)&&(identical(other.pending, pending) || other.pending == pending)&&(identical(other.variance, variance) || other.variance == variance)&&(identical(other.createdAt, createdAt) || other.createdAt == createdAt)&&(identical(other.updatedAt, updatedAt) || other.updatedAt == updatedAt));
}

@JsonKey(includeFromJson: false, includeToJson: false)
@override
int get hashCode => Object.hash(runtimeType,id,officeId,name,period,status,startedById,startedAt,closedById,closedAt,officeName,startedByName,closedByName,total,found,pending,variance,createdAt,updatedAt);

@override
String toString() {
  return 'StockOpnameSessionDto(id: $id, officeId: $officeId, name: $name, period: $period, status: $status, startedById: $startedById, startedAt: $startedAt, closedById: $closedById, closedAt: $closedAt, officeName: $officeName, startedByName: $startedByName, closedByName: $closedByName, total: $total, found: $found, pending: $pending, variance: $variance, createdAt: $createdAt, updatedAt: $updatedAt)';
}


}

/// @nodoc
abstract mixin class _$StockOpnameSessionDtoCopyWith<$Res> implements $StockOpnameSessionDtoCopyWith<$Res> {
  factory _$StockOpnameSessionDtoCopyWith(_StockOpnameSessionDto value, $Res Function(_StockOpnameSessionDto) _then) = __$StockOpnameSessionDtoCopyWithImpl;
@override @useResult
$Res call({
 String id,@JsonKey(name: 'office_id') String officeId, String? name, DateTime? period, String status,@JsonKey(name: 'started_by_id') String startedById,@JsonKey(name: 'started_at') DateTime? startedAt,@JsonKey(name: 'closed_by_id') String? closedById,@JsonKey(name: 'closed_at') DateTime? closedAt,@JsonKey(name: 'office_name') String? officeName,@JsonKey(name: 'started_by_name') String? startedByName,@JsonKey(name: 'closed_by_name') String? closedByName, int? total, int? found, int? pending, int? variance,@JsonKey(name: 'created_at') DateTime? createdAt,@JsonKey(name: 'updated_at') DateTime? updatedAt
});




}
/// @nodoc
class __$StockOpnameSessionDtoCopyWithImpl<$Res>
    implements _$StockOpnameSessionDtoCopyWith<$Res> {
  __$StockOpnameSessionDtoCopyWithImpl(this._self, this._then);

  final _StockOpnameSessionDto _self;
  final $Res Function(_StockOpnameSessionDto) _then;

/// Create a copy of StockOpnameSessionDto
/// with the given fields replaced by the non-null parameter values.
@override @pragma('vm:prefer-inline') $Res call({Object? id = null,Object? officeId = null,Object? name = freezed,Object? period = freezed,Object? status = null,Object? startedById = null,Object? startedAt = freezed,Object? closedById = freezed,Object? closedAt = freezed,Object? officeName = freezed,Object? startedByName = freezed,Object? closedByName = freezed,Object? total = freezed,Object? found = freezed,Object? pending = freezed,Object? variance = freezed,Object? createdAt = freezed,Object? updatedAt = freezed,}) {
  return _then(_StockOpnameSessionDto(
id: null == id ? _self.id : id // ignore: cast_nullable_to_non_nullable
as String,officeId: null == officeId ? _self.officeId : officeId // ignore: cast_nullable_to_non_nullable
as String,name: freezed == name ? _self.name : name // ignore: cast_nullable_to_non_nullable
as String?,period: freezed == period ? _self.period : period // ignore: cast_nullable_to_non_nullable
as DateTime?,status: null == status ? _self.status : status // ignore: cast_nullable_to_non_nullable
as String,startedById: null == startedById ? _self.startedById : startedById // ignore: cast_nullable_to_non_nullable
as String,startedAt: freezed == startedAt ? _self.startedAt : startedAt // ignore: cast_nullable_to_non_nullable
as DateTime?,closedById: freezed == closedById ? _self.closedById : closedById // ignore: cast_nullable_to_non_nullable
as String?,closedAt: freezed == closedAt ? _self.closedAt : closedAt // ignore: cast_nullable_to_non_nullable
as DateTime?,officeName: freezed == officeName ? _self.officeName : officeName // ignore: cast_nullable_to_non_nullable
as String?,startedByName: freezed == startedByName ? _self.startedByName : startedByName // ignore: cast_nullable_to_non_nullable
as String?,closedByName: freezed == closedByName ? _self.closedByName : closedByName // ignore: cast_nullable_to_non_nullable
as String?,total: freezed == total ? _self.total : total // ignore: cast_nullable_to_non_nullable
as int?,found: freezed == found ? _self.found : found // ignore: cast_nullable_to_non_nullable
as int?,pending: freezed == pending ? _self.pending : pending // ignore: cast_nullable_to_non_nullable
as int?,variance: freezed == variance ? _self.variance : variance // ignore: cast_nullable_to_non_nullable
as int?,createdAt: freezed == createdAt ? _self.createdAt : createdAt // ignore: cast_nullable_to_non_nullable
as DateTime?,updatedAt: freezed == updatedAt ? _self.updatedAt : updatedAt // ignore: cast_nullable_to_non_nullable
as DateTime?,
  ));
}


}

// dart format on
