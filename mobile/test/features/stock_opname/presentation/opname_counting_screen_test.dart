import 'dart:async';

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:inventra_mobile/core/camera/scan_camera.dart';
import 'package:inventra_mobile/core/connectivity/connectivity_provider.dart';
import 'package:inventra_mobile/core/widgets/app_skeleton.dart';
import 'package:inventra_mobile/core/widgets/offline_banner.dart';
import 'package:inventra_mobile/core/widgets/sync_pill.dart';
import 'package:inventra_mobile/features/stock_opname/data/stock_opname_item_dto.dart';
import 'package:inventra_mobile/features/stock_opname/data/stock_opname_repository.dart';
import 'package:inventra_mobile/features/stock_opname/data/stock_opname_session_dto.dart';
import 'package:inventra_mobile/features/stock_opname/presentation/opname_counting_screen.dart';

import '../../../helpers/fake_scan_camera.dart';
import '../../../helpers/fake_stock_opname_repository.dart';
import '../../../helpers/test_app.dart';

const StockOpnameSessionDto _session = StockOpnameSessionDto(
  id: 'op-1',
  officeId: 'office-1',
  name: 'Opname Tahunan 2026',
  status: 'counting',
  startedById: 'user-1',
  officeName: 'Cabang Jakarta Selatan',
  total: 5,
  found: 1,
  pending: 2,
  variance: 2,
);

/// countedAt memakai waktu LOKAL supaya render jam deterministik lintas zona.
final List<StockOpnameItemDto> _items = <StockOpnameItemDto>[
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
  const StockOpnameItemDto(
    id: 'item-4',
    sessionId: 'op-1',
    assetId: 'asset-4',
    assetName: 'UPS APC Smart 1500VA',
    assetTag: 'JKT01-ELK-2024-00042',
    expected: true,
    result: 'pending',
  ),
  const StockOpnameItemDto(
    id: 'item-5',
    sessionId: 'op-1',
    assetId: 'asset-5',
    assetName: 'Rak Arsip Besi 4 Tingkat',
    assetTag: 'JKT01-FUR-2023-00219',
    expected: true,
    result: 'pending',
  ),
];

/// Temuan di luar snapshot yang muncul saat tag asing dipindai.
const StockOpnameItemDto _unexpectedItem = StockOpnameItemDto(
  id: 'item-9',
  sessionId: 'op-1',
  assetId: 'asset-9',
  assetName: 'Kamera Sony A6400',
  assetTag: 'JKT01-ELK-2026-00099',
  expected: false,
  result: 'pending',
);

/// Repository yang tidak pernah selesai — untuk state loading.
class _NeverRepository extends FakeStockOpnameRepository {
  @override
  Future<StockOpnameSessionDto> session(String id) =>
      Completer<StockOpnameSessionDto>().future;
}

void main() {
  late FakeScanCamera camera;

  Widget buildScreen({
    required FakeStockOpnameRepository repository,
    bool online = true,
    String sessionId = 'op-1',
  }) {
    final ProviderContainer container = ProviderContainer.test(
      overrides: [
        stockOpnameRepositoryProvider.overrideWithValue(repository),
        isOnlineProvider.overrideWith((Ref ref) => Stream<bool>.value(online)),
        scanCameraFactoryProvider.overrideWithValue(() => camera),
      ],
    );
    return buildScreenHarness(
      container: container,
      child: OpnameCountingScreen(sessionId: sessionId),
    );
  }

  FakeStockOpnameRepository fullRepository() => FakeStockOpnameRepository(
    sessionsData: <StockOpnameSessionDto>[_session],
    itemsData: _items,
  );

  setUp(() {
    camera = FakeScanCamera();
  });

  group('state data', () {
    testWidgets('header, progress ring, rekap, dan daftar terbaru', (
      WidgetTester tester,
    ) async {
      await tester.pumpWidget(buildScreen(repository: fullRepository()));
      await tester.pumpAndSettle();

      // Header sesi.
      expect(find.text('Opname Tahunan 2026'), findsOneWidget);
      // Ring: counted = total - pending = 3 dari 5.
      expect(find.text('3'), findsOneWidget);
      expect(find.text(l10nId.opnameCountingRingTotal(5)), findsOneWidget);
      // Pill sync online-only: tersinkron.
      expect(find.text(l10nId.commonSyncSynced), findsOneWidget);
      // Tombol scan + manual aktif.
      expect(find.text(l10nId.opnameCountingScanButton), findsOneWidget);
      expect(find.text(l10nId.opnameCountingManualButton), findsOneWidget);
      // Daftar terbaru: hanya item yang sudah dihitung, dengan chip hasil.
      expect(
        find.text(l10nId.opnameCountingRecentHeader.toUpperCase()),
        findsOneWidget,
      );
      expect(find.text('Monitor Dell U2723'), findsOneWidget);
      expect(find.text('Kursi Ergonomis Fantoni'), findsOneWidget);
      expect(find.text('Printer HP LaserJet M404'), findsOneWidget);
      expect(find.text('UPS APC Smart 1500VA'), findsNothing);
      expect(find.text(l10nId.opnameResultFound), findsOneWidget);
      expect(find.text(l10nId.opnameResultDamaged), findsOneWidget);
      expect(find.text(l10nId.opnameResultMisplaced), findsOneWidget);
      // Jam hitung item terbaru (locale id memakai titik).
      expect(find.textContaining('09.40', findRichText: true), findsOneWidget);
      // Tooltip menuju variance tersedia.
      expect(
        find.byTooltip(l10nId.opnameCountingVarianceTooltip),
        findsOneWidget,
      );
    });

    testWidgets('belum ada yang dipindai menampilkan teks kosong', (
      WidgetTester tester,
    ) async {
      await tester.pumpWidget(
        buildScreen(
          repository: FakeStockOpnameRepository(
            sessionsData: <StockOpnameSessionDto>[_session],
            itemsData: <StockOpnameItemDto>[_items[3], _items[4]],
          ),
        ),
      );
      await tester.pumpAndSettle();

      expect(find.text(l10nId.opnameCountingRecentEmpty), findsOneWidget);
    });
  });

  group('offline (online-only M0)', () {
    testWidgets('banner tampil dan scan + input manual DINONAKTIFKAN', (
      WidgetTester tester,
    ) async {
      await tester.pumpWidget(
        buildScreen(repository: fullRepository(), online: false),
      );
      await tester.pumpAndSettle();

      expect(find.byType(OfflineBanner), findsOneWidget);
      expect(find.text(l10nId.opnameOfflineBanner), findsOneWidget);

      final FilledButton scanButton = tester.widget<FilledButton>(
        find.ancestor(
          of: find.text(l10nId.opnameCountingScanButton),
          matching: find.byType(FilledButton),
        ),
      );
      expect(scanButton.onPressed, isNull);

      final TextButton manualButton = tester.widget<TextButton>(
        find.ancestor(
          of: find.text(l10nId.opnameCountingManualButton),
          matching: find.byType(TextButton),
        ),
      );
      expect(manualButton.onPressed, isNull);

      final SyncPill pill = tester.widget<SyncPill>(find.byType(SyncPill));
      expect(pill.status, SyncPillStatus.offline);
    });
  });

  group('alur scan', () {
    testWidgets(
      'scan kamera sukses: temuan di luar snapshot ditambahkan + sheet hasil',
      (WidgetTester tester) async {
        final FakeStockOpnameRepository repository = fullRepository();
        repository.unexpectedByTag['JKT01-ELK-2026-00099'] = _unexpectedItem;

        await tester.pumpWidget(buildScreen(repository: repository));
        await tester.pumpAndSettle();
        expect(find.text('Kamera Sony A6400'), findsNothing);

        await tester.tap(find.text(l10nId.opnameCountingScanButton));
        await tester.pumpAndSettle();
        // Halaman kamera terbuka (judul overlay scan).
        expect(find.text(l10nId.scanTitle), findsOneWidget);

        camera.detect('JKT01-ELK-2026-00099');
        await tester.pumpAndSettle();

        // Sheet hasil terbuka untuk item di luar snapshot.
        expect(repository.scanCalls, 1);
        expect(find.text(l10nId.opnameSheetResultLabel), findsOneWidget);
        expect(find.text(l10nId.opnameSheetOutOfSnapshotInfo), findsOneWidget);
        expect(find.text('Kamera Sony A6400'), findsOneWidget);

        await tester.tap(find.text(l10nId.opnameSheetSave));
        await tester.pumpAndSettle();

        // Hasil tersimpan: snackbar + item muncul di daftar terbaru dengan
        // penanda di luar catatan.
        expect(repository.setResultCalls, 1);
        expect(find.text(l10nId.opnameResultSavedSnack), findsOneWidget);
        expect(find.text('Kamera Sony A6400'), findsOneWidget);
        expect(find.textContaining(l10nId.opnameOutOfSnapshot), findsOneWidget);
      },
    );

    testWidgets('input manual: tag tidak dikenal menampilkan pesan 404', (
      WidgetTester tester,
    ) async {
      final FakeStockOpnameRepository repository = fullRepository();

      await tester.pumpWidget(buildScreen(repository: repository));
      await tester.pumpAndSettle();

      await tester.tap(find.text(l10nId.opnameCountingManualButton));
      await tester.pumpAndSettle();
      expect(find.text(l10nId.scanManualSheetTitle), findsOneWidget);

      await tester.enterText(find.byType(TextField), 'TAG-ASING');
      await tester.tap(find.text(l10nId.scanManualSubmit));
      await tester.pumpAndSettle();

      expect(repository.scanCalls, 1);
      expect(
        find.text(l10nId.opnameScanErrorNotFound('TAG-ASING')),
        findsOneWidget,
      );
    });

    testWidgets('scan item snapshot lalu ubah hasil jadi Rusak', (
      WidgetTester tester,
    ) async {
      final FakeStockOpnameRepository repository = fullRepository();

      await tester.pumpWidget(buildScreen(repository: repository));
      await tester.pumpAndSettle();

      await tester.tap(find.text(l10nId.opnameCountingScanButton));
      await tester.pumpAndSettle();
      camera.detect('JKT01-ELK-2024-00042');
      await tester.pumpAndSettle();

      // Sheet untuk item snapshot (UPS, masih pending) — tanpa info di luar
      // catatan.
      expect(find.text('UPS APC Smart 1500VA'), findsOneWidget);
      expect(find.text(l10nId.opnameSheetOutOfSnapshotInfo), findsNothing);

      final Finder sheet = find.byType(BottomSheet);
      await tester.tap(
        find.descendant(
          of: sheet,
          matching: find.text(l10nId.opnameResultDamaged),
        ),
      );
      await tester.pump();
      await tester.tap(find.text(l10nId.opnameSheetSave));
      await tester.pumpAndSettle();

      expect(repository.setResultCalls, 1);
      // Item kini tampil di daftar terbaru dengan chip Rusak (dua item rusak).
      expect(find.text('UPS APC Smart 1500VA'), findsOneWidget);
      expect(find.text(l10nId.opnameResultDamaged), findsNWidgets(2));
    });
  });

  group('state loading dan error', () {
    testWidgets('loading menampilkan skeleton', (WidgetTester tester) async {
      await tester.pumpWidget(buildScreen(repository: _NeverRepository()));
      await tester.pump();

      expect(find.byType(AppSkeleton), findsWidgets);
    });

    testWidgets('404 sesi menampilkan empty state tidak ditemukan', (
      WidgetTester tester,
    ) async {
      await tester.pumpWidget(
        buildScreen(repository: fullRepository(), sessionId: 'op-x'),
      );
      await tester.pumpAndSettle();

      expect(find.text(l10nId.opnameDetailNotFoundTitle), findsOneWidget);
      expect(find.text(l10nId.opnameDetailNotFoundBody), findsOneWidget);
    });
  });
}
