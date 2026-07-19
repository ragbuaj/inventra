import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:inventra_mobile/core/api/app_failure.dart';
import 'package:inventra_mobile/core/auth/auth_controller.dart';
import 'package:inventra_mobile/core/auth/auth_session.dart';
import 'package:inventra_mobile/core/masterdata/reference_lookup_repository.dart';
import 'package:inventra_mobile/core/utils/clock.dart';
import 'package:inventra_mobile/core/widgets/confirm_dialog.dart';
import 'package:inventra_mobile/features/approval/data/approval_repository.dart';
import 'package:inventra_mobile/features/approval/data/request_detail_dto.dart';
import 'package:inventra_mobile/features/approval/data/request_dto.dart';
import 'package:inventra_mobile/features/approval/data/request_step_dto.dart';
import 'package:inventra_mobile/features/approval/presentation/approval_detail_screen.dart';
import 'package:mocktail/mocktail.dart';

import '../../../helpers/fake_auth_controller.dart';
import '../../../helpers/fake_reference_lookup.dart';
import '../../../helpers/test_app.dart';

class _MockApprovalRepository extends Mock implements ApprovalRepository {}

const String _requestId = 'req-1';

/// Detail mutasi menunggu keputusan (maker BUKAN pengguna tes `user-1`).
const RequestDetailDto _pendingTransfer = RequestDetailDto(
  id: _requestId,
  type: 'asset_transfer',
  status: 'pending',
  amount: '18750000.00',
  currentStep: 2,
  officeId: 'office-jaksel',
  targetId: 'asset-1',
  targetEntity: 'assets',
  reason: 'Mutasi Laptop Dell Latitude 5440 ke KCP Kebayoran Baru',
  requestedById: 'user-maker',
  requestedByName: 'Dewi Lestari',
  requestedByRole: 'Staf Umum',
  officeName: 'Cabang Jakarta Selatan',
  createdAt: null,
  payload: <String, dynamic>{
    'from_office_id': 'office-jaksel',
    'to_office_id': 'office-kebbaru',
    'to_room_id': 'room-layanan',
    'condition_sent': 'Baik, siap pakai',
    'transfer_date': '2026-08-01',
  },
  steps: <RequestStepDto>[
    RequestStepDto(
      stepOrder: 1,
      requiredLevel: 'office',
      approverName: 'Siti Rahayu',
      decision: 'approved',
      decidedAt: null,
    ),
    RequestStepDto(
      stepOrder: 2,
      requiredLevel: 'wilayah',
      approverName: 'Hendra Gunawan',
      decision: 'pending',
    ),
  ],
);

const Map<String, String> _resolvedNames = <String, String>{
  'asset:asset-1': 'Laptop Dell Latitude 5440 · JKT01-ELK-2026-00001',
  'office:office-jaksel': 'Cabang Jakarta Selatan',
  'office:office-kebbaru': 'KCP Kebayoran Baru',
  'room:room-layanan': 'Lantai 1 · R. Layanan',
};

const ApprovalDetailData _pendingData = ApprovalDetailData(
  request: _pendingTransfer,
  maskedFields: <String>{},
);

RequestDto _decidedResponse(String status) => RequestDto(
  id: _requestId,
  type: 'asset_transfer',
  status: status,
  currentStep: 2,
  requestedById: 'user-maker',
);

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
          FakeReferenceLookup(_resolvedNames),
        ),
        authControllerProvider.overrideWith(
          () =>
              FakeAuthController(initialSession: const Authenticated(fakeUser)),
        ),
        clockProvider.overrideWithValue(() => DateTime.utc(2026, 7, 19, 9)),
      ],
    );
  }

  /// Detail didorong di atas layar peluncur supaya pop setelah keputusan bisa
  /// diverifikasi (pola navigasi produksi: detail di atas inbox).
  Future<void> pumpDetail(WidgetTester tester, {bool pushed = false}) async {
    tester.view.physicalSize = const Size(500, 1800);
    tester.view.devicePixelRatio = 1.0;
    addTearDown(tester.view.reset);
    final ProviderContainer container = createContainer();
    if (!pushed) {
      await tester.pumpWidget(
        buildScreenHarness(
          container: container,
          child: const ApprovalDetailScreen(requestId: _requestId),
        ),
      );
      return;
    }
    await tester.pumpWidget(
      buildScreenHarness(
        container: container,
        child: Builder(
          builder: (BuildContext context) => Scaffold(
            body: Center(
              child: ElevatedButton(
                onPressed: () => Navigator.of(context).push(
                  MaterialPageRoute<void>(
                    builder: (BuildContext context) =>
                        const ApprovalDetailScreen(requestId: _requestId),
                  ),
                ),
                child: const Text('buka detail'),
              ),
            ),
          ),
        ),
      ),
    );
    await tester.tap(find.text('buka detail'));
    await tester.pumpAndSettle();
  }

  group('state data', () {
    testWidgets(
      'header + data payload ter-resolve NAMA (tanpa UUID) + jenjang',
      (WidgetTester tester) async {
        when(
          () => repository.detail(_requestId),
        ).thenAnswer((_) async => _pendingData);
        await pumpDetail(tester);
        await tester.pumpAndSettle();

        expect(find.text(l10nId.approvalDetailTitle), findsOneWidget);
        expect(find.text(l10nId.approvalTypeAssetTransfer), findsOneWidget);
        expect(
          find.text('Mutasi Laptop Dell Latitude 5440 ke KCP Kebayoran Baru'),
          findsWidgets,
        );
        expect(find.text('Dewi Lestari'), findsWidgets);
        expect(find.text('Staf Umum · Cabang Jakarta Selatan'), findsOneWidget);
        // Payload mutasi: aset target + perubahan kantor + ruangan tujuan.
        expect(
          find.text('Laptop Dell Latitude 5440 · JKT01-ELK-2026-00001'),
          findsOneWidget,
        );
        expect(find.text('Cabang Jakarta Selatan'), findsWidgets);
        expect(find.text('KCP Kebayoran Baru'), findsOneWidget);
        expect(find.text('Lantai 1 · R. Layanan'), findsOneWidget);
        expect(find.text('Baik, siap pakai'), findsOneWidget);
        // Nominal request diformat rupiah.
        expect(find.text('Rp 18.750.000'), findsOneWidget);
        // UUID mentah tidak pernah tampil.
        expect(find.textContaining('office-kebbaru'), findsNothing);
        expect(find.textContaining('asset-1'), findsNothing);
        // Jenjang: maker + dua approver, tahap aktif "Menunggu keputusan"
        // (judul baris timeline adalah Text.rich: nama + peran dalam span).
        expect(
          find.textContaining(
            l10nId.approvalDetailStepMaker,
            findRichText: true,
          ),
          findsOneWidget,
        );
        expect(
          find.textContaining('Siti Rahayu', findRichText: true),
          findsOneWidget,
        );
        expect(
          find.textContaining('Hendra Gunawan', findRichText: true),
          findsOneWidget,
        );
        expect(find.text(l10nId.approvalDetailStepWaiting), findsOneWidget);
        // Aksi tersedia (pengguna bukan maker, status pending).
        expect(find.text(l10nId.approvalDetailApprove), findsOneWidget);
        expect(find.text(l10nId.approvalDetailReject), findsOneWidget);
      },
    );

    testWidgets('payload dimask field permission: penanda dibatasi', (
      WidgetTester tester,
    ) async {
      const ApprovalDetailData masked = ApprovalDetailData(
        request: RequestDetailDto(
          id: _requestId,
          type: 'asset_disposal',
          status: 'pending',
          currentStep: 1,
          requestedById: 'user-maker',
          requestedByName: 'Rudi Hartono',
        ),
        maskedFields: <String>{'amount', 'payload', 'reason'},
      );
      when(() => repository.detail(_requestId)).thenAnswer((_) async => masked);
      await pumpDetail(tester);
      await tester.pumpAndSettle();

      expect(find.text(l10nId.approvalDetailRestrictedData), findsOneWidget);
      // Jenis sensitif menampilkan banner peringatan + penanda.
      expect(find.text(l10nId.approvalDetailSensitiveBanner), findsOneWidget);
      expect(find.text(l10nId.approvalCardSensitive), findsOneWidget);
    });
  });

  group('aksi approve', () {
    testWidgets(
      'Setujui -> ConfirmDialog -> panggil API -> refresh inbox + pop',
      (WidgetTester tester) async {
        when(
          () => repository.detail(_requestId),
        ).thenAnswer((_) async => _pendingData);
        when(
          () => repository.approve(_requestId, note: any(named: 'note')),
        ).thenAnswer((_) async => _decidedResponse('approved'));
        await pumpDetail(tester, pushed: true);

        await tester.tap(find.text(l10nId.approvalDetailApprove));
        await tester.pumpAndSettle();

        expect(find.byType(ConfirmDialog), findsOneWidget);
        expect(
          find.text(l10nId.approvalDetailApproveConfirmTitle),
          findsOneWidget,
        );

        await tester.tap(find.text(l10nId.approvalDetailApproveConfirmAction));
        await tester.pumpAndSettle();

        verify(() => repository.approve(_requestId, note: '')).called(1);
        // Kembali ke layar sebelumnya + SnackBar sukses.
        expect(find.byType(ApprovalDetailScreen), findsNothing);
        expect(find.text(l10nId.approvalDetailApprovedSnack), findsOneWidget);
      },
    );

    testWidgets('batal pada dialog: API tidak dipanggil', (
      WidgetTester tester,
    ) async {
      when(
        () => repository.detail(_requestId),
      ).thenAnswer((_) async => _pendingData);
      await pumpDetail(tester);
      await tester.pumpAndSettle();

      await tester.tap(find.text(l10nId.approvalDetailApprove));
      await tester.pumpAndSettle();
      await tester.tap(find.text(l10nId.commonCancel));
      await tester.pumpAndSettle();

      verifyNever(() => repository.approve(any(), note: any(named: 'note')));
      expect(find.byType(ApprovalDetailScreen), findsOneWidget);
    });

    testWidgets('403 SoD dari server: pesan i18n sopan + tetap di layar', (
      WidgetTester tester,
    ) async {
      when(
        () => repository.detail(_requestId),
      ).thenAnswer((_) async => _pendingData);
      when(
        () => repository.approve(_requestId, note: any(named: 'note')),
      ).thenThrow(const ForbiddenFailure());
      await pumpDetail(tester, pushed: true);

      await tester.tap(find.text(l10nId.approvalDetailApprove));
      await tester.pumpAndSettle();
      await tester.tap(find.text(l10nId.approvalDetailApproveConfirmAction));
      await tester.pumpAndSettle();

      expect(find.text(l10nId.approvalDetailErrorSod), findsOneWidget);
      expect(find.byType(ApprovalDetailScreen), findsOneWidget);
      // Detail dimuat ulang (eligibility berubah di server).
      verify(() => repository.detail(_requestId)).called(2);
    });

    testWidgets('409 (sudah diputus di tempat lain): pesan konflik + reload', (
      WidgetTester tester,
    ) async {
      when(
        () => repository.detail(_requestId),
      ).thenAnswer((_) async => _pendingData);
      when(
        () => repository.approve(_requestId, note: any(named: 'note')),
      ).thenThrow(const ConflictFailure());
      await pumpDetail(tester, pushed: true);

      await tester.tap(find.text(l10nId.approvalDetailApprove));
      await tester.pumpAndSettle();
      await tester.tap(find.text(l10nId.approvalDetailApproveConfirmAction));
      await tester.pumpAndSettle();

      expect(find.text(l10nId.approvalDetailErrorConflict), findsOneWidget);
      verify(() => repository.detail(_requestId)).called(2);
    });
  });

  group('aksi reject', () {
    testWidgets(
      'catatan diketik -> dialog tolak mengutip catatan -> API menerima note',
      (WidgetTester tester) async {
        when(
          () => repository.detail(_requestId),
        ).thenAnswer((_) async => _pendingData);
        when(
          () => repository.reject(_requestId, note: any(named: 'note')),
        ).thenAnswer((_) async => _decidedResponse('rejected'));
        await pumpDetail(tester, pushed: true);

        await tester.enterText(
          find.byType(TextField),
          'Unit tujuan belum siap menerima',
        );
        await tester.tap(find.text(l10nId.approvalDetailReject));
        await tester.pumpAndSettle();

        expect(
          find.text(l10nId.approvalDetailRejectConfirmTitle),
          findsOneWidget,
        );
        expect(find.text('"Unit tujuan belum siap menerima"'), findsOneWidget);

        await tester.tap(find.text(l10nId.approvalDetailRejectConfirmAction));
        await tester.pumpAndSettle();

        verify(
          () => repository.reject(
            _requestId,
            note: 'Unit tujuan belum siap menerima',
          ),
        ).called(1);
        expect(find.byType(ApprovalDetailScreen), findsNothing);
        expect(find.text(l10nId.approvalDetailRejectedSnack), findsOneWidget);
      },
    );

    testWidgets('tanpa catatan: dialog tanpa kutipan, API note kosong', (
      WidgetTester tester,
    ) async {
      when(
        () => repository.detail(_requestId),
      ).thenAnswer((_) async => _pendingData);
      when(
        () => repository.reject(_requestId, note: any(named: 'note')),
      ).thenAnswer((_) async => _decidedResponse('rejected'));
      await pumpDetail(tester, pushed: true);

      await tester.tap(find.text(l10nId.approvalDetailReject));
      await tester.pumpAndSettle();

      expect(find.text(l10nId.approvalDetailYourNote), findsNothing);

      await tester.tap(find.text(l10nId.approvalDetailRejectConfirmAction));
      await tester.pumpAndSettle();

      verify(() => repository.reject(_requestId, note: '')).called(1);
    });
  });

  group('guard SoD dan status dari respons detail', () {
    testWidgets('pengguna adalah maker: aksi diganti banner SoD', (
      WidgetTester tester,
    ) async {
      const ApprovalDetailData ownRequest = ApprovalDetailData(
        request: RequestDetailDto(
          id: _requestId,
          type: 'asset_transfer',
          status: 'pending',
          currentStep: 1,
          // Sama dengan fakeUser.id — field kontrak requested_by_id.
          requestedById: 'user-1',
          requestedByName: 'Budi Santoso',
        ),
        maskedFields: <String>{},
      );
      when(
        () => repository.detail(_requestId),
      ).thenAnswer((_) async => ownRequest);
      await pumpDetail(tester);
      await tester.pumpAndSettle();

      expect(find.text(l10nId.approvalDetailSodOwnRequest), findsOneWidget);
      expect(find.text(l10nId.approvalDetailApprove), findsNothing);
      expect(find.text(l10nId.approvalDetailReject), findsNothing);
      expect(find.byType(TextField), findsNothing);
    });

    testWidgets(
      'status approved oleh pengguna: banner "Anda telah menyetujui"',
      (WidgetTester tester) async {
        const ApprovalDetailData decided = ApprovalDetailData(
          request: RequestDetailDto(
            id: _requestId,
            type: 'asset_transfer',
            status: 'approved',
            currentStep: 2,
            requestedById: 'user-maker',
            requestedByName: 'Dewi Lestari',
            decidedById: 'user-1',
            decisionNote: 'Setuju, pastikan serah terima didokumentasikan.',
          ),
          maskedFields: <String>{},
        );
        when(
          () => repository.detail(_requestId),
        ).thenAnswer((_) async => decided);
        await pumpDetail(tester);
        await tester.pumpAndSettle();

        expect(
          find.text(l10nId.approvalDetailDecidedByYouApproved),
          findsOneWidget,
        );
        expect(find.text(l10nId.approvalDetailApprove), findsNothing);
        // Catatan keputusan tampil sebagai kutipan.
        expect(
          find.text('"Setuju, pastikan serah terima didokumentasikan."'),
          findsOneWidget,
        );
      },
    );

    testWidgets('status rejected oleh orang lain: banner generik', (
      WidgetTester tester,
    ) async {
      const ApprovalDetailData decided = ApprovalDetailData(
        request: RequestDetailDto(
          id: _requestId,
          type: 'asset_transfer',
          status: 'rejected',
          currentStep: 1,
          requestedById: 'user-maker',
          decidedById: 'user-lain',
        ),
        maskedFields: <String>{},
      );
      when(
        () => repository.detail(_requestId),
      ).thenAnswer((_) async => decided);
      await pumpDetail(tester);
      await tester.pumpAndSettle();

      expect(find.text(l10nId.approvalDetailDecidedRejected), findsOneWidget);
      expect(find.text(l10nId.approvalDetailReject), findsNothing);
    });
  });

  group('state loading dan error', () {
    testWidgets('loading: skeleton tanpa aksi', (WidgetTester tester) async {
      when(() => repository.detail(_requestId)).thenAnswer((_) async {
        await Future<void>.delayed(const Duration(milliseconds: 50));
        return _pendingData;
      });
      await pumpDetail(tester);
      await tester.pump();

      expect(find.text(l10nId.approvalDetailApprove), findsNothing);
      await tester.pumpAndSettle();
      expect(find.text(l10nId.approvalDetailApprove), findsOneWidget);
    });

    testWidgets('404: pesan tidak ditemukan', (WidgetTester tester) async {
      when(
        () => repository.detail(_requestId),
      ).thenThrow(const NotFoundFailure());
      await pumpDetail(tester);
      await tester.pumpAndSettle();

      expect(find.text(l10nId.approvalDetailNotFoundTitle), findsOneWidget);
    });

    testWidgets('403 melihat detail: pesan akses dibatasi', (
      WidgetTester tester,
    ) async {
      when(
        () => repository.detail(_requestId),
      ).thenThrow(const ForbiddenFailure());
      await pumpDetail(tester);
      await tester.pumpAndSettle();

      expect(find.text(l10nId.approvalDetailForbiddenTitle), findsOneWidget);
    });

    testWidgets('offline: retry memuat ulang', (WidgetTester tester) async {
      when(
        () => repository.detail(_requestId),
      ).thenThrow(const NetworkFailure());
      await pumpDetail(tester);
      await tester.pumpAndSettle();

      expect(find.text(l10nId.approvalDetailErrorTitle), findsOneWidget);

      when(
        () => repository.detail(_requestId),
      ).thenAnswer((_) async => _pendingData);
      await tester.tap(find.text(l10nId.commonRetry));
      await tester.pumpAndSettle();

      expect(find.text(l10nId.approvalDetailApprove), findsOneWidget);
    });
  });
}
