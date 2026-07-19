import 'package:flutter/material.dart';
import 'package:material_symbols_icons/symbols.dart';

import '../i18n/gen/app_localizations.dart';

/// AlertDialog konfirmasi Component Library: ikon dalam lingkaran, judul,
/// isi, lalu sepasang tombol Batal + aksi utama. [destructive] memakai warna
/// error (mockup "konfirmasi destruktif"); selain itu warna primary.
class ConfirmDialog extends StatelessWidget {
  const ConfirmDialog({
    required this.title,
    required this.message,
    required this.confirmLabel,
    this.cancelLabel,
    this.icon,
    this.destructive = false,
    super.key,
  });

  final String title;
  final String message;
  final String confirmLabel;

  /// Null memakai `commonCancel`.
  final String? cancelLabel;

  /// Null memakai ikon default per jenis (report untuk destruktif).
  final IconData? icon;

  final bool destructive;

  /// Menampilkan dialog; true bila aksi utama dipilih, false bila batal atau
  /// dialog ditutup lewat barrier.
  static Future<bool> show(
    BuildContext context, {
    required String title,
    required String message,
    required String confirmLabel,
    String? cancelLabel,
    IconData? icon,
    bool destructive = false,
  }) async {
    final bool? confirmed = await showDialog<bool>(
      context: context,
      builder: (BuildContext context) => ConfirmDialog(
        title: title,
        message: message,
        confirmLabel: confirmLabel,
        cancelLabel: cancelLabel,
        icon: icon,
        destructive: destructive,
      ),
    );
    return confirmed ?? false;
  }

  @override
  Widget build(BuildContext context) {
    final ThemeData theme = Theme.of(context);
    final ColorScheme scheme = theme.colorScheme;
    final AppLocalizations l10n = AppLocalizations.of(context);

    final Color iconBg = destructive
        ? scheme.errorContainer
        : scheme.primaryContainer;
    final Color iconColor = destructive ? scheme.error : scheme.primary;
    final IconData iconData =
        icon ?? (destructive ? Symbols.report_rounded : Symbols.help_rounded);

    return Dialog(
      shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(20)),
      child: Padding(
        padding: const EdgeInsets.all(22),
        child: Column(
          mainAxisSize: MainAxisSize.min,
          crossAxisAlignment: CrossAxisAlignment.start,
          children: <Widget>[
            Container(
              width: 48,
              height: 48,
              decoration: BoxDecoration(color: iconBg, shape: BoxShape.circle),
              child: Icon(iconData, size: 26, color: iconColor),
            ),
            const SizedBox(height: 14),
            Text(
              title,
              style: theme.textTheme.titleMedium?.copyWith(fontSize: 17),
            ),
            const SizedBox(height: 6),
            Text(
              message,
              style: TextStyle(
                fontSize: 13,
                height: 1.5,
                color: scheme.onSurfaceVariant,
              ),
            ),
            const SizedBox(height: 18),
            Row(
              children: <Widget>[
                Expanded(
                  child: OutlinedButton(
                    style: OutlinedButton.styleFrom(
                      minimumSize: const Size(0, 46),
                      side: BorderSide(color: scheme.outline, width: 1.5),
                      foregroundColor: theme.textTheme.labelLarge?.color,
                      shape: RoundedRectangleBorder(
                        borderRadius: BorderRadius.circular(12),
                      ),
                      // copyWith dari labelLarge mempertahankan font Inter.
                      textStyle: theme.textTheme.labelLarge?.copyWith(
                        fontSize: 14,
                        fontWeight: FontWeight.w600,
                      ),
                    ),
                    onPressed: () => Navigator.of(context).pop(false),
                    child: Text(cancelLabel ?? l10n.commonCancel),
                  ),
                ),
                const SizedBox(width: 10),
                Expanded(
                  child: FilledButton(
                    style: FilledButton.styleFrom(
                      minimumSize: const Size(0, 46),
                      backgroundColor: destructive
                          ? scheme.error
                          : scheme.primary,
                      foregroundColor: destructive
                          ? scheme.onError
                          : scheme.onPrimary,
                      shape: RoundedRectangleBorder(
                        borderRadius: BorderRadius.circular(12),
                      ),
                      textStyle: theme.textTheme.labelLarge?.copyWith(
                        fontSize: 14,
                        fontWeight: FontWeight.w700,
                      ),
                    ),
                    onPressed: () => Navigator.of(context).pop(true),
                    child: Text(confirmLabel),
                  ),
                ),
              ],
            ),
          ],
        ),
      ),
    );
  }
}
