import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';
import 'package:material_symbols_icons/symbols.dart';

import '../../../app/theme.dart';
import '../../../core/api/app_failure.dart';
import '../../../core/i18n/gen/app_localizations.dart';
import '../../../core/i18n/request_type_label.dart';
import '../../../core/utils/clock.dart';
import '../../../core/widgets/app_skeleton.dart';
import '../../../core/widgets/confirm_dialog.dart';
import '../../../core/widgets/empty_state.dart';
import '../../../core/widgets/status_chip.dart';
import '../../approval/data/approval_repository.dart' show ApprovalStatusFilter;
import '../../approval/data/request_dto.dart';
import '../../approval/presentation/request_presentation.dart';
import 'my_requests_controller.dart';

/// Pengajuan Saya (1:1 mockup "Inventra Mobile - Pengajuan Saya"): lensa MAKER
/// atas pengajuan yang dibuat pengguna sendiri (`GET /requests?mine=true`),
/// filter status, kartu tanpa aksi keputusan, dan tombol Batalkan untuk
/// pengajuan `pending` sendiri (`POST /requests/:id/cancel`). Beda dari Inbox
/// Approval yang berorientasi keputusan.
class MyRequestsScreen extends ConsumerStatefulWidget {
  const MyRequestsScreen({super.key});

  @override
  ConsumerState<MyRequestsScreen> createState() => _MyRequestsScreenState();
}

class _MyRequestsScreenState extends ConsumerState<MyRequestsScreen> {
  ApprovalStatusFilter _filter = ApprovalStatusFilter.pending;

  Future<void> _refresh() async {
    ref.invalidate(myRequestsProvider(_filter));
    try {
      await ref.read(myRequestsProvider(_filter).future);
    } on Object {
      // Kegagalan refresh tercermin sebagai state error daftar.
    }
  }

  Future<void> _cancel(RequestDto request) async {
    final AppLocalizations l10n = AppLocalizations.of(context);
    final bool confirmed = await ConfirmDialog.show(
      context,
      title: l10n.myRequestsCancelConfirmTitle,
      message: l10n.myRequestsCancelConfirmBody,
      confirmLabel: l10n.myRequestsCancel,
      destructive: true,
    );
    if (!confirmed) {
      return;
    }
    try {
      await ref.read(myRequestsProvider(_filter).notifier).cancel(request.id);
      if (!mounted) {
        return;
      }
      ScaffoldMessenger.of(
        context,
      ).showSnackBar(SnackBar(content: Text(l10n.myRequestsCancelSuccess)));
    } on Object {
      if (!mounted) {
        return;
      }
      ScaffoldMessenger.of(
        context,
      ).showSnackBar(SnackBar(content: Text(l10n.myRequestsCancelError)));
    }
  }

  @override
  Widget build(BuildContext context) {
    final AppLocalizations l10n = AppLocalizations.of(context);
    final AsyncValue<MyRequestsState> state = ref.watch(
      myRequestsProvider(_filter),
    );

    return Scaffold(
      appBar: AppBar(title: Text(l10n.myRequestsTitle)),
      body: SafeArea(
        child: Column(
          children: <Widget>[
            _FilterRow(
              selected: _filter,
              onSelect: (ApprovalStatusFilter filter) =>
                  setState(() => _filter = filter),
            ),
            Expanded(
              child: state.when(
                data: (MyRequestsState data) => _RequestList(
                  state: data,
                  onRefresh: _refresh,
                  onLoadMore: () =>
                      ref.read(myRequestsProvider(_filter).notifier).loadMore(),
                  onCancel: _cancel,
                ),
                loading: () => const _LoadingSkeleton(),
                error: (Object error, StackTrace stackTrace) => _ErrorState(
                  failure: error,
                  onRetry: () => ref.invalidate(myRequestsProvider(_filter)),
                ),
              ),
            ),
          ],
        ),
      ),
    );
  }
}

/// Baris chip filter status (tanpa badge angka — ini lensa maker, bukan inbox).
class _FilterRow extends StatelessWidget {
  const _FilterRow({required this.selected, required this.onSelect});

  final ApprovalStatusFilter selected;
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
            padding: const EdgeInsets.symmetric(horizontal: 14, vertical: 8),
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

class _RequestList extends ConsumerWidget {
  const _RequestList({
    required this.state,
    required this.onRefresh,
    required this.onLoadMore,
    required this.onCancel,
  });

  final MyRequestsState state;
  final Future<void> Function() onRefresh;
  final VoidCallback onLoadMore;
  final Future<void> Function(RequestDto) onCancel;

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final AppLocalizations l10n = AppLocalizations.of(context);

    if (state.items.isEmpty) {
      return EmptyState(
        icon: Symbols.description_rounded,
        title: l10n.myRequestsEmptyTitle,
        subtitle: l10n.myRequestsEmptyBody,
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
          itemCount: state.items.length + (showFooter ? 1 : 0),
          separatorBuilder: (BuildContext context, int index) =>
              const SizedBox(height: 10),
          itemBuilder: (BuildContext context, int index) {
            if (index == state.items.length) {
              return _ListFooter(
                isLoading: state.isLoadingMore,
                failed: state.loadMoreFailed,
                onRetry: onLoadMore,
              );
            }
            final RequestDto request = state.items[index];
            return _RequestCard(
              request: request,
              onCancel: request.status == 'pending'
                  ? () => onCancel(request)
                  : null,
            );
          },
        ),
      ),
    );
  }
}

/// Kartu pengajuan maker: ikon jenis, label jenis, penanda sensitif, waktu
/// relatif, judul, nominal, chip status, dan (bila pending) tombol Batalkan.
/// Tap membuka detail read-only pengajuan (maker boleh melihat pengajuannya
/// sendiri lepas dari office scope — bypass maker di GET /requests/:id).
class _RequestCard extends ConsumerWidget {
  const _RequestCard({required this.request, this.onCancel});

  final RequestDto request;
  final VoidCallback? onCancel;

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final ThemeData theme = Theme.of(context);
    final ColorScheme scheme = theme.colorScheme;
    final AppLocalizations l10n = AppLocalizations.of(context);
    final String localeName = Localizations.localeOf(context).languageCode;
    final StatusColorSet typeColors = statusColorSetOf(
      context,
      requestTypeVariant(request.type),
    );
    final (String statusLabel, StatusChipVariant statusVariant) =
        requestStatusPresentation(l10n, request.status);
    final String? amount = request.amount;
    final DateTime? createdAt = request.createdAt;

    return Material(
      color: theme.cardTheme.color ?? scheme.surface,
      shape: RoundedRectangleBorder(
        borderRadius: BorderRadius.circular(18),
        side: BorderSide(color: scheme.outlineVariant),
      ),
      clipBehavior: Clip.antiAlias,
      child: InkWell(
        onTap: () => context.push('/my-requests/${request.id}'),
        child: Padding(
          padding: const EdgeInsets.fromLTRB(16, 14, 16, 12),
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: <Widget>[
              Row(
                children: <Widget>[
                  Container(
                    width: 32,
                    height: 32,
                    decoration: BoxDecoration(
                      color: typeColors.bg,
                      borderRadius: BorderRadius.circular(10),
                    ),
                    child: Icon(
                      requestTypeIcon(request.type),
                      size: 17,
                      color: typeColors.text,
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
                        color: typeColors.text,
                      ),
                    ),
                  ),
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
              const SizedBox(height: 10),
              Row(
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
                    ),
                  const Spacer(),
                  StatusChip(label: statusLabel, variant: statusVariant),
                ],
              ),
              if (onCancel != null) ...<Widget>[
                const Divider(height: 20),
                Align(
                  alignment: Alignment.centerRight,
                  child: TextButton.icon(
                    onPressed: onCancel,
                    style: TextButton.styleFrom(foregroundColor: scheme.error),
                    icon: const Icon(Symbols.cancel_rounded, size: 18),
                    label: Text(l10n.myRequestsCancel),
                  ),
                ),
              ],
            ],
          ),
        ),
      ),
    );
  }
}

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
              l10n.myRequestsLoadMoreFailed,
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
        title: l10n.myRequestsErrorTitle,
        subtitle: l10n.myRequestsErrorNetworkBody,
        actionLabel: l10n.commonRetry,
        onAction: onRetry,
      ),
      ForbiddenFailure() => EmptyState(
        icon: Symbols.lock_rounded,
        title: l10n.myRequestsForbiddenTitle,
        subtitle: l10n.myRequestsForbiddenBody,
      ),
      _ => EmptyState(
        icon: Symbols.error_rounded,
        title: l10n.myRequestsErrorTitle,
        subtitle: l10n.myRequestsErrorGenericBody,
        actionLabel: l10n.commonRetry,
        onAction: onRetry,
      ),
    };
  }
}
