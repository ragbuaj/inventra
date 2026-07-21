import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:inventra_mobile/core/api/app_failure.dart';
import 'package:inventra_mobile/core/utils/clock.dart';
import 'package:inventra_mobile/core/widgets/app_skeleton.dart';
import 'package:inventra_mobile/core/widgets/status_chip.dart';
import 'package:inventra_mobile/features/my_assets/data/my_assets_repository.dart';
import 'package:inventra_mobile/features/my_assets/presentation/my_assets_screen.dart';
import 'package:mocktail/mocktail.dart';

import '../../../helpers/test_app.dart';

class _MockMyAssetsRepository extends Mock implements MyAssetsRepository {}

final DateTime _frozenNow = DateTime.utc(2026, 7, 21, 9);

MyAssignmentDto _held({
  required String name,
  String tag = 'JKT01-ELK-2026-00001',
  String? checkoutDate = '2026-07-01T00:00:00Z',
  String? dueDate,
}) {
  return MyAssignmentDto(
    assetName: name,
    assetTag: tag,
    status: 'active',
    checkoutDate: checkoutDate,
    dueDate: dueDate,
  );
}

void main() {
  late _MockMyAssetsRepository repository;

  setUp(() {
    repository = _MockMyAssetsRepository();
  });

  ProviderContainer createContainer() {
    return ProviderContainer.test(
      overrides: [
        myAssetsRepositoryProvider.overrideWithValue(repository),
        clockProvider.overrideWithValue(() => _frozenNow),
      ],
    );
  }

  Future<void> pump(WidgetTester tester) async {
    tester.view.physicalSize = const Size(500, 1600);
    tester.view.devicePixelRatio = 1.0;
    addTearDown(tester.view.reset);
    await tester.pumpWidget(
      buildScreenHarness(
        container: createContainer(),
        child: const MyAssetsScreen(),
      ),
    );
  }

  testWidgets('kartu: nama + kode + chip Dipinjam + jumlah + jatuh tempo', (
    WidgetTester tester,
  ) async {
    when(() => repository.list()).thenAnswer(
      (_) async => <MyAssignmentDto>[
        _held(name: 'Laptop Dell Latitude 5440', dueDate: '2026-08-01'),
        _held(
          name: 'Proyektor Epson EB-X500',
          tag: 'JKT01-ELK-2026-00014',
          dueDate: '2026-08-15',
        ),
      ],
    );
    await pump(tester);
    await tester.pumpAndSettle();

    expect(find.text(l10nId.myAssetsTitle), findsOneWidget);
    expect(find.text(l10nId.myAssetsCount(2)), findsOneWidget);
    expect(find.text('Laptop Dell Latitude 5440'), findsOneWidget);
    expect(find.text('JKT01-ELK-2026-00001'), findsOneWidget);
    expect(
      find.descendant(
        of: find.byType(StatusChip),
        matching: find.text(l10nId.assetDetailStatusAssigned),
      ),
      findsNWidgets(2),
    );
    expect(find.text(l10nId.myAssetsHeldSince('1 Jul 2026')), findsNWidgets(2));
    expect(find.text(l10nId.myAssetsDue('1 Agu 2026')), findsOneWidget);
    // Tidak ada yang terlambat (jatuh tempo di masa depan).
    expect(find.text(l10nId.myAssetsOverdue), findsNothing);
  });

  testWidgets('jatuh tempo lewat: penanda Terlambat', (
    WidgetTester tester,
  ) async {
    when(() => repository.list()).thenAnswer(
      (_) async => <MyAssignmentDto>[
        _held(name: 'Kamera DSLR Canon', dueDate: '2026-07-10'),
      ],
    );
    await pump(tester);
    await tester.pumpAndSettle();

    expect(find.text(l10nId.myAssetsOverdue), findsOneWidget);
  });

  testWidgets('kosong: empty state', (WidgetTester tester) async {
    when(() => repository.list()).thenAnswer((_) async => <MyAssignmentDto>[]);
    await pump(tester);
    await tester.pumpAndSettle();

    expect(find.text(l10nId.myAssetsEmptyTitle), findsOneWidget);
    expect(find.text(l10nId.myAssetsEmptyBody), findsOneWidget);
  });

  testWidgets('loading: skeleton', (WidgetTester tester) async {
    when(() => repository.list()).thenAnswer((_) async {
      await Future<void>.delayed(const Duration(milliseconds: 50));
      return <MyAssignmentDto>[];
    });
    await pump(tester);
    await tester.pump();

    expect(find.byType(AppSkeleton), findsWidgets);
    await tester.pumpAndSettle();
  });

  testWidgets('offline: pesan jaringan + retry memuat ulang', (
    WidgetTester tester,
  ) async {
    when(() => repository.list()).thenThrow(const NetworkFailure());
    await pump(tester);
    await tester.pumpAndSettle();

    expect(find.text(l10nId.myAssetsErrorTitle), findsOneWidget);
    expect(find.text(l10nId.myAssetsErrorNetworkBody), findsOneWidget);

    when(() => repository.list()).thenAnswer(
      (_) async => <MyAssignmentDto>[_held(name: 'Setelah retry')],
    );
    await tester.tap(find.text(l10nId.commonRetry));
    await tester.pumpAndSettle();

    expect(find.text('Setelah retry'), findsOneWidget);
  });

  testWidgets('403: pesan akses tanpa retry', (WidgetTester tester) async {
    when(() => repository.list()).thenThrow(const ForbiddenFailure());
    await pump(tester);
    await tester.pumpAndSettle();

    expect(find.text(l10nId.myAssetsForbiddenTitle), findsOneWidget);
    expect(find.text(l10nId.commonRetry), findsNothing);
  });
}
