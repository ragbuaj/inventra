import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';
import 'package:material_symbols_icons/symbols.dart';

import '../../../app/theme.dart';
import '../../../core/auth/auth_controller.dart';
import '../../../core/auth/auth_session.dart';
import '../../../core/connectivity/connectivity_provider.dart';
import '../../../core/i18n/gen/app_localizations.dart';
import '../../../core/i18n/request_type_label.dart';
import '../../../core/utils/clock.dart';
import '../../../core/widgets/app_skeleton.dart';
import '../../../core/widgets/offline_banner.dart';
import '../../../core/widgets/status_chip.dart';
import '../../../core/widgets/sync_pill.dart';
import '../../approval/data/approval_repository.dart';
import '../../approval/data/request_dto.dart';
import '../../approval/presentation/approval_inbox_controller.dart';
import '../../approval/presentation/inbox_count_provider.dart';
import '../../approval/presentation/request_presentation.dart';
import '../../notifications/presentation/unread_count_provider.dart';
import '../../stock_opname/data/stock_opname_session_dto.dart';
import '../../stock_opname/presentation/opname_presentation.dart';
import 'home_providers.dart';

/// Beranda 1:1 mockup "Inventra Mobile - Beranda": header sapaan (avatar
/// inisial menuju profil, lonceng berbadge unread menuju notifikasi), kartu
/// Sesi Opname Aktif (reuse provider daftar sesi), kartu Approval Menunggu
/// (reuse provider inbox pending), dan quick actions 4 tile.
///
/// SEMUA panggilan ringkasan non-fatal dan independen: tiap kartu punya
/// loading/error/empty sendiri — halaman tidak pernah gagal total karena satu
/// kartu (pelajaran repo: supplementary call fatal memblokir halaman).
class HomeScreen extends ConsumerWidget {
  const HomeScreen({super.key});

  Future<void> _refresh(WidgetRef ref) async {
    ref.invalidate(homeActiveOpnameSessionProvider);
    ref.invalidate(approvalInboxProvider(ApprovalStatusFilter.pending));
    ref.invalidate(approvalInboxCountProvider);
    ref.invalidate(notificationsUnreadCountProvider);
    try {
      await Future.wait(<Future<void>>[
        ref.read(homeActiveOpnameSessionProvider.future),
        ref.read(approvalInboxProvider(ApprovalStatusFilter.pending).future),
      ]);
    } on Object {
      // Kegagalan refresh sudah tercermin sebagai state error kartunya.
    }
  }

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final AppLocalizations l10n = AppLocalizations.of(context);
    final bool offline = isOffline(ref.watch(isOnlineProvider));

    return Scaffold(
      body: SafeArea(
        child: RefreshIndicator(
          onRefresh: () => _refresh(ref),
          child: ListView(
            physics: const AlwaysScrollableScrollPhysics(),
            padding: const EdgeInsets.fromLTRB(20, 4, 20, 24),
            children: <Widget>[
              const _Header(),
              if (offline)
                Padding(
                  padding: const EdgeInsets.only(top: 12),
                  child: OfflineBanner(message: l10n.homeOfflineBanner),
                ),
              const SizedBox(height: 14),
              _OpnameCard(offline: offline),
              const SizedBox(height: 14),
              const _ApprovalCard(),
              const SizedBox(height: 16),
              const _QuickActions(),
            ],
          ),
        ),
      ),
    );
  }
}

/// Inisial nama untuk avatar tanpa foto: huruf depan dua kata pertama.
@visibleForTesting
String avatarInitials(String name) {
  final List<String> words = name
      .trim()
      .split(RegExp(r'\s+'))
      .where((String word) => word.isNotEmpty)
      .toList(growable: false);
  if (words.isEmpty) {
    return '?';
  }
  final StringBuffer buffer = StringBuffer();
  for (final String word in words.take(2)) {
    buffer.write(word[0].toUpperCase());
  }
  return buffer.toString();
}

/// Nama panggilan sapaan: kata pertama nama lengkap.
@visibleForTesting
String greetingName(String name) {
  final List<String> words = name.trim().split(RegExp(r'\s+'));
  return words.isEmpty || words.first.isEmpty ? name : words.first;
}

/// Header 1:1 mockup: avatar inisial (menuju /account), "Halo, {nama}" +
/// nama kantor, dan lonceng notifikasi berbadge unread (menuju tab Notif).
class _Header extends ConsumerWidget {
  const _Header();

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final ThemeData theme = Theme.of(context);
    final ColorScheme scheme = theme.colorScheme;
    final AppLocalizations l10n = AppLocalizations.of(context);
    final AuthSession? session = ref.watch(authControllerProvider).value;
    final String name = session is Authenticated ? session.user.name : '';
    // Subjudul: nama kantor (lookup non-fatal); fallback email selama belum
    // ter-resolve. Nama peran tidak tersedia untuk klien mobile (endpoint
    // roles berada di grup authzadmin yang menolak audience mobile).
    final String? officeName = ref.watch(homeOfficeNameProvider).value;
    final String subtitle =
        officeName ?? (session is Authenticated ? session.user.email : '');
    final int unreadCount = ref.watch(unreadNotificationCountProvider);

    return Row(
      children: <Widget>[
        Semantics(
          button: true,
          label: l10n.homeAccountTooltip,
          child: Material(
            key: const ValueKey<String>('home-avatar'),
            color: scheme.primaryContainer,
            shape: const CircleBorder(),
            clipBehavior: Clip.antiAlias,
            child: InkWell(
              onTap: () => context.push('/account'),
              child: Container(
                width: 42,
                height: 42,
                alignment: Alignment.center,
                child: Text(
                  avatarInitials(name),
                  style: TextStyle(
                    fontSize: 14,
                    fontWeight: FontWeight.w700,
                    color: scheme.onPrimaryContainer,
                  ),
                ),
              ),
            ),
          ),
        ),
        const SizedBox(width: 12),
        Expanded(
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: <Widget>[
              Text(
                l10n.homeGreeting(greetingName(name)),
                maxLines: 1,
                overflow: TextOverflow.ellipsis,
                style: TextStyle(
                  fontSize: 17,
                  fontWeight: FontWeight.w700,
                  color: scheme.onSurface,
                ),
              ),
              if (subtitle.isNotEmpty)
                Text(
                  subtitle,
                  maxLines: 1,
                  overflow: TextOverflow.ellipsis,
                  style: TextStyle(
                    fontSize: 11.5,
                    color: theme.textTheme.bodySmall?.color,
                  ),
                ),
            ],
          ),
        ),
        const SizedBox(width: 12),
        _BellButton(unreadCount: unreadCount),
      ],
    );
  }
}

/// Lonceng notifikasi header dengan badge unread merah (mockup) — menuju tab
/// Notifikasi.
class _BellButton extends StatelessWidget {
  const _BellButton({required this.unreadCount});

  final int unreadCount;

  @override
  Widget build(BuildContext context) {
    final ThemeData theme = Theme.of(context);
    final ColorScheme scheme = theme.colorScheme;
    final AppLocalizations l10n = AppLocalizations.of(context);

    Widget icon = Icon(
      Symbols.notifications_rounded,
      size: 22,
      color: theme.textTheme.labelLarge?.color,
    );
    if (unreadCount > 0) {
      icon = Stack(
        clipBehavior: Clip.none,
        children: <Widget>[
          icon,
          Positioned(
            top: -7,
            right: -8,
            child: Container(
              constraints: const BoxConstraints(minWidth: 17),
              height: 17,
              padding: const EdgeInsets.symmetric(horizontal: 4),
              alignment: Alignment.center,
              decoration: ShapeDecoration(
                color: scheme.error,
                shape: StadiumBorder(
                  side: BorderSide(
                    color: theme.scaffoldBackgroundColor,
                    width: 2,
                  ),
                ),
              ),
              child: Text(
                unreadCount > 99 ? '99+' : '$unreadCount',
                style: TextStyle(
                  fontSize: 10,
                  height: 1,
                  fontWeight: FontWeight.w700,
                  color: scheme.onError,
                ),
              ),
            ),
          ),
        ],
      );
    }

    return Semantics(
      button: true,
      label: l10n.homeNotificationsTooltip,
      child: Material(
        key: const ValueKey<String>('home-bell'),
        color: theme.cardTheme.color ?? scheme.surface,
        shape: CircleBorder(side: BorderSide(color: scheme.outlineVariant)),
        clipBehavior: Clip.antiAlias,
        child: InkWell(
          onTap: () => context.go('/notifications'),
          child: SizedBox(width: 42, height: 42, child: Center(child: icon)),
        ),
      ),
    );
  }
}

/// Kerangka kartu ringkasan (permukaan card radius 20, mockup).
class _SummaryCard extends StatelessWidget {
  const _SummaryCard({required this.child});

  final Widget child;

  @override
  Widget build(BuildContext context) {
    final ThemeData theme = Theme.of(context);

    return Container(
      width: double.infinity,
      padding: const EdgeInsets.all(18),
      decoration: BoxDecoration(
        color: theme.cardTheme.color ?? theme.colorScheme.surface,
        borderRadius: BorderRadius.circular(InventraDimens.radiusCardMain),
        border: Border.all(color: theme.colorScheme.outlineVariant),
      ),
      child: child,
    );
  }
}

/// Judul kecil kartu ringkasan (13 w700, mockup).
class _CardTitle extends StatelessWidget {
  const _CardTitle({required this.label});

  final String label;

  @override
  Widget build(BuildContext context) {
    return Text(
      label,
      style: TextStyle(
        fontSize: 13,
        fontWeight: FontWeight.w700,
        color: Theme.of(context).colorScheme.onSurface,
      ),
    );
  }
}

/// Isi kartu saat sumbernya gagal: pesan singkat + Coba lagi. Kartu lain
/// tidak terpengaruh (non-fatal per kartu).
class _CardError extends StatelessWidget {
  const _CardError({required this.message, required this.onRetry});

  final String message;
  final VoidCallback onRetry;

  @override
  Widget build(BuildContext context) {
    final ThemeData theme = Theme.of(context);

    return Row(
      children: <Widget>[
        Icon(
          Symbols.error_rounded,
          size: 18,
          color: theme.textTheme.labelSmall?.color,
        ),
        const SizedBox(width: 8),
        Expanded(
          child: Text(
            message,
            style: TextStyle(
              fontSize: 12,
              color: theme.textTheme.bodySmall?.color,
            ),
          ),
        ),
        TextButton(
          onPressed: onRetry,
          child: Text(AppLocalizations.of(context).commonRetry),
        ),
      ],
    );
  }
}

/// Kartu "Sesi Opname Aktif" — reuse [homeActiveOpnameSessionProvider]
/// (daftar sesi + KPI detail). Loading skeleton, error + retry, dan empty
/// (tidak ada sesi berjalan) berdiri sendiri di dalam kartu.
class _OpnameCard extends ConsumerWidget {
  const _OpnameCard({required this.offline});

  final bool offline;

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final AppLocalizations l10n = AppLocalizations.of(context);
    final AsyncValue<StockOpnameSessionDto?> state = ref.watch(
      homeActiveOpnameSessionProvider,
    );

    return _SummaryCard(
      child: state.when(
        data: (StockOpnameSessionDto? session) => session == null
            ? const _OpnameEmpty()
            : _OpnameContent(session: session, offline: offline),
        loading: () => const Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: <Widget>[
            AppSkeleton(height: 13, width: 130, borderRadius: 7),
            SizedBox(height: 14),
            AppSkeleton(height: 17, width: 220, borderRadius: 8),
            SizedBox(height: 10),
            AppSkeleton(height: 11, width: 170, borderRadius: 6),
            SizedBox(height: 18),
            AppSkeleton(height: 8, borderRadius: 999),
            SizedBox(height: 18),
            AppSkeleton(height: 48, borderRadius: 13),
          ],
        ),
        error: (Object error, StackTrace stackTrace) => Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: <Widget>[
            _CardTitle(label: l10n.homeOpnameCardTitle),
            const SizedBox(height: 8),
            _CardError(
              message: l10n.homeOpnameErrorBody,
              onRetry: () => ref.invalidate(homeActiveOpnameSessionProvider),
            ),
          ],
        ),
      ),
    );
  }
}

class _OpnameEmpty extends StatelessWidget {
  const _OpnameEmpty();

  @override
  Widget build(BuildContext context) {
    final ThemeData theme = Theme.of(context);
    final AppLocalizations l10n = AppLocalizations.of(context);

    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: <Widget>[
        _CardTitle(label: l10n.homeOpnameCardTitle),
        const SizedBox(height: 8),
        Text(
          l10n.homeOpnameEmptyBody,
          style: TextStyle(
            fontSize: 12,
            color: theme.textTheme.bodySmall?.color,
          ),
        ),
        const SizedBox(height: 12),
        OutlinedButton(
          style: OutlinedButton.styleFrom(
            minimumSize: const Size.fromHeight(46),
            side: BorderSide(color: theme.colorScheme.primary, width: 1.5),
            textStyle: theme.textTheme.labelLarge?.copyWith(
              fontSize: 13.5,
              fontWeight: FontWeight.w700,
            ),
            foregroundColor: theme.colorScheme.onPrimaryContainer,
          ),
          onPressed: () => context.go('/stock-opname'),
          child: Text(l10n.homeOpnameOpenList),
        ),
      ],
    );
  }
}

class _OpnameContent extends StatelessWidget {
  const _OpnameContent({required this.session, required this.offline});

  final StockOpnameSessionDto session;
  final bool offline;

  @override
  Widget build(BuildContext context) {
    final ThemeData theme = Theme.of(context);
    final ColorScheme scheme = theme.colorScheme;
    final AppLocalizations l10n = AppLocalizations.of(context);
    final String localeName = Localizations.localeOf(context).languageCode;
    final int? total = session.total;
    final int? counted = opnameCountedOf(session);
    final String subtitle = opnameSessionSubtitle(session, localeName);

    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: <Widget>[
        Row(
          mainAxisAlignment: MainAxisAlignment.spaceBetween,
          children: <Widget>[
            Flexible(child: _CardTitle(label: l10n.homeOpnameCardTitle)),
            const SizedBox(width: 8),
            // Online-only M0: pill sinkron/offline (drift/antrean baru M5) —
            // deviasi yang sama dengan layar Daftar Sesi Opname.
            SyncPill(
              status: offline ? SyncPillStatus.offline : SyncPillStatus.synced,
            ),
          ],
        ),
        const SizedBox(height: 12),
        Text(
          opnameSessionTitle(session),
          style: TextStyle(
            fontSize: 15.5,
            fontWeight: FontWeight.w700,
            color: scheme.onSurface,
          ),
        ),
        if (subtitle.isNotEmpty) ...<Widget>[
          const SizedBox(height: 2),
          Text(
            subtitle,
            style: TextStyle(
              fontSize: 12,
              color: theme.textTheme.bodySmall?.color,
            ),
          ),
        ],
        if (total != null && counted != null) ...<Widget>[
          const SizedBox(height: 14),
          Row(
            mainAxisAlignment: MainAxisAlignment.spaceBetween,
            children: <Widget>[
              Flexible(
                child: Text(
                  l10n.homeOpnameProgress(counted, total),
                  overflow: TextOverflow.ellipsis,
                  style: TextStyle(
                    fontSize: 12,
                    color: theme.textTheme.bodySmall?.color,
                  ),
                ),
              ),
              const SizedBox(width: 8),
              Text(
                '${(total == 0 ? 0 : counted / total * 100).round()}%',
                style: TextStyle(
                  fontSize: 12,
                  fontWeight: FontWeight.w700,
                  color: scheme.onPrimaryContainer,
                ),
              ),
            ],
          ),
          const SizedBox(height: 6),
          ClipRRect(
            borderRadius: BorderRadius.circular(999),
            child: LinearProgressIndicator(
              value: total == 0 ? 0 : (counted / total).clamp(0, 1).toDouble(),
              minHeight: 8,
              backgroundColor: scheme.outlineVariant,
              color: scheme.primary,
            ),
          ),
        ],
        const SizedBox(height: 16),
        FilledButton.icon(
          style: FilledButton.styleFrom(
            minimumSize: const Size.fromHeight(48),
            textStyle: theme.textTheme.labelLarge?.copyWith(
              fontSize: 14,
              fontWeight: FontWeight.w700,
            ),
          ),
          onPressed: () => context.push('/stock-opname/${session.id}'),
          icon: const Icon(Symbols.play_arrow_rounded, size: 20),
          label: Text(l10n.homeOpnameContinue),
        ),
      ],
    );
  }
}

/// Kartu "Approval Menunggu" — reuse provider daftar inbox pending (angka
/// besar = total server, dua pengajuan terbaru, CTA Buka Inbox). Error/empty
/// berdiri sendiri; kartu lain tetap hidup.
class _ApprovalCard extends ConsumerWidget {
  const _ApprovalCard();

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final AppLocalizations l10n = AppLocalizations.of(context);
    final AsyncValue<ApprovalInboxState> state = ref.watch(
      approvalInboxProvider(ApprovalStatusFilter.pending),
    );

    return _SummaryCard(
      child: state.when(
        data: (ApprovalInboxState data) => _ApprovalContent(state: data),
        loading: () => const Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: <Widget>[
            Row(
              children: <Widget>[
                AppSkeleton(height: 38, width: 52, borderRadius: 10),
                SizedBox(width: 14),
                Expanded(
                  child: Column(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    children: <Widget>[
                      AppSkeleton(height: 13, width: 150, borderRadius: 7),
                      SizedBox(height: 8),
                      AppSkeleton(height: 10, width: 110, borderRadius: 5),
                    ],
                  ),
                ),
              ],
            ),
            SizedBox(height: 16),
            AppSkeleton(height: 12, borderRadius: 6),
            SizedBox(height: 10),
            AppSkeleton(height: 12, width: 220, borderRadius: 6),
            SizedBox(height: 16),
            AppSkeleton(height: 46, borderRadius: 13),
          ],
        ),
        error: (Object error, StackTrace stackTrace) => Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: <Widget>[
            _CardTitle(label: l10n.homeApprovalCardTitle),
            const SizedBox(height: 8),
            _CardError(
              message: l10n.homeApprovalErrorBody,
              onRetry: () => ref.invalidate(
                approvalInboxProvider(ApprovalStatusFilter.pending),
              ),
            ),
          ],
        ),
      ),
    );
  }
}

class _ApprovalContent extends ConsumerWidget {
  const _ApprovalContent({required this.state});

  final ApprovalInboxState state;

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final ThemeData theme = Theme.of(context);
    final ColorScheme scheme = theme.colorScheme;
    final AppLocalizations l10n = AppLocalizations.of(context);
    final DateTime now = ref.watch(clockProvider)();
    // "N di antaranya > 3 hari" dihitung dari halaman pertama pending yang
    // sudah termuat (kartu ini ringkasan — inbox sumber lengkapnya).
    final int staleCount = state.items
        .where(
          (RequestDto request) =>
              request.createdAt != null &&
              now.difference(request.createdAt!).inDays > 3,
        )
        .length;
    final List<RequestDto> recent = state.items.take(2).toList(growable: false);

    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: <Widget>[
        Row(
          children: <Widget>[
            Text(
              '${state.total}',
              style: TextStyle(
                fontSize: 38,
                fontWeight: FontWeight.w800,
                height: 1,
                letterSpacing: 38 * InventraDimens.titleLetterSpacingEm,
                color: scheme.onSurface,
              ),
            ),
            const SizedBox(width: 14),
            Expanded(
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: <Widget>[
                  _CardTitle(label: l10n.homeApprovalCardTitle),
                  // Subjudul: jumlah stale (> 3 hari) bila ada; pesan kosong
                  // hanya saat memang tidak ada pengajuan pending.
                  if (state.total == 0 || staleCount > 0)
                    Text(
                      state.total == 0
                          ? l10n.homeApprovalEmptyBody
                          : l10n.homeApprovalStale(staleCount),
                      style: TextStyle(
                        fontSize: 11.5,
                        color: theme.textTheme.bodySmall?.color,
                      ),
                    ),
                ],
              ),
            ),
          ],
        ),
        if (recent.isNotEmpty) ...<Widget>[
          const SizedBox(height: 12),
          for (final RequestDto request in recent)
            _ApprovalRow(request: request, now: now),
        ],
        const SizedBox(height: 8),
        OutlinedButton(
          style: OutlinedButton.styleFrom(
            minimumSize: const Size.fromHeight(46),
            side: BorderSide(color: scheme.primary, width: 1.5),
            textStyle: theme.textTheme.labelLarge?.copyWith(
              fontSize: 13.5,
              fontWeight: FontWeight.w700,
            ),
            foregroundColor: scheme.onPrimaryContainer,
          ),
          onPressed: () => context.go('/approval'),
          child: Text(l10n.homeApprovalOpenInbox),
        ),
      ],
    );
  }
}

/// Satu baris pengajuan terbaru: titik amber, "Jenis · judul", "maker · waktu".
class _ApprovalRow extends StatelessWidget {
  const _ApprovalRow({required this.request, required this.now});

  final RequestDto request;
  final DateTime now;

  @override
  Widget build(BuildContext context) {
    final ThemeData theme = Theme.of(context);
    final AppLocalizations l10n = AppLocalizations.of(context);
    final String localeName = Localizations.localeOf(context).languageCode;
    final StatusColorSet warning = statusColorSetOf(
      context,
      StatusChipVariant.warning,
    );
    final DateTime? createdAt = request.createdAt;
    final String line1 =
        '${requestTypeLabel(l10n, request.type)} · '
        '${requestTitle(l10n, request.type, request.reason)}';
    final String line2 = <String>[
      if (request.requestedByName != null) request.requestedByName!,
      if (createdAt != null)
        formatRelativeTime(l10n, now, createdAt, localeName),
    ].join(' · ');

    return InkWell(
      onTap: () => context.push('/approval/${request.id}'),
      child: Container(
        padding: const EdgeInsets.symmetric(vertical: 9),
        decoration: BoxDecoration(
          border: Border(
            top: BorderSide(color: theme.colorScheme.outlineVariant),
          ),
        ),
        child: Row(
          children: <Widget>[
            Container(
              width: 8,
              height: 8,
              decoration: BoxDecoration(
                color: warning.dot,
                shape: BoxShape.circle,
              ),
            ),
            const SizedBox(width: 10),
            Expanded(
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: <Widget>[
                  Text(
                    line1,
                    maxLines: 1,
                    overflow: TextOverflow.ellipsis,
                    style: TextStyle(
                      fontSize: 12.5,
                      fontWeight: FontWeight.w600,
                      color: theme.colorScheme.onSurface,
                    ),
                  ),
                  if (line2.isNotEmpty)
                    Text(
                      line2,
                      maxLines: 1,
                      overflow: TextOverflow.ellipsis,
                      style: TextStyle(
                        fontSize: 11,
                        color: theme.textTheme.bodySmall?.color,
                      ),
                    ),
                ],
              ),
            ),
          ],
        ),
      ),
    );
  }
}

/// Quick actions 1:1 mockup: Pindai Aset / Sesi Opname / Approval (badge
/// jumlah menunggu keputusan) / Notifikasi.
class _QuickActions extends ConsumerWidget {
  const _QuickActions();

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final AppLocalizations l10n = AppLocalizations.of(context);
    final InventraStatusColors colors = Theme.of(
      context,
    ).extension<InventraStatusColors>()!;
    final int approvalCount = ref.watch(approvalPendingBadgeProvider);

    return Column(
      children: <Widget>[
        Row(
          children: <Widget>[
            Expanded(
              child: _QuickAction(
                label: l10n.homeQuickScan,
                icon: Symbols.qr_code_scanner_rounded,
                colorSet: colors.success,
                onTap: () => context.go('/scan'),
              ),
            ),
            Expanded(
              child: _QuickAction(
                label: l10n.homeQuickOpname,
                icon: Symbols.fact_check_rounded,
                colorSet: colors.info,
                onTap: () => context.go('/stock-opname'),
              ),
            ),
            Expanded(
              child: _QuickAction(
                label: l10n.homeQuickApproval,
                icon: Symbols.approval_rounded,
                colorSet: colors.warning,
                badgeCount: approvalCount,
                onTap: () => context.go('/approval'),
              ),
            ),
            Expanded(
              child: _QuickAction(
                label: l10n.homeQuickNotifications,
                icon: Symbols.notifications_rounded,
                colorSet: colors.neutral,
                onTap: () => context.go('/notifications'),
              ),
            ),
          ],
        ),
        const SizedBox(height: 14),
        Row(
          children: <Widget>[
            Expanded(
              child: _QuickAction(
                label: l10n.homeQuickCatalog,
                icon: Symbols.inventory_2_rounded,
                colorSet: colors.info,
                onTap: () => context.push('/catalog'),
              ),
            ),
            Expanded(
              child: _QuickAction(
                label: l10n.homeQuickMyAssets,
                icon: Symbols.badge_rounded,
                colorSet: colors.success,
                onTap: () => context.push('/my-assets'),
              ),
            ),
            Expanded(
              child: _QuickAction(
                label: l10n.homeQuickMyRequests,
                icon: Symbols.description_rounded,
                colorSet: colors.neutral,
                onTap: () => context.push('/my-requests'),
              ),
            ),
            Expanded(
              child: _QuickAction(
                label: l10n.homeQuickRegister,
                icon: Symbols.add_box_rounded,
                colorSet: colors.warning,
                onTap: () => context.push('/register-asset'),
              ),
            ),
          ],
        ),
      ],
    );
  }
}

class _QuickAction extends StatelessWidget {
  const _QuickAction({
    required this.label,
    required this.icon,
    required this.colorSet,
    required this.onTap,
    this.badgeCount = 0,
  });

  final String label;
  final IconData icon;
  final StatusColorSet colorSet;
  final VoidCallback onTap;
  final int badgeCount;

  @override
  Widget build(BuildContext context) {
    final ThemeData theme = Theme.of(context);
    final ColorScheme scheme = theme.colorScheme;

    Widget tile = Container(
      width: 54,
      height: 54,
      decoration: BoxDecoration(
        color: colorSet.bg,
        borderRadius: BorderRadius.circular(17),
      ),
      child: Icon(icon, size: 26, color: colorSet.text),
    );
    if (badgeCount > 0) {
      tile = Stack(
        clipBehavior: Clip.none,
        children: <Widget>[
          tile,
          Positioned(
            top: -4,
            right: -4,
            child: Container(
              constraints: const BoxConstraints(minWidth: 18),
              height: 18,
              padding: const EdgeInsets.symmetric(horizontal: 4),
              alignment: Alignment.center,
              decoration: ShapeDecoration(
                color: scheme.error,
                shape: StadiumBorder(
                  side: BorderSide(
                    color: theme.scaffoldBackgroundColor,
                    width: 2,
                  ),
                ),
              ),
              child: Text(
                badgeCount > 99 ? '99+' : '$badgeCount',
                style: TextStyle(
                  fontSize: 10,
                  height: 1,
                  fontWeight: FontWeight.w700,
                  color: scheme.onError,
                ),
              ),
            ),
          ),
        ],
      );
    }

    return Semantics(
      button: true,
      label: label,
      child: InkResponse(
        onTap: onTap,
        radius: 44,
        child: Column(
          mainAxisSize: MainAxisSize.min,
          children: <Widget>[
            tile,
            const SizedBox(height: 6),
            Text(
              label,
              maxLines: 1,
              overflow: TextOverflow.ellipsis,
              style: TextStyle(
                fontSize: 11,
                fontWeight: FontWeight.w600,
                color: theme.textTheme.labelLarge?.color,
              ),
            ),
          ],
        ),
      ),
    );
  }
}
