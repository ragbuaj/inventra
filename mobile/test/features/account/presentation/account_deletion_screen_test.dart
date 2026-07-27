import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:inventra_mobile/features/account/presentation/account_deletion_screen.dart';

import '../../../helpers/test_app.dart';

void main() {
  testWidgets('merender pengantar, tiga seksi, dan email kontak (id)', (
    WidgetTester tester,
  ) async {
    tester.view.physicalSize = const Size(1000, 2400);
    tester.view.devicePixelRatio = 1.0;
    addTearDown(tester.view.reset);

    await tester.pumpWidget(
      buildScreenHarness(child: const AccountDeletionScreen()),
    );
    await tester.pumpAndSettle();

    expect(find.text(l10nId.accountDeletionIntro), findsOneWidget);
    expect(find.text(l10nId.accountDeletionDeletedTitle), findsOneWidget);
    expect(find.text(l10nId.accountDeletionRetainedTitle), findsOneWidget);
    expect(find.text(l10nId.accountDeletionHowTitle), findsOneWidget);
    expect(find.text(accountDeletionContactEmail), findsOneWidget);
    // Kedua aksi kontak tersedia: kirim email + salin.
    expect(
      find.byKey(const ValueKey<String>('account-deletion-email')),
      findsOneWidget,
    );
    expect(
      find.byKey(const ValueKey<String>('account-deletion-copy')),
      findsOneWidget,
    );
  });

  group('accountDeletionMailtoUri', () {
    test('membangun mailto dengan penerima, subjek, dan isi ter-encode', () {
      final Uri uri = accountDeletionMailtoUri(
        email: 'admin@inventra.local',
        subject: 'Hapus Akun',
        body: 'Baris satu\nBaris dua',
      );

      expect(uri.scheme, 'mailto');
      expect(uri.path, 'admin@inventra.local');
      // Spasi jadi %20 (bukan '+'); newline jadi %0A.
      expect(uri.query, contains('subject=Hapus%20Akun'));
      expect(uri.query, contains('body=Baris%20satu%0ABaris%20dua'));
      // queryParameters mendekode kembali ke nilai asli.
      expect(uri.queryParameters['subject'], 'Hapus Akun');
      expect(uri.queryParameters['body'], 'Baris satu\nBaris dua');
    });
  });

  testWidgets('tombol salin menyalin email admin ke clipboard + snackbar', (
    WidgetTester tester,
  ) async {
    tester.view.physicalSize = const Size(1000, 2400);
    tester.view.devicePixelRatio = 1.0;
    addTearDown(tester.view.reset);

    // Rekam panggilan Clipboard.setData lewat mock platform channel.
    String? copied;
    tester.binding.defaultBinaryMessenger.setMockMethodCallHandler(
      SystemChannels.platform,
      (MethodCall call) async {
        if (call.method == 'Clipboard.setData') {
          copied = (call.arguments as Map<Object?, Object?>)['text'] as String?;
        }
        return null;
      },
    );
    addTearDown(
      () => tester.binding.defaultBinaryMessenger.setMockMethodCallHandler(
        SystemChannels.platform,
        null,
      ),
    );

    await tester.pumpWidget(
      buildScreenHarness(child: const AccountDeletionScreen()),
    );
    await tester.pumpAndSettle();

    await tester.tap(
      find.byKey(const ValueKey<String>('account-deletion-copy')),
    );
    await tester.pumpAndSettle();

    expect(copied, accountDeletionContactEmail);
    expect(find.text(l10nId.accountDeletionCopied), findsOneWidget);
  });
}
