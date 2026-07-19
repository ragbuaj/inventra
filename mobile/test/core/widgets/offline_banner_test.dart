import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:inventra_mobile/app/theme.dart';
import 'package:inventra_mobile/core/widgets/offline_banner.dart';
import 'package:material_symbols_icons/symbols.dart';

import '../../helpers/test_app.dart';

void main() {
  testWidgets('memakai pesan default dari ARB dan warna warning', (
    WidgetTester tester,
  ) async {
    await tester.pumpWidget(buildWidgetHarness(const OfflineBanner()));

    expect(find.text(l10nId.commonOfflineBanner), findsOneWidget);
    expect(find.byIcon(Symbols.cloud_off_rounded), findsOneWidget);
    final Text label = tester.widget<Text>(
      find.text(l10nId.commonOfflineBanner),
    );
    expect(label.style?.color, InventraStatusColors.light.warning.text);
  });

  testWidgets('pesan kustom menggantikan default', (WidgetTester tester) async {
    await tester.pumpWidget(
      buildWidgetHarness(
        const OfflineBanner(message: 'Offline — mode baca saja'),
      ),
    );

    expect(find.text('Offline — mode baca saja'), findsOneWidget);
    expect(find.text(l10nId.commonOfflineBanner), findsNothing);
  });
}
