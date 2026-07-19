import 'package:flutter_test/flutter_test.dart';
import 'package:inventra_mobile/features/stock_opname/data/stock_opname_item_dto.dart';
import 'package:inventra_mobile/features/stock_opname/data/stock_opname_item_result_dto.dart';
import 'package:inventra_mobile/features/stock_opname/data/stock_opname_scan_result_dto.dart';
import 'package:inventra_mobile/features/stock_opname/data/stock_opname_session_dto.dart';
import 'package:inventra_mobile/features/stock_opname/data/stock_opname_session_list_dto.dart';

/// `StockOpnameSession` respons daftar (tanpa KPI) — bentuk persis kontrak.
final Map<String, dynamic> listSessionJson = <String, dynamic>{
  'id': 'op-1',
  'office_id': 'office-1',
  'name': 'Opname Tahunan 2026',
  'period': '2026-07-01',
  'status': 'counting',
  'started_by_id': 'user-1',
  'started_at': '2026-07-14T01:00:00Z',
  'closed_by_id': null,
  'closed_at': null,
  'office_name': 'Cabang Jakarta Selatan',
  'started_by_name': 'Dewi Lestari',
  'closed_by_name': null,
  'created_at': '2026-07-10T01:00:00Z',
  'updated_at': '2026-07-14T01:00:00Z',
};

/// `StockOpnameSession` respons get by id — dengan KPI counter.
final Map<String, dynamic> detailSessionJson = <String, dynamic>{
  ...listSessionJson,
  'total': 150,
  'found': 120,
  'pending': 22,
  'variance': 8,
};

/// `StockOpnameItem` lengkap — item snapshot yang sudah dihitung.
final Map<String, dynamic> fullItemJson = <String, dynamic>{
  'id': 'item-1',
  'session_id': 'op-1',
  'asset_id': 'asset-1',
  'asset_name': 'Monitor Dell U2723',
  'asset_tag': 'JKT01-ELK-2026-00014',
  'office_name': 'Cabang Jakarta Selatan',
  'room_name': 'R. Operasional',
  'floor_name': 'Lantai 2',
  'expected': true,
  'result': 'found',
  'note': 'Kondisi baik',
  'counted_by_name': 'Dewi Lestari',
  'counted_at': '2026-07-19T02:40:00Z',
  'followup_request_id': null,
  'followup_record_id': null,
};

void main() {
  group('StockOpnameSessionDto', () {
    test('parse respons daftar: field wajib + nullable tanpa KPI', () {
      final StockOpnameSessionDto dto = StockOpnameSessionDto.fromJson(
        listSessionJson,
      );

      expect(dto.id, 'op-1');
      expect(dto.officeId, 'office-1');
      expect(dto.name, 'Opname Tahunan 2026');
      // Tanggal polos tanpa offset di-parse sebagai waktu lokal.
      expect(dto.period, DateTime(2026, 7));
      expect(dto.status, 'counting');
      expect(dto.startedById, 'user-1');
      expect(dto.officeName, 'Cabang Jakarta Selatan');
      expect(dto.startedByName, 'Dewi Lestari');
      expect(dto.closedById, isNull);
      expect(dto.total, isNull);
      expect(dto.found, isNull);
      expect(dto.pending, isNull);
      expect(dto.variance, isNull);
    });

    test('parse respons detail: KPI counter terisi', () {
      final StockOpnameSessionDto dto = StockOpnameSessionDto.fromJson(
        detailSessionJson,
      );

      expect(dto.total, 150);
      expect(dto.found, 120);
      expect(dto.pending, 22);
      expect(dto.variance, 8);
    });

    test('name/period null (kontrak nullable) tetap ter-parse', () {
      final StockOpnameSessionDto dto = StockOpnameSessionDto.fromJson(
        <String, dynamic>{...listSessionJson, 'name': null, 'period': null},
      );

      expect(dto.name, isNull);
      expect(dto.period, isNull);
    });
  });

  test('StockOpnameSessionListDto parse halaman', () {
    final StockOpnameSessionListDto dto = StockOpnameSessionListDto.fromJson(
      <String, dynamic>{
        'data': <Map<String, dynamic>>[listSessionJson],
        'total': 3,
        'limit': 100,
        'offset': 0,
      },
    );

    expect(dto.data.single.id, 'op-1');
    expect(dto.total, 3);
    expect(dto.limit, 100);
    expect(dto.offset, 0);
  });

  group('StockOpnameItemDto', () {
    test('parse item lengkap', () {
      final StockOpnameItemDto dto = StockOpnameItemDto.fromJson(fullItemJson);

      expect(dto.id, 'item-1');
      expect(dto.sessionId, 'op-1');
      expect(dto.assetId, 'asset-1');
      expect(dto.assetName, 'Monitor Dell U2723');
      expect(dto.assetTag, 'JKT01-ELK-2026-00014');
      expect(dto.roomName, 'R. Operasional');
      expect(dto.floorName, 'Lantai 2');
      expect(dto.expected, isTrue);
      expect(dto.result, 'found');
      expect(dto.note, 'Kondisi baik');
      expect(dto.countedAt, DateTime.utc(2026, 7, 19, 2, 40));
      expect(dto.followupRequestId, isNull);
      expect(dto.followupRecordId, isNull);
    });

    test('temuan di luar snapshot: expected false + followup terisi', () {
      final StockOpnameItemDto dto =
          StockOpnameItemDto.fromJson(<String, dynamic>{
            ...fullItemJson,
            'expected': false,
            'result': 'not_found',
            'followup_request_id': 'req-9',
          });

      expect(dto.expected, isFalse);
      expect(dto.result, 'not_found');
      expect(dto.followupRequestId, 'req-9');
    });
  });

  test('StockOpnameScanResultDto parse respons scan', () {
    final StockOpnameScanResultDto dto =
        StockOpnameScanResultDto.fromJson(<String, dynamic>{
          'id': 'item-9',
          'session_id': 'op-1',
          'asset_id': 'asset-9',
          'expected': false,
          'result': 'pending',
        });

    expect(dto.id, 'item-9');
    expect(dto.sessionId, 'op-1');
    expect(dto.assetId, 'asset-9');
    expect(dto.expected, isFalse);
    expect(dto.result, 'pending');
  });

  test('StockOpnameItemResultDto parse respons PATCH hasil', () {
    final StockOpnameItemResultDto dto =
        StockOpnameItemResultDto.fromJson(<String, dynamic>{
          'id': 'item-1',
          'session_id': 'op-1',
          'asset_id': 'asset-1',
          'expected': true,
          'result': 'damaged',
          'note': 'Engsel patah',
          'counted_at': '2026-07-19T03:00:00Z',
        });

    expect(dto.result, 'damaged');
    expect(dto.note, 'Engsel patah');
    expect(dto.countedAt, DateTime.utc(2026, 7, 19, 3));
  });
}
