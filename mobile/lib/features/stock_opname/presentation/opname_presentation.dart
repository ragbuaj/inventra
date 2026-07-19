import 'package:flutter/material.dart';
import 'package:intl/intl.dart';
import 'package:material_symbols_icons/symbols.dart';

import '../../../core/i18n/gen/app_localizations.dart';
import '../../../core/widgets/status_chip.dart';
import '../data/stock_opname_repository.dart';
import '../data/stock_opname_session_dto.dart';

/// Label + varian warna chip status sesi opname. Pemetaan keluarga semantik:
/// open = neutral (terjadwal), counting = success (berjalan, paritas mockup),
/// reconciling = info, closed = neutral.
(String, StatusChipVariant) opnameSessionStatusPresentation(
  AppLocalizations l10n,
  String status,
) {
  return switch (status) {
    'open' => (l10n.opnameStatusOpen, StatusChipVariant.neutral),
    'counting' => (l10n.opnameStatusCounting, StatusChipVariant.success),
    'reconciling' => (l10n.opnameStatusReconciling, StatusChipVariant.info),
    'closed' => (l10n.opnameStatusClosed, StatusChipVariant.neutral),
    _ => (status, StatusChipVariant.neutral),
  };
}

/// Label + varian warna chip hasil item (mockup: Ditemukan hijau, Rusak amber,
/// Salah Lokasi biru; Tidak Ditemukan merah; pending slate).
(String, StatusChipVariant) opnameItemResultPresentation(
  AppLocalizations l10n,
  String result,
) {
  return switch (OpnameItemResult.tryParse(result)) {
    OpnameItemResult.found => (
      l10n.opnameResultFound,
      StatusChipVariant.success,
    ),
    OpnameItemResult.notFound => (
      l10n.opnameResultNotFound,
      StatusChipVariant.danger,
    ),
    OpnameItemResult.damaged => (
      l10n.opnameResultDamaged,
      StatusChipVariant.warning,
    ),
    OpnameItemResult.misplaced => (
      l10n.opnameResultMisplaced,
      StatusChipVariant.info,
    ),
    OpnameItemResult.pending ||
    null => (l10n.opnameResultPending, StatusChipVariant.neutral),
  };
}

/// Ikon hasil item (mockup: check_circle/help/build/wrong_location).
IconData opnameItemResultIcon(String result) {
  return switch (OpnameItemResult.tryParse(result)) {
    OpnameItemResult.found => Symbols.check_circle_rounded,
    OpnameItemResult.notFound => Symbols.help_rounded,
    OpnameItemResult.damaged => Symbols.build_rounded,
    OpnameItemResult.misplaced => Symbols.wrong_location_rounded,
    OpnameItemResult.pending || null => Symbols.schedule_rounded,
  };
}

/// Judul kartu/header sesi: `name` dari kontrak; fallback nama kantor bila
/// null (name nullable di kontrak).
String opnameSessionTitle(StockOpnameSessionDto session) =>
    session.name ?? session.officeName ?? session.id;

/// Subjudul sesi: "kantor · periode" (periode format bulan penuh locale,
/// mis. "Juli 2026" — kontrak menormalkan period ke tanggal 1 tiap bulan).
String opnameSessionSubtitle(StockOpnameSessionDto session, String localeName) {
  final DateTime? period = session.period;
  return <String>[
    if (session.officeName != null) session.officeName!,
    if (period != null) DateFormat.yMMMM(localeName).format(period),
  ].join(' · ');
}

/// Jumlah item terhitung dari KPI detail sesi (total - pending); null bila
/// KPI tidak dikirim (respons daftar).
int? opnameCountedOf(StockOpnameSessionDto session) {
  final int? total = session.total;
  final int? pending = session.pending;
  if (total == null || pending == null) {
    return null;
  }
  return total - pending;
}

/// Jam hitung item untuk baris "Baru saja dipindai" (mockup "09.40" — pola
/// jam-menit locale; id memakai titik).
String opnameCountedTime(DateTime countedAt, String localeName) =>
    DateFormat.Hm(localeName).format(countedAt.toLocal());
