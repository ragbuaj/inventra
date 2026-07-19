// GENERATED CODE - DO NOT MODIFY BY HAND
// coverage:ignore-file
// ignore_for_file: type=lint
// ignore_for_file: unused_element, deprecated_member_use, deprecated_member_use_from_same_package, use_function_type_syntax_for_parameters, unnecessary_const, avoid_init_to_null, invalid_override_different_default_values_named, prefer_expression_function_bodies, annotate_overrides, invalid_annotation_target, unnecessary_question_mark

part of 'request_detail_dto.dart';

// **************************************************************************
// FreezedGenerator
// **************************************************************************

// dart format off
T _$identity<T>(T value) => value;

/// @nodoc
mixin _$RequestDetailDto {

 String get id; String get type; String get status; String? get amount;@JsonKey(name: 'current_step') int get currentStep;@JsonKey(name: 'office_id') String? get officeId;@JsonKey(name: 'target_id') String? get targetId;@JsonKey(name: 'target_entity') String? get targetEntity; String? get reason;@JsonKey(name: 'requested_by_id') String get requestedById;@JsonKey(name: 'requested_by_name') String? get requestedByName;@JsonKey(name: 'requested_by_role') String? get requestedByRole;@JsonKey(name: 'office_name') String? get officeName;@JsonKey(name: 'decided_by_id') String? get decidedById;@JsonKey(name: 'decision_note') String? get decisionNote;@JsonKey(name: 'created_at') DateTime? get createdAt; Map<String, dynamic>? get payload; List<RequestStepDto> get steps;
/// Create a copy of RequestDetailDto
/// with the given fields replaced by the non-null parameter values.
@JsonKey(includeFromJson: false, includeToJson: false)
@pragma('vm:prefer-inline')
$RequestDetailDtoCopyWith<RequestDetailDto> get copyWith => _$RequestDetailDtoCopyWithImpl<RequestDetailDto>(this as RequestDetailDto, _$identity);

  /// Serializes this RequestDetailDto to a JSON map.
  Map<String, dynamic> toJson();


@override
bool operator ==(Object other) {
  return identical(this, other) || (other.runtimeType == runtimeType&&other is RequestDetailDto&&(identical(other.id, id) || other.id == id)&&(identical(other.type, type) || other.type == type)&&(identical(other.status, status) || other.status == status)&&(identical(other.amount, amount) || other.amount == amount)&&(identical(other.currentStep, currentStep) || other.currentStep == currentStep)&&(identical(other.officeId, officeId) || other.officeId == officeId)&&(identical(other.targetId, targetId) || other.targetId == targetId)&&(identical(other.targetEntity, targetEntity) || other.targetEntity == targetEntity)&&(identical(other.reason, reason) || other.reason == reason)&&(identical(other.requestedById, requestedById) || other.requestedById == requestedById)&&(identical(other.requestedByName, requestedByName) || other.requestedByName == requestedByName)&&(identical(other.requestedByRole, requestedByRole) || other.requestedByRole == requestedByRole)&&(identical(other.officeName, officeName) || other.officeName == officeName)&&(identical(other.decidedById, decidedById) || other.decidedById == decidedById)&&(identical(other.decisionNote, decisionNote) || other.decisionNote == decisionNote)&&(identical(other.createdAt, createdAt) || other.createdAt == createdAt)&&const DeepCollectionEquality().equals(other.payload, payload)&&const DeepCollectionEquality().equals(other.steps, steps));
}

@JsonKey(includeFromJson: false, includeToJson: false)
@override
int get hashCode => Object.hash(runtimeType,id,type,status,amount,currentStep,officeId,targetId,targetEntity,reason,requestedById,requestedByName,requestedByRole,officeName,decidedById,decisionNote,createdAt,const DeepCollectionEquality().hash(payload),const DeepCollectionEquality().hash(steps));

@override
String toString() {
  return 'RequestDetailDto(id: $id, type: $type, status: $status, amount: $amount, currentStep: $currentStep, officeId: $officeId, targetId: $targetId, targetEntity: $targetEntity, reason: $reason, requestedById: $requestedById, requestedByName: $requestedByName, requestedByRole: $requestedByRole, officeName: $officeName, decidedById: $decidedById, decisionNote: $decisionNote, createdAt: $createdAt, payload: $payload, steps: $steps)';
}


}

/// @nodoc
abstract mixin class $RequestDetailDtoCopyWith<$Res>  {
  factory $RequestDetailDtoCopyWith(RequestDetailDto value, $Res Function(RequestDetailDto) _then) = _$RequestDetailDtoCopyWithImpl;
@useResult
$Res call({
 String id, String type, String status, String? amount,@JsonKey(name: 'current_step') int currentStep,@JsonKey(name: 'office_id') String? officeId,@JsonKey(name: 'target_id') String? targetId,@JsonKey(name: 'target_entity') String? targetEntity, String? reason,@JsonKey(name: 'requested_by_id') String requestedById,@JsonKey(name: 'requested_by_name') String? requestedByName,@JsonKey(name: 'requested_by_role') String? requestedByRole,@JsonKey(name: 'office_name') String? officeName,@JsonKey(name: 'decided_by_id') String? decidedById,@JsonKey(name: 'decision_note') String? decisionNote,@JsonKey(name: 'created_at') DateTime? createdAt, Map<String, dynamic>? payload, List<RequestStepDto> steps
});




}
/// @nodoc
class _$RequestDetailDtoCopyWithImpl<$Res>
    implements $RequestDetailDtoCopyWith<$Res> {
  _$RequestDetailDtoCopyWithImpl(this._self, this._then);

  final RequestDetailDto _self;
  final $Res Function(RequestDetailDto) _then;

/// Create a copy of RequestDetailDto
/// with the given fields replaced by the non-null parameter values.
@pragma('vm:prefer-inline') @override $Res call({Object? id = null,Object? type = null,Object? status = null,Object? amount = freezed,Object? currentStep = null,Object? officeId = freezed,Object? targetId = freezed,Object? targetEntity = freezed,Object? reason = freezed,Object? requestedById = null,Object? requestedByName = freezed,Object? requestedByRole = freezed,Object? officeName = freezed,Object? decidedById = freezed,Object? decisionNote = freezed,Object? createdAt = freezed,Object? payload = freezed,Object? steps = null,}) {
  return _then(_self.copyWith(
id: null == id ? _self.id : id // ignore: cast_nullable_to_non_nullable
as String,type: null == type ? _self.type : type // ignore: cast_nullable_to_non_nullable
as String,status: null == status ? _self.status : status // ignore: cast_nullable_to_non_nullable
as String,amount: freezed == amount ? _self.amount : amount // ignore: cast_nullable_to_non_nullable
as String?,currentStep: null == currentStep ? _self.currentStep : currentStep // ignore: cast_nullable_to_non_nullable
as int,officeId: freezed == officeId ? _self.officeId : officeId // ignore: cast_nullable_to_non_nullable
as String?,targetId: freezed == targetId ? _self.targetId : targetId // ignore: cast_nullable_to_non_nullable
as String?,targetEntity: freezed == targetEntity ? _self.targetEntity : targetEntity // ignore: cast_nullable_to_non_nullable
as String?,reason: freezed == reason ? _self.reason : reason // ignore: cast_nullable_to_non_nullable
as String?,requestedById: null == requestedById ? _self.requestedById : requestedById // ignore: cast_nullable_to_non_nullable
as String,requestedByName: freezed == requestedByName ? _self.requestedByName : requestedByName // ignore: cast_nullable_to_non_nullable
as String?,requestedByRole: freezed == requestedByRole ? _self.requestedByRole : requestedByRole // ignore: cast_nullable_to_non_nullable
as String?,officeName: freezed == officeName ? _self.officeName : officeName // ignore: cast_nullable_to_non_nullable
as String?,decidedById: freezed == decidedById ? _self.decidedById : decidedById // ignore: cast_nullable_to_non_nullable
as String?,decisionNote: freezed == decisionNote ? _self.decisionNote : decisionNote // ignore: cast_nullable_to_non_nullable
as String?,createdAt: freezed == createdAt ? _self.createdAt : createdAt // ignore: cast_nullable_to_non_nullable
as DateTime?,payload: freezed == payload ? _self.payload : payload // ignore: cast_nullable_to_non_nullable
as Map<String, dynamic>?,steps: null == steps ? _self.steps : steps // ignore: cast_nullable_to_non_nullable
as List<RequestStepDto>,
  ));
}

}


/// Adds pattern-matching-related methods to [RequestDetailDto].
extension RequestDetailDtoPatterns on RequestDetailDto {
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

@optionalTypeArgs TResult maybeMap<TResult extends Object?>(TResult Function( _RequestDetailDto value)?  $default,{required TResult orElse(),}){
final _that = this;
switch (_that) {
case _RequestDetailDto() when $default != null:
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

@optionalTypeArgs TResult map<TResult extends Object?>(TResult Function( _RequestDetailDto value)  $default,){
final _that = this;
switch (_that) {
case _RequestDetailDto():
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

@optionalTypeArgs TResult? mapOrNull<TResult extends Object?>(TResult? Function( _RequestDetailDto value)?  $default,){
final _that = this;
switch (_that) {
case _RequestDetailDto() when $default != null:
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

@optionalTypeArgs TResult maybeWhen<TResult extends Object?>(TResult Function( String id,  String type,  String status,  String? amount, @JsonKey(name: 'current_step')  int currentStep, @JsonKey(name: 'office_id')  String? officeId, @JsonKey(name: 'target_id')  String? targetId, @JsonKey(name: 'target_entity')  String? targetEntity,  String? reason, @JsonKey(name: 'requested_by_id')  String requestedById, @JsonKey(name: 'requested_by_name')  String? requestedByName, @JsonKey(name: 'requested_by_role')  String? requestedByRole, @JsonKey(name: 'office_name')  String? officeName, @JsonKey(name: 'decided_by_id')  String? decidedById, @JsonKey(name: 'decision_note')  String? decisionNote, @JsonKey(name: 'created_at')  DateTime? createdAt,  Map<String, dynamic>? payload,  List<RequestStepDto> steps)?  $default,{required TResult orElse(),}) {final _that = this;
switch (_that) {
case _RequestDetailDto() when $default != null:
return $default(_that.id,_that.type,_that.status,_that.amount,_that.currentStep,_that.officeId,_that.targetId,_that.targetEntity,_that.reason,_that.requestedById,_that.requestedByName,_that.requestedByRole,_that.officeName,_that.decidedById,_that.decisionNote,_that.createdAt,_that.payload,_that.steps);case _:
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

@optionalTypeArgs TResult when<TResult extends Object?>(TResult Function( String id,  String type,  String status,  String? amount, @JsonKey(name: 'current_step')  int currentStep, @JsonKey(name: 'office_id')  String? officeId, @JsonKey(name: 'target_id')  String? targetId, @JsonKey(name: 'target_entity')  String? targetEntity,  String? reason, @JsonKey(name: 'requested_by_id')  String requestedById, @JsonKey(name: 'requested_by_name')  String? requestedByName, @JsonKey(name: 'requested_by_role')  String? requestedByRole, @JsonKey(name: 'office_name')  String? officeName, @JsonKey(name: 'decided_by_id')  String? decidedById, @JsonKey(name: 'decision_note')  String? decisionNote, @JsonKey(name: 'created_at')  DateTime? createdAt,  Map<String, dynamic>? payload,  List<RequestStepDto> steps)  $default,) {final _that = this;
switch (_that) {
case _RequestDetailDto():
return $default(_that.id,_that.type,_that.status,_that.amount,_that.currentStep,_that.officeId,_that.targetId,_that.targetEntity,_that.reason,_that.requestedById,_that.requestedByName,_that.requestedByRole,_that.officeName,_that.decidedById,_that.decisionNote,_that.createdAt,_that.payload,_that.steps);case _:
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

@optionalTypeArgs TResult? whenOrNull<TResult extends Object?>(TResult? Function( String id,  String type,  String status,  String? amount, @JsonKey(name: 'current_step')  int currentStep, @JsonKey(name: 'office_id')  String? officeId, @JsonKey(name: 'target_id')  String? targetId, @JsonKey(name: 'target_entity')  String? targetEntity,  String? reason, @JsonKey(name: 'requested_by_id')  String requestedById, @JsonKey(name: 'requested_by_name')  String? requestedByName, @JsonKey(name: 'requested_by_role')  String? requestedByRole, @JsonKey(name: 'office_name')  String? officeName, @JsonKey(name: 'decided_by_id')  String? decidedById, @JsonKey(name: 'decision_note')  String? decisionNote, @JsonKey(name: 'created_at')  DateTime? createdAt,  Map<String, dynamic>? payload,  List<RequestStepDto> steps)?  $default,) {final _that = this;
switch (_that) {
case _RequestDetailDto() when $default != null:
return $default(_that.id,_that.type,_that.status,_that.amount,_that.currentStep,_that.officeId,_that.targetId,_that.targetEntity,_that.reason,_that.requestedById,_that.requestedByName,_that.requestedByRole,_that.officeName,_that.decidedById,_that.decisionNote,_that.createdAt,_that.payload,_that.steps);case _:
  return null;

}
}

}

/// @nodoc
@JsonSerializable()

class _RequestDetailDto implements RequestDetailDto {
  const _RequestDetailDto({required this.id, required this.type, required this.status, this.amount, @JsonKey(name: 'current_step') required this.currentStep, @JsonKey(name: 'office_id') this.officeId, @JsonKey(name: 'target_id') this.targetId, @JsonKey(name: 'target_entity') this.targetEntity, this.reason, @JsonKey(name: 'requested_by_id') required this.requestedById, @JsonKey(name: 'requested_by_name') this.requestedByName, @JsonKey(name: 'requested_by_role') this.requestedByRole, @JsonKey(name: 'office_name') this.officeName, @JsonKey(name: 'decided_by_id') this.decidedById, @JsonKey(name: 'decision_note') this.decisionNote, @JsonKey(name: 'created_at') this.createdAt, final  Map<String, dynamic>? payload, final  List<RequestStepDto> steps = const <RequestStepDto>[]}): _payload = payload,_steps = steps;
  factory _RequestDetailDto.fromJson(Map<String, dynamic> json) => _$RequestDetailDtoFromJson(json);

@override final  String id;
@override final  String type;
@override final  String status;
@override final  String? amount;
@override@JsonKey(name: 'current_step') final  int currentStep;
@override@JsonKey(name: 'office_id') final  String? officeId;
@override@JsonKey(name: 'target_id') final  String? targetId;
@override@JsonKey(name: 'target_entity') final  String? targetEntity;
@override final  String? reason;
@override@JsonKey(name: 'requested_by_id') final  String requestedById;
@override@JsonKey(name: 'requested_by_name') final  String? requestedByName;
@override@JsonKey(name: 'requested_by_role') final  String? requestedByRole;
@override@JsonKey(name: 'office_name') final  String? officeName;
@override@JsonKey(name: 'decided_by_id') final  String? decidedById;
@override@JsonKey(name: 'decision_note') final  String? decisionNote;
@override@JsonKey(name: 'created_at') final  DateTime? createdAt;
 final  Map<String, dynamic>? _payload;
@override Map<String, dynamic>? get payload {
  final value = _payload;
  if (value == null) return null;
  if (_payload is EqualUnmodifiableMapView) return _payload;
  // ignore: implicit_dynamic_type
  return EqualUnmodifiableMapView(value);
}

 final  List<RequestStepDto> _steps;
@override@JsonKey() List<RequestStepDto> get steps {
  if (_steps is EqualUnmodifiableListView) return _steps;
  // ignore: implicit_dynamic_type
  return EqualUnmodifiableListView(_steps);
}


/// Create a copy of RequestDetailDto
/// with the given fields replaced by the non-null parameter values.
@override @JsonKey(includeFromJson: false, includeToJson: false)
@pragma('vm:prefer-inline')
_$RequestDetailDtoCopyWith<_RequestDetailDto> get copyWith => __$RequestDetailDtoCopyWithImpl<_RequestDetailDto>(this, _$identity);

@override
Map<String, dynamic> toJson() {
  return _$RequestDetailDtoToJson(this, );
}

@override
bool operator ==(Object other) {
  return identical(this, other) || (other.runtimeType == runtimeType&&other is _RequestDetailDto&&(identical(other.id, id) || other.id == id)&&(identical(other.type, type) || other.type == type)&&(identical(other.status, status) || other.status == status)&&(identical(other.amount, amount) || other.amount == amount)&&(identical(other.currentStep, currentStep) || other.currentStep == currentStep)&&(identical(other.officeId, officeId) || other.officeId == officeId)&&(identical(other.targetId, targetId) || other.targetId == targetId)&&(identical(other.targetEntity, targetEntity) || other.targetEntity == targetEntity)&&(identical(other.reason, reason) || other.reason == reason)&&(identical(other.requestedById, requestedById) || other.requestedById == requestedById)&&(identical(other.requestedByName, requestedByName) || other.requestedByName == requestedByName)&&(identical(other.requestedByRole, requestedByRole) || other.requestedByRole == requestedByRole)&&(identical(other.officeName, officeName) || other.officeName == officeName)&&(identical(other.decidedById, decidedById) || other.decidedById == decidedById)&&(identical(other.decisionNote, decisionNote) || other.decisionNote == decisionNote)&&(identical(other.createdAt, createdAt) || other.createdAt == createdAt)&&const DeepCollectionEquality().equals(other._payload, _payload)&&const DeepCollectionEquality().equals(other._steps, _steps));
}

@JsonKey(includeFromJson: false, includeToJson: false)
@override
int get hashCode => Object.hash(runtimeType,id,type,status,amount,currentStep,officeId,targetId,targetEntity,reason,requestedById,requestedByName,requestedByRole,officeName,decidedById,decisionNote,createdAt,const DeepCollectionEquality().hash(_payload),const DeepCollectionEquality().hash(_steps));

@override
String toString() {
  return 'RequestDetailDto(id: $id, type: $type, status: $status, amount: $amount, currentStep: $currentStep, officeId: $officeId, targetId: $targetId, targetEntity: $targetEntity, reason: $reason, requestedById: $requestedById, requestedByName: $requestedByName, requestedByRole: $requestedByRole, officeName: $officeName, decidedById: $decidedById, decisionNote: $decisionNote, createdAt: $createdAt, payload: $payload, steps: $steps)';
}


}

/// @nodoc
abstract mixin class _$RequestDetailDtoCopyWith<$Res> implements $RequestDetailDtoCopyWith<$Res> {
  factory _$RequestDetailDtoCopyWith(_RequestDetailDto value, $Res Function(_RequestDetailDto) _then) = __$RequestDetailDtoCopyWithImpl;
@override @useResult
$Res call({
 String id, String type, String status, String? amount,@JsonKey(name: 'current_step') int currentStep,@JsonKey(name: 'office_id') String? officeId,@JsonKey(name: 'target_id') String? targetId,@JsonKey(name: 'target_entity') String? targetEntity, String? reason,@JsonKey(name: 'requested_by_id') String requestedById,@JsonKey(name: 'requested_by_name') String? requestedByName,@JsonKey(name: 'requested_by_role') String? requestedByRole,@JsonKey(name: 'office_name') String? officeName,@JsonKey(name: 'decided_by_id') String? decidedById,@JsonKey(name: 'decision_note') String? decisionNote,@JsonKey(name: 'created_at') DateTime? createdAt, Map<String, dynamic>? payload, List<RequestStepDto> steps
});




}
/// @nodoc
class __$RequestDetailDtoCopyWithImpl<$Res>
    implements _$RequestDetailDtoCopyWith<$Res> {
  __$RequestDetailDtoCopyWithImpl(this._self, this._then);

  final _RequestDetailDto _self;
  final $Res Function(_RequestDetailDto) _then;

/// Create a copy of RequestDetailDto
/// with the given fields replaced by the non-null parameter values.
@override @pragma('vm:prefer-inline') $Res call({Object? id = null,Object? type = null,Object? status = null,Object? amount = freezed,Object? currentStep = null,Object? officeId = freezed,Object? targetId = freezed,Object? targetEntity = freezed,Object? reason = freezed,Object? requestedById = null,Object? requestedByName = freezed,Object? requestedByRole = freezed,Object? officeName = freezed,Object? decidedById = freezed,Object? decisionNote = freezed,Object? createdAt = freezed,Object? payload = freezed,Object? steps = null,}) {
  return _then(_RequestDetailDto(
id: null == id ? _self.id : id // ignore: cast_nullable_to_non_nullable
as String,type: null == type ? _self.type : type // ignore: cast_nullable_to_non_nullable
as String,status: null == status ? _self.status : status // ignore: cast_nullable_to_non_nullable
as String,amount: freezed == amount ? _self.amount : amount // ignore: cast_nullable_to_non_nullable
as String?,currentStep: null == currentStep ? _self.currentStep : currentStep // ignore: cast_nullable_to_non_nullable
as int,officeId: freezed == officeId ? _self.officeId : officeId // ignore: cast_nullable_to_non_nullable
as String?,targetId: freezed == targetId ? _self.targetId : targetId // ignore: cast_nullable_to_non_nullable
as String?,targetEntity: freezed == targetEntity ? _self.targetEntity : targetEntity // ignore: cast_nullable_to_non_nullable
as String?,reason: freezed == reason ? _self.reason : reason // ignore: cast_nullable_to_non_nullable
as String?,requestedById: null == requestedById ? _self.requestedById : requestedById // ignore: cast_nullable_to_non_nullable
as String,requestedByName: freezed == requestedByName ? _self.requestedByName : requestedByName // ignore: cast_nullable_to_non_nullable
as String?,requestedByRole: freezed == requestedByRole ? _self.requestedByRole : requestedByRole // ignore: cast_nullable_to_non_nullable
as String?,officeName: freezed == officeName ? _self.officeName : officeName // ignore: cast_nullable_to_non_nullable
as String?,decidedById: freezed == decidedById ? _self.decidedById : decidedById // ignore: cast_nullable_to_non_nullable
as String?,decisionNote: freezed == decisionNote ? _self.decisionNote : decisionNote // ignore: cast_nullable_to_non_nullable
as String?,createdAt: freezed == createdAt ? _self.createdAt : createdAt // ignore: cast_nullable_to_non_nullable
as DateTime?,payload: freezed == payload ? _self._payload : payload // ignore: cast_nullable_to_non_nullable
as Map<String, dynamic>?,steps: null == steps ? _self._steps : steps // ignore: cast_nullable_to_non_nullable
as List<RequestStepDto>,
  ));
}


}

// dart format on
