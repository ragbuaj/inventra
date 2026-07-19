/// Pemetaan presentasi `SessionView` (device_type/browser/os/last_seen) ke
/// ikon, judul baris, dan label waktu relatif — dipakai kartu Sesi Perangkat
/// layar Profil. Nilai `device_type` di luar enum kontrak jatuh ke ikon
/// generik (klien tidak menebak makna nilai baru).
library;

import 'package:flutter/material.dart';
import 'package:intl/intl.dart';
import 'package:material_symbols_icons/symbols.dart';

import '../../../core/i18n/gen/app_localizations.dart';
import '../data/session_dto.dart';

/// Ikon tile per `device_type` kontrak (desktop/mobile/tablet/unknown).
IconData sessionDeviceIcon(String deviceType) {
  return switch (deviceType) {
    'mobile' => Symbols.smartphone_rounded,
    'tablet' => Symbols.tablet_rounded,
    'desktop' => Symbols.laptop_windows_rounded,
    _ => Symbols.devices_rounded,
  };
}

/// Judul baris sesi: "OS · Browser" (mockup "Windows · Chrome"). Kontrak tidak
/// memuat model perangkat/nama aplikasi — bagian kosong dilewati.
String sessionTitle(SessionDto session) {
  return <String>[
    if (session.os.isNotEmpty) session.os,
    if (session.browser.isNotEmpty) session.browser,
  ].join(' · ');
}

/// Subjudul baris sesi: "lokasi · IP · waktu" — lokasi GeoIP bisa kosong
/// (kontrak), IP selalu ada; sesi ini memakai "aktif sekarang".
String sessionSubtitle(
  AppLocalizations l10n,
  SessionDto session,
  DateTime now,
  String localeName,
) {
  final String time = session.current
      ? l10n.accountSessionActiveNow
      : sessionRelativeTime(l10n, now, session.lastSeenAt, localeName);
  return <String>[
    if (session.location.isNotEmpty) session.location,
    if (session.ipAddress.isNotEmpty) session.ipAddress,
    time,
  ].join(' · ');
}

/// Waktu relatif last_seen ("2 jam lalu", "kemarin"); lebih dari 7 hari
/// memakai tanggal pendek — pola yang sama dengan kartu inbox approval.
String sessionRelativeTime(
  AppLocalizations l10n,
  DateTime now,
  DateTime time,
  String localeName,
) {
  final Duration diff = now.difference(time);
  if (diff.inMinutes < 1) {
    return l10n.accountTimeJustNow;
  }
  if (diff.inHours < 1) {
    return l10n.accountTimeMinutesAgo(diff.inMinutes);
  }
  if (diff.inHours < 24) {
    return l10n.accountTimeHoursAgo(diff.inHours);
  }
  if (diff.inHours < 48) {
    return l10n.accountTimeYesterday;
  }
  if (diff.inDays < 7) {
    return l10n.accountTimeDaysAgo(diff.inDays);
  }
  return DateFormat('d MMM y', localeName).format(time);
}
