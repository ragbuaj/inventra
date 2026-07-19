import 'package:flutter/material.dart';
import 'package:material_symbols_icons/symbols.dart';

import '../../app/theme.dart';
import '../i18n/gen/app_localizations.dart';

/// Status sinkronisasi yang ditampilkan [SyncPill] (FR-M5.6: status antrean
/// selalu terlihat).
enum SyncPillStatus { synced, pending, syncing, failed, offline }

/// Pill status sync Component Library: ikon + label dalam pill berwarna
/// keluarga semantik [InventraStatusColors]. Ikon berputar saat [SyncPillStatus.syncing]
/// kecuali pengguna meminta animasi dikurangi.
class SyncPill extends StatefulWidget {
  const SyncPill({required this.status, this.pendingCount = 0, super.key});

  final SyncPillStatus status;

  /// Jumlah item antrean; hanya dipakai label [SyncPillStatus.pending].
  final int pendingCount;

  @override
  State<SyncPill> createState() => _SyncPillState();
}

class _SyncPillState extends State<SyncPill>
    with SingleTickerProviderStateMixin {
  late final AnimationController _spinController = AnimationController(
    vsync: this,
    duration: const Duration(milliseconds: 1200),
  );

  @override
  void dispose() {
    _spinController.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    final InventraStatusColors colors = Theme.of(
      context,
    ).extension<InventraStatusColors>()!;
    final AppLocalizations l10n = AppLocalizations.of(context);

    final (
      StatusColorSet set,
      IconData icon,
      String label,
    ) = switch (widget.status) {
      SyncPillStatus.synced => (
        colors.success,
        Symbols.cloud_done_rounded,
        l10n.commonSyncSynced,
      ),
      SyncPillStatus.pending => (
        colors.warning,
        Symbols.cloud_upload_rounded,
        l10n.commonSyncPending(widget.pendingCount),
      ),
      SyncPillStatus.syncing => (
        colors.info,
        Symbols.sync_rounded,
        l10n.commonSyncSyncing,
      ),
      SyncPillStatus.failed => (
        colors.danger,
        Symbols.sync_problem_rounded,
        l10n.commonSyncFailed,
      ),
      SyncPillStatus.offline => (
        colors.neutral,
        Symbols.cloud_off_rounded,
        l10n.commonSyncOffline,
      ),
    };

    final bool spinning =
        widget.status == SyncPillStatus.syncing &&
        !MediaQuery.disableAnimationsOf(context);
    if (spinning && !_spinController.isAnimating) {
      _spinController.repeat();
    } else if (!spinning && _spinController.isAnimating) {
      _spinController.stop();
    }

    Widget iconWidget = Icon(icon, size: 17, color: set.text);
    if (spinning) {
      iconWidget = RotationTransition(
        turns: _spinController,
        child: iconWidget,
      );
    }

    return Semantics(
      liveRegion: true,
      child: Container(
        padding: const EdgeInsets.symmetric(horizontal: 14, vertical: 7),
        decoration: ShapeDecoration(
          color: set.bg,
          shape: const StadiumBorder(),
        ),
        child: Row(
          mainAxisSize: MainAxisSize.min,
          children: <Widget>[
            iconWidget,
            const SizedBox(width: 7),
            Text(
              label,
              style: TextStyle(
                fontSize: 12,
                fontWeight: FontWeight.w600,
                color: set.text,
              ),
            ),
          ],
        ),
      ),
    );
  }
}
