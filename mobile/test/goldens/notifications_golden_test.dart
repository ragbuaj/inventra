@Tags(<String>['golden'])
library;

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:inventra_mobile/app/theme.dart';
import 'package:inventra_mobile/core/i18n/gen/app_localizations.dart';
import 'package:inventra_mobile/core/utils/clock.dart';
import 'package:inventra_mobile/features/notifications/data/notification_dto.dart';
import 'package:inventra_mobile/features/notifications/data/notifications_repository.dart';
import 'package:inventra_mobile/features/notifications/presentation/notifications_screen.dart';

import '../helpers/fake_notifications_repository.dart';
import '../helpers/golden_fonts.dart';

/// Waktu beku paritas mockup (09.41).
final DateTime _frozenNow = DateTime(2026, 7, 19, 9, 41);

/// Feed paritas mockup "Feed terisi — campuran dibaca/belum": dua unread hari
/// ini, maintenance terbaca, dua kartu kemarin, satu kartu bertanggal.
/// ("Sinkronisasi selesai" mockup bukan type kontrak — tidak dirender.)
final List<NotificationDto> _goldenFeed = <NotificationDto>[
  NotificationDto(
    id: 'n-1',
    type: 'approval_pending',
    params: const <String, dynamic>{
      'request_type': 'asset_disposal',
      'step': '1',
    },
    entityType: 'requests',
    entityId: 'req-1',
    createdAt: DateTime(2026, 7, 19, 9, 31),
  ),
  NotificationDto(
    id: 'n-2',
    type: 'approval_decided',
    params: const <String, dynamic>{
      'request_type': 'asset_create',
      'status': 'approved',
    },
    entityType: 'requests',
    entityId: 'req-2',
    createdAt: DateTime(2026, 7, 19, 8, 41),
  ),
  NotificationDto(
    id: 'n-3',
    type: 'maintenance_due',
    params: const <String, dynamic>{
      'asset_tag': 'JKT01-ELK-2024-00031',
      'asset_name': 'AC Ruang Server',
      'due_date': '2026-07-25',
    },
    entityType: 'assets',
    entityId: 'asset-31',
    readAt: DateTime(2026, 7, 19, 7),
    createdAt: DateTime(2026, 7, 19, 6, 41),
  ),
  NotificationDto(
    id: 'n-4',
    type: 'asset_returned',
    params: const <String, dynamic>{
      'asset_tag': 'JKT01-ELK-2026-00001',
      'asset_name': 'Proyektor Epson EB-X500',
    },
    entityType: 'assets',
    entityId: 'asset-1',
    readAt: DateTime(2026, 7, 18, 17),
    createdAt: DateTime(2026, 7, 18, 16, 40),
  ),
  NotificationDto(
    id: 'n-5',
    type: 'approval_decided',
    params: const <String, dynamic>{
      'request_type': 'assignment',
      'status': 'rejected',
    },
    entityType: 'requests',
    entityId: 'req-5',
    readAt: DateTime(2026, 7, 18, 12),
    createdAt: DateTime(2026, 7, 18, 11, 5),
  ),
  NotificationDto(
    id: 'n-6',
    type: 'asset_returned',
    params: const <String, dynamic>{
      'asset_tag': 'JKT01-FUR-2025-00104',
      'asset_name': 'Kursi Kerja Ergonomis',
    },
    entityType: 'assets',
    entityId: 'asset-104',
    readAt: DateTime(2026, 7, 16, 10),
    createdAt: DateTime(2026, 7, 16, 9, 15),
  ),
];

/// Golden layar Notifikasi light + dark (feed campuran dibaca/belum, aksi
/// tandai semua, seksi per hari). Digenerate dan diverifikasi lokal
/// (Windows): `flutter test --update-goldens --tags golden`.
void main() {
  setUpAll(loadAppFonts);

  Widget buildScreen(ThemeData theme) {
    return ProviderScope(
      overrides: [
        notificationsRepositoryProvider.overrideWithValue(
          FakeNotificationsRepository(feed: _goldenFeed),
        ),
        clockProvider.overrideWithValue(() => _frozenNow),
      ],
      child: MaterialApp(
        theme: theme,
        locale: const Locale('id'),
        localizationsDelegates: AppLocalizations.localizationsDelegates,
        supportedLocales: const <Locale>[Locale('id'), Locale('en')],
        home: const NotificationsScreen(),
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

  testWidgets('notifikasi light', (WidgetTester tester) async {
    await pumpAtPhoneSize(tester, buildScreen(InventraTheme.light));
    await expectLater(
      find.byType(NotificationsScreen),
      matchesGoldenFile('notifications_light.png'),
    );
  });

  testWidgets('notifikasi dark', (WidgetTester tester) async {
    await pumpAtPhoneSize(tester, buildScreen(InventraTheme.dark));
    await expectLater(
      find.byType(NotificationsScreen),
      matchesGoldenFile('notifications_dark.png'),
    );
  });
}
