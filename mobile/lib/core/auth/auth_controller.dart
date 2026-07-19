import 'dart:async';

import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../api/app_failure.dart';
import '../masterdata/reference_lookup_repository.dart';
import 'auth_session.dart';
import 'data/auth_repository.dart';
import 'data/token_response_dto.dart';
import 'data/user_dto.dart';
import 'refresh_outcome.dart';
import 'session_manager.dart';

/// State sesi global (ARCHITECTURE bagian 2 dan 6).
///
/// Deklarasi `AsyncNotifierProvider` manual tanpa codegen mengikuti dokumentasi
/// resmi Riverpod 3: https://riverpod.dev/docs/concepts2/providers
final AsyncNotifierProvider<AuthController, AuthSession>
authControllerProvider = AsyncNotifierProvider<AuthController, AuthSession>(
  AuthController.new,
);

class AuthController extends AsyncNotifier<AuthSession> {
  /// Cold start (ARCHITECTURE bagian 6): baca refresh token dari secure
  /// storage, coba `POST /auth/refresh`, lalu `GET /auth/me`. Kegagalan
  /// berarti pengguna ke login — build tidak pernah melempar supaya router
  /// cukup melihat [Unauthenticated]. Kegagalan JARINGAN mempertahankan
  /// refresh token (launch berikutnya mencoba lagi); hanya penolakan
  /// definitif yang membersihkan storage.
  @override
  Future<AuthSession> build() async {
    final SessionManager session = ref.watch(sessionManagerProvider);
    session.onSessionExpired = _handleSessionExpired;
    ref.onDispose(() {
      if (session.onSessionExpired == _handleSessionExpired) {
        session.onSessionExpired = null;
      }
    });

    switch (await session.refresh()) {
      case RefreshOutcome.rejected:
        await session.clear();
        return const Unauthenticated();
      case RefreshOutcome.networkFailed:
        // Offline saat cold start: token tetap tersimpan, tanpa sesi aktif.
        return const Unauthenticated();
      case RefreshOutcome.success:
        break;
    }
    try {
      final UserDto user = await ref.read(authRepositoryProvider).me();
      return Authenticated(user);
    } on NetworkFailure {
      // Sesi sebenarnya hidup tapi profil tidak terambil; jangan buang
      // refresh token — cukup kembali ke login sampai jaringan pulih.
      session.accessToken = null;
      return const Unauthenticated();
    } on AppFailure {
      await session.clear();
      return const Unauthenticated();
    }
  }

  /// Login per kontrak `POST /auth/login` (klien mobile menerima
  /// `refresh_token` di body, tanpa cookie). Kegagalan menjadi `AsyncError`
  /// berisi [AppFailure] — layar login yang menerjemahkannya ke i18n.
  Future<void> login({required String email, required String password}) async {
    state = const AsyncLoading<AuthSession>();
    state = await AsyncValue.guard<AuthSession>(() async {
      final AuthRepository repository = ref.read(authRepositoryProvider);
      final TokenResponseDto tokens = await repository.login(
        email: email,
        password: password,
      );
      await ref.read(sessionManagerProvider).adoptTokens((
        accessToken: tokens.accessToken,
        refreshToken: tokens.refreshToken,
      ));
      final UserDto user = await repository.me();
      return Authenticated(user);
    });
  }

  /// Logout: panggil endpoint (best effort), lalu bersihkan storage + memori
  /// apa pun hasilnya — sesi lokal selalu berakhir bersih.
  Future<void> logout() async {
    final SessionManager session = ref.read(sessionManagerProvider);
    try {
      final String? refreshToken = await session.readRefreshToken();
      if (refreshToken != null && refreshToken.isNotEmpty) {
        await ref
            .read(authRepositoryProvider)
            .logout(refreshToken: refreshToken);
      }
    } on AppFailure {
      // Diabaikan dengan sengaja: kegagalan server tidak boleh menahan
      // pembersihan sesi lokal.
    } finally {
      await session.clear();
      ref.read(referenceLookupRepositoryProvider).clear();
      state = const AsyncData<AuthSession>(Unauthenticated());
    }
  }

  /// Dipanggil SessionManager saat refresh ditolak definitif pada 401 (sesi
  /// dicabut/kedaluwarsa di server) — bersihkan lokal tanpa memanggil
  /// endpoint. Kegagalan jaringan tidak pernah sampai ke sini.
  void _handleSessionExpired() {
    unawaited(ref.read(sessionManagerProvider).clear());
    ref.read(referenceLookupRepositoryProvider).clear();
    state = const AsyncData<AuthSession>(Unauthenticated());
  }
}
