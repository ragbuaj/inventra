import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';

import '../core/auth/auth_controller.dart';
import '../core/auth/auth_session.dart';
import '../features/account/presentation/account_security_screen.dart';
import '../features/account/presentation/profile_screen.dart';
import '../features/account/presentation/settings_screen.dart';
import '../features/approval/presentation/approval_detail_screen.dart';
import '../features/approval/presentation/approval_inbox_screen.dart';
import '../features/asset_detail/presentation/asset_detail_screen.dart';
import '../features/asset_register/presentation/asset_register_screen.dart';
import '../features/catalog/presentation/catalog_screen.dart';
import '../features/home/presentation/home_screen.dart';
import '../features/login/presentation/login_screen.dart';
import '../features/my_assets/presentation/my_assets_screen.dart';
import '../features/my_requests/presentation/my_requests_screen.dart';
import '../features/notifications/presentation/notifications_screen.dart';
import '../features/scan/presentation/scan_screen.dart';
import '../features/stock_opname/presentation/opname_counting_screen.dart';
import '../features/stock_opname/presentation/opname_session_list_screen.dart';
import '../features/stock_opname/presentation/opname_variance_screen.dart';
import 'shell.dart';

/// Router aplikasi: guard auth + seluruh tabel rute v1 (plan M0 Task 7;
/// seluruh layar terisi sejak Task 12). Layar sekunder (detail aset/approval/
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
        path: '/catalog',
        name: 'catalog',
        builder: (BuildContext context, GoRouterState state) =>
            const CatalogScreen(),
      ),
      GoRoute(
        path: '/register-asset',
        name: 'register-asset',
        builder: (BuildContext context, GoRouterState state) =>
            const AssetRegisterScreen(),
      ),
      GoRoute(
        path: '/my-assets',
        name: 'my-assets',
        builder: (BuildContext context, GoRouterState state) =>
            const MyAssetsScreen(),
      ),
      GoRoute(
        path: '/my-requests',
        name: 'my-requests',
        builder: (BuildContext context, GoRouterState state) =>
            const MyRequestsScreen(),
        routes: <RouteBase>[
          GoRoute(
            path: ':id',
            name: 'my-request-detail',
            builder: (BuildContext context, GoRouterState state) =>
                ApprovalDetailScreen(requestId: state.pathParameters['id']!),
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
            const ProfileScreen(),
      ),
      GoRoute(
        path: '/account-security',
        name: 'account-security',
        builder: (BuildContext context, GoRouterState state) =>
            const AccountSecurityScreen(),
      ),
      GoRoute(
        path: '/settings',
        name: 'settings',
        builder: (BuildContext context, GoRouterState state) =>
            const SettingsScreen(),
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
