import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:inventra_mobile/core/widgets/empty_state.dart';
import 'package:material_symbols_icons/symbols.dart';

import '../../helpers/test_app.dart';

void main() {
  testWidgets('menampilkan ikon, judul, dan subjudul', (
    WidgetTester tester,
  ) async {
    await tester.pumpWidget(
      buildWidgetHarness(
        const EmptyState(
          icon: Symbols.inventory_2_rounded,
          title: 'Belum ada sesi opname',
          subtitle: 'Mulai sesi baru untuk menginventaris aset di lokasi Anda.',
        ),
      ),
    );

    expect(find.byIcon(Symbols.inventory_2_rounded), findsOneWidget);
    expect(find.text('Belum ada sesi opname'), findsOneWidget);
    expect(
      find.text('Mulai sesi baru untuk menginventaris aset di lokasi Anda.'),
      findsOneWidget,
    );
    expect(find.byType(FilledButton), findsNothing);
  });

  testWidgets('tanpa subjudul hanya judul yang dirender', (
    WidgetTester tester,
  ) async {
    await tester.pumpWidget(
      buildWidgetHarness(
        const EmptyState(
          icon: Symbols.search_rounded,
          title: 'Tidak ada hasil',
        ),
      ),
    );

    expect(find.text('Tidak ada hasil'), findsOneWidget);
    expect(find.byType(FilledButton), findsNothing);
  });

  testWidgets('aksi opsional dirender dan memanggil callback', (
    WidgetTester tester,
  ) async {
    int tapped = 0;
    await tester.pumpWidget(
      buildWidgetHarness(
        EmptyState(
          icon: Symbols.inventory_2_rounded,
          title: 'Belum ada sesi opname',
          actionLabel: 'Mulai Sesi Opname',
          onAction: () => tapped += 1,
        ),
      ),
    );

    await tester.tap(find.text('Mulai Sesi Opname'));
    expect(tapped, 1);
  });
}
