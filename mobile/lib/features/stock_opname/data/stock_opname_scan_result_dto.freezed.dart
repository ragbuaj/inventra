// GENERATED CODE - DO NOT MODIFY BY HAND
// coverage:ignore-file
// ignore_for_file: type=lint
// ignore_for_file: unused_element, deprecated_member_use, deprecated_member_use_from_same_package, use_function_type_syntax_for_parameters, unnecessary_const, avoid_init_to_null, invalid_override_different_default_values_named, prefer_expression_function_bodies, annotate_overrides, invalid_annotation_target, unnecessary_question_mark

part of 'stock_opname_scan_result_dto.dart';

// **************************************************************************
// FreezedGenerator
// **************************************************************************

// dart format off
T _$identity<T>(T value) => value;

/// @nodoc
mixin _$StockOpnameScanResultDto {

 String get id;@JsonKey(name: 'session_id') String get sessionId;@JsonKey(name: 'asset_id') String get assetId; bool get expected; String get result;
/// Create a copy of StockOpnameScanResultDto
/// with the given fields replaced by the non-null parameter values.
@JsonKey(includeFromJson: false, includeToJson: false)
@pragma('vm:prefer-inline')
$StockOpnameScanResultDtoCopyWith<StockOpnameScanResultDto> get copyWith => _$StockOpnameScanResultDtoCopyWithImpl<StockOpnameScanResultDto>(this as StockOpnameScanResultDto, _$identity);

  /// Serializes this StockOpnameScanResultDto to a JSON map.
  Map<String, dynamic> toJson();


@override
bool operator ==(Object other) {
  return identical(this, other) || (other.runtimeType == runtimeType&&other is StockOpnameScanResultDto&&(identical(other.id, id) || other.id == id)&&(identical(other.sessionId, sessionId) || other.sessionId == sessionId)&&(identical(other.assetId, assetId) || other.assetId == assetId)&&(identical(other.expected, expected) || other.expected == expected)&&(identical(other.result, result) || other.result == result));
}

@JsonKey(includeFromJson: false, includeToJson: false)
@override
int get hashCode => Object.hash(runtimeType,id,sessionId,assetId,expected,result);

@override
String toString() {
  return 'StockOpnameScanResultDto(id: $id, sessionId: $sessionId, assetId: $assetId, expected: $expected, result: $result)';
}


}

/// @nodoc
abstract mixin class $StockOpnameScanResultDtoCopyWith<$Res>  {
  factory $StockOpnameScanResultDtoCopyWith(StockOpnameScanResultDto value, $Res Function(StockOpnameScanResultDto) _then) = _$StockOpnameScanResultDtoCopyWithImpl;
@useResult
$Res call({
 String id,@JsonKey(name: 'session_id') String sessionId,@JsonKey(name: 'asset_id') String assetId, bool expected, String result
});




}
/// @nodoc
class _$StockOpnameScanResultDtoCopyWithImpl<$Res>
    implements $StockOpnameScanResultDtoCopyWith<$Res> {
  _$StockOpnameScanResultDtoCopyWithImpl(this._self, this._then);

  final StockOpnameScanResultDto _self;
  final $Res Function(StockOpnameScanResultDto) _then;

/// Create a copy of StockOpnameScanResultDto
/// with the given fields replaced by the non-null parameter values.
@pragma('vm:prefer-inline') @override $Res call({Object? id = null,Object? sessionId = null,Object? assetId = null,Object? expected = null,Object? result = null,}) {
  return _then(_self.copyWith(
id: null == id ? _self.id : id // ignore: cast_nullable_to_non_nullable
as String,sessionId: null == sessionId ? _self.sessionId : sessionId // ignore: cast_nullable_to_non_nullable
as String,assetId: null == assetId ? _self.assetId : assetId // ignore: cast_nullable_to_non_nullable
as String,expected: null == expected ? _self.expected : expected // ignore: cast_nullable_to_non_nullable
as bool,result: null == result ? _self.result : result // ignore: cast_nullable_to_non_nullable
as String,
  ));
}

}


/// Adds pattern-matching-related methods to [StockOpnameScanResultDto].
extension StockOpnameScanResultDtoPatterns on StockOpnameScanResultDto {
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

@optionalTypeArgs TResult maybeMap<TResult extends Object?>(TResult Function( _StockOpnameScanResultDto value)?  $default,{required TResult orElse(),}){
final _that = this;
switch (_that) {
case _StockOpnameScanResultDto() when $default != null:
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

@optionalTypeArgs TResult map<TResult extends Object?>(TResult Function( _StockOpnameScanResultDto value)  $default,){
final _that = this;
switch (_that) {
case _StockOpnameScanResultDto():
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

@optionalTypeArgs TResult? mapOrNull<TResult extends Object?>(TResult? Function( _StockOpnameScanResultDto value)?  $default,){
final _that = this;
switch (_that) {
case _StockOpnameScanResultDto() when $default != null:
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

@optionalTypeArgs TResult maybeWhen<TResult extends Object?>(TResult Function( String id, @JsonKey(name: 'session_id')  String sessionId, @JsonKey(name: 'asset_id')  String assetId,  bool expected,  String result)?  $default,{required TResult orElse(),}) {final _that = this;
switch (_that) {
case _StockOpnameScanResultDto() when $default != null:
return $default(_that.id,_that.sessionId,_that.assetId,_that.expected,_that.result);case _:
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

@optionalTypeArgs TResult when<TResult extends Object?>(TResult Function( String id, @JsonKey(name: 'session_id')  String sessionId, @JsonKey(name: 'asset_id')  String assetId,  bool expected,  String result)  $default,) {final _that = this;
switch (_that) {
case _StockOpnameScanResultDto():
return $default(_that.id,_that.sessionId,_that.assetId,_that.expected,_that.result);case _:
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

@optionalTypeArgs TResult? whenOrNull<TResult extends Object?>(TResult? Function( String id, @JsonKey(name: 'session_id')  String sessionId, @JsonKey(name: 'asset_id')  String assetId,  bool expected,  String result)?  $default,) {final _that = this;
switch (_that) {
case _StockOpnameScanResultDto() when $default != null:
return $default(_that.id,_that.sessionId,_that.assetId,_that.expected,_that.result);case _:
  return null;

}
}

}

/// @nodoc
@JsonSerializable()

class _StockOpnameScanResultDto implements StockOpnameScanResultDto {
  const _StockOpnameScanResultDto({required this.id, @JsonKey(name: 'session_id') required this.sessionId, @JsonKey(name: 'asset_id') required this.assetId, required this.expected, required this.result});
  factory _StockOpnameScanResultDto.fromJson(Map<String, dynamic> json) => _$StockOpnameScanResultDtoFromJson(json);

@override final  String id;
@override@JsonKey(name: 'session_id') final  String sessionId;
@override@JsonKey(name: 'asset_id') final  String assetId;
@override final  bool expected;
@override final  String result;

/// Create a copy of StockOpnameScanResultDto
/// with the given fields replaced by the non-null parameter values.
@override @JsonKey(includeFromJson: false, includeToJson: false)
@pragma('vm:prefer-inline')
_$StockOpnameScanResultDtoCopyWith<_StockOpnameScanResultDto> get copyWith => __$StockOpnameScanResultDtoCopyWithImpl<_StockOpnameScanResultDto>(this, _$identity);

@override
Map<String, dynamic> toJson() {
  return _$StockOpnameScanResultDtoToJson(this, );
}

@override
bool operator ==(Object other) {
  return identical(this, other) || (other.runtimeType == runtimeType&&other is _StockOpnameScanResultDto&&(identical(other.id, id) || other.id == id)&&(identical(other.sessionId, sessionId) || other.sessionId == sessionId)&&(identical(other.assetId, assetId) || other.assetId == assetId)&&(identical(other.expected, expected) || other.expected == expected)&&(identical(other.result, result) || other.result == result));
}

@JsonKey(includeFromJson: false, includeToJson: false)
@override
int get hashCode => Object.hash(runtimeType,id,sessionId,assetId,expected,result);

@override
String toString() {
  return 'StockOpnameScanResultDto(id: $id, sessionId: $sessionId, assetId: $assetId, expected: $expected, result: $result)';
}


}

/// @nodoc
abstract mixin class _$StockOpnameScanResultDtoCopyWith<$Res> implements $StockOpnameScanResultDtoCopyWith<$Res> {
  factory _$StockOpnameScanResultDtoCopyWith(_StockOpnameScanResultDto value, $Res Function(_StockOpnameScanResultDto) _then) = __$StockOpnameScanResultDtoCopyWithImpl;
@override @useResult
$Res call({
 String id,@JsonKey(name: 'session_id') String sessionId,@JsonKey(name: 'asset_id') String assetId, bool expected, String result
});




}
/// @nodoc
class __$StockOpnameScanResultDtoCopyWithImpl<$Res>
    implements _$StockOpnameScanResultDtoCopyWith<$Res> {
  __$StockOpnameScanResultDtoCopyWithImpl(this._self, this._then);

  final _StockOpnameScanResultDto _self;
  final $Res Function(_StockOpnameScanResultDto) _then;

/// Create a copy of StockOpnameScanResultDto
/// with the given fields replaced by the non-null parameter values.
@override @pragma('vm:prefer-inline') $Res call({Object? id = null,Object? sessionId = null,Object? assetId = null,Object? expected = null,Object? result = null,}) {
  return _then(_StockOpnameScanResultDto(
id: null == id ? _self.id : id // ignore: cast_nullable_to_non_nullable
as String,sessionId: null == sessionId ? _self.sessionId : sessionId // ignore: cast_nullable_to_non_nullable
as String,assetId: null == assetId ? _self.assetId : assetId // ignore: cast_nullable_to_non_nullable
as String,expected: null == expected ? _self.expected : expected // ignore: cast_nullable_to_non_nullable
as bool,result: null == result ? _self.result : result // ignore: cast_nullable_to_non_nullable
as String,
  ));
}


}

// dart format on
