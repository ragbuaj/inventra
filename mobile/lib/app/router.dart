import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';
import 'package:material_symbols_icons/symbols.dart';

import '../core/auth/auth_controller.dart';
import '../core/auth/auth_session.dart';
import '../core/i18n/gen/app_localizations.dart';
import '../core/widgets/empty_state.dart';
import '../features/home/presentation/home_screen.dart';
import '../features/login/presentation/login_screen.dart';
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
                    _ComingSoonScreen(
                      title: (AppLocalizations l10n) => l10n.shellTabOpname,
                      icon: Symbols.fact_check_rounded,
                    ),
                routes: <RouteBase>[
                  GoRoute(
                    path: ':id',
                    name: 'stock-opname-detail',
                    parentNavigatorKey: rootNavigatorKey,
                    builder: (BuildContext context, GoRouterState state) =>
                        _ComingSoonScreen(
                          title: (AppLocalizations l10n) =>
                              l10n.opnameDetailTitle,
                          icon: Symbols.fact_check_rounded,
                        ),
                    routes: <RouteBase>[
                      GoRoute(
                        path: 'variance',
                        name: 'stock-opname-variance',
                        parentNavigatorKey: rootNavigatorKey,
                        builder: (BuildContext context, GoRouterState state) =>
                            _ComingSoonScreen(
                              title: (AppLocalizations l10n) =>
                                  l10n.opnameVarianceTitle,
                              icon: Symbols.fact_check_rounded,
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
                    _ComingSoonScreen(
                      title: (AppLocalizations l10n) => l10n.shellTabScan,
                      icon: Symbols.qr_code_scanner_rounded,
                    ),
              ),
            ],
          ),
          StatefulShellBranch(
            routes: <RouteBase>[
              GoRoute(
                path: '/approval',
                name: 'approval',
                builder: (BuildContext context, GoRouterState state) =>
                    _ComingSoonScreen(
                      title: (AppLocalizations l10n) => l10n.shellTabApproval,
                      icon: Symbols.approval_rounded,
                    ),
                routes: <RouteBase>[
                  GoRoute(
                    path: ':id',
                    name: 'approval-detail',
                    parentNavigatorKey: rootNavigatorKey,
                    builder: (BuildContext context, GoRouterState state) =>
                        _ComingSoonScreen(
                          title: (AppLocalizations l10n) =>
                              l10n.approvalDetailTitle,
                          icon: Symbols.approval_rounded,
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
                    _ComingSoonScreen(
                      title: (AppLocalizations l10n) => l10n.notificationsTitle,
                      icon: Symbols.notifications_rounded,
                    ),
              ),
            ],
          ),
        ],
      ),
      GoRoute(
        path: '/assets/:tag',
        name: 'asset-detail',
        builder: (BuildContext context, GoRouterState state) =>
            _ComingSoonScreen(
              title: (AppLocalizations l10n) => l10n.assetDetailTitle,
              icon: Symbols.inventory_2_rounded,
            ),
      ),
      GoRoute(
        path: '/account',
        name: 'account',
        builder: (BuildContext context, GoRouterState state) =>
            _ComingSoonScreen(
              title: (AppLocalizations l10n) => l10n.accountTitle,
              icon: Symbols.person_rounded,
            ),
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
