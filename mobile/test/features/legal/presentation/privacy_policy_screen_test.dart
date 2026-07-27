import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:inventra_mobile/features/legal/presentation/privacy_policy_screen.dart';

import '../../../helpers/test_app.dart';

void main() {
  testWidgets('merender judul, pengantar, tanggal, dan enam seksi (id)', (
    WidgetTester tester,
  ) async {
    // Viewport tinggi supaya seluruh seksi ListView ter-build tanpa scroll
    // (SliverList membangun anak secara lazy).
    tester.view.physicalSize = const Size(1000, 3000);
    tester.view.devicePixelRatio = 1.0;
    addTearDown(tester.view.reset);

    await tester.pumpWidget(
      buildScreenHarness(child: const PrivacyPolicyScreen()),
    );
    await tester.pumpAndSettle();

    expect(find.text(l10nId.privacyTitle), findsWidgets);
    expect(find.text(l10nId.privacyLastUpdated), findsOneWidget);
    expect(find.text(l10nId.privacyIntro), findsOneWidget);

    // Keenam judul seksi hadir.
    expect(find.text(l10nId.privacyCollectTitle), findsOneWidget);
    expect(find.text(l10nId.privacyUseTitle), findsOneWidget);
    expect(find.text(l10nId.privacyStorageTitle), findsOneWidget);
    expect(find.text(l10nId.privacySharingTitle), findsOneWidget);
    expect(find.text(l10nId.privacyRightsTitle), findsOneWidget);
    expect(find.text(l10nId.privacyContactTitle), findsOneWidget);
  });

  testWidgets('isi seksi dapat digulir hingga kontak terlihat', (
    WidgetTester tester,
  ) async {
    await tester.pumpWidget(
      buildScreenHarness(child: const PrivacyPolicyScreen()),
    );
    await tester.pumpAndSettle();

    await tester.scrollUntilVisible(
      find.text(l10nId.privacyContactBody),
      300,
      scrollable: find.byType(Scrollable),
    );
    expect(find.text(l10nId.privacyContactBody), findsOneWidget);
  });
}
