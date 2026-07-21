import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';
import 'package:intl/intl.dart';
import 'package:material_symbols_icons/symbols.dart';

import '../../../app/theme.dart';
import '../../../core/api/app_failure.dart';
import '../../../core/i18n/gen/app_localizations.dart';
import '../../../core/utils/clock.dart';
import '../../../core/widgets/app_skeleton.dart';
import '../../../core/widgets/empty_state.dart';
import '../../../core/widgets/status_chip.dart';
import '../data/my_assets_repository.dart';

/// Aset Saya (1:1 mockup "Inventra Mobile - Aset Saya"): daftar aset yang
/// sedang dipegang pengguna (`GET /assignments/mine?status=active`), read-only,
/// dengan penanda "Terlambat" bila melewati jatuh tempo. Menu tersendiri; tap
/// kartu membuka Detail Aset lewat tag.
class MyAssetsScreen extends ConsumerWidget {
  const MyAssetsScreen({super.key});

  Future<void> _refresh(WidgetRef ref) async {
    ref.invalidate(myAssetsProvider);
    try {
      await ref.read(myAssetsProvider.future);
    } on Object {
      // Kegagalan refresh tercermin sebagai state error daftar.
    }
  }

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final AppLocalizations l10n = AppLocalizations.of(context);
    final AsyncValue<List<MyAssignmentDto>> state = ref.watch(myAssetsProvider);

    return Scaffold(
      appBar: AppBar(title: Text(l10n.myAssetsTitle)),
      body: SafeArea(
        child: state.when(
          data: (List<MyAssignmentDto> items) => _MyAssetsList(
            items: items,
            onRefresh: () => _refresh(ref),
          ),
          loading: () => const _LoadingSkeleton(),
          error: (Object error, StackTrace stackTrace) => _ErrorState(
            failure: error,
            onRetry: () => ref.invalidate(myAssetsProvider),
          ),
        ),
      ),
    );
  }
}

class _MyAssetsList extends ConsumerWidget {
  const _MyAssetsList({required this.items, required this.onRefresh});

  final List<MyAssignmentDto> items;
  final Future<void> Function() onRefresh;

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final AppLocalizations l10n = AppLocalizations.of(context);

    if (items.isEmpty) {
      return EmptyState(
        icon: Symbols.inventory_2_rounded,
        title: l10n.myAssetsEmptyTitle,
        subtitle: l10n.myAssetsEmptyBody,
      );
    }

    final DateTime now = ref.watch(clockProvider)();
    final String localeName = Localizations.localeOf(context).languageCode;

    return RefreshIndicator(
      onRefresh: onRefresh,
      child: ListView.separated(
        physics: const AlwaysScrollableScrollPhysics(),
        padding: const EdgeInsets.fromLTRB(20, 8, 20, 24),
        itemCount: items.length + 1,
        separatorBuilder: (BuildContext context, int index) =>
            const SizedBox(height: 10),
        itemBuilder: (BuildContext context, int index) {
          if (index == 0) {
            return Padding(
              padding: const EdgeInsets.only(bottom: 4),
              child: Text(
                l10n.myAssetsCount(items.length),
                style: TextStyle(
                  fontSize: 12.5,
                  color: Theme.of(context).textTheme.labelMedium?.color,
                ),
              ),
            );
          }
          return _AssetCard(
            assignment: items[index - 1],
            now: now,
            localeName: localeName,
          );
        },
      ),
    );
  }
}

/// Kartu aset yang dipegang: nama, kode, chip "Dipinjam", dipinjam sejak +
/// jatuh tempo; penanda "Terlambat" merah bila lewat tempo. Tap membuka Detail
/// Aset lewat tag.
class _AssetCard extends StatelessWidget {
  const _AssetCard({
    required this.assignment,
    required this.now,
    required this.localeName,
  });

  final MyAssignmentDto assignment;
  final DateTime now;
  final String localeName;

  @override
  Widget build(BuildContext context) {
    final ThemeData theme = Theme.of(context);
    final ColorScheme scheme = theme.colorScheme;
    final AppLocalizations l10n = AppLocalizations.of(context);
    final String tag = assignment.assetTag;
    final bool overdue = _isOverdue(assignment.dueDate, now);
    final String? checkoutText = _formatDate(assignment.checkoutDate, localeName);
    final String? dueText = _formatDate(assignment.dueDate, localeName);

    return Material(
      color: theme.cardTheme.color ?? scheme.surface,
      shape: RoundedRectangleBorder(
        borderRadius: BorderRadius.circular(18),
        side: BorderSide(color: scheme.outlineVariant),
      ),
      clipBehavior: Clip.antiAlias,
      child: InkWell(
        onTap: tag.isEmpty
            ? null
            : () => context.push('/assets/${Uri.encodeComponent(tag)}'),
        child: Padding(
          padding: const EdgeInsets.all(14),
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: <Widget>[
              Row(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: <Widget>[
                  Expanded(
                    child: Text(
                      assignment.assetName.isEmpty
                          ? l10n.catalogUnnamedAsset
                          : assignment.assetName,
                      maxLines: 2,
                      overflow: TextOverflow.ellipsis,
                      style: TextStyle(
                        fontSize: 14.5,
                        fontWeight: FontWeight.w700,
                        color: scheme.onSurface,
                      ),
                    ),
                  ),
                  const SizedBox(width: 8),
                  StatusChip(
                    label: l10n.assetDetailStatusAssigned,
                    variant: StatusChipVariant.info,
                  ),
                ],
              ),
              if (tag.isNotEmpty) ...<Widget>[
                const SizedBox(height: 4),
                Row(
                  children: <Widget>[
                    Icon(
                      Symbols.barcode_rounded,
                      size: 14,
                      color: theme.textTheme.labelSmall?.color,
                    ),
                    const SizedBox(width: 4),
                    Flexible(
                      child: Text(
                        tag,
                        maxLines: 1,
                        overflow: TextOverflow.ellipsis,
                        style: TextStyle(
                          fontSize: 12,
                          color: theme.textTheme.bodySmall?.color,
                        ),
                      ),
                    ),
                  ],
                ),
              ],
              const SizedBox(height: 10),
              if (checkoutText != null)
                _MetaLine(
                  icon: Symbols.event_available_rounded,
                  text: l10n.myAssetsHeldSince(checkoutText),
                ),
              if (dueText != null) ...<Widget>[
                const SizedBox(height: 4),
                Row(
                  children: <Widget>[
                    _MetaLine(
                      icon: Symbols.event_rounded,
                      text: l10n.myAssetsDue(dueText),
                      danger: overdue,
                    ),
                    if (overdue) ...<Widget>[
                      const SizedBox(width: 8),
                      _OverdueBadge(label: l10n.myAssetsOverdue),
                    ],
                  ],
                ),
              ],
            ],
          ),
        ),
      ),
    );
  }
}

class _MetaLine extends StatelessWidget {
  const _MetaLine({
    required this.icon,
    required this.text,
    this.danger = false,
  });

  final IconData icon;
  final String text;
  final bool danger;

  @override
  Widget build(BuildContext context) {
    final ThemeData theme = Theme.of(context);
    final Color color = danger
        ? theme.colorScheme.error
        : (theme.textTheme.bodySmall?.color ?? theme.colorScheme.onSurface);
    return Row(
      mainAxisSize: MainAxisSize.min,
      children: <Widget>[
        Icon(icon, size: 14, color: color),
        const SizedBox(width: 6),
        Text(
          text,
          style: TextStyle(
            fontSize: 12,
            color: color,
            fontWeight: danger ? FontWeight.w600 : FontWeight.w400,
          ),
        ),
      ],
    );
  }
}

class _OverdueBadge extends StatelessWidget {
  const _OverdueBadge({required this.label});

  final String label;

  @override
  Widget build(BuildContext context) {
    final StatusColorSet danger = statusColorSetOf(
      context,
      StatusChipVariant.danger,
    );
    return Container(
      padding: const EdgeInsets.symmetric(horizontal: 8, vertical: 2),
      decoration: ShapeDecoration(color: danger.bg, shape: const StadiumBorder()),
      child: Text(
        label,
        style: TextStyle(
          fontSize: 10.5,
          fontWeight: FontWeight.w700,
          color: danger.text,
        ),
      ),
    );
  }
}

/// True bila [dueDate] ("2006-01-02") jatuh sebelum hari ini (perbandingan
/// tanggal saja, mengabaikan jam).
bool _isOverdue(String? dueDate, DateTime now) {
  if (dueDate == null) {
    return false;
  }
  final DateTime? due = DateTime.tryParse(dueDate);
  if (due == null) {
    return false;
  }
  final DateTime today = DateTime(now.year, now.month, now.day);
  final DateTime dueDay = DateTime(due.year, due.month, due.day);
  return dueDay.isBefore(today);
}

/// Format tanggal ISO ke "d MMM y" sesuai locale (paritas format approval).
/// Null / tak terparse -> null (baris tidak dirender).
String? _formatDate(String? iso, String localeName) {
  if (iso == null) {
    return null;
  }
  final DateTime? date = DateTime.tryParse(iso);
  if (date == null) {
    return null;
  }
  return DateFormat('d MMM y', localeName).format(date);
}

/// Skeleton loading: empat kerangka kartu.
class _LoadingSkeleton extends StatelessWidget {
  const _LoadingSkeleton();

  @override
  Widget build(BuildContext context) {
    final ThemeData theme = Theme.of(context);
    final ColorScheme scheme = theme.colorScheme;

    Widget card() => Container(
      padding: const EdgeInsets.all(14),
      decoration: BoxDecoration(
        color: theme.cardTheme.color ?? scheme.surface,
        borderRadius: BorderRadius.circular(18),
        border: Border.all(color: scheme.outlineVariant),
      ),
      child: const Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: <Widget>[
          Row(
            children: <Widget>[
              Expanded(child: AppSkeleton(height: 14, borderRadius: 7)),
              SizedBox(width: 8),
              AppSkeleton(height: 24, width: 84, borderRadius: 999),
            ],
          ),
          SizedBox(height: 10),
          AppSkeleton(height: 11, width: 150, borderRadius: 6),
          SizedBox(height: 8),
          AppSkeleton(height: 11, width: 120, borderRadius: 6),
        ],
      ),
    );

    return ListView(
      padding: const EdgeInsets.fromLTRB(20, 8, 20, 24),
      children: <Widget>[
        for (int i = 0; i < 4; i++) ...<Widget>[
          if (i > 0) const SizedBox(height: 10),
          card(),
        ],
      ],
    );
  }
}

/// Cabang error daftar: offline, 403, dan generik.
class _ErrorState extends StatelessWidget {
  const _ErrorState({required this.failure, required this.onRetry});

  final Object failure;
  final VoidCallback onRetry;

  @override
  Widget build(BuildContext context) {
    final AppLocalizations l10n = AppLocalizations.of(context);

    return switch (failure) {
      NetworkFailure() => EmptyState(
        icon: Symbols.wifi_off_rounded,
        title: l10n.myAssetsErrorTitle,
        subtitle: l10n.myAssetsErrorNetworkBody,
        actionLabel: l10n.commonRetry,
        onAction: onRetry,
      ),
      ForbiddenFailure() => EmptyState(
        icon: Symbols.lock_rounded,
        title: l10n.myAssetsForbiddenTitle,
        subtitle: l10n.myAssetsForbiddenBody,
      ),
      _ => EmptyState(
        icon: Symbols.error_rounded,
        title: l10n.myAssetsErrorTitle,
        subtitle: l10n.myAssetsErrorGenericBody,
        actionLabel: l10n.commonRetry,
        onAction: onRetry,
      ),
    };
  }
}
