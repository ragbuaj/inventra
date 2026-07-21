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

const AssetDto _assigned = AssetDto(
  id: 'asset-1',
  assetTag: 'JKT01-ELK-2026-00001',
  name: 'Laptop Dell Latitude 5440',
  status: 'assigned',
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

  testWidgets('Manager aset available: tombol Check-out (bukan Pinjam)', (
    WidgetTester tester,
  ) async {
    await pumpBar(
      tester,
      permissions: <String>{'request.create', 'assignment.manage'},
    );
    expect(find.text(l10nId.assetActionCheckout), findsOneWidget);
    expect(find.text(l10nId.assetActionBorrow), findsNothing);
  });

  testWidgets('Manager aset assigned: tombol Check-in', (
    WidgetTester tester,
  ) async {
    await pumpBar(
      tester,
      permissions: <String>{'assignment.manage'},
      asset: _assigned,
    );
    expect(find.text(l10nId.assetActionCheckin), findsOneWidget);
  });

  testWidgets('Check-out: pilih pegawai lalu submit memanggil checkout', (
    WidgetTester tester,
  ) async {
    when(
      () => repository.searchEmployees(any()),
    ).thenAnswer((_) async => const <EmployeeOption>[EmployeeOption('emp-1', 'Budi Santoso')]);
    when(
      () => repository.checkout(
        assetId: any(named: 'assetId'),
        employeeId: any(named: 'employeeId'),
        checkoutDate: any(named: 'checkoutDate'),
        dueDate: any(named: 'dueDate'),
        conditionOut: any(named: 'conditionOut'),
      ),
    ).thenAnswer((_) async {});

    await pumpBar(tester, permissions: <String>{'assignment.manage'});

    await tester.tap(find.text(l10nId.assetActionCheckout));
    await tester.pumpAndSettle();
    expect(find.text(l10nId.checkoutSheetTitle), findsOneWidget);

    await tester.tap(find.text('Budi Santoso'));
    await tester.pumpAndSettle();
    await tester.tap(find.text(l10nId.checkoutSubmit).last);
    await tester.pumpAndSettle();

    verify(
      () => repository.checkout(
        assetId: 'asset-1',
        employeeId: 'emp-1',
        checkoutDate: any(named: 'checkoutDate'),
        dueDate: null,
        conditionOut: any(named: 'conditionOut'),
      ),
    ).called(1);
    expect(find.text(l10nId.checkoutSuccess), findsOneWidget);
  });

  testWidgets('Check-out tanpa pegawai: validasi menahan submit', (
    WidgetTester tester,
  ) async {
    when(
      () => repository.searchEmployees(any()),
    ).thenAnswer((_) async => const <EmployeeOption>[]);

    await pumpBar(tester, permissions: <String>{'assignment.manage'});
    await tester.tap(find.text(l10nId.assetActionCheckout));
    await tester.pumpAndSettle();
    await tester.tap(find.text(l10nId.checkoutSubmit).last);
    await tester.pumpAndSettle();

    expect(find.text(l10nId.checkoutEmployeeRequired), findsOneWidget);
    verifyNever(
      () => repository.checkout(
        assetId: any(named: 'assetId'),
        employeeId: any(named: 'employeeId'),
        checkoutDate: any(named: 'checkoutDate'),
        dueDate: any(named: 'dueDate'),
        conditionOut: any(named: 'conditionOut'),
      ),
    );
  });

  testWidgets('Check-in: resolusi penugasan aktif lalu submit', (
    WidgetTester tester,
  ) async {
    when(() => repository.activeAssignment(any())).thenAnswer(
      (_) async =>
          const ActiveAssignment(id: 'as-1', holderName: 'Budi Santoso'),
    );
    when(
      () => repository.checkin(
        assignmentId: any(named: 'assignmentId'),
        conditionIn: any(named: 'conditionIn'),
        needsMaintenance: any(named: 'needsMaintenance'),
      ),
    ).thenAnswer((_) async {});

    await pumpBar(
      tester,
      permissions: <String>{'assignment.manage'},
      asset: _assigned,
    );

    await tester.tap(find.text(l10nId.assetActionCheckin));
    await tester.pumpAndSettle();
    expect(find.text(l10nId.checkinSheetTitle), findsOneWidget);
    expect(find.text('Budi Santoso'), findsOneWidget);

    await tester.tap(find.text(l10nId.checkinSubmit).last);
    await tester.pumpAndSettle();

    verify(
      () => repository.checkin(
        assignmentId: 'as-1',
        conditionIn: any(named: 'conditionIn'),
        needsMaintenance: false,
      ),
    ).called(1);
    expect(find.text(l10nId.checkinSuccess), findsOneWidget);
  });

  testWidgets('Lapor Kerusakan: pilih kategori lalu kirim memanggil report', (
    WidgetTester tester,
  ) async {
    when(() => repository.problemCategories()).thenAnswer(
      (_) async =>
          const <ProblemCategory>[ProblemCategory('pc-1', 'Layar Rusak')],
    );
    when(
      () => repository.reportDamage(
        assetId: any(named: 'assetId'),
        problemCategoryId: any(named: 'problemCategoryId'),
        description: any(named: 'description'),
      ),
    ).thenAnswer((_) async {});

    await pumpBar(tester, permissions: <String>{'request.create'});

    await tester.tap(find.text(l10nId.assetActionReportDamage));
    await tester.pumpAndSettle();
    expect(find.text(l10nId.reportCategoryLabel), findsOneWidget);
    // Tombol tambah foto (opsional, M8 image_picker) tersedia.
    expect(
      find.byKey(const ValueKey<String>('report-add-photo')),
      findsOneWidget,
    );

    await tester.tap(find.byType(DropdownButtonFormField<String>));
    await tester.pumpAndSettle();
    await tester.tap(find.text('Layar Rusak').last);
    await tester.pumpAndSettle();

    await tester.tap(find.text(l10nId.reportSubmit));
    await tester.pumpAndSettle();

    verify(
      () => repository.reportDamage(
        assetId: 'asset-1',
        problemCategoryId: 'pc-1',
        description: any(named: 'description'),
      ),
    ).called(1);
    expect(find.text(l10nId.reportSuccess), findsOneWidget);
  });

  testWidgets('Lapor Kerusakan tanpa kategori: validasi menahan kirim', (
    WidgetTester tester,
  ) async {
    when(
      () => repository.problemCategories(),
    ).thenAnswer((_) async => const <ProblemCategory>[ProblemCategory('pc-1', 'Layar Rusak')]);

    await pumpBar(tester, permissions: <String>{'request.create'});
    await tester.tap(find.text(l10nId.assetActionReportDamage));
    await tester.pumpAndSettle();
    await tester.tap(find.text(l10nId.reportSubmit));
    await tester.pumpAndSettle();

    expect(find.text(l10nId.reportCategoryRequired), findsOneWidget);
    verifyNever(
      () => repository.reportDamage(
        assetId: any(named: 'assetId'),
        problemCategoryId: any(named: 'problemCategoryId'),
        description: any(named: 'description'),
      ),
    );
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
