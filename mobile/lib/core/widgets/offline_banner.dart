import 'package:flutter/material.dart';
import 'package:material_symbols_icons/symbols.dart';

import '../../app/theme.dart';
import '../i18n/gen/app_localizations.dart';

/// Banner offline slim (Component Library bagian Konektivitas): tenang dan
/// informatif, bukan alarm. Warna keluarga warning dari [InventraStatusColors].
class OfflineBanner extends StatelessWidget {
  const OfflineBanner({this.message, super.key});

  /// Null memakai teks default `commonOfflineBanner`.
  final String? message;

  @override
  Widget build(BuildContext context) {
    final InventraStatusColors colors = Theme.of(
      context,
    ).extension<InventraStatusColors>()!;
    final StatusColorSet warning = colors.warning;
    final String text =
        message ?? AppLocalizations.of(context).commonOfflineBanner;

    return Semantics(
      liveRegion: true,
      child: Container(
        padding: const EdgeInsets.symmetric(horizontal: 14, vertical: 10),
        decoration: BoxDecoration(
          color: warning.bg,
          borderRadius: BorderRadius.circular(12),
          border: Border.all(color: warning.dot.withValues(alpha: 0.35)),
        ),
        child: Row(
          children: <Widget>[
            Icon(Symbols.cloud_off_rounded, size: 20, color: warning.text),
            const SizedBox(width: 10),
            Expanded(
              child: Text(
                text,
                style: TextStyle(
                  fontSize: 13,
                  fontWeight: FontWeight.w600,
                  color: warning.text,
                ),
              ),
            ),
          ],
        ),
      ),
    );
  }
}
