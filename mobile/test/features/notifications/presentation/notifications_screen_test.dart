import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:inventra_mobile/app/router.dart';
import 'package:inventra_mobile/core/api/app_failure.dart';
import 'package:inventra_mobile/core/auth/auth_controller.dart';
import 'package:inventra_mobile/core/auth/auth_session.dart';
import 'package:inventra_mobile/core/camera/scan_camera.dart';
import 'package:inventra_mobile/core/masterdata/reference_lookup_repository.dart';
import 'package:inventra_mobile/core/utils/clock.dart';
import 'package:inventra_mobile/core/widgets/app_skeleton.dart';
import 'package:inventra_mobile/features/approval/data/approval_repository.dart';
import 'package:inventra_mobile/features/notifications/data/notification_dto.dart';
import 'package:inventra_mobile/features/notifications/data/notification_list_dto.dart';
import 'package:inventra_mobile/features/notifications/data/notifications_repository.dart';
import 'package:inventra_mobile/features/notifications/presentation/notifications_screen.dart';
import 'package:inventra_mobile/features/stock_opname/data/stock_opname_repository.dart';
import 'package:mocktail/mocktail.dart';

import '../../../helpers/fake_auth_controller.dart';
import '../../../helpers/fake_reference_lookup.dart';
import '../../../helpers/fake_scan_camera.dart';
import '../../../helpers/fake_stock_opname_repository.dart';
import '../../../helpers/test_app.dart';

class _MockNotificationsRepository extends Mock
    implements NotificationsRepository {}

class _MockApprovalRepository extends Mock implements ApprovalRepository {}

/// Waktu beku lokal: sejajar dengan data mockup (09.41).
final DateTime _frozenNow = DateTime(2026, 7, 19, 9, 41);

NotificationDto _notification({
  required String id,
  String type = 'approval_pending',
  Map<String, dynamic> params = const <String, dynamic>{},
  String? entityType,
  String? entityId,
  DateTime? readAt,
  DateTime? createdAt,
}) {
  return NotificationDto(
    id: id,
    type: type,
    params: params,
    entityType: entityType,
    entityId: entityId,
    readAt: readAt,
    createdAt: createdAt ?? _frozenNow.subtract(const Duration(minutes: 10)),
  );
}

NotificationListDto _page(
  List<NotificationDto> items, {
  int? total,
  int offset = 0,
}) {
  return NotificationListDto(
    data: items,
    total: total ?? items.length,
    limit: 20,
    offset: offset,
  );
}

Finder _unreadDot(String id) =>
    find.byKey(ValueKey<String>('notification-unread-$id'));

void main() {
  late _MockNotificationsRepository repository;

  setUp(() {
    repository = _MockNotificationsRepository();
    when(() => repository.unreadCount()).thenAnswer((_) async => 0);
  });

  void stubList(NotificationListDto page, {int offset = 0}) {
    when(
      () => repository.list(
        read: any(named: 'read'),
        offset: offset,
        limit: any(named: 'limit'),
      ),
    ).thenAnswer((_) async => page);
  }

  ProviderContainer createContainer() {
    return ProviderContainer.test(
      overrides: [
        notificationsRepositoryProvider.overrideWithValue(repository),
        clockProvider.overrideWithValue(() => _frozenNow),
      ],
    );
  }

  Future<void> pumpScreen(WidgetTester tester) async {
    tester.view.physicalSize = const Size(500, 1600);
    tester.view.devicePixelRatio = 1.0;
    addTearDown(tester.view.reset);
    await tester.pumpWidget(
      buildScreenHarness(
        container: createContainer(),
        child: const NotificationsScreen(),
      ),
    );
  }

  group('state data', () {
    testWidgets('feed: seksi per hari + judul/isi per type + penanda unread', (
      WidgetTester tester,
    ) async {
      stubList(
        _page(<NotificationDto>[
          _notification(
            id: 'n-1',
            params: <String, dynamic>{
              'request_type': 'asset_disposal',
              'step': '1',
            },
            createdAt: _frozenNow.subtract(const Duration(minutes: 10)),
          ),
          _notification(
            id: 'n-2',
            type: 'maintenance_due',
            params: <String, dynamic>{
              'asset_tag': 'JKT01-ELK-2024-00031',
              'asset_name': 'AC Ruang Server',
              'due_date': '2026-07-25',
            },
            readAt: _frozenNow.subtract(const Duration(hours: 1)),
            createdAt: _frozenNow.subtract(const Duration(hours: 3)),
          ),
          _notification(
            id: 'n-3',
            type: 'asset_returned',
            params: <String, dynamic>{
              'asset_name': 'Proyektor Epson EB-X500',
              'asset_tag': 'JKT01-ELK-2026-00001',
            },
            readAt: _frozenNow.subtract(const Duration(hours: 10)),
            createdAt: DateTime(2026, 7, 18, 16, 40),
          ),
        ]),
      );
      await pumpScreen(tester);
      await tester.pumpAndSettle();

      // Seksi per hari (header dirender uppercase, mockup).
      expect(
        find.text(l10nId.notificationsSectionToday.toUpperCase()),
        findsOneWidget,
      );
      expect(
        find.text(l10nId.notificationsSectionYesterday.toUpperCase()),
        findsOneWidget,
      );
      // Judul + isi dirender klien dari type + params (ADR-0014).
      expect(
        find.text(l10nId.notificationsApprovalPendingTitle),
        findsOneWidget,
      );
      expect(
        find.text(
          l10nId.notificationsApprovalPendingBody(
            l10nId.approvalTypeAssetDisposal,
            '1',
          ),
        ),
        findsOneWidget,
      );
      expect(
        find.text(l10nId.notificationsMaintenanceDueTitle),
        findsOneWidget,
      );
      expect(find.text(l10nId.notificationsAssetReturnedTitle), findsOneWidget);
      // Label waktu: relatif hari ini + "Kemarin, {jam}".
      expect(find.text(l10nId.notificationsTimeMinutesAgo(10)), findsOneWidget);
      expect(
        find.text(l10nId.notificationsTimeYesterdayAt('16.40')),
        findsOneWidget,
      );
      // Penanda unread hanya pada notifikasi belum dibaca.
      expect(_unreadDot('n-1'), findsOneWidget);
      expect(_unreadDot('n-2'), findsNothing);
      expect(_unreadDot('n-3'), findsNothing);
    });

    testWidgets('tap unread tanpa target: markRead + penanda hilang', (
      WidgetTester tester,
    ) async {
      final NotificationDto unread = _notification(id: 'n-1');
      stubList(_page(<NotificationDto>[unread]));
      when(
        () => repository.markRead('n-1'),
      ).thenAnswer((_) async => unread.copyWith(readAt: _frozenNow));
      await pumpScreen(tester);
      await tester.pumpAndSettle();

      await tester.tap(find.byKey(const ValueKey<String>('notification-n-1')));
      await tester.pumpAndSettle();

      verify(() => repository.markRead('n-1')).called(1);
      expect(_unreadDot('n-1'), findsNothing);
    });

    testWidgets('markRead gagal: penanda unread kembali (revert)', (
      WidgetTester tester,
    ) async {
      stubList(_page(<NotificationDto>[_notification(id: 'n-1')]));
      when(() => repository.markRead('n-1')).thenThrow(const NetworkFailure());
      await pumpScreen(tester);
      await tester.pumpAndSettle();

      await tester.tap(find.byKey(const ValueKey<String>('notification-n-1')));
      await tester.pumpAndSettle();

      verify(() => repository.markRead('n-1')).called(1);
      expect(_unreadDot('n-1'), findsOneWidget);
    });

    testWidgets('tap notifikasi terbaca: TANPA panggilan markRead', (
      WidgetTester tester,
    ) async {
      stubList(
        _page(<NotificationDto>[_notification(id: 'n-2', readAt: _frozenNow)]),
      );
      await pumpScreen(tester);
      await tester.pumpAndSettle();

      await tester.tap(find.byKey(const ValueKey<String>('notification-n-2')));
      await tester.pumpAndSettle();

      verifyNever(() => repository.markRead(any()));
    });
  });

  group('tandai semua dibaca', () {
    testWidgets('sukses: repo dipanggil, semua penanda + aksi hilang', (
      WidgetTester tester,
    ) async {
      stubList(
        _page(<NotificationDto>[
          _notification(id: 'n-1'),
          _notification(
            id: 'n-2',
            readAt: _frozenNow,
            createdAt: _frozenNow.subtract(const Duration(hours: 2)),
          ),
        ]),
      );
      when(() => repository.markAllRead()).thenAnswer((_) async {});
      await pumpScreen(tester);
      await tester.pumpAndSettle();

      expect(find.text(l10nId.notificationsMarkAllRead), findsOneWidget);
      await tester.tap(find.text(l10nId.notificationsMarkAllRead));
      await tester.pumpAndSettle();

      verify(() => repository.markAllRead()).called(1);
      expect(_unreadDot('n-1'), findsNothing);
      // Aksi disembunyikan karena tidak ada lagi yang belum dibaca.
      expect(find.text(l10nId.notificationsMarkAllRead), findsNothing);
    });

    testWidgets('gagal: snackbar + penanda unread tetap', (
      WidgetTester tester,
    ) async {
      stubList(_page(<NotificationDto>[_notification(id: 'n-1')]));
      when(() => repository.markAllRead()).thenThrow(const NetworkFailure());
      await pumpScreen(tester);
      await tester.pumpAndSettle();

      await tester.tap(find.text(l10nId.notificationsMarkAllRead));
      await tester.pumpAndSettle();

      expect(find.text(l10nId.notificationsMarkAllFailed), findsOneWidget);
      expect(_unreadDot('n-1'), findsOneWidget);
    });

    testWidgets('feed tanpa unread: aksi tidak dirender', (
      WidgetTester tester,
    ) async {
      stubList(
        _page(<NotificationDto>[_notification(id: 'n-2', readAt: _frozenNow)]),
      );
      await pumpScreen(tester);
      await tester.pumpAndSettle();

      expect(find.text(l10nId.notificationsMarkAllRead), findsNothing);
    });
  });

  group('state kosong, loading, error', () {
    testWidgets('empty state 1:1 mockup', (WidgetTester tester) async {
      stubList(_page(const <NotificationDto>[]));
      await pumpScreen(tester);
      await tester.pumpAndSettle();

      expect(find.text(l10nId.notificationsEmptyTitle), findsOneWidget);
      expect(find.text(l10nId.notificationsEmptyBody), findsOneWidget);
    });

    testWidgets('loading: skeleton kartu tampil', (WidgetTester tester) async {
      when(
        () => repository.list(
          read: any(named: 'read'),
          offset: any(named: 'offset'),
          limit: any(named: 'limit'),
        ),
      ).thenAnswer((_) async {
        await Future<void>.delayed(const Duration(milliseconds: 50));
        return _page(const <NotificationDto>[]);
      });
      await pumpScreen(tester);
      await tester.pump();

      expect(find.byType(AppSkeleton), findsWidgets);
      await tester.pumpAndSettle();
    });

    testWidgets('offline: pesan jaringan + retry memuat ulang', (
      WidgetTester tester,
    ) async {
      when(
        () => repository.list(
          read: any(named: 'read'),
          offset: any(named: 'offset'),
          limit: any(named: 'limit'),
        ),
      ).thenThrow(const NetworkFailure());
      await pumpScreen(tester);
      await tester.pumpAndSettle();

      expect(find.text(l10nId.notificationsErrorTitle), findsOneWidget);
      expect(find.text(l10nId.notificationsErrorNetworkBody), findsOneWidget);

      stubList(_page(<NotificationDto>[_notification(id: 'n-1')]));
      await tester.tap(find.text(l10nId.commonRetry));
      await tester.pumpAndSettle();

      expect(
        find.text(l10nId.notificationsApprovalPendingTitle),
        findsOneWidget,
      );
    });

    testWidgets('error lain: pesan generik', (WidgetTester tester) async {
      when(
        () => repository.list(
          read: any(named: 'read'),
          offset: any(named: 'offset'),
          limit: any(named: 'limit'),
        ),
      ).thenThrow(const ServerFailure());
      await pumpScreen(tester);
      await tester.pumpAndSettle();

      expect(find.text(l10nId.notificationsErrorGenericBody), findsOneWidget);
    });
  });

  group('infinite scroll dan refresh', () {
    testWidgets('scroll ke bawah memuat halaman berikutnya (offset 20)', (
      WidgetTester tester,
    ) async {
      final List<NotificationDto> firstPage = List<NotificationDto>.generate(
        20,
        (int i) => _notification(
          id: 'n-$i',
          readAt: _frozenNow,
          createdAt: _frozenNow.subtract(Duration(minutes: 10 + i)),
        ),
      );
      stubList(_page(firstPage, total: 25));
      stubList(
        _page(
          List<NotificationDto>.generate(
            5,
            (int i) => _notification(
              id: 'n-2$i',
              type: 'asset_returned',
              params: <String, dynamic>{'asset_name': 'Aset lanjutan $i'},
              readAt: _frozenNow,
              createdAt: _frozenNow.subtract(Duration(hours: 5 + i)),
            ),
          ),
          total: 25,
          offset: 20,
        ),
        offset: 20,
      );
      await pumpScreen(tester);
      await tester.pumpAndSettle();

      await tester.fling(find.byType(ListView), const Offset(0, -2400), 3000);
      await tester.pumpAndSettle();

      verify(
        () => repository.list(
          read: any(named: 'read'),
          offset: 20,
          limit: any(named: 'limit'),
        ),
      ).called(1);

      await tester.fling(find.byType(ListView), const Offset(0, -2400), 3000);
      await tester.pumpAndSettle();
      expect(find.text('Aset lanjutan 4'), findsOneWidget);
    });

    testWidgets('pull-to-refresh memuat ulang feed', (
      WidgetTester tester,
    ) async {
      stubList(
        _page(<NotificationDto>[
          _notification(
            id: 'n-1',
            type: 'asset_returned',
            params: <String, dynamic>{'asset_name': 'Sebelum refresh'},
          ),
        ]),
      );
      await pumpScreen(tester);
      await tester.pumpAndSettle();

      stubList(
        _page(<NotificationDto>[
          _notification(
            id: 'n-2',
            type: 'asset_returned',
            params: <String, dynamic>{'asset_name': 'Sesudah refresh'},
          ),
        ]),
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
    testWidgets(
      'tap notifikasi approval: mark read + membuka detail approval',
      (WidgetTester tester) async {
        final NotificationDto unread = _notification(
          id: 'n-1',
          params: <String, dynamic>{
            'request_type': 'asset_disposal',
            'step': '1',
          },
          entityType: 'requests',
          entityId: 'req-1',
        );
        stubList(_page(<NotificationDto>[unread]));
        when(
          () => repository.markRead('n-1'),
        ).thenAnswer((_) async => unread.copyWith(readAt: _frozenNow));
        final _MockApprovalRepository approvalRepository =
            _MockApprovalRepository();
        when(
          () => approvalRepository.detail('req-1'),
        ).thenAnswer((_) async => throw const NetworkFailure());
        when(() => approvalRepository.inboxCount()).thenAnswer((_) async => 0);

        final ProviderContainer container = ProviderContainer.test(
          overrides: [
            authControllerProvider.overrideWith(
              () => FakeAuthController(
                initialSession: const Authenticated(fakeUser),
              ),
            ),
            scanCameraFactoryProvider.overrideWithValue(FakeScanCamera.new),
            notificationsRepositoryProvider.overrideWithValue(repository),
            approvalRepositoryProvider.overrideWithValue(approvalRepository),
            referenceLookupRepositoryProvider.overrideWithValue(
              FakeReferenceLookup(),
            ),
            stockOpnameRepositoryProvider.overrideWithValue(
              FakeStockOpnameRepository(),
            ),
            clockProvider.overrideWithValue(() => _frozenNow),
          ],
        );
        await tester.pumpWidget(RouterTestApp(container: container));
        await tester.pumpAndSettle();

        // Pindah ke tab Notifikasi.
        container.read(appRouterProvider).go('/notifications');
        await tester.pumpAndSettle();
        expect(
          find.text(l10nId.notificationsApprovalPendingTitle),
          findsOneWidget,
        );

        await tester.tap(
          find.byKey(const ValueKey<String>('notification-n-1')),
        );
        await tester.pumpAndSettle();

        verify(() => repository.markRead('n-1')).called(1);
        // Detail approval terbuka pada navigator root (error dirender sopan).
        expect(find.text(l10nId.approvalDetailTitle), findsOneWidget);
      },
    );
  });
}
