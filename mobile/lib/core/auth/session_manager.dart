import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../api/app_failure.dart';
import 'data/auth_repository.dart';
import 'refresh_outcome.dart';
import 'token_storage.dart';

/// Pasangan token hasil refresh yang diadopsi [SessionManager].
typedef SessionTokens = ({String accessToken, String? refreshToken});

/// Menjalankan `POST /auth/refresh`; melempar [AppFailure] bila gagal.
typedef RefreshExecutor = Future<SessionTokens> Function(String refreshToken);

/// Pemegang sesi runtime (ARCHITECTURE bagian 6): access token hanya di
/// memori, refresh token lewat [TokenStorage], dan refresh **single-flight** —
/// pemanggil bersamaan (interceptor 401 maupun cold start) menunggu satu
/// proses refresh yang sama.
class SessionManager {
  SessionManager({required this._tokenStorage, required this._refreshExecutor});

  final TokenStorage _tokenStorage;
  final RefreshExecutor _refreshExecutor;

  /// Access token aktif; hanya di memori, dibaca AuthInterceptor per request.
  String? accessToken;

  /// Dipasang oleh AuthController; dipanggil saat sesi dinyatakan mati.
  void Function()? onSessionExpired;

  Future<RefreshOutcome>? _inFlightRefresh;

  Future<String?> readRefreshToken() => _tokenStorage.readRefreshToken();

  /// Mengadopsi token hasil login/refresh: access ke memori, refresh (bila
  /// ada — rotasi selalu mengirim yang baru) ke secure storage.
  Future<void> adoptTokens(SessionTokens tokens) async {
    accessToken = tokens.accessToken;
    final String? refreshToken = tokens.refreshToken;
    if (refreshToken != null && refreshToken.isNotEmpty) {
      await _tokenStorage.saveRefreshToken(refreshToken);
    }
  }

  /// Refresh single-flight. Lihat [RefreshOutcome]: penolakan definitif
  /// menghapus refresh token di sini juga (token sudah mati di server);
  /// kegagalan jaringan mempertahankannya supaya bisa dicoba lagi.
  Future<RefreshOutcome> refresh() {
    return _inFlightRefresh ??= _doRefresh().whenComplete(() {
      _inFlightRefresh = null;
    });
  }

  Future<RefreshOutcome> _doRefresh() async {
    final String? storedToken = await _tokenStorage.readRefreshToken();
    if (storedToken == null || storedToken.isEmpty) {
      return RefreshOutcome.rejected;
    }
    try {
      final SessionTokens tokens = await _refreshExecutor(storedToken);
      await adoptTokens(tokens);
      return RefreshOutcome.success;
    } on UnauthorizedFailure {
      // Hanya penolakan otentik token oleh server yang menghancurkan sesi.
      await clear();
      return RefreshOutcome.rejected;
    } on AppFailure {
      // Konservatif untuk aplikasi lapangan: jaringan, 5xx, rate limit, dan
      // kegagalan lain dianggap sementara — token dipertahankan, dicoba lagi.
      return RefreshOutcome.networkFailed;
    } catch (_) {
      // Error tak terduga di luar AppFailure (mis. parse response.data null)
      // tidak boleh lolos single-flight sebagai error mentah. Diperlakukan
      // sama konservatifnya: sesi dipertahankan, dicoba lagi nanti.
      return RefreshOutcome.networkFailed;
    }
  }

  /// Membersihkan sesi: memori dan secure storage.
  Future<void> clear() async {
    accessToken = null;
    await _tokenStorage.clear();
  }

  /// Dipanggil AuthInterceptor saat refresh ditolak definitif pada 401.
  void notifySessionExpired() => onSessionExpired?.call();
}

final Provider<SessionManager>
sessionManagerProvider = Provider<SessionManager>((Ref ref) {
  return SessionManager(
    tokenStorage: ref.watch(tokenStorageProvider),
    // ref.read (bukan watch) dan lazy: repository baru dibangun saat refresh
    // pertama, memutus siklus init dio -> session manager -> repository -> dio.
    refreshExecutor: (String refreshToken) async {
      final tokenResponse = await ref
          .read(authRepositoryProvider)
          .refresh(refreshToken: refreshToken);
      return (
        accessToken: tokenResponse.accessToken,
        refreshToken: tokenResponse.refreshToken,
      );
    },
  );
});
