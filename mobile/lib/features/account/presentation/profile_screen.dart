import 'dart:typed_data';

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';
import 'package:material_symbols_icons/symbols.dart';

import '../../../app/theme.dart';
import '../../../core/auth/auth_controller.dart';
import '../../../core/auth/auth_session.dart';
import '../../../core/auth/data/user_dto.dart';
import '../../../core/i18n/gen/app_localizations.dart';
import '../../../core/utils/clock.dart';
import '../../../core/widgets/app_skeleton.dart';
import '../../../core/widgets/confirm_dialog.dart';
import '../data/session_dto.dart';
import 'account_providers.dart';
import 'session_presentation.dart';

/// Layar Profil 1:1 mockup "Inventra Mobile - Profil": kartu identitas
/// (avatar foto bila tersedia — fallback inisial, nama, email, kantor via
/// lookup non-fatal, catatan penyuntingan di web), kartu Sesi Perangkat
/// (daftar sesi aktif, sesi ini ditandai dan tidak bisa dicabut dari daftar,
/// Cabut per sesi lain via ConfirmDialog), tombol keluar dari semua perangkat
/// lain, dan tombol Keluar (logout sesi ini) — ditambah state loading/error/
/// empty untuk daftar sesi.
class ProfileScreen extends ConsumerStatefulWidget {
  const ProfileScreen({super.key});

  @override
  ConsumerState<ProfileScreen> createState() => _ProfileScreenState();
}

class _ProfileScreenState extends ConsumerState<ProfileScreen> {
  Future<void> _revokeSession(SessionDto session) async {
    final AppLocalizations l10n = AppLocalizations.of(context);
    final String name = sessionTitle(session);
    final bool confirmed = await ConfirmDialog.show(
      context,
      title: l10n.accountSessionRevokeConfirmTitle,
      message: l10n.accountSessionRevokeConfirmBody(name),
      confirmLabel: l10n.accountSessionRevokeConfirmAction,
      icon: Symbols.phonelink_erase_rounded,
      destructive: true,
    );
    if (!confirmed) {
      return;
    }
    final bool ok = await ref
        .read(accountSessionsProvider.notifier)
        .revoke(session.id);
    if (!mounted) {
      return;
    }
    final AppLocalizations snackL10n = AppLocalizations.of(context);
    ScaffoldMessenger.of(context).showSnackBar(
      SnackBar(
        content: Text(
          ok
              ? snackL10n.accountSessionRevokedSnack(name)
              : snackL10n.accountSessionRevokeFailed,
        ),
      ),
    );
  }

  Future<void> _revokeOthers(int otherCount) async {
    final AppLocalizations l10n = AppLocalizations.of(context);
    final bool confirmed = await ConfirmDialog.show(
      context,
      title: l10n.accountRevokeOthersConfirmTitle,
      message: l10n.accountRevokeOthersConfirmBody(otherCount),
      confirmLabel: l10n.accountRevokeOthersConfirmAction,
      icon: Symbols.devices_off_rounded,
      destructive: true,
    );
    if (!confirmed) {
      return;
    }
    final bool ok = await ref
        .read(accountSessionsProvider.notifier)
        .revokeOthers();
    if (!mounted || ok) {
      return;
    }
    ScaffoldMessenger.of(context).showSnackBar(
      SnackBar(
        content: Text(AppLocalizations.of(context).accountRevokeOthersFailed),
      ),
    );
  }

  Future<void> _logout() async {
    final AppLocalizations l10n = AppLocalizations.of(context);
    final bool confirmed = await ConfirmDialog.show(
      context,
      title: l10n.accountLogoutConfirmTitle,
      message: l10n.accountLogoutConfirmBody,
      confirmLabel: l10n.accountLogoutConfirmAction,
      icon: Symbols.logout_rounded,
      destructive: true,
    );
    if (confirmed) {
      // Guard router memindahkan ke /login begitu sesi berakhir.
      await ref.read(authControllerProvider.notifier).logout();
    }
  }

  @override
  Widget build(BuildContext context) {
    final AppLocalizations l10n = AppLocalizations.of(context);
    final ThemeData theme = Theme.of(context);
    final AsyncValue<List<SessionDto>> sessions = ref.watch(
      accountSessionsProvider,
    );

    return Scaffold(
      appBar: AppBar(
        title: Text(l10n.accountTitle),
        actions: <Widget>[
          // Entri Pengaturan di app bar (mockup): ikon + label menuju
          // /settings.
          Padding(
            padding: const EdgeInsets.only(right: 8),
            child: TextButton.icon(
              key: const ValueKey<String>('profile-settings'),
              style: TextButton.styleFrom(
                foregroundColor: theme.textTheme.bodySmall?.color,
                textStyle: theme.textTheme.labelLarge?.copyWith(
                  fontSize: 12.5,
                  fontWeight: FontWeight.w600,
                ),
              ),
              onPressed: () => context.push('/settings'),
              icon: const Icon(Symbols.settings_rounded, size: 19),
              label: Text(l10n.accountSettingsButton),
            ),
          ),
        ],
      ),
      body: SafeArea(
        child: Column(
          children: <Widget>[
            Expanded(
              child: ListView(
                padding: const EdgeInsets.fromLTRB(20, 4, 20, 8),
                children: <Widget>[
                  const _IdentityCard(),
                  const SizedBox(height: 14),
                  ...sessions.when(
                    data: (List<SessionDto> data) => _sessionsContent(data),
                    loading: () => const <Widget>[_SessionsSkeleton()],
                    error: (Object error, StackTrace stackTrace) => <Widget>[
                      _SessionsError(
                        onRetry: () => ref.invalidate(accountSessionsProvider),
                      ),
                    ],
                  ),
                ],
              ),
            ),
            Padding(
              padding: const EdgeInsets.fromLTRB(20, 12, 20, 8),
              child: FilledButton.icon(
                key: const ValueKey<String>('profile-logout'),
                style: FilledButton.styleFrom(
                  minimumSize: const Size.fromHeight(
                    InventraDimens.buttonHeightPrimary,
                  ),
                  backgroundColor: theme.colorScheme.error,
                  foregroundColor: theme.colorScheme.onError,
                  textStyle: theme.textTheme.labelLarge?.copyWith(
                    fontSize: 14.5,
                    fontWeight: FontWeight.w700,
                  ),
                ),
                onPressed: _logout,
                icon: const Icon(Symbols.logout_rounded, size: 20),
                label: Text(l10n.accountLogout),
              ),
            ),
          ],
        ),
      ),
    );
  }

  List<Widget> _sessionsContent(List<SessionDto> data) {
    final int otherCount = data
        .where((SessionDto session) => !session.current)
        .length;

    return <Widget>[
      _SessionsCard(sessions: data, onRevoke: _revokeSession),
      if (otherCount > 0) ...<Widget>[
        const SizedBox(height: 14),
        _RevokeOthersButton(onPressed: () => _revokeOthers(otherCount)),
      ],
    ];
  }
}

/// Inisial nama untuk avatar tanpa foto: huruf depan dua kata pertama (pola
/// yang sama dengan header Beranda).
@visibleForTesting
String profileInitials(String name) {
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

/// Kartu identitas 1:1 mockup: avatar 84 (foto bila endpoint avatar
/// mengembalikan bytes, selain itu inisial), nama, email, kantor (lookup
/// non-fatal), dan catatan penyuntingan dari web.
///
/// Deviasi tercatat: badge nama peran mockup ("Asset Manager") tidak dirender
/// — endpoint roles berada di grup authzadmin yang menolak audience mobile
/// (alasan yang sama dengan header Beranda).
class _IdentityCard extends ConsumerWidget {
  const _IdentityCard();

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final ThemeData theme = Theme.of(context);
    final ColorScheme scheme = theme.colorScheme;
    final AppLocalizations l10n = AppLocalizations.of(context);
    final AuthSession? session = ref.watch(authControllerProvider).value;
    final UserDto? user = session is Authenticated ? session.user : null;
    final Uint8List? avatarBytes = ref.watch(accountAvatarProvider).value;
    final String? officeName = ref.watch(accountOfficeNameProvider).value;

    return Padding(
      padding: const EdgeInsets.fromLTRB(0, 10, 0, 2),
      child: Column(
        children: <Widget>[
          Container(
            width: 84,
            height: 84,
            clipBehavior: Clip.antiAlias,
            decoration: ShapeDecoration(
              color: scheme.primaryContainer,
              shape: CircleBorder(
                side: BorderSide(
                  color: scheme.primaryContainer,
                  width: 2,
                  strokeAlign: BorderSide.strokeAlignOutside,
                ),
              ),
            ),
            child: avatarBytes == null
                ? Center(
                    child: Text(
                      profileInitials(user?.name ?? ''),
                      style: TextStyle(
                        fontSize: 28,
                        fontWeight: FontWeight.w800,
                        color: scheme.onPrimaryContainer,
                      ),
                    ),
                  )
                : Image.memory(
                    avatarBytes,
                    key: const ValueKey<String>('profile-avatar-photo'),
                    fit: BoxFit.cover,
                    gaplessPlayback: true,
                  ),
          ),
          const SizedBox(height: 12),
          Text(
            user?.name ?? '',
            textAlign: TextAlign.center,
            style: TextStyle(
              fontSize: 19,
              fontWeight: FontWeight.w800,
              letterSpacing: 19 * InventraDimens.titleLetterSpacingEm,
              color: scheme.onSurface,
            ),
          ),
          const SizedBox(height: 6),
          Text(
            user?.email ?? '',
            textAlign: TextAlign.center,
            style: TextStyle(
              fontSize: 12.5,
              color: theme.textTheme.bodySmall?.color,
            ),
          ),
          if (officeName != null) ...<Widget>[
            const SizedBox(height: 3),
            Row(
              mainAxisSize: MainAxisSize.min,
              children: <Widget>[
                Icon(
                  Symbols.account_balance_rounded,
                  size: 15,
                  color: theme.textTheme.bodySmall?.color,
                ),
                const SizedBox(width: 5),
                Text(
                  officeName,
                  style: TextStyle(
                    fontSize: 12.5,
                    color: theme.textTheme.bodySmall?.color,
                  ),
                ),
              ],
            ),
          ],
          const SizedBox(height: 8),
          Text(
            l10n.accountEditOnWeb,
            textAlign: TextAlign.center,
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

/// Kerangka kartu Sesi Perangkat (permukaan card radius 18, mockup).
class _SessionsCardShell extends StatelessWidget {
  const _SessionsCardShell({required this.child});

  final Widget child;

  @override
  Widget build(BuildContext context) {
    final ThemeData theme = Theme.of(context);

    return Container(
      width: double.infinity,
      padding: const EdgeInsets.fromLTRB(16, 15, 16, 15),
      decoration: BoxDecoration(
        color: theme.cardTheme.color ?? theme.colorScheme.surface,
        borderRadius: BorderRadius.circular(18),
        border: Border.all(color: theme.colorScheme.outlineVariant),
      ),
      child: child,
    );
  }
}

/// Judul kecil uppercase kartu Sesi Perangkat (mockup).
class _SessionsCardTitle extends StatelessWidget {
  const _SessionsCardTitle();

  @override
  Widget build(BuildContext context) {
    return Text(
      AppLocalizations.of(context).accountSessionsTitle.toUpperCase(),
      style: TextStyle(
        fontSize: 12,
        fontWeight: FontWeight.w700,
        letterSpacing: 0.6,
        color: Theme.of(context).textTheme.bodySmall?.color,
      ),
    );
  }
}

/// Kartu daftar sesi: sesi ini ditandai badge tanpa aksi (mencabutnya lewat
/// tombol Keluar); sesi lain memiliki tombol Cabut. Daftar kosong (kondisi
/// yang mestinya tidak terjadi — sesi ini selalu aktif) tetap dirender sopan.
class _SessionsCard extends StatelessWidget {
  const _SessionsCard({required this.sessions, required this.onRevoke});

  final List<SessionDto> sessions;
  final ValueChanged<SessionDto> onRevoke;

  @override
  Widget build(BuildContext context) {
    final ThemeData theme = Theme.of(context);
    final AppLocalizations l10n = AppLocalizations.of(context);

    return _SessionsCardShell(
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: <Widget>[
          const _SessionsCardTitle(),
          const SizedBox(height: 12),
          if (sessions.isEmpty)
            Text(
              l10n.accountSessionsEmpty,
              style: TextStyle(
                fontSize: 12,
                color: theme.textTheme.bodySmall?.color,
              ),
            )
          else
            for (int i = 0; i < sessions.length; i++) ...<Widget>[
              if (i > 0) const Divider(height: 1),
              Padding(
                padding: EdgeInsets.only(
                  top: i == 0 ? 0 : 12,
                  bottom: i == sessions.length - 1 ? 0 : 12,
                ),
                child: _SessionRow(
                  session: sessions[i],
                  onRevoke: () => onRevoke(sessions[i]),
                ),
              ),
            ],
        ],
      ),
    );
  }
}

/// Satu baris sesi 1:1 mockup: tile ikon perangkat (hijau untuk sesi ini),
/// judul + badge "Perangkat ini", subjudul lokasi · IP · waktu, tombol Cabut
/// untuk sesi lain.
class _SessionRow extends ConsumerWidget {
  const _SessionRow({required this.session, required this.onRevoke});

  final SessionDto session;
  final VoidCallback onRevoke;

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final ThemeData theme = Theme.of(context);
    final ColorScheme scheme = theme.colorScheme;
    final AppLocalizations l10n = AppLocalizations.of(context);
    final String localeName = Localizations.localeOf(context).languageCode;
    final InventraStatusColors colors = theme
        .extension<InventraStatusColors>()!;
    final DateTime now = ref.watch(clockProvider)();
    final StatusColorSet tile = session.current
        ? colors.success
        : colors.neutral;

    return Row(
      key: ValueKey<String>('account-session-${session.id}'),
      children: <Widget>[
        Container(
          width: 40,
          height: 40,
          decoration: BoxDecoration(
            color: tile.bg,
            borderRadius: BorderRadius.circular(11),
          ),
          child: Icon(
            sessionDeviceIcon(session.deviceType),
            size: 20,
            color: tile.text,
          ),
        ),
        const SizedBox(width: 12),
        Expanded(
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: <Widget>[
              Row(
                children: <Widget>[
                  Flexible(
                    child: Text(
                      sessionTitle(session),
                      maxLines: 1,
                      overflow: TextOverflow.ellipsis,
                      style: TextStyle(
                        fontSize: 13,
                        fontWeight: session.current
                            ? FontWeight.w700
                            : FontWeight.w600,
                        color: scheme.onSurface,
                      ),
                    ),
                  ),
                  if (session.current) ...<Widget>[
                    const SizedBox(width: 7),
                    Container(
                      padding: const EdgeInsets.symmetric(
                        horizontal: 8,
                        vertical: 2,
                      ),
                      decoration: ShapeDecoration(
                        color: colors.success.bg,
                        shape: const StadiumBorder(),
                      ),
                      child: Text(
                        l10n.accountSessionCurrentBadge,
                        style: TextStyle(
                          fontSize: 10,
                          fontWeight: FontWeight.w700,
                          color: colors.success.text,
                        ),
                      ),
                    ),
                  ],
                ],
              ),
              const SizedBox(height: 2),
              Text(
                sessionSubtitle(l10n, session, now, localeName),
                maxLines: 1,
                overflow: TextOverflow.ellipsis,
                style: TextStyle(
                  fontSize: 11,
                  color: theme.textTheme.labelSmall?.color,
                ),
              ),
            ],
          ),
        ),
        if (!session.current) ...<Widget>[
          const SizedBox(width: 8),
          OutlinedButton(
            key: ValueKey<String>('account-session-revoke-${session.id}'),
            style: OutlinedButton.styleFrom(
              minimumSize: const Size(0, 34),
              padding: const EdgeInsets.symmetric(horizontal: 14),
              side: BorderSide(
                color: scheme.error.withValues(alpha: 0.45),
                width: 1.5,
              ),
              foregroundColor: scheme.error,
              shape: RoundedRectangleBorder(
                borderRadius: BorderRadius.circular(9),
              ),
              textStyle: theme.textTheme.labelLarge?.copyWith(
                fontSize: 12,
                fontWeight: FontWeight.w700,
              ),
            ),
            onPressed: onRevoke,
            child: Text(l10n.accountSessionRevoke),
          ),
        ],
      ],
    );
  }
}

/// Tombol "Keluar dari semua perangkat lain" (mockup) — tampil hanya saat ada
/// sesi lain yang bisa dicabut.
class _RevokeOthersButton extends StatelessWidget {
  const _RevokeOthersButton({required this.onPressed});

  final VoidCallback onPressed;

  @override
  Widget build(BuildContext context) {
    final ThemeData theme = Theme.of(context);

    return OutlinedButton.icon(
      key: const ValueKey<String>('profile-revoke-others'),
      style: OutlinedButton.styleFrom(
        minimumSize: const Size.fromHeight(InventraDimens.buttonHeightStandard),
        side: BorderSide(color: theme.colorScheme.outline, width: 1.5),
        foregroundColor: theme.textTheme.labelLarge?.color,
        textStyle: theme.textTheme.labelLarge?.copyWith(
          fontSize: 13.5,
          fontWeight: FontWeight.w700,
        ),
      ),
      onPressed: onPressed,
      icon: const Icon(Symbols.devices_off_rounded, size: 19),
      label: Text(AppLocalizations.of(context).accountRevokeOthers),
    );
  }
}

/// Skeleton kartu Sesi Perangkat + tombol keluar-semua (mockup loading).
class _SessionsSkeleton extends StatelessWidget {
  const _SessionsSkeleton();

  @override
  Widget build(BuildContext context) {
    Widget row({required bool withButton}) => Row(
      children: <Widget>[
        const AppSkeleton(height: 40, width: 40, borderRadius: 11),
        const SizedBox(width: 12),
        const Expanded(
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: <Widget>[
              AppSkeleton(height: 12, width: 150, borderRadius: 6),
              SizedBox(height: 7),
              AppSkeleton(height: 10, width: 190, borderRadius: 5),
            ],
          ),
        ),
        if (withButton) ...<Widget>[
          const SizedBox(width: 8),
          const AppSkeleton(height: 34, width: 64, borderRadius: 9),
        ],
      ],
    );

    return Column(
      children: <Widget>[
        _SessionsCardShell(
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: <Widget>[
              const AppSkeleton(height: 11, width: 110, borderRadius: 6),
              const SizedBox(height: 13),
              row(withButton: false),
              const SizedBox(height: 13),
              row(withButton: true),
              const SizedBox(height: 13),
              row(withButton: true),
            ],
          ),
        ),
        const SizedBox(height: 14),
        const AppSkeleton(height: 48, borderRadius: 14),
      ],
    );
  }
}

/// Cabang error daftar sesi: pesan + Coba lagi di dalam kartu — identitas dan
/// tombol Keluar tetap berfungsi.
class _SessionsError extends StatelessWidget {
  const _SessionsError({required this.onRetry});

  final VoidCallback onRetry;

  @override
  Widget build(BuildContext context) {
    final ThemeData theme = Theme.of(context);
    final AppLocalizations l10n = AppLocalizations.of(context);

    return _SessionsCardShell(
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: <Widget>[
          const _SessionsCardTitle(),
          const SizedBox(height: 8),
          Row(
            children: <Widget>[
              Icon(
                Symbols.error_rounded,
                size: 18,
                color: theme.textTheme.labelSmall?.color,
              ),
              const SizedBox(width: 8),
              Expanded(
                child: Text(
                  l10n.accountSessionsErrorBody,
                  style: TextStyle(
                    fontSize: 12,
                    color: theme.textTheme.bodySmall?.color,
                  ),
                ),
              ),
              TextButton(onPressed: onRetry, child: Text(l10n.commonRetry)),
            ],
          ),
        ],
      ),
    );
  }
}
