import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:inventra_mobile/core/api/app_failure.dart';
import 'package:inventra_mobile/core/auth/data/auth_repository.dart';
import 'package:inventra_mobile/features/login/presentation/forgot_password_screen.dart';
import 'package:mocktail/mocktail.dart';

import '../../../helpers/test_app.dart';

class _MockAuthRepository extends Mock implements AuthRepository {}

void main() {
  late _MockAuthRepository repository;

  setUp(() {
    repository = _MockAuthRepository();
  });

  Future<void> pump(WidgetTester tester) async {
    final ProviderContainer container = ProviderContainer.test(
      overrides: [
        authRepositoryProvider.overrideWithValue(repository),
      ],
    );
    await tester.pumpWidget(
      buildScreenHarness(
        container: container,
        child: const ForgotPasswordScreen(),
      ),
    );
    await tester.pumpAndSettle();
  }

  testWidgets('email kosong: pesan validasi, tidak memanggil repository', (
    WidgetTester tester,
  ) async {
    await pump(tester);
    await tester.tap(find.byKey(const ValueKey<String>('forgot-submit')));
    await tester.pumpAndSettle();

    expect(find.text(l10nId.forgotEmailRequired), findsOneWidget);
    verifyNever(() => repository.forgotPassword(any()));
  });

  testWidgets('submit sukses: forgotPassword dipanggil + konfirmasi', (
    WidgetTester tester,
  ) async {
    when(() => repository.forgotPassword(any())).thenAnswer((_) async {});

    await pump(tester);
    await tester.enterText(find.byType(TextField), 'ragil@inventra.local');
    await tester.tap(find.byKey(const ValueKey<String>('forgot-submit')));
    await tester.pumpAndSettle();

    verify(() => repository.forgotPassword('ragil@inventra.local')).called(1);
    expect(find.text(l10nId.forgotSentTitle), findsOneWidget);
    expect(find.text(l10nId.forgotSentBody), findsOneWidget);
  });

  testWidgets(
    'anti-enumerasi: pesan konfirmasi identik untuk email tak dikenal',
    (WidgetTester tester) async {
      // Server SELALU 200 walau akun tidak ada; UI menampilkan pesan sama.
      when(() => repository.forgotPassword(any())).thenAnswer((_) async {});

      await pump(tester);
      await tester.enterText(find.byType(TextField), 'tidak-ada@nowhere.local');
      await tester.tap(find.byKey(const ValueKey<String>('forgot-submit')));
      await tester.pumpAndSettle();

      expect(find.text(l10nId.forgotSentBody), findsOneWidget);
      // Tidak membocorkan status akun — hanya tombol kembali ke login.
      expect(
        find.byKey(const ValueKey<String>('forgot-back-to-login')),
        findsOneWidget,
      );
    },
  );

  testWidgets('gagal jaringan: pesan error inline, tetap di form', (
    WidgetTester tester,
  ) async {
    when(
      () => repository.forgotPassword(any()),
    ).thenThrow(const NetworkFailure());

    await pump(tester);
    await tester.enterText(find.byType(TextField), 'ragil@inventra.local');
    await tester.tap(find.byKey(const ValueKey<String>('forgot-submit')));
    await tester.pumpAndSettle();

    expect(find.text(l10nId.forgotError), findsOneWidget);
    expect(find.text(l10nId.forgotSentTitle), findsNothing);
  });
}
