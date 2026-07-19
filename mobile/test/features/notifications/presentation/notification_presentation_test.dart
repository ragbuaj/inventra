import 'package:flutter_test/flutter_test.dart';
import 'package:intl/date_symbol_data_local.dart';
import 'package:inventra_mobile/core/widgets/status_chip.dart';
import 'package:inventra_mobile/features/notifications/data/notification_dto.dart';
import 'package:inventra_mobile/features/notifications/presentation/notification_presentation.dart';
import 'package:material_symbols_icons/symbols.dart';

import '../../../helpers/test_app.dart';

NotificationDto _notification({
  String id = 'notif-1',
  String type = 'approval_pending',
  Map<String, dynamic> params = const <String, dynamic>{},
  String? entityType,
  String? entityId,
  DateTime? createdAt,
}) {
  return NotificationDto(
    id: id,
    type: type,
    params: params,
    entityType: entityType,
    entityId: entityId,
    createdAt: createdAt ?? DateTime.utc(2026, 7, 19, 2),
  );
}

void main() {
  setUpAll(() => initializeDateFormatting('id'));

  group('judul + isi per type (ADR-0014: dirender klien dari params)', () {
    test('approval_pending: judul tetap + isi label jenis + langkah', () {
      final NotificationDto n = _notification(
        params: <String, dynamic>{
          'request_type': 'asset_disposal',
          'step': '2',
        },
      );

      expect(
        notificationTitle(l10nId, n),
        l10nId.notificationsApprovalPendingTitle,
      );
      expect(
        notificationBody(l10nId, n, 'id'),
        l10nId.notificationsApprovalPendingBody(
          l10nId.approvalTypeAssetDisposal,
          '2',
        ),
      );
      expect(notificationIcon(n), Symbols.approval_rounded);
      expect(notificationVariant(n), StatusChipVariant.warning);
    });

    test('approval_pending tanpa step: isi hanya label jenis', () {
      final NotificationDto n = _notification(
        params: <String, dynamic>{'request_type': 'assignment'},
      );

      expect(notificationBody(l10nId, n, 'id'), l10nId.approvalTypeAssignment);
    });

    test('approval_decided approved: judul disetujui, hijau, check', () {
      final NotificationDto n = _notification(
        type: 'approval_decided',
        params: <String, dynamic>{
          'request_type': 'asset_create',
          'status': 'approved',
        },
      );

      expect(
        notificationTitle(l10nId, n),
        l10nId.notificationsApprovalApprovedTitle,
      );
      expect(notificationBody(l10nId, n, 'id'), l10nId.approvalTypeAssetCreate);
      expect(notificationIcon(n), Symbols.check_circle_rounded);
      expect(notificationVariant(n), StatusChipVariant.success);
    });

    test('approval_decided rejected: judul ditolak, merah, cancel', () {
      final NotificationDto n = _notification(
        type: 'approval_decided',
        params: <String, dynamic>{
          'request_type': 'assignment',
          'status': 'rejected',
        },
      );

      expect(
        notificationTitle(l10nId, n),
        l10nId.notificationsApprovalRejectedTitle,
      );
      expect(notificationIcon(n), Symbols.cancel_rounded);
      expect(notificationVariant(n), StatusChipVariant.danger);
    });

    test('approval_decided status asing: judul fallback netral', () {
      final NotificationDto n = _notification(
        type: 'approval_decided',
        params: <String, dynamic>{
          'request_type': 'assignment',
          'status': 'escalated',
        },
      );

      expect(
        notificationTitle(l10nId, n),
        l10nId.notificationsApprovalDecidedTitle,
      );
      expect(notificationVariant(n), StatusChipVariant.neutral);
    });

    test('maintenance_due: isi "aset (tag) — jatuh tempo tanggal"', () {
      final NotificationDto n = _notification(
        type: 'maintenance_due',
        params: <String, dynamic>{
          'asset_tag': 'JKT01-ELK-2024-00031',
          'asset_name': 'AC Ruang Server',
          'due_date': '2026-07-25',
        },
      );

      expect(
        notificationTitle(l10nId, n),
        l10nId.notificationsMaintenanceDueTitle,
      );
      expect(
        notificationBody(l10nId, n, 'id'),
        l10nId.notificationsMaintenanceDueBody(
          'AC Ruang Server (JKT01-ELK-2024-00031)',
          '25 Jul 2026',
        ),
      );
      expect(notificationIcon(n), Symbols.build_rounded);
      expect(notificationVariant(n), StatusChipVariant.warning);
    });

    test('maintenance_due tanpa due_date: isi hanya label aset', () {
      final NotificationDto n = _notification(
        type: 'maintenance_due',
        params: <String, dynamic>{'asset_name': 'AC Ruang Server'},
      );

      expect(notificationBody(l10nId, n, 'id'), 'AC Ruang Server');
    });

    test('asset_returned: isi label aset dari params', () {
      final NotificationDto n = _notification(
        type: 'asset_returned',
        params: <String, dynamic>{
          'asset_tag': 'JKT01-ELK-2026-00001',
          'asset_name': 'Proyektor Epson EB-X500',
        },
      );

      expect(
        notificationTitle(l10nId, n),
        l10nId.notificationsAssetReturnedTitle,
      );
      expect(
        notificationBody(l10nId, n, 'id'),
        'Proyektor Epson EB-X500 (JKT01-ELK-2026-00001)',
      );
      expect(notificationIcon(n), Symbols.inventory_2_rounded);
      expect(notificationVariant(n), StatusChipVariant.info);
    });

    test('type asing: judul nilai kawat apa adanya, netral, tanpa isi', () {
      final NotificationDto n = _notification(
        type: 'future_type',
        params: <String, dynamic>{'anything': 'x'},
      );

      expect(notificationTitle(l10nId, n), 'future_type');
      expect(notificationBody(l10nId, n, 'id'), isNull);
      expect(notificationIcon(n), Symbols.notifications_rounded);
      expect(notificationVariant(n), StatusChipVariant.neutral);
    });

    test('params kosong: isi null, judul tetap dirender', () {
      final NotificationDto n = _notification();

      expect(
        notificationTitle(l10nId, n),
        l10nId.notificationsApprovalPendingTitle,
      );
      expect(notificationBody(l10nId, n, 'id'), isNull);
    });
  });

  group('target navigasi tap', () {
    test('entity requests menuju detail approval', () {
      final NotificationDto n = _notification(
        entityType: 'requests',
        entityId: 'req-9',
      );

      expect(notificationTargetLocation(n), '/approval/req-9');
    });

    test('entity assets menuju detail aset via params.asset_tag', () {
      final NotificationDto n = _notification(
        type: 'asset_returned',
        entityType: 'assets',
        entityId: 'asset-1',
        params: <String, dynamic>{'asset_tag': 'JKT01-ELK-2026-00001'},
      );

      expect(notificationTargetLocation(n), '/assets/JKT01-ELK-2026-00001');
    });

    test('entity assets TANPA asset_tag: tanpa target (id saja tak cukup)', () {
      final NotificationDto n = _notification(
        type: 'asset_returned',
        entityType: 'assets',
        entityId: 'asset-1',
      );

      expect(notificationTargetLocation(n), isNull);
    });

    test('entity null / type asing: tanpa target', () {
      expect(notificationTargetLocation(_notification()), isNull);
      expect(
        notificationTargetLocation(
          _notification(entityType: 'widgets', entityId: 'w-1'),
        ),
        isNull,
      );
    });
  });

  group('label waktu dan seksi (clock beku)', () {
    final DateTime now = DateTime(2026, 7, 19, 9, 41);

    test('hari ini: baru saja / menit / jam', () {
      expect(
        notificationTimeLabel(
          l10nId,
          now,
          DateTime(2026, 7, 19, 9, 40, 30),
          'id',
        ),
        l10nId.notificationsTimeJustNow,
      );
      expect(
        notificationTimeLabel(l10nId, now, DateTime(2026, 7, 19, 9, 31), 'id'),
        l10nId.notificationsTimeMinutesAgo(10),
      );
      expect(
        notificationTimeLabel(l10nId, now, DateTime(2026, 7, 19, 6, 41), 'id'),
        l10nId.notificationsTimeHoursAgo(3),
      );
    });

    test('kemarin: "Kemarin, {jam}"', () {
      expect(
        notificationTimeLabel(l10nId, now, DateTime(2026, 7, 18, 16, 40), 'id'),
        l10nId.notificationsTimeYesterdayAt('16.40'),
      );
    });

    test('lebih lama: "{tanggal pendek}, {jam}"', () {
      expect(
        notificationTimeLabel(l10nId, now, DateTime(2026, 7, 16, 9, 15), 'id'),
        l10nId.notificationsTimeAt('16 Jul', '09.15'),
      );
    });

    test('seksi: Hari ini / Kemarin / tanggal penuh', () {
      expect(
        notificationSectionLabel(l10nId, now, DateTime(2026, 7, 19, 1), 'id'),
        l10nId.notificationsSectionToday,
      );
      expect(
        notificationSectionLabel(l10nId, now, DateTime(2026, 7, 18, 23), 'id'),
        l10nId.notificationsSectionYesterday,
      );
      expect(
        notificationSectionLabel(l10nId, now, DateTime(2026, 7, 16, 9), 'id'),
        '16 Jul 2026',
      );
    });
  });
}
