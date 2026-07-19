import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:inventra_mobile/core/api/app_failure.dart';
import 'package:inventra_mobile/core/auth/auth_controller.dart';
import 'package:inventra_mobile/core/auth/auth_session.dart';
import 'package:inventra_mobile/core/auth/data/user_dto.dart';
import 'package:inventra_mobile/core/camera/scan_camera.dart';
import 'package:inventra_mobile/core/connectivity/connectivity_provider.dart';
import 'package:inventra_mobile/core/masterdata/reference_lookup_repository.dart';
import 'package:inventra_mobile/core/utils/clock.dart';
import 'package:inventra_mobile/features/account/data/account_repository.dart';
import 'package:inventra_mobile/features/approval/data/approval_repository.dart';
import 'package:inventra_mobile/features/approval/data/request_dto.dart';
import 'package:inventra_mobile/features/approval/data/request_list_dto.dart';
import 'package:inventra_mobile/features/home/presentation/home_screen.dart';
import 'package:inventra_mobile/features/notifications/data/notifications_repository.dart';
import 'package:inventra_mobile/features/stock_opname/data/stock_opname_repository.dart';
import 'package:inventra_mobile/features/stock_opname/data/stock_opname_session_dto.dart';
import 'package:inventra_mobile/features/stock_opname/data/stock_opname_session_list_dto.dart';
import 'package:mocktail/mocktail.dart';

import '../../../helpers/fake_account_repository.dart';
import '../../../helpers/fake_auth_controller.dart';
import '../../../helpers/fake_notifications_repository.dart';
import '../../../helpers/fake_reference_lookup.dart';
import '../../../helpers/fake_scan_camera.dart';
import '../../../helpers/fake_stock_opname_repository.dart';
import '../../../helpers/test_app.dart';

class _MockApprovalRepository extends Mock implements ApprovalRepository {}

/// [FakeStockOpnameRepository] yang daftar sesinya gagal — untuk membuktikan
/// kartu opname error TIDAK menjatuhkan kartu lain.
class _FailingStockOpnameRepository extends FakeStockOpnameRepository {
  @override
  Future<StockOpnameSessionListDto> sessions({
    String? status,
    int limit = 20,
    int offset = 0,
  }) async => throw const NetworkFailure();
}

final DateTime _frozenNow = DateTime(2026, 7, 19, 9, 41);

/// Pengguna dengan kantor terisi (subjudul header di-resolve via lookup).
const UserDto _user = UserDto(
  id: 'user-1',
  name: 'Andi Pratama',
  email: 'andi.pratama@bank.co.id',
  roleId: 'role-1',
  officeId: 'office-1',
  status: 'active',
  googleLinked: false,
);

final StockOpnameSessionDto _runningSession = StockOpnameSessionDto(
  id: 'op-1',
  officeId: 'office-1',
  name: 'Opname Semester II - Lantai 3',
  period: DateTime(2026, 7),
  status: 'counting',
  startedById: 'user-1',
  officeName: 'Cabang Jakarta Selatan',
  total: 150,
  found: 120,
  pending: 22,
  variance: 8,
);

RequestDto _request({
  required String id,
  String type = 'assignment',
  String? reason,
  String requestedByName = 'Dewi Lestari',
  DateTime? createdAt,
}) {
  return RequestDto(
    id: id,
    type: type,
    status: 'pending',
    currentStep: 1,
    reason: reason,
    requestedById: 'user-maker',
    requestedByName: requestedByName,
    createdAt: createdAt ?? _frozenNow.subtract(const Duration(hours: 2)),
  );
}

RequestListDto _pendingPage(List<RequestDto> items, {int? total}) {
  return RequestListDto(
    data: items,
    total: total ?? items.length,
    limit: 20,
    offset: 0,
  );
}

void main() {
  late _MockApprovalRepository approvalRepository;

  setUp(() {
    approvalRepository = _MockApprovalRepository();
    when(() => approvalRepository.inboxCount()).thenAnswer((_) async => 12);
  });

  void stubPending(RequestListDto page) {
    when(
      () => approvalRepository.list(
        filter: ApprovalStatusFilter.pending,
        offset: any(named: 'offset'),
        limit: any(named: 'limit'),
      ),
    ).thenAnswer((_) async => page);
  }

  ProviderContainer createContainer({
    StockOpnameRepository? opnameRepository,
    bool online = true,
  }) {
    return ProviderContainer.test(
      overrides: [
        authControllerProvider.overrideWith(
          () => FakeAuthController(initialSession: const Authenticated(_user)),
        ),
        stockOpnameRepositoryProvider.overrideWithValue(
          opnameRepository ??
              FakeStockOpnameRepository(
                sessionsData: <StockOpnameSessionDto>[_runningSession],
              ),
        ),
        approvalRepositoryProvider.overrideWithValue(approvalRepository),
        notificationsRepositoryProvider.overrideWithValue(
          FakeNotificationsRepository(),
        ),
        // Rute /account kini layar Profil nyata (Task 12) — tes navigasi
        // avatar tidak boleh menyentuh HTTP daftar sesi.
        accountRepositoryProvider.overrideWithValue(FakeAccountRepository()),
        referenceLookupRepositoryProvider.overrideWithValue(
          FakeReferenceLookup(<String, String>{
            'office:office-1': 'Cabang Jakarta Selatan',
          }),
        ),
        isOnlineProvider.overrideWith((Ref ref) => Stream<bool>.value(online)),
        clockProvider.overrideWithValue(() => _frozenNow),
      ],
    );
  }

  Future<void> pumpHome(
    WidgetTester tester, {
    ProviderContainer? container,
  }) async {
    tester.view.physicalSize = const Size(500, 1800);
    tester.view.devicePixelRatio = 1.0;
    addTearDown(tester.view.reset);
    await tester.pumpWidget(
      buildScreenHarness(
        container: container ?? createContainer(),
        child: const HomeScreen(),
      ),
    );
  }

  group('helper header', () {
    test('avatarInitials: dua kata pertama, defensif', () {
      expect(avatarInitials('Andi Pratama'), 'AP');
      expect(avatarInitials('Siti'), 'S');
      expect(avatarInitials('  budi   santoso jaya '), 'BS');
      expect(avatarInitials(''), '?');
    });

    test('greetingName: kata pertama nama lengkap', () {
      expect(greetingName('Andi Pratama'), 'Andi');
      expect(greetingName('Siti'), 'Siti');
    });
  });

  group('state data penuh', () {
    testWidgets('header: sapaan nama + kantor + inisial avatar', (
      WidgetTester tester,
    ) async {
      stubPending(_pendingPage(<RequestDto>[_request(id: 'req-1')]));
      await pumpHome(tester);
      await tester.pumpAndSettle();

      expect(find.text(l10nId.homeGreeting('Andi')), findsOneWidget);
      expect(find.text('Cabang Jakarta Selatan'), findsOneWidget);
      expect(find.text('AP'), findsOneWidget);
    });

    testWidgets('kartu opname: nama sesi + progress + CTA lanjutkan', (
      WidgetTester tester,
    ) async {
      stubPending(_pendingPage(const <RequestDto>[], total: 0));
      await pumpHome(tester);
      await tester.pumpAndSettle();

      expect(find.text(l10nId.homeOpnameCardTitle), findsOneWidget);
      expect(find.text('Opname Semester II - Lantai 3'), findsOneWidget);
      // KPI dari detail sesi: counted = total - pending = 128 dari 150 (85%).
      expect(find.text(l10nId.homeOpnameProgress(128, 150)), findsOneWidget);
      expect(find.text('85%'), findsOneWidget);
      expect(find.text(l10nId.homeOpnameContinue), findsOneWidget);
      expect(find.text(l10nId.commonSyncSynced), findsOneWidget);
    });

    testWidgets(
      'kartu approval: total server + subjudul stale + dua baris terbaru',
      (WidgetTester tester) async {
        stubPending(
          _pendingPage(<RequestDto>[
            _request(id: 'req-1', reason: 'Proyektor Epson EB-X500'),
            _request(
              id: 'req-2',
              type: 'asset_disposal',
              reason: '3 unit PC Desktop Lenovo',
              requestedByName: 'Rudi Hartono',
              createdAt: _frozenNow.subtract(const Duration(days: 5)),
            ),
            _request(id: 'req-3', reason: 'Tidak dirender (baris ke-3)'),
          ], total: 12),
        );
        await pumpHome(tester);
        await tester.pumpAndSettle();

        expect(find.text(l10nId.homeApprovalCardTitle), findsOneWidget);
        // Angka besar kartu + badge quick action Approval (inbox count 12).
        expect(find.text('12'), findsNWidgets(2));
        expect(find.text(l10nId.homeApprovalStale(1)), findsOneWidget);
        expect(
          find.text(
            '${l10nId.approvalTypeAssignment} · Proyektor Epson EB-X500',
          ),
          findsOneWidget,
        );
        expect(
          find.text(
            '${l10nId.approvalTypeAssetDisposal} · 3 unit PC Desktop Lenovo',
          ),
          findsOneWidget,
        );
        expect(
          find.text('Dewi Lestari · ${l10nId.approvalTimeHoursAgo(2)}'),
          findsOneWidget,
        );
        // Hanya dua pengajuan terbaru yang dirender.
        expect(find.textContaining('baris ke-3'), findsNothing);
        expect(find.text(l10nId.homeApprovalOpenInbox), findsOneWidget);
      },
    );

    testWidgets('quick actions: empat label tile', (WidgetTester tester) async {
      stubPending(_pendingPage(const <RequestDto>[], total: 0));
      await pumpHome(tester);
      await tester.pumpAndSettle();

      expect(find.text(l10nId.homeQuickScan), findsOneWidget);
      expect(find.text(l10nId.homeQuickOpname), findsOneWidget);
      expect(find.text(l10nId.homeQuickApproval), findsOneWidget);
      expect(find.text(l10nId.homeQuickNotifications), findsOneWidget);
    });

    testWidgets('offline: banner + pill kartu opname offline', (
      WidgetTester tester,
    ) async {
      stubPending(_pendingPage(const <RequestDto>[], total: 0));
      await pumpHome(tester, container: createContainer(online: false));
      await tester.pumpAndSettle();

      expect(find.text(l10nId.homeOfflineBanner), findsOneWidget);
      expect(find.text(l10nId.commonSyncOffline), findsOneWidget);
    });
  });

  group('empty state per kartu', () {
    testWidgets('tanpa sesi berjalan: pesan kosong + aksi Buka Opname', (
      WidgetTester tester,
    ) async {
      stubPending(_pendingPage(const <RequestDto>[], total: 0));
      final StockOpnameSessionDto closed = StockOpnameSessionDto(
        id: 'op-2',
        officeId: 'office-1',
        name: 'Opname Semester I',
        status: 'closed',
        startedById: 'user-1',
        period: DateTime(2026),
      );
      await pumpHome(
        tester,
        container: createContainer(
          opnameRepository: FakeStockOpnameRepository(
            sessionsData: <StockOpnameSessionDto>[closed],
          ),
        ),
      );
      await tester.pumpAndSettle();

      expect(find.text(l10nId.homeOpnameEmptyBody), findsOneWidget);
      expect(find.text(l10nId.homeOpnameOpenList), findsOneWidget);
      expect(find.text(l10nId.homeOpnameContinue), findsNothing);
    });

    testWidgets('tanpa pengajuan pending: angka 0 + pesan kosong', (
      WidgetTester tester,
    ) async {
      when(() => approvalRepository.inboxCount()).thenAnswer((_) async => 0);
      stubPending(_pendingPage(const <RequestDto>[], total: 0));
      await pumpHome(tester);
      await tester.pumpAndSettle();

      expect(find.text('0'), findsOneWidget);
      expect(find.text(l10nId.homeApprovalEmptyBody), findsOneWidget);
      expect(find.text(l10nId.homeApprovalOpenInbox), findsOneWidget);
    });
  });

  group('kartu gagal independen (non-fatal)', () {
    testWidgets(
      'approval gagal: kartu lain + halaman tetap hidup, retry memulihkan',
      (WidgetTester tester) async {
        when(
          () => approvalRepository.list(
            filter: ApprovalStatusFilter.pending,
            offset: any(named: 'offset'),
            limit: any(named: 'limit'),
          ),
        ).thenThrow(const NetworkFailure());
        await pumpHome(tester);
        await tester.pumpAndSettle();

        // Kartu approval menampilkan error ...
        expect(find.text(l10nId.homeApprovalErrorBody), findsOneWidget);
        // ... sementara header, kartu opname, dan quick actions tetap hidup.
        expect(find.text(l10nId.homeGreeting('Andi')), findsOneWidget);
        expect(find.text('Opname Semester II - Lantai 3'), findsOneWidget);
        expect(find.text(l10nId.homeQuickScan), findsOneWidget);

        stubPending(
          _pendingPage(<RequestDto>[
            _request(id: 'req-1', reason: 'Setelah retry'),
          ]),
        );
        await tester.tap(find.text(l10nId.commonRetry));
        await tester.pumpAndSettle();

        expect(
          find.text('${l10nId.approvalTypeAssignment} · Setelah retry'),
          findsOneWidget,
        );
      },
    );

    testWidgets('opname gagal: kartu approval tetap hidup', (
      WidgetTester tester,
    ) async {
      stubPending(_pendingPage(<RequestDto>[_request(id: 'req-1')], total: 3));
      await pumpHome(
        tester,
        container: createContainer(
          opnameRepository: _FailingStockOpnameRepository(),
        ),
      );
      await tester.pumpAndSettle();

      expect(find.text(l10nId.homeOpnameErrorBody), findsOneWidget);
      expect(find.text('3'), findsOneWidget);
      expect(find.text(l10nId.homeApprovalCardTitle), findsOneWidget);
      expect(find.text(l10nId.homeGreeting('Andi')), findsOneWidget);
    });
  });

  group('navigasi (router penuh)', () {
    Future<ProviderContainer> pumpRouter(WidgetTester tester) async {
      stubPending(_pendingPage(<RequestDto>[_request(id: 'req-1')], total: 1));
      final ProviderContainer container = ProviderContainer.test(
        overrides: [
          authControllerProvider.overrideWith(
            () =>
                FakeAuthController(initialSession: const Authenticated(_user)),
          ),
          scanCameraFactoryProvider.overrideWithValue(FakeScanCamera.new),
          stockOpnameRepositoryProvider.overrideWithValue(
            FakeStockOpnameRepository(
              sessionsData: <StockOpnameSessionDto>[_runningSession],
            ),
          ),
          approvalRepositoryProvider.overrideWithValue(approvalRepository),
          notificationsRepositoryProvider.overrideWithValue(
            FakeNotificationsRepository(),
          ),
          referenceLookupRepositoryProvider.overrideWithValue(
            FakeReferenceLookup(),
          ),
          clockProvider.overrideWithValue(() => _frozenNow),
        ],
      );
      tester.view.physicalSize = const Size(500, 1800);
      tester.view.devicePixelRatio = 1.0;
      addTearDown(tester.view.reset);
      await tester.pumpWidget(RouterTestApp(container: container));
      await tester.pumpAndSettle();
      return container;
    }

    testWidgets('quick action Sesi Opname membuka tab opname', (
      WidgetTester tester,
    ) async {
      await pumpRouter(tester);

      await tester.tap(find.text(l10nId.homeQuickOpname));
      await tester.pumpAndSettle();

      expect(find.text(l10nId.opnameSessionsTitle), findsOneWidget);
    });

    testWidgets('quick action Approval membuka inbox', (
      WidgetTester tester,
    ) async {
      await pumpRouter(tester);

      // Label quick action (11px) dibedakan dari label tab shell (10.5px).
      await tester.tap(
        find.byWidgetPredicate(
          (Widget w) =>
              w is Text &&
              w.data == l10nId.homeQuickApproval &&
              w.style?.fontSize == 11,
        ),
      );
      await tester.pumpAndSettle();

      expect(find.text(l10nId.approvalInboxFilterApproved), findsOneWidget);
    });

    testWidgets('lonceng header membuka tab notifikasi', (
      WidgetTester tester,
    ) async {
      await pumpRouter(tester);

      await tester.tap(find.byKey(const ValueKey<String>('home-bell')));
      await tester.pumpAndSettle();

      expect(find.text(l10nId.notificationsEmptyTitle), findsOneWidget);
    });

    testWidgets('avatar membuka profil di atas shell', (
      WidgetTester tester,
    ) async {
      await pumpRouter(tester);

      await tester.tap(find.byKey(const ValueKey<String>('home-avatar')));
      await tester.pumpAndSettle();

      expect(find.text(l10nId.accountTitle), findsOneWidget);
      expect(find.text(l10nId.shellTabScan), findsNothing);
    });

    testWidgets('CTA Lanjutkan membuka layar counting sesi', (
      WidgetTester tester,
    ) async {
      await pumpRouter(tester);

      await tester.tap(find.text(l10nId.homeOpnameContinue));
      await tester.pumpAndSettle();

      expect(find.text('Opname Semester II - Lantai 3'), findsWidgets);
      expect(find.text(l10nId.shellTabScan), findsNothing);
    });
  });
}
