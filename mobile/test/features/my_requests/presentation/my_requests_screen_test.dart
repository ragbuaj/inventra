import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:inventra_mobile/core/api/app_failure.dart';
import 'package:inventra_mobile/core/utils/clock.dart';
import 'package:inventra_mobile/core/widgets/app_skeleton.dart';
import 'package:inventra_mobile/features/approval/data/approval_repository.dart'
    show ApprovalStatusFilter;
import 'package:inventra_mobile/features/approval/data/request_dto.dart';
import 'package:inventra_mobile/features/approval/data/request_list_dto.dart';
import 'package:inventra_mobile/features/my_requests/data/my_requests_repository.dart';
import 'package:inventra_mobile/features/my_requests/presentation/my_requests_screen.dart';
import 'package:mocktail/mocktail.dart';

import '../../../helpers/test_app.dart';

class _MockMyRequestsRepository extends Mock implements MyRequestsRepository {}

final DateTime _frozenNow = DateTime.utc(2026, 7, 20, 11);

RequestDto _request({
  String id = 'req-1',
  String type = 'assignment',
  String status = 'pending',
  String? amount,
  String? reason = 'Peminjaman Proyektor Epson',
}) {
  return RequestDto(
    id: id,
    type: type,
    status: status,
    amount: amount,
    currentStep: 1,
    reason: reason,
    requestedById: 'user-me',
    createdAt: _frozenNow.subtract(const Duration(hours: 2)),
  );
}

RequestListDto _page(List<RequestDto> items) {
  return RequestListDto(
    data: items,
    total: items.length,
    limit: 20,
    offset: 0,
  );
}

void main() {
  late _MockMyRequestsRepository repository;

  setUp(() {
    repository = _MockMyRequestsRepository();
  });

  ProviderContainer createContainer() {
    return ProviderContainer.test(
      overrides: [
        myRequestsRepositoryProvider.overrideWithValue(repository),
        clockProvider.overrideWithValue(() => _frozenNow),
      ],
    );
  }

  void stubList(ApprovalStatusFilter filter, RequestListDto page) {
    when(
      () => repository.list(
        filter: filter,
        offset: 0,
        limit: any(named: 'limit'),
      ),
    ).thenAnswer((_) async => page);
  }

  Future<void> pump(WidgetTester tester) async {
    tester.view.physicalSize = const Size(500, 1600);
    tester.view.devicePixelRatio = 1.0;
    addTearDown(tester.view.reset);
    await tester.pumpWidget(
      buildScreenHarness(
        container: createContainer(),
        child: const MyRequestsScreen(),
      ),
    );
  }

  testWidgets('kartu: jenis + judul + nominal + chip status + Batalkan', (
    WidgetTester tester,
  ) async {
    stubList(
      ApprovalStatusFilter.pending,
      _page(<RequestDto>[
        _request(type: 'asset_create', amount: '154800000.00', reason: 'Registrasi 12 Laptop'),
      ]),
    );
    await pump(tester);
    await tester.pumpAndSettle();

    expect(find.text(l10nId.myRequestsTitle), findsOneWidget);
    expect(find.text(l10nId.approvalTypeAssetCreate), findsOneWidget);
    expect(find.text('Registrasi 12 Laptop'), findsOneWidget);
    expect(find.text('Rp 154.800.000'), findsOneWidget);
    expect(find.text(l10nId.approvalTimeHoursAgo(2)), findsOneWidget);
    // Pengajuan pending: tombol Batalkan tampil.
    expect(find.text(l10nId.myRequestsCancel), findsOneWidget);
  });

  testWidgets('pengajuan non-pending: tanpa tombol Batalkan', (
    WidgetTester tester,
  ) async {
    stubList(
      ApprovalStatusFilter.pending,
      _page(<RequestDto>[_request(status: 'approved')]),
    );
    await pump(tester);
    await tester.pumpAndSettle();

    expect(find.text(l10nId.myRequestsCancel), findsNothing);
  });

  testWidgets('filter: tap Disetujui memuat daftar approved', (
    WidgetTester tester,
  ) async {
    stubList(
      ApprovalStatusFilter.pending,
      _page(<RequestDto>[_request(reason: 'Pengajuan pending')]),
    );
    stubList(
      ApprovalStatusFilter.approved,
      _page(<RequestDto>[_request(id: 'req-9', status: 'approved', reason: 'Pengajuan disetujui')]),
    );
    await pump(tester);
    await tester.pumpAndSettle();

    await tester.tap(find.text(l10nId.approvalInboxFilterApproved));
    await tester.pumpAndSettle();

    expect(find.text('Pengajuan disetujui'), findsOneWidget);
    expect(find.text('Pengajuan pending'), findsNothing);
  });

  testWidgets('Batalkan: konfirmasi lalu batal, daftar dimuat ulang', (
    WidgetTester tester,
  ) async {
    List<RequestDto> pending = <RequestDto>[
      _request(reason: 'Pengajuan yang dibatalkan'),
    ];
    when(
      () => repository.list(
        filter: ApprovalStatusFilter.pending,
        offset: 0,
        limit: any(named: 'limit'),
      ),
    ).thenAnswer((_) async => _page(pending));
    when(() => repository.cancel('req-1')).thenAnswer((_) async {
      pending = <RequestDto>[];
      return _request(status: 'cancelled');
    });
    await pump(tester);
    await tester.pumpAndSettle();

    // Tombol Batalkan di kartu.
    await tester.tap(find.text(l10nId.myRequestsCancel));
    await tester.pumpAndSettle();

    // Dialog konfirmasi: tekan tombol utama (FilledButton) "Batalkan".
    await tester.tap(
      find.widgetWithText(FilledButton, l10nId.myRequestsCancel),
    );
    await tester.pumpAndSettle();

    verify(() => repository.cancel('req-1')).called(1);
    expect(find.text(l10nId.myRequestsCancelSuccess), findsOneWidget);
    // Daftar dimuat ulang tanpa item.
    expect(find.text('Pengajuan yang dibatalkan'), findsNothing);
  });

  testWidgets('Batalkan gagal: SnackBar error, item tetap ada', (
    WidgetTester tester,
  ) async {
    stubList(
      ApprovalStatusFilter.pending,
      _page(<RequestDto>[_request(reason: 'Pengajuan tetap ada')]),
    );
    when(() => repository.cancel('req-1')).thenThrow(const ConflictFailure());
    await pump(tester);
    await tester.pumpAndSettle();

    await tester.tap(find.text(l10nId.myRequestsCancel));
    await tester.pumpAndSettle();
    await tester.tap(find.widgetWithText(FilledButton, l10nId.myRequestsCancel));
    await tester.pumpAndSettle();

    verify(() => repository.cancel('req-1')).called(1);
    expect(find.text(l10nId.myRequestsCancelError), findsOneWidget);
    // Daftar tidak berubah (item masih ada).
    expect(find.text('Pengajuan tetap ada'), findsOneWidget);
  });

  testWidgets('Batalkan: batal dialog tidak memanggil cancel', (
    WidgetTester tester,
  ) async {
    stubList(
      ApprovalStatusFilter.pending,
      _page(<RequestDto>[_request()]),
    );
    await pump(tester);
    await tester.pumpAndSettle();

    await tester.tap(find.text(l10nId.myRequestsCancel));
    await tester.pumpAndSettle();
    await tester.tap(find.widgetWithText(OutlinedButton, l10nId.commonCancel));
    await tester.pumpAndSettle();

    verifyNever(() => repository.cancel(any()));
  });

  testWidgets('kosong: empty state', (WidgetTester tester) async {
    stubList(ApprovalStatusFilter.pending, _page(<RequestDto>[]));
    await pump(tester);
    await tester.pumpAndSettle();

    expect(find.text(l10nId.myRequestsEmptyTitle), findsOneWidget);
  });

  testWidgets('loading: skeleton', (WidgetTester tester) async {
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
    await pump(tester);
    await tester.pump();

    expect(find.byType(AppSkeleton), findsWidgets);
    await tester.pumpAndSettle();
  });

  testWidgets('offline: pesan jaringan + retry', (WidgetTester tester) async {
    when(
      () => repository.list(
        filter: ApprovalStatusFilter.pending,
        offset: any(named: 'offset'),
        limit: any(named: 'limit'),
      ),
    ).thenThrow(const NetworkFailure());
    await pump(tester);
    await tester.pumpAndSettle();

    expect(find.text(l10nId.myRequestsErrorTitle), findsOneWidget);
    expect(find.text(l10nId.myRequestsErrorNetworkBody), findsOneWidget);

    stubList(
      ApprovalStatusFilter.pending,
      _page(<RequestDto>[_request(reason: 'Setelah retry')]),
    );
    await tester.tap(find.text(l10nId.commonRetry));
    await tester.pumpAndSettle();

    expect(find.text('Setelah retry'), findsOneWidget);
  });

  testWidgets('403: pesan akses tanpa retry', (WidgetTester tester) async {
    when(
      () => repository.list(
        filter: ApprovalStatusFilter.pending,
        offset: any(named: 'offset'),
        limit: any(named: 'limit'),
      ),
    ).thenThrow(const ForbiddenFailure());
    await pump(tester);
    await tester.pumpAndSettle();

    expect(find.text(l10nId.myRequestsForbiddenTitle), findsOneWidget);
    expect(find.text(l10nId.commonRetry), findsNothing);
  });
}
