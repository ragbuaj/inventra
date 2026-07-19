// GENERATED CODE - DO NOT MODIFY BY HAND
// coverage:ignore-file
// ignore_for_file: type=lint
// ignore_for_file: unused_element, deprecated_member_use, deprecated_member_use_from_same_package, use_function_type_syntax_for_parameters, unnecessary_const, avoid_init_to_null, invalid_override_different_default_values_named, prefer_expression_function_bodies, annotate_overrides, invalid_annotation_target, unnecessary_question_mark

part of 'stock_opname_item_dto.dart';

// **************************************************************************
// FreezedGenerator
// **************************************************************************

// dart format off
T _$identity<T>(T value) => value;

/// @nodoc
mixin _$StockOpnameItemDto {

 String get id;@JsonKey(name: 'session_id') String get sessionId;@JsonKey(name: 'asset_id') String get assetId;@JsonKey(name: 'asset_name') String? get assetName;@JsonKey(name: 'asset_tag') String? get assetTag;@JsonKey(name: 'office_name') String? get officeName;@JsonKey(name: 'room_name') String? get roomName;@JsonKey(name: 'floor_name') String? get floorName; bool get expected; String get result; String? get note;@JsonKey(name: 'counted_by_name') String? get countedByName;@JsonKey(name: 'counted_at') DateTime? get countedAt;@JsonKey(name: 'followup_request_id') String? get followupRequestId;@JsonKey(name: 'followup_record_id') String? get followupRecordId;
/// Create a copy of StockOpnameItemDto
/// with the given fields replaced by the non-null parameter values.
@JsonKey(includeFromJson: false, includeToJson: false)
@pragma('vm:prefer-inline')
$StockOpnameItemDtoCopyWith<StockOpnameItemDto> get copyWith => _$StockOpnameItemDtoCopyWithImpl<StockOpnameItemDto>(this as StockOpnameItemDto, _$identity);

  /// Serializes this StockOpnameItemDto to a JSON map.
  Map<String, dynamic> toJson();


@override
bool operator ==(Object other) {
  return identical(this, other) || (other.runtimeType == runtimeType&&other is StockOpnameItemDto&&(identical(other.id, id) || other.id == id)&&(identical(other.sessionId, sessionId) || other.sessionId == sessionId)&&(identical(other.assetId, assetId) || other.assetId == assetId)&&(identical(other.assetName, assetName) || other.assetName == assetName)&&(identical(other.assetTag, assetTag) || other.assetTag == assetTag)&&(identical(other.officeName, officeName) || other.officeName == officeName)&&(identical(other.roomName, roomName) || other.roomName == roomName)&&(identical(other.floorName, floorName) || other.floorName == floorName)&&(identical(other.expected, expected) || other.expected == expected)&&(identical(other.result, result) || other.result == result)&&(identical(other.note, note) || other.note == note)&&(identical(other.countedByName, countedByName) || other.countedByName == countedByName)&&(identical(other.countedAt, countedAt) || other.countedAt == countedAt)&&(identical(other.followupRequestId, followupRequestId) || other.followupRequestId == followupRequestId)&&(identical(other.followupRecordId, followupRecordId) || other.followupRecordId == followupRecordId));
}

@JsonKey(includeFromJson: false, includeToJson: false)
@override
int get hashCode => Object.hash(runtimeType,id,sessionId,assetId,assetName,assetTag,officeName,roomName,floorName,expected,result,note,countedByName,countedAt,followupRequestId,followupRecordId);

@override
String toString() {
  return 'StockOpnameItemDto(id: $id, sessionId: $sessionId, assetId: $assetId, assetName: $assetName, assetTag: $assetTag, officeName: $officeName, roomName: $roomName, floorName: $floorName, expected: $expected, result: $result, note: $note, countedByName: $countedByName, countedAt: $countedAt, followupRequestId: $followupRequestId, followupRecordId: $followupRecordId)';
}


}

/// @nodoc
abstract mixin class $StockOpnameItemDtoCopyWith<$Res>  {
  factory $StockOpnameItemDtoCopyWith(StockOpnameItemDto value, $Res Function(StockOpnameItemDto) _then) = _$StockOpnameItemDtoCopyWithImpl;
@useResult
$Res call({
 String id,@JsonKey(name: 'session_id') String sessionId,@JsonKey(name: 'asset_id') String assetId,@JsonKey(name: 'asset_name') String? assetName,@JsonKey(name: 'asset_tag') String? assetTag,@JsonKey(name: 'office_name') String? officeName,@JsonKey(name: 'room_name') String? roomName,@JsonKey(name: 'floor_name') String? floorName, bool expected, String result, String? note,@JsonKey(name: 'counted_by_name') String? countedByName,@JsonKey(name: 'counted_at') DateTime? countedAt,@JsonKey(name: 'followup_request_id') String? followupRequestId,@JsonKey(name: 'followup_record_id') String? followupRecordId
});




}
/// @nodoc
class _$StockOpnameItemDtoCopyWithImpl<$Res>
    implements $StockOpnameItemDtoCopyWith<$Res> {
  _$StockOpnameItemDtoCopyWithImpl(this._self, this._then);

  final StockOpnameItemDto _self;
  final $Res Function(StockOpnameItemDto) _then;

/// Create a copy of StockOpnameItemDto
/// with the given fields replaced by the non-null parameter values.
@pragma('vm:prefer-inline') @override $Res call({Object? id = null,Object? sessionId = null,Object? assetId = null,Object? assetName = freezed,Object? assetTag = freezed,Object? officeName = freezed,Object? roomName = freezed,Object? floorName = freezed,Object? expected = null,Object? result = null,Object? note = freezed,Object? countedByName = freezed,Object? countedAt = freezed,Object? followupRequestId = freezed,Object? followupRecordId = freezed,}) {
  return _then(_self.copyWith(
id: null == id ? _self.id : id // ignore: cast_nullable_to_non_nullable
as String,sessionId: null == sessionId ? _self.sessionId : sessionId // ignore: cast_nullable_to_non_nullable
as String,assetId: null == assetId ? _self.assetId : assetId // ignore: cast_nullable_to_non_nullable
as String,assetName: freezed == assetName ? _self.assetName : assetName // ignore: cast_nullable_to_non_nullable
as String?,assetTag: freezed == assetTag ? _self.assetTag : assetTag // ignore: cast_nullable_to_non_nullable
as String?,officeName: freezed == officeName ? _self.officeName : officeName // ignore: cast_nullable_to_non_nullable
as String?,roomName: freezed == roomName ? _self.roomName : roomName // ignore: cast_nullable_to_non_nullable
as String?,floorName: freezed == floorName ? _self.floorName : floorName // ignore: cast_nullable_to_non_nullable
as String?,expected: null == expected ? _self.expected : expected // ignore: cast_nullable_to_non_nullable
as bool,result: null == result ? _self.result : result // ignore: cast_nullable_to_non_nullable
as String,note: freezed == note ? _self.note : note // ignore: cast_nullable_to_non_nullable
as String?,countedByName: freezed == countedByName ? _self.countedByName : countedByName // ignore: cast_nullable_to_non_nullable
as String?,countedAt: freezed == countedAt ? _self.countedAt : countedAt // ignore: cast_nullable_to_non_nullable
as DateTime?,followupRequestId: freezed == followupRequestId ? _self.followupRequestId : followupRequestId // ignore: cast_nullable_to_non_nullable
as String?,followupRecordId: freezed == followupRecordId ? _self.followupRecordId : followupRecordId // ignore: cast_nullable_to_non_nullable
as String?,
  ));
}

}


/// Adds pattern-matching-related methods to [StockOpnameItemDto].
extension StockOpnameItemDtoPatterns on StockOpnameItemDto {
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

@optionalTypeArgs TResult maybeMap<TResult extends Object?>(TResult Function( _StockOpnameItemDto value)?  $default,{required TResult orElse(),}){
final _that = this;
switch (_that) {
case _StockOpnameItemDto() when $default != null:
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

@optionalTypeArgs TResult map<TResult extends Object?>(TResult Function( _StockOpnameItemDto value)  $default,){
final _that = this;
switch (_that) {
case _StockOpnameItemDto():
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

@optionalTypeArgs TResult? mapOrNull<TResult extends Object?>(TResult? Function( _StockOpnameItemDto value)?  $default,){
final _that = this;
switch (_that) {
case _StockOpnameItemDto() when $default != null:
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

@optionalTypeArgs TResult maybeWhen<TResult extends Object?>(TResult Function( String id, @JsonKey(name: 'session_id')  String sessionId, @JsonKey(name: 'asset_id')  String assetId, @JsonKey(name: 'asset_name')  String? assetName, @JsonKey(name: 'asset_tag')  String? assetTag, @JsonKey(name: 'office_name')  String? officeName, @JsonKey(name: 'room_name')  String? roomName, @JsonKey(name: 'floor_name')  String? floorName,  bool expected,  String result,  String? note, @JsonKey(name: 'counted_by_name')  String? countedByName, @JsonKey(name: 'counted_at')  DateTime? countedAt, @JsonKey(name: 'followup_request_id')  String? followupRequestId, @JsonKey(name: 'followup_record_id')  String? followupRecordId)?  $default,{required TResult orElse(),}) {final _that = this;
switch (_that) {
case _StockOpnameItemDto() when $default != null:
return $default(_that.id,_that.sessionId,_that.assetId,_that.assetName,_that.assetTag,_that.officeName,_that.roomName,_that.floorName,_that.expected,_that.result,_that.note,_that.countedByName,_that.countedAt,_that.followupRequestId,_that.followupRecordId);case _:
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

@optionalTypeArgs TResult when<TResult extends Object?>(TResult Function( String id, @JsonKey(name: 'session_id')  String sessionId, @JsonKey(name: 'asset_id')  String assetId, @JsonKey(name: 'asset_name')  String? assetName, @JsonKey(name: 'asset_tag')  String? assetTag, @JsonKey(name: 'office_name')  String? officeName, @JsonKey(name: 'room_name')  String? roomName, @JsonKey(name: 'floor_name')  String? floorName,  bool expected,  String result,  String? note, @JsonKey(name: 'counted_by_name')  String? countedByName, @JsonKey(name: 'counted_at')  DateTime? countedAt, @JsonKey(name: 'followup_request_id')  String? followupRequestId, @JsonKey(name: 'followup_record_id')  String? followupRecordId)  $default,) {final _that = this;
switch (_that) {
case _StockOpnameItemDto():
return $default(_that.id,_that.sessionId,_that.assetId,_that.assetName,_that.assetTag,_that.officeName,_that.roomName,_that.floorName,_that.expected,_that.result,_that.note,_that.countedByName,_that.countedAt,_that.followupRequestId,_that.followupRecordId);case _:
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

@optionalTypeArgs TResult? whenOrNull<TResult extends Object?>(TResult? Function( String id, @JsonKey(name: 'session_id')  String sessionId, @JsonKey(name: 'asset_id')  String assetId, @JsonKey(name: 'asset_name')  String? assetName, @JsonKey(name: 'asset_tag')  String? assetTag, @JsonKey(name: 'office_name')  String? officeName, @JsonKey(name: 'room_name')  String? roomName, @JsonKey(name: 'floor_name')  String? floorName,  bool expected,  String result,  String? note, @JsonKey(name: 'counted_by_name')  String? countedByName, @JsonKey(name: 'counted_at')  DateTime? countedAt, @JsonKey(name: 'followup_request_id')  String? followupRequestId, @JsonKey(name: 'followup_record_id')  String? followupRecordId)?  $default,) {final _that = this;
switch (_that) {
case _StockOpnameItemDto() when $default != null:
return $default(_that.id,_that.sessionId,_that.assetId,_that.assetName,_that.assetTag,_that.officeName,_that.roomName,_that.floorName,_that.expected,_that.result,_that.note,_that.countedByName,_that.countedAt,_that.followupRequestId,_that.followupRecordId);case _:
  return null;

}
}

}

/// @nodoc
@JsonSerializable()

class _StockOpnameItemDto implements StockOpnameItemDto {
  const _StockOpnameItemDto({required this.id, @JsonKey(name: 'session_id') required this.sessionId, @JsonKey(name: 'asset_id') required this.assetId, @JsonKey(name: 'asset_name') this.assetName, @JsonKey(name: 'asset_tag') this.assetTag, @JsonKey(name: 'office_name') this.officeName, @JsonKey(name: 'room_name') this.roomName, @JsonKey(name: 'floor_name') this.floorName, required this.expected, required this.result, this.note, @JsonKey(name: 'counted_by_name') this.countedByName, @JsonKey(name: 'counted_at') this.countedAt, @JsonKey(name: 'followup_request_id') this.followupRequestId, @JsonKey(name: 'followup_record_id') this.followupRecordId});
  factory _StockOpnameItemDto.fromJson(Map<String, dynamic> json) => _$StockOpnameItemDtoFromJson(json);

@override final  String id;
@override@JsonKey(name: 'session_id') final  String sessionId;
@override@JsonKey(name: 'asset_id') final  String assetId;
@override@JsonKey(name: 'asset_name') final  String? assetName;
@override@JsonKey(name: 'asset_tag') final  String? assetTag;
@override@JsonKey(name: 'office_name') final  String? officeName;
@override@JsonKey(name: 'room_name') final  String? roomName;
@override@JsonKey(name: 'floor_name') final  String? floorName;
@override final  bool expected;
@override final  String result;
@override final  String? note;
@override@JsonKey(name: 'counted_by_name') final  String? countedByName;
@override@JsonKey(name: 'counted_at') final  DateTime? countedAt;
@override@JsonKey(name: 'followup_request_id') final  String? followupRequestId;
@override@JsonKey(name: 'followup_record_id') final  String? followupRecordId;

/// Create a copy of StockOpnameItemDto
/// with the given fields replaced by the non-null parameter values.
@override @JsonKey(includeFromJson: false, includeToJson: false)
@pragma('vm:prefer-inline')
_$StockOpnameItemDtoCopyWith<_StockOpnameItemDto> get copyWith => __$StockOpnameItemDtoCopyWithImpl<_StockOpnameItemDto>(this, _$identity);

@override
Map<String, dynamic> toJson() {
  return _$StockOpnameItemDtoToJson(this, );
}

@override
bool operator ==(Object other) {
  return identical(this, other) || (other.runtimeType == runtimeType&&other is _StockOpnameItemDto&&(identical(other.id, id) || other.id == id)&&(identical(other.sessionId, sessionId) || other.sessionId == sessionId)&&(identical(other.assetId, assetId) || other.assetId == assetId)&&(identical(other.assetName, assetName) || other.assetName == assetName)&&(identical(other.assetTag, assetTag) || other.assetTag == assetTag)&&(identical(other.officeName, officeName) || other.officeName == officeName)&&(identical(other.roomName, roomName) || other.roomName == roomName)&&(identical(other.floorName, floorName) || other.floorName == floorName)&&(identical(other.expected, expected) || other.expected == expected)&&(identical(other.result, result) || other.result == result)&&(identical(other.note, note) || other.note == note)&&(identical(other.countedByName, countedByName) || other.countedByName == countedByName)&&(identical(other.countedAt, countedAt) || other.countedAt == countedAt)&&(identical(other.followupRequestId, followupRequestId) || other.followupRequestId == followupRequestId)&&(identical(other.followupRecordId, followupRecordId) || other.followupRecordId == followupRecordId));
}

@JsonKey(includeFromJson: false, includeToJson: false)
@override
int get hashCode => Object.hash(runtimeType,id,sessionId,assetId,assetName,assetTag,officeName,roomName,floorName,expected,result,note,countedByName,countedAt,followupRequestId,followupRecordId);

@override
String toString() {
  return 'StockOpnameItemDto(id: $id, sessionId: $sessionId, assetId: $assetId, assetName: $assetName, assetTag: $assetTag, officeName: $officeName, roomName: $roomName, floorName: $floorName, expected: $expected, result: $result, note: $note, countedByName: $countedByName, countedAt: $countedAt, followupRequestId: $followupRequestId, followupRecordId: $followupRecordId)';
}


}

/// @nodoc
abstract mixin class _$StockOpnameItemDtoCopyWith<$Res> implements $StockOpnameItemDtoCopyWith<$Res> {
  factory _$StockOpnameItemDtoCopyWith(_StockOpnameItemDto value, $Res Function(_StockOpnameItemDto) _then) = __$StockOpnameItemDtoCopyWithImpl;
@override @useResult
$Res call({
 String id,@JsonKey(name: 'session_id') String sessionId,@JsonKey(name: 'asset_id') String assetId,@JsonKey(name: 'asset_name') String? assetName,@JsonKey(name: 'asset_tag') String? assetTag,@JsonKey(name: 'office_name') String? officeName,@JsonKey(name: 'room_name') String? roomName,@JsonKey(name: 'floor_name') String? floorName, bool expected, String result, String? note,@JsonKey(name: 'counted_by_name') String? countedByName,@JsonKey(name: 'counted_at') DateTime? countedAt,@JsonKey(name: 'followup_request_id') String? followupRequestId,@JsonKey(name: 'followup_record_id') String? followupRecordId
});




}
/// @nodoc
class __$StockOpnameItemDtoCopyWithImpl<$Res>
    implements _$StockOpnameItemDtoCopyWith<$Res> {
  __$StockOpnameItemDtoCopyWithImpl(this._self, this._then);

  final _StockOpnameItemDto _self;
  final $Res Function(_StockOpnameItemDto) _then;

/// Create a copy of StockOpnameItemDto
/// with the given fields replaced by the non-null parameter values.
@override @pragma('vm:prefer-inline') $Res call({Object? id = null,Object? sessionId = null,Object? assetId = null,Object? assetName = freezed,Object? assetTag = freezed,Object? officeName = freezed,Object? roomName = freezed,Object? floorName = freezed,Object? expected = null,Object? result = null,Object? note = freezed,Object? countedByName = freezed,Object? countedAt = freezed,Object? followupRequestId = freezed,Object? followupRecordId = freezed,}) {
  return _then(_StockOpnameItemDto(
id: null == id ? _self.id : id // ignore: cast_nullable_to_non_nullable
as String,sessionId: null == sessionId ? _self.sessionId : sessionId // ignore: cast_nullable_to_non_nullable
as String,assetId: null == assetId ? _self.assetId : assetId // ignore: cast_nullable_to_non_nullable
as String,assetName: freezed == assetName ? _self.assetName : assetName // ignore: cast_nullable_to_non_nullable
as String?,assetTag: freezed == assetTag ? _self.assetTag : assetTag // ignore: cast_nullable_to_non_nullable
as String?,officeName: freezed == officeName ? _self.officeName : officeName // ignore: cast_nullable_to_non_nullable
as String?,roomName: freezed == roomName ? _self.roomName : roomName // ignore: cast_nullable_to_non_nullable
as String?,floorName: freezed == floorName ? _self.floorName : floorName // ignore: cast_nullable_to_non_nullable
as String?,expected: null == expected ? _self.expected : expected // ignore: cast_nullable_to_non_nullable
as bool,result: null == result ? _self.result : result // ignore: cast_nullable_to_non_nullable
as String,note: freezed == note ? _self.note : note // ignore: cast_nullable_to_non_nullable
as String?,countedByName: freezed == countedByName ? _self.countedByName : countedByName // ignore: cast_nullable_to_non_nullable
as String?,countedAt: freezed == countedAt ? _self.countedAt : countedAt // ignore: cast_nullable_to_non_nullable
as DateTime?,followupRequestId: freezed == followupRequestId ? _self.followupRequestId : followupRequestId // ignore: cast_nullable_to_non_nullable
as String?,followupRecordId: freezed == followupRecordId ? _self.followupRecordId : followupRecordId // ignore: cast_nullable_to_non_nullable
as String?,
  ));
}


}

// dart format on
