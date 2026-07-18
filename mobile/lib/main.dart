import 'package:flutter/material.dart';

import 'app/theme.dart';
import 'core/i18n/gen/app_localizations.dart';

void main() {
  runApp(const InventraApp());
}

/// Root aplikasi: MaterialApp + tema Inventra + i18n.
///
/// Router go_router dan shell bottom-nav dipasang di fase berikutnya
/// (Task 7 plan M0); untuk sekarang home berupa placeholder.
class InventraApp extends StatelessWidget {
  const InventraApp({super.key});

  @override
  Widget build(BuildContext context) {
    return MaterialApp(
      onGenerateTitle: (BuildContext context) =>
          AppLocalizations.of(context).appTitle,
      theme: InventraTheme.light,
      darkTheme: InventraTheme.dark,
      localizationsDelegates: AppLocalizations.localizationsDelegates,
      // Default id; en menjadi fallback untuk perangkat non-id.
      supportedLocales: const <Locale>[Locale('id'), Locale('en')],
      home: const _PlaceholderHome(),
    );
  }
}

class _PlaceholderHome extends StatelessWidget {
  const _PlaceholderHome();

  @override
  Widget build(BuildContext context) {
    final AppLocalizations l10n = AppLocalizations.of(context);
    final ThemeData theme = Theme.of(context);

    return Scaffold(
      body: SafeArea(
        child: Center(
          child: Column(
            mainAxisAlignment: MainAxisAlignment.center,
            children: <Widget>[
              Text(l10n.appTitle, style: theme.textTheme.titleLarge),
              const SizedBox(height: 8),
              Text(l10n.commonComingSoon, style: theme.textTheme.bodySmall),
            ],
          ),
        ),
      ),
    );
  }
}
