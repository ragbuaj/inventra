import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';

import 'app/locale_controller.dart';
import 'app/router.dart';
import 'app/theme.dart';
import 'app/theme_mode_controller.dart';
import 'core/i18n/gen/app_localizations.dart';
import 'core/prefs/app_preferences.dart';

Future<void> main() async {
  WidgetsFlutterBinding.ensureInitialized();
  // Preferensi tampilan (bahasa + tema) dibaca SEBELUM frame pertama supaya
  // cold start langsung merender pilihan tersimpan tanpa kedipan tema/bahasa.
  final AppPreferences preferences = await SharedPrefsAppPreferences.create();
  runApp(
    ProviderScope(
      overrides: [appPreferencesProvider.overrideWithValue(preferences)],
      child: const InventraApp(),
    ),
  );
}

/// Root aplikasi: MaterialApp.router + tema Inventra (mode dari preferensi) +
/// i18n + guard auth (lihat app/router.dart).
class InventraApp extends ConsumerWidget {
  const InventraApp({super.key});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final GoRouter router = ref.watch(appRouterProvider);
    final Locale? locale = ref.watch(localeControllerProvider);
    final ThemeMode themeMode = ref.watch(themeModeControllerProvider);

    return MaterialApp.router(
      onGenerateTitle: (BuildContext context) =>
          AppLocalizations.of(context).appTitle,
      theme: InventraTheme.light,
      darkTheme: InventraTheme.dark,
      themeMode: themeMode,
      routerConfig: router,
      locale: locale,
      localizationsDelegates: AppLocalizations.localizationsDelegates,
      // Default id; en menjadi fallback untuk perangkat non-id.
      supportedLocales: const <Locale>[Locale('id'), Locale('en')],
    );
  }
}
