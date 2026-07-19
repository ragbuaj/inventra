import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:inventra_mobile/core/api/app_failure.dart';
import 'package:inventra_mobile/core/auth/auth_controller.dart';
import 'package:inventra_mobile/core/auth/auth_session.dart';
import 'package:inventra_mobile/core/masterdata/reference_lookup_repository.dart';
import 'package:inventra_mobile/core/utils/clock.dart';
import 'package:inventra_mobile/core/widgets/app_skeleton.dart';
import 'package:inventra_mobile/core/widgets/status_chip.dart';
import 'package:inventra_mobile/features/approval/data/approval_repository.dart';
import 'package:inventra_mobile/features/approval/data/request_dto.dart';
import 'package:inventra_mobile/features/approval/data/request_list_dto.dart';
import 'package:inventra_mobile/features/approval/presentation/approval_inbox_screen.dart';
import 'package:inventra_mobile/core/camera/scan_camera.dart';
import 'package:mocktail/mocktail.dart';

import '../../../helpers/fake_auth_controller.dart';
import '../../../helpers/fake_reference_lookup.dart';
import '../../../helpers/fake_scan_camera.dart';
import '../../../helpers/test_app.dart';

class _MockApprovalRepository extends Mock implements ApprovalRepository {}

/// Waktu beku: kartu pertama dibuat 2 jam sebelumnya.
final DateTime _frozenNow = DateTime.utc(2026, 7, 19, 9);

RequestDto _request({
  required String id,
  String type = 'asset_transfer',
  String status = 'pending',
  String? amount,
  String? reason,
  String requestedByName = 'Dewi Lestari',
  String officeName = 'Cabang Jakarta Selatan',
  DateTime? createdAt,
}) {
  return RequestDto(
    id: id,
    type: type,
    status: status,
    amount: amount,
    currentStep: 1,
    reason: reason,
    requestedById: 'user-maker',
    requestedByName: requestedByName,
    officeName: officeName,
    createdAt: createdAt ?? _frozenNow.subtract(const Duration(hours: 2)),
  );
}

RequestListDto _page(List<RequestDto> items, {int? total, int offset = 0}) {
  return RequestListDto(
    data: items,
    total: total ?? items.length,
    limit: 20,
    offset: offset,
  );
}

void main() {
  late _MockApprovalRepository repository;

  setUp(() {
    repository = _MockApprovalRepository();
  });

  ProviderContainer createContainer() {
    return ProviderContainer.test(
      overrides: [
        approvalRepositoryProvider.overrideWithValue(repository),
        referenceLookupRepositoryProvider.overrideWithValue(
          FakeReferenceLookup(),
        ),
        clockProvider.overrideWithValue(() => _frozenNow),
      ],
    );
  }

  void stubList(
    ApprovalStatusFilter filter,
    RequestListDto page, {
    int offset = 0,
  }) {
    when(
      () => repository.list(
        filter: filter,
        offset: offset,
        limit: any(named: 'limit'),
      ),
    ).thenAnswer((_) async => page);
  }

  Future<void> pumpInbox(WidgetTester tester) async {
    tester.view.physicalSize = const Size(500, 1600);
    tester.view.devicePixelRatio = 1.0;
    addTearDown(tester.view.reset);
    await tester.pumpWidget(
      buildScreenHarness(
        container: createContainer(),
        child: const ApprovalInboxScreen(),
      ),
    );
  }

  group('state data', () {
    testWidgets(
      'kartu: jenis + judul + maker · kantor + nominal + chip status + waktu',
      (WidgetTester tester) async {
        stubList(
          ApprovalStatusFilter.pending,
          _page(<RequestDto>[
            _request(
              id: 'req-1',
              type: 'asset_create',
              amount: '154800000.00',
              reason: 'Registrasi 12 Laptop Asus ExpertBook',
            ),
            _request(
              id: 'req-2',
              type: 'asset_disposal',
              amount: '27450000.00',
              reason: 'Penghapusan 3 PC Desktop Lenovo M70q',
              requestedByName: 'Rudi Hartono',
            ),
          ], total: 2),
        );
        await pumpInbox(tester);
        await tester.pumpAndSettle();

        expect(find.text(l10nId.approvalInboxTitle), findsOneWidget);
        expect(find.text(l10nId.approvalTypeAssetCreate), findsOneWidget);
        expect(find.text(l10nId.approvalTypeAssetDisposal), findsOneWidget);
        expect(
          find.text('Registrasi 12 Laptop Asus ExpertBook'),
          findsOneWidget,
        );
        expect(
          find.text('Dewi Lestari · Cabang Jakarta Selatan'),
          findsOneWidget,
        );
        expect(find.text('Rp 154.800.000'), findsOneWidget);
        expect(find.text('Rp 27.450.000'), findsOneWidget);
        // Chip status pada kartu (teks "Menunggu" juga muncul di chip filter).
        expect(
          find.descendant(
            of: find.byType(StatusChip),
            matching: find.text(l10nId.approvalStatusPending),
          ),
          findsNWidgets(2),
        );
        // Waktu relatif dari clock beku.
        expect(find.text(l10nId.approvalTimeHoursAgo(2)), findsNWidgets(2));
        // Penanda sensitif hanya pada kartu penghapusan.
        expect(find.text(l10nId.approvalCardSensitive), findsOneWidget);
        // Chip Menunggu membawa jumlah total.
        expect(find.text('2'), findsOneWidget);
      },
    );

    testWidgets('nominal absen (dimask): baris nominal tidak dirender', (
      WidgetTester tester,
    ) async {
      stubList(
        ApprovalStatusFilter.pending,
        _page(<RequestDto>[
          _request(id: 'req-3', reason: 'Mutasi 12 Kursi Kerja ke Lantai 5'),
        ]),
      );
      await pumpInbox(tester);
      await tester.pumpAndSettle();

      expect(find.textContaining('Rp '), findsNothing);
      expect(find.text('Mutasi 12 Kursi Kerja ke Lantai 5'), findsOneWidget);
    });

    testWidgets('tanpa reason: judul memakai label jenis', (
      WidgetTester tester,
    ) async {
      stubList(
        ApprovalStatusFilter.pending,
        _page(<RequestDto>[_request(id: 'req-4', type: 'assignment')]),
      );
      await pumpInbox(tester);
      await tester.pumpAndSettle();

      // Label jenis muncul dua kali: baris jenis + judul fallback.
      expect(find.text(l10nId.approvalTypeAssignment), findsNWidgets(2));
    });
  });

  group('filter', () {
    testWidgets('tap chip Disetujui memuat daftar status approved', (
      WidgetTester tester,
    ) async {
      stubList(
        ApprovalStatusFilter.pending,
        _page(<RequestDto>[_request(id: 'req-1', reason: 'Pengajuan pending')]),
      );
      stubList(
        ApprovalStatusFilter.approved,
        _page(<RequestDto>[
          _request(
            id: 'req-9',
            status: 'approved',
            reason: 'Pengajuan yang sudah disetujui',
          ),
        ]),
      );
      await pumpInbox(tester);
      await tester.pumpAndSettle();

      await tester.tap(find.text(l10nId.approvalInboxFilterApproved));
      await tester.pumpAndSettle();

      expect(find.text('Pengajuan yang sudah disetujui'), findsOneWidget);
      expect(find.text('Pengajuan pending'), findsNothing);
      verify(
        () => repository.list(
          filter: ApprovalStatusFilter.approved,
          offset: 0,
          limit: any(named: 'limit'),
        ),
      ).called(1);
    });

    testWidgets(
      'empty pending: pesan khusus + aksi "Lihat riwayat" pindah ke Semua',
      (WidgetTester tester) async {
        stubList(ApprovalStatusFilter.pending, _page(<RequestDto>[]));
        stubList(
          ApprovalStatusFilter.all,
          _page(<RequestDto>[
            _request(
              id: 'req-9',
              status: 'approved',
              reason: 'Riwayat approval',
            ),
          ]),
        );
        await pumpInbox(tester);
        await tester.pumpAndSettle();

        expect(
          find.text(l10nId.approvalInboxEmptyPendingTitle),
          findsOneWidget,
        );
        expect(find.text(l10nId.approvalInboxEmptyPendingBody), findsOneWidget);

        await tester.tap(find.text(l10nId.approvalInboxEmptyPendingAction));
        await tester.pumpAndSettle();

        expect(find.text('Riwayat approval'), findsOneWidget);
      },
    );

    testWidgets('empty filter lain: pesan generik tanpa aksi', (
      WidgetTester tester,
    ) async {
      stubList(
        ApprovalStatusFilter.pending,
        _page(<RequestDto>[_request(id: 'req-1')]),
      );
      stubList(ApprovalStatusFilter.rejected, _page(<RequestDto>[]));
      await pumpInbox(tester);
      await tester.pumpAndSettle();

      await tester.tap(find.text(l10nId.approvalInboxFilterRejected));
      await tester.pumpAndSettle();

      expect(find.text(l10nId.approvalInboxEmptyFilteredTitle), findsOneWidget);
      expect(find.text(l10nId.approvalInboxEmptyPendingAction), findsNothing);
    });
  });

  group('state loading dan error', () {
    testWidgets('loading: skeleton kartu tampil', (WidgetTester tester) async {
      when(
        () => repository.list(
          filter: ApprovalStatusFilter.pending,
          offset: any(named: 'offset'),
          limit: any(named: 'limit'),
        ),
      ).thenAnswer((_) async {
        await Future<void>.delayed(const Duration(milliseconds: 50));
        return _page(<RequestDto>[]);
      });
      await pumpInbox(tester);
      await tester.pump();

      expect(find.byType(AppSkeleton), findsWidgets);
      await tester.pumpAndSettle();
    });

    testWidgets('offline: pesan jaringan + retry memuat ulang', (
      WidgetTester tester,
    ) async {
      when(
        () => repository.list(
          filter: ApprovalStatusFilter.pending,
          offset: any(named: 'offset'),
          limit: any(named: 'limit'),
        ),
      ).thenThrow(const NetworkFailure());
      await pumpInbox(tester);
      await tester.pumpAndSettle();

      expect(find.text(l10nId.approvalInboxErrorTitle), findsOneWidget);
      expect(find.text(l10nId.approvalInboxErrorNetworkBody), findsOneWidget);

      stubList(
        ApprovalStatusFilter.pending,
        _page(<RequestDto>[_request(id: 'req-1', reason: 'Setelah retry')]),
      );
      await tester.tap(find.text(l10nId.commonRetry));
      await tester.pumpAndSettle();

      expect(find.text('Setelah retry'), findsOneWidget);
    });

    testWidgets('403: pesan akses dibatasi tanpa tombol retry', (
      WidgetTester tester,
    ) async {
      when(
        () => repository.list(
          filter: ApprovalStatusFilter.pending,
          offset: any(named: 'offset'),
          limit: any(named: 'limit'),
        ),
      ).thenThrow(const ForbiddenFailure());
      await pumpInbox(tester);
      await tester.pumpAndSettle();

      expect(find.text(l10nId.approvalInboxForbiddenTitle), findsOneWidget);
      expect(find.text(l10nId.commonRetry), findsNothing);
    });
  });

  group('infinite scroll dan refresh', () {
    testWidgets('scroll ke bawah memuat halaman berikutnya (offset 20)', (
      WidgetTester tester,
    ) async {
      final List<RequestDto> firstPage = List<RequestDto>.generate(
        20,
        (int i) => _request(id: 'req-$i', reason: 'Pengajuan nomor $i'),
      );
      stubList(ApprovalStatusFilter.pending, _page(firstPage, total: 25));
      stubList(
        ApprovalStatusFilter.pending,
        _page(
          List<RequestDto>.generate(
            5,
            (int i) => _request(id: 'req-2$i', reason: 'Pengajuan lanjutan $i'),
          ),
          total: 25,
          offset: 20,
        ),
        offset: 20,
      );
      await pumpInbox(tester);
      await tester.pumpAndSettle();

      await tester.fling(find.byType(ListView), const Offset(0, -2400), 3000);
      await tester.pumpAndSettle();

      verify(
        () => repository.list(
          filter: ApprovalStatusFilter.pending,
          offset: 20,
          limit: any(named: 'limit'),
        ),
      ).called(1);

      await tester.fling(find.byType(ListView), const Offset(0, -2400), 3000);
      await tester.pumpAndSettle();
      expect(find.text('Pengajuan lanjutan 4'), findsOneWidget);
    });

    testWidgets('pull-to-refresh memuat ulang daftar', (
      WidgetTester tester,
    ) async {
      stubList(
        ApprovalStatusFilter.pending,
        _page(<RequestDto>[_request(id: 'req-1', reason: 'Sebelum refresh')]),
      );
      await pumpInbox(tester);
      await tester.pumpAndSettle();

      stubList(
        ApprovalStatusFilter.pending,
        _page(<RequestDto>[_request(id: 'req-2', reason: 'Sesudah refresh')]),
      );
      await tester.drag(find.byType(ListView), const Offset(0, 400));
      await tester.pump();
      await tester.pump(const Duration(seconds: 1));
      await tester.pumpAndSettle();

      expect(find.text('Sesudah refresh'), findsOneWidget);
      expect(find.text('Sebelum refresh'), findsNothing);
    });
  });

  group('navigasi (router penuh)', () {
    testWidgets('tab Approval: badge inbox count; tap kartu membuka detail', (
      WidgetTester tester,
    ) async {
      stubList(
        ApprovalStatusFilter.pending,
        _page(<RequestDto>[
          _request(id: 'req-1', reason: 'Registrasi 12 Laptop'),
        ]),
      );
      when(() => repository.inboxCount()).thenAnswer((_) async => 17);
      when(
        () => repository.detail('req-1'),
      ).thenAnswer((_) async => throw const NetworkFailure());

      final ProviderContainer container = ProviderContainer.test(
        overrides: [
          authControllerProvider.overrideWith(
            () => FakeAuthController(
              initialSession: const Authenticated(fakeUser),
            ),
          ),
          scanCameraFactoryProvider.overrideWithValue(FakeScanCamera.new),
          approvalRepositoryProvider.overrideWithValue(repository),
          referenceLookupRepositoryProvider.overrideWithValue(
            FakeReferenceLookup(),
          ),
          clockProvider.overrideWithValue(() => _frozenNow),
        ],
      );
      await tester.pumpWidget(RouterTestApp(container: container));
      await tester.pumpAndSettle();

      // Badge tab Approval dari GET /requests/inbox/count — di tab shell dan
      // quick action Approval Beranda (Task 11).
      expect(find.text('17'), findsNWidgets(2));

      await tester.tap(
        find.byWidgetPredicate(
          (Widget w) =>
              w is Text &&
              w.data == l10nId.shellTabApproval &&
              w.style?.fontSize == 10.5,
        ),
      );
      await tester.pumpAndSettle();

      // Layar inbox tampil (chip filter hanya ada di inbox; judul AppBar
      // "Approval" bertabrakan teks dengan label tab).
      expect(find.text(l10nId.approvalInboxFilterApproved), findsOneWidget);

      await tester.tap(find.text('Registrasi 12 Laptop'));
      await tester.pumpAndSettle();

      // Detail terbuka pada navigator root (error network dirender sopan).
      expect(find.text(l10nId.approvalDetailTitle), findsOneWidget);
      expect(find.text(l10nId.approvalDetailErrorTitle), findsOneWidget);
    });
  });
}
