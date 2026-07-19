/// Pemetaan presentasi `Notification` kontrak (type + params) ke judul/isi
/// i18n, ikon, keluarga warna, target navigasi, dan label waktu — dipakai
/// layar Notifikasi. Server TIDAK mengirim kalimat jadi (ADR-0014): klien
/// merender kalimat dari `type` + `params`. Type di luar kontrak dirender apa
/// adanya dengan varian netral (klien tidak menebak makna nilai baru).
library;

import 'package:flutter/material.dart';
import 'package:intl/intl.dart';
import 'package:material_symbols_icons/symbols.dart';

import '../../../core/i18n/gen/app_localizations.dart';
// Peta label jenis pengajuan di core: params.request_type notifikasi memakai
// enum kawat yang sama dengan `Request.type` (kunci approvalType*).
import '../../../core/i18n/request_type_label.dart';
import '../../../core/widgets/status_chip.dart';
import '../data/notification_dto.dart';

/// Nilai `type` kontrak (`approval_pending|approval_decided|maintenance_due|
/// asset_returned`); [unknown] untuk nilai server yang lebih baru.
enum NotificationKind {
  approvalPending('approval_pending'),
  approvalDecided('approval_decided'),
  maintenanceDue('maintenance_due'),
  assetReturned('asset_returned'),
  unknown(null);

  const NotificationKind(this.wire);

  final String? wire;

  static NotificationKind parse(String type) {
    for (final NotificationKind kind in NotificationKind.values) {
      if (kind.wire == type) {
        return kind;
      }
    }
    return NotificationKind.unknown;
  }
}

/// Nilai params [key] bila berupa string tak kosong; params adalah objek
/// bebas dari server sehingga tiap kunci dibaca defensif.
String? notificationParam(NotificationDto notification, String key) {
  final Object? value = notification.params[key];
  if (value is String && value.isNotEmpty) {
    return value;
  }
  return null;
}

bool _isDecidedApproved(NotificationDto n) =>
    notificationParam(n, 'status') == 'approved';

bool _isDecidedRejected(NotificationDto n) =>
    notificationParam(n, 'status') == 'rejected';

/// Ikon lingkaran kartu (mockup: approval/check_circle/cancel/build/
/// inventory_2; lonceng untuk type asing).
IconData notificationIcon(NotificationDto notification) {
  return switch (NotificationKind.parse(notification.type)) {
    NotificationKind.approvalPending => Symbols.approval_rounded,
    NotificationKind.approvalDecided when _isDecidedRejected(notification) =>
      Symbols.cancel_rounded,
    NotificationKind.approvalDecided => Symbols.check_circle_rounded,
    NotificationKind.maintenanceDue => Symbols.build_rounded,
    NotificationKind.assetReturned => Symbols.inventory_2_rounded,
    NotificationKind.unknown => Symbols.notifications_rounded,
  };
}

/// Keluarga warna tile ikon (mockup: pending amber, disetujui hijau, ditolak
/// merah, maintenance amber, aset kembali biru).
StatusChipVariant notificationVariant(NotificationDto notification) {
  return switch (NotificationKind.parse(notification.type)) {
    NotificationKind.approvalPending => StatusChipVariant.warning,
    NotificationKind.approvalDecided when _isDecidedApproved(notification) =>
      StatusChipVariant.success,
    NotificationKind.approvalDecided when _isDecidedRejected(notification) =>
      StatusChipVariant.danger,
    NotificationKind.approvalDecided => StatusChipVariant.neutral,
    NotificationKind.maintenanceDue => StatusChipVariant.warning,
    NotificationKind.assetReturned => StatusChipVariant.info,
    NotificationKind.unknown => StatusChipVariant.neutral,
  };
}

/// Judul kartu. Type asing dirender apa adanya (nilai kawat), bukan ditebak.
String notificationTitle(AppLocalizations l10n, NotificationDto notification) {
  return switch (NotificationKind.parse(notification.type)) {
    NotificationKind.approvalPending => l10n.notificationsApprovalPendingTitle,
    NotificationKind.approvalDecided when _isDecidedApproved(notification) =>
      l10n.notificationsApprovalApprovedTitle,
    NotificationKind.approvalDecided when _isDecidedRejected(notification) =>
      l10n.notificationsApprovalRejectedTitle,
    NotificationKind.approvalDecided => l10n.notificationsApprovalDecidedTitle,
    NotificationKind.maintenanceDue => l10n.notificationsMaintenanceDueTitle,
    NotificationKind.assetReturned => l10n.notificationsAssetReturnedTitle,
    NotificationKind.unknown => notification.type,
  };
}

/// Label aset dari params: "nama (TAG)"; salah satunya absen berarti yang ada
/// saja; keduanya absen berarti null.
String? _assetLabel(NotificationDto notification) {
  final String? name = notificationParam(notification, 'asset_name');
  final String? tag = notificationParam(notification, 'asset_tag');
  if (name != null && tag != null) {
    return '$name ($tag)';
  }
  return name ?? tag;
}

/// Baris isi kartu, dirakit dari params. null berarti baris tidak dirender
/// (params tidak membawa data yang bisa ditampilkan).
String? notificationBody(
  AppLocalizations l10n,
  NotificationDto notification,
  String localeName,
) {
  switch (NotificationKind.parse(notification.type)) {
    case NotificationKind.approvalPending:
      final String? type = notificationParam(notification, 'request_type');
      final String? step = notificationParam(notification, 'step');
      if (type == null) {
        return null;
      }
      final String label = requestTypeLabel(l10n, type);
      if (step == null) {
        return label;
      }
      return l10n.notificationsApprovalPendingBody(label, step);
    case NotificationKind.approvalDecided:
      final String? type = notificationParam(notification, 'request_type');
      return type == null ? null : requestTypeLabel(l10n, type);
    case NotificationKind.maintenanceDue:
      final String? asset = _assetLabel(notification);
      final String? dueDate = notificationParam(notification, 'due_date');
      if (asset == null) {
        return dueDate == null
            ? null
            : l10n.notificationsMaintenanceDueDateOnly(
                _formatDueDate(dueDate, localeName),
              );
      }
      if (dueDate == null) {
        return asset;
      }
      return l10n.notificationsMaintenanceDueBody(
        asset,
        _formatDueDate(dueDate, localeName),
      );
    case NotificationKind.assetReturned:
      return _assetLabel(notification);
    case NotificationKind.unknown:
      return null;
  }
}

/// `due_date` params (`YYYY-MM-DD`) ke tanggal pendek locale; string tak
/// terparse dikembalikan apa adanya.
String _formatDueDate(String raw, String localeName) {
  final DateTime? date = DateTime.tryParse(raw);
  if (date == null) {
    return raw;
  }
  return DateFormat('d MMM y', localeName).format(date);
}

/// Lokasi rute in-app saat kartu di-tap, dari `entity_type`+`entity_id`
/// (deep-link push FCM sendiri baru masuk M3):
/// `requests` menuju detail approval; `assets` menuju detail aset via
/// `params.asset_tag` (rute by-tag — id saja tidak cukup). null berarti tap
/// hanya menandai dibaca.
String? notificationTargetLocation(NotificationDto notification) {
  final String? entityType = notification.entityType;
  final String? entityId = notification.entityId;
  if (entityType == 'requests' && entityId != null && entityId.isNotEmpty) {
    return '/approval/${Uri.encodeComponent(entityId)}';
  }
  if (entityType == 'assets') {
    final String? tag = notificationParam(notification, 'asset_tag');
    if (tag != null) {
      return '/assets/${Uri.encodeComponent(tag)}';
    }
  }
  return null;
}

bool _isSameDay(DateTime a, DateTime b) =>
    a.year == b.year && a.month == b.month && a.day == b.day;

/// Label seksi feed per hari (mockup: "Hari ini" / "Kemarin" / "16 Jul 2026").
String notificationSectionLabel(
  AppLocalizations l10n,
  DateTime now,
  DateTime createdAt,
  String localeName,
) {
  final DateTime local = createdAt.toLocal();
  if (_isSameDay(local, now)) {
    return l10n.notificationsSectionToday;
  }
  if (_isSameDay(local, now.subtract(const Duration(days: 1)))) {
    return l10n.notificationsSectionYesterday;
  }
  return DateFormat('d MMM y', localeName).format(local);
}

/// Label waktu kartu (mockup): hari ini relatif ("10 menit lalu"), kemarin
/// "Kemarin, 16.40", lebih lama "16 Jul, 09.15".
String notificationTimeLabel(
  AppLocalizations l10n,
  DateTime now,
  DateTime createdAt,
  String localeName,
) {
  final DateTime local = createdAt.toLocal();
  if (_isSameDay(local, now)) {
    final Duration diff = now.difference(local);
    if (diff.inMinutes < 1) {
      return l10n.notificationsTimeJustNow;
    }
    if (diff.inHours < 1) {
      return l10n.notificationsTimeMinutesAgo(diff.inMinutes);
    }
    return l10n.notificationsTimeHoursAgo(diff.inHours);
  }
  final String time = DateFormat.Hm(localeName).format(local);
  if (_isSameDay(local, now.subtract(const Duration(days: 1)))) {
    return l10n.notificationsTimeYesterdayAt(time);
  }
  return l10n.notificationsTimeAt(
    DateFormat('d MMM', localeName).format(local),
    time,
  );
}
