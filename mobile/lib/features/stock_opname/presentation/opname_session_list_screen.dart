import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';
import 'package:material_symbols_icons/symbols.dart';

import '../../../core/api/app_failure.dart';
import '../../../core/connectivity/connectivity_provider.dart';
import '../../../core/i18n/gen/app_localizations.dart';
import '../../../core/widgets/app_skeleton.dart';
import '../../../core/widgets/empty_state.dart';
import '../../../core/widgets/offline_banner.dart';
import '../../../core/widgets/status_chip.dart';
import '../../../core/widgets/sync_pill.dart';
import '../data/stock_opname_session_dto.dart';
import 'opname_presentation.dart';
import 'opname_sessions_provider.dart';

/// Layar Daftar Sesi Opname 1:1 mockup "Inventra Mobile - Daftar Sesi Opname":
/// chip filter (Berjalan/Selesai/Semua), kartu sesi (nama, kantor · periode,
/// progress KPI, chip status, SyncPill, CTA lanjut menghitung), empty state,
/// skeleton, error + retry, pull-to-refresh, dan OfflineBanner saat offline.
///
/// Fase M0 online-only: elemen snapshot/antrean offline mockup dirender
/// sebagai SyncPill berstatus tersinkron/offline — tanpa drift/antrean (M5).
class OpnameSessionListScreen extends ConsumerStatefulWidget {
  const OpnameSessionListScreen({super.key});

  @override
  ConsumerState<OpnameSessionListScreen> createState() =>
      _OpnameSessionListScreenState();
}

class _OpnameSessionListScreenState
    extends ConsumerState<OpnameSessionListScreen> {
  OpnameSessionTab _tab = OpnameSessionTab.running;

  Future<void> _refresh() async {
    ref.invalidate(opnameSessionsProvider);
    try {
      await ref.read(opnameSessionsProvider.future);
    } on Object {
      // Kegagalan refresh sudah tercermin sebagai state error daftar.
    }
  }

  @override
  Widget build(BuildContext context) {
    final AppLocalizations l10n = AppLocalizations.of(context);
    final AsyncValue<List<StockOpnameSessionDto>> state = ref.watch(
      opnameSessionsProvider,
    );
    final bool offline = isOffline(ref.watch(isOnlineProvider));

    return Scaffold(
      appBar: AppBar(title: Text(l10n.opnameSessionsTitle)),
      body: SafeArea(
        child: Column(
          children: <Widget>[
            if (offline)
              Padding(
                padding: const EdgeInsets.fromLTRB(20, 0, 20, 10),
                child: OfflineBanner(message: l10n.opnameOfflineBanner),
              ),
            _FilterRow(
              selected: _tab,
              onSelect: (OpnameSessionTab tab) => setState(() => _tab = tab),
            ),
            Expanded(
              child: state.when(
                data: (List<StockOpnameSessionDto> sessions) => _SessionList(
                  tab: _tab,
                  sessions: sessions
                      .where(_tab.matches)
                      .toList(growable: false),
                  offline: offline,
                  onRefresh: _refresh,
                ),
                loading: () => const _LoadingSkeleton(),
                error: (Object error, StackTrace stackTrace) => _ErrorState(
                  failure: error,
                  onRetry: () => ref.invalidate(opnameSessionsProvider),
                ),
              ),
            ),
          ],
        ),
      ),
    );
  }
}

/// Baris chip filter tab (pill aktif primary — pola chip inbox approval).
class _FilterRow extends StatelessWidget {
  const _FilterRow({required this.selected, required this.onSelect});

  final OpnameSessionTab selected;
  final ValueChanged<OpnameSessionTab> onSelect;

  @override
  Widget build(BuildContext context) {
    final AppLocalizations l10n = AppLocalizations.of(context);

    String label(OpnameSessionTab tab) => switch (tab) {
      OpnameSessionTab.running => l10n.opnameSessionsFilterRunning,
      OpnameSessionTab.closed => l10n.opnameSessionsFilterClosed,
      OpnameSessionTab.all => l10n.opnameSessionsFilterAll,
    };

    return SingleChildScrollView(
      scrollDirection: Axis.horizontal,
      padding: const EdgeInsets.fromLTRB(20, 6, 20, 12),
      child: Row(
        children: <Widget>[
          for (final OpnameSessionTab tab
              in OpnameSessionTab.values) ...<Widget>[
            if (tab != OpnameSessionTab.values.first) const SizedBox(width: 8),
            _FilterChip(
              label: label(tab),
              active: tab == selected,
              onTap: () => onSelect(tab),
            ),
          ],
        ],
      ),
    );
  }
}

class _FilterChip extends StatelessWidget {
  const _FilterChip({
    required this.label,
    required this.active,
    required this.onTap,
  });

  final String label;
  final bool active;
  final VoidCallback onTap;

  @override
  Widget build(BuildContext context) {
    final ThemeData theme = Theme.of(context);
    final ColorScheme scheme = theme.colorScheme;

    return Semantics(
      button: true,
      selected: active,
      child: Material(
        color: active
            ? scheme.primary
            : theme.cardTheme.color ?? scheme.surface,
        shape: StadiumBorder(
          side: active
              ? BorderSide.none
              : BorderSide(color: scheme.outlineVariant),
        ),
        clipBehavior: Clip.antiAlias,
        child: InkWell(
          onTap: onTap,
          child: Padding(
            padding: const EdgeInsets.symmetric(horizontal: 16, vertical: 8),
            child: Text(
              label,
              style: TextStyle(
                fontSize: 12.5,
                fontWeight: active ? FontWeight.w700 : FontWeight.w600,
                color: active
                    ? scheme.onPrimary
                    : theme.textTheme.labelMedium?.color,
              ),
            ),
          ),
        ),
      ),
    );
  }
}

/// Daftar kartu sesi + pull-to-refresh + catatan kaki; empty state per tab.
class _SessionList extends StatelessWidget {
  const _SessionList({
    required this.tab,
    required this.sessions,
    required this.offline,
    required this.onRefresh,
  });

  final OpnameSessionTab tab;
  final List<StockOpnameSessionDto> sessions;
  final bool offline;
  final Future<void> Function() onRefresh;

  @override
  Widget build(BuildContext context) {
    final AppLocalizations l10n = AppLocalizations.of(context);

    if (sessions.isEmpty) {
      if (tab == OpnameSessionTab.running) {
        return EmptyState(
          icon: Symbols.fact_check_rounded,
          title: l10n.opnameSessionsEmptyTitle,
          subtitle: l10n.opnameSessionsEmptyBody,
        );
      }
      return EmptyState(
        icon: Symbols.fact_check_rounded,
        title: l10n.opnameSessionsEmptyFilteredTitle,
        subtitle: l10n.opnameSessionsEmptyFilteredBody,
      );
    }

    return RefreshIndicator(
      onRefresh: onRefresh,
      child: ListView.separated(
        physics: const AlwaysScrollableScrollPhysics(),
        padding: const EdgeInsets.fromLTRB(20, 0, 20, 24),
        itemCount: sessions.length + 1,
        separatorBuilder: (BuildContext context, int index) =>
            const SizedBox(height: 12),
        itemBuilder: (BuildContext context, int index) {
          if (index == sessions.length) {
            return const _Footnote();
          }
          return _SessionCard(session: sessions[index], offline: offline);
        },
      ),
    );
  }
}

/// Catatan kaki daftar (mockup): sesi dikelola dari aplikasi web.
class _Footnote extends StatelessWidget {
  const _Footnote();

  @override
  Widget build(BuildContext context) {
    final ThemeData theme = Theme.of(context);
    final AppLocalizations l10n = AppLocalizations.of(context);
    final Color? muted = theme.textTheme.labelSmall?.color;

    return Padding(
      padding: const EdgeInsets.only(top: 2),
      child: Row(
        mainAxisAlignment: MainAxisAlignment.center,
        children: <Widget>[
          Icon(Symbols.info_rounded, size: 15, color: muted),
          const SizedBox(width: 7),
          Flexible(
            child: Text(
              l10n.opnameSessionsFootnote,
              style: TextStyle(fontSize: 11.5, color: muted),
            ),
          ),
        ],
      ),
    );
  }
}

/// Kartu sesi 1:1 mockup: judul + chip status, kantor · periode, progress KPI,
/// SyncPill (online-only), dan CTA "Lanjutkan Menghitung" / chip Berita Acara.
class _SessionCard extends StatelessWidget {
  const _SessionCard({required this.session, required this.offline});

  final StockOpnameSessionDto session;
  final bool offline;

  @override
  Widget build(BuildContext context) {
    final ThemeData theme = Theme.of(context);
    final ColorScheme scheme = theme.colorScheme;
    final AppLocalizations l10n = AppLocalizations.of(context);
    final String localeName = Localizations.localeOf(context).languageCode;
    final bool closed = session.status == 'closed';
    final (String statusLabel, StatusChipVariant statusVariant) =
        opnameSessionStatusPresentation(l10n, session.status);
    final int? total = session.total;
    final int? counted = opnameCountedOf(session);

    return Material(
      color: theme.cardTheme.color ?? scheme.surface,
      shape: RoundedRectangleBorder(
        borderRadius: BorderRadius.circular(20),
        side: BorderSide(color: scheme.outlineVariant),
      ),
      clipBehavior: Clip.antiAlias,
      child: InkWell(
        onTap: () => context.push('/stock-opname/${session.id}'),
        child: Padding(
          padding: const EdgeInsets.all(16),
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: <Widget>[
              Row(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: <Widget>[
                  Expanded(
                    child: Text(
                      opnameSessionTitle(session),
                      style: TextStyle(
                        fontSize: 15,
                        fontWeight: FontWeight.w700,
                        height: 1.35,
                        color: closed
                            ? theme.textTheme.labelLarge?.color
                            : scheme.onSurface,
                      ),
                    ),
                  ),
                  const SizedBox(width: 10),
                  StatusChip(label: statusLabel, variant: statusVariant),
                ],
              ),
              const SizedBox(height: 6),
              Text(
                opnameSessionSubtitle(session, localeName),
                style: TextStyle(
                  fontSize: 12,
                  color: closed
                      ? theme.textTheme.labelSmall?.color
                      : theme.textTheme.bodySmall?.color,
                ),
              ),
              if (total != null && counted != null) ...<Widget>[
                const SizedBox(height: 12),
                _ProgressRow(counted: counted, total: total, closed: closed),
              ],
              const SizedBox(height: 12),
              if (closed)
                _ReportOnWebChip(label: l10n.opnameSessionsReportOnWeb)
              else ...<Widget>[
                SyncPill(
                  status: offline
                      ? SyncPillStatus.offline
                      : SyncPillStatus.synced,
                ),
                const SizedBox(height: 14),
                FilledButton.icon(
                  style: FilledButton.styleFrom(
                    minimumSize: const Size.fromHeight(50),
                    textStyle: theme.textTheme.labelLarge?.copyWith(
                      fontSize: 14.5,
                      fontWeight: FontWeight.w700,
                    ),
                  ),
                  onPressed: () => context.push('/stock-opname/${session.id}'),
                  icon: const Icon(Symbols.play_arrow_rounded, size: 20),
                  label: Text(l10n.opnameSessionsContinue),
                ),
              ],
            ],
          ),
        ),
      ),
    );
  }
}

/// Baris progress kartu sesi: "x dari y tercocokkan" + persen + bar.
class _ProgressRow extends StatelessWidget {
  const _ProgressRow({
    required this.counted,
    required this.total,
    required this.closed,
  });

  final int counted;
  final int total;
  final bool closed;

  @override
  Widget build(BuildContext context) {
    final ThemeData theme = Theme.of(context);
    final ColorScheme scheme = theme.colorScheme;
    final AppLocalizations l10n = AppLocalizations.of(context);
    final double fraction = total == 0 ? 0 : counted / total;
    final Color barColor = closed
        ? (theme.textTheme.labelSmall?.color ?? scheme.outline)
        : scheme.primary;

    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: <Widget>[
        Row(
          mainAxisAlignment: MainAxisAlignment.spaceBetween,
          children: <Widget>[
            Flexible(
              child: Text(
                l10n.opnameSessionsProgress(counted, total),
                overflow: TextOverflow.ellipsis,
                style: TextStyle(
                  fontSize: 12,
                  color: closed
                      ? theme.textTheme.labelSmall?.color
                      : theme.textTheme.bodySmall?.color,
                ),
              ),
            ),
            const SizedBox(width: 8),
            Text(
              '${(fraction * 100).round()}%',
              style: TextStyle(
                fontSize: 12,
                fontWeight: FontWeight.w700,
                color: closed
                    ? theme.textTheme.bodySmall?.color
                    : scheme.onPrimaryContainer,
              ),
            ),
          ],
        ),
        const SizedBox(height: 6),
        ClipRRect(
          borderRadius: BorderRadius.circular(999),
          child: LinearProgressIndicator(
            value: fraction,
            minHeight: 8,
            backgroundColor: scheme.outlineVariant,
            color: barColor,
          ),
        ),
      ],
    );
  }
}

/// Chip info kartu sesi selesai: Berita Acara diakses dari aplikasi web.
class _ReportOnWebChip extends StatelessWidget {
  const _ReportOnWebChip({required this.label});

  final String label;

  @override
  Widget build(BuildContext context) {
    final ThemeData theme = Theme.of(context);
    final Color? muted = theme.textTheme.bodySmall?.color;

    return Container(
      width: double.infinity,
      padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 10),
      decoration: BoxDecoration(
        color: theme.scaffoldBackgroundColor,
        borderRadius: BorderRadius.circular(11),
        border: Border.all(color: theme.colorScheme.outlineVariant),
      ),
      child: Row(
        children: <Widget>[
          Icon(Symbols.description_rounded, size: 16, color: muted),
          const SizedBox(width: 6),
          Text(
            label,
            style: TextStyle(
              fontSize: 12,
              fontWeight: FontWeight.w500,
              color: muted,
            ),
          ),
        ],
      ),
    );
  }
}

/// Skeleton loading: dua kerangka kartu sesi (mockup state loading).
class _LoadingSkeleton extends StatelessWidget {
  const _LoadingSkeleton();

  @override
  Widget build(BuildContext context) {
    final ThemeData theme = Theme.of(context);
    final ColorScheme scheme = theme.colorScheme;

    Widget card() => Container(
      padding: const EdgeInsets.all(16),
      decoration: BoxDecoration(
        color: theme.cardTheme.color ?? scheme.surface,
        borderRadius: BorderRadius.circular(20),
        border: Border.all(color: scheme.outlineVariant),
      ),
      child: const Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: <Widget>[
          Row(
            mainAxisAlignment: MainAxisAlignment.spaceBetween,
            children: <Widget>[
              AppSkeleton(height: 15, width: 200, borderRadius: 8),
              AppSkeleton(height: 24, width: 80, borderRadius: 999),
            ],
          ),
          SizedBox(height: 12),
          AppSkeleton(height: 11, width: 150, borderRadius: 6),
          SizedBox(height: 14),
          AppSkeleton(height: 8, borderRadius: 999),
          SizedBox(height: 14),
          AppSkeleton(height: 50, borderRadius: 14),
        ],
      ),
    );

    return ListView(
      padding: const EdgeInsets.fromLTRB(20, 0, 20, 24),
      children: <Widget>[card(), const SizedBox(height: 12), card()],
    );
  }
}

/// Cabang error daftar sesi: offline, 403 (tanpa akses), dan generik.
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
        title: l10n.opnameSessionsErrorTitle,
        subtitle: l10n.opnameErrorNetworkBody,
        actionLabel: l10n.commonRetry,
        onAction: onRetry,
      ),
      ForbiddenFailure() => EmptyState(
        icon: Symbols.lock_rounded,
        title: l10n.opnameForbiddenTitle,
        subtitle: l10n.opnameForbiddenBody,
      ),
      _ => EmptyState(
        icon: Symbols.error_rounded,
        title: l10n.opnameSessionsErrorTitle,
        subtitle: l10n.opnameErrorGenericBody,
        actionLabel: l10n.commonRetry,
        onAction: onRetry,
      ),
    };
  }
}
