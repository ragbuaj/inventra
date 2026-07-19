import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:inventra_mobile/app/theme.dart';
import 'package:inventra_mobile/core/auth/auth_controller.dart';
import 'package:inventra_mobile/core/auth/auth_session.dart';
import 'package:inventra_mobile/features/notifications/presentation/unread_count_provider.dart';
import 'package:material_symbols_icons/symbols.dart';

import '../helpers/fake_auth_controller.dart';
import '../helpers/test_app.dart';

void main() {
  ProviderContainer createContainer({int unreadCount = 0}) {
    return ProviderContainer.test(
      overrides: [
        authControllerProvider.overrideWith(
          () =>
              FakeAuthController(initialSession: const Authenticated(fakeUser)),
        ),
        unreadNotificationCountProvider.overrideWithValue(unreadCount),
      ],
    );
  }

  // Label slot bottom-nav (fontSize 10.5) — membedakannya dari judul AppBar
  // yang bisa memakai teks sama (mis. "Beranda").
  Finder navLabel(String text) => find.byWidgetPredicate(
    (Widget w) => w is Text && w.data == text && w.style?.fontSize == 10.5,
  );

  testWidgets('menampilkan 5 slot dengan label dan ikon sesuai mockup', (
    WidgetTester tester,
  ) async {
    await tester.pumpWidget(RouterTestApp(container: createContainer()));
    await tester.pumpAndSettle();

    expect(navLabel(l10nId.shellTabHome), findsOneWidget);
    expect(navLabel(l10nId.shellTabOpname), findsOneWidget);
    expect(navLabel(l10nId.shellTabScan), findsOneWidget);
    expect(navLabel(l10nId.shellTabApproval), findsOneWidget);
    expect(navLabel(l10nId.shellTabNotifications), findsOneWidget);

    expect(find.byIcon(Symbols.home_rounded), findsWidgets);
    expect(find.byIcon(Symbols.fact_check_rounded), findsOneWidget);
    expect(find.byIcon(Symbols.qr_code_scanner_rounded), findsOneWidget);
    expect(find.byIcon(Symbols.approval_rounded), findsOneWidget);
    expect(find.byIcon(Symbols.notifications_rounded), findsOneWidget);
  });

  testWidgets('tab aktif memakai pill primary-container, non-aktif tidak', (
    WidgetTester tester,
  ) async {
    await tester.pumpWidget(RouterTestApp(container: createContainer()));
    await tester.pumpAndSettle();

    final ColorScheme scheme = InventraTheme.light.colorScheme;
    final Iterable<Container> pills = tester
        .widgetList<Container>(find.byType(Container))
        .where(
          (Container c) =>
              c.decoration is ShapeDecoration &&
              (c.decoration! as ShapeDecoration).color ==
                  scheme.primaryContainer &&
              (c.decoration! as ShapeDecoration).shape is StadiumBorder,
        );
    // Hanya satu pill aktif (Beranda).
    expect(pills.length, 1);

    final Text activeLabel = tester.widget<Text>(navLabel(l10nId.shellTabHome));
    expect(activeLabel.style?.fontWeight, FontWeight.w700);
    expect(activeLabel.style?.color, scheme.onPrimaryContainer);

    final Text inactiveLabel = tester.widget<Text>(
      navLabel(l10nId.shellTabOpname),
    );
    expect(inactiveLabel.style?.fontWeight, FontWeight.w500);
    expect(inactiveLabel.style?.color, scheme.onSurfaceVariant);
  });

  testWidgets('tap tab Opname berpindah branch dan memindahkan pill aktif', (
    WidgetTester tester,
  ) async {
    await tester.pumpWidget(RouterTestApp(container: createContainer()));
    await tester.pumpAndSettle();

    await tester.tap(navLabel(l10nId.shellTabOpname));
    await tester.pumpAndSettle();

    // App bar placeholder Opname + label tab = dua teks.
    expect(find.text(l10nId.shellTabOpname), findsNWidgets(2));
    expect(find.text(l10nId.commonComingSoon), findsOneWidget);

    final Text opnameLabel = tester.widget<Text>(
      navLabel(l10nId.shellTabOpname),
    );
    expect(opnameLabel.style?.fontWeight, FontWeight.w700);
  });

  testWidgets('tombol Pindai tengah membuka tab scan', (
    WidgetTester tester,
  ) async {
    await tester.pumpWidget(RouterTestApp(container: createContainer()));
    await tester.pumpAndSettle();

    await tester.tap(find.byIcon(Symbols.qr_code_scanner_rounded));
    await tester.pumpAndSettle();

    // App bar placeholder scan memakai judul yang sama dengan label tombol.
    expect(find.text(l10nId.shellTabScan), findsNWidgets(2));
    expect(find.text(l10nId.commonComingSoon), findsOneWidget);
  });

  testWidgets('tombol Pindai bergaya FAB: primary, radius 19, border cutout', (
    WidgetTester tester,
  ) async {
    await tester.pumpWidget(RouterTestApp(container: createContainer()));
    await tester.pumpAndSettle();

    final ColorScheme scheme = InventraTheme.light.colorScheme;
    final Material fabMaterial = tester.widget<Material>(
      find
          .ancestor(
            of: find.byIcon(Symbols.qr_code_scanner_rounded),
            matching: find.byType(Material),
          )
          .first,
    );
    expect(fabMaterial.color, scheme.primary);
    final RoundedRectangleBorder shape =
        fabMaterial.shape! as RoundedRectangleBorder;
    expect(shape.borderRadius, BorderRadius.circular(19));
    // Border 4px warna bar (efek cutout terhadap bar putih light).
    expect(shape.side.width, 4);
    expect(shape.side.color, InventraTheme.light.cardTheme.color);

    final Container fabShadowBox = tester.widget<Container>(
      find
          .ancestor(
            of: find.byIcon(Symbols.qr_code_scanner_rounded),
            matching: find.byType(Container),
          )
          .first,
    );
    final BoxDecoration decoration = fabShadowBox.decoration! as BoxDecoration;
    expect(decoration.boxShadow, isNotEmpty);
  });

  group('badge unread Notif', () {
    testWidgets('tampil dengan angka saat count > 0', (
      WidgetTester tester,
    ) async {
      await tester.pumpWidget(
        RouterTestApp(container: createContainer(unreadCount: 3)),
      );
      await tester.pumpAndSettle();

      expect(find.text('3'), findsOneWidget);
    });

    testWidgets('disembunyikan saat count 0', (WidgetTester tester) async {
      await tester.pumpWidget(RouterTestApp(container: createContainer()));
      await tester.pumpAndSettle();

      expect(find.text('0'), findsNothing);
    });

    testWidgets('count di atas 99 ditampilkan 99+', (
      WidgetTester tester,
    ) async {
      await tester.pumpWidget(
        RouterTestApp(container: createContainer(unreadCount: 120)),
      );
      await tester.pumpAndSettle();

      expect(find.text('99+'), findsOneWidget);
    });
  });
}
