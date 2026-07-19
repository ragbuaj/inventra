import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:inventra_mobile/app/theme.dart';
import 'package:inventra_mobile/core/widgets/confirm_dialog.dart';
import 'package:material_symbols_icons/symbols.dart';

import '../../helpers/test_app.dart';

void main() {
  Future<bool?> pumpAndOpen(
    WidgetTester tester, {
    bool destructive = false,
    String? cancelLabel,
  }) async {
    bool? result;
    await tester.pumpWidget(
      buildScreenHarness(
        child: Scaffold(
          body: Builder(
            builder: (BuildContext context) => Center(
              child: FilledButton(
                onPressed: () async {
                  result = await ConfirmDialog.show(
                    context,
                    title: 'Tolak pengajuan?',
                    message: 'Tindakan ini tidak dapat dibatalkan.',
                    confirmLabel: 'Tolak',
                    cancelLabel: cancelLabel,
                    destructive: destructive,
                  );
                },
                child: const Text('Buka'),
              ),
            ),
          ),
        ),
      ),
    );
    await tester.tap(find.text('Buka'));
    await tester.pumpAndSettle();
    return result;
  }

  testWidgets('menampilkan judul, isi, dan kedua aksi', (
    WidgetTester tester,
  ) async {
    await pumpAndOpen(tester, destructive: true);

    expect(find.text('Tolak pengajuan?'), findsOneWidget);
    expect(find.text('Tindakan ini tidak dapat dibatalkan.'), findsOneWidget);
    expect(find.text('Tolak'), findsOneWidget);
    expect(find.text(l10nId.commonCancel), findsOneWidget);
    expect(find.byIcon(Symbols.report_rounded), findsOneWidget);
  });

  testWidgets('hasil true saat konfirmasi, false saat batal', (
    WidgetTester tester,
  ) async {
    bool? result;
    Future<void> open(String tapTarget) async {
      await tester.pumpWidget(
        buildScreenHarness(
          child: Scaffold(
            body: Builder(
              builder: (BuildContext context) => Center(
                child: FilledButton(
                  onPressed: () async {
                    result = await ConfirmDialog.show(
                      context,
                      title: 'Keluar dari akun?',
                      message: 'Sesi akan diakhiri.',
                      confirmLabel: 'Keluar',
                    );
                  },
                  child: const Text('Buka'),
                ),
              ),
            ),
          ),
        ),
      );
      await tester.tap(find.text('Buka'));
      await tester.pumpAndSettle();
      await tester.tap(find.text(tapTarget));
      await tester.pumpAndSettle();
    }

    await open('Keluar');
    expect(result, isTrue);

    await open(l10nId.commonCancel);
    expect(result, isFalse);
  });

  testWidgets('varian destruktif memakai warna error untuk aksi utama', (
    WidgetTester tester,
  ) async {
    await pumpAndOpen(tester, destructive: true);

    final ColorScheme scheme = InventraTheme.light.colorScheme;
    final FilledButton confirm = tester.widget<FilledButton>(
      find.widgetWithText(FilledButton, 'Tolak'),
    );
    expect(
      confirm.style?.backgroundColor?.resolve(<WidgetState>{}),
      scheme.error,
    );
  });

  testWidgets('varian non-destruktif memakai warna primary + ikon default', (
    WidgetTester tester,
  ) async {
    await pumpAndOpen(tester);

    final ColorScheme scheme = InventraTheme.light.colorScheme;
    final FilledButton confirm = tester.widget<FilledButton>(
      find.widgetWithText(FilledButton, 'Tolak'),
    );
    expect(
      confirm.style?.backgroundColor?.resolve(<WidgetState>{}),
      scheme.primary,
    );
    expect(find.byIcon(Symbols.help_rounded), findsOneWidget);
  });

  testWidgets('label batal bisa diganti', (WidgetTester tester) async {
    await pumpAndOpen(tester, cancelLabel: 'Nanti saja');
    expect(find.text('Nanti saja'), findsOneWidget);
    expect(find.text(l10nId.commonCancel), findsNothing);
  });
}
