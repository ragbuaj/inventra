@Tags(<String>['golden'])
library;

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:inventra_mobile/app/theme.dart';
import 'package:inventra_mobile/core/i18n/gen/app_localizations.dart';
import 'package:inventra_mobile/core/masterdata/reference_lookup_repository.dart';
import 'package:inventra_mobile/features/asset_detail/data/asset_dto.dart';
import 'package:inventra_mobile/features/catalog/data/asset_list_dto.dart';
import 'package:inventra_mobile/features/catalog/data/catalog_repository.dart';
import 'package:inventra_mobile/features/catalog/presentation/catalog_screen.dart';
import 'package:mocktail/mocktail.dart';

import '../helpers/fake_reference_lookup.dart';
import '../helpers/golden_fonts.dart';

class _MockCatalogRepository extends Mock implements CatalogRepository {}

/// Empat aset variasi status (tersedia/dipinjam/maintenance/dilepas) untuk
/// merender kartu katalog + chip status.
final List<AssetDto> _goldenItems = <AssetDto>[
  const AssetDto(
    id: 'a1',
    assetTag: 'JKT01-ELK-2026-00001',
    name: 'Laptop Dell Latitude 5440',
    status: 'available',
    officeId: 'off-1',
  ),
  const AssetDto(
    id: 'a2',
    assetTag: 'JKT01-ELK-2026-00014',
    name: 'Proyektor Epson EB-X500',
    status: 'assigned',
    officeId: 'off-1',
  ),
  const AssetDto(
    id: 'a3',
    assetTag: 'JKT01-ATK-2024-00031',
    name: 'AC Split Daikin 1 PK Ruang Server',
    status: 'under_maintenance',
    officeId: 'off-2',
  ),
  const AssetDto(
    id: 'a4',
    assetTag: 'JKT01-FRN-2020-00088',
    name: 'Meja Kerja Kayu 120x60',
    status: 'disposed',
    officeId: 'off-2',
  ),
];

/// Golden Katalog Aset light + dark (daftar terisi + baris filter). Digenerate
/// dan diverifikasi lokal: `flutter test --update-goldens --tags golden`.
void main() {
  setUpAll(loadAppFonts);

  Widget buildCatalog(ThemeData theme) {
    final _MockCatalogRepository repository = _MockCatalogRepository();
    when(
      () => repository.list(
        search: null,
        categoryId: null,
        status: null,
        officeId: null,
        offset: 0,
        limit: any(named: 'limit'),
      ),
    ).thenAnswer(
      (_) async =>
          AssetListDto(data: _goldenItems, total: 4, limit: 20, offset: 0),
    );

    return ProviderScope(
      overrides: [
        catalogRepositoryProvider.overrideWithValue(repository),
        referenceLookupRepositoryProvider.overrideWithValue(
          FakeReferenceLookup(<String, String>{
            'office:off-1': 'Cabang Jakarta Selatan',
            'office:off-2': 'KCP Kebayoran Baru',
          }),
        ),
      ],
      child: MaterialApp(
        theme: theme,
        locale: const Locale('id'),
        localizationsDelegates: AppLocalizations.localizationsDelegates,
        supportedLocales: const <Locale>[Locale('id'), Locale('en')],
        home: const CatalogScreen(),
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

  testWidgets('katalog aset light', (WidgetTester tester) async {
    await pumpAtPhoneSize(tester, buildCatalog(InventraTheme.light));
    await expectLater(
      find.byType(CatalogScreen),
      matchesGoldenFile('catalog_light.png'),
    );
  });

  testWidgets('katalog aset dark', (WidgetTester tester) async {
    await pumpAtPhoneSize(tester, buildCatalog(InventraTheme.dark));
    await expectLater(
      find.byType(CatalogScreen),
      matchesGoldenFile('catalog_dark.png'),
    );
  });
}
