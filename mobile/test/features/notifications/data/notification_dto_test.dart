import 'package:flutter_test/flutter_test.dart';
import 'package:inventra_mobile/features/notifications/data/notification_dto.dart';
import 'package:inventra_mobile/features/notifications/data/notification_list_dto.dart';

/// JSON `Notification` lengkap persis contoh kontrak (approval_pending).
const Map<String, dynamic> fullNotificationJson = <String, dynamic>{
  'id': 'notif-1',
  'type': 'approval_pending',
  'params': <String, dynamic>{'request_type': 'asset_disposal', 'step': '1'},
  'entity_type': 'requests',
  'entity_id': 'req-1',
  'read_at': null,
  'created_at': '2026-07-19T02:31:00Z',
};

/// JSON `Notification` sudah dibaca dengan entity nullable terisi null.
const Map<String, dynamic> readNotificationJson = <String, dynamic>{
  'id': 'notif-2',
  'type': 'maintenance_due',
  'params': <String, dynamic>{
    'asset_tag': 'JKT01-ELK-2024-00031',
    'asset_name': 'AC Ruang Server',
    'due_date': '2026-07-25',
  },
  'entity_type': null,
  'entity_id': null,
  'read_at': '2026-07-18T09:00:00Z',
  'created_at': '2026-07-18T06:00:00Z',
};

void main() {
  group('NotificationDto', () {
    test('parse JSON lengkap: field kontrak terpetakan snake_case', () {
      final NotificationDto dto = NotificationDto.fromJson(
        fullNotificationJson,
      );

      expect(dto.id, 'notif-1');
      expect(dto.type, 'approval_pending');
      expect(dto.params, <String, dynamic>{
        'request_type': 'asset_disposal',
        'step': '1',
      });
      expect(dto.entityType, 'requests');
      expect(dto.entityId, 'req-1');
      expect(dto.readAt, isNull);
      expect(dto.createdAt, DateTime.utc(2026, 7, 19, 2, 31));
    });

    test('parse notifikasi terbaca: read_at terisi, entity null aman', () {
      final NotificationDto dto = NotificationDto.fromJson(
        readNotificationJson,
      );

      expect(dto.readAt, DateTime.utc(2026, 7, 18, 9));
      expect(dto.entityType, isNull);
      expect(dto.entityId, isNull);
    });

    test('params absen di JSON: default map kosong, bukan error', () {
      final NotificationDto dto = NotificationDto.fromJson(<String, dynamic>{
        'id': 'notif-3',
        'type': 'future_type',
        'created_at': '2026-07-19T02:31:00Z',
      });

      expect(dto.params, isEmpty);
      expect(dto.type, 'future_type');
    });
  });

  group('NotificationListDto', () {
    test('parse halaman: data + total/limit/offset', () {
      final NotificationListDto page = NotificationListDto.fromJson(
        <String, dynamic>{
          'data': <Map<String, dynamic>>[
            fullNotificationJson,
            readNotificationJson,
          ],
          'total': 25,
          'limit': 20,
          'offset': 0,
        },
      );

      expect(page.data, hasLength(2));
      expect(page.data.first.id, 'notif-1');
      expect(page.total, 25);
      expect(page.limit, 20);
      expect(page.offset, 0);
    });

    test('data absen: default list kosong', () {
      final NotificationListDto page = NotificationListDto.fromJson(
        <String, dynamic>{'total': 0, 'limit': 20, 'offset': 0},
      );

      expect(page.data, isEmpty);
    });
  });
}
