import 'dart:async';

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:inventra_mobile/core/api/app_failure.dart';
import 'package:inventra_mobile/core/connectivity/connectivity_provider.dart';
import 'package:inventra_mobile/core/widgets/app_skeleton.dart';
import 'package:inventra_mobile/features/stock_opname/data/stock_opname_item_dto.dart';
import 'package:inventra_mobile/features/stock_opname/data/stock_opname_repository.dart';
import 'package:inventra_mobile/features/stock_opname/data/stock_opname_session_dto.dart';
import 'package:inventra_mobile/features/stock_opname/presentation/opname_variance_screen.dart';

import '../../../helpers/fake_stock_opname_repository.dart';
import '../../../helpers/test_app.dart';

const StockOpnameSessionDto _session = StockOpnameSessionDto(
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

const List<StockOpnameItemDto> _varianceItems = <StockOpnameItemDto>[
  // Tercocokkan — TIDAK ikut daftar variance.
  StockOpnameItemDto(
    id: 'item-0',
    sessionId: 'op-1',
    assetId: 'asset-0',
    assetName: 'Monitor Dell U2723',
    assetTag: 'JKT01-ELK-2026-00014',
    expected: true,
    result: 'found',
  ),
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
    expected: true,
    result: 'not_found',
    followupRequestId: 'req-1',
  ),
  StockOpnameItemDto(
    id: 'item-3',
    sessionId: 'op-1',
    assetId: 'asset-3',
    assetName: 'Genset Perkins 100 kVA',
    assetTag: 'JKT01-MSN-2022-00007',
    expected: true,
    result: 'damaged',
    followupRecordId: 'rec-1',
  ),
  StockOpnameItemDto(
    id: 'item-4',
    sessionId: 'op-1',
    assetId: 'asset-4',
    assetName: 'Printer HP LaserJet M404',
    assetTag: 'JKT01-ELK-2025-00087',
    expected: true,
    result: 'misplaced',
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

/// Repository dengan detail sesi yang gagal — untuk cabang error.
class _FailingRepository extends FakeStockOpnameRepository {
  _FailingRepository(this.failure);

  final AppFailure failure;

  @override
  Future<StockOpnameSessionDto> session(String id) async => throw failure;
}

/// Repository yang tidak pernah selesai — untuk state loading.
class _NeverRepository extends FakeStockOpnameRepository {
  @override
  Future<StockOpnameSessionDto> session(String id) =>
      Completer<StockOpnameSessionDto>().future;
}

void main() {
  ProviderContainer createContainer(StockOpnameRepository repository) {
    return ProviderContainer.test(
      overrides: [
        stockOpnameRepositoryProvider.overrideWithValue(repository),
        isOnlineProvider.overrideWith((Ref ref) => Stream<bool>.value(true)),
      ],
    );
  }

  Widget buildScreen(StockOpnameRepository repository) {
    return buildScreenHarness(
      container: createContainer(repository),
      child: const OpnameVarianceScreen(sessionId: 'op-1'),
    );
  }

  FakeStockOpnameRepository fullRepository() => FakeStockOpnameRepository(
    sessionsData: <StockOpnameSessionDto>[_session],
    itemsData: _varianceItems,
  );

  /// Viewport tinggi supaya seluruh kelompok + catatan kaki ter-build
  /// (ListView lazy — konten di luar layar tidak dirender).
  void useTallViewport(WidgetTester tester) {
    tester.view.physicalSize = const Size(390, 2600);
    tester.view.devicePixelRatio = 1.0;
    addTearDown(tester.view.reset);
  }

  group('state data', () {
    testWidgets('ringkasan empat kategori + kelompok item', (
      WidgetTester tester,
    ) async {
      useTallViewport(tester);
      await tester.pumpWidget(buildScreen(fullRepository()));
      await tester.pumpAndSettle();

      // Header sesi + toggle.
      expect(find.text('Opname Tahunan 2026'), findsOneWidget);
      expect(find.text(l10nId.opnameVarianceTabItems), findsOneWidget);
      expect(find.text(l10nId.opnameVarianceTabVariance), findsOneWidget);

      // Judul kelompok dengan jumlah (label ringkasan memakai teks sama).
      expect(
        find.text('${l10nId.opnameResultNotFound.toUpperCase()} (2)'),
        findsOneWidget,
      );
      expect(
        find.text('${l10nId.opnameResultDamaged.toUpperCase()} (1)'),
        findsOneWidget,
      );
      expect(
        find.text('${l10nId.opnameResultMisplaced.toUpperCase()} (1)'),
        findsOneWidget,
      );
      expect(
        find.text('${l10nId.opnameOutOfSnapshot.toUpperCase()} (1)'),
        findsOneWidget,
      );

      // Item variance tampil; item tercocokkan tidak.
      expect(find.text('Kamera Sony A6400'), findsOneWidget);
      expect(find.text('Kursi Rapat Chitose'), findsOneWidget);
      expect(find.text('Genset Perkins 100 kVA'), findsOneWidget);
      expect(find.text('Printer HP LaserJet M404'), findsOneWidget);
      expect(find.text('Proyektor Epson EB-X500'), findsOneWidget);
      expect(find.text('Monitor Dell U2723'), findsNothing);

      // Catatan petugas + lokasi terakhir.
      expect(
        find.text(
          l10nId.opnameVarianceNote(
            'Sudah dicari di seluruh lantai 2, tidak ada.',
          ),
        ),
        findsOneWidget,
      );
      expect(
        find.textContaining(
          l10nId.opnameVarianceLastLocation('R. Marketing, Lantai 2'),
        ),
        findsOneWidget,
      );

      // Status tindak lanjut tiga varian.
      expect(find.text(l10nId.opnameVarianceFollowupNone), findsNWidgets(3));
      expect(find.text(l10nId.opnameVarianceFollowupRequested), findsOneWidget);
      expect(find.text(l10nId.opnameVarianceFollowupRecord), findsOneWidget);

      // Catatan kaki.
      expect(find.text(l10nId.opnameVarianceFootnote), findsOneWidget);
    });

    testWidgets('chip hasil per item variance (StatusChip)', (
      WidgetTester tester,
    ) async {
      useTallViewport(tester);
      await tester.pumpWidget(buildScreen(fullRepository()));
      await tester.pumpAndSettle();

      // Label hasil dipakai kartu ringkasan DAN chip per item (judul kelompok
      // memakai varian uppercase sehingga tidak ikut terhitung):
      // not_found = 1 ringkasan + 2 chip; damaged/misplaced = 1 ringkasan +
      // 1 chip.
      expect(find.text(l10nId.opnameResultNotFound), findsNWidgets(3));
      expect(find.text(l10nId.opnameResultDamaged), findsNWidgets(2));
      expect(find.text(l10nId.opnameResultMisplaced), findsNWidgets(2));
      // Temuan di luar catatan berhasil ditemukan -> chip Ditemukan (tidak
      // ada kartu ringkasan "Ditemukan").
      expect(find.text(l10nId.opnameResultFound), findsOneWidget);
    });
  });

  testWidgets('empty state: semua tercocokkan', (WidgetTester tester) async {
    await tester.pumpWidget(
      buildScreen(
        FakeStockOpnameRepository(
          sessionsData: <StockOpnameSessionDto>[_session],
          itemsData: <StockOpnameItemDto>[_varianceItems.first],
        ),
      ),
    );
    await tester.pumpAndSettle();

    expect(find.text(l10nId.opnameVarianceEmptyTitle), findsOneWidget);
    expect(find.text(l10nId.opnameVarianceEmptyBody(150)), findsOneWidget);
    expect(find.text(l10nId.opnameVarianceFootnote), findsOneWidget);
  });

  testWidgets('segmen Item kembali (pop) ke layar sebelumnya', (
    WidgetTester tester,
  ) async {
    await tester.pumpWidget(
      buildScreenHarness(
        container: createContainer(fullRepository()),
        child: const Scaffold(body: SizedBox.shrink()),
      ),
    );
    final NavigatorState navigator = tester.state<NavigatorState>(
      find.byType(Navigator),
    );
    unawaited(
      navigator.push(
        MaterialPageRoute<void>(
          builder: (BuildContext context) =>
              const OpnameVarianceScreen(sessionId: 'op-1'),
        ),
      ),
    );
    await tester.pumpAndSettle();
    expect(find.byType(OpnameVarianceScreen), findsOneWidget);

    await tester.tap(find.text(l10nId.opnameVarianceTabItems));
    await tester.pumpAndSettle();

    expect(find.byType(OpnameVarianceScreen), findsNothing);
  });

  testWidgets('state loading menampilkan skeleton', (
    WidgetTester tester,
  ) async {
    await tester.pumpWidget(buildScreen(_NeverRepository()));
    await tester.pump();

    expect(find.byType(AppSkeleton), findsWidgets);
  });

  group('state error', () {
    testWidgets('gangguan jaringan: pesan + coba lagi', (
      WidgetTester tester,
    ) async {
      await tester.pumpWidget(
        buildScreen(_FailingRepository(const NetworkFailure())),
      );
      await tester.pumpAndSettle();

      expect(find.text(l10nId.opnameDetailErrorTitle), findsOneWidget);
      expect(find.text(l10nId.opnameErrorNetworkBody), findsOneWidget);
      expect(find.text(l10nId.commonRetry), findsOneWidget);
    });

    testWidgets('404 sesi: empty state tidak ditemukan', (
      WidgetTester tester,
    ) async {
      await tester.pumpWidget(
        buildScreen(_FailingRepository(const NotFoundFailure())),
      );
      await tester.pumpAndSettle();

      expect(find.text(l10nId.opnameDetailNotFoundTitle), findsOneWidget);
    });
  });
}
