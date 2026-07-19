import 'dart:typed_data';

import 'package:inventra_mobile/core/api/app_failure.dart';
import 'package:inventra_mobile/features/account/data/account_repository.dart';
import 'package:inventra_mobile/features/account/data/session_dto.dart';

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
    this.avatarBytes,
  }) : sessionsData = List<SessionDto>.of(
         sessions ?? <SessionDto>[fakeCurrentSession],
       );

  final List<SessionDto> sessionsData;
  bool failSessions;
  bool failRevoke;
  bool failRevokeOthers;
  Uint8List? avatarBytes;

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
  Future<Uint8List?> avatar() async => avatarBytes;
}
