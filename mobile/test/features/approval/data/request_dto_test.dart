import 'package:flutter_test/flutter_test.dart';
import 'package:inventra_mobile/features/approval/data/request_detail_dto.dart';
import 'package:inventra_mobile/features/approval/data/request_dto.dart';
import 'package:inventra_mobile/features/approval/data/request_list_dto.dart';

/// Fixture item `Request` lengkap sesuai skema openapi (dipakai juga tes
/// repository).
const Map<String, dynamic> fullRequestJson = <String, dynamic>{
  'id': 'req-1',
  'type': 'asset_transfer',
  'status': 'pending',
  'amount': '154800000.00',
  'current_step': 2,
  'office_id': 'office-jaksel',
  'target_id': 'asset-1',
  'target_entity': 'assets',
  'reason': 'Mutasi Laptop Dell Latitude 5440 ke KCP Kebayoran Baru',
  'requested_by_id': 'user-maker',
  'requested_by_name': 'Dewi Lestari',
  'requested_by_role': 'Staf Umum',
  'office_name': 'Cabang Jakarta Selatan',
  'decided_by_id': null,
  'decision_note': null,
  'created_at': '2026-07-18T08:12:00Z',
};

/// Fixture `RequestDetail`: Request + payload mutasi + dua step.
final Map<String, dynamic> fullRequestDetailJson = <String, dynamic>{
  ...fullRequestJson,
  'payload': <String, dynamic>{
    'from_office_id': 'office-jaksel',
    'to_office_id': 'office-kebbaru',
    'to_room_id': 'room-layanan',
    'reason': 'Kebutuhan perangkat teller baru per Agustus 2026.',
    'condition_sent': 'good',
    'transfer_date': '2026-08-01',
  },
  'steps': <Map<String, dynamic>>[
    <String, dynamic>{
      'step_order': 1,
      'required_level': 'office',
      'approver_id': 'user-siti',
      'approver_name': 'Siti Rahayu',
      'decision': 'approved',
      'note': 'OK',
      'decided_at': '2026-07-18T10:00:00Z',
    },
    <String, dynamic>{
      'step_order': 2,
      'required_level': 'wilayah',
      'approver_id': null,
      'approver_name': 'Hendra Gunawan',
      'decision': 'pending',
      'note': null,
      'decided_at': null,
    },
  ],
};

void main() {
  group('RequestDto', () {
    test('parse JSON lengkap: field snake_case terpetakan persis', () {
      final RequestDto dto = RequestDto.fromJson(fullRequestJson);

      expect(dto.id, 'req-1');
      expect(dto.type, 'asset_transfer');
      expect(dto.status, 'pending');
      expect(dto.amount, '154800000.00');
      expect(dto.currentStep, 2);
      expect(dto.targetId, 'asset-1');
      expect(dto.targetEntity, 'assets');
      expect(dto.requestedById, 'user-maker');
      expect(dto.requestedByName, 'Dewi Lestari');
      expect(dto.requestedByRole, 'Staf Umum');
      expect(dto.officeName, 'Cabang Jakarta Selatan');
      expect(dto.decidedById, isNull);
      expect(dto.createdAt, DateTime.utc(2026, 7, 18, 8, 12));
    });

    test('amount/reason absen (dimask field permission): DTO null', () {
      final Map<String, dynamic> masked =
          Map<String, dynamic>.of(fullRequestJson)
            ..remove('amount')
            ..remove('reason');

      final RequestDto dto = RequestDto.fromJson(masked);

      expect(dto.amount, isNull);
      expect(dto.reason, isNull);
    });
  });

  group('RequestDetailDto', () {
    test('parse payload + steps', () {
      final RequestDetailDto dto = RequestDetailDto.fromJson(
        fullRequestDetailJson,
      );

      expect(dto.payload?['to_office_id'], 'office-kebbaru');
      expect(dto.steps, hasLength(2));
      expect(dto.steps.first.approverName, 'Siti Rahayu');
      expect(dto.steps.first.decision, 'approved');
      expect(dto.steps.first.decidedAt, DateTime.utc(2026, 7, 18, 10));
      expect(dto.steps.last.requiredLevel, 'wilayah');
      expect(dto.steps.last.decision, 'pending');
    });

    test('steps absen: default list kosong', () {
      final RequestDetailDto dto = RequestDetailDto.fromJson(fullRequestJson);

      expect(dto.steps, isEmpty);
      expect(dto.payload, isNull);
    });
  });

  group('RequestListDto', () {
    test('parse halaman list', () {
      final RequestListDto dto = RequestListDto.fromJson(<String, dynamic>{
        'data': <Map<String, dynamic>>[fullRequestJson],
        'total': 21,
        'limit': 20,
        'offset': 0,
      });

      expect(dto.data, hasLength(1));
      expect(dto.data.single.id, 'req-1');
      expect(dto.total, 21);
      expect(dto.limit, 20);
      expect(dto.offset, 0);
    });
  });
}
