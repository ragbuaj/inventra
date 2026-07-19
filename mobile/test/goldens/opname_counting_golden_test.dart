@Tags(<String>['golden'])
library;

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:inventra_mobile/app/theme.dart';
import 'package:inventra_mobile/core/camera/scan_camera.dart';
import 'package:inventra_mobile/core/connectivity/connectivity_provider.dart';
import 'package:inventra_mobile/core/i18n/gen/app_localizations.dart';
import 'package:inventra_mobile/features/stock_opname/data/stock_opname_item_dto.dart';
import 'package:inventra_mobile/features/stock_opname/data/stock_opname_repository.dart';
import 'package:inventra_mobile/features/stock_opname/data/stock_opname_session_dto.dart';
import 'package:inventra_mobile/features/stock_opname/presentation/opname_counting_screen.dart';

import '../helpers/fake_scan_camera.dart';
import '../helpers/fake_stock_opname_repository.dart';
import '../helpers/golden_fonts.dart';

const StockOpnameSessionDto _goldenSession = StockOpnameSessionDto(
  id: 'op-1',
  officeId: 'office-1',
  name: 'Opname Tahunan 2026',
  status: 'counting',
  startedById: 'user-1',
  officeName: 'Cabang Jakarta Selatan',
  total: 150,
  found: 120,
  pending: 22,
  variance: 8,
);

/// Baris "Baru saja dipindai" paritas mockup counting online normal
/// (Ditemukan/Rusak/Salah lokasi/Ditemukan). countedAt waktu LOKAL supaya
/// jam yang dirender deterministik pada mesin verifikasi golden.
final List<StockOpnameItemDto> _goldenItems = <StockOpnameItemDto>[
  StockOpnameItemDto(
    id: 'item-1',
    sessionId: 'op-1',
    assetId: 'asset-1',
    assetName: 'Monitor Dell U2723',
    assetTag: 'JKT01-ELK-2026-00014',
    expected: true,
    result: 'found',
    countedAt: DateTime(2026, 7, 19, 9, 40),
  ),
  StockOpnameItemDto(
    id: 'item-2',
    sessionId: 'op-1',
    assetId: 'asset-2',
    assetName: 'Kursi Ergonomis Fantoni',
    assetTag: 'JKT01-FUR-2025-00112',
    expected: true,
    result: 'damaged',
    countedAt: DateTime(2026, 7, 19, 9, 38),
  ),
  StockOpnameItemDto(
    id: 'item-3',
    sessionId: 'op-1',
    assetId: 'asset-3',
    assetName: 'Printer HP LaserJet M404',
    assetTag: 'JKT01-ELK-2025-00087',
    expected: true,
    result: 'misplaced',
    countedAt: DateTime(2026, 7, 19, 9, 36),
  ),
  StockOpnameItemDto(
    id: 'item-4',
    sessionId: 'op-1',
    assetId: 'asset-4',
    assetName: 'UPS APC Smart 1500VA',
    assetTag: 'JKT01-ELK-2024-00042',
    expected: true,
    result: 'found',
    countedAt: DateTime(2026, 7, 19, 9, 35),
  ),
];

/// Golden Opname Counting light + dark (state online normal). Kamera memakai
/// stub deterministik (unavailable) — halaman kamera tidak dirender di golden,
/// tetapi factory tetap dioverride supaya tidak ada jalur plugin yang
/// tersentuh. Digenerate dan diverifikasi lokal (Windows):
/// `flutter test --update-goldens --tags golden`.
void main() {
  setUpAll(loadAppFonts);

  Widget buildScreen(ThemeData theme) {
    return ProviderScope(
      overrides: [
        stockOpnameRepositoryProvider.overrideWithValue(
          FakeStockOpnameRepository(
            sessionsData: const <StockOpnameSessionDto>[_goldenSession],
            itemsData: _goldenItems,
          ),
        ),
        isOnlineProvider.overrideWith((Ref ref) => Stream<bool>.value(true)),
        scanCameraFactoryProvider.overrideWithValue(
          () => FakeScanCamera(unavailable: true),
        ),
      ],
      child: MaterialApp(
        theme: theme,
        locale: const Locale('id'),
        localizationsDelegates: AppLocalizations.localizationsDelegates,
        supportedLocales: const <Locale>[Locale('id'), Locale('en')],
        home: const OpnameCountingScreen(sessionId: 'op-1'),
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

  testWidgets('opname counting light', (WidgetTester tester) async {
    await pumpAtPhoneSize(tester, buildScreen(InventraTheme.light));
    await expectLater(
      find.byType(OpnameCountingScreen),
      matchesGoldenFile('opname_counting_light.png'),
    );
  });

  testWidgets('opname counting dark', (WidgetTester tester) async {
    await pumpAtPhoneSize(tester, buildScreen(InventraTheme.dark));
    await expectLater(
      find.byType(OpnameCountingScreen),
      matchesGoldenFile('opname_counting_dark.png'),
    );
  });
}
