import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';
import 'package:material_symbols_icons/symbols.dart';

import '../core/auth/auth_controller.dart';
import '../core/auth/auth_session.dart';
import '../core/i18n/gen/app_localizations.dart';
import '../core/widgets/confirm_dialog.dart';
import '../core/widgets/empty_state.dart';
import '../features/approval/presentation/approval_detail_screen.dart';
import '../features/approval/presentation/approval_inbox_screen.dart';
import '../features/asset_detail/presentation/asset_detail_screen.dart';
import '../features/home/presentation/home_screen.dart';
import '../features/login/presentation/login_screen.dart';
import '../features/notifications/presentation/notifications_screen.dart';
import '../features/scan/presentation/scan_screen.dart';
import '../features/stock_opname/presentation/opname_counting_screen.dart';
import '../features/stock_opname/presentation/opname_session_list_screen.dart';
import '../features/stock_opname/presentation/opname_variance_screen.dart';
import 'shell.dart';

/// Router aplikasi: guard auth + seluruh tabel rute v1 (plan M0 Task 7).
///
/// Rute yang layarnya belum dibangun diisi [_ComingSoonScreen]; Task 8-12
/// mengganti placeholder masing-masing. Layar sekunder (detail aset/approval/
/// opname, profil, pengaturan) didaftarkan pada navigator root sehingga tampil
/// DI ATAS shell tanpa bottom nav.
final Provider<GoRouter> appRouterProvider = Provider<GoRouter>((Ref ref) {
  final GlobalKey<NavigatorState> rootNavigatorKey = GlobalKey<NavigatorState>(
    debugLabel: 'root',
  );
  final _RouterRefreshNotifier refreshNotifier = _RouterRefreshNotifier();
  // Setiap perubahan sesi (login/logout/refresh gagal) memicu evaluasi ulang
  // redirect via refreshListenable — pola resmi go_router untuk guard auth.
  ref.listen<AsyncValue<AuthSession>>(
    authControllerProvider,
    (AsyncValue<AuthSession>? previous, AsyncValue<AuthSession> next) =>
        refreshNotifier.notify(),
  );
  ref.onDispose(refreshNotifier.dispose);

  final GoRouter router = GoRouter(
    navigatorKey: rootNavigatorKey,
    initialLocation: '/',
    refreshListenable: refreshNotifier,
    redirect: (BuildContext context, GoRouterState state) {
      // Cold start (AsyncLoading) diperlakukan belum login: pengguna melihat
      // layar login sampai refresh sesi selesai, lalu guard memindahkannya.
      final bool loggedIn =
          ref.read(authControllerProvider).value is Authenticated;
      final bool onLogin = state.matchedLocation == '/login';
      if (!loggedIn) {
        return onLogin ? null : '/login';
      }
      return onLogin ? '/' : null;
    },
    routes: <RouteBase>[
      GoRoute(
        path: '/login',
        name: 'login',
        builder: (BuildContext context, GoRouterState state) =>
            const LoginScreen(),
      ),
      StatefulShellRoute.indexedStack(
        builder:
            (
              BuildContext context,
              GoRouterState state,
              StatefulNavigationShell navigationShell,
            ) => AppShell(navigationShell: navigationShell),
        branches: <StatefulShellBranch>[
          StatefulShellBranch(
            routes: <RouteBase>[
              GoRoute(
                path: '/',
                name: 'home',
                builder: (BuildContext context, GoRouterState state) =>
                    const HomeScreen(),
              ),
            ],
          ),
          StatefulShellBranch(
            routes: <RouteBase>[
              GoRoute(
                path: '/stock-opname',
                name: 'stock-opname',
                builder: (BuildContext context, GoRouterState state) =>
                    const OpnameSessionListScreen(),
                routes: <RouteBase>[
                  GoRoute(
                    path: ':id',
                    name: 'stock-opname-detail',
                    parentNavigatorKey: rootNavigatorKey,
                    builder: (BuildContext context, GoRouterState state) =>
                        OpnameCountingScreen(
                          sessionId: state.pathParameters['id']!,
                        ),
                    routes: <RouteBase>[
                      GoRoute(
                        path: 'variance',
                        name: 'stock-opname-variance',
                        parentNavigatorKey: rootNavigatorKey,
                        builder: (BuildContext context, GoRouterState state) =>
                            OpnameVarianceScreen(
                              sessionId: state.pathParameters['id']!,
                            ),
                      ),
                    ],
                  ),
                ],
              ),
            ],
          ),
          StatefulShellBranch(
            routes: <RouteBase>[
              GoRoute(
                path: '/scan',
                name: 'scan',
                builder: (BuildContext context, GoRouterState state) =>
                    const ScanScreen(),
              ),
            ],
          ),
          StatefulShellBranch(
            routes: <RouteBase>[
              GoRoute(
                path: '/approval',
                name: 'approval',
                builder: (BuildContext context, GoRouterState state) =>
                    const ApprovalInboxScreen(),
                routes: <RouteBase>[
                  GoRoute(
                    path: ':id',
                    name: 'approval-detail',
                    parentNavigatorKey: rootNavigatorKey,
                    builder: (BuildContext context, GoRouterState state) =>
                        ApprovalDetailScreen(
                          requestId: state.pathParameters['id']!,
                        ),
                  ),
                ],
              ),
            ],
          ),
          StatefulShellBranch(
            routes: <RouteBase>[
              GoRoute(
                path: '/notifications',
                name: 'notifications',
                builder: (BuildContext context, GoRouterState state) =>
                    const NotificationsScreen(),
              ),
            ],
          ),
        ],
      ),
      GoRoute(
        path: '/assets/:tag',
        name: 'asset-detail',
        builder: (BuildContext context, GoRouterState state) =>
            AssetDetailScreen(tag: state.pathParameters['tag']!),
      ),
      GoRoute(
        path: '/account',
        name: 'account',
        builder: (BuildContext context, GoRouterState state) =>
            const _AccountPlaceholderScreen(),
      ),
      GoRoute(
        path: '/settings',
        name: 'settings',
        builder: (BuildContext context, GoRouterState state) =>
            _ComingSoonScreen(
              title: (AppLocalizations l10n) => l10n.settingsTitle,
              icon: Symbols.settings_rounded,
            ),
      ),
    ],
  );
  ref.onDispose(router.dispose);
  return router;
});

/// Jembatan Riverpod -> [Listenable] untuk `refreshListenable` go_router.
class _RouterRefreshNotifier extends ChangeNotifier {
  void notify() => notifyListeners();
}

/// Placeholder Profil (Task 12 plan M0) + aksi logout SEMENTARA di app bar:
/// sejak Beranda menjadi layar ringkasan 1:1 (Task 11) tanpa aksi logout di
/// mockup-nya, akses logout menumpang di sini (avatar Beranda menuju rute
/// ini) sampai layar Profil menggantikannya di Task 12.
class _AccountPlaceholderScreen extends ConsumerWidget {
  const _AccountPlaceholderScreen();

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final AppLocalizations l10n = AppLocalizations.of(context);

    return Scaffold(
      appBar: AppBar(
        title: Text(l10n.accountTitle),
        actions: <Widget>[
          IconButton(
            tooltip: l10n.homeLogoutTooltip,
            icon: const Icon(Symbols.logout_rounded),
            onPressed: () async {
              final bool confirmed = await ConfirmDialog.show(
                context,
                title: l10n.homeLogoutConfirmTitle,
                message: l10n.homeLogoutConfirmMessage,
                confirmLabel: l10n.homeLogoutConfirmAction,
                icon: Symbols.logout_rounded,
                destructive: true,
              );
              if (confirmed) {
                // Guard router memindahkan ke /login begitu sesi berakhir.
                await ref.read(authControllerProvider.notifier).logout();
              }
            },
          ),
        ],
      ),
      body: SafeArea(
        child: EmptyState(
          icon: Symbols.person_rounded,
          title: l10n.commonComingSoon,
          subtitle: l10n.commonComingSoonBody,
        ),
      ),
    );
  }
}

/// Placeholder rute yang layarnya menyusul di Task 8-12 plan M0.
class _ComingSoonScreen extends StatelessWidget {
  const _ComingSoonScreen({required this.title, required this.icon});

  final String Function(AppLocalizations) title;
  final IconData icon;

  @override
  Widget build(BuildContext context) {
    final AppLocalizations l10n = AppLocalizations.of(context);

    return Scaffold(
      appBar: AppBar(title: Text(title(l10n))),
      body: SafeArea(
        child: EmptyState(
          icon: icon,
          title: l10n.commonComingSoon,
          subtitle: l10n.commonComingSoonBody,
        ),
      ),
    );
  }
}
