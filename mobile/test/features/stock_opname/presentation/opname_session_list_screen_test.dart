import 'dart:async';

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:inventra_mobile/core/api/app_failure.dart';
import 'package:inventra_mobile/core/connectivity/connectivity_provider.dart';
import 'package:inventra_mobile/core/widgets/app_skeleton.dart';
import 'package:inventra_mobile/core/widgets/offline_banner.dart';
import 'package:inventra_mobile/core/widgets/sync_pill.dart';
import 'package:inventra_mobile/features/stock_opname/data/stock_opname_repository.dart';
import 'package:inventra_mobile/features/stock_opname/data/stock_opname_session_dto.dart';
import 'package:inventra_mobile/features/stock_opname/data/stock_opname_session_list_dto.dart';
import 'package:inventra_mobile/features/stock_opname/presentation/opname_session_list_screen.dart';

import '../../../helpers/fake_stock_opname_repository.dart';
import '../../../helpers/test_app.dart';

const StockOpnameSessionDto _runningSession = StockOpnameSessionDto(
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

const StockOpnameSessionDto _closedSession = StockOpnameSessionDto(
  id: 'op-2',
  officeId: 'office-1',
  name: 'Opname Semester I 2026',
  status: 'closed',
  startedById: 'user-1',
  officeName: 'Cabang Jakarta Selatan',
  total: 150,
  found: 150,
  pending: 0,
  variance: 0,
);

/// Repository yang gagal dengan [failure] — untuk cabang error.
class _FailingRepository extends FakeStockOpnameRepository {
  _FailingRepository(this.failure);

  final AppFailure failure;

  @override
  Future<StockOpnameSessionListDto> sessions({
    String? status,
    int limit = 20,
    int offset = 0,
  }) async => throw failure;
}

/// Repository yang tidak pernah selesai — untuk state loading.
class _NeverRepository extends FakeStockOpnameRepository {
  @override
  Future<StockOpnameSessionListDto> sessions({
    String? status,
    int limit = 20,
    int offset = 0,
  }) => Completer<StockOpnameSessionListDto>().future;
}

void main() {
  /// Viewport tinggi supaya seluruh kartu + catatan kaki ter-build (ListView
  /// lazy — konten di luar layar tidak dirender).
  void useTallViewport(WidgetTester tester) {
    tester.view.physicalSize = const Size(390, 1800);
    tester.view.devicePixelRatio = 1.0;
    addTearDown(tester.view.reset);
  }

  Widget buildScreen({
    required StockOpnameRepository repository,
    bool online = true,
  }) {
    final ProviderContainer container = ProviderContainer.test(
      overrides: [
        stockOpnameRepositoryProvider.overrideWithValue(repository),
        isOnlineProvider.overrideWith((Ref ref) => Stream<bool>.value(online)),
      ],
    );
    return buildScreenHarness(
      container: container,
      child: const OpnameSessionListScreen(),
    );
  }

  group('state data', () {
    testWidgets('kartu sesi berjalan: judul, subjudul, progress, CTA', (
      WidgetTester tester,
    ) async {
      useTallViewport(tester);
      await tester.pumpWidget(
        buildScreen(
          repository: FakeStockOpnameRepository(
            sessionsData: <StockOpnameSessionDto>[_runningSession],
          ),
        ),
      );
      await tester.pumpAndSettle();

      expect(find.text(l10nId.opnameSessionsTitle), findsOneWidget);
      expect(find.text('Opname Tahunan 2026'), findsOneWidget);
      expect(find.text('Cabang Jakarta Selatan'), findsOneWidget);
      // KPI detail: counted = total - pending = 128.
      expect(
        find.text(l10nId.opnameSessionsProgress(128, 150)),
        findsOneWidget,
      );
      expect(find.text('85%'), findsOneWidget);
      // "Berjalan" tampil dua kali: chip filter aktif + StatusChip status
      // sesi counting (kunci ARB memang sama-sama "Berjalan").
      expect(find.text(l10nId.opnameStatusCounting), findsNWidgets(2));
      expect(find.text(l10nId.opnameSessionsContinue), findsOneWidget);
      // Online-only: pill sync menunjukkan tersinkron.
      expect(find.text(l10nId.commonSyncSynced), findsOneWidget);
      expect(find.text(l10nId.opnameSessionsFootnote), findsOneWidget);
    });

    testWidgets('kartu sesi selesai: chip Berita Acara tanpa CTA', (
      WidgetTester tester,
    ) async {
      useTallViewport(tester);
      await tester.pumpWidget(
        buildScreen(
          repository: FakeStockOpnameRepository(
            sessionsData: <StockOpnameSessionDto>[_closedSession],
          ),
        ),
      );
      await tester.pumpAndSettle();

      // Tab default Berjalan menyembunyikan sesi closed.
      expect(find.text('Opname Semester I 2026'), findsNothing);

      await tester.tap(find.text(l10nId.opnameSessionsFilterClosed));
      await tester.pumpAndSettle();

      expect(find.text('Opname Semester I 2026'), findsOneWidget);
      // "Selesai" tampil dua kali: chip filter aktif + StatusChip status sesi
      // (kunci ARB memang sama-sama "Selesai").
      expect(find.text(l10nId.opnameStatusClosed), findsNWidgets(2));
      expect(find.text(l10nId.opnameSessionsReportOnWeb), findsOneWidget);
      expect(find.text(l10nId.opnameSessionsContinue), findsNothing);
      expect(find.text('100%'), findsOneWidget);
    });

    testWidgets('tab Semua menampilkan kedua sesi', (
      WidgetTester tester,
    ) async {
      useTallViewport(tester);
      await tester.pumpWidget(
        buildScreen(
          repository: FakeStockOpnameRepository(
            sessionsData: <StockOpnameSessionDto>[
              _runningSession,
              _closedSession,
            ],
          ),
        ),
      );
      await tester.pumpAndSettle();

      await tester.tap(find.text(l10nId.opnameSessionsFilterAll));
      await tester.pumpAndSettle();

      expect(find.text('Opname Tahunan 2026'), findsOneWidget);
      expect(find.text('Opname Semester I 2026'), findsOneWidget);
    });
  });

  group('empty state', () {
    testWidgets('tab Berjalan kosong memakai teks mockup', (
      WidgetTester tester,
    ) async {
      await tester.pumpWidget(
        buildScreen(repository: FakeStockOpnameRepository()),
      );
      await tester.pumpAndSettle();

      expect(find.text(l10nId.opnameSessionsEmptyTitle), findsOneWidget);
      expect(find.text(l10nId.opnameSessionsEmptyBody), findsOneWidget);
    });

    testWidgets('tab Selesai kosong memakai teks filter', (
      WidgetTester tester,
    ) async {
      await tester.pumpWidget(
        buildScreen(
          repository: FakeStockOpnameRepository(
            sessionsData: <StockOpnameSessionDto>[_runningSession],
          ),
        ),
      );
      await tester.pumpAndSettle();

      await tester.tap(find.text(l10nId.opnameSessionsFilterClosed));
      await tester.pumpAndSettle();

      expect(
        find.text(l10nId.opnameSessionsEmptyFilteredTitle),
        findsOneWidget,
      );
    });
  });

  testWidgets('state loading menampilkan skeleton', (
    WidgetTester tester,
  ) async {
    await tester.pumpWidget(buildScreen(repository: _NeverRepository()));
    await tester.pump();

    expect(find.byType(AppSkeleton), findsWidgets);
  });

  group('state error', () {
    testWidgets('gangguan jaringan: pesan offline + tombol coba lagi', (
      WidgetTester tester,
    ) async {
      await tester.pumpWidget(
        buildScreen(repository: _FailingRepository(const NetworkFailure())),
      );
      await tester.pumpAndSettle();

      expect(find.text(l10nId.opnameSessionsErrorTitle), findsOneWidget);
      expect(find.text(l10nId.opnameErrorNetworkBody), findsOneWidget);
      expect(find.text(l10nId.commonRetry), findsOneWidget);
    });

    testWidgets('403 dirender sopan tanpa tombol coba lagi', (
      WidgetTester tester,
    ) async {
      await tester.pumpWidget(
        buildScreen(repository: _FailingRepository(const ForbiddenFailure())),
      );
      await tester.pumpAndSettle();

      expect(find.text(l10nId.opnameForbiddenTitle), findsOneWidget);
      expect(find.text(l10nId.opnameForbiddenBody), findsOneWidget);
      expect(find.text(l10nId.commonRetry), findsNothing);
    });
  });

  testWidgets('offline: banner tampil + SyncPill kartu berstatus offline', (
    WidgetTester tester,
  ) async {
    await tester.pumpWidget(
      buildScreen(
        repository: FakeStockOpnameRepository(
          sessionsData: <StockOpnameSessionDto>[_runningSession],
        ),
        online: false,
      ),
    );
    await tester.pumpAndSettle();

    expect(find.byType(OfflineBanner), findsOneWidget);
    expect(find.text(l10nId.opnameOfflineBanner), findsOneWidget);
    final SyncPill pill = tester.widget<SyncPill>(find.byType(SyncPill));
    expect(pill.status, SyncPillStatus.offline);
  });
}
