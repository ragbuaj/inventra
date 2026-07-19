import 'dart:async';

import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:inventra_mobile/core/api/app_failure.dart';
import 'package:inventra_mobile/core/auth/auth_controller.dart';
import 'package:inventra_mobile/core/auth/auth_session.dart';
import 'package:inventra_mobile/core/auth/data/user_dto.dart';

/// Pengguna dummy untuk sesi [Authenticated] di tes widget.
const UserDto fakeUser = UserDto(
  id: 'user-1',
  name: 'Budi Santoso',
  email: 'budi.santoso@bank.co.id',
  roleId: 'role-1',
  status: 'active',
  googleLinked: false,
);

/// AuthController palsu untuk tes router/shell/login: build tidak menyentuh
/// SessionManager (tidak ada platform channel), login/logout diskrip dari tes.
class FakeAuthController extends AuthController {
  FakeAuthController({
    this.initialSession = const Unauthenticated(),
    this.failureOnLogin,
    this.holdLogin = false,
  });

  final AuthSession initialSession;

  /// Bila diisi, login berakhir `AsyncError` berisi failure ini.
  final AppFailure? failureOnLogin;

  /// Bila true, login menggantung sampai [releaseLogin] — untuk state loading.
  final bool holdLogin;

  final Completer<void> _loginGate = Completer<void>();
  final List<({String email, String password})> loginCalls =
      <({String email, String password})>[];
  int logoutCalls = 0;

  void releaseLogin() {
    if (!_loginGate.isCompleted) {
      _loginGate.complete();
    }
  }

  @override
  Future<AuthSession> build() async => initialSession;

  @override
  Future<void> login({required String email, required String password}) async {
    loginCalls.add((email: email, password: password));
    state = const AsyncLoading<AuthSession>();
    if (holdLogin) {
      await _loginGate.future;
    }
    final AppFailure? failure = failureOnLogin;
    state = failure == null
        ? const AsyncData<AuthSession>(Authenticated(fakeUser))
        : AsyncError<AuthSession>(failure, StackTrace.current);
  }

  @override
  Future<void> logout() async {
    logoutCalls += 1;
    state = const AsyncData<AuthSession>(Unauthenticated());
  }
}
