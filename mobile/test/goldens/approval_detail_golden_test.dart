@Tags(<String>['golden'])
library;

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:inventra_mobile/app/theme.dart';
import 'package:inventra_mobile/core/auth/auth_controller.dart';
import 'package:inventra_mobile/core/auth/auth_session.dart';
import 'package:inventra_mobile/core/i18n/gen/app_localizations.dart';
import 'package:inventra_mobile/core/masterdata/reference_lookup_repository.dart';
import 'package:inventra_mobile/core/utils/clock.dart';
import 'package:inventra_mobile/features/approval/data/approval_repository.dart';
import 'package:inventra_mobile/features/approval/data/request_detail_dto.dart';
import 'package:inventra_mobile/features/approval/data/request_step_dto.dart';
import 'package:inventra_mobile/features/approval/presentation/approval_detail_screen.dart';
import 'package:mocktail/mocktail.dart';

import '../helpers/fake_auth_controller.dart';
import '../helpers/fake_reference_lookup.dart';
import '../helpers/golden_fonts.dart';

class _MockApprovalRepository extends Mock implements ApprovalRepository {}

const String _requestId = 'req-1';

/// Data variasi mockup "Menunggu keputusan — mutasi": payload mutasi lengkap
/// dengan jenjang tiga baris (maker selesai, tahap aktif, tahap berikutnya).
final ApprovalDetailData _goldenData = ApprovalDetailData(
  request: RequestDetailDto(
    id: _requestId,
    type: 'asset_transfer',
    status: 'pending',
    amount: '18750000.00',
    currentStep: 1,
    officeId: 'office-jaksel',
    targetId: 'asset-1',
    targetEntity: 'assets',
    reason: 'Mutasi Laptop Dell Latitude 5440 ke KCP Kebayoran Baru',
    requestedById: 'user-dewi',
    requestedByName: 'Dewi Lestari',
    requestedByRole: 'Staf Umum',
    officeName: 'Cabang Jakarta Selatan',
    createdAt: DateTime.utc(2026, 7, 18, 8, 12),
    payload: const <String, dynamic>{
      'from_office_id': 'office-jaksel',
      'to_office_id': 'office-kebbaru',
      'to_room_id': 'room-layanan',
      'reason': 'Kebutuhan perangkat teller baru di KCP Kebayoran Baru.',
    },
    steps: const <RequestStepDto>[
      RequestStepDto(
        stepOrder: 1,
        requiredLevel: 'office',
        approverName: 'Siti Rahayu',
        decision: 'pending',
      ),
      RequestStepDto(
        stepOrder: 2,
        requiredLevel: 'wilayah',
        approverName: 'Hendra Gunawan',
        decision: 'pending',
      ),
    ],
  ),
  maskedFields: const <String>{},
);

const Map<String, String> _goldenNames = <String, String>{
  'asset:asset-1': 'Laptop Dell Latitude 5440 · JKT01-ELK-2026-00001',
  'office:office-jaksel': 'Cabang Jakarta Selatan',
  'office:office-kebbaru': 'KCP Kebayoran Baru',
  'room:room-layanan': 'Lantai 1 · R. Layanan',
};

/// Golden Detail Approval light + dark (menunggu keputusan). Digenerate dan
/// diverifikasi lokal (Windows): `flutter test --update-goldens --tags golden`.
void main() {
  setUpAll(loadAppFonts);

  Widget buildDetail(ThemeData theme) {
    final _MockApprovalRepository repository = _MockApprovalRepository();
    when(
      () => repository.detail(_requestId),
    ).thenAnswer((_) async => _goldenData);

    return ProviderScope(
      overrides: [
        approvalRepositoryProvider.overrideWithValue(repository),
        referenceLookupRepositoryProvider.overrideWithValue(
          FakeReferenceLookup(_goldenNames),
        ),
        authControllerProvider.overrideWith(
          () =>
              FakeAuthController(initialSession: const Authenticated(fakeUser)),
        ),
        clockProvider.overrideWithValue(() => DateTime.utc(2026, 7, 19, 9)),
      ],
      child: MaterialApp(
        theme: theme,
        locale: const Locale('id'),
        localizationsDelegates: AppLocalizations.localizationsDelegates,
        supportedLocales: const <Locale>[Locale('id'), Locale('en')],
        home: const ApprovalDetailScreen(requestId: _requestId),
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

  testWidgets('detail approval menunggu light', (WidgetTester tester) async {
    await pumpAtPhoneSize(tester, buildDetail(InventraTheme.light));
    await expectLater(
      find.byType(ApprovalDetailScreen),
      matchesGoldenFile('approval_detail_light.png'),
    );
  });

  testWidgets('detail approval menunggu dark', (WidgetTester tester) async {
    await pumpAtPhoneSize(tester, buildDetail(InventraTheme.dark));
    await expectLater(
      find.byType(ApprovalDetailScreen),
      matchesGoldenFile('approval_detail_dark.png'),
    );
  });
}
