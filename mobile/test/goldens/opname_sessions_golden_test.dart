@Tags(<String>['golden'])
library;

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:inventra_mobile/app/theme.dart';
import 'package:inventra_mobile/core/connectivity/connectivity_provider.dart';
import 'package:inventra_mobile/core/i18n/gen/app_localizations.dart';
import 'package:inventra_mobile/features/stock_opname/data/stock_opname_repository.dart';
import 'package:inventra_mobile/features/stock_opname/data/stock_opname_session_dto.dart';
import 'package:inventra_mobile/features/stock_opname/presentation/opname_session_list_screen.dart';

import '../helpers/fake_stock_opname_repository.dart';
import '../helpers/golden_fonts.dart';

/// Dua kartu variasi mockup "Daftar terisi": sesi berjalan (progress 85% +
/// CTA) dan sesi selesai (100% + chip Berita Acara) — data paritas mockup.
final List<StockOpnameSessionDto> _goldenSessions = <StockOpnameSessionDto>[
  StockOpnameSessionDto(
    id: 'op-1',
    officeId: 'office-1',
    name: 'Opname Tahunan 2026',
    period: DateTime(2026, 7),
    status: 'counting',
    startedById: 'user-1',
    officeName: 'Cabang Jakarta Selatan',
    total: 150,
    found: 120,
    pending: 22,
    variance: 8,
  ),
  StockOpnameSessionDto(
    id: 'op-2',
    officeId: 'office-1',
    name: 'Opname Semester I 2026',
    period: DateTime(2026),
    status: 'closed',
    startedById: 'user-1',
    officeName: 'Cabang Jakarta Selatan',
    total: 150,
    found: 150,
    pending: 0,
    variance: 0,
  ),
];

/// Golden Daftar Sesi Opname light + dark (tab Berjalan menampilkan sesi
/// berjalan; sesi selesai tersaring — paritas perilaku tab mockup; kartu
/// selesai diverifikasi widget test). Digenerate dan diverifikasi lokal
/// (Windows): `flutter test --update-goldens --tags golden`.
void main() {
  setUpAll(loadAppFonts);

  Widget buildScreen(ThemeData theme) {
    return ProviderScope(
      overrides: [
        stockOpnameRepositoryProvider.overrideWithValue(
          FakeStockOpnameRepository(sessionsData: _goldenSessions),
        ),
        isOnlineProvider.overrideWith((Ref ref) => Stream<bool>.value(true)),
      ],
      child: MaterialApp(
        theme: theme,
        locale: const Locale('id'),
        localizationsDelegates: AppLocalizations.localizationsDelegates,
        supportedLocales: const <Locale>[Locale('id'), Locale('en')],
        home: const OpnameSessionListScreen(),
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

  testWidgets('daftar sesi opname light', (WidgetTester tester) async {
    await pumpAtPhoneSize(tester, buildScreen(InventraTheme.light));
    await expectLater(
      find.byType(OpnameSessionListScreen),
      matchesGoldenFile('opname_sessions_light.png'),
    );
  });

  testWidgets('daftar sesi opname dark', (WidgetTester tester) async {
    await pumpAtPhoneSize(tester, buildScreen(InventraTheme.dark));
    await expectLater(
      find.byType(OpnameSessionListScreen),
      matchesGoldenFile('opname_sessions_dark.png'),
    );
  });
}
