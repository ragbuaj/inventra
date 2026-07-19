import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:inventra_mobile/app/theme.dart';
import 'package:inventra_mobile/core/widgets/status_chip.dart';

import '../../helpers/test_app.dart';

void main() {
  const Map<StatusChipVariant, String> labels = <StatusChipVariant, String>{
    StatusChipVariant.success: 'Tersedia',
    StatusChipVariant.info: 'Dipinjam',
    StatusChipVariant.warning: 'Maintenance',
    StatusChipVariant.danger: 'Hilang',
    StatusChipVariant.neutral: 'Dilepas',
  };

  StatusColorSet setFor(InventraStatusColors colors, StatusChipVariant v) {
    return switch (v) {
      StatusChipVariant.success => colors.success,
      StatusChipVariant.info => colors.info,
      StatusChipVariant.warning => colors.warning,
      StatusChipVariant.danger => colors.danger,
      StatusChipVariant.neutral => colors.neutral,
    };
  }

  for (final MapEntry<StatusChipVariant, String> entry in labels.entries) {
    testWidgets('varian ${entry.key.name} memakai triplet warna tema light', (
      WidgetTester tester,
    ) async {
      await tester.pumpWidget(
        buildWidgetHarness(StatusChip(label: entry.value, variant: entry.key)),
      );

      final StatusColorSet expected = setFor(
        InventraStatusColors.light,
        entry.key,
      );

      final Text label = tester.widget<Text>(find.text(entry.value));
      expect(label.style?.color, expected.text);

      final Iterable<Container> containers = tester.widgetList<Container>(
        find.descendant(
          of: find.byType(StatusChip),
          matching: find.byType(Container),
        ),
      );
      // Pill luar (ShapeDecoration bg) + dot bulat (BoxDecoration).
      expect(
        containers.any(
          (Container c) =>
              c.decoration is ShapeDecoration &&
              (c.decoration! as ShapeDecoration).color == expected.bg,
        ),
        isTrue,
      );
      expect(
        containers.any(
          (Container c) =>
              c.decoration is BoxDecoration &&
              (c.decoration! as BoxDecoration).color == expected.dot &&
              (c.decoration! as BoxDecoration).shape == BoxShape.circle,
        ),
        isTrue,
      );
    });
  }

  testWidgets('mengikuti tema dark lewat ThemeExtension', (
    WidgetTester tester,
  ) async {
    await tester.pumpWidget(
      buildWidgetHarness(
        const StatusChip(label: 'Tersedia', variant: StatusChipVariant.success),
        theme: InventraTheme.dark,
      ),
    );

    final Text label = tester.widget<Text>(find.text('Tersedia'));
    expect(label.style?.color, InventraStatusColors.dark.success.text);
  });
}
