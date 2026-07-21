import 'dart:typed_data';

import 'package:inventra_mobile/core/api/app_failure.dart';
import 'package:inventra_mobile/features/account/data/account_repository.dart';
import 'package:inventra_mobile/features/account/data/profile_dto.dart';
import 'package:inventra_mobile/features/account/data/session_dto.dart';

/// Profil default untuk tes (akun tertaut pegawai).
final ProfileDto fakeProfile = ProfileDto(
  id: 'user-1',
  name: 'Andi Saputra',
  email: 'andi@inventra.local',
  phone: '0812-3456-7890',
  roleName: 'Asset Manager',
  officeName: 'Cabang Jakarta Selatan',
  employeeName: 'Andi Saputra',
  employeeCode: 'EMP-001',
  employeeStatus: 'Aktif',
  departmentName: 'Umum & GA',
  positionName: 'Staf Aset',
  joinedAt: DateTime(2026, 1, 15),
);

/// Sesi "perangkat ini" default untuk tes yang hanya butuh daftar valid.
final SessionDto fakeCurrentSession = SessionDto(
  id: 'sess-current',
  browser: 'Inventra App',
  os: 'Android',
  deviceType: 'mobile',
  ipAddress: '103.28.11.4',
  location: 'Jakarta, Indonesia',
  createdAt: DateTime(2026, 7, 1, 8),
  lastSeenAt: DateTime(2026, 7, 19, 9, 41),
  current: true,
);

/// [AccountRepository] palsu berbasis data in-memory untuk widget/golden/
/// router test — tanpa Dio/HTTP. revokeSession/revokeOtherSessions diterapkan
/// ke [sessionsData]; kegagalan bisa diskrip per operasi (field mutable
/// supaya tes retry bisa memulihkannya di tengah jalan).
class FakeAccountRepository implements AccountRepository {
  FakeAccountRepository({
    List<SessionDto>? sessions,
    this.failSessions = false,
    this.failRevoke = false,
    this.failRevokeOthers = false,
    this.failProfile = false,
    this.avatarBytes,
    ProfileDto? profile,
  }) : sessionsData = List<SessionDto>.of(
         sessions ?? <SessionDto>[fakeCurrentSession],
       ),
       profileData = profile ?? fakeProfile;

  final List<SessionDto> sessionsData;
  ProfileDto profileData;
  bool failSessions;
  bool failRevoke;
  bool failRevokeOthers;
  bool failProfile;
  bool failUpdate = false;
  Uint8List? avatarBytes;
  final List<(String, String?)> updateCalls = <(String, String?)>[];

  final List<String> revokeCalls = <String>[];
  int revokeOthersCalls = 0;
  int sessionsCalls = 0;

  @override
  Future<List<SessionDto>> sessions() async {
    sessionsCalls += 1;
    if (failSessions) {
      throw const NetworkFailure();
    }
    return List<SessionDto>.of(sessionsData);
  }

  @override
  Future<void> revokeSession(String id) async {
    revokeCalls.add(id);
    if (failRevoke) {
      throw const ServerFailure();
    }
    sessionsData.removeWhere((SessionDto session) => session.id == id);
  }

  @override
  Future<int> revokeOtherSessions() async {
    revokeOthersCalls += 1;
    if (failRevokeOthers) {
      throw const ServerFailure();
    }
    final int before = sessionsData.length;
    sessionsData.removeWhere((SessionDto session) => !session.current);
    return before - sessionsData.length;
  }

  @override
  Future<ProfileDto> getProfile() async {
    if (failProfile) {
      throw const NetworkFailure();
    }
    return profileData;
  }

  @override
  Future<ProfileDto> updateProfile({
    required String name,
    String? phone,
  }) async {
    updateCalls.add((name, phone));
    if (failUpdate) {
      throw const ServerFailure();
    }
    profileData = profileData.copyWith(name: name, phone: phone ?? '');
    return profileData;
  }

  @override
  Future<Uint8List?> avatar() async => avatarBytes;
}
