import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:go_router/go_router.dart';
import 'package:inventra_mobile/app/theme.dart';
import 'package:inventra_mobile/core/i18n/gen/app_localizations.dart';
import 'package:inventra_mobile/features/asset_register/data/asset_register_repository.dart';
import 'package:inventra_mobile/features/asset_register/presentation/asset_register_screen.dart';
import 'package:inventra_mobile/features/catalog/data/filter_options_repository.dart';
import 'package:mocktail/mocktail.dart';

import '../../../helpers/test_app.dart';

class _MockAssetRegisterRepository extends Mock
    implements AssetRegisterRepository {}

void main() {
  late _MockAssetRegisterRepository repository;

  setUp(() {
    repository = _MockAssetRegisterRepository();
  });

  Future<void> pump(WidgetTester tester) async {
    tester.view.physicalSize = const Size(600, 2000);
    tester.view.devicePixelRatio = 1.0;
    addTearDown(tester.view.reset);

    final GoRouter router = GoRouter(
      initialLocation: '/register-asset',
      routes: <RouteBase>[
        GoRoute(
          path: '/register-asset',
          builder: (BuildContext context, GoRouterState state) =>
              const AssetRegisterScreen(),
        ),
        GoRoute(
          path: '/my-requests',
          builder: (BuildContext context, GoRouterState state) =>
              const Scaffold(body: Text('MY REQUESTS')),
        ),
      ],
    );

    final ProviderContainer container = ProviderContainer.test(
      overrides: [
        assetRegisterRepositoryProvider.overrideWithValue(repository),
        catalogCategoryOptionsProvider.overrideWith(
          (ref) async =>
              <FilterOption>[const FilterOption('cat-1', 'Elektronik')],
        ),
        catalogOfficeOptionsProvider.overrideWith(
          (ref) async => <FilterOption>[
            const FilterOption('off-1', 'Cabang Jakarta Selatan'),
          ],
        ),
      ],
    );

    await tester.pumpWidget(
      UncontrolledProviderScope(
        container: container,
        child: MaterialApp.router(
          routerConfig: router,
          theme: InventraTheme.light,
          locale: const Locale('id'),
          localizationsDelegates: AppLocalizations.localizationsDelegates,
          supportedLocales: const <Locale>[Locale('id'), Locale('en')],
        ),
      ),
    );
    await tester.pumpAndSettle();
  }

  testWidgets('alur lengkap: isi -> submit -> register + ke Pengajuan Saya', (
    WidgetTester tester,
  ) async {
    when(
      () => repository.register(
        name: any(named: 'name'),
        categoryId: any(named: 'categoryId'),
        officeId: any(named: 'officeId'),
        assetClass: any(named: 'assetClass'),
        purchaseCost: any(named: 'purchaseCost'),
        purchaseDate: any(named: 'purchaseDate'),
        serialNumber: any(named: 'serialNumber'),
        notes: any(named: 'notes'),
      ),
    ).thenAnswer((_) async {});

    await pump(tester);

    // Langkah 1: nama + kategori.
    await tester.enterText(find.byType(TextField).first, 'Laptop Dell');
    await tester.tap(find.byType(DropdownButtonFormField<String>));
    await tester.pumpAndSettle();
    await tester.tap(find.text('Elektronik').last);
    await tester.pumpAndSettle();
    await tester.tap(find.text(l10nId.registerNext));
    await tester.pumpAndSettle();

    // Langkah 2: kantor.
    await tester.tap(find.byType(DropdownButtonFormField<String>));
    await tester.pumpAndSettle();
    await tester.tap(find.text('Cabang Jakarta Selatan').last);
    await tester.pumpAndSettle();
    await tester.tap(find.text(l10nId.registerNext));
    await tester.pumpAndSettle();

    // Langkah 3: kirim.
    await tester.tap(find.text(l10nId.registerSubmit));
    await tester.pumpAndSettle();

    verify(
      () => repository.register(
        name: 'Laptop Dell',
        categoryId: 'cat-1',
        officeId: 'off-1',
        assetClass: 'tangible',
        purchaseCost: any(named: 'purchaseCost'),
        purchaseDate: any(named: 'purchaseDate'),
        serialNumber: any(named: 'serialNumber'),
        notes: any(named: 'notes'),
      ),
    ).called(1);
    expect(find.text('MY REQUESTS'), findsOneWidget);
  });

  testWidgets('intangible + harga konkret: nilai persis diteruskan register', (
    WidgetTester tester,
  ) async {
    when(
      () => repository.register(
        name: any(named: 'name'),
        categoryId: any(named: 'categoryId'),
        officeId: any(named: 'officeId'),
        assetClass: any(named: 'assetClass'),
        purchaseCost: any(named: 'purchaseCost'),
        purchaseDate: any(named: 'purchaseDate'),
        serialNumber: any(named: 'serialNumber'),
        notes: any(named: 'notes'),
      ),
    ).thenAnswer((_) async {});

    await pump(tester);

    // Langkah 1: nama + kategori + pilih kelas Tak Berwujud (intangible).
    await tester.enterText(find.byType(TextField).first, 'Lisensi ERP');
    await tester.tap(find.byType(DropdownButtonFormField<String>));
    await tester.pumpAndSettle();
    await tester.tap(find.text('Elektronik').last);
    await tester.pumpAndSettle();
    await tester.tap(find.text(l10nId.registerClassIntangible));
    await tester.pumpAndSettle();
    await tester.tap(find.text(l10nId.registerNext));
    await tester.pumpAndSettle();

    // Langkah 2: kantor + harga perolehan konkret (field pertama).
    await tester.tap(find.byType(DropdownButtonFormField<String>));
    await tester.pumpAndSettle();
    await tester.tap(find.text('Cabang Jakarta Selatan').last);
    await tester.pumpAndSettle();
    await tester.enterText(find.byType(TextField).first, '2500000');
    await tester.pumpAndSettle();
    await tester.tap(find.text(l10nId.registerNext));
    await tester.pumpAndSettle();

    // Langkah 3: kirim.
    await tester.tap(find.text(l10nId.registerSubmit));
    await tester.pumpAndSettle();

    verify(
      () => repository.register(
        name: 'Lisensi ERP',
        categoryId: 'cat-1',
        officeId: 'off-1',
        assetClass: 'intangible',
        purchaseCost: '2500000',
        purchaseDate: any(named: 'purchaseDate'),
        serialNumber: any(named: 'serialNumber'),
        notes: any(named: 'notes'),
      ),
    ).called(1);
  });

  testWidgets('langkah 1 tanpa nama: validasi menahan lanjut', (
    WidgetTester tester,
  ) async {
    await pump(tester);
    await tester.tap(find.text(l10nId.registerNext));
    await tester.pumpAndSettle();

    expect(find.text(l10nId.registerNameRequired), findsOneWidget);
  });

  testWidgets('harga perolehan menolak keystroke non-numerik', (
    WidgetTester tester,
  ) async {
    await pump(tester);

    // Ke langkah 2 (isi nama + kategori dulu).
    await tester.enterText(find.byType(TextField).first, 'Laptop');
    await tester.tap(find.byType(DropdownButtonFormField<String>));
    await tester.pumpAndSettle();
    await tester.tap(find.text('Elektronik').last);
    await tester.pumpAndSettle();
    await tester.tap(find.text(l10nId.registerNext));
    await tester.pumpAndSettle();

    // Field harga = TextField pertama di langkah 2; ketik campuran huruf+angka
    // -> hanya angka tersisa (inputFormatters menolak keystroke non-numerik).
    await tester.enterText(find.byType(TextField).first, '15a00b0');
    await tester.pump();
    final TextField costField = tester.widget<TextField>(
      find.byType(TextField).first,
    );
    expect(costField.controller?.text, '15000');
  });

  testWidgets('harga perolehan menolak titik ribuan (digit-only)', (
    WidgetTester tester,
  ) async {
    await pump(tester);
    await tester.enterText(find.byType(TextField).first, 'Laptop');
    await tester.tap(find.byType(DropdownButtonFormField<String>));
    await tester.pumpAndSettle();
    await tester.tap(find.text('Elektronik').last);
    await tester.pumpAndSettle();
    await tester.tap(find.text(l10nId.registerNext));
    await tester.pumpAndSettle();

    // "1.000.000" (pemisah ribuan) sebelumnya lolos dan membuat purchase_cost
    // malformed; kini titik ditolak -> "1000000" (rupiah bulat, parseable).
    await tester.enterText(find.byType(TextField).first, '1.000.000');
    await tester.pump();
    final TextField costField = tester.widget<TextField>(
      find.byType(TextField).first,
    );
    expect(costField.controller?.text, '1000000');
  });
}
