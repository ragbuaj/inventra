@Tags(<String>['golden'])
library;

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:inventra_mobile/app/theme.dart';
import 'package:inventra_mobile/core/auth/auth_controller.dart';
import 'package:inventra_mobile/core/auth/auth_session.dart';
import 'package:inventra_mobile/core/auth/data/user_dto.dart';
import 'package:inventra_mobile/core/i18n/gen/app_localizations.dart';
import 'package:inventra_mobile/core/masterdata/reference_lookup_repository.dart';
import 'package:inventra_mobile/core/utils/clock.dart';
import 'package:inventra_mobile/features/account/data/account_repository.dart';
import 'package:inventra_mobile/features/account/data/session_dto.dart';
import 'package:inventra_mobile/features/account/presentation/profile_screen.dart';

import '../helpers/fake_account_repository.dart';
import '../helpers/fake_auth_controller.dart';
import '../helpers/fake_reference_lookup.dart';
import '../helpers/golden_fonts.dart';

/// Waktu beku paritas mockup (09.41).
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

/// Tiga sesi paritas mockup "Profil + 3 sesi perangkat": sesi ini (mobile,
/// aktif sekarang), web Windows · Chrome (2 jam lalu), dan mobile lain di
/// Depok (3 hari lalu).
final List<SessionDto> _goldenSessions = <SessionDto>[
  SessionDto(
    id: 'sess-current',
    browser: 'Inventra App',
    os: 'Android',
    deviceType: 'mobile',
    ipAddress: '103.28.11.4',
    location: 'Jakarta, Indonesia',
    createdAt: DateTime(2026, 7, 1, 8),
    lastSeenAt: _frozenNow,
    current: true,
  ),
  SessionDto(
    id: 'sess-web',
    browser: 'Chrome',
    os: 'Windows',
    deviceType: 'desktop',
    ipAddress: '103.28.11.8',
    location: 'Jakarta, Indonesia',
    createdAt: DateTime(2026, 7, 10, 1),
    lastSeenAt: _frozenNow.subtract(const Duration(hours: 2)),
    current: false,
  ),
  SessionDto(
    id: 'sess-old',
    browser: 'Chrome',
    os: 'Android',
    deviceType: 'mobile',
    ipAddress: '114.10.20.5',
    location: 'Depok, Indonesia',
    createdAt: DateTime(2026, 6, 20, 9),
    lastSeenAt: _frozenNow.subtract(const Duration(days: 3)),
    current: false,
  ),
];

/// Golden layar Profil light + dark (kartu identitas + 3 sesi + tombol
/// keluar-semua + Keluar). Digenerate dan diverifikasi lokal (Windows):
/// `flutter test --update-goldens --tags golden`.
void main() {
  setUpAll(loadAppFonts);

  Widget buildScreen(ThemeData theme) {
    return ProviderScope(
      overrides: [
        authControllerProvider.overrideWith(
          () => FakeAuthController(initialSession: const Authenticated(_user)),
        ),
        accountRepositoryProvider.overrideWithValue(
          FakeAccountRepository(sessions: _goldenSessions),
        ),
        referenceLookupRepositoryProvider.overrideWithValue(
          FakeReferenceLookup(<String, String>{
            'office:office-1': 'Cabang Jakarta Selatan',
          }),
        ),
        clockProvider.overrideWithValue(() => _frozenNow),
      ],
      child: MaterialApp(
        theme: theme,
        locale: const Locale('id'),
        localizationsDelegates: AppLocalizations.localizationsDelegates,
        supportedLocales: const <Locale>[Locale('id'), Locale('en')],
        home: const ProfileScreen(),
      ),
    );
  }

  Future<void> pumpAtPhoneSize(WidgetTester tester, Widget widget) async {
    tester.view.physicalSize = const Size(390, 844);
    tester.view.devicePixelRatio = 1.0;
    addTearDown(tester.view.reset);
    await tester.pumpWidget(widget);
    await tester.pumpAndSettle();
  }

  testWidgets('profil light', (WidgetTester tester) async {
    await pumpAtPhoneSize(tester, buildScreen(InventraTheme.light));
    await expectLater(
      find.byType(ProfileScreen),
      matchesGoldenFile('profile_light.png'),
    );
  });

  testWidgets('profil dark', (WidgetTester tester) async {
    await pumpAtPhoneSize(tester, buildScreen(InventraTheme.dark));
    await expectLater(
      find.byType(ProfileScreen),
      matchesGoldenFile('profile_dark.png'),
    );
  });
}
