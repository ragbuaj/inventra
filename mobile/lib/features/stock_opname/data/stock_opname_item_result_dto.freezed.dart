// GENERATED CODE - DO NOT MODIFY BY HAND
// coverage:ignore-file
// ignore_for_file: type=lint
// ignore_for_file: unused_element, deprecated_member_use, deprecated_member_use_from_same_package, use_function_type_syntax_for_parameters, unnecessary_const, avoid_init_to_null, invalid_override_different_default_values_named, prefer_expression_function_bodies, annotate_overrides, invalid_annotation_target, unnecessary_question_mark

part of 'stock_opname_item_result_dto.dart';

// **************************************************************************
// FreezedGenerator
// **************************************************************************

// dart format off
T _$identity<T>(T value) => value;

/// @nodoc
mixin _$StockOpnameItemResultDto {

 String get id;@JsonKey(name: 'session_id') String get sessionId;@JsonKey(name: 'asset_id') String get assetId; bool get expected; String get result; String? get note;@JsonKey(name: 'counted_at') DateTime? get countedAt;
/// Create a copy of StockOpnameItemResultDto
/// with the given fields replaced by the non-null parameter values.
@JsonKey(includeFromJson: false, includeToJson: false)
@pragma('vm:prefer-inline')
$StockOpnameItemResultDtoCopyWith<StockOpnameItemResultDto> get copyWith => _$StockOpnameItemResultDtoCopyWithImpl<StockOpnameItemResultDto>(this as StockOpnameItemResultDto, _$identity);

  /// Serializes this StockOpnameItemResultDto to a JSON map.
  Map<String, dynamic> toJson();


@override
bool operator ==(Object other) {
  return identical(this, other) || (other.runtimeType == runtimeType&&other is StockOpnameItemResultDto&&(identical(other.id, id) || other.id == id)&&(identical(other.sessionId, sessionId) || other.sessionId == sessionId)&&(identical(other.assetId, assetId) || other.assetId == assetId)&&(identical(other.expected, expected) || other.expected == expected)&&(identical(other.result, result) || other.result == result)&&(identical(other.note, note) || other.note == note)&&(identical(other.countedAt, countedAt) || other.countedAt == countedAt));
}

@JsonKey(includeFromJson: false, includeToJson: false)
@override
int get hashCode => Object.hash(runtimeType,id,sessionId,assetId,expected,result,note,countedAt);

@override
String toString() {
  return 'StockOpnameItemResultDto(id: $id, sessionId: $sessionId, assetId: $assetId, expected: $expected, result: $result, note: $note, countedAt: $countedAt)';
}


}

/// @nodoc
abstract mixin class $StockOpnameItemResultDtoCopyWith<$Res>  {
  factory $StockOpnameItemResultDtoCopyWith(StockOpnameItemResultDto value, $Res Function(StockOpnameItemResultDto) _then) = _$StockOpnameItemResultDtoCopyWithImpl;
@useResult
$Res call({
 String id,@JsonKey(name: 'session_id') String sessionId,@JsonKey(name: 'asset_id') String assetId, bool expected, String result, String? note,@JsonKey(name: 'counted_at') DateTime? countedAt
});




}
/// @nodoc
class _$StockOpnameItemResultDtoCopyWithImpl<$Res>
    implements $StockOpnameItemResultDtoCopyWith<$Res> {
  _$StockOpnameItemResultDtoCopyWithImpl(this._self, this._then);

  final StockOpnameItemResultDto _self;
  final $Res Function(StockOpnameItemResultDto) _then;

/// Create a copy of StockOpnameItemResultDto
/// with the given fields replaced by the non-null parameter values.
@pragma('vm:prefer-inline') @override $Res call({Object? id = null,Object? sessionId = null,Object? assetId = null,Object? expected = null,Object? result = null,Object? note = freezed,Object? countedAt = freezed,}) {
  return _then(_self.copyWith(
id: null == id ? _self.id : id // ignore: cast_nullable_to_non_nullable
as String,sessionId: null == sessionId ? _self.sessionId : sessionId // ignore: cast_nullable_to_non_nullable
as String,assetId: null == assetId ? _self.assetId : assetId // ignore: cast_nullable_to_non_nullable
as String,expected: null == expected ? _self.expected : expected // ignore: cast_nullable_to_non_nullable
as bool,result: null == result ? _self.result : result // ignore: cast_nullable_to_non_nullable
as String,note: freezed == note ? _self.note : note // ignore: cast_nullable_to_non_nullable
as String?,countedAt: freezed == countedAt ? _self.countedAt : countedAt // ignore: cast_nullable_to_non_nullable
as DateTime?,
  ));
}

}


/// Adds pattern-matching-related methods to [StockOpnameItemResultDto].
extension StockOpnameItemResultDtoPatterns on StockOpnameItemResultDto {
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

@optionalTypeArgs TResult maybeMap<TResult extends Object?>(TResult Function( _StockOpnameItemResultDto value)?  $default,{required TResult orElse(),}){
final _that = this;
switch (_that) {
case _StockOpnameItemResultDto() when $default != null:
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

@optionalTypeArgs TResult map<TResult extends Object?>(TResult Function( _StockOpnameItemResultDto value)  $default,){
final _that = this;
switch (_that) {
case _StockOpnameItemResultDto():
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

@optionalTypeArgs TResult? mapOrNull<TResult extends Object?>(TResult? Function( _StockOpnameItemResultDto value)?  $default,){
final _that = this;
switch (_that) {
case _StockOpnameItemResultDto() when $default != null:
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

@optionalTypeArgs TResult maybeWhen<TResult extends Object?>(TResult Function( String id, @JsonKey(name: 'session_id')  String sessionId, @JsonKey(name: 'asset_id')  String assetId,  bool expected,  String result,  String? note, @JsonKey(name: 'counted_at')  DateTime? countedAt)?  $default,{required TResult orElse(),}) {final _that = this;
switch (_that) {
case _StockOpnameItemResultDto() when $default != null:
return $default(_that.id,_that.sessionId,_that.assetId,_that.expected,_that.result,_that.note,_that.countedAt);case _:
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

@optionalTypeArgs TResult when<TResult extends Object?>(TResult Function( String id, @JsonKey(name: 'session_id')  String sessionId, @JsonKey(name: 'asset_id')  String assetId,  bool expected,  String result,  String? note, @JsonKey(name: 'counted_at')  DateTime? countedAt)  $default,) {final _that = this;
switch (_that) {
case _StockOpnameItemResultDto():
return $default(_that.id,_that.sessionId,_that.assetId,_that.expected,_that.result,_that.note,_that.countedAt);case _:
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

@optionalTypeArgs TResult? whenOrNull<TResult extends Object?>(TResult? Function( String id, @JsonKey(name: 'session_id')  String sessionId, @JsonKey(name: 'asset_id')  String assetId,  bool expected,  String result,  String? note, @JsonKey(name: 'counted_at')  DateTime? countedAt)?  $default,) {final _that = this;
switch (_that) {
case _StockOpnameItemResultDto() when $default != null:
return $default(_that.id,_that.sessionId,_that.assetId,_that.expected,_that.result,_that.note,_that.countedAt);case _:
  return null;

}
}

}

/// @nodoc
@JsonSerializable()

class _StockOpnameItemResultDto implements StockOpnameItemResultDto {
  const _StockOpnameItemResultDto({required this.id, @JsonKey(name: 'session_id') required this.sessionId, @JsonKey(name: 'asset_id') required this.assetId, required this.expected, required this.result, this.note, @JsonKey(name: 'counted_at') this.countedAt});
  factory _StockOpnameItemResultDto.fromJson(Map<String, dynamic> json) => _$StockOpnameItemResultDtoFromJson(json);

@override final  String id;
@override@JsonKey(name: 'session_id') final  String sessionId;
@override@JsonKey(name: 'asset_id') final  String assetId;
@override final  bool expected;
@override final  String result;
@override final  String? note;
@override@JsonKey(name: 'counted_at') final  DateTime? countedAt;

/// Create a copy of StockOpnameItemResultDto
/// with the given fields replaced by the non-null parameter values.
@override @JsonKey(includeFromJson: false, includeToJson: false)
@pragma('vm:prefer-inline')
_$StockOpnameItemResultDtoCopyWith<_StockOpnameItemResultDto> get copyWith => __$StockOpnameItemResultDtoCopyWithImpl<_StockOpnameItemResultDto>(this, _$identity);

@override
Map<String, dynamic> toJson() {
  return _$StockOpnameItemResultDtoToJson(this, );
}

@override
bool operator ==(Object other) {
  return identical(this, other) || (other.runtimeType == runtimeType&&other is _StockOpnameItemResultDto&&(identical(other.id, id) || other.id == id)&&(identical(other.sessionId, sessionId) || other.sessionId == sessionId)&&(identical(other.assetId, assetId) || other.assetId == assetId)&&(identical(other.expected, expected) || other.expected == expected)&&(identical(other.result, result) || other.result == result)&&(identical(other.note, note) || other.note == note)&&(identical(other.countedAt, countedAt) || other.countedAt == countedAt));
}

@JsonKey(includeFromJson: false, includeToJson: false)
@override
int get hashCode => Object.hash(runtimeType,id,sessionId,assetId,expected,result,note,countedAt);

@override
String toString() {
  return 'StockOpnameItemResultDto(id: $id, sessionId: $sessionId, assetId: $assetId, expected: $expected, result: $result, note: $note, countedAt: $countedAt)';
}


}

/// @nodoc
abstract mixin class _$StockOpnameItemResultDtoCopyWith<$Res> implements $StockOpnameItemResultDtoCopyWith<$Res> {
  factory _$StockOpnameItemResultDtoCopyWith(_StockOpnameItemResultDto value, $Res Function(_StockOpnameItemResultDto) _then) = __$StockOpnameItemResultDtoCopyWithImpl;
@override @useResult
$Res call({
 String id,@JsonKey(name: 'session_id') String sessionId,@JsonKey(name: 'asset_id') String assetId, bool expected, String result, String? note,@JsonKey(name: 'counted_at') DateTime? countedAt
});




}
/// @nodoc
class __$StockOpnameItemResultDtoCopyWithImpl<$Res>
    implements _$StockOpnameItemResultDtoCopyWith<$Res> {
  __$StockOpnameItemResultDtoCopyWithImpl(this._self, this._then);

  final _StockOpnameItemResultDto _self;
  final $Res Function(_StockOpnameItemResultDto) _then;

/// Create a copy of StockOpnameItemResultDto
/// with the given fields replaced by the non-null parameter values.
@override @pragma('vm:prefer-inline') $Res call({Object? id = null,Object? sessionId = null,Object? assetId = null,Object? expected = null,Object? result = null,Object? note = freezed,Object? countedAt = freezed,}) {
  return _then(_StockOpnameItemResultDto(
id: null == id ? _self.id : id // ignore: cast_nullable_to_non_nullable
as String,sessionId: null == sessionId ? _self.sessionId : sessionId // ignore: cast_nullable_to_non_nullable
as String,assetId: null == assetId ? _self.assetId : assetId // ignore: cast_nullable_to_non_nullable
as String,expected: null == expected ? _self.expected : expected // ignore: cast_nullable_to_non_nullable
as bool,result: null == result ? _self.result : result // ignore: cast_nullable_to_non_nullable
as String,note: freezed == note ? _self.note : note // ignore: cast_nullable_to_non_nullable
as String?,countedAt: freezed == countedAt ? _self.countedAt : countedAt // ignore: cast_nullable_to_non_nullable
as DateTime?,
  ));
}


}

// dart format on
