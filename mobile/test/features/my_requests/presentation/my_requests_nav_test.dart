import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:go_router/go_router.dart';
import 'package:inventra_mobile/app/theme.dart';
import 'package:inventra_mobile/core/api/app_failure.dart';
import 'package:inventra_mobile/core/auth/auth_controller.dart';
import 'package:inventra_mobile/core/auth/auth_session.dart';
import 'package:inventra_mobile/core/i18n/gen/app_localizations.dart';
import 'package:inventra_mobile/core/masterdata/reference_lookup_repository.dart';
import 'package:inventra_mobile/features/approval/data/approval_repository.dart';
import 'package:inventra_mobile/features/approval/data/request_dto.dart';
import 'package:inventra_mobile/features/approval/data/request_list_dto.dart';
import 'package:inventra_mobile/features/approval/presentation/approval_detail_screen.dart';
import 'package:inventra_mobile/features/my_requests/data/my_requests_repository.dart';
import 'package:inventra_mobile/features/my_requests/presentation/my_requests_screen.dart';
import 'package:mocktail/mocktail.dart';

import '../../../helpers/fake_auth_controller.dart';
import '../../../helpers/fake_reference_lookup.dart';
import '../../../helpers/test_app.dart';

class _MockMyRequestsRepository extends Mock implements MyRequestsRepository {}

class _MockApprovalRepository extends Mock implements ApprovalRepository {}

void main() {
  testWidgets('tap kartu membuka detail read-only pengajuan', (
    WidgetTester tester,
  ) async {
    tester.view.physicalSize = const Size(500, 1600);
    tester.view.devicePixelRatio = 1.0;
    addTearDown(tester.view.reset);

    final _MockMyRequestsRepository myRequests = _MockMyRequestsRepository();
    final _MockApprovalRepository approval = _MockApprovalRepository();

    when(
      () => myRequests.list(
        filter: ApprovalStatusFilter.pending,
        offset: 0,
        limit: any(named: 'limit'),
      ),
    ).thenAnswer(
      (_) async => const RequestListDto(
        data: <RequestDto>[
          RequestDto(
            id: 'req-1',
            type: 'assignment',
            status: 'pending',
            currentStep: 1,
            reason: 'Peminjaman Proyektor',
            requestedById: 'user-me',
          ),
        ],
        total: 1,
        limit: 20,
        offset: 0,
      ),
    );
    // Detail dibuka dari route reuse; error jaringan dirender sopan (cukup untuk
    // membuktikan navigasi sampai ke ApprovalDetailScreen).
    when(
      () => approval.detail('req-1'),
    ).thenThrow(const NetworkFailure());

    final GoRouter router = GoRouter(
      initialLocation: '/my-requests',
      routes: <RouteBase>[
        GoRoute(
          path: '/my-requests',
          builder: (BuildContext context, GoRouterState state) =>
              const MyRequestsScreen(),
          routes: <RouteBase>[
            GoRoute(
              path: ':id',
              builder: (BuildContext context, GoRouterState state) =>
                  ApprovalDetailScreen(requestId: state.pathParameters['id']!),
            ),
          ],
        ),
      ],
    );

    final ProviderContainer container = ProviderContainer.test(
      overrides: [
        myRequestsRepositoryProvider.overrideWithValue(myRequests),
        approvalRepositoryProvider.overrideWithValue(approval),
        referenceLookupRepositoryProvider.overrideWithValue(
          FakeReferenceLookup(),
        ),
        authControllerProvider.overrideWith(
          () =>
              FakeAuthController(initialSession: const Authenticated(fakeUser)),
        ),
      ],
    );

    await tester.pumpWidget(
      UncontrolledProviderScope(
        container: container,
        child: MaterialApp.router(
          routerConfig: router,
          theme: InventraTheme.light,
          locale: const Locale('id'),
          localizationsDelegates: AppLocalizations.localizationsDelegates,
          supportedLocales: const <Locale>[Locale('id'), Locale('en')],
        ),
      ),
    );
    await tester.pumpAndSettle();

    await tester.tap(find.text('Peminjaman Proyektor'));
    await tester.pumpAndSettle();

    // Layar Detail Approval terbuka (AppBar-nya), error jaringan dirender sopan.
    expect(find.text(l10nId.approvalDetailTitle), findsOneWidget);
    expect(find.text(l10nId.approvalDetailErrorTitle), findsOneWidget);
  });
}
