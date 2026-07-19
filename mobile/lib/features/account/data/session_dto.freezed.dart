// GENERATED CODE - DO NOT MODIFY BY HAND
// coverage:ignore-file
// ignore_for_file: type=lint
// ignore_for_file: unused_element, deprecated_member_use, deprecated_member_use_from_same_package, use_function_type_syntax_for_parameters, unnecessary_const, avoid_init_to_null, invalid_override_different_default_values_named, prefer_expression_function_bodies, annotate_overrides, invalid_annotation_target, unnecessary_question_mark

part of 'session_dto.dart';

// **************************************************************************
// FreezedGenerator
// **************************************************************************

// dart format off
T _$identity<T>(T value) => value;

/// @nodoc
mixin _$SessionDto {

 String get id; String get browser; String get os;@JsonKey(name: 'device_type') String get deviceType;@JsonKey(name: 'ip_address') String get ipAddress; String get location;@JsonKey(name: 'created_at') DateTime get createdAt;@JsonKey(name: 'last_seen_at') DateTime get lastSeenAt; bool get current;
/// Create a copy of SessionDto
/// with the given fields replaced by the non-null parameter values.
@JsonKey(includeFromJson: false, includeToJson: false)
@pragma('vm:prefer-inline')
$SessionDtoCopyWith<SessionDto> get copyWith => _$SessionDtoCopyWithImpl<SessionDto>(this as SessionDto, _$identity);

  /// Serializes this SessionDto to a JSON map.
  Map<String, dynamic> toJson();


@override
bool operator ==(Object other) {
  return identical(this, other) || (other.runtimeType == runtimeType&&other is SessionDto&&(identical(other.id, id) || other.id == id)&&(identical(other.browser, browser) || other.browser == browser)&&(identical(other.os, os) || other.os == os)&&(identical(other.deviceType, deviceType) || other.deviceType == deviceType)&&(identical(other.ipAddress, ipAddress) || other.ipAddress == ipAddress)&&(identical(other.location, location) || other.location == location)&&(identical(other.createdAt, createdAt) || other.createdAt == createdAt)&&(identical(other.lastSeenAt, lastSeenAt) || other.lastSeenAt == lastSeenAt)&&(identical(other.current, current) || other.current == current));
}

@JsonKey(includeFromJson: false, includeToJson: false)
@override
int get hashCode => Object.hash(runtimeType,id,browser,os,deviceType,ipAddress,location,createdAt,lastSeenAt,current);

@override
String toString() {
  return 'SessionDto(id: $id, browser: $browser, os: $os, deviceType: $deviceType, ipAddress: $ipAddress, location: $location, createdAt: $createdAt, lastSeenAt: $lastSeenAt, current: $current)';
}


}

/// @nodoc
abstract mixin class $SessionDtoCopyWith<$Res>  {
  factory $SessionDtoCopyWith(SessionDto value, $Res Function(SessionDto) _then) = _$SessionDtoCopyWithImpl;
@useResult
$Res call({
 String id, String browser, String os,@JsonKey(name: 'device_type') String deviceType,@JsonKey(name: 'ip_address') String ipAddress, String location,@JsonKey(name: 'created_at') DateTime createdAt,@JsonKey(name: 'last_seen_at') DateTime lastSeenAt, bool current
});




}
/// @nodoc
class _$SessionDtoCopyWithImpl<$Res>
    implements $SessionDtoCopyWith<$Res> {
  _$SessionDtoCopyWithImpl(this._self, this._then);

  final SessionDto _self;
  final $Res Function(SessionDto) _then;

/// Create a copy of SessionDto
/// with the given fields replaced by the non-null parameter values.
@pragma('vm:prefer-inline') @override $Res call({Object? id = null,Object? browser = null,Object? os = null,Object? deviceType = null,Object? ipAddress = null,Object? location = null,Object? createdAt = null,Object? lastSeenAt = null,Object? current = null,}) {
  return _then(_self.copyWith(
id: null == id ? _self.id : id // ignore: cast_nullable_to_non_nullable
as String,browser: null == browser ? _self.browser : browser // ignore: cast_nullable_to_non_nullable
as String,os: null == os ? _self.os : os // ignore: cast_nullable_to_non_nullable
as String,deviceType: null == deviceType ? _self.deviceType : deviceType // ignore: cast_nullable_to_non_nullable
as String,ipAddress: null == ipAddress ? _self.ipAddress : ipAddress // ignore: cast_nullable_to_non_nullable
as String,location: null == location ? _self.location : location // ignore: cast_nullable_to_non_nullable
as String,createdAt: null == createdAt ? _self.createdAt : createdAt // ignore: cast_nullable_to_non_nullable
as DateTime,lastSeenAt: null == lastSeenAt ? _self.lastSeenAt : lastSeenAt // ignore: cast_nullable_to_non_nullable
as DateTime,current: null == current ? _self.current : current // ignore: cast_nullable_to_non_nullable
as bool,
  ));
}

}


/// Adds pattern-matching-related methods to [SessionDto].
extension SessionDtoPatterns on SessionDto {
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

@optionalTypeArgs TResult maybeMap<TResult extends Object?>(TResult Function( _SessionDto value)?  $default,{required TResult orElse(),}){
final _that = this;
switch (_that) {
case _SessionDto() when $default != null:
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

@optionalTypeArgs TResult map<TResult extends Object?>(TResult Function( _SessionDto value)  $default,){
final _that = this;
switch (_that) {
case _SessionDto():
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

@optionalTypeArgs TResult? mapOrNull<TResult extends Object?>(TResult? Function( _SessionDto value)?  $default,){
final _that = this;
switch (_that) {
case _SessionDto() when $default != null:
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

@optionalTypeArgs TResult maybeWhen<TResult extends Object?>(TResult Function( String id,  String browser,  String os, @JsonKey(name: 'device_type')  String deviceType, @JsonKey(name: 'ip_address')  String ipAddress,  String location, @JsonKey(name: 'created_at')  DateTime createdAt, @JsonKey(name: 'last_seen_at')  DateTime lastSeenAt,  bool current)?  $default,{required TResult orElse(),}) {final _that = this;
switch (_that) {
case _SessionDto() when $default != null:
return $default(_that.id,_that.browser,_that.os,_that.deviceType,_that.ipAddress,_that.location,_that.createdAt,_that.lastSeenAt,_that.current);case _:
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

@optionalTypeArgs TResult when<TResult extends Object?>(TResult Function( String id,  String browser,  String os, @JsonKey(name: 'device_type')  String deviceType, @JsonKey(name: 'ip_address')  String ipAddress,  String location, @JsonKey(name: 'created_at')  DateTime createdAt, @JsonKey(name: 'last_seen_at')  DateTime lastSeenAt,  bool current)  $default,) {final _that = this;
switch (_that) {
case _SessionDto():
return $default(_that.id,_that.browser,_that.os,_that.deviceType,_that.ipAddress,_that.location,_that.createdAt,_that.lastSeenAt,_that.current);case _:
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

@optionalTypeArgs TResult? whenOrNull<TResult extends Object?>(TResult? Function( String id,  String browser,  String os, @JsonKey(name: 'device_type')  String deviceType, @JsonKey(name: 'ip_address')  String ipAddress,  String location, @JsonKey(name: 'created_at')  DateTime createdAt, @JsonKey(name: 'last_seen_at')  DateTime lastSeenAt,  bool current)?  $default,) {final _that = this;
switch (_that) {
case _SessionDto() when $default != null:
return $default(_that.id,_that.browser,_that.os,_that.deviceType,_that.ipAddress,_that.location,_that.createdAt,_that.lastSeenAt,_that.current);case _:
  return null;

}
}

}

/// @nodoc
@JsonSerializable()

class _SessionDto implements SessionDto {
  const _SessionDto({required this.id, required this.browser, required this.os, @JsonKey(name: 'device_type') required this.deviceType, @JsonKey(name: 'ip_address') required this.ipAddress, required this.location, @JsonKey(name: 'created_at') required this.createdAt, @JsonKey(name: 'last_seen_at') required this.lastSeenAt, required this.current});
  factory _SessionDto.fromJson(Map<String, dynamic> json) => _$SessionDtoFromJson(json);

@override final  String id;
@override final  String browser;
@override final  String os;
@override@JsonKey(name: 'device_type') final  String deviceType;
@override@JsonKey(name: 'ip_address') final  String ipAddress;
@override final  String location;
@override@JsonKey(name: 'created_at') final  DateTime createdAt;
@override@JsonKey(name: 'last_seen_at') final  DateTime lastSeenAt;
@override final  bool current;

/// Create a copy of SessionDto
/// with the given fields replaced by the non-null parameter values.
@override @JsonKey(includeFromJson: false, includeToJson: false)
@pragma('vm:prefer-inline')
_$SessionDtoCopyWith<_SessionDto> get copyWith => __$SessionDtoCopyWithImpl<_SessionDto>(this, _$identity);

@override
Map<String, dynamic> toJson() {
  return _$SessionDtoToJson(this, );
}

@override
bool operator ==(Object other) {
  return identical(this, other) || (other.runtimeType == runtimeType&&other is _SessionDto&&(identical(other.id, id) || other.id == id)&&(identical(other.browser, browser) || other.browser == browser)&&(identical(other.os, os) || other.os == os)&&(identical(other.deviceType, deviceType) || other.deviceType == deviceType)&&(identical(other.ipAddress, ipAddress) || other.ipAddress == ipAddress)&&(identical(other.location, location) || other.location == location)&&(identical(other.createdAt, createdAt) || other.createdAt == createdAt)&&(identical(other.lastSeenAt, lastSeenAt) || other.lastSeenAt == lastSeenAt)&&(identical(other.current, current) || other.current == current));
}

@JsonKey(includeFromJson: false, includeToJson: false)
@override
int get hashCode => Object.hash(runtimeType,id,browser,os,deviceType,ipAddress,location,createdAt,lastSeenAt,current);

@override
String toString() {
  return 'SessionDto(id: $id, browser: $browser, os: $os, deviceType: $deviceType, ipAddress: $ipAddress, location: $location, createdAt: $createdAt, lastSeenAt: $lastSeenAt, current: $current)';
}


}

/// @nodoc
abstract mixin class _$SessionDtoCopyWith<$Res> implements $SessionDtoCopyWith<$Res> {
  factory _$SessionDtoCopyWith(_SessionDto value, $Res Function(_SessionDto) _then) = __$SessionDtoCopyWithImpl;
@override @useResult
$Res call({
 String id, String browser, String os,@JsonKey(name: 'device_type') String deviceType,@JsonKey(name: 'ip_address') String ipAddress, String location,@JsonKey(name: 'created_at') DateTime createdAt,@JsonKey(name: 'last_seen_at') DateTime lastSeenAt, bool current
});




}
/// @nodoc
class __$SessionDtoCopyWithImpl<$Res>
    implements _$SessionDtoCopyWith<$Res> {
  __$SessionDtoCopyWithImpl(this._self, this._then);

  final _SessionDto _self;
  final $Res Function(_SessionDto) _then;

/// Create a copy of SessionDto
/// with the given fields replaced by the non-null parameter values.
@override @pragma('vm:prefer-inline') $Res call({Object? id = null,Object? browser = null,Object? os = null,Object? deviceType = null,Object? ipAddress = null,Object? location = null,Object? createdAt = null,Object? lastSeenAt = null,Object? current = null,}) {
  return _then(_SessionDto(
id: null == id ? _self.id : id // ignore: cast_nullable_to_non_nullable
as String,browser: null == browser ? _self.browser : browser // ignore: cast_nullable_to_non_nullable
as String,os: null == os ? _self.os : os // ignore: cast_nullable_to_non_nullable
as String,deviceType: null == deviceType ? _self.deviceType : deviceType // ignore: cast_nullable_to_non_nullable
as String,ipAddress: null == ipAddress ? _self.ipAddress : ipAddress // ignore: cast_nullable_to_non_nullable
as String,location: null == location ? _self.location : location // ignore: cast_nullable_to_non_nullable
as String,createdAt: null == createdAt ? _self.createdAt : createdAt // ignore: cast_nullable_to_non_nullable
as DateTime,lastSeenAt: null == lastSeenAt ? _self.lastSeenAt : lastSeenAt // ignore: cast_nullable_to_non_nullable
as DateTime,current: null == current ? _self.current : current // ignore: cast_nullable_to_non_nullable
as bool,
  ));
}


}

// dart format on
