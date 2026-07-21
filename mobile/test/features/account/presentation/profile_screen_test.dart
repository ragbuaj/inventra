import 'dart:convert';
import 'dart:typed_data';

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:inventra_mobile/core/auth/auth_controller.dart';
import 'package:inventra_mobile/core/auth/auth_session.dart';
import 'package:inventra_mobile/core/auth/data/user_dto.dart';
import 'package:inventra_mobile/core/masterdata/reference_lookup_repository.dart';
import 'package:inventra_mobile/core/utils/clock.dart';
import 'package:inventra_mobile/core/widgets/app_skeleton.dart';
import 'package:inventra_mobile/features/account/data/account_repository.dart';
import 'package:inventra_mobile/features/account/data/profile_dto.dart';
import 'package:inventra_mobile/features/account/data/session_dto.dart';
import 'package:inventra_mobile/features/account/presentation/profile_screen.dart';

import '../../../helpers/fake_account_repository.dart';
import '../../../helpers/fake_auth_controller.dart';
import '../../../helpers/fake_reference_lookup.dart';
import '../../../helpers/test_app.dart';

final DateTime _frozenNow = DateTime(2026, 7, 19, 9, 41);

const UserDto _user = UserDto(
  id: 'user-1',
  name: 'Andi Saputra',
  email: 'andi.saputra@bank.co.id',
  roleId: 'role-1',
  officeId: 'office-1',
  status: 'active',
  googleLinked: false,
);

/// PNG transparan 1x1 valid — cukup untuk merender Image.memory di tes.
final Uint8List _tinyPng = Uint8List.fromList(const <int>[
  0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A, 0x00, 0x00, 0x00, 0x0D, //
  0x49, 0x48, 0x44, 0x52, 0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x01, //
  0x08, 0x06, 0x00, 0x00, 0x00, 0x1F, 0x15, 0xC4, 0x89, 0x00, 0x00, 0x00, //
  0x0A, 0x49, 0x44, 0x41, 0x54, 0x78, 0x9C, 0x63, 0x00, 0x01, 0x00, 0x00, //
  0x05, 0x00, 0x01, 0x0D, 0x0A, 0x2D, 0xB4, 0x00, 0x00, 0x00, 0x00, 0x49, //
  0x45, 0x4E, 0x44, 0xAE, 0x42, 0x60, 0x82,
]);

SessionDto _session({
  required String id,
  required String os,
  required String browser,
  String deviceType = 'desktop',
  String location = 'Jakarta, Indonesia',
  String ip = '103.28.11.8',
  Duration lastSeenAgo = const Duration(hours: 2),
  bool current = false,
}) {
  return SessionDto(
    id: id,
    browser: browser,
    os: os,
    deviceType: deviceType,
    ipAddress: ip,
    location: location,
    createdAt: _frozenNow.subtract(const Duration(days: 10)),
    lastSeenAt: _frozenNow.subtract(lastSeenAgo),
    current: current,
  );
}

/// Tiga sesi paritas mockup: sesi ini (mobile) + web Windows (2 jam lalu) +
/// mobile lain (3 hari lalu, Depok).
List<SessionDto> _threeSessions() => <SessionDto>[
  _session(
    id: 'sess-current',
    os: 'Android',
    browser: 'Inventra App',
    deviceType: 'mobile',
    ip: '103.28.11.4',
    lastSeenAgo: Duration.zero,
    current: true,
  ),
  _session(id: 'sess-web', os: 'Windows', browser: 'Chrome'),
  _session(
    id: 'sess-old',
    os: 'Android',
    browser: 'Chrome',
    deviceType: 'mobile',
    location: 'Depok, Indonesia',
    ip: '114.10.20.5',
    lastSeenAgo: const Duration(days: 3),
  ),
];

void main() {
  late FakeAuthController auth;

  ProviderContainer createContainer(FakeAccountRepository repository) {
    auth = FakeAuthController(initialSession: const Authenticated(_user));
    return ProviderContainer.test(
      overrides: [
        authControllerProvider.overrideWith(() => auth),
        accountRepositoryProvider.overrideWithValue(repository),
        referenceLookupRepositoryProvider.overrideWithValue(
          FakeReferenceLookup(<String, String>{
            'office:office-1': 'Cabang Jakarta Selatan',
          }),
        ),
        clockProvider.overrideWithValue(() => _frozenNow),
      ],
    );
  }

  Future<ProviderContainer> pumpProfile(
    WidgetTester tester,
    FakeAccountRepository repository, {
    bool settle = true,
  }) async {
    tester.view.physicalSize = const Size(500, 2000);
    tester.view.devicePixelRatio = 1.0;
    addTearDown(tester.view.reset);
    final ProviderContainer container = createContainer(repository);
    await tester.pumpWidget(
      buildScreenHarness(container: container, child: const ProfileScreen()),
    );
    if (settle) {
      await tester.pumpAndSettle();
    }
    return container;
  }

  group('detail profil (GET /auth/profile)', () {
    testWidgets('kartu Detail Pegawai + Informasi Akun terisi', (
      WidgetTester tester,
    ) async {
      await pumpProfile(
        tester,
        FakeAccountRepository(sessions: _threeSessions()),
      );

      expect(find.text(l10nId.profileEmployeeDetailTitle), findsOneWidget);
      expect(find.text('EMP-001'), findsOneWidget);
      expect(find.text('Umum & GA'), findsOneWidget);
      expect(find.text('Staf Aset'), findsOneWidget);
      expect(find.text(l10nId.profileAccountInfoTitle), findsOneWidget);
      expect(find.text('andi@inventra.local'), findsOneWidget);
      expect(find.text(l10nId.profileLoginEmail), findsOneWidget);
    });

    testWidgets('akun tanpa pegawai: catatan, bukan grid kosong', (
      WidgetTester tester,
    ) async {
      await pumpProfile(
        tester,
        FakeAccountRepository(
          sessions: _threeSessions(),
          profile: const ProfileDto(
            id: 'u2',
            name: 'Admin',
            email: 'admin@inventra.local',
          ),
        ),
      );

      expect(find.text(l10nId.profileNoEmployee), findsOneWidget);
    });
  });

  group('ubah data diri (PUT /auth/profile)', () {
    testWidgets('Ubah -> Simpan memanggil updateProfile + nilai baru', (
      WidgetTester tester,
    ) async {
      final FakeAccountRepository repo = FakeAccountRepository(
        sessions: _threeSessions(),
      );
      await pumpProfile(tester, repo);

      await tester.tap(find.byKey(const ValueKey<String>('profile-edit')));
      await tester.pumpAndSettle();
      await tester.enterText(find.byType(TextField).first, 'Budi Hartono');
      await tester.tap(find.byKey(const ValueKey<String>('profile-save')));
      await tester.pumpAndSettle();

      expect(repo.updateCalls, hasLength(1));
      expect(repo.updateCalls.first.$1, 'Budi Hartono');
      expect(find.text(l10nId.profileUpdateSuccess), findsOneWidget);
      expect(find.text('Budi Hartono'), findsWidgets);
    });

    testWidgets('nama kosong: validasi menahan simpan', (
      WidgetTester tester,
    ) async {
      final FakeAccountRepository repo = FakeAccountRepository(
        sessions: _threeSessions(),
      );
      await pumpProfile(tester, repo);

      await tester.tap(find.byKey(const ValueKey<String>('profile-edit')));
      await tester.pumpAndSettle();
      await tester.enterText(find.byType(TextField).first, '   ');
      await tester.tap(find.byKey(const ValueKey<String>('profile-save')));
      await tester.pumpAndSettle();

      expect(find.text(l10nId.profileNameRequired), findsOneWidget);
      expect(repo.updateCalls, isEmpty);
    });

    testWidgets('Batal: kembali ke mode baca tanpa update', (
      WidgetTester tester,
    ) async {
      final FakeAccountRepository repo = FakeAccountRepository(
        sessions: _threeSessions(),
      );
      await pumpProfile(tester, repo);

      await tester.tap(find.byKey(const ValueKey<String>('profile-edit')));
      await tester.pumpAndSettle();
      await tester.tap(find.text(l10nId.commonCancel));
      await tester.pumpAndSettle();

      expect(find.byKey(const ValueKey<String>('profile-edit')), findsOneWidget);
      expect(repo.updateCalls, isEmpty);
    });
  });

  group('avatar (POST/DELETE /auth/avatar)', () {
    final Uint8List png = base64Decode(
      'iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mNk+M9QDwADhgGAWjR9awAAAABJRU5ErkJggg==',
    );

    testWidgets('foto ada: badge -> Hapus memanggil deleteAvatar', (
      WidgetTester tester,
    ) async {
      final FakeAccountRepository repo = FakeAccountRepository(
        sessions: _threeSessions(),
        avatarBytes: png,
      );
      await pumpProfile(tester, repo);

      await tester.tap(
        find.byKey(const ValueKey<String>('profile-avatar-edit')),
      );
      await tester.pumpAndSettle();
      await tester.tap(find.byKey(const ValueKey<String>('avatar-remove')));
      await tester.pumpAndSettle();

      expect(repo.deleteAvatarCalls, 1);
      expect(find.text(l10nId.avatarRemoved), findsOneWidget);
    });

    testWidgets('tanpa foto: opsi Hapus tidak muncul', (
      WidgetTester tester,
    ) async {
      await pumpProfile(
        tester,
        FakeAccountRepository(sessions: _threeSessions()),
      );

      await tester.tap(
        find.byKey(const ValueKey<String>('profile-avatar-edit')),
      );
      await tester.pumpAndSettle();

      expect(find.byKey(const ValueKey<String>('avatar-remove')), findsNothing);
      expect(find.text(l10nId.avatarFromGallery), findsOneWidget);
    });
  });

  group('kartu identitas', () {
    testWidgets('nama, email, inisial avatar, dan kantor (lookup)', (
      WidgetTester tester,
    ) async {
      await pumpProfile(
        tester,
        FakeAccountRepository(sessions: _threeSessions()),
      );

      // Nama tampil di header identitas dan kartu Data Diri (read view).
      expect(find.text('Andi Saputra'), findsWidgets);
      expect(find.text('andi.saputra@bank.co.id'), findsOneWidget);
      expect(find.text('AS'), findsOneWidget);
      expect(find.text('Cabang Jakarta Selatan'), findsOneWidget);
      expect(find.text(l10nId.accountEditOnWeb), findsOneWidget);
      expect(
        find.byKey(const ValueKey<String>('profile-avatar-photo')),
        findsNothing,
      );
    });

    testWidgets('foto avatar dirender bila endpoint mengembalikan bytes', (
      WidgetTester tester,
    ) async {
      const UserDto withAvatar = UserDto(
        id: 'user-1',
        name: 'Andi Saputra',
        email: 'andi.saputra@bank.co.id',
        roleId: 'role-1',
        status: 'active',
        hasAvatar: true,
        googleLinked: false,
      );
      final FakeAccountRepository repository = FakeAccountRepository(
        sessions: _threeSessions(),
        avatarBytes: _tinyPng,
      );
      final ProviderContainer container = ProviderContainer.test(
        overrides: [
          authControllerProvider.overrideWith(
            () => FakeAuthController(
              initialSession: const Authenticated(withAvatar),
            ),
          ),
          accountRepositoryProvider.overrideWithValue(repository),
          referenceLookupRepositoryProvider.overrideWithValue(
            FakeReferenceLookup(),
          ),
          clockProvider.overrideWithValue(() => _frozenNow),
        ],
      );
      await tester.pumpWidget(
        buildScreenHarness(container: container, child: const ProfileScreen()),
      );
      await tester.pumpAndSettle();

      expect(
        find.byKey(const ValueKey<String>('profile-avatar-photo')),
        findsOneWidget,
      );
      expect(find.text('AS'), findsNothing);
    });
  });

  group('daftar sesi', () {
    testWidgets('sesi ini ditandai badge tanpa tombol Cabut; sesi lain '
        'punya tombol', (WidgetTester tester) async {
      await pumpProfile(
        tester,
        FakeAccountRepository(sessions: _threeSessions()),
      );

      expect(find.text(l10nId.accountSessionCurrentBadge), findsOneWidget);
      expect(find.text('Android · Inventra App'), findsOneWidget);
      expect(
        find.byKey(
          const ValueKey<String>('account-session-revoke-sess-current'),
        ),
        findsNothing,
      );
      expect(
        find.byKey(const ValueKey<String>('account-session-revoke-sess-web')),
        findsOneWidget,
      );
      expect(
        find.byKey(const ValueKey<String>('account-session-revoke-sess-old')),
        findsOneWidget,
      );
    });

    testWidgets('subjudul: sesi ini "aktif sekarang", sesi lain waktu '
        'relatif + lokasi + IP', (WidgetTester tester) async {
      await pumpProfile(
        tester,
        FakeAccountRepository(sessions: _threeSessions()),
      );

      expect(
        find.text(
          'Jakarta, Indonesia · 103.28.11.4 · '
          '${l10nId.accountSessionActiveNow}',
        ),
        findsOneWidget,
      );
      expect(
        find.text(
          'Jakarta, Indonesia · 103.28.11.8 · '
          '${l10nId.accountTimeHoursAgo(2)}',
        ),
        findsOneWidget,
      );
      expect(
        find.text(
          'Depok, Indonesia · 114.10.20.5 · ${l10nId.accountTimeDaysAgo(3)}',
        ),
        findsOneWidget,
      );
    });

    testWidgets('revoke: konfirmasi -> baris hilang + snackbar sukses', (
      WidgetTester tester,
    ) async {
      final FakeAccountRepository repository = FakeAccountRepository(
        sessions: _threeSessions(),
      );
      await pumpProfile(tester, repository);

      await tester.tap(
        find.byKey(const ValueKey<String>('account-session-revoke-sess-web')),
      );
      await tester.pumpAndSettle();
      expect(
        find.text(l10nId.accountSessionRevokeConfirmTitle),
        findsOneWidget,
      );
      expect(
        find.text(l10nId.accountSessionRevokeConfirmBody('Windows · Chrome')),
        findsOneWidget,
      );

      await tester.tap(find.text(l10nId.accountSessionRevokeConfirmAction));
      await tester.pumpAndSettle();

      expect(repository.revokeCalls, <String>['sess-web']);
      expect(
        find.byKey(const ValueKey<String>('account-session-sess-web')),
        findsNothing,
      );
      expect(
        find.text(l10nId.accountSessionRevokedSnack('Windows · Chrome')),
        findsOneWidget,
      );
      // Sesi lain tetap ada.
      expect(
        find.byKey(const ValueKey<String>('account-session-sess-old')),
        findsOneWidget,
      );
    });

    testWidgets('revoke batal: tanpa panggilan server, baris tetap', (
      WidgetTester tester,
    ) async {
      final FakeAccountRepository repository = FakeAccountRepository(
        sessions: _threeSessions(),
      );
      await pumpProfile(tester, repository);

      await tester.tap(
        find.byKey(const ValueKey<String>('account-session-revoke-sess-web')),
      );
      await tester.pumpAndSettle();
      await tester.tap(find.text(l10nId.commonCancel));
      await tester.pumpAndSettle();

      expect(repository.revokeCalls, isEmpty);
      expect(
        find.byKey(const ValueKey<String>('account-session-sess-web')),
        findsOneWidget,
      );
    });

    testWidgets('revoke gagal: snackbar gagal, baris tetap di daftar', (
      WidgetTester tester,
    ) async {
      final FakeAccountRepository repository = FakeAccountRepository(
        sessions: _threeSessions(),
        failRevoke: true,
      );
      await pumpProfile(tester, repository);

      await tester.tap(
        find.byKey(const ValueKey<String>('account-session-revoke-sess-web')),
      );
      await tester.pumpAndSettle();
      await tester.tap(find.text(l10nId.accountSessionRevokeConfirmAction));
      await tester.pumpAndSettle();

      expect(find.text(l10nId.accountSessionRevokeFailed), findsOneWidget);
      expect(
        find.byKey(const ValueKey<String>('account-session-sess-web')),
        findsOneWidget,
      );
    });

    testWidgets('keluar semua perangkat lain: konfirmasi berjumlah -> '
        'hanya sesi ini tersisa', (WidgetTester tester) async {
      final FakeAccountRepository repository = FakeAccountRepository(
        sessions: _threeSessions(),
      );
      await pumpProfile(tester, repository);

      await tester.tap(
        find.byKey(const ValueKey<String>('profile-revoke-others')),
      );
      await tester.pumpAndSettle();
      expect(
        find.text(l10nId.accountRevokeOthersConfirmBody(2)),
        findsOneWidget,
      );

      await tester.tap(find.text(l10nId.accountRevokeOthersConfirmAction));
      await tester.pumpAndSettle();

      expect(repository.revokeOthersCalls, 1);
      expect(
        find.byKey(const ValueKey<String>('account-session-sess-web')),
        findsNothing,
      );
      expect(
        find.byKey(const ValueKey<String>('account-session-sess-old')),
        findsNothing,
      );
      expect(
        find.byKey(const ValueKey<String>('account-session-sess-current')),
        findsOneWidget,
      );
      // Tanpa sesi lain, tombol keluar-semua ikut hilang.
      expect(
        find.byKey(const ValueKey<String>('profile-revoke-others')),
        findsNothing,
      );
    });

    testWidgets('keluar semua gagal: snackbar, daftar utuh', (
      WidgetTester tester,
    ) async {
      final FakeAccountRepository repository = FakeAccountRepository(
        sessions: _threeSessions(),
        failRevokeOthers: true,
      );
      await pumpProfile(tester, repository);

      await tester.tap(
        find.byKey(const ValueKey<String>('profile-revoke-others')),
      );
      await tester.pumpAndSettle();
      await tester.tap(find.text(l10nId.accountRevokeOthersConfirmAction));
      await tester.pumpAndSettle();

      expect(find.text(l10nId.accountRevokeOthersFailed), findsOneWidget);
      expect(
        find.byKey(const ValueKey<String>('account-session-sess-web')),
        findsOneWidget,
      );
    });
  });

  group('state layar', () {
    testWidgets('loading: skeleton kartu sesi', (WidgetTester tester) async {
      await pumpProfile(
        tester,
        FakeAccountRepository(sessions: _threeSessions()),
        settle: false,
      );

      expect(find.byType(AppSkeleton), findsWidgets);
      expect(
        find.byKey(const ValueKey<String>('account-session-sess-web')),
        findsNothing,
      );
    });

    testWidgets('empty: kartu menampilkan pesan kosong tanpa tombol '
        'keluar-semua', (WidgetTester tester) async {
      await pumpProfile(
        tester,
        FakeAccountRepository(sessions: <SessionDto>[]),
      );

      expect(find.text(l10nId.accountSessionsEmpty), findsOneWidget);
      expect(
        find.byKey(const ValueKey<String>('profile-revoke-others')),
        findsNothing,
      );
    });

    testWidgets('error: pesan + Coba lagi memuat ulang daftar', (
      WidgetTester tester,
    ) async {
      final FakeAccountRepository repository = FakeAccountRepository(
        sessions: _threeSessions(),
        failSessions: true,
      );
      await pumpProfile(tester, repository);

      expect(find.text(l10nId.accountSessionsErrorBody), findsOneWidget);

      repository.failSessions = false;
      await tester.tap(find.text(l10nId.commonRetry));
      await tester.pumpAndSettle();

      expect(
        find.byKey(const ValueKey<String>('account-session-sess-web')),
        findsOneWidget,
      );
    });
  });

  group('logout', () {
    testWidgets('tombol Keluar berdialog; konfirmasi memanggil logout', (
      WidgetTester tester,
    ) async {
      await pumpProfile(
        tester,
        FakeAccountRepository(sessions: _threeSessions()),
      );

      await tester.tap(find.byKey(const ValueKey<String>('profile-logout')));
      await tester.pumpAndSettle();
      expect(find.text(l10nId.accountLogoutConfirmTitle), findsOneWidget);

      await tester.tap(find.text(l10nId.accountLogoutConfirmAction));
      await tester.pumpAndSettle();

      expect(auth.logoutCalls, 1);
    });

    testWidgets('batal: sesi dipertahankan', (WidgetTester tester) async {
      await pumpProfile(
        tester,
        FakeAccountRepository(sessions: _threeSessions()),
      );

      await tester.tap(find.byKey(const ValueKey<String>('profile-logout')));
      await tester.pumpAndSettle();
      await tester.tap(find.text(l10nId.commonCancel));
      await tester.pumpAndSettle();

      expect(auth.logoutCalls, 0);
    });
  });
}
