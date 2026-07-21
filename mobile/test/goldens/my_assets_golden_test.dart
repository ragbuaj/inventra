@Tags(<String>['golden'])
library;

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:inventra_mobile/app/theme.dart';
import 'package:inventra_mobile/core/i18n/gen/app_localizations.dart';
import 'package:inventra_mobile/core/utils/clock.dart';
import 'package:inventra_mobile/features/my_assets/data/my_assets_repository.dart';
import 'package:inventra_mobile/features/my_assets/presentation/my_assets_screen.dart';
import 'package:mocktail/mocktail.dart';

import '../helpers/golden_fonts.dart';

class _MockMyAssetsRepository extends Mock implements MyAssetsRepository {}

final DateTime _frozenNow = DateTime.utc(2026, 7, 21, 9);

/// Tiga aset dipegang: satu jatuh tempo mendatang, satu terlambat, satu tanpa
/// jatuh tempo.
final List<MyAssignmentDto> _goldenItems = <MyAssignmentDto>[
  const MyAssignmentDto(
    assetName: 'Laptop Dell Latitude 5440',
    assetTag: 'JKT01-ELK-2026-00001',
    status: 'active',
    checkoutDate: '2026-07-01T00:00:00Z',
    dueDate: '2026-08-01',
  ),
  const MyAssignmentDto(
    assetName: 'Proyektor Epson EB-X500',
    assetTag: 'JKT01-ELK-2026-00014',
    status: 'active',
    checkoutDate: '2026-06-20T00:00:00Z',
    dueDate: '2026-07-10',
  ),
  const MyAssignmentDto(
    assetName: 'Headset Logitech H390',
    assetTag: 'JKT01-ELK-2026-00052',
    status: 'active',
    checkoutDate: '2026-07-15T00:00:00Z',
  ),
];

/// Golden Aset Saya light + dark. Digenerate & diverifikasi lokal:
/// `flutter test --update-goldens --tags golden`.
void main() {
  setUpAll(loadAppFonts);

  Widget buildScreen(ThemeData theme) {
    final _MockMyAssetsRepository repository = _MockMyAssetsRepository();
    when(() => repository.list()).thenAnswer((_) async => _goldenItems);

    return ProviderScope(
      overrides: [
        myAssetsRepositoryProvider.overrideWithValue(repository),
        clockProvider.overrideWithValue(() => _frozenNow),
      ],
      child: MaterialApp(
        theme: theme,
        locale: const Locale('id'),
        localizationsDelegates: AppLocalizations.localizationsDelegates,
        supportedLocales: const <Locale>[Locale('id'), Locale('en')],
        home: const MyAssetsScreen(),
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

  testWidgets('aset saya light', (WidgetTester tester) async {
    await pumpAtPhoneSize(tester, buildScreen(InventraTheme.light));
    await expectLater(
      find.byType(MyAssetsScreen),
      matchesGoldenFile('my_assets_light.png'),
    );
  });

  testWidgets('aset saya dark', (WidgetTester tester) async {
    await pumpAtPhoneSize(tester, buildScreen(InventraTheme.dark));
    await expectLater(
      find.byType(MyAssetsScreen),
      matchesGoldenFile('my_assets_dark.png'),
    );
  });
}
