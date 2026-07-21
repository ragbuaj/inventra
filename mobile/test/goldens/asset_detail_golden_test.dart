@Tags(<String>['golden'])
library;

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:inventra_mobile/app/theme.dart';
import 'package:inventra_mobile/core/authz/permissions_provider.dart';
import 'package:inventra_mobile/core/i18n/gen/app_localizations.dart';
import 'package:inventra_mobile/features/asset_detail/data/asset_detail_repository.dart';
import 'package:inventra_mobile/features/asset_detail/data/asset_dto.dart';
import 'package:inventra_mobile/core/masterdata/reference_lookup_repository.dart';
import 'package:inventra_mobile/features/asset_detail/presentation/asset_detail_screen.dart';
import 'package:mocktail/mocktail.dart';

import '../helpers/fake_reference_lookup.dart';
import '../helpers/golden_fonts.dart';

class _MockAssetDetailRepository extends Mock
    implements AssetDetailRepository {}

const String _tag = 'JKT01-ELK-2026-00001';

/// Data variasi mockup "Field sensitif dibatasi": detail lengkap dengan nilai
/// finansial dimask field permission (dua penanda gembok di seksi Nilai).
const AssetDetailData _goldenData = AssetDetailData(
  asset: AssetDto(
    id: 'a-1',
    assetTag: _tag,
    name: 'Laptop Dell Latitude 5440',
    categoryId: 'cat-elektronik',
    officeId: 'office-jaksel',
    brandId: 'brand-dell',
    modelId: 'model-latitude-5440',
    roomId: 'room-operasional',
    vendorId: 'vendor-mitra',
    currentHolderEmployeeId: 'emp-rina',
    status: 'available',
    assetClass: 'tangible',
    serialNumber: 'SN-11223344',
    purchaseDate: '2026-02-12',
  ),
  maskedFields: <String>{
    'purchase_cost',
    'book_value',
    'accumulated_depreciation',
  },
);

/// Nama referensi ter-resolve (nilai mockup) untuk id di [_goldenData].
const Map<String, String> _goldenNames = <String, String>{
  'office:office-jaksel': 'Cabang Jakarta Selatan',
  'room:room-operasional': 'Lantai 2 · Ruang Operasional',
  'employee:emp-rina': 'Rina Kusuma',
  'category:cat-elektronik': 'Elektronik — Laptop',
  'brand:brand-dell': 'Dell',
  'model:model-latitude-5440': 'Latitude 5440',
  'vendor:vendor-mitra': 'PT Mitra Teknologi Nusantara',
};

/// Golden Detail Aset light + dark (variasi field dibatasi). Digenerate dan
/// diverifikasi lokal (Windows): `flutter test --update-goldens --tags golden`.
void main() {
  setUpAll(loadAppFonts);

  Widget buildDetail(ThemeData theme) {
    final _MockAssetDetailRepository repository = _MockAssetDetailRepository();
    when(() => repository.getByTag(_tag)).thenAnswer((_) async => _goldenData);

    return ProviderScope(
      overrides: [
        assetDetailRepositoryProvider.overrideWithValue(repository),
        referenceLookupRepositoryProvider.overrideWithValue(
          FakeReferenceLookup(_goldenNames),
        ),
        // FR-M7: bar aksi Detail Aset. request.create -> tombol Pinjam bila aset
        // available (deterministik; tanpa HTTP nyata).
        permissionsProvider.overrideWith((ref) async => const <String>{
          'request.create',
        }),
      ],
      child: MaterialApp(
        theme: theme,
        locale: const Locale('id'),
        localizationsDelegates: AppLocalizations.localizationsDelegates,
        supportedLocales: const <Locale>[Locale('id'), Locale('en')],
        home: const AssetDetailScreen(tag: _tag),
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

  testWidgets('detail aset field dibatasi light', (WidgetTester tester) async {
    await pumpAtPhoneSize(tester, buildDetail(InventraTheme.light));
    await expectLater(
      find.byType(AssetDetailScreen),
      matchesGoldenFile('asset_detail_light.png'),
    );
  });

  testWidgets('detail aset field dibatasi dark', (WidgetTester tester) async {
    await pumpAtPhoneSize(tester, buildDetail(InventraTheme.dark));
    await expectLater(
      find.byType(AssetDetailScreen),
      matchesGoldenFile('asset_detail_dark.png'),
    );
  });
}
