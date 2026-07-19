import 'dart:async';

import 'package:flutter_test/flutter_test.dart';
import 'package:inventra_mobile/core/api/app_failure.dart';
import 'package:inventra_mobile/core/auth/refresh_outcome.dart';
import 'package:inventra_mobile/core/auth/session_manager.dart';

import '../../helpers/fakes.dart';

void main() {
  test(
    'refresh tanpa token tersimpan: rejected, executor tidak dipanggil',
    () async {
      int calls = 0;
      final SessionManager session = SessionManager(
        tokenStorage: InMemoryTokenStorage(),
        refreshExecutor: (String refreshToken) async {
          calls += 1;
          return (accessToken: 'a', refreshToken: 'r');
        },
      );

      expect(await session.refresh(), RefreshOutcome.rejected);
      expect(calls, 0);
      expect(session.accessToken, isNull);
    },
  );

  test(
    'refresh sukses: access ke memori, refresh token rotasi tersimpan',
    () async {
      final InMemoryTokenStorage storage = InMemoryTokenStorage('rt-1');
      final SessionManager session = SessionManager(
        tokenStorage: storage,
        refreshExecutor: (String refreshToken) async {
          expect(refreshToken, 'rt-1');
          return (accessToken: 'access-1', refreshToken: 'rt-2');
        },
      );

      expect(await session.refresh(), RefreshOutcome.success);
      expect(session.accessToken, 'access-1');
      expect(storage.refreshToken, 'rt-2');
    },
  );

  test(
    'refresh ditolak backend (401): rejected dan refresh token DIHAPUS',
    () async {
      final InMemoryTokenStorage storage = InMemoryTokenStorage('rt-1');
      final SessionManager session = SessionManager(
        tokenStorage: storage,
        refreshExecutor: (String refreshToken) async =>
            throw const UnauthorizedFailure(),
      );

      expect(await session.refresh(), RefreshOutcome.rejected);
      expect(session.accessToken, isNull);
      expect(storage.refreshToken, isNull);
      expect(storage.clearCount, 1);
    },
  );

  // Konservatif: hanya UnauthorizedFailure yang menghancurkan sesi; kegagalan
  // lain (jaringan, 5xx/500, rate limit/429, tak terpetakan) dianggap
  // sementara.
  for (final AppFailure failure in <AppFailure>[
    const NetworkFailure(),
    const ServerFailure(),
    const RateLimitedFailure(),
    const UnknownFailure(),
  ]) {
    test('refresh gagal sementara (${failure.runtimeType}): networkFailed dan '
        'refresh token DIPERTAHANKAN', () async {
      final InMemoryTokenStorage storage = InMemoryTokenStorage('rt-1');
      final SessionManager session = SessionManager(
        tokenStorage: storage,
        refreshExecutor: (String refreshToken) async => throw failure,
      );

      expect(await session.refresh(), RefreshOutcome.networkFailed);
      expect(session.accessToken, isNull);
      expect(storage.refreshToken, 'rt-1');
      expect(storage.clearCount, 0);
    });
  }

  test('refresh gagal error non-AppFailure (mis. parse null): networkFailed '
      'dan refresh token DIPERTAHANKAN', () async {
    final InMemoryTokenStorage storage = InMemoryTokenStorage('rt-1');
    final SessionManager session = SessionManager(
      tokenStorage: storage,
      // Error tak terduga di luar AppFailure (mis. NoSuchMethodError saat
      // membaca field respons null) tidak boleh lolos sebagai error mentah.
      refreshExecutor: (String refreshToken) async =>
          throw StateError('unexpected'),
    );

    expect(await session.refresh(), RefreshOutcome.networkFailed);
    expect(session.accessToken, isNull);
    expect(storage.refreshToken, 'rt-1');
    expect(storage.clearCount, 0);
  });

  test(
    'single-flight: refresh bersamaan memakai satu executor yang sama',
    () async {
      int calls = 0;
      final Completer<void> gate = Completer<void>();
      final SessionManager session = SessionManager(
        tokenStorage: InMemoryTokenStorage('rt-1'),
        refreshExecutor: (String refreshToken) async {
          calls += 1;
          await gate.future;
          return (accessToken: 'access-1', refreshToken: 'rt-2');
        },
      );

      final Future<RefreshOutcome> first = session.refresh();
      final Future<RefreshOutcome> second = session.refresh();
      gate.complete();

      expect(
        await Future.wait(<Future<RefreshOutcome>>[first, second]),
        <RefreshOutcome>[RefreshOutcome.success, RefreshOutcome.success],
      );
      expect(calls, 1);
    },
  );

  test('setelah refresh selesai, panggilan berikutnya refresh lagi', () async {
    int calls = 0;
    final SessionManager session = SessionManager(
      tokenStorage: InMemoryTokenStorage('rt-1'),
      refreshExecutor: (String refreshToken) async {
        calls += 1;
        return (accessToken: 'access-$calls', refreshToken: 'rt-$calls');
      },
    );

    expect(await session.refresh(), RefreshOutcome.success);
    expect(await session.refresh(), RefreshOutcome.success);
    expect(calls, 2);
  });

  test('adoptTokens tanpa refresh token tidak menyentuh storage', () async {
    final InMemoryTokenStorage storage = InMemoryTokenStorage('rt-1');
    final SessionManager session = SessionManager(
      tokenStorage: storage,
      refreshExecutor: (String refreshToken) async =>
          (accessToken: 'a', refreshToken: null),
    );

    await session.adoptTokens((accessToken: 'access-1', refreshToken: null));

    expect(session.accessToken, 'access-1');
    expect(storage.refreshToken, 'rt-1');
    expect(storage.saveCount, 0);
  });

  test('clear membersihkan memori dan storage', () async {
    final InMemoryTokenStorage storage = InMemoryTokenStorage('rt-1');
    final SessionManager session = SessionManager(
      tokenStorage: storage,
      refreshExecutor: (String refreshToken) async =>
          (accessToken: 'a', refreshToken: 'r'),
    );
    session.accessToken = 'access-1';

    await session.clear();

    expect(session.accessToken, isNull);
    expect(storage.refreshToken, isNull);
    expect(storage.clearCount, 1);
  });

  test('notifySessionExpired memanggil callback bila terpasang', () {
    final SessionManager session = SessionManager(
      tokenStorage: InMemoryTokenStorage(),
      refreshExecutor: (String refreshToken) async =>
          (accessToken: 'a', refreshToken: 'r'),
    );

    // Tanpa callback: tidak melempar.
    session.notifySessionExpired();

    int calls = 0;
    session.onSessionExpired = () => calls += 1;
    session.notifySessionExpired();
    expect(calls, 1);
  });
}
