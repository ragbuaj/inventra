import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:inventra_mobile/app/locale_controller.dart';
import 'package:inventra_mobile/app/router.dart';
import 'package:inventra_mobile/app/theme.dart';
import 'package:inventra_mobile/core/i18n/gen/app_localizations.dart';

/// Resolusi ARB langsung untuk assertion via kunci i18n.
final AppLocalizations l10nId = lookupAppLocalizations(const Locale('id'));
final AppLocalizations l10nEn = lookupAppLocalizations(const Locale('en'));

/// App penuh dengan router + guard, dikendalikan [ProviderContainer] milik tes
/// (lewat [UncontrolledProviderScope]) supaya tes bisa membaca GoRouter.
class RouterTestApp extends StatelessWidget {
  const RouterTestApp({required this.container, super.key});

  final ProviderContainer container;

  @override
  Widget build(BuildContext context) {
    return UncontrolledProviderScope(
      container: container,
      child: const _LocaleAwareMaterialApp(home: null, useRouter: true),
    );
  }
}

/// Harness satu layar (tanpa router) yang tetap merespons perubahan locale —
/// dipakai tes LoginScreen dan komponen bersama. [container] opsional untuk
/// override provider (buat dengan `ProviderContainer.test(overrides: ...)`).
Widget buildScreenHarness({
  required Widget child,
  ProviderContainer? container,
  ThemeData? theme,
}) {
  final Widget app = _LocaleAwareMaterialApp(
    home: child,
    useRouter: false,
    theme: theme,
  );
  if (container == null) {
    return ProviderScope(child: app);
  }
  return UncontrolledProviderScope(container: container, child: app);
}

/// Harness komponen kecil: layar terang default + Scaffold pembungkus.
Widget buildWidgetHarness(Widget child, {ThemeData? theme}) {
  return buildScreenHarness(
    theme: theme,
    child: Scaffold(body: Center(child: child)),
  );
}

class _LocaleAwareMaterialApp extends ConsumerWidget {
  const _LocaleAwareMaterialApp({
    required this.home,
    required this.useRouter,
    this.theme,
  });

  final Widget? home;
  final bool useRouter;
  final ThemeData? theme;

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    // Perangkat tes ber-locale en_US; tanpa pilihan pengguna, paksa id agar
    // assertion kunci ARB id konsisten dengan perilaku produk di lapangan.
    final Locale locale =
        ref.watch(localeControllerProvider) ?? const Locale('id');
    const List<Locale> locales = <Locale>[Locale('id'), Locale('en')];

    if (useRouter) {
      return MaterialApp.router(
        theme: theme ?? InventraTheme.light,
        darkTheme: InventraTheme.dark,
        routerConfig: ref.watch(appRouterProvider),
        locale: locale,
        localizationsDelegates: AppLocalizations.localizationsDelegates,
        supportedLocales: locales,
      );
    }
    return MaterialApp(
      theme: theme ?? InventraTheme.light,
      locale: locale,
      localizationsDelegates: AppLocalizations.localizationsDelegates,
      supportedLocales: locales,
      home: home,
    );
  }
}
