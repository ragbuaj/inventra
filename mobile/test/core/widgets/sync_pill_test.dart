import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:inventra_mobile/app/theme.dart';
import 'package:inventra_mobile/core/widgets/sync_pill.dart';
import 'package:material_symbols_icons/symbols.dart';

import '../../helpers/test_app.dart';

void main() {
  testWidgets('synced: label Tersinkron dengan warna success', (
    WidgetTester tester,
  ) async {
    await tester.pumpWidget(
      buildWidgetHarness(const SyncPill(status: SyncPillStatus.synced)),
    );

    expect(find.text(l10nId.commonSyncSynced), findsOneWidget);
    expect(find.byIcon(Symbols.cloud_done_rounded), findsOneWidget);
    final Text label = tester.widget<Text>(find.text(l10nId.commonSyncSynced));
    expect(label.style?.color, InventraStatusColors.light.success.text);
  });

  testWidgets('pending: menampilkan jumlah antrean dengan warna warning', (
    WidgetTester tester,
  ) async {
    await tester.pumpWidget(
      buildWidgetHarness(
        const SyncPill(status: SyncPillStatus.pending, pendingCount: 12),
      ),
    );

    expect(find.text(l10nId.commonSyncPending(12)), findsOneWidget);
    expect(find.byIcon(Symbols.cloud_upload_rounded), findsOneWidget);
    final Text label = tester.widget<Text>(
      find.text(l10nId.commonSyncPending(12)),
    );
    expect(label.style?.color, InventraStatusColors.light.warning.text);
  });

  testWidgets('syncing: label Menyinkronkan dengan ikon berputar', (
    WidgetTester tester,
  ) async {
    await tester.pumpWidget(
      buildWidgetHarness(const SyncPill(status: SyncPillStatus.syncing)),
    );
    await tester.pump(const Duration(milliseconds: 100));

    expect(find.text(l10nId.commonSyncSyncing), findsOneWidget);
    // Scoped ke SyncPill: rute MaterialApp punya RotationTransition sendiri.
    expect(
      find.descendant(
        of: find.byType(SyncPill),
        matching: find.byType(RotationTransition),
      ),
      findsOneWidget,
    );
  });

  testWidgets('failed: label gagal dengan warna danger', (
    WidgetTester tester,
  ) async {
    await tester.pumpWidget(
      buildWidgetHarness(const SyncPill(status: SyncPillStatus.failed)),
    );

    expect(find.text(l10nId.commonSyncFailed), findsOneWidget);
    expect(find.byIcon(Symbols.sync_problem_rounded), findsOneWidget);
    final Text label = tester.widget<Text>(find.text(l10nId.commonSyncFailed));
    expect(label.style?.color, InventraStatusColors.light.danger.text);
  });

  testWidgets('offline: label offline dengan warna neutral, tanpa putaran', (
    WidgetTester tester,
  ) async {
    await tester.pumpWidget(
      buildWidgetHarness(const SyncPill(status: SyncPillStatus.offline)),
    );

    expect(find.text(l10nId.commonSyncOffline), findsOneWidget);
    expect(find.byIcon(Symbols.cloud_off_rounded), findsOneWidget);
    expect(
      find.descendant(
        of: find.byType(SyncPill),
        matching: find.byType(RotationTransition),
      ),
      findsNothing,
    );
    final Text label = tester.widget<Text>(find.text(l10nId.commonSyncOffline));
    expect(label.style?.color, InventraStatusColors.light.neutral.text);
  });
}
