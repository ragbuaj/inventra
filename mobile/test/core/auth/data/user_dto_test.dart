import 'package:flutter_test/flutter_test.dart';
import 'package:inventra_mobile/core/auth/data/user_dto.dart';

void main() {
  test('fromJson memetakan seluruh field kontrak User', () {
    final UserDto user = UserDto.fromJson(<String, dynamic>{
      'id': 'user-1',
      'name': 'Ragil',
      'email': 'ragil@inventra.local',
      'role_id': 'role-1',
      'office_id': 'office-1',
      'employee_id': 'emp-1',
      'status': 'active',
      'has_avatar': true,
      'google_linked': true,
      'created_at': '2026-07-19T08:00:00Z',
      'updated_at': null,
    });

    expect(user.id, 'user-1');
    expect(user.name, 'Ragil');
    expect(user.email, 'ragil@inventra.local');
    expect(user.roleId, 'role-1');
    expect(user.officeId, 'office-1');
    expect(user.employeeId, 'emp-1');
    expect(user.status, 'active');
    expect(user.hasAvatar, isTrue);
    expect(user.googleLinked, isTrue);
    expect(user.createdAt, DateTime.utc(2026, 7, 19, 8));
    expect(user.updatedAt, isNull);
  });

  test(
    'field opsional absen: nullable menjadi null, has_avatar default false',
    () {
      final UserDto user = UserDto.fromJson(<String, dynamic>{
        'id': 'user-1',
        'name': 'Ragil',
        'email': 'ragil@inventra.local',
        'role_id': 'role-1',
        'status': 'active',
        'google_linked': false,
      });

      expect(user.officeId, isNull);
      expect(user.employeeId, isNull);
      expect(user.hasAvatar, isFalse);
      expect(user.createdAt, isNull);
      expect(user.updatedAt, isNull);
    },
  );

  test('toJson mengeluarkan kunci snake_case', () {
    const UserDto user = UserDto(
      id: 'user-1',
      name: 'Ragil',
      email: 'ragil@inventra.local',
      roleId: 'role-1',
      status: 'active',
      googleLinked: false,
    );

    final Map<String, dynamic> json = user.toJson();
    expect(json['role_id'], 'role-1');
    expect(json['google_linked'], isFalse);
    expect(json['has_avatar'], isFalse);
    expect(json.containsKey('roleId'), isFalse);
  });
}
