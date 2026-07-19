@Tags(<String>['golden'])
library;

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:inventra_mobile/app/theme.dart';
import 'package:inventra_mobile/core/auth/auth_controller.dart';
import 'package:inventra_mobile/core/auth/auth_session.dart';
import 'package:inventra_mobile/core/auth/data/user_dto.dart';
import 'package:inventra_mobile/core/connectivity/connectivity_provider.dart';
import 'package:inventra_mobile/core/i18n/gen/app_localizations.dart';
import 'package:inventra_mobile/core/masterdata/reference_lookup_repository.dart';
import 'package:inventra_mobile/core/utils/clock.dart';
import 'package:inventra_mobile/features/approval/data/approval_repository.dart';
import 'package:inventra_mobile/features/approval/data/request_dto.dart';
import 'package:inventra_mobile/features/approval/data/request_list_dto.dart';
import 'package:inventra_mobile/features/home/presentation/home_screen.dart';
import 'package:inventra_mobile/features/notifications/data/notification_dto.dart';
import 'package:inventra_mobile/features/notifications/data/notifications_repository.dart';
import 'package:inventra_mobile/features/stock_opname/data/stock_opname_repository.dart';
import 'package:inventra_mobile/features/stock_opname/data/stock_opname_session_dto.dart';
import 'package:mocktail/mocktail.dart';

import '../helpers/fake_auth_controller.dart';
import '../helpers/fake_notifications_repository.dart';
import '../helpers/fake_reference_lookup.dart';
import '../helpers/fake_stock_opname_repository.dart';
import '../helpers/golden_fonts.dart';

class _MockApprovalRepository extends Mock implements ApprovalRepository {}

/// Waktu beku paritas mockup (09.41).
final DateTime _frozenNow = DateTime(2026, 7, 19, 9, 41);

/// Data paritas mockup "Data penuh — Manager aset": sesi opname 85%,
/// 12 approval menunggu (2 teratas), unread notifikasi 3.
const UserDto _goldenUser = UserDto(
  id: 'user-1',
  name: 'Andi Pratama',
  email: 'andi.pratama@bank.co.id',
  roleId: 'role-1',
  officeId: 'office-1',
  status: 'active',
  googleLinked: false,
);

final StockOpnameSessionDto _goldenSession = StockOpnameSessionDto(
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

final RequestListDto _goldenPending = RequestListDto(
  data: <RequestDto>[
    RequestDto(
      id: 'req-1',
      type: 'assignment',
      status: 'pending',
      currentStep: 1,
      reason: 'Proyektor Epson EB-X500',
      requestedById: 'user-2',
      requestedByName: 'Dewi Lestari',
      createdAt: DateTime(2026, 7, 19, 7, 41),
    ),
    RequestDto(
      id: 'req-2',
      type: 'asset_disposal',
      status: 'pending',
      currentStep: 1,
      reason: '3 unit PC Desktop Lenovo',
      requestedById: 'user-3',
      requestedByName: 'Rudi Hartono',
      createdAt: DateTime(2026, 7, 14, 9),
    ),
  ],
  total: 12,
  limit: 20,
  offset: 0,
);

List<NotificationDto> _unreadFeed() => List<NotificationDto>.generate(
  3,
  (int i) => NotificationDto(
    id: 'n-$i',
    type: 'approval_pending',
    params: const <String, dynamic>{'request_type': 'assignment', 'step': '1'},
    createdAt: DateTime(2026, 7, 19, 9, i),
  ),
);

/// Golden Beranda light + dark (header sapaan, kartu opname aktif, kartu
/// approval menunggu, quick actions berbadge). Digenerate dan diverifikasi
/// lokal (Windows): `flutter test --update-goldens --tags golden`.
void main() {
  setUpAll(loadAppFonts);

  Widget buildScreen(ThemeData theme) {
    final _MockApprovalRepository approvalRepository =
        _MockApprovalRepository();
    when(
      () => approvalRepository.list(
        filter: ApprovalStatusFilter.pending,
        offset: any(named: 'offset'),
        limit: any(named: 'limit'),
      ),
    ).thenAnswer((_) async => _goldenPending);
    when(() => approvalRepository.inboxCount()).thenAnswer((_) async => 12);

    return ProviderScope(
      overrides: [
        authControllerProvider.overrideWith(
          () => FakeAuthController(
            initialSession: const Authenticated(_goldenUser),
          ),
        ),
        stockOpnameRepositoryProvider.overrideWithValue(
          FakeStockOpnameRepository(
            sessionsData: <StockOpnameSessionDto>[_goldenSession],
          ),
        ),
        approvalRepositoryProvider.overrideWithValue(approvalRepository),
        notificationsRepositoryProvider.overrideWithValue(
          FakeNotificationsRepository(feed: _unreadFeed()),
        ),
        referenceLookupRepositoryProvider.overrideWithValue(
          FakeReferenceLookup(<String, String>{
            'office:office-1': 'Cabang Jakarta Selatan',
          }),
        ),
        isOnlineProvider.overrideWith((Ref ref) => Stream<bool>.value(true)),
        clockProvider.overrideWithValue(() => _frozenNow),
      ],
      child: MaterialApp(
        theme: theme,
        locale: const Locale('id'),
        localizationsDelegates: AppLocalizations.localizationsDelegates,
        supportedLocales: const <Locale>[Locale('id'), Locale('en')],
        home: const HomeScreen(),
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

  testWidgets('beranda light', (WidgetTester tester) async {
    await pumpAtPhoneSize(tester, buildScreen(InventraTheme.light));
    await expectLater(
      find.byType(HomeScreen),
      matchesGoldenFile('home_light.png'),
    );
  });

  testWidgets('beranda dark', (WidgetTester tester) async {
    await pumpAtPhoneSize(tester, buildScreen(InventraTheme.dark));
    await expectLater(
      find.byType(HomeScreen),
      matchesGoldenFile('home_dark.png'),
    );
  });
}
