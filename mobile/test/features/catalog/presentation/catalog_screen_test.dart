import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:inventra_mobile/core/api/app_failure.dart';
import 'package:inventra_mobile/core/masterdata/reference_lookup_repository.dart';
import 'package:inventra_mobile/core/widgets/app_skeleton.dart';
import 'package:inventra_mobile/core/widgets/status_chip.dart';
import 'package:inventra_mobile/features/asset_detail/data/asset_dto.dart';
import 'package:inventra_mobile/features/catalog/data/asset_list_dto.dart';
import 'package:inventra_mobile/features/catalog/data/catalog_repository.dart';
import 'package:inventra_mobile/features/catalog/presentation/catalog_screen.dart';
import 'package:mocktail/mocktail.dart';

import '../../../helpers/fake_reference_lookup.dart';
import '../../../helpers/test_app.dart';

class _MockCatalogRepository extends Mock implements CatalogRepository {}

AssetDto _asset({
  required String id,
  String? tag = 'JKT01-ELK-2026-00001',
  String? name = 'Laptop Dell Latitude 5440',
  String? status = 'available',
  String? officeId = 'off-1',
}) {
  return AssetDto(
    id: id,
    assetTag: tag,
    name: name,
    status: status,
    officeId: officeId,
  );
}

AssetListDto _page(List<AssetDto> items, {int? total, int offset = 0}) {
  return AssetListDto(
    data: items,
    total: total ?? items.length,
    limit: 20,
    offset: offset,
  );
}

void main() {
  late _MockCatalogRepository repository;

  setUp(() {
    repository = _MockCatalogRepository();
  });

  ProviderContainer createContainer() {
    return ProviderContainer.test(
      overrides: [
        catalogRepositoryProvider.overrideWithValue(repository),
        referenceLookupRepositoryProvider.overrideWithValue(
          FakeReferenceLookup(<String, String>{
            'office:off-1': 'Cabang Jakarta Selatan',
          }),
        ),
      ],
    );
  }

  /// Stub satu halaman untuk (search, offset) tertentu.
  void stubList(AssetListDto page, {String? search, int offset = 0}) {
    when(
      () => repository.list(
        search: search,
        offset: offset,
        limit: any(named: 'limit'),
      ),
    ).thenAnswer((_) async => page);
  }

  Future<void> pumpCatalog(WidgetTester tester) async {
    tester.view.physicalSize = const Size(500, 1600);
    tester.view.devicePixelRatio = 1.0;
    addTearDown(tester.view.reset);
    await tester.pumpWidget(
      buildScreenHarness(
        container: createContainer(),
        child: const CatalogScreen(),
      ),
    );
  }

  group('state data', () {
    testWidgets('kartu: nama + kode + chip status + nama kantor', (
      WidgetTester tester,
    ) async {
      stubList(_page(<AssetDto>[_asset(id: 'a1')]));
      await pumpCatalog(tester);
      await tester.pumpAndSettle();

      expect(find.text(l10nId.catalogTitle), findsOneWidget);
      expect(find.text('Laptop Dell Latitude 5440'), findsOneWidget);
      expect(find.text('JKT01-ELK-2026-00001'), findsOneWidget);
      expect(
        find.descendant(
          of: find.byType(StatusChip),
          matching: find.text(l10nId.assetDetailStatusAvailable),
        ),
        findsOneWidget,
      );
      expect(find.text('Cabang Jakarta Selatan'), findsOneWidget);
    });

    testWidgets('nama aset dimask: fallback "Aset tanpa nama"', (
      WidgetTester tester,
    ) async {
      stubList(_page(<AssetDto>[_asset(id: 'a1', name: null)]));
      await pumpCatalog(tester);
      await tester.pumpAndSettle();

      expect(find.text(l10nId.catalogUnnamedAsset), findsOneWidget);
    });

    testWidgets('status Dipinjam: chip varian info', (
      WidgetTester tester,
    ) async {
      stubList(_page(<AssetDto>[_asset(id: 'a1', status: 'assigned')]));
      await pumpCatalog(tester);
      await tester.pumpAndSettle();

      expect(find.text(l10nId.assetDetailStatusAssigned), findsOneWidget);
    });
  });

  group('empty', () {
    testWidgets('tanpa aset: empty state umum', (WidgetTester tester) async {
      stubList(_page(<AssetDto>[]));
      await pumpCatalog(tester);
      await tester.pumpAndSettle();

      expect(find.text(l10nId.catalogEmptyTitle), findsOneWidget);
      expect(find.text(l10nId.catalogResetFilter), findsNothing);
    });

    testWidgets('pencarian tak cocok: empty state + Reset mengosongkan', (
      WidgetTester tester,
    ) async {
      stubList(_page(<AssetDto>[_asset(id: 'a1')]));
      stubList(_page(<AssetDto>[]), search: 'zzz');
      await pumpCatalog(tester);
      await tester.pumpAndSettle();

      await tester.enterText(find.byType(TextField), 'zzz');
      await tester.pump(const Duration(milliseconds: 350));
      await tester.pumpAndSettle();

      expect(find.text(l10nId.catalogEmptySearchTitle), findsOneWidget);

      await tester.tap(find.text(l10nId.catalogResetFilter));
      await tester.pumpAndSettle();

      // Kembali ke daftar penuh (search dikosongkan).
      expect(find.text('Laptop Dell Latitude 5440'), findsOneWidget);
    });
  });

  group('pencarian', () {
    testWidgets('mengetik memuat ulang dengan parameter search (debounce)', (
      WidgetTester tester,
    ) async {
      stubList(_page(<AssetDto>[_asset(id: 'a1', name: 'Kursi Kerja')]));
      stubList(
        _page(<AssetDto>[_asset(id: 'a2', name: 'Laptop Asus')]),
        search: 'laptop',
      );
      await pumpCatalog(tester);
      await tester.pumpAndSettle();

      await tester.enterText(find.byType(TextField), 'laptop');
      await tester.pump(const Duration(milliseconds: 350));
      await tester.pumpAndSettle();

      expect(find.text('Laptop Asus'), findsOneWidget);
      expect(find.text('Kursi Kerja'), findsNothing);
      verify(
        () => repository.list(
          search: 'laptop',
          offset: 0,
          limit: any(named: 'limit'),
        ),
      ).called(1);
    });
  });

  group('loading dan error', () {
    testWidgets('loading: skeleton kartu tampil', (WidgetTester tester) async {
      when(
        () => repository.list(
          search: any(named: 'search'),
          offset: any(named: 'offset'),
          limit: any(named: 'limit'),
        ),
      ).thenAnswer((_) async {
        await Future<void>.delayed(const Duration(milliseconds: 50));
        return _page(<AssetDto>[]);
      });
      await pumpCatalog(tester);
      await tester.pump();

      expect(find.byType(AppSkeleton), findsWidgets);
      await tester.pumpAndSettle();
    });

    testWidgets('offline: pesan jaringan + retry memuat ulang', (
      WidgetTester tester,
    ) async {
      when(
        () => repository.list(
          search: any(named: 'search'),
          offset: any(named: 'offset'),
          limit: any(named: 'limit'),
        ),
      ).thenThrow(const NetworkFailure());
      await pumpCatalog(tester);
      await tester.pumpAndSettle();

      expect(find.text(l10nId.catalogErrorTitle), findsOneWidget);
      expect(find.text(l10nId.catalogErrorNetworkBody), findsOneWidget);

      stubList(_page(<AssetDto>[_asset(id: 'a1', name: 'Setelah retry')]));
      await tester.tap(find.text(l10nId.commonRetry));
      await tester.pumpAndSettle();

      expect(find.text('Setelah retry'), findsOneWidget);
    });

    testWidgets('403: pesan akses dibatasi tanpa tombol retry', (
      WidgetTester tester,
    ) async {
      when(
        () => repository.list(
          search: any(named: 'search'),
          offset: any(named: 'offset'),
          limit: any(named: 'limit'),
        ),
      ).thenThrow(const ForbiddenFailure());
      await pumpCatalog(tester);
      await tester.pumpAndSettle();

      expect(find.text(l10nId.catalogForbiddenTitle), findsOneWidget);
      expect(find.text(l10nId.commonRetry), findsNothing);
    });
  });

  group('infinite scroll', () {
    testWidgets('scroll ke bawah memuat halaman berikutnya (offset 20)', (
      WidgetTester tester,
    ) async {
      final List<AssetDto> firstPage = List<AssetDto>.generate(
        20,
        (int i) => _asset(id: 'a$i', name: 'Aset nomor $i', tag: 'TAG-$i'),
      );
      stubList(_page(firstPage, total: 25));
      stubList(
        _page(
          List<AssetDto>.generate(
            5,
            (int i) => _asset(id: 'b$i', name: 'Aset lanjutan $i', tag: 'B-$i'),
          ),
          total: 25,
          offset: 20,
        ),
        offset: 20,
      );
      await pumpCatalog(tester);
      await tester.pumpAndSettle();

      await tester.fling(find.byType(ListView), const Offset(0, -2400), 3000);
      await tester.pumpAndSettle();

      verify(
        () => repository.list(
          search: null,
          offset: 20,
          limit: any(named: 'limit'),
        ),
      ).called(1);

      await tester.fling(find.byType(ListView), const Offset(0, -2400), 3000);
      await tester.pumpAndSettle();
      expect(find.text('Aset lanjutan 4'), findsOneWidget);
    });
  });
}
