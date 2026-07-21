import 'dart:async';

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:inventra_mobile/core/api/app_failure.dart';
import 'package:inventra_mobile/core/authz/permissions_provider.dart';
import 'package:inventra_mobile/core/masterdata/reference_lookup_repository.dart';
import 'package:inventra_mobile/core/widgets/app_skeleton.dart';
import 'package:inventra_mobile/core/widgets/status_chip.dart';
import 'package:inventra_mobile/features/asset_detail/data/asset_detail_repository.dart';
import 'package:inventra_mobile/features/asset_detail/data/asset_dto.dart';
import 'package:inventra_mobile/features/asset_detail/presentation/asset_detail_screen.dart';
import 'package:mocktail/mocktail.dart';

import '../../../helpers/fake_reference_lookup.dart';
import '../../../helpers/test_app.dart';

class _MockAssetDetailRepository extends Mock
    implements AssetDetailRepository {}

const String _tag = 'JKT01-ELK-2026-00001';

const AssetDto _fullAsset = AssetDto(
  id: 'a-1',
  assetTag: _tag,
  name: 'Laptop Dell Latitude 5440',
  categoryId: 'cat-elektronik',
  officeId: 'office-jaksel',
  brandId: 'brand-dell',
  modelId: 'model-latitude-5440',
  roomId: 'room-operasional',
  vendorId: 'vendor-mitra',
  currentHolderEmployeeId: 'emp-rina',
  status: 'available',
  assetClass: 'tangible',
  serialNumber: 'SN-11223344',
  purchaseDate: '2026-02-12',
  purchaseCost: '18750000.00',
  bookValue: '15312500.00',
  accumulatedDepreciation: '3437500.00',
);

const AssetDetailData _fullData = AssetDetailData(
  asset: _fullAsset,
  maskedFields: <String>{},
);

/// Varian field permission: nilai finansial tidak dikirim backend.
const AssetDetailData _restrictedData = AssetDetailData(
  asset: AssetDto(
    id: 'a-1',
    assetTag: _tag,
    name: 'Laptop Dell Latitude 5440',
    categoryId: 'cat-elektronik',
    officeId: 'office-jaksel',
    status: 'assigned',
    assetClass: 'tangible',
    serialNumber: 'SN-11223344',
    purchaseDate: '2026-02-12',
  ),
  maskedFields: <String>{
    'purchase_cost',
    'book_value',
    'accumulated_depreciation',
  },
);

/// Nama referensi ter-resolve untuk id pada [_fullAsset] (kunci fake:
/// `<jenis>:<id>`).
const Map<String, String> _resolvedNames = <String, String>{
  'office:office-jaksel': 'Cabang Jakarta Selatan',
  'room:room-operasional': 'Lantai 2 · Ruang Operasional',
  'employee:emp-rina': 'Rina Kusuma',
  'category:cat-elektronik': 'Elektronik — Laptop',
  'brand:brand-dell': 'Dell',
  'model:model-latitude-5440': 'Latitude 5440',
  'vendor:vendor-mitra': 'PT Mitra Teknologi Nusantara',
};

void main() {
  late _MockAssetDetailRepository repository;
  late FakeReferenceLookup lookup;

  setUp(() {
    repository = _MockAssetDetailRepository();
    lookup = FakeReferenceLookup(_resolvedNames);
  });

  Future<ProviderContainer> pumpDetail(WidgetTester tester) async {
    // Viewport tinggi supaya seluruh ListView (sampai seksi Nilai) terbangun
    // tanpa scroll — ListView lazy tidak membangun anak di luar layar.
    tester.view.physicalSize = const Size(500, 1600);
    tester.view.devicePixelRatio = 1.0;
    addTearDown(tester.view.reset);
    final ProviderContainer container = ProviderContainer.test(
      overrides: [
        assetDetailRepositoryProvider.overrideWithValue(repository),
        referenceLookupRepositoryProvider.overrideWithValue(lookup),
        // Detail Aset kini merender AssetActionBar (FR-M7) yang membaca
        // permissionsProvider; stub Set kosong -> tanpa aksi (read-only), tidak
        // memukul HTTP nyata.
        permissionsProvider.overrideWith((ref) async => const <String>{}),
      ],
    );
    await tester.pumpWidget(
      buildScreenHarness(
        container: container,
        child: const AssetDetailScreen(tag: _tag),
      ),
    );
    return container;
  }

  group('state data', () {
    testWidgets('detail penuh: header, chip status, seluruh seksi', (
      WidgetTester tester,
    ) async {
      when(() => repository.getByTag(_tag)).thenAnswer((_) async => _fullData);
      await pumpDetail(tester);
      await tester.pumpAndSettle();

      expect(find.text(l10nId.assetDetailTitle), findsOneWidget);
      expect(find.text('Laptop Dell Latitude 5440'), findsOneWidget);
      expect(find.text(_tag), findsOneWidget);
      // Chip status available -> "Tersedia" varian success.
      expect(find.text(l10nId.assetDetailStatusAvailable), findsOneWidget);
      expect(find.byType(StatusChip), findsOneWidget);
      // Judul seksi (uppercase dari kunci ARB).
      expect(
        find.text(l10nId.assetDetailSectionPlacement.toUpperCase()),
        findsOneWidget,
      );
      expect(
        find.text(l10nId.assetDetailSectionInfo.toUpperCase()),
        findsOneWidget,
      );
      expect(
        find.text(l10nId.assetDetailSectionValue.toUpperCase()),
        findsOneWidget,
      );
      // Nilai referensi = NAMA hasil lookup master data; UUID/id mentah tidak
      // pernah tampil. Nilai literal (serial/tanggal/rupiah) diformat.
      expect(find.text('Cabang Jakarta Selatan'), findsOneWidget);
      expect(find.text('Lantai 2 · Ruang Operasional'), findsOneWidget);
      expect(find.text('Rina Kusuma'), findsOneWidget);
      expect(find.text('Elektronik — Laptop'), findsOneWidget);
      expect(find.text('Dell · Latitude 5440'), findsOneWidget);
      expect(find.text('PT Mitra Teknologi Nusantara'), findsOneWidget);
      expect(find.text('office-jaksel'), findsNothing);
      expect(find.text('cat-elektronik'), findsNothing);
      expect(find.text('SN-11223344'), findsOneWidget);
      expect(find.text('12 Feb 2026'), findsOneWidget);
      expect(find.textContaining('18.750.000'), findsOneWidget);
      expect(find.textContaining('15.312.500'), findsOneWidget);
      // Tidak ada penanda dibatasi.
      expect(find.text(l10nId.assetDetailRestrictedBadge), findsNothing);
      expect(find.byTooltip(l10nId.assetDetailRestrictedTooltip), findsNothing);
    });

    testWidgets('field dikirim null dirender em-dash TANPA penanda dibatasi', (
      WidgetTester tester,
    ) async {
      // Pemegang & ruangan null (aset tak dipegang siapa pun) — bukan mask.
      const AssetDetailData data = AssetDetailData(
        asset: AssetDto(
          assetTag: _tag,
          name: 'Laptop Dell Latitude 5440',
          categoryId: 'cat-elektronik',
          officeId: 'office-jaksel',
          status: 'available',
        ),
        maskedFields: <String>{},
      );
      when(() => repository.getByTag(_tag)).thenAnswer((_) async => data);
      await pumpDetail(tester);
      await tester.pumpAndSettle();

      expect(find.text('—'), findsWidgets);
      expect(find.byTooltip(l10nId.assetDetailRestrictedTooltip), findsNothing);
    });

    testWidgets(
      'lookup nama gagal (non-fatal): em-dash tanpa UUID, layar tetap hidup',
      (WidgetTester tester) async {
        // Fake tanpa peta nama = semua lookup menghasilkan null (perilaku
        // repository saat offline/403/404 — non-fatal by design).
        lookup = FakeReferenceLookup();
        when(
          () => repository.getByTag(_tag),
        ).thenAnswer((_) async => _fullData);
        await pumpDetail(tester);
        await tester.pumpAndSettle();

        // Layar tetap hidup dengan data non-referensi.
        expect(find.text('Laptop Dell Latitude 5440'), findsOneWidget);
        expect(find.text('SN-11223344'), findsOneWidget);
        expect(find.textContaining('18.750.000'), findsOneWidget);
        // Sel referensi em-dash — UUID mentah tidak pernah tampil.
        expect(find.text('—'), findsWidgets);
        expect(find.text('office-jaksel'), findsNothing);
        expect(find.text('cat-elektronik'), findsNothing);
        expect(find.text('room-operasional'), findsNothing);
        expect(find.text('emp-rina'), findsNothing);
        expect(find.text('vendor-mitra'), findsNothing);
        // Bukan mask: tanpa penanda dibatasi.
        expect(
          find.byTooltip(l10nId.assetDetailRestrictedTooltip),
          findsNothing,
        );
      },
    );

    testWidgets('field dibatasi: badge seksi + gembok + em-dash + tooltip', (
      WidgetTester tester,
    ) async {
      when(
        () => repository.getByTag(_tag),
      ).thenAnswer((_) async => _restrictedData);
      await pumpDetail(tester);
      await tester.pumpAndSettle();

      expect(find.text(l10nId.assetDetailRestrictedBadge), findsOneWidget);
      // Harga beli + nilai buku dimask -> dua penanda gembok ber-tooltip.
      expect(
        find.byTooltip(l10nId.assetDetailRestrictedTooltip),
        findsNWidgets(2),
      );
      expect(find.text('—'), findsWidgets);
      // Status assigned -> "Dipinjam".
      expect(find.text(l10nId.assetDetailStatusAssigned), findsOneWidget);
    });
  });

  testWidgets('state loading menampilkan skeleton', (
    WidgetTester tester,
  ) async {
    final Completer<AssetDetailData> gate = Completer<AssetDetailData>();
    when(() => repository.getByTag(_tag)).thenAnswer((_) => gate.future);
    await pumpDetail(tester);
    await tester.pump();

    expect(find.byType(AppSkeleton), findsWidgets);

    gate.complete(_fullData);
    await tester.pumpAndSettle();
    expect(find.byType(AppSkeleton), findsNothing);
    expect(find.text('Laptop Dell Latitude 5440'), findsOneWidget);
  });

  group('state error', () {
    testWidgets('offline: pesan jaringan + retry memuat ulang', (
      WidgetTester tester,
    ) async {
      when(() => repository.getByTag(_tag)).thenThrow(const NetworkFailure());
      await pumpDetail(tester);
      await tester.pumpAndSettle();

      expect(find.text(l10nId.assetDetailErrorTitle), findsOneWidget);
      expect(find.text(l10nId.assetDetailErrorNetworkBody), findsOneWidget);

      // Retry: kegagalan berikutnya sukses.
      when(() => repository.getByTag(_tag)).thenAnswer((_) async => _fullData);
      await tester.tap(find.text(l10nId.commonRetry));
      await tester.pumpAndSettle();

      expect(find.text('Laptop Dell Latitude 5440'), findsOneWidget);
      verify(() => repository.getByTag(_tag)).called(2);
    });

    testWidgets('kegagalan lain: pesan generik + retry', (
      WidgetTester tester,
    ) async {
      when(() => repository.getByTag(_tag)).thenThrow(const ServerFailure());
      await pumpDetail(tester);
      await tester.pumpAndSettle();

      expect(find.text(l10nId.assetDetailErrorTitle), findsOneWidget);
      expect(find.text(l10nId.assetDetailErrorGenericBody), findsOneWidget);
      expect(find.text(l10nId.commonRetry), findsOneWidget);
    });

    testWidgets('403: state akses dibatasi tanpa retry', (
      WidgetTester tester,
    ) async {
      when(() => repository.getByTag(_tag)).thenThrow(const ForbiddenFailure());
      await pumpDetail(tester);
      await tester.pumpAndSettle();

      expect(find.text(l10nId.assetDetailForbiddenTitle), findsOneWidget);
      expect(find.text(l10nId.assetDetailForbiddenBody), findsOneWidget);
      expect(find.text(l10nId.commonRetry), findsNothing);
    });

    testWidgets('404: empty state khusus dengan tag + aksi pindai lagi', (
      WidgetTester tester,
    ) async {
      when(() => repository.getByTag(_tag)).thenThrow(const NotFoundFailure());
      await pumpDetail(tester);
      await tester.pumpAndSettle();

      expect(find.text(l10nId.assetDetailNotFoundTitle), findsOneWidget);
      expect(find.text(l10nId.assetDetailNotFoundBody(_tag)), findsOneWidget);
      expect(find.text(l10nId.assetDetailScanAgain), findsOneWidget);
    });
  });
}
