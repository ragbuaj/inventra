@Tags(<String>['golden'])
library;

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:inventra_mobile/app/theme.dart';
import 'package:inventra_mobile/core/connectivity/connectivity_provider.dart';
import 'package:inventra_mobile/core/i18n/gen/app_localizations.dart';
import 'package:inventra_mobile/features/stock_opname/data/stock_opname_item_dto.dart';
import 'package:inventra_mobile/features/stock_opname/data/stock_opname_repository.dart';
import 'package:inventra_mobile/features/stock_opname/data/stock_opname_session_dto.dart';
import 'package:inventra_mobile/features/stock_opname/presentation/opname_variance_screen.dart';

import '../helpers/fake_stock_opname_repository.dart';
import '../helpers/golden_fonts.dart';

const StockOpnameSessionDto _goldenSession = StockOpnameSessionDto(
  id: 'op-1',
  officeId: 'office-1',
  name: 'Opname Tahunan 2026',
  status: 'reconciling',
  startedById: 'user-1',
  officeName: 'Cabang Jakarta Selatan',
  total: 150,
  found: 141,
  pending: 0,
  variance: 9,
);

/// Item variance paritas mockup "Variance terisi campuran": tidak ditemukan
/// (catatan + belum ditindaklanjuti; sudah diajukan penghapusan), salah
/// lokasi, rusak, dan temuan di luar catatan.
const List<StockOpnameItemDto> _goldenItems = <StockOpnameItemDto>[
  StockOpnameItemDto(
    id: 'item-1',
    sessionId: 'op-1',
    assetId: 'asset-1',
    assetName: 'Kamera Sony A6400',
    assetTag: 'JKT01-ELK-2025-00061',
    roomName: 'R. Marketing',
    floorName: 'Lantai 2',
    expected: true,
    result: 'not_found',
    note: 'Sudah dicari di seluruh lantai 2, tidak ada.',
  ),
  StockOpnameItemDto(
    id: 'item-2',
    sessionId: 'op-1',
    assetId: 'asset-2',
    assetName: 'Kursi Rapat Chitose',
    assetTag: 'JKT01-FUR-2024-00088',
    roomName: 'R. Rapat',
    floorName: 'Lantai 3',
    expected: true,
    result: 'not_found',
    followupRequestId: 'req-1',
  ),
  StockOpnameItemDto(
    id: 'item-3',
    sessionId: 'op-1',
    assetId: 'asset-3',
    assetName: 'Printer HP LaserJet M404',
    assetTag: 'JKT01-ELK-2025-00087',
    roomName: 'R. Operasional',
    floorName: 'Lantai 2',
    expected: true,
    result: 'misplaced',
  ),
  StockOpnameItemDto(
    id: 'item-4',
    sessionId: 'op-1',
    assetId: 'asset-4',
    assetName: 'Genset Perkins 100 kVA',
    assetTag: 'JKT01-MSN-2022-00007',
    expected: true,
    result: 'damaged',
    followupRecordId: 'rec-1',
  ),
  StockOpnameItemDto(
    id: 'item-5',
    sessionId: 'op-1',
    assetId: 'asset-5',
    assetName: 'Proyektor Epson EB-X500',
    assetTag: 'JKT01-ELK-2023-00031',
    expected: false,
    result: 'found',
  ),
];

/// Golden Variance Opname light + dark (state terisi campuran). Digenerate
/// dan diverifikasi lokal (Windows):
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
      ],
      child: MaterialApp(
        theme: theme,
        locale: const Locale('id'),
        localizationsDelegates: AppLocalizations.localizationsDelegates,
        supportedLocales: const <Locale>[Locale('id'), Locale('en')],
        home: const OpnameVarianceScreen(sessionId: 'op-1'),
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

  testWidgets('variance opname light', (WidgetTester tester) async {
    await pumpAtPhoneSize(tester, buildScreen(InventraTheme.light));
    await expectLater(
      find.byType(OpnameVarianceScreen),
      matchesGoldenFile('opname_variance_light.png'),
    );
  });

  testWidgets('variance opname dark', (WidgetTester tester) async {
    await pumpAtPhoneSize(tester, buildScreen(InventraTheme.dark));
    await expectLater(
      find.byType(OpnameVarianceScreen),
      matchesGoldenFile('opname_variance_dark.png'),
    );
  });
}
