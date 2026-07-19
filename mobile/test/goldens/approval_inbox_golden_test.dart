@Tags(<String>['golden'])
library;

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:inventra_mobile/app/theme.dart';
import 'package:inventra_mobile/core/i18n/gen/app_localizations.dart';
import 'package:inventra_mobile/core/utils/clock.dart';
import 'package:inventra_mobile/features/approval/data/approval_repository.dart';
import 'package:inventra_mobile/features/approval/data/request_dto.dart';
import 'package:inventra_mobile/features/approval/data/request_list_dto.dart';
import 'package:inventra_mobile/features/approval/presentation/approval_inbox_screen.dart';
import 'package:mocktail/mocktail.dart';

import '../helpers/golden_fonts.dart';

class _MockApprovalRepository extends Mock implements ApprovalRepository {}

final DateTime _frozenNow = DateTime.utc(2026, 7, 19, 9);

/// Empat kartu variasi mockup "Daftar Menunggu terisi": registrasi + nominal,
/// penghapusan sensitif + nominal, mutasi tanpa nominal, peminjaman tanpa
/// nominal.
final List<RequestDto> _goldenItems = <RequestDto>[
  RequestDto(
    id: 'req-1',
    type: 'asset_create',
    status: 'pending',
    amount: '154800000.00',
    currentStep: 1,
    reason: 'Registrasi 12 Laptop Asus ExpertBook',
    requestedById: 'user-dewi',
    requestedByName: 'Dewi Lestari',
    officeName: 'Cabang Jakarta Selatan',
    createdAt: _frozenNow.subtract(const Duration(hours: 2)),
  ),
  RequestDto(
    id: 'req-2',
    type: 'asset_disposal',
    status: 'pending',
    amount: '27450000.00',
    currentStep: 1,
    reason: 'Penghapusan 3 PC Desktop Lenovo M70q',
    requestedById: 'user-rudi',
    requestedByName: 'Rudi Hartono',
    officeName: 'Cabang Jakarta Selatan',
    createdAt: _frozenNow.subtract(const Duration(hours: 5)),
  ),
  RequestDto(
    id: 'req-3',
    type: 'asset_transfer',
    status: 'pending',
    currentStep: 1,
    reason: 'Mutasi 12 Kursi Kerja ke Lantai 5',
    requestedById: 'user-andi',
    requestedByName: 'Andi Prasetyo',
    officeName: 'Cabang Jakarta Selatan',
    createdAt: _frozenNow.subtract(const Duration(hours: 26)),
  ),
  RequestDto(
    id: 'req-4',
    type: 'assignment',
    status: 'pending',
    currentStep: 1,
    reason: 'Peminjaman Proyektor Epson EB-X500',
    requestedById: 'user-sari',
    requestedByName: 'Sari Wulandari',
    officeName: 'KCP Kebayoran Baru',
    createdAt: _frozenNow.subtract(const Duration(hours: 30)),
  ),
];

/// Golden Inbox Approval light + dark (filter Menunggu terisi). Digenerate dan
/// diverifikasi lokal (Windows): `flutter test --update-goldens --tags golden`.
void main() {
  setUpAll(loadAppFonts);

  Widget buildInbox(ThemeData theme) {
    final _MockApprovalRepository repository = _MockApprovalRepository();
    when(
      () => repository.list(
        filter: ApprovalStatusFilter.pending,
        offset: 0,
        limit: any(named: 'limit'),
      ),
    ).thenAnswer(
      (_) async =>
          RequestListDto(data: _goldenItems, total: 17, limit: 20, offset: 0),
    );

    return ProviderScope(
      overrides: [
        approvalRepositoryProvider.overrideWithValue(repository),
        clockProvider.overrideWithValue(() => _frozenNow),
      ],
      child: MaterialApp(
        theme: theme,
        locale: const Locale('id'),
        localizationsDelegates: AppLocalizations.localizationsDelegates,
        supportedLocales: const <Locale>[Locale('id'), Locale('en')],
        home: const ApprovalInboxScreen(),
      ),
    );
  }

  Future<void> pumpAtPhoneSize(WidgetTester tester, Widget widget) async {
    tester.view.physicalSize = const Size(390, 844);
    tester.view.devicePixelRatio = 1.0;
    addTearDown(tester.view.reset);
    await tester.pumpWidget(widget);
    await tester.pumpAndSettle();
  }

  testWidgets('inbox approval menunggu light', (WidgetTester tester) async {
    await pumpAtPhoneSize(tester, buildInbox(InventraTheme.light));
    await expectLater(
      find.byType(ApprovalInboxScreen),
      matchesGoldenFile('approval_inbox_light.png'),
    );
  });

  testWidgets('inbox approval menunggu dark', (WidgetTester tester) async {
    await pumpAtPhoneSize(tester, buildInbox(InventraTheme.dark));
    await expectLater(
      find.byType(ApprovalInboxScreen),
      matchesGoldenFile('approval_inbox_dark.png'),
    );
  });
}
