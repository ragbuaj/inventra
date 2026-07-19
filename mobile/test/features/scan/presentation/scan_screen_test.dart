import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:go_router/go_router.dart';
import 'package:inventra_mobile/app/theme.dart';
import 'package:inventra_mobile/core/i18n/gen/app_localizations.dart';
import 'package:inventra_mobile/features/scan/presentation/scan_camera.dart';
import 'package:inventra_mobile/features/scan/presentation/scan_screen.dart';

import '../../../helpers/fake_scan_camera.dart';
import '../../../helpers/test_app.dart';

void main() {
  late FakeScanCamera camera;
  late List<String> pushedTags;
  late GoRouter router;

  setUp(() {
    camera = FakeScanCamera();
    pushedTags = <String>[];
  });

  /// Harness router mini: /scan (layar uji), /assets/:tag (probe pencatat
  /// push), / (tujuan tombol tutup). Kamera diganti [FakeScanCamera] — kamera
  /// nyata tidak pernah disentuh tes.
  Widget buildApp() {
    router = GoRouter(
      initialLocation: '/scan',
      routes: <RouteBase>[
        GoRoute(
          path: '/',
          builder: (BuildContext context, GoRouterState state) =>
              const Scaffold(body: Text('home-probe')),
        ),
        GoRoute(
          path: '/scan',
          builder: (BuildContext context, GoRouterState state) =>
              const ScanScreen(),
        ),
        GoRoute(
          path: '/assets/:tag',
          builder: (BuildContext context, GoRouterState state) {
            final String tag = state.pathParameters['tag']!;
            pushedTags.add(tag);
            return Scaffold(body: Text('detail-probe:$tag'));
          },
        ),
      ],
    );
    return ProviderScope(
      overrides: [scanCameraFactoryProvider.overrideWithValue(() => camera)],
      child: MaterialApp.router(
        theme: InventraTheme.light,
        routerConfig: router,
        locale: const Locale('id'),
        localizationsDelegates: AppLocalizations.localizationsDelegates,
        supportedLocales: const <Locale>[Locale('id'), Locale('en')],
      ),
    );
  }

  testWidgets('menampilkan overlay lengkap saat kamera siap', (
    WidgetTester tester,
  ) async {
    await tester.pumpWidget(buildApp());
    await tester.pumpAndSettle();

    expect(find.text(l10nId.scanTitle), findsOneWidget);
    expect(find.text(l10nId.scanHint), findsOneWidget);
    expect(find.text(l10nId.scanManualButton), findsOneWidget);
    expect(find.byTooltip(l10nId.scanCloseTooltip), findsOneWidget);
    expect(find.byTooltip(l10nId.scanTorchOnTooltip), findsOneWidget);
    expect(find.text(l10nId.scanCameraUnavailableTitle), findsNothing);
  });

  testWidgets('deteksi kamera menavigasi ke detail aset', (
    WidgetTester tester,
  ) async {
    await tester.pumpWidget(buildApp());
    await tester.pumpAndSettle();

    camera.detect('JKT01-ELK-2026-00001');
    await tester.pumpAndSettle();

    expect(pushedTags, <String>['JKT01-ELK-2026-00001']);
    expect(find.text('detail-probe:JKT01-ELK-2026-00001'), findsOneWidget);
  });

  testWidgets('deteksi beruntun tag yang sama hanya push sekali (debounce)', (
    WidgetTester tester,
  ) async {
    await tester.pumpWidget(buildApp());
    await tester.pumpAndSettle();

    camera
      ..detect('TAG-1')
      ..detect('TAG-1')
      ..detect('TAG-2');
    await tester.pumpAndSettle();

    expect(pushedTags, <String>['TAG-1']);
  });

  testWidgets(
    'setelah kembali dari detail, deteksi dalam masa jeda diabaikan',
    (WidgetTester tester) async {
      await tester.pumpWidget(buildApp());
      await tester.pumpAndSettle();

      camera.detect('TAG-1');
      await tester.pumpAndSettle();
      expect(pushedTags, <String>['TAG-1']);

      // Kembali ke layar scan; kamera masih mengarah ke label yang sama.
      router.pop();
      await tester.pumpAndSettle();
      camera.detect('TAG-1');
      await tester.pumpAndSettle();

      expect(pushedTags, <String>['TAG-1']);
      expect(find.text(l10nId.scanTitle), findsOneWidget);
    },
  );

  testWidgets('jalur manual: sheet -> isi kode -> Cari -> detail aset', (
    WidgetTester tester,
  ) async {
    await tester.pumpWidget(buildApp());
    await tester.pumpAndSettle();

    await tester.tap(find.text(l10nId.scanManualButton));
    await tester.pumpAndSettle();

    expect(find.text(l10nId.scanManualFieldLabel), findsOneWidget);
    expect(find.text(l10nId.scanManualFieldHelper), findsOneWidget);

    await tester.enterText(find.byType(TextField), 'JKT01-ELK-2026-00002');
    await tester.tap(find.text(l10nId.scanManualSubmit));
    await tester.pumpAndSettle();

    expect(pushedTags, <String>['JKT01-ELK-2026-00002']);
    expect(find.text('detail-probe:JKT01-ELK-2026-00002'), findsOneWidget);
  });

  testWidgets('submit manual kosong tidak menavigasi dan sheet tetap', (
    WidgetTester tester,
  ) async {
    await tester.pumpWidget(buildApp());
    await tester.pumpAndSettle();

    await tester.tap(find.text(l10nId.scanManualButton));
    await tester.pumpAndSettle();
    await tester.tap(find.text(l10nId.scanManualSubmit));
    await tester.pumpAndSettle();

    expect(pushedTags, isEmpty);
    expect(find.text(l10nId.scanManualFieldLabel), findsOneWidget);
  });

  testWidgets('toggle torch memanggil kamera dan menukar tooltip', (
    WidgetTester tester,
  ) async {
    await tester.pumpWidget(buildApp());
    await tester.pumpAndSettle();

    await tester.tap(find.byTooltip(l10nId.scanTorchOnTooltip));
    await tester.pumpAndSettle();

    expect(camera.toggleTorchCalls, 1);
    expect(find.byTooltip(l10nId.scanTorchOffTooltip), findsOneWidget);
  });

  testWidgets('tombol tutup keluar dari layar scan', (
    WidgetTester tester,
  ) async {
    await tester.pumpWidget(buildApp());
    await tester.pumpAndSettle();

    await tester.tap(find.byTooltip(l10nId.scanCloseTooltip));
    await tester.pumpAndSettle();

    expect(find.text('home-probe'), findsOneWidget);
  });

  group('kamera tidak tersedia', () {
    setUp(() {
      camera = FakeScanCamera(unavailable: true);
    });

    testWidgets('menampilkan state jelas tanpa bingkai target', (
      WidgetTester tester,
    ) async {
      await tester.pumpWidget(buildApp());
      await tester.pumpAndSettle();

      expect(find.text(l10nId.scanCameraUnavailableTitle), findsOneWidget);
      expect(find.text(l10nId.scanCameraUnavailableBody), findsOneWidget);
      expect(find.text(l10nId.scanHint), findsNothing);
    });

    testWidgets('jalur manual tetap berfungsi', (WidgetTester tester) async {
      await tester.pumpWidget(buildApp());
      await tester.pumpAndSettle();

      await tester.tap(find.text(l10nId.scanManualButton));
      await tester.pumpAndSettle();
      await tester.enterText(find.byType(TextField), 'TAG-MANUAL');
      await tester.tap(find.text(l10nId.scanManualSubmit));
      await tester.pumpAndSettle();

      expect(pushedTags, <String>['TAG-MANUAL']);
    });

    testWidgets('toggle torch dinonaktifkan', (WidgetTester tester) async {
      await tester.pumpWidget(buildApp());
      await tester.pumpAndSettle();

      await tester.tap(find.byTooltip(l10nId.scanTorchOnTooltip));
      await tester.pumpAndSettle();

      expect(camera.toggleTorchCalls, 0);
    });
  });
}
