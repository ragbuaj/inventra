@Tags(<String>['golden'])
library;

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:inventra_mobile/app/theme.dart';
import 'package:inventra_mobile/core/i18n/gen/app_localizations.dart';
import 'package:inventra_mobile/core/utils/clock.dart';
import 'package:inventra_mobile/features/approval/data/approval_repository.dart'
    show ApprovalStatusFilter;
import 'package:inventra_mobile/features/approval/data/request_dto.dart';
import 'package:inventra_mobile/features/approval/data/request_list_dto.dart';
import 'package:inventra_mobile/features/my_requests/data/my_requests_repository.dart';
import 'package:inventra_mobile/features/my_requests/presentation/my_requests_screen.dart';
import 'package:mocktail/mocktail.dart';

import '../helpers/golden_fonts.dart';

class _MockMyRequestsRepository extends Mock implements MyRequestsRepository {}

final DateTime _frozenNow = DateTime.utc(2026, 7, 20, 11);

RequestDto _request({
  required String id,
  required String type,
  required String reason,
  String status = 'pending',
  String? amount,
  Duration age = const Duration(hours: 2),
}) {
  return RequestDto(
    id: id,
    type: type,
    status: status,
    amount: amount,
    currentStep: 1,
    reason: reason,
    requestedById: 'user-me',
    createdAt: _frozenNow.subtract(age),
  );
}

/// Tiga pengajuan pending (Batalkan tampil): registrasi + nominal, peminjaman,
/// lapor kerusakan.
final List<RequestDto> _goldenItems = <RequestDto>[
  _request(
    id: 'req-1',
    type: 'asset_create',
    amount: '154800000.00',
    reason: 'Registrasi 12 Laptop Asus ExpertBook',
  ),
  _request(
    id: 'req-2',
    type: 'assignment',
    reason: 'Peminjaman Proyektor Epson EB-X500',
    age: const Duration(hours: 5),
  ),
  _request(
    id: 'req-3',
    type: 'maintenance',
    reason: 'AC Ruang Server bunyi kasar',
    age: const Duration(hours: 26),
  ),
];

/// Golden Pengajuan Saya light + dark. Digenerate & diverifikasi lokal:
/// `flutter test --update-goldens --tags golden`.
void main() {
  setUpAll(loadAppFonts);

  Widget buildScreen(ThemeData theme) {
    final _MockMyRequestsRepository repository = _MockMyRequestsRepository();
    when(
      () => repository.list(
        filter: ApprovalStatusFilter.pending,
        offset: 0,
        limit: any(named: 'limit'),
      ),
    ).thenAnswer(
      (_) async =>
          RequestListDto(data: _goldenItems, total: 3, limit: 20, offset: 0),
    );

    return ProviderScope(
      overrides: [
        myRequestsRepositoryProvider.overrideWithValue(repository),
        clockProvider.overrideWithValue(() => _frozenNow),
      ],
      child: MaterialApp(
        theme: theme,
        locale: const Locale('id'),
        localizationsDelegates: AppLocalizations.localizationsDelegates,
        supportedLocales: const <Locale>[Locale('id'), Locale('en')],
        home: const MyRequestsScreen(),
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

  testWidgets('pengajuan saya light', (WidgetTester tester) async {
    await pumpAtPhoneSize(tester, buildScreen(InventraTheme.light));
    await expectLater(
      find.byType(MyRequestsScreen),
      matchesGoldenFile('my_requests_light.png'),
    );
  });

  testWidgets('pengajuan saya dark', (WidgetTester tester) async {
    await pumpAtPhoneSize(tester, buildScreen(InventraTheme.dark));
    await expectLater(
      find.byType(MyRequestsScreen),
      matchesGoldenFile('my_requests_dark.png'),
    );
  });
}
