import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:inventra_mobile/core/api/app_failure.dart';
import 'package:inventra_mobile/features/account/data/account_security_repository.dart';
import 'package:inventra_mobile/features/account/presentation/account_providers.dart';
import 'package:inventra_mobile/features/account/presentation/account_security_screen.dart';
import 'package:mocktail/mocktail.dart';

import '../../../helpers/fake_account_repository.dart';
import '../../../helpers/test_app.dart';

class _MockSecurityRepository extends Mock
    implements AccountSecurityRepository {}

void main() {
  late _MockSecurityRepository repository;

  setUp(() {
    repository = _MockSecurityRepository();
  });

  Future<void> pump(WidgetTester tester) async {
    tester.view.physicalSize = const Size(500, 1400);
    tester.view.devicePixelRatio = 1.0;
    addTearDown(tester.view.reset);
    final ProviderContainer container = ProviderContainer.test(
      overrides: [
        accountSecurityRepositoryProvider.overrideWithValue(repository),
        accountProfileProvider.overrideWith((ref) async => fakeProfile),
      ],
    );
    await tester.pumpWidget(
      buildScreenHarness(
        container: container,
        child: const AccountSecurityScreen(),
      ),
    );
    await tester.pumpAndSettle();
  }

  testWidgets('email pengguna tampil di baris Email', (
    WidgetTester tester,
  ) async {
    await pump(tester);
    expect(find.text(fakeProfile.email), findsOneWidget);
  });

  group('ganti password', () {
    testWidgets('submit sukses -> requestPasswordChange + cek email', (
      WidgetTester tester,
    ) async {
      when(
        () => repository.requestPasswordChange(any()),
      ).thenAnswer((_) async {});

      await pump(tester);
      await tester.tap(
        find.byKey(const ValueKey<String>('security-change-password')),
      );
      await tester.pumpAndSettle();
      await tester.enterText(find.byType(TextField).first, 'rahasia123');
      await tester.tap(
        find.byKey(const ValueKey<String>('security-password-submit')),
      );
      await tester.pumpAndSettle();

      verify(() => repository.requestPasswordChange('rahasia123')).called(1);
      expect(find.text(l10nId.securityCheckEmailTitle), findsOneWidget);
    });

    testWidgets('password salah (400): pesan inline, bukan cek email', (
      WidgetTester tester,
    ) async {
      when(
        () => repository.requestPasswordChange(any()),
      ).thenThrow(const ValidationFailure('password lama salah'));

      await pump(tester);
      await tester.tap(
        find.byKey(const ValueKey<String>('security-change-password')),
      );
      await tester.pumpAndSettle();
      await tester.enterText(find.byType(TextField).first, 'salah');
      await tester.tap(
        find.byKey(const ValueKey<String>('security-password-submit')),
      );
      await tester.pumpAndSettle();

      expect(find.text(l10nId.securityWrongPassword), findsOneWidget);
      expect(find.text(l10nId.securityCheckEmailTitle), findsNothing);
    });

    testWidgets('gagal jaringan: pesan error generik inline (bukan cek email)', (
      WidgetTester tester,
    ) async {
      when(
        () => repository.requestPasswordChange(any()),
      ).thenThrow(const NetworkFailure());

      await pump(tester);
      await tester.tap(
        find.byKey(const ValueKey<String>('security-change-password')),
      );
      await tester.pumpAndSettle();
      await tester.enterText(find.byType(TextField).first, 'rahasia123');
      await tester.tap(
        find.byKey(const ValueKey<String>('security-password-submit')),
      );
      await tester.pumpAndSettle();

      expect(find.text(l10nId.securityError), findsOneWidget);
      expect(find.text(l10nId.securityCheckEmailTitle), findsNothing);
    });
  });

  group('ganti email', () {
    testWidgets('submit sukses -> requestEmailChange + cek email', (
      WidgetTester tester,
    ) async {
      when(
        () => repository.requestEmailChange(
          newEmail: any(named: 'newEmail'),
          currentPassword: any(named: 'currentPassword'),
        ),
      ).thenAnswer((_) async {});

      await pump(tester);
      await tester.tap(
        find.byKey(const ValueKey<String>('security-change-email')),
      );
      await tester.pumpAndSettle();
      await tester.enterText(find.byType(TextField).at(0), 'baru@x.local');
      await tester.enterText(find.byType(TextField).at(1), 'rahasia123');
      await tester.tap(
        find.byKey(const ValueKey<String>('security-email-submit')),
      );
      await tester.pumpAndSettle();

      verify(
        () => repository.requestEmailChange(
          newEmail: 'baru@x.local',
          currentPassword: 'rahasia123',
        ),
      ).called(1);
      expect(find.text(l10nId.securityCheckEmailTitle), findsOneWidget);
    });

    testWidgets('email sudah dipakai (409): pesan inline', (
      WidgetTester tester,
    ) async {
      when(
        () => repository.requestEmailChange(
          newEmail: any(named: 'newEmail'),
          currentPassword: any(named: 'currentPassword'),
        ),
      ).thenThrow(const ConflictFailure());

      await pump(tester);
      await tester.tap(
        find.byKey(const ValueKey<String>('security-change-email')),
      );
      await tester.pumpAndSettle();
      await tester.enterText(find.byType(TextField).at(0), 'ada@x.local');
      await tester.enterText(find.byType(TextField).at(1), 'p');
      await tester.tap(
        find.byKey(const ValueKey<String>('security-email-submit')),
      );
      await tester.pumpAndSettle();

      expect(find.text(l10nId.securityEmailInUse), findsOneWidget);
    });

    testWidgets('email salah format: validasi klien, tak panggil repository', (
      WidgetTester tester,
    ) async {
      await pump(tester);
      await tester.tap(
        find.byKey(const ValueKey<String>('security-change-email')),
      );
      await tester.pumpAndSettle();
      await tester.enterText(find.byType(TextField).at(0), 'bukan-email');
      await tester.enterText(find.byType(TextField).at(1), 'rahasia123');
      await tester.tap(
        find.byKey(const ValueKey<String>('security-email-submit')),
      );
      await tester.pumpAndSettle();

      // Pesan format email (bukan "password lama salah" yang menyesatkan).
      expect(find.text(l10nId.securityInvalidEmail), findsOneWidget);
      verifyNever(
        () => repository.requestEmailChange(
          newEmail: any(named: 'newEmail'),
          currentPassword: any(named: 'currentPassword'),
        ),
      );
    });

    testWidgets('gagal jaringan: pesan error generik inline', (
      WidgetTester tester,
    ) async {
      when(
        () => repository.requestEmailChange(
          newEmail: any(named: 'newEmail'),
          currentPassword: any(named: 'currentPassword'),
        ),
      ).thenThrow(const NetworkFailure());

      await pump(tester);
      await tester.tap(
        find.byKey(const ValueKey<String>('security-change-email')),
      );
      await tester.pumpAndSettle();
      await tester.enterText(find.byType(TextField).at(0), 'baru@x.local');
      await tester.enterText(find.byType(TextField).at(1), 'rahasia123');
      await tester.tap(
        find.byKey(const ValueKey<String>('security-email-submit')),
      );
      await tester.pumpAndSettle();

      expect(find.text(l10nId.securityError), findsOneWidget);
      expect(find.text(l10nId.securityCheckEmailTitle), findsNothing);
    });
  });
}
