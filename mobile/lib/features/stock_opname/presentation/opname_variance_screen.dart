import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';
import 'package:material_symbols_icons/symbols.dart';

import '../../../app/theme.dart';
import '../../../core/api/app_failure.dart';
import '../../../core/i18n/gen/app_localizations.dart';
import '../../../core/widgets/app_skeleton.dart';
import '../../../core/widgets/empty_state.dart';
import '../../../core/widgets/status_chip.dart';
import '../data/stock_opname_item_dto.dart';
import '../data/stock_opname_repository.dart';
import '../data/stock_opname_session_dto.dart';
import 'opname_presentation.dart';
import 'opname_session_detail_provider.dart';

/// Layar Variance Opname 1:1 mockup "Inventra Mobile - Variance Opname":
/// toggle Item | Variance (Item kembali ke counting), ringkasan empat kategori
/// (Tidak Ditemukan/Rusak/Salah Lokasi/Di Luar Catatan), daftar item variance
/// berkelompok dengan StatusChip + status tindak lanjut, empty state "Tidak
/// ada selisih", skeleton, dan error + retry.
///
/// Selisih dihitung klien dari `GET .../items` ([OpnameVarianceData]) — tidak
/// ada endpoint variance khusus di kontrak. Aksi tindak lanjut (pengajuan
/// penghapusan/mutasi/maintenance) dilakukan dari aplikasi web pada fase ini;
/// layar merender status tindak lanjut dari `followup_request_id`/
/// `followup_record_id`.
class OpnameVarianceScreen extends ConsumerWidget {
  const OpnameVarianceScreen({required this.sessionId, super.key});

  final String sessionId;

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final AppLocalizations l10n = AppLocalizations.of(context);
    final String localeName = Localizations.localeOf(context).languageCode;
    final AsyncValue<OpnameSessionDetail> state = ref.watch(
      opnameSessionDetailProvider(sessionId),
    );
    final StockOpnameSessionDto? session = state.value?.session;

    return Scaffold(
      appBar: AppBar(
        title: session == null
            ? Text(l10n.opnameVarianceTitle)
            : Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: <Widget>[
                  Text(
                    opnameSessionTitle(session),
                    maxLines: 1,
                    overflow: TextOverflow.ellipsis,
                    style: const TextStyle(
                      fontSize: 15,
                      fontWeight: FontWeight.w700,
                    ),
                  ),
                  Text(
                    opnameSessionSubtitle(session, localeName),
                    maxLines: 1,
                    overflow: TextOverflow.ellipsis,
                    style: TextStyle(
                      fontSize: 11,
                      fontWeight: FontWeight.w400,
                      color: Theme.of(context).textTheme.bodySmall?.color,
                    ),
                  ),
                ],
              ),
      ),
      body: SafeArea(
        child: Column(
          children: <Widget>[
            Padding(
              padding: const EdgeInsets.fromLTRB(20, 0, 20, 12),
              child: _ViewToggle(
                // Kembali ke counting; bila layar dibuka langsung (deep link)
                // tidak ada yang bisa di-pop — go ke rute counting.
                onItemsTap: () {
                  final NavigatorState navigator = Navigator.of(context);
                  if (navigator.canPop()) {
                    navigator.pop();
                  } else {
                    context.go('/stock-opname/$sessionId');
                  }
                },
              ),
            ),
            Expanded(
              child: state.when(
                data: (OpnameSessionDetail detail) => _VarianceBody(
                  detail: detail,
                  data: OpnameVarianceData.fromItems(detail.items),
                ),
                loading: () => const _LoadingSkeleton(),
                error: (Object error, StackTrace stackTrace) => _ErrorState(
                  failure: error,
                  onRetry: () =>
                      ref.invalidate(opnameSessionDetailProvider(sessionId)),
                ),
              ),
            ),
          ],
        ),
      ),
    );
  }
}

/// Toggle dua segmen Item | Variance (mockup). Variance selalu aktif di layar
/// ini; segmen Item kembali ke layar counting.
class _ViewToggle extends StatelessWidget {
  const _ViewToggle({required this.onItemsTap});

  final VoidCallback onItemsTap;

  @override
  Widget build(BuildContext context) {
    final ThemeData theme = Theme.of(context);
    final ColorScheme scheme = theme.colorScheme;
    final AppLocalizations l10n = AppLocalizations.of(context);

    Widget segment({
      required String label,
      required bool active,
      VoidCallback? onTap,
    }) {
      return Expanded(
        child: Semantics(
          button: true,
          selected: active,
          child: Material(
            color: active
                ? theme.cardTheme.color ?? scheme.surface
                : Colors.transparent,
            borderRadius: BorderRadius.circular(10),
            clipBehavior: Clip.antiAlias,
            child: InkWell(
              onTap: onTap,
              child: SizedBox(
                height: 36,
                child: Center(
                  child: Text(
                    label,
                    style: TextStyle(
                      fontSize: 13,
                      fontWeight: active ? FontWeight.w700 : FontWeight.w600,
                      color: active
                          ? scheme.onSurface
                          : theme.textTheme.bodySmall?.color,
                    ),
                  ),
                ),
              ),
            ),
          ),
        ),
      );
    }

    return Container(
      padding: const EdgeInsets.all(4),
      decoration: BoxDecoration(
        color: scheme.secondaryContainer,
        borderRadius: BorderRadius.circular(13),
      ),
      child: Row(
        children: <Widget>[
          segment(
            label: l10n.opnameVarianceTabItems,
            active: false,
            onTap: onItemsTap,
          ),
          segment(label: l10n.opnameVarianceTabVariance, active: true),
        ],
      ),
    );
  }
}

/// Isi layar saat data termuat: ringkasan + kelompok item, atau empty state
/// "Tidak ada selisih" bila semua tercocokkan.
class _VarianceBody extends StatelessWidget {
  const _VarianceBody({required this.detail, required this.data});

  final OpnameSessionDetail detail;
  final OpnameVarianceData data;

  @override
  Widget build(BuildContext context) {
    final AppLocalizations l10n = AppLocalizations.of(context);
    final InventraStatusColors colors = Theme.of(
      context,
    ).extension<InventraStatusColors>()!;

    if (data.isEmpty) {
      return _NoVarianceState(
        total: detail.session.total ?? detail.items.length,
      );
    }

    return ListView(
      padding: const EdgeInsets.fromLTRB(20, 0, 20, 20),
      children: <Widget>[
        Row(
          children: <Widget>[
            Expanded(
              child: _SummaryCard(
                count: data.notFound.length,
                label: l10n.opnameResultNotFound,
                set: colors.danger,
              ),
            ),
            const SizedBox(width: 8),
            Expanded(
              child: _SummaryCard(
                count: data.damaged.length,
                label: l10n.opnameResultDamaged,
                set: colors.warning,
              ),
            ),
            const SizedBox(width: 8),
            Expanded(
              child: _SummaryCard(
                count: data.misplaced.length,
                label: l10n.opnameResultMisplaced,
                set: colors.info,
              ),
            ),
            const SizedBox(width: 8),
            Expanded(
              child: _SummaryCard(
                count: data.unexpected.length,
                label: l10n.opnameOutOfSnapshot,
                set: colors.neutral,
              ),
            ),
          ],
        ),
        _Group(
          icon: Symbols.help_rounded,
          label: l10n.opnameResultNotFound,
          set: colors.danger,
          items: data.notFound,
        ),
        _Group(
          icon: Symbols.build_rounded,
          label: l10n.opnameResultDamaged,
          set: colors.warning,
          items: data.damaged,
        ),
        _Group(
          icon: Symbols.wrong_location_rounded,
          label: l10n.opnameResultMisplaced,
          set: colors.info,
          items: data.misplaced,
        ),
        _Group(
          icon: Symbols.playlist_add_rounded,
          label: l10n.opnameOutOfSnapshot,
          set: colors.neutral,
          items: data.unexpected,
        ),
        const SizedBox(height: 10),
        const _Footnote(),
      ],
    );
  }
}

/// Kartu ringkasan satu kategori variance (angka besar + label).
class _SummaryCard extends StatelessWidget {
  const _SummaryCard({
    required this.count,
    required this.label,
    required this.set,
  });

  final int count;
  final String label;
  final StatusColorSet set;

  @override
  Widget build(BuildContext context) {
    final ThemeData theme = Theme.of(context);

    return Container(
      padding: const EdgeInsets.symmetric(horizontal: 6, vertical: 10),
      decoration: BoxDecoration(
        color: theme.cardTheme.color ?? theme.colorScheme.surface,
        borderRadius: BorderRadius.circular(14),
        border: Border.all(color: set.dot.withValues(alpha: 0.35)),
      ),
      child: Column(
        children: <Widget>[
          Text(
            '$count',
            style: TextStyle(
              fontSize: 20,
              fontWeight: FontWeight.w800,
              height: 1.1,
              color: set.text,
            ),
          ),
          const SizedBox(height: 3),
          Text(
            label,
            textAlign: TextAlign.center,
            maxLines: 2,
            style: TextStyle(
              fontSize: 9.5,
              fontWeight: FontWeight.w600,
              color: set.text,
            ),
          ),
        ],
      ),
    );
  }
}

/// Satu kelompok kategori: header (ikon + label + jumlah) dan kartu item.
class _Group extends StatelessWidget {
  const _Group({
    required this.icon,
    required this.label,
    required this.set,
    required this.items,
  });

  final IconData icon;
  final String label;
  final StatusColorSet set;
  final List<StockOpnameItemDto> items;

  @override
  Widget build(BuildContext context) {
    if (items.isEmpty) {
      return const SizedBox.shrink();
    }

    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: <Widget>[
        Padding(
          padding: const EdgeInsets.fromLTRB(0, 12, 0, 8),
          child: Row(
            children: <Widget>[
              Icon(icon, size: 15, color: set.text),
              const SizedBox(width: 6),
              Text(
                '${label.toUpperCase()} (${items.length})',
                style: TextStyle(
                  fontSize: 11.5,
                  fontWeight: FontWeight.w700,
                  letterSpacing: 0.55,
                  color: set.text,
                ),
              ),
            ],
          ),
        ),
        for (int i = 0; i < items.length; i++) ...<Widget>[
          if (i > 0) const SizedBox(height: 10),
          _VarianceItemCard(item: items[i]),
        ],
      ],
    );
  }
}

/// Kartu item variance: nama, tag + lokasi terakhir, catatan petugas, chip
/// hasil, dan status tindak lanjut.
class _VarianceItemCard extends StatelessWidget {
  const _VarianceItemCard({required this.item});

  final StockOpnameItemDto item;

  @override
  Widget build(BuildContext context) {
    final ThemeData theme = Theme.of(context);
    final ColorScheme scheme = theme.colorScheme;
    final AppLocalizations l10n = AppLocalizations.of(context);
    final (String resultLabel, StatusChipVariant resultVariant) =
        opnameItemResultPresentation(l10n, item.result);
    final String? note = item.note;
    final String location = <String>[
      if (item.roomName != null) item.roomName!,
      if (item.floorName != null) item.floorName!,
    ].join(', ');
    final String subtitle = <String>[
      if (item.assetTag != null) item.assetTag!,
      if (location.isNotEmpty) l10n.opnameVarianceLastLocation(location),
    ].join(' · ');

    return Container(
      padding: const EdgeInsets.fromLTRB(14, 13, 14, 13),
      decoration: BoxDecoration(
        color: theme.cardTheme.color ?? scheme.surface,
        borderRadius: BorderRadius.circular(16),
        border: Border.all(color: scheme.outlineVariant),
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: <Widget>[
          Row(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: <Widget>[
              Expanded(
                child: Text(
                  item.assetName ?? item.assetId,
                  style: const TextStyle(
                    fontSize: 13.5,
                    fontWeight: FontWeight.w700,
                  ),
                ),
              ),
              const SizedBox(width: 8),
              StatusChip(label: resultLabel, variant: resultVariant),
            ],
          ),
          const SizedBox(height: 2),
          Text(
            subtitle,
            style: TextStyle(
              fontSize: 10.5,
              color: theme.textTheme.labelSmall?.color,
            ),
          ),
          if (note != null && note.isNotEmpty) ...<Widget>[
            const SizedBox(height: 7),
            Text(
              l10n.opnameVarianceNote(note),
              style: TextStyle(
                fontSize: 11.5,
                color: theme.textTheme.bodySmall?.color,
              ),
            ),
          ],
          const SizedBox(height: 10),
          Container(height: 1, color: scheme.outlineVariant),
          const SizedBox(height: 9),
          _FollowupStatus(item: item),
        ],
      ),
    );
  }
}

/// Status tindak lanjut item variance dari `followup_request_id` /
/// `followup_record_id` (aksi tindak lanjut dilakukan dari aplikasi web).
class _FollowupStatus extends StatelessWidget {
  const _FollowupStatus({required this.item});

  final StockOpnameItemDto item;

  @override
  Widget build(BuildContext context) {
    final ThemeData theme = Theme.of(context);
    final InventraStatusColors colors = theme
        .extension<InventraStatusColors>()!;
    final AppLocalizations l10n = AppLocalizations.of(context);

    if (item.followupRequestId != null) {
      return Row(
        children: <Widget>[
          Icon(Symbols.schedule_rounded, size: 13, color: colors.warning.text),
          const SizedBox(width: 5),
          Flexible(
            child: Text(
              l10n.opnameVarianceFollowupRequested,
              overflow: TextOverflow.ellipsis,
              style: TextStyle(
                fontSize: 11,
                fontWeight: FontWeight.w600,
                color: colors.warning.text,
              ),
            ),
          ),
        ],
      );
    }
    if (item.followupRecordId != null) {
      return Row(
        children: <Widget>[
          Icon(Symbols.build_rounded, size: 13, color: colors.success.text),
          const SizedBox(width: 5),
          Flexible(
            child: Text(
              l10n.opnameVarianceFollowupRecord,
              overflow: TextOverflow.ellipsis,
              style: TextStyle(
                fontSize: 11,
                fontWeight: FontWeight.w600,
                color: colors.success.text,
              ),
            ),
          ),
        ],
      );
    }
    return Row(
      children: <Widget>[
        Container(
          width: 7,
          height: 7,
          decoration: BoxDecoration(
            color: colors.neutral.dot,
            shape: BoxShape.circle,
          ),
        ),
        const SizedBox(width: 5),
        Flexible(
          child: Text(
            l10n.opnameVarianceFollowupNone,
            overflow: TextOverflow.ellipsis,
            style: TextStyle(
              fontSize: 11,
              fontWeight: FontWeight.w600,
              color: theme.textTheme.labelSmall?.color,
            ),
          ),
        ),
      ],
    );
  }
}

/// Empty state "Tidak ada selisih" 1:1 mockup: lingkaran hijau + ikon
/// verified, judul, dan penjelasan jumlah aset tercocokkan.
class _NoVarianceState extends StatelessWidget {
  const _NoVarianceState({required this.total});

  final int total;

  @override
  Widget build(BuildContext context) {
    final ThemeData theme = Theme.of(context);
    final InventraStatusColors colors = theme
        .extension<InventraStatusColors>()!;
    final AppLocalizations l10n = AppLocalizations.of(context);

    return Column(
      children: <Widget>[
        Expanded(
          child: Center(
            child: Padding(
              padding: const EdgeInsets.symmetric(horizontal: 40),
              child: Column(
                mainAxisSize: MainAxisSize.min,
                children: <Widget>[
                  Container(
                    width: 104,
                    height: 104,
                    decoration: BoxDecoration(
                      color: colors.success.bg,
                      shape: BoxShape.circle,
                    ),
                    child: Icon(
                      Symbols.verified_rounded,
                      size: 54,
                      color: colors.success.dot,
                    ),
                  ),
                  const SizedBox(height: 18),
                  Text(
                    l10n.opnameVarianceEmptyTitle,
                    style: TextStyle(
                      fontSize: 17,
                      fontWeight: FontWeight.w800,
                      color: theme.colorScheme.onSurface,
                    ),
                  ),
                  const SizedBox(height: 6),
                  Text(
                    l10n.opnameVarianceEmptyBody(total),
                    textAlign: TextAlign.center,
                    style: TextStyle(
                      fontSize: 13,
                      height: 1.55,
                      color: theme.textTheme.bodySmall?.color,
                    ),
                  ),
                ],
              ),
            ),
          ),
        ),
        const Padding(
          padding: EdgeInsets.fromLTRB(20, 0, 20, 6),
          child: _Footnote(),
        ),
      ],
    );
  }
}

/// Catatan kaki layar variance (mockup): penyelesaian sesi dari web.
class _Footnote extends StatelessWidget {
  const _Footnote();

  @override
  Widget build(BuildContext context) {
    final ThemeData theme = Theme.of(context);
    final AppLocalizations l10n = AppLocalizations.of(context);
    final Color? muted = theme.textTheme.labelSmall?.color;

    return Row(
      mainAxisAlignment: MainAxisAlignment.center,
      children: <Widget>[
        Icon(Symbols.info_rounded, size: 15, color: muted),
        const SizedBox(width: 7),
        Flexible(
          child: Text(
            l10n.opnameVarianceFootnote,
            style: TextStyle(fontSize: 11.5, color: muted),
          ),
        ),
      ],
    );
  }
}

/// Skeleton loading variance: kerangka ringkasan + kartu item (mockup).
class _LoadingSkeleton extends StatelessWidget {
  const _LoadingSkeleton();

  @override
  Widget build(BuildContext context) {
    Widget card() => Container(
      padding: const EdgeInsets.fromLTRB(14, 13, 14, 13),
      decoration: BoxDecoration(
        color:
            Theme.of(context).cardTheme.color ??
            Theme.of(context).colorScheme.surface,
        borderRadius: BorderRadius.circular(16),
        border: Border.all(color: Theme.of(context).colorScheme.outlineVariant),
      ),
      child: const Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: <Widget>[
          AppSkeleton(height: 13, width: 190, borderRadius: 7),
          SizedBox(height: 8),
          AppSkeleton(height: 10, width: 240, borderRadius: 5),
          SizedBox(height: 12),
          Row(
            mainAxisAlignment: MainAxisAlignment.spaceBetween,
            children: <Widget>[
              AppSkeleton(height: 11, width: 120, borderRadius: 6),
              AppSkeleton(height: 24, width: 96, borderRadius: 999),
            ],
          ),
        ],
      ),
    );

    return ListView(
      padding: const EdgeInsets.fromLTRB(20, 0, 20, 20),
      children: <Widget>[
        const Row(
          children: <Widget>[
            Expanded(child: AppSkeleton(height: 62, borderRadius: 14)),
            SizedBox(width: 8),
            Expanded(child: AppSkeleton(height: 62, borderRadius: 14)),
            SizedBox(width: 8),
            Expanded(child: AppSkeleton(height: 62, borderRadius: 14)),
            SizedBox(width: 8),
            Expanded(child: AppSkeleton(height: 62, borderRadius: 14)),
          ],
        ),
        const SizedBox(height: 14),
        const AppSkeleton(height: 11, width: 140, borderRadius: 6),
        const SizedBox(height: 10),
        card(),
        const SizedBox(height: 10),
        card(),
      ],
    );
  }
}

/// Cabang error variance: offline, 404, 403, dan generik.
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
        title: l10n.opnameDetailErrorTitle,
        subtitle: l10n.opnameErrorNetworkBody,
        actionLabel: l10n.commonRetry,
        onAction: onRetry,
      ),
      NotFoundFailure() => EmptyState(
        icon: Symbols.search_off_rounded,
        title: l10n.opnameDetailNotFoundTitle,
        subtitle: l10n.opnameDetailNotFoundBody,
      ),
      ForbiddenFailure() => EmptyState(
        icon: Symbols.lock_rounded,
        title: l10n.opnameForbiddenTitle,
        subtitle: l10n.opnameForbiddenBody,
      ),
      _ => EmptyState(
        icon: Symbols.error_rounded,
        title: l10n.opnameDetailErrorTitle,
        subtitle: l10n.opnameErrorGenericBody,
        actionLabel: l10n.commonRetry,
        onAction: onRetry,
      ),
    };
  }
}
