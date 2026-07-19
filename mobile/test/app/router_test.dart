import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:go_router/go_router.dart';
import 'package:inventra_mobile/app/router.dart';
import 'package:inventra_mobile/core/auth/auth_controller.dart';
import 'package:inventra_mobile/core/auth/auth_session.dart';
import 'package:inventra_mobile/core/masterdata/reference_lookup_repository.dart';
import 'package:inventra_mobile/features/asset_detail/data/asset_detail_repository.dart';
import 'package:inventra_mobile/features/asset_detail/data/asset_dto.dart';
import 'package:inventra_mobile/features/login/presentation/login_screen.dart';
import 'package:inventra_mobile/features/stock_opname/data/stock_opname_repository.dart';
import 'package:inventra_mobile/features/stock_opname/data/stock_opname_session_dto.dart';

import '../helpers/fake_auth_controller.dart';
import '../helpers/fake_reference_lookup.dart';
import '../helpers/fake_stock_opname_repository.dart';
import '../helpers/test_app.dart';

/// Stub repository detail aset: rute `/assets/:tag` kini layar nyata (Task 8)
/// sehingga tes router harus memutus jalur HTTP-nya.
class _StubAssetDetailRepository implements AssetDetailRepository {
  @override
  Future<AssetDetailData> getByTag(String tag) async => AssetDetailData(
    asset: AssetDto(assetTag: tag, name: 'Aset Uji', status: 'available'),
    maskedFields: const <String>{},
  );
}

void main() {
  late ProviderContainer container;

  ProviderContainer createContainer(FakeAuthController fake) {
    return ProviderContainer.test(
      overrides: [
        authControllerProvider.overrideWith(() => fake),
        assetDetailRepositoryProvider.overrideWithValue(
          _StubAssetDetailRepository(),
        ),
        // Lookup nama referensi diputus dari HTTP nyata (non-fatal, hasil
        // null berarti sel em-dash — cukup untuk asersi rute).
        referenceLookupRepositoryProvider.overrideWithValue(
          FakeReferenceLookup(),
        ),
        // Rute /stock-opname/* kini layar nyata (Task 10) — jalur HTTP-nya
        // diputus dengan repository palsu berisi satu sesi.
        stockOpnameRepositoryProvider.overrideWithValue(
          FakeStockOpnameRepository(
            sessionsData: <StockOpnameSessionDto>[
              const StockOpnameSessionDto(
                id: 'op-1',
                officeId: 'office-1',
                name: 'Opname Uji 2026',
                status: 'counting',
                startedById: 'user-1',
                officeName: 'Cabang Jakarta Selatan',
                total: 10,
                found: 4,
                pending: 6,
                variance: 0,
              ),
            ],
          ),
        ),
      ],
    );
  }

  group('guard auth', () {
    testWidgets('belum login diarahkan ke /login', (WidgetTester tester) async {
      container = createContainer(FakeAuthController());
      await tester.pumpWidget(RouterTestApp(container: container));
      await tester.pumpAndSettle();

      expect(find.byType(LoginScreen), findsOneWidget);
      expect(find.text(l10nId.loginCardSubtitle), findsOneWidget);
      // Shell bottom-nav tidak ada di layar login.
      expect(find.text(l10nId.shellTabScan), findsNothing);
    });

    testWidgets('sudah login mendarat di beranda dalam shell', (
      WidgetTester tester,
    ) async {
      container = createContainer(
        FakeAuthController(initialSession: const Authenticated(fakeUser)),
      );
      await tester.pumpWidget(RouterTestApp(container: container));
      await tester.pumpAndSettle();

      expect(find.byType(LoginScreen), findsNothing);
      // App bar beranda + label tab.
      expect(find.text(l10nId.homeTitle), findsNWidgets(2));
      expect(find.text(l10nId.shellTabScan), findsOneWidget);
    });

    testWidgets('sudah login mengakses /login dialihkan ke beranda', (
      WidgetTester tester,
    ) async {
      container = createContainer(
        FakeAuthController(initialSession: const Authenticated(fakeUser)),
      );
      await tester.pumpWidget(RouterTestApp(container: container));
      await tester.pumpAndSettle();

      container.read(appRouterProvider).go('/login');
      await tester.pumpAndSettle();

      expect(find.byType(LoginScreen), findsNothing);
      expect(find.text(l10nId.homeTitle), findsNWidgets(2));
    });

    testWidgets('belum login mengakses rute dalam dibelokkan ke /login', (
      WidgetTester tester,
    ) async {
      container = createContainer(FakeAuthController());
      await tester.pumpWidget(RouterTestApp(container: container));
      await tester.pumpAndSettle();

      container.read(appRouterProvider).go('/assets/JKT01-ELK-2026-00001');
      await tester.pumpAndSettle();

      expect(find.byType(LoginScreen), findsOneWidget);
      expect(find.text(l10nId.assetDetailTitle), findsNothing);
    });

    testWidgets('logout dari beranda kembali ke /login', (
      WidgetTester tester,
    ) async {
      final FakeAuthController fake = FakeAuthController(
        initialSession: const Authenticated(fakeUser),
      );
      container = createContainer(fake);
      await tester.pumpWidget(RouterTestApp(container: container));
      await tester.pumpAndSettle();

      await tester.tap(find.byTooltip(l10nId.homeLogoutTooltip));
      await tester.pumpAndSettle();
      expect(find.text(l10nId.homeLogoutConfirmTitle), findsOneWidget);

      await tester.tap(find.text(l10nId.homeLogoutConfirmAction));
      await tester.pumpAndSettle();

      expect(fake.logoutCalls, 1);
      expect(find.byType(LoginScreen), findsOneWidget);
    });

    testWidgets('batal pada dialog logout mempertahankan sesi', (
      WidgetTester tester,
    ) async {
      final FakeAuthController fake = FakeAuthController(
        initialSession: const Authenticated(fakeUser),
      );
      container = createContainer(fake);
      await tester.pumpWidget(RouterTestApp(container: container));
      await tester.pumpAndSettle();

      await tester.tap(find.byTooltip(l10nId.homeLogoutTooltip));
      await tester.pumpAndSettle();
      await tester.tap(find.text(l10nId.commonCancel));
      await tester.pumpAndSettle();

      expect(fake.logoutCalls, 0);
      expect(find.byType(LoginScreen), findsNothing);
      expect(find.text(l10nId.homeTitle), findsNWidgets(2));
    });
  });

  group('layar sekunder di atas shell', () {
    testWidgets('detail aset tampil tanpa bottom nav', (
      WidgetTester tester,
    ) async {
      container = createContainer(
        FakeAuthController(initialSession: const Authenticated(fakeUser)),
      );
      await tester.pumpWidget(RouterTestApp(container: container));
      await tester.pumpAndSettle();

      container.read(appRouterProvider).go('/assets/JKT01-ELK-2026-00001');
      await tester.pumpAndSettle();

      expect(find.text(l10nId.assetDetailTitle), findsOneWidget);
      expect(find.text('Aset Uji'), findsOneWidget);
      expect(find.text('JKT01-ELK-2026-00001'), findsOneWidget);
      expect(find.text(l10nId.shellTabScan), findsNothing);
    });

    testWidgets('detail dan variance opname tampil tanpa bottom nav', (
      WidgetTester tester,
    ) async {
      container = createContainer(
        FakeAuthController(initialSession: const Authenticated(fakeUser)),
      );
      await tester.pumpWidget(RouterTestApp(container: container));
      await tester.pumpAndSettle();

      final GoRouter router = container.read(appRouterProvider);
      router.go('/stock-opname/op-1');
      await tester.pumpAndSettle();
      expect(find.text('Opname Uji 2026'), findsOneWidget);
      expect(find.text(l10nId.opnameCountingScanButton), findsOneWidget);
      expect(find.text(l10nId.shellTabScan), findsNothing);

      router.go('/stock-opname/op-1/variance');
      await tester.pumpAndSettle();
      expect(find.text(l10nId.opnameVarianceTabVariance), findsOneWidget);
      expect(find.text(l10nId.opnameVarianceEmptyTitle), findsOneWidget);
      expect(find.text(l10nId.shellTabScan), findsNothing);
    });

    testWidgets('profil dan pengaturan tampil tanpa bottom nav', (
      WidgetTester tester,
    ) async {
      container = createContainer(
        FakeAuthController(initialSession: const Authenticated(fakeUser)),
      );
      await tester.pumpWidget(RouterTestApp(container: container));
      await tester.pumpAndSettle();

      final GoRouter router = container.read(appRouterProvider);
      router.go('/account');
      await tester.pumpAndSettle();
      expect(find.text(l10nId.accountTitle), findsOneWidget);
      expect(find.text(l10nId.shellTabScan), findsNothing);

      router.go('/settings');
      await tester.pumpAndSettle();
      expect(find.text(l10nId.settingsTitle), findsOneWidget);
      expect(find.text(l10nId.shellTabScan), findsNothing);
    });
  });
}
