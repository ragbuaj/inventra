import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';
import 'package:material_symbols_icons/symbols.dart';

import '../core/i18n/gen/app_localizations.dart';
import '../features/notifications/presentation/unread_count_provider.dart';

/// Shell bottom-nav 5 slot 1:1 mockup Beranda: Beranda / Opname / Pindai
/// (tombol tengah menjorok) / Approval / Notif (badge unread).
class AppShell extends ConsumerWidget {
  const AppShell({required this.navigationShell, super.key});

  final StatefulNavigationShell navigationShell;

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final ThemeData theme = Theme.of(context);
    final ColorScheme scheme = theme.colorScheme;
    final AppLocalizations l10n = AppLocalizations.of(context);
    // Bar memakai warna permukaan card (putih light / slate-800 dark) sesuai
    // mockup; ini juga warna border cutout tombol Pindai.
    final Color barColor = theme.cardTheme.color ?? scheme.surface;
    final int unreadCount = ref.watch(unreadNotificationCountProvider);
    // Branch scan (index 2) tampil full screen tanpa bar/FAB sesuai mockup
    // "Inventra Mobile - Scan"; keluar lewat tombol tutup di layarnya.
    final bool fullScreenScan = navigationShell.currentIndex == 2;

    return Scaffold(
      body: navigationShell,
      // FAB docked di tengah bar supaya tombol yang menjorok ke atas tetap
      // bisa di-tap penuh (widget yang hanya digeser secara visual kehilangan
      // hit-test di luar bounds slot-nya).
      floatingActionButtonLocation: FloatingActionButtonLocation.centerDocked,
      floatingActionButton: fullScreenScan
          ? null
          : _ScanFab(
              label: l10n.shellTabScan,
              barColor: barColor,
              onPressed: () => navigationShell.goBranch(
                2,
                initialLocation: navigationShell.currentIndex == 2,
              ),
            ),
      bottomNavigationBar: fullScreenScan
          ? null
          : Material(
              color: barColor,
              child: SafeArea(
                top: false,
                child: Container(
                  decoration: BoxDecoration(
                    border: Border(
                      top: BorderSide(color: scheme.outlineVariant),
                    ),
                  ),
                  padding: const EdgeInsets.fromLTRB(8, 10, 8, 6),
                  child: Row(
                    crossAxisAlignment: CrossAxisAlignment.end,
                    children: <Widget>[
                      _NavTab(
                        shell: navigationShell,
                        index: 0,
                        icon: Symbols.home_rounded,
                        label: l10n.shellTabHome,
                      ),
                      _NavTab(
                        shell: navigationShell,
                        index: 1,
                        icon: Symbols.fact_check_rounded,
                        label: l10n.shellTabOpname,
                      ),
                      _ScanSlot(
                        shell: navigationShell,
                        index: 2,
                        label: l10n.shellTabScan,
                      ),
                      _NavTab(
                        shell: navigationShell,
                        index: 3,
                        icon: Symbols.approval_rounded,
                        label: l10n.shellTabApproval,
                      ),
                      _NavTab(
                        shell: navigationShell,
                        index: 4,
                        icon: Symbols.notifications_rounded,
                        label: l10n.shellTabNotifications,
                        badgeCount: unreadCount,
                        badgeBorderColor: barColor,
                      ),
                    ],
                  ),
                ),
              ),
            ),
    );
  }
}

/// Slot tab biasa: ikon (pill primary-container saat aktif) + label.
class _NavTab extends StatelessWidget {
  const _NavTab({
    required this.shell,
    required this.index,
    required this.icon,
    required this.label,
    this.badgeCount = 0,
    this.badgeBorderColor,
  });

  final StatefulNavigationShell shell;
  final int index;
  final IconData icon;
  final String label;
  final int badgeCount;
  final Color? badgeBorderColor;

  @override
  Widget build(BuildContext context) {
    final ColorScheme scheme = Theme.of(context).colorScheme;
    final bool active = shell.currentIndex == index;
    final Color contentColor = active
        ? scheme.onPrimaryContainer
        : scheme.onSurfaceVariant;

    Widget iconWidget = Icon(
      icon,
      size: 22,
      weight: active ? 700 : 500,
      color: contentColor,
    );
    if (badgeCount > 0) {
      iconWidget = Stack(
        clipBehavior: Clip.none,
        children: <Widget>[
          iconWidget,
          Positioned(
            top: -4,
            right: -8,
            child: _UnreadBadge(
              count: badgeCount,
              borderColor: badgeBorderColor ?? scheme.surface,
            ),
          ),
        ],
      );
    }

    return Expanded(
      child: Semantics(
        button: true,
        selected: active,
        child: InkResponse(
          onTap: () => shell.goBranch(
            index,
            // Tap ulang tab aktif kembali ke akar branch (konvensi platform).
            initialLocation: index == shell.currentIndex,
          ),
          radius: 42,
          child: Column(
            mainAxisSize: MainAxisSize.min,
            children: <Widget>[
              Container(
                width: 54,
                height: 29,
                alignment: Alignment.center,
                decoration: active
                    ? ShapeDecoration(
                        color: scheme.primaryContainer,
                        shape: const StadiumBorder(),
                      )
                    : null,
                child: iconWidget,
              ),
              const SizedBox(height: 3),
              Text(
                label,
                style: TextStyle(
                  fontSize: 10.5,
                  fontWeight: active ? FontWeight.w700 : FontWeight.w500,
                  color: contentColor,
                ),
              ),
            ],
          ),
        ),
      ),
    );
  }
}

/// Slot tengah bar: area kosong (tempat FAB menumpang) + label "Pindai".
/// Label ikut bisa di-tap sebagai target alternatif tombol scan.
class _ScanSlot extends StatelessWidget {
  const _ScanSlot({
    required this.shell,
    required this.index,
    required this.label,
  });

  final StatefulNavigationShell shell;
  final int index;
  final String label;

  @override
  Widget build(BuildContext context) {
    final ColorScheme scheme = Theme.of(context).colorScheme;

    return Expanded(
      child: Semantics(
        button: true,
        selected: shell.currentIndex == index,
        child: InkResponse(
          onTap: () => shell.goBranch(
            index,
            initialLocation: index == shell.currentIndex,
          ),
          radius: 42,
          child: Column(
            mainAxisSize: MainAxisSize.min,
            children: <Widget>[
              const SizedBox(width: 54, height: 29),
              const SizedBox(height: 3),
              Text(
                label,
                style: TextStyle(
                  fontSize: 10.5,
                  fontWeight: FontWeight.w700,
                  color: scheme.primary,
                ),
              ),
            ],
          ),
        ),
      ),
    );
  }
}

/// Tombol Pindai tengah 1:1 mockup: kotak 56 radius 19 primary, ikon
/// qr_code_scanner putih, border 4 warna bar (efek cutout), shadow hijau
/// lembut; menjorok ke atas melewati bar via FAB center-docked.
class _ScanFab extends StatelessWidget {
  const _ScanFab({
    required this.label,
    required this.barColor,
    required this.onPressed,
  });

  final String label;
  final Color barColor;
  final VoidCallback onPressed;

  @override
  Widget build(BuildContext context) {
    final ColorScheme scheme = Theme.of(context).colorScheme;

    return Semantics(
      button: true,
      label: label,
      child: Container(
        width: 56,
        height: 56,
        decoration: BoxDecoration(
          borderRadius: BorderRadius.circular(19),
          boxShadow: <BoxShadow>[
            BoxShadow(
              color: scheme.primary.withValues(alpha: 0.45),
              blurRadius: 18,
              offset: const Offset(0, 8),
            ),
          ],
        ),
        child: Material(
          color: scheme.primary,
          clipBehavior: Clip.antiAlias,
          shape: RoundedRectangleBorder(
            borderRadius: BorderRadius.circular(19),
            side: BorderSide(color: barColor, width: 4),
          ),
          child: InkWell(
            onTap: onPressed,
            child: Center(
              child: Icon(
                Symbols.qr_code_scanner_rounded,
                size: 29,
                color: scheme.onPrimary,
              ),
            ),
          ),
        ),
      ),
    );
  }
}

/// Badge angka unread merah di atas ikon tab.
class _UnreadBadge extends StatelessWidget {
  const _UnreadBadge({required this.count, required this.borderColor});

  final int count;
  final Color borderColor;

  @override
  Widget build(BuildContext context) {
    final ColorScheme scheme = Theme.of(context).colorScheme;

    return Container(
      constraints: const BoxConstraints(minWidth: 15),
      height: 15,
      padding: const EdgeInsets.symmetric(horizontal: 3),
      alignment: Alignment.center,
      decoration: ShapeDecoration(
        color: scheme.error,
        shape: StadiumBorder(side: BorderSide(color: borderColor, width: 2)),
      ),
      child: Text(
        count > 99 ? '99+' : '$count',
        style: TextStyle(
          fontSize: 9,
          height: 1,
          fontWeight: FontWeight.w700,
          color: scheme.onError,
        ),
      ),
    );
  }
}
