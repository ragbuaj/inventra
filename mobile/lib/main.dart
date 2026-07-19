import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';

import 'app/locale_controller.dart';
import 'app/router.dart';
import 'app/theme.dart';
import 'core/i18n/gen/app_localizations.dart';

void main() {
  runApp(const ProviderScope(child: InventraApp()));
}

/// Root aplikasi: MaterialApp.router + tema Inventra + i18n + guard auth
/// (lihat app/router.dart).
class InventraApp extends ConsumerWidget {
  const InventraApp({super.key});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final GoRouter router = ref.watch(appRouterProvider);
    final Locale? locale = ref.watch(localeControllerProvider);

    return MaterialApp.router(
      onGenerateTitle: (BuildContext context) =>
          AppLocalizations.of(context).appTitle,
      theme: InventraTheme.light,
      darkTheme: InventraTheme.dark,
      routerConfig: router,
      locale: locale,
      localizationsDelegates: AppLocalizations.localizationsDelegates,
      // Default id; en menjadi fallback untuk perangkat non-id.
      supportedLocales: const <Locale>[Locale('id'), Locale('en')],
    );
  }
}
