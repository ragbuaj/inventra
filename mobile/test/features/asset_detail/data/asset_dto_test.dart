import 'package:flutter_test/flutter_test.dart';
import 'package:inventra_mobile/features/asset_detail/data/asset_dto.dart';

/// JSON lengkap sesuai skema `Asset` openapi.yaml (kunci snake_case persis).
const Map<String, dynamic> fullAssetJson = <String, dynamic>{
  'id': 'a0000000-0000-0000-0000-000000000001',
  'asset_tag': 'JKT01-ELK-2026-00001',
  'name': 'Laptop Dell Latitude 5440',
  'category_id': 'c0000000-0000-0000-0000-000000000001',
  'office_id': 'o0000000-0000-0000-0000-000000000001',
  'brand_id': 'b0000000-0000-0000-0000-000000000001',
  'model_id': 'm0000000-0000-0000-0000-000000000001',
  'room_id': 'r0000000-0000-0000-0000-000000000001',
  'unit_id': null,
  'vendor_id': 'v0000000-0000-0000-0000-000000000001',
  'current_holder_employee_id': 'e0000000-0000-0000-0000-000000000001',
  'created_by_id': null,
  'status': 'available',
  'asset_class': 'tangible',
  'serial_number': 'SN-11223344',
  'purchase_date': '2026-02-12',
  'purchase_cost': '18750000.00',
  'book_value': '15312500.00',
  'accumulated_depreciation': '3437500.00',
  'salvage_value': null,
  'impairment_loss': null,
  'po_number': 'PO-2026-041',
  'funding_source': 'capex',
  'warranty_expiry': '2028-02-12',
  'capitalized': true,
  'depreciation_method': 'straight_line',
  'useful_life_months': 48,
  'fiscal_group': null,
  'fiscal_life_months': null,
  'acquisition_bast_no': null,
  'excluded_from_valuation': false,
  'valuation_exclusion_reason': null,
  'notes': null,
  'created_at': '2026-02-12T08:00:00Z',
  'updated_at': '2026-07-01T08:00:00Z',
};

void main() {
  test('assetSchemaKeys memuat persis seluruh kunci JSON kontrak', () {
    expect(assetSchemaKeys.toSet(), fullAssetJson.keys.toSet());
  });

  test('fromJson memetakan seluruh field snake_case', () {
    final AssetDto dto = AssetDto.fromJson(fullAssetJson);

    expect(dto.id, 'a0000000-0000-0000-0000-000000000001');
    expect(dto.assetTag, 'JKT01-ELK-2026-00001');
    expect(dto.name, 'Laptop Dell Latitude 5440');
    expect(dto.categoryId, 'c0000000-0000-0000-0000-000000000001');
    expect(dto.officeId, 'o0000000-0000-0000-0000-000000000001');
    expect(dto.currentHolderEmployeeId, 'e0000000-0000-0000-0000-000000000001');
    expect(dto.status, 'available');
    expect(dto.assetClass, 'tangible');
    expect(dto.serialNumber, 'SN-11223344');
    expect(dto.purchaseDate, '2026-02-12');
    expect(dto.purchaseCost, '18750000.00');
    expect(dto.bookValue, '15312500.00');
    expect(dto.accumulatedDepreciation, '3437500.00');
    expect(dto.usefulLifeMonths, 48);
    expect(dto.capitalized, isTrue);
    expect(dto.createdAt, DateTime.utc(2026, 2, 12, 8));
    // Field dikirim null tetap null.
    expect(dto.unitId, isNull);
    expect(dto.notes, isNull);
  });

  test('field yang absen dari JSON (field permission) menjadi null', () {
    final Map<String, dynamic> masked = Map<String, dynamic>.of(fullAssetJson)
      ..remove('purchase_cost')
      ..remove('book_value')
      ..remove('accumulated_depreciation');

    final AssetDto dto = AssetDto.fromJson(masked);

    expect(dto.purchaseCost, isNull);
    expect(dto.bookValue, isNull);
    expect(dto.accumulatedDepreciation, isNull);
    // Field lain tidak terpengaruh.
    expect(dto.name, 'Laptop Dell Latitude 5440');
  });

  test('JSON minimum (hampir semua field dimask) tetap terparse', () {
    final AssetDto dto = AssetDto.fromJson(const <String, dynamic>{
      'asset_tag': 'JKT01-ELK-2026-00001',
    });

    expect(dto.assetTag, 'JKT01-ELK-2026-00001');
    expect(dto.id, isNull);
    expect(dto.name, isNull);
    expect(dto.status, isNull);
  });
}
