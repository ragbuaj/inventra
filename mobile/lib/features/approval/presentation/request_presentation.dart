/// Pemetaan presentasi nilai kontrak `Request` (type/status) ke label i18n,
/// ikon, dan keluarga warna — dipakai bersama layar Inbox dan Detail Approval.
/// Nilai tak dikenal dirender apa adanya dengan varian netral (klien tidak
/// menebak makna nilai baru).
library;

import 'package:flutter/material.dart';
import 'package:intl/intl.dart';
import 'package:material_symbols_icons/symbols.dart';

import '../../../core/i18n/gen/app_localizations.dart';
// Label jenis pengajuan dan statusColorSetOf naik ke core sejak Task 11 —
// dipakai juga fitur notifications (core/i18n/request_type_label.dart dan
// core/widgets/status_chip.dart).
import '../../../core/i18n/request_type_label.dart';
import '../../../core/widgets/status_chip.dart';

/// Ikon jenis pengajuan (mockup Inbox Approval).
IconData requestTypeIcon(String type) {
  return switch (type) {
    'asset_create' => Symbols.add_box_rounded,
    'asset_disposal' => Symbols.delete_forever_rounded,
    'asset_transfer' => Symbols.swap_horiz_rounded,
    'assignment' => Symbols.handshake_rounded,
    'maintenance' => Symbols.build_rounded,
    'valuation_exclusion' => Symbols.price_change_rounded,
    _ => Symbols.description_rounded,
  };
}

/// Keluarga warna jenis pengajuan. Deviasi tercatat: mockup memakai indigo
/// untuk Peminjaman — tema tidak punya keluarga indigo, dipetakan ke info.
StatusChipVariant requestTypeVariant(String type) {
  return switch (type) {
    'asset_create' => StatusChipVariant.success,
    'asset_disposal' => StatusChipVariant.danger,
    'asset_transfer' => StatusChipVariant.info,
    'assignment' => StatusChipVariant.info,
    'maintenance' => StatusChipVariant.warning,
    'valuation_exclusion' => StatusChipVariant.warning,
    _ => StatusChipVariant.neutral,
  };
}

/// Jenis bertanda "sensitif" pada mockup (penghapusan & pengecualian valuasi).
bool isSensitiveRequestType(String type) =>
    type == 'asset_disposal' || type == 'valuation_exclusion';

/// Label + varian chip status pengajuan (enum `status` openapi).
(String, StatusChipVariant) requestStatusPresentation(
  AppLocalizations l10n,
  String status,
) {
  return switch (status) {
    'pending' => (l10n.approvalStatusPending, StatusChipVariant.warning),
    'approved' => (l10n.approvalStatusApproved, StatusChipVariant.success),
    'rejected' => (l10n.approvalStatusRejected, StatusChipVariant.danger),
    'cancelled' => (l10n.approvalStatusCancelled, StatusChipVariant.neutral),
    final String other => (other, StatusChipVariant.neutral),
  };
}

/// Judul pengajuan untuk kartu/heading. Kontrak `Request` tidak punya field
/// judul (deviasi tercatat terhadap judul naratif mockup): pakai `reason`
/// bila ada, selain itu label jenis.
String requestTitle(AppLocalizations l10n, String type, String? reason) {
  final String? trimmed = reason?.trim();
  if (trimmed != null && trimmed.isNotEmpty) {
    return trimmed;
  }
  return requestTypeLabel(l10n, type);
}

/// Rupiah tanpa desimal dari string desimal kontrak; string tak terparse
/// dikembalikan apa adanya.
String formatIdrAmount(String raw, String localeName) {
  final double? value = double.tryParse(raw);
  if (value == null) {
    return raw;
  }
  return NumberFormat.currency(
    locale: localeName,
    symbol: 'Rp ',
    decimalDigits: 0,
  ).format(value);
}

/// Tanggal pendek "d MMM y"; string non-tanggal dikembalikan apa adanya.
String formatShortDate(String raw, String localeName) {
  final DateTime? date = DateTime.tryParse(raw);
  if (date == null) {
    return raw;
  }
  return DateFormat('d MMM y', localeName).format(date);
}

/// Waktu relatif kartu inbox ("2 jam lalu", "kemarin"); lebih dari 7 hari
/// memakai tanggal pendek.
String formatRelativeTime(
  AppLocalizations l10n,
  DateTime now,
  DateTime time,
  String localeName,
) {
  final Duration diff = now.difference(time);
  if (diff.inMinutes < 1) {
    return l10n.approvalTimeJustNow;
  }
  if (diff.inHours < 1) {
    return l10n.approvalTimeMinutesAgo(diff.inMinutes);
  }
  if (diff.inHours < 24) {
    return l10n.approvalTimeHoursAgo(diff.inHours);
  }
  if (diff.inHours < 48) {
    return l10n.approvalTimeYesterday;
  }
  if (diff.inDays < 7) {
    return l10n.approvalTimeDaysAgo(diff.inDays);
  }
  return DateFormat('d MMM y', localeName).format(time);
}
