import 'dart:async';

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
import '../data/notification_dto.dart';
import 'notification_presentation.dart';
import 'notifications_controller.dart';
import 'unread_count_provider.dart';

/// Layar Notifikasi 1:1 mockup "Inventra Mobile - Notifikasi": aksi "Tandai
/// semua dibaca" di header, feed berseksi per hari (Hari ini/Kemarin/tanggal),
/// kartu per jenis (ikon berwarna, judul+isi dirender klien dari type+params
/// via i18n — ADR-0014), penanda unread (kartu hijau + titik), tap = tandai
/// dibaca + navigasi ke rute terkait, pull-to-refresh, infinite scroll
/// limit/offset, empty state, skeleton, dan error + retry.
class NotificationsScreen extends ConsumerStatefulWidget {
  const NotificationsScreen({super.key});

  @override
  ConsumerState<NotificationsScreen> createState() =>
      _NotificationsScreenState();
}

class _NotificationsScreenState extends ConsumerState<NotificationsScreen> {
  Future<void> _refresh() async {
    ref.invalidate(notificationsUnreadCountProvider);
    ref.invalidate(notificationsFeedProvider);
    try {
      await ref.read(notificationsFeedProvider.future);
    } on Object {
      // Kegagalan refresh sudah tercermin sebagai state error feed.
    }
  }

  Future<void> _markAllRead() async {
    final bool ok = await ref
        .read(notificationsFeedProvider.notifier)
        .markAllRead();
    if (!ok && mounted) {
      ScaffoldMessenger.of(context).showSnackBar(
        SnackBar(
          content: Text(
            AppLocalizations.of(context).notificationsMarkAllFailed,
          ),
        ),
      );
    }
  }

  void _onCardTap(NotificationDto notification) {
    if (notification.readAt == null) {
      // Optimistis + non-fatal di controller; navigasi tidak menunggu server.
      unawaited(
        ref.read(notificationsFeedProvider.notifier).markRead(notification.id),
      );
    }
    final String? target = notificationTargetLocation(notification);
    if (target != null) {
      context.push(target);
    }
  }

  @override
  Widget build(BuildContext context) {
    final AppLocalizations l10n = AppLocalizations.of(context);
    final AsyncValue<NotificationsState> state = ref.watch(
      notificationsFeedProvider,
    );
    final bool hasUnread = state.value?.hasUnread ?? false;

    return Scaffold(
      appBar: AppBar(
        title: Text(l10n.notificationsTitle),
        actions: <Widget>[
          // Aksi tampil hanya saat ada yang belum dibaca (mockup empty state
          // tidak menampilkannya; menandai feed yang sudah terbaca semua
          // adalah no-op).
          if (hasUnread)
            Padding(
              padding: const EdgeInsets.only(right: 8),
              child: TextButton(
                onPressed: _markAllRead,
                child: Text(
                  l10n.notificationsMarkAllRead,
                  style: const TextStyle(
                    fontSize: 12.5,
                    fontWeight: FontWeight.w700,
                  ),
                ),
              ),
            ),
        ],
      ),
      body: SafeArea(
        child: state.when(
          data: (NotificationsState data) => _Feed(
            state: data,
            onRefresh: _refresh,
            onLoadMore: () =>
                ref.read(notificationsFeedProvider.notifier).loadMore(),
            onCardTap: _onCardTap,
          ),
          loading: () => const _LoadingSkeleton(),
          error: (Object error, StackTrace stackTrace) => _ErrorState(
            failure: error,
            onRetry: () => ref.invalidate(notificationsFeedProvider),
          ),
        ),
      ),
    );
  }
}

/// Feed berseksi + pull-to-refresh + infinite scroll; empty state bila kosong.
class _Feed extends ConsumerWidget {
  const _Feed({
    required this.state,
    required this.onRefresh,
    required this.onLoadMore,
    required this.onCardTap,
  });

  final NotificationsState state;
  final Future<void> Function() onRefresh;
  final VoidCallback onLoadMore;
  final ValueChanged<NotificationDto> onCardTap;

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final AppLocalizations l10n = AppLocalizations.of(context);
    final String localeName = Localizations.localeOf(context).languageCode;

    if (state.items.isEmpty) {
      return EmptyState(
        icon: Symbols.notifications_off_rounded,
        title: l10n.notificationsEmptyTitle,
        subtitle: l10n.notificationsEmptyBody,
      );
    }

    final DateTime now = ref.watch(clockProvider)();
    // Feed sudah terurut terbaru lebih dulu (kontrak); header seksi disisip
    // setiap label hari berubah.
    final List<Widget> children = <Widget>[];
    String? lastSection;
    for (final NotificationDto item in state.items) {
      final String section = notificationSectionLabel(
        l10n,
        now,
        item.createdAt,
        localeName,
      );
      if (section != lastSection) {
        lastSection = section;
        children.add(_SectionHeader(label: section));
      }
      children.add(
        Padding(
          padding: const EdgeInsets.only(bottom: 8),
          child: _NotificationCard(
            notification: item,
            now: now,
            onTap: () => onCardTap(item),
          ),
        ),
      );
    }
    final bool showFooter =
        state.isLoadingMore || state.loadMoreFailed || state.hasMore;
    if (showFooter) {
      children.add(
        _ListFooter(
          isLoading: state.isLoadingMore,
          failed: state.loadMoreFailed,
          onRetry: onLoadMore,
        ),
      );
    }

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
        child: ListView(
          physics: const AlwaysScrollableScrollPhysics(),
          padding: const EdgeInsets.fromLTRB(20, 0, 20, 24),
          children: children,
        ),
      ),
    );
  }
}

/// Header seksi hari (uppercase kecil, mockup).
class _SectionHeader extends StatelessWidget {
  const _SectionHeader({required this.label});

  final String label;

  @override
  Widget build(BuildContext context) {
    final ThemeData theme = Theme.of(context);

    return Padding(
      padding: const EdgeInsets.fromLTRB(0, 6, 0, 8),
      child: Text(
        label.toUpperCase(),
        style: TextStyle(
          fontSize: 11.5,
          fontWeight: FontWeight.w700,
          letterSpacing: 0.7,
          color: theme.textTheme.labelSmall?.color,
        ),
      ),
    );
  }
}

/// Kartu notifikasi 1:1 mockup: tile ikon bundar berwarna per jenis, judul
/// (tebal saat unread), isi satu baris, label waktu; kartu unread berlatar
/// hijau lembut + titik primary di kanan.
class _NotificationCard extends ConsumerWidget {
  const _NotificationCard({
    required this.notification,
    required this.now,
    required this.onTap,
  });

  final NotificationDto notification;
  final DateTime now;
  final VoidCallback onTap;

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final ThemeData theme = Theme.of(context);
    final ColorScheme scheme = theme.colorScheme;
    final AppLocalizations l10n = AppLocalizations.of(context);
    final String localeName = Localizations.localeOf(context).languageCode;
    final bool unread = notification.readAt == null;
    final StatusColorSet tile = statusColorSetOf(
      context,
      notificationVariant(notification),
    );
    final String? body = notificationBody(l10n, notification, localeName);

    // Latar unread: primaryContainer tipis di atas latar layar (mockup
    // green-50 light / hijau gelap dark); border primaryContainer.
    final Color background = unread
        ? Color.alphaBlend(
            scheme.primaryContainer.withValues(alpha: 0.35),
            theme.scaffoldBackgroundColor,
          )
        : theme.cardTheme.color ?? scheme.surface;
    final Color border = unread
        ? scheme.primaryContainer
        : scheme.outlineVariant;

    return Material(
      key: ValueKey<String>('notification-${notification.id}'),
      color: background,
      shape: RoundedRectangleBorder(
        borderRadius: BorderRadius.circular(16),
        side: BorderSide(color: border),
      ),
      clipBehavior: Clip.antiAlias,
      child: InkWell(
        onTap: onTap,
        child: Padding(
          padding: const EdgeInsets.fromLTRB(14, 13, 14, 13),
          child: Row(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: <Widget>[
              Container(
                width: 40,
                height: 40,
                decoration: BoxDecoration(
                  color: tile.bg,
                  shape: BoxShape.circle,
                ),
                child: Icon(
                  notificationIcon(notification),
                  size: 20,
                  color: tile.text,
                ),
              ),
              const SizedBox(width: 12),
              Expanded(
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: <Widget>[
                    Text(
                      notificationTitle(l10n, notification),
                      style: TextStyle(
                        fontSize: 13.5,
                        fontWeight: unread ? FontWeight.w700 : FontWeight.w600,
                        color: scheme.onSurface,
                      ),
                    ),
                    if (body != null) ...<Widget>[
                      const SizedBox(height: 2),
                      Text(
                        body,
                        maxLines: 1,
                        overflow: TextOverflow.ellipsis,
                        style: TextStyle(
                          fontSize: 12,
                          color: theme.textTheme.bodySmall?.color,
                        ),
                      ),
                    ],
                    const SizedBox(height: 4),
                    Text(
                      notificationTimeLabel(
                        l10n,
                        now,
                        notification.createdAt,
                        localeName,
                      ),
                      style: TextStyle(
                        fontSize: 11,
                        color: theme.textTheme.labelSmall?.color,
                      ),
                    ),
                  ],
                ),
              ),
              if (unread) ...<Widget>[
                const SizedBox(width: 8),
                Padding(
                  padding: const EdgeInsets.only(top: 5),
                  child: Container(
                    key: ValueKey<String>(
                      'notification-unread-${notification.id}',
                    ),
                    width: 9,
                    height: 9,
                    decoration: BoxDecoration(
                      color: scheme.primary,
                      shape: BoxShape.circle,
                    ),
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

/// Kaki feed: spinner saat memuat halaman berikutnya; baris retry bila gagal;
/// kosong bila masih ada halaman tetapi belum diminta.
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
              l10n.notificationsLoadMoreFailed,
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

/// Skeleton loading: header seksi + empat kerangka kartu (mockup).
class _LoadingSkeleton extends StatelessWidget {
  const _LoadingSkeleton();

  @override
  Widget build(BuildContext context) {
    final ThemeData theme = Theme.of(context);
    final ColorScheme scheme = theme.colorScheme;

    Widget card() => Container(
      padding: const EdgeInsets.fromLTRB(14, 13, 14, 13),
      decoration: BoxDecoration(
        color: theme.cardTheme.color ?? scheme.surface,
        borderRadius: BorderRadius.circular(16),
        border: Border.all(color: scheme.outlineVariant),
      ),
      child: const Row(
        children: <Widget>[
          AppSkeleton(height: 40, width: 40, borderRadius: 999),
          SizedBox(width: 12),
          Expanded(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: <Widget>[
                AppSkeleton(height: 13, width: 200, borderRadius: 7),
                SizedBox(height: 8),
                AppSkeleton(height: 10, width: 250, borderRadius: 5),
                SizedBox(height: 7),
                AppSkeleton(height: 9, width: 90, borderRadius: 5),
              ],
            ),
          ),
        ],
      ),
    );

    return ListView(
      padding: const EdgeInsets.fromLTRB(20, 6, 20, 24),
      children: <Widget>[
        const AppSkeleton(height: 11, width: 70, borderRadius: 6),
        const SizedBox(height: 10),
        for (int i = 0; i < 4; i++) ...<Widget>[
          if (i > 0) const SizedBox(height: 8),
          card(),
        ],
      ],
    );
  }
}

/// Cabang error feed: offline dan generik (endpoint tanpa permission khusus).
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
        title: l10n.notificationsErrorTitle,
        subtitle: l10n.notificationsErrorNetworkBody,
        actionLabel: l10n.commonRetry,
        onAction: onRetry,
      ),
      _ => EmptyState(
        icon: Symbols.error_rounded,
        title: l10n.notificationsErrorTitle,
        subtitle: l10n.notificationsErrorGenericBody,
        actionLabel: l10n.commonRetry,
        onAction: onRetry,
      ),
    };
  }
}
