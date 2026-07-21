import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:inventra_mobile/core/authz/permissions_provider.dart';
import 'package:inventra_mobile/features/asset_detail/data/asset_action_repository.dart';
import 'package:inventra_mobile/features/asset_detail/data/asset_dto.dart';
import 'package:inventra_mobile/features/asset_detail/presentation/asset_action_bar.dart';
import 'package:mocktail/mocktail.dart';

import '../../../helpers/test_app.dart';

class _MockAssetActionRepository extends Mock implements AssetActionRepository {}

const AssetDto _available = AssetDto(
  id: 'asset-1',
  assetTag: 'JKT01-ELK-2026-00001',
  name: 'Laptop Dell Latitude 5440',
  status: 'available',
);

void main() {
  late _MockAssetActionRepository repository;

  setUp(() {
    repository = _MockAssetActionRepository();
  });

  Future<void> pumpBar(
    WidgetTester tester, {
    required Set<String> permissions,
    AssetDto asset = _available,
  }) async {
    tester.view.physicalSize = const Size(500, 1200);
    tester.view.devicePixelRatio = 1.0;
    addTearDown(tester.view.reset);
    final ProviderContainer container = ProviderContainer.test(
      overrides: [
        permissionsProvider.overrideWith((ref) async => permissions),
        assetActionRepositoryProvider.overrideWithValue(repository),
      ],
    );
    await tester.pumpWidget(
      buildScreenHarness(
        container: container,
        child: Scaffold(
          body: const SizedBox.shrink(),
          bottomNavigationBar: AssetActionBar(asset: asset),
        ),
      ),
    );
    await tester.pumpAndSettle();
  }

  testWidgets('Staf (request.create) aset available: tombol Pinjam', (
    WidgetTester tester,
  ) async {
    await pumpBar(tester, permissions: <String>{'request.create'});
    expect(find.text(l10nId.assetActionBorrow), findsOneWidget);
  });

  testWidgets('tanpa izin aksi: bar tidak dirender', (
    WidgetTester tester,
  ) async {
    await pumpBar(tester, permissions: <String>{'asset.view'});
    expect(find.text(l10nId.assetActionBorrow), findsNothing);
  });

  testWidgets('Manager aset available: Check-out belum dirender (fase M7-5)', (
    WidgetTester tester,
  ) async {
    await pumpBar(
      tester,
      permissions: <String>{'request.create', 'assignment.manage'},
    );
    // assetActionsFor menghitung checkout+reportDamage, tapi keduanya belum
    // terpasang -> bar kosong (tidak ada Pinjam).
    expect(find.text(l10nId.assetActionBorrow), findsNothing);
    expect(find.text(l10nId.assetActionCheckout), findsNothing);
  });

  testWidgets('Pinjam: sheet lalu Ajukan memanggil borrow + SnackBar sukses', (
    WidgetTester tester,
  ) async {
    when(
      () => repository.borrow(
        assetId: any(named: 'assetId'),
        dueDate: any(named: 'dueDate'),
        notes: any(named: 'notes'),
      ),
    ).thenAnswer((_) async {});

    await pumpBar(tester, permissions: <String>{'request.create'});

    await tester.tap(find.text(l10nId.assetActionBorrow));
    await tester.pumpAndSettle();

    expect(find.text(l10nId.borrowSheetTitle), findsOneWidget);

    await tester.tap(find.text(l10nId.borrowSubmit));
    await tester.pumpAndSettle();

    verify(
      () => repository.borrow(
        assetId: 'asset-1',
        dueDate: null,
        notes: any(named: 'notes'),
      ),
    ).called(1);
    expect(find.text(l10nId.borrowSuccess), findsOneWidget);
  });
}
