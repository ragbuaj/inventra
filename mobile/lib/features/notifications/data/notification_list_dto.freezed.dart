// GENERATED CODE - DO NOT MODIFY BY HAND
// coverage:ignore-file
// ignore_for_file: type=lint
// ignore_for_file: unused_element, deprecated_member_use, deprecated_member_use_from_same_package, use_function_type_syntax_for_parameters, unnecessary_const, avoid_init_to_null, invalid_override_different_default_values_named, prefer_expression_function_bodies, annotate_overrides, invalid_annotation_target, unnecessary_question_mark

part of 'notification_list_dto.dart';

// **************************************************************************
// FreezedGenerator
// **************************************************************************

// dart format off
T _$identity<T>(T value) => value;

/// @nodoc
mixin _$NotificationListDto {

 List<NotificationDto> get data; int get total; int get limit; int get offset;
/// Create a copy of NotificationListDto
/// with the given fields replaced by the non-null parameter values.
@JsonKey(includeFromJson: false, includeToJson: false)
@pragma('vm:prefer-inline')
$NotificationListDtoCopyWith<NotificationListDto> get copyWith => _$NotificationListDtoCopyWithImpl<NotificationListDto>(this as NotificationListDto, _$identity);

  /// Serializes this NotificationListDto to a JSON map.
  Map<String, dynamic> toJson();


@override
bool operator ==(Object other) {
  return identical(this, other) || (other.runtimeType == runtimeType&&other is NotificationListDto&&const DeepCollectionEquality().equals(other.data, data)&&(identical(other.total, total) || other.total == total)&&(identical(other.limit, limit) || other.limit == limit)&&(identical(other.offset, offset) || other.offset == offset));
}

@JsonKey(includeFromJson: false, includeToJson: false)
@override
int get hashCode => Object.hash(runtimeType,const DeepCollectionEquality().hash(data),total,limit,offset);

@override
String toString() {
  return 'NotificationListDto(data: $data, total: $total, limit: $limit, offset: $offset)';
}


}

/// @nodoc
abstract mixin class $NotificationListDtoCopyWith<$Res>  {
  factory $NotificationListDtoCopyWith(NotificationListDto value, $Res Function(NotificationListDto) _then) = _$NotificationListDtoCopyWithImpl;
@useResult
$Res call({
 List<NotificationDto> data, int total, int limit, int offset
});




}
/// @nodoc
class _$NotificationListDtoCopyWithImpl<$Res>
    implements $NotificationListDtoCopyWith<$Res> {
  _$NotificationListDtoCopyWithImpl(this._self, this._then);

  final NotificationListDto _self;
  final $Res Function(NotificationListDto) _then;

/// Create a copy of NotificationListDto
/// with the given fields replaced by the non-null parameter values.
@pragma('vm:prefer-inline') @override $Res call({Object? data = null,Object? total = null,Object? limit = null,Object? offset = null,}) {
  return _then(_self.copyWith(
data: null == data ? _self.data : data // ignore: cast_nullable_to_non_nullable
as List<NotificationDto>,total: null == total ? _self.total : total // ignore: cast_nullable_to_non_nullable
as int,limit: null == limit ? _self.limit : limit // ignore: cast_nullable_to_non_nullable
as int,offset: null == offset ? _self.offset : offset // ignore: cast_nullable_to_non_nullable
as int,
  ));
}

}


/// Adds pattern-matching-related methods to [NotificationListDto].
extension NotificationListDtoPatterns on NotificationListDto {
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

@optionalTypeArgs TResult maybeMap<TResult extends Object?>(TResult Function( _NotificationListDto value)?  $default,{required TResult orElse(),}){
final _that = this;
switch (_that) {
case _NotificationListDto() when $default != null:
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

@optionalTypeArgs TResult map<TResult extends Object?>(TResult Function( _NotificationListDto value)  $default,){
final _that = this;
switch (_that) {
case _NotificationListDto():
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

@optionalTypeArgs TResult? mapOrNull<TResult extends Object?>(TResult? Function( _NotificationListDto value)?  $default,){
final _that = this;
switch (_that) {
case _NotificationListDto() when $default != null:
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

@optionalTypeArgs TResult maybeWhen<TResult extends Object?>(TResult Function( List<NotificationDto> data,  int total,  int limit,  int offset)?  $default,{required TResult orElse(),}) {final _that = this;
switch (_that) {
case _NotificationListDto() when $default != null:
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

@optionalTypeArgs TResult when<TResult extends Object?>(TResult Function( List<NotificationDto> data,  int total,  int limit,  int offset)  $default,) {final _that = this;
switch (_that) {
case _NotificationListDto():
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

@optionalTypeArgs TResult? whenOrNull<TResult extends Object?>(TResult? Function( List<NotificationDto> data,  int total,  int limit,  int offset)?  $default,) {final _that = this;
switch (_that) {
case _NotificationListDto() when $default != null:
return $default(_that.data,_that.total,_that.limit,_that.offset);case _:
  return null;

}
}

}

/// @nodoc
@JsonSerializable()

class _NotificationListDto implements NotificationListDto {
  const _NotificationListDto({final  List<NotificationDto> data = const <NotificationDto>[], required this.total, required this.limit, required this.offset}): _data = data;
  factory _NotificationListDto.fromJson(Map<String, dynamic> json) => _$NotificationListDtoFromJson(json);

 final  List<NotificationDto> _data;
@override@JsonKey() List<NotificationDto> get data {
  if (_data is EqualUnmodifiableListView) return _data;
  // ignore: implicit_dynamic_type
  return EqualUnmodifiableListView(_data);
}

@override final  int total;
@override final  int limit;
@override final  int offset;

/// Create a copy of NotificationListDto
/// with the given fields replaced by the non-null parameter values.
@override @JsonKey(includeFromJson: false, includeToJson: false)
@pragma('vm:prefer-inline')
_$NotificationListDtoCopyWith<_NotificationListDto> get copyWith => __$NotificationListDtoCopyWithImpl<_NotificationListDto>(this, _$identity);

@override
Map<String, dynamic> toJson() {
  return _$NotificationListDtoToJson(this, );
}

@override
bool operator ==(Object other) {
  return identical(this, other) || (other.runtimeType == runtimeType&&other is _NotificationListDto&&const DeepCollectionEquality().equals(other._data, _data)&&(identical(other.total, total) || other.total == total)&&(identical(other.limit, limit) || other.limit == limit)&&(identical(other.offset, offset) || other.offset == offset));
}

@JsonKey(includeFromJson: false, includeToJson: false)
@override
int get hashCode => Object.hash(runtimeType,const DeepCollectionEquality().hash(_data),total,limit,offset);

@override
String toString() {
  return 'NotificationListDto(data: $data, total: $total, limit: $limit, offset: $offset)';
}


}

/// @nodoc
abstract mixin class _$NotificationListDtoCopyWith<$Res> implements $NotificationListDtoCopyWith<$Res> {
  factory _$NotificationListDtoCopyWith(_NotificationListDto value, $Res Function(_NotificationListDto) _then) = __$NotificationListDtoCopyWithImpl;
@override @useResult
$Res call({
 List<NotificationDto> data, int total, int limit, int offset
});




}
/// @nodoc
class __$NotificationListDtoCopyWithImpl<$Res>
    implements _$NotificationListDtoCopyWith<$Res> {
  __$NotificationListDtoCopyWithImpl(this._self, this._then);

  final _NotificationListDto _self;
  final $Res Function(_NotificationListDto) _then;

/// Create a copy of NotificationListDto
/// with the given fields replaced by the non-null parameter values.
@override @pragma('vm:prefer-inline') $Res call({Object? data = null,Object? total = null,Object? limit = null,Object? offset = null,}) {
  return _then(_NotificationListDto(
data: null == data ? _self._data : data // ignore: cast_nullable_to_non_nullable
as List<NotificationDto>,total: null == total ? _self.total : total // ignore: cast_nullable_to_non_nullable
as int,limit: null == limit ? _self.limit : limit // ignore: cast_nullable_to_non_nullable
as int,offset: null == offset ? _self.offset : offset // ignore: cast_nullable_to_non_nullable
as int,
  ));
}


}

// dart format on
