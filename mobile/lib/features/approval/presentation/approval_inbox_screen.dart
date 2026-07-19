import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';
import 'package:material_symbols_icons/symbols.dart';

import '../../../app/theme.dart';
import '../../../core/api/app_failure.dart';
import '../../../core/i18n/gen/app_localizations.dart';
import '../../../core/utils/clock.dart';
import '../../../core/widgets/app_skeleton.dart';
import '../../../core/widgets/empty_state.dart';
import '../../../core/widgets/status_chip.dart';
import '../data/approval_repository.dart';
import '../data/request_dto.dart';
import 'approval_inbox_controller.dart';
import 'inbox_count_provider.dart';
import 'request_presentation.dart';

/// Nilai kosong (maker/kantor tak ter-resolve backend).
const String _emDash = '—';

/// Layar Inbox Approval 1:1 mockup "Inventra Mobile - Inbox Approval":
/// chip filter status (Menunggu/Disetujui/Ditolak/Semua), kartu pengajuan
/// (ikon jenis, judul, maker · kantor, nominal, chip status), pull-to-refresh,
/// infinite scroll limit/offset, empty state per filter, skeleton, dan error
/// + retry. Data dari `GET /requests` (data-scope backend).
class ApprovalInboxScreen extends ConsumerStatefulWidget {
  const ApprovalInboxScreen({super.key});

  @override
  ConsumerState<ApprovalInboxScreen> createState() =>
      _ApprovalInboxScreenState();
}

class _ApprovalInboxScreenState extends ConsumerState<ApprovalInboxScreen> {
  ApprovalStatusFilter _filter = ApprovalStatusFilter.pending;

  Future<void> _refresh() async {
    ref.invalidate(approvalInboxCountProvider);
    ref.invalidate(approvalInboxProvider(_filter));
    try {
      await ref.read(approvalInboxProvider(_filter).future);
    } on Object {
      // Kegagalan refresh sudah tercermin sebagai state error daftar.
    }
  }

  @override
  Widget build(BuildContext context) {
    final AppLocalizations l10n = AppLocalizations.of(context);
    final AsyncValue<ApprovalInboxState> state = ref.watch(
      approvalInboxProvider(_filter),
    );
    // Angka chip Menunggu = total daftar pending (paritas badge mockup);
    // tab pending adalah default sehingga nilainya sudah termuat sejak awal.
    final int pendingTotal =
        ref
            .watch(approvalInboxProvider(ApprovalStatusFilter.pending))
            .value
            ?.total ??
        0;

    return Scaffold(
      appBar: AppBar(title: Text(l10n.approvalInboxTitle)),
      body: SafeArea(
        child: Column(
          children: <Widget>[
            _FilterRow(
              selected: _filter,
              pendingCount: pendingTotal,
              onSelect: (ApprovalStatusFilter filter) =>
                  setState(() => _filter = filter),
            ),
            Expanded(
              child: state.when(
                data: (ApprovalInboxState data) => _InboxList(
                  filter: _filter,
                  state: data,
                  onRefresh: _refresh,
                  onLoadMore: () => ref
                      .read(approvalInboxProvider(_filter).notifier)
                      .loadMore(),
                  onShowHistory: () =>
                      setState(() => _filter = ApprovalStatusFilter.all),
                ),
                loading: () => const _LoadingSkeleton(),
                error: (Object error, StackTrace stackTrace) => _ErrorState(
                  failure: error,
                  onRetry: () => ref.invalidate(approvalInboxProvider(_filter)),
                ),
              ),
            ),
          ],
        ),
      ),
    );
  }
}

/// Baris chip filter status (scroll horizontal): pill aktif primary; chip
/// Menunggu membawa angka total.
class _FilterRow extends StatelessWidget {
  const _FilterRow({
    required this.selected,
    required this.pendingCount,
    required this.onSelect,
  });

  final ApprovalStatusFilter selected;
  final int pendingCount;
  final ValueChanged<ApprovalStatusFilter> onSelect;

  @override
  Widget build(BuildContext context) {
    final AppLocalizations l10n = AppLocalizations.of(context);

    String label(ApprovalStatusFilter filter) => switch (filter) {
      ApprovalStatusFilter.pending => l10n.approvalInboxFilterPending,
      ApprovalStatusFilter.approved => l10n.approvalInboxFilterApproved,
      ApprovalStatusFilter.rejected => l10n.approvalInboxFilterRejected,
      ApprovalStatusFilter.all => l10n.approvalInboxFilterAll,
    };

    return SingleChildScrollView(
      scrollDirection: Axis.horizontal,
      padding: const EdgeInsets.fromLTRB(20, 6, 20, 12),
      child: Row(
        children: <Widget>[
          for (final ApprovalStatusFilter filter
              in ApprovalStatusFilter.values) ...<Widget>[
            if (filter != ApprovalStatusFilter.values.first)
              const SizedBox(width: 8),
            _FilterChip(
              label: label(filter),
              count: filter == ApprovalStatusFilter.pending && pendingCount > 0
                  ? pendingCount
                  : null,
              active: filter == selected,
              onTap: () => onSelect(filter),
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
    this.count,
  });

  final String label;
  final bool active;
  final VoidCallback onTap;
  final int? count;

  @override
  Widget build(BuildContext context) {
    final ThemeData theme = Theme.of(context);
    final ColorScheme scheme = theme.colorScheme;
    final int? countValue = count;

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
            padding: const EdgeInsets.symmetric(horizontal: 14, vertical: 8),
            child: Row(
              mainAxisSize: MainAxisSize.min,
              children: <Widget>[
                Text(
                  label,
                  style: TextStyle(
                    fontSize: 12.5,
                    fontWeight: active ? FontWeight.w700 : FontWeight.w600,
                    color: active
                        ? scheme.onPrimary
                        : theme.textTheme.labelMedium?.color,
                  ),
                ),
                if (countValue != null) ...<Widget>[
                  const SizedBox(width: 6),
                  Container(
                    padding: const EdgeInsets.symmetric(
                      horizontal: 7,
                      vertical: 1,
                    ),
                    decoration: ShapeDecoration(
                      color: active
                          ? scheme.onPrimary.withValues(alpha: 0.25)
                          : scheme.secondaryContainer,
                      shape: const StadiumBorder(),
                    ),
                    child: Text(
                      '$countValue',
                      style: TextStyle(
                        fontSize: 11,
                        fontWeight: FontWeight.w600,
                        color: active
                            ? scheme.onPrimary
                            : theme.textTheme.labelMedium?.color,
                      ),
                    ),
                  ),
                ],
              ],
            ),
          ),
        ),
      ),
    );
  }
}

/// Daftar kartu + pull-to-refresh + infinite scroll; empty state per filter.
class _InboxList extends ConsumerWidget {
  const _InboxList({
    required this.filter,
    required this.state,
    required this.onRefresh,
    required this.onLoadMore,
    required this.onShowHistory,
  });

  final ApprovalStatusFilter filter;
  final ApprovalInboxState state;
  final Future<void> Function() onRefresh;
  final VoidCallback onLoadMore;
  final VoidCallback onShowHistory;

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final AppLocalizations l10n = AppLocalizations.of(context);

    if (state.items.isEmpty) {
      if (filter == ApprovalStatusFilter.pending) {
        return EmptyState(
          icon: Symbols.task_alt_rounded,
          title: l10n.approvalInboxEmptyPendingTitle,
          subtitle: l10n.approvalInboxEmptyPendingBody,
          actionLabel: l10n.approvalInboxEmptyPendingAction,
          onAction: onShowHistory,
        );
      }
      return EmptyState(
        icon: Symbols.inbox_rounded,
        title: l10n.approvalInboxEmptyFilteredTitle,
        subtitle: l10n.approvalInboxEmptyFilteredBody,
      );
    }

    final bool showFooter =
        state.isLoadingMore || state.loadMoreFailed || state.hasMore;

    return NotificationListener<ScrollNotification>(
      onNotification: (ScrollNotification notification) {
        if (notification.metrics.axis == Axis.vertical &&
            notification.metrics.pixels >=
                notification.metrics.maxScrollExtent - 320) {
          onLoadMore();
        }
        return false;
      },
      child: RefreshIndicator(
        onRefresh: onRefresh,
        child: ListView.separated(
          physics: const AlwaysScrollableScrollPhysics(),
          padding: const EdgeInsets.fromLTRB(20, 0, 20, 24),
          itemCount: state.items.length + 1 + (showFooter ? 1 : 0),
          separatorBuilder: (BuildContext context, int index) =>
              const SizedBox(height: 10),
          itemBuilder: (BuildContext context, int index) {
            if (index == 0) {
              return const _PullHint();
            }
            if (index == state.items.length + 1) {
              return _ListFooter(
                isLoading: state.isLoadingMore,
                failed: state.loadMoreFailed,
                onRetry: onLoadMore,
              );
            }
            final RequestDto request = state.items[index - 1];
            return _RequestCard(
              request: request,
              onTap: () => context.push('/approval/${request.id}'),
            );
          },
        ),
      ),
    );
  }
}

/// Petunjuk "Tarik untuk menyegarkan" di atas kartu pertama (mockup).
class _PullHint extends StatelessWidget {
  const _PullHint();

  @override
  Widget build(BuildContext context) {
    final ThemeData theme = Theme.of(context);
    final AppLocalizations l10n = AppLocalizations.of(context);

    return Padding(
      padding: const EdgeInsets.only(top: 2),
      child: Row(
        mainAxisAlignment: MainAxisAlignment.center,
        children: <Widget>[
          Icon(
            Symbols.arrow_downward_rounded,
            size: 14,
            color: theme.textTheme.labelSmall?.color,
          ),
          const SizedBox(width: 6),
          Text(
            l10n.approvalInboxPullToRefresh,
            style: TextStyle(
              fontSize: 11,
              color: theme.textTheme.labelSmall?.color,
            ),
          ),
        ],
      ),
    );
  }
}

/// Kaki daftar: spinner saat memuat halaman berikutnya; baris retry bila
/// gagal; kosong bila masih ada halaman tetapi belum diminta.
class _ListFooter extends StatelessWidget {
  const _ListFooter({
    required this.isLoading,
    required this.failed,
    required this.onRetry,
  });

  final bool isLoading;
  final bool failed;
  final VoidCallback onRetry;

  @override
  Widget build(BuildContext context) {
    final AppLocalizations l10n = AppLocalizations.of(context);

    if (isLoading) {
      return const Padding(
        padding: EdgeInsets.symmetric(vertical: 14),
        child: Center(
          child: SizedBox(
            width: 22,
            height: 22,
            child: CircularProgressIndicator(strokeWidth: 2.5),
          ),
        ),
      );
    }
    if (failed) {
      return Padding(
        padding: const EdgeInsets.symmetric(vertical: 4),
        child: Row(
          mainAxisAlignment: MainAxisAlignment.center,
          children: <Widget>[
            Text(
              l10n.approvalInboxLoadMoreFailed,
              style: TextStyle(
                fontSize: 12,
                color: Theme.of(context).colorScheme.onSurfaceVariant,
              ),
            ),
            TextButton(onPressed: onRetry, child: Text(l10n.commonRetry)),
          ],
        ),
      );
    }
    return const SizedBox(height: 4);
  }
}

/// Kartu pengajuan 1:1 mockup: ikon jenis, label jenis, penanda sensitif,
/// waktu relatif, judul, maker · kantor, nominal (bila dikirim backend), dan
/// chip status.
class _RequestCard extends ConsumerWidget {
  const _RequestCard({required this.request, required this.onTap});

  final RequestDto request;
  final VoidCallback onTap;

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final ThemeData theme = Theme.of(context);
    final ColorScheme scheme = theme.colorScheme;
    final AppLocalizations l10n = AppLocalizations.of(context);
    final String localeName = Localizations.localeOf(context).languageCode;
    final StatusColorSet chipColors = statusColorSetOf(
      context,
      requestTypeVariant(request.type),
    );
    final (String statusLabel, StatusChipVariant statusVariant) =
        requestStatusPresentation(l10n, request.status);
    final String? amount = request.amount;
    final DateTime? createdAt = request.createdAt;
    final String makerLine = <String>[
      request.requestedByName ?? _emDash,
      if (request.officeName != null) request.officeName!,
    ].join(' · ');

    return Material(
      color: theme.cardTheme.color ?? scheme.surface,
      shape: RoundedRectangleBorder(
        borderRadius: BorderRadius.circular(18),
        side: BorderSide(color: scheme.outlineVariant),
      ),
      clipBehavior: Clip.antiAlias,
      child: InkWell(
        onTap: onTap,
        child: Padding(
          padding: const EdgeInsets.fromLTRB(16, 14, 16, 14),
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: <Widget>[
              Row(
                children: <Widget>[
                  Container(
                    width: 32,
                    height: 32,
                    decoration: BoxDecoration(
                      color: chipColors.bg,
                      borderRadius: BorderRadius.circular(10),
                    ),
                    child: Icon(
                      requestTypeIcon(request.type),
                      size: 17,
                      color: chipColors.text,
                    ),
                  ),
                  const SizedBox(width: 8),
                  Flexible(
                    child: Text(
                      requestTypeLabel(l10n, request.type),
                      overflow: TextOverflow.ellipsis,
                      style: TextStyle(
                        fontSize: 11.5,
                        fontWeight: FontWeight.w700,
                        color: chipColors.text,
                      ),
                    ),
                  ),
                  if (isSensitiveRequestType(request.type)) ...<Widget>[
                    const SizedBox(width: 8),
                    _SensitiveMarker(label: l10n.approvalCardSensitive),
                  ],
                  const Spacer(),
                  if (createdAt != null)
                    Text(
                      formatRelativeTime(
                        l10n,
                        ref.watch(clockProvider)(),
                        createdAt,
                        localeName,
                      ),
                      style: TextStyle(
                        fontSize: 11,
                        color: theme.textTheme.labelSmall?.color,
                      ),
                    ),
                ],
              ),
              const SizedBox(height: 8),
              Text(
                requestTitle(l10n, request.type, request.reason),
                maxLines: 2,
                overflow: TextOverflow.ellipsis,
                style: TextStyle(
                  fontSize: 14.5,
                  fontWeight: FontWeight.w700,
                  color: scheme.onSurface,
                ),
              ),
              const SizedBox(height: 4),
              Text(
                makerLine,
                maxLines: 1,
                overflow: TextOverflow.ellipsis,
                style: TextStyle(
                  fontSize: 12,
                  color: theme.textTheme.bodySmall?.color,
                ),
              ),
              const SizedBox(height: 10),
              Row(
                mainAxisAlignment: MainAxisAlignment.spaceBetween,
                children: <Widget>[
                  if (amount != null)
                    Flexible(
                      child: Text(
                        formatIdrAmount(amount, localeName),
                        overflow: TextOverflow.ellipsis,
                        style: TextStyle(
                          fontSize: 13.5,
                          fontWeight: FontWeight.w700,
                          color: scheme.onSurface,
                        ),
                      ),
                    )
                  else
                    const SizedBox.shrink(),
                  StatusChip(label: statusLabel, variant: statusVariant),
                ],
              ),
            ],
          ),
        ),
      ),
    );
  }
}

/// Penanda "sensitif": titik amber + label kecil (mockup kartu penghapusan).
class _SensitiveMarker extends StatelessWidget {
  const _SensitiveMarker({required this.label});

  final String label;

  @override
  Widget build(BuildContext context) {
    final StatusColorSet warning = statusColorSetOf(
      context,
      StatusChipVariant.warning,
    );

    return Row(
      mainAxisSize: MainAxisSize.min,
      children: <Widget>[
        Container(
          width: 7,
          height: 7,
          decoration: BoxDecoration(color: warning.dot, shape: BoxShape.circle),
        ),
        const SizedBox(width: 4),
        Text(
          label,
          style: TextStyle(
            fontSize: 10.5,
            fontWeight: FontWeight.w600,
            color: warning.text,
          ),
        ),
      ],
    );
  }
}

/// Skeleton loading: empat kerangka kartu (mockup state loading).
class _LoadingSkeleton extends StatelessWidget {
  const _LoadingSkeleton();

  @override
  Widget build(BuildContext context) {
    final ThemeData theme = Theme.of(context);
    final ColorScheme scheme = theme.colorScheme;

    Widget card() => Container(
      padding: const EdgeInsets.fromLTRB(16, 14, 16, 14),
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
              AppSkeleton(height: 32, width: 32, borderRadius: 10),
              SizedBox(width: 8),
              AppSkeleton(height: 11, width: 90, borderRadius: 6),
            ],
          ),
          SizedBox(height: 12),
          AppSkeleton(height: 14, width: 240, borderRadius: 7),
          SizedBox(height: 9),
          AppSkeleton(height: 11, width: 170, borderRadius: 6),
          SizedBox(height: 12),
          Row(
            mainAxisAlignment: MainAxisAlignment.spaceBetween,
            children: <Widget>[
              AppSkeleton(height: 13, width: 110, borderRadius: 7),
              AppSkeleton(height: 24, width: 90, borderRadius: 999),
            ],
          ),
        ],
      ),
    );

    return ListView(
      padding: const EdgeInsets.fromLTRB(20, 0, 20, 24),
      children: <Widget>[
        for (int i = 0; i < 4; i++) ...<Widget>[
          if (i > 0) const SizedBox(height: 10),
          card(),
        ],
      ],
    );
  }
}

/// Cabang error daftar: offline, 403 (tanpa akses), dan generik.
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
        title: l10n.approvalInboxErrorTitle,
        subtitle: l10n.approvalInboxErrorNetworkBody,
        actionLabel: l10n.commonRetry,
        onAction: onRetry,
      ),
      ForbiddenFailure() => EmptyState(
        icon: Symbols.lock_rounded,
        title: l10n.approvalInboxForbiddenTitle,
        subtitle: l10n.approvalInboxForbiddenBody,
      ),
      _ => EmptyState(
        icon: Symbols.error_rounded,
        title: l10n.approvalInboxErrorTitle,
        subtitle: l10n.approvalInboxErrorGenericBody,
        actionLabel: l10n.commonRetry,
        onAction: onRetry,
      ),
    };
  }
}
