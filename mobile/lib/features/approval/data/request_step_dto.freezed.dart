// GENERATED CODE - DO NOT MODIFY BY HAND
// coverage:ignore-file
// ignore_for_file: type=lint
// ignore_for_file: unused_element, deprecated_member_use, deprecated_member_use_from_same_package, use_function_type_syntax_for_parameters, unnecessary_const, avoid_init_to_null, invalid_override_different_default_values_named, prefer_expression_function_bodies, annotate_overrides, invalid_annotation_target, unnecessary_question_mark

part of 'request_step_dto.dart';

// **************************************************************************
// FreezedGenerator
// **************************************************************************

// dart format off
T _$identity<T>(T value) => value;

/// @nodoc
mixin _$RequestStepDto {

@JsonKey(name: 'step_order') int? get stepOrder;@JsonKey(name: 'required_level') String? get requiredLevel;@JsonKey(name: 'approver_id') String? get approverId;@JsonKey(name: 'approver_name') String? get approverName; String? get decision; String? get note;@JsonKey(name: 'decided_at') DateTime? get decidedAt;
/// Create a copy of RequestStepDto
/// with the given fields replaced by the non-null parameter values.
@JsonKey(includeFromJson: false, includeToJson: false)
@pragma('vm:prefer-inline')
$RequestStepDtoCopyWith<RequestStepDto> get copyWith => _$RequestStepDtoCopyWithImpl<RequestStepDto>(this as RequestStepDto, _$identity);

  /// Serializes this RequestStepDto to a JSON map.
  Map<String, dynamic> toJson();


@override
bool operator ==(Object other) {
  return identical(this, other) || (other.runtimeType == runtimeType&&other is RequestStepDto&&(identical(other.stepOrder, stepOrder) || other.stepOrder == stepOrder)&&(identical(other.requiredLevel, requiredLevel) || other.requiredLevel == requiredLevel)&&(identical(other.approverId, approverId) || other.approverId == approverId)&&(identical(other.approverName, approverName) || other.approverName == approverName)&&(identical(other.decision, decision) || other.decision == decision)&&(identical(other.note, note) || other.note == note)&&(identical(other.decidedAt, decidedAt) || other.decidedAt == decidedAt));
}

@JsonKey(includeFromJson: false, includeToJson: false)
@override
int get hashCode => Object.hash(runtimeType,stepOrder,requiredLevel,approverId,approverName,decision,note,decidedAt);

@override
String toString() {
  return 'RequestStepDto(stepOrder: $stepOrder, requiredLevel: $requiredLevel, approverId: $approverId, approverName: $approverName, decision: $decision, note: $note, decidedAt: $decidedAt)';
}


}

/// @nodoc
abstract mixin class $RequestStepDtoCopyWith<$Res>  {
  factory $RequestStepDtoCopyWith(RequestStepDto value, $Res Function(RequestStepDto) _then) = _$RequestStepDtoCopyWithImpl;
@useResult
$Res call({
@JsonKey(name: 'step_order') int? stepOrder,@JsonKey(name: 'required_level') String? requiredLevel,@JsonKey(name: 'approver_id') String? approverId,@JsonKey(name: 'approver_name') String? approverName, String? decision, String? note,@JsonKey(name: 'decided_at') DateTime? decidedAt
});




}
/// @nodoc
class _$RequestStepDtoCopyWithImpl<$Res>
    implements $RequestStepDtoCopyWith<$Res> {
  _$RequestStepDtoCopyWithImpl(this._self, this._then);

  final RequestStepDto _self;
  final $Res Function(RequestStepDto) _then;

/// Create a copy of RequestStepDto
/// with the given fields replaced by the non-null parameter values.
@pragma('vm:prefer-inline') @override $Res call({Object? stepOrder = freezed,Object? requiredLevel = freezed,Object? approverId = freezed,Object? approverName = freezed,Object? decision = freezed,Object? note = freezed,Object? decidedAt = freezed,}) {
  return _then(_self.copyWith(
stepOrder: freezed == stepOrder ? _self.stepOrder : stepOrder // ignore: cast_nullable_to_non_nullable
as int?,requiredLevel: freezed == requiredLevel ? _self.requiredLevel : requiredLevel // ignore: cast_nullable_to_non_nullable
as String?,approverId: freezed == approverId ? _self.approverId : approverId // ignore: cast_nullable_to_non_nullable
as String?,approverName: freezed == approverName ? _self.approverName : approverName // ignore: cast_nullable_to_non_nullable
as String?,decision: freezed == decision ? _self.decision : decision // ignore: cast_nullable_to_non_nullable
as String?,note: freezed == note ? _self.note : note // ignore: cast_nullable_to_non_nullable
as String?,decidedAt: freezed == decidedAt ? _self.decidedAt : decidedAt // ignore: cast_nullable_to_non_nullable
as DateTime?,
  ));
}

}


/// Adds pattern-matching-related methods to [RequestStepDto].
extension RequestStepDtoPatterns on RequestStepDto {
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

@optionalTypeArgs TResult maybeMap<TResult extends Object?>(TResult Function( _RequestStepDto value)?  $default,{required TResult orElse(),}){
final _that = this;
switch (_that) {
case _RequestStepDto() when $default != null:
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

@optionalTypeArgs TResult map<TResult extends Object?>(TResult Function( _RequestStepDto value)  $default,){
final _that = this;
switch (_that) {
case _RequestStepDto():
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

@optionalTypeArgs TResult? mapOrNull<TResult extends Object?>(TResult? Function( _RequestStepDto value)?  $default,){
final _that = this;
switch (_that) {
case _RequestStepDto() when $default != null:
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

@optionalTypeArgs TResult maybeWhen<TResult extends Object?>(TResult Function(@JsonKey(name: 'step_order')  int? stepOrder, @JsonKey(name: 'required_level')  String? requiredLevel, @JsonKey(name: 'approver_id')  String? approverId, @JsonKey(name: 'approver_name')  String? approverName,  String? decision,  String? note, @JsonKey(name: 'decided_at')  DateTime? decidedAt)?  $default,{required TResult orElse(),}) {final _that = this;
switch (_that) {
case _RequestStepDto() when $default != null:
return $default(_that.stepOrder,_that.requiredLevel,_that.approverId,_that.approverName,_that.decision,_that.note,_that.decidedAt);case _:
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

@optionalTypeArgs TResult when<TResult extends Object?>(TResult Function(@JsonKey(name: 'step_order')  int? stepOrder, @JsonKey(name: 'required_level')  String? requiredLevel, @JsonKey(name: 'approver_id')  String? approverId, @JsonKey(name: 'approver_name')  String? approverName,  String? decision,  String? note, @JsonKey(name: 'decided_at')  DateTime? decidedAt)  $default,) {final _that = this;
switch (_that) {
case _RequestStepDto():
return $default(_that.stepOrder,_that.requiredLevel,_that.approverId,_that.approverName,_that.decision,_that.note,_that.decidedAt);case _:
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

@optionalTypeArgs TResult? whenOrNull<TResult extends Object?>(TResult? Function(@JsonKey(name: 'step_order')  int? stepOrder, @JsonKey(name: 'required_level')  String? requiredLevel, @JsonKey(name: 'approver_id')  String? approverId, @JsonKey(name: 'approver_name')  String? approverName,  String? decision,  String? note, @JsonKey(name: 'decided_at')  DateTime? decidedAt)?  $default,) {final _that = this;
switch (_that) {
case _RequestStepDto() when $default != null:
return $default(_that.stepOrder,_that.requiredLevel,_that.approverId,_that.approverName,_that.decision,_that.note,_that.decidedAt);case _:
  return null;

}
}

}

/// @nodoc
@JsonSerializable()

class _RequestStepDto implements RequestStepDto {
  const _RequestStepDto({@JsonKey(name: 'step_order') this.stepOrder, @JsonKey(name: 'required_level') this.requiredLevel, @JsonKey(name: 'approver_id') this.approverId, @JsonKey(name: 'approver_name') this.approverName, this.decision, this.note, @JsonKey(name: 'decided_at') this.decidedAt});
  factory _RequestStepDto.fromJson(Map<String, dynamic> json) => _$RequestStepDtoFromJson(json);

@override@JsonKey(name: 'step_order') final  int? stepOrder;
@override@JsonKey(name: 'required_level') final  String? requiredLevel;
@override@JsonKey(name: 'approver_id') final  String? approverId;
@override@JsonKey(name: 'approver_name') final  String? approverName;
@override final  String? decision;
@override final  String? note;
@override@JsonKey(name: 'decided_at') final  DateTime? decidedAt;

/// Create a copy of RequestStepDto
/// with the given fields replaced by the non-null parameter values.
@override @JsonKey(includeFromJson: false, includeToJson: false)
@pragma('vm:prefer-inline')
_$RequestStepDtoCopyWith<_RequestStepDto> get copyWith => __$RequestStepDtoCopyWithImpl<_RequestStepDto>(this, _$identity);

@override
Map<String, dynamic> toJson() {
  return _$RequestStepDtoToJson(this, );
}

@override
bool operator ==(Object other) {
  return identical(this, other) || (other.runtimeType == runtimeType&&other is _RequestStepDto&&(identical(other.stepOrder, stepOrder) || other.stepOrder == stepOrder)&&(identical(other.requiredLevel, requiredLevel) || other.requiredLevel == requiredLevel)&&(identical(other.approverId, approverId) || other.approverId == approverId)&&(identical(other.approverName, approverName) || other.approverName == approverName)&&(identical(other.decision, decision) || other.decision == decision)&&(identical(other.note, note) || other.note == note)&&(identical(other.decidedAt, decidedAt) || other.decidedAt == decidedAt));
}

@JsonKey(includeFromJson: false, includeToJson: false)
@override
int get hashCode => Object.hash(runtimeType,stepOrder,requiredLevel,approverId,approverName,decision,note,decidedAt);

@override
String toString() {
  return 'RequestStepDto(stepOrder: $stepOrder, requiredLevel: $requiredLevel, approverId: $approverId, approverName: $approverName, decision: $decision, note: $note, decidedAt: $decidedAt)';
}


}

/// @nodoc
abstract mixin class _$RequestStepDtoCopyWith<$Res> implements $RequestStepDtoCopyWith<$Res> {
  factory _$RequestStepDtoCopyWith(_RequestStepDto value, $Res Function(_RequestStepDto) _then) = __$RequestStepDtoCopyWithImpl;
@override @useResult
$Res call({
@JsonKey(name: 'step_order') int? stepOrder,@JsonKey(name: 'required_level') String? requiredLevel,@JsonKey(name: 'approver_id') String? approverId,@JsonKey(name: 'approver_name') String? approverName, String? decision, String? note,@JsonKey(name: 'decided_at') DateTime? decidedAt
});




}
/// @nodoc
class __$RequestStepDtoCopyWithImpl<$Res>
    implements _$RequestStepDtoCopyWith<$Res> {
  __$RequestStepDtoCopyWithImpl(this._self, this._then);

  final _RequestStepDto _self;
  final $Res Function(_RequestStepDto) _then;

/// Create a copy of RequestStepDto
/// with the given fields replaced by the non-null parameter values.
@override @pragma('vm:prefer-inline') $Res call({Object? stepOrder = freezed,Object? requiredLevel = freezed,Object? approverId = freezed,Object? approverName = freezed,Object? decision = freezed,Object? note = freezed,Object? decidedAt = freezed,}) {
  return _then(_RequestStepDto(
stepOrder: freezed == stepOrder ? _self.stepOrder : stepOrder // ignore: cast_nullable_to_non_nullable
as int?,requiredLevel: freezed == requiredLevel ? _self.requiredLevel : requiredLevel // ignore: cast_nullable_to_non_nullable
as String?,approverId: freezed == approverId ? _self.approverId : approverId // ignore: cast_nullable_to_non_nullable
as String?,approverName: freezed == approverName ? _self.approverName : approverName // ignore: cast_nullable_to_non_nullable
as String?,decision: freezed == decision ? _self.decision : decision // ignore: cast_nullable_to_non_nullable
as String?,note: freezed == note ? _self.note : note // ignore: cast_nullable_to_non_nullable
as String?,decidedAt: freezed == decidedAt ? _self.decidedAt : decidedAt // ignore: cast_nullable_to_non_nullable
as DateTime?,
  ));
}


}

// dart format on
