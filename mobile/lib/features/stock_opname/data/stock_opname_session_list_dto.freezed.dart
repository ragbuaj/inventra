// GENERATED CODE - DO NOT MODIFY BY HAND
// coverage:ignore-file
// ignore_for_file: type=lint
// ignore_for_file: unused_element, deprecated_member_use, deprecated_member_use_from_same_package, use_function_type_syntax_for_parameters, unnecessary_const, avoid_init_to_null, invalid_override_different_default_values_named, prefer_expression_function_bodies, annotate_overrides, invalid_annotation_target, unnecessary_question_mark

part of 'stock_opname_session_list_dto.dart';

// **************************************************************************
// FreezedGenerator
// **************************************************************************

// dart format off
T _$identity<T>(T value) => value;

/// @nodoc
mixin _$StockOpnameSessionListDto {

 List<StockOpnameSessionDto> get data; int get total; int get limit; int get offset;
/// Create a copy of StockOpnameSessionListDto
/// with the given fields replaced by the non-null parameter values.
@JsonKey(includeFromJson: false, includeToJson: false)
@pragma('vm:prefer-inline')
$StockOpnameSessionListDtoCopyWith<StockOpnameSessionListDto> get copyWith => _$StockOpnameSessionListDtoCopyWithImpl<StockOpnameSessionListDto>(this as StockOpnameSessionListDto, _$identity);

  /// Serializes this StockOpnameSessionListDto to a JSON map.
  Map<String, dynamic> toJson();


@override
bool operator ==(Object other) {
  return identical(this, other) || (other.runtimeType == runtimeType&&other is StockOpnameSessionListDto&&const DeepCollectionEquality().equals(other.data, data)&&(identical(other.total, total) || other.total == total)&&(identical(other.limit, limit) || other.limit == limit)&&(identical(other.offset, offset) || other.offset == offset));
}

@JsonKey(includeFromJson: false, includeToJson: false)
@override
int get hashCode => Object.hash(runtimeType,const DeepCollectionEquality().hash(data),total,limit,offset);

@override
String toString() {
  return 'StockOpnameSessionListDto(data: $data, total: $total, limit: $limit, offset: $offset)';
}


}

/// @nodoc
abstract mixin class $StockOpnameSessionListDtoCopyWith<$Res>  {
  factory $StockOpnameSessionListDtoCopyWith(StockOpnameSessionListDto value, $Res Function(StockOpnameSessionListDto) _then) = _$StockOpnameSessionListDtoCopyWithImpl;
@useResult
$Res call({
 List<StockOpnameSessionDto> data, int total, int limit, int offset
});




}
/// @nodoc
class _$StockOpnameSessionListDtoCopyWithImpl<$Res>
    implements $StockOpnameSessionListDtoCopyWith<$Res> {
  _$StockOpnameSessionListDtoCopyWithImpl(this._self, this._then);

  final StockOpnameSessionListDto _self;
  final $Res Function(StockOpnameSessionListDto) _then;

/// Create a copy of StockOpnameSessionListDto
/// with the given fields replaced by the non-null parameter values.
@pragma('vm:prefer-inline') @override $Res call({Object? data = null,Object? total = null,Object? limit = null,Object? offset = null,}) {
  return _then(_self.copyWith(
data: null == data ? _self.data : data // ignore: cast_nullable_to_non_nullable
as List<StockOpnameSessionDto>,total: null == total ? _self.total : total // ignore: cast_nullable_to_non_nullable
as int,limit: null == limit ? _self.limit : limit // ignore: cast_nullable_to_non_nullable
as int,offset: null == offset ? _self.offset : offset // ignore: cast_nullable_to_non_nullable
as int,
  ));
}

}


/// Adds pattern-matching-related methods to [StockOpnameSessionListDto].
extension StockOpnameSessionListDtoPatterns on StockOpnameSessionListDto {
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

@optionalTypeArgs TResult maybeMap<TResult extends Object?>(TResult Function( _StockOpnameSessionListDto value)?  $default,{required TResult orElse(),}){
final _that = this;
switch (_that) {
case _StockOpnameSessionListDto() when $default != null:
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

@optionalTypeArgs TResult map<TResult extends Object?>(TResult Function( _StockOpnameSessionListDto value)  $default,){
final _that = this;
switch (_that) {
case _StockOpnameSessionListDto():
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

@optionalTypeArgs TResult? mapOrNull<TResult extends Object?>(TResult? Function( _StockOpnameSessionListDto value)?  $default,){
final _that = this;
switch (_that) {
case _StockOpnameSessionListDto() when $default != null:
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

@optionalTypeArgs TResult maybeWhen<TResult extends Object?>(TResult Function( List<StockOpnameSessionDto> data,  int total,  int limit,  int offset)?  $default,{required TResult orElse(),}) {final _that = this;
switch (_that) {
case _StockOpnameSessionListDto() when $default != null:
return $default(_that.data,_that.total,_that.limit,_that.offset);case _:
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

@optionalTypeArgs TResult when<TResult extends Object?>(TResult Function( List<StockOpnameSessionDto> data,  int total,  int limit,  int offset)  $default,) {final _that = this;
switch (_that) {
case _StockOpnameSessionListDto():
return $default(_that.data,_that.total,_that.limit,_that.offset);case _:
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

@optionalTypeArgs TResult? whenOrNull<TResult extends Object?>(TResult? Function( List<StockOpnameSessionDto> data,  int total,  int limit,  int offset)?  $default,) {final _that = this;
switch (_that) {
case _StockOpnameSessionListDto() when $default != null:
return $default(_that.data,_that.total,_that.limit,_that.offset);case _:
  return null;

}
}

}

/// @nodoc
@JsonSerializable()

class _StockOpnameSessionListDto implements StockOpnameSessionListDto {
  const _StockOpnameSessionListDto({required final  List<StockOpnameSessionDto> data, required this.total, required this.limit, required this.offset}): _data = data;
  factory _StockOpnameSessionListDto.fromJson(Map<String, dynamic> json) => _$StockOpnameSessionListDtoFromJson(json);

 final  List<StockOpnameSessionDto> _data;
@override List<StockOpnameSessionDto> get data {
  if (_data is EqualUnmodifiableListView) return _data;
  // ignore: implicit_dynamic_type
  return EqualUnmodifiableListView(_data);
}

@override final  int total;
@override final  int limit;
@override final  int offset;

/// Create a copy of StockOpnameSessionListDto
/// with the given fields replaced by the non-null parameter values.
@override @JsonKey(includeFromJson: false, includeToJson: false)
@pragma('vm:prefer-inline')
_$StockOpnameSessionListDtoCopyWith<_StockOpnameSessionListDto> get copyWith => __$StockOpnameSessionListDtoCopyWithImpl<_StockOpnameSessionListDto>(this, _$identity);

@override
Map<String, dynamic> toJson() {
  return _$StockOpnameSessionListDtoToJson(this, );
}

@override
bool operator ==(Object other) {
  return identical(this, other) || (other.runtimeType == runtimeType&&other is _StockOpnameSessionListDto&&const DeepCollectionEquality().equals(other._data, _data)&&(identical(other.total, total) || other.total == total)&&(identical(other.limit, limit) || other.limit == limit)&&(identical(other.offset, offset) || other.offset == offset));
}

@JsonKey(includeFromJson: false, includeToJson: false)
@override
int get hashCode => Object.hash(runtimeType,const DeepCollectionEquality().hash(_data),total,limit,offset);

@override
String toString() {
  return 'StockOpnameSessionListDto(data: $data, total: $total, limit: $limit, offset: $offset)';
}


}

/// @nodoc
abstract mixin class _$StockOpnameSessionListDtoCopyWith<$Res> implements $StockOpnameSessionListDtoCopyWith<$Res> {
  factory _$StockOpnameSessionListDtoCopyWith(_StockOpnameSessionListDto value, $Res Function(_StockOpnameSessionListDto) _then) = __$StockOpnameSessionListDtoCopyWithImpl;
@override @useResult
$Res call({
 List<StockOpnameSessionDto> data, int total, int limit, int offset
});




}
/// @nodoc
class __$StockOpnameSessionListDtoCopyWithImpl<$Res>
    implements _$StockOpnameSessionListDtoCopyWith<$Res> {
  __$StockOpnameSessionListDtoCopyWithImpl(this._self, this._then);

  final _StockOpnameSessionListDto _self;
  final $Res Function(_StockOpnameSessionListDto) _then;

/// Create a copy of StockOpnameSessionListDto
/// with the given fields replaced by the non-null parameter values.
@override @pragma('vm:prefer-inline') $Res call({Object? data = null,Object? total = null,Object? limit = null,Object? offset = null,}) {
  return _then(_StockOpnameSessionListDto(
data: null == data ? _self._data : data // ignore: cast_nullable_to_non_nullable
as List<StockOpnameSessionDto>,total: null == total ? _self.total : total // ignore: cast_nullable_to_non_nullable
as int,limit: null == limit ? _self.limit : limit // ignore: cast_nullable_to_non_nullable
as int,offset: null == offset ? _self.offset : offset // ignore: cast_nullable_to_non_nullable
as int,
  ));
}


}

// dart format on
