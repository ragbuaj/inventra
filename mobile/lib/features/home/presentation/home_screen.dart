import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:material_symbols_icons/symbols.dart';

import '../../../core/auth/auth_controller.dart';
import '../../../core/i18n/gen/app_localizations.dart';
import '../../../core/widgets/confirm_dialog.dart';
import '../../../core/widgets/empty_state.dart';

/// Beranda sementara (Task 7 plan M0): placeholder + aksi logout di app bar
/// sebagai penanda sampai layar ringkasan (Task 11) dan menu profil (Task 12)
/// menggantikannya.
class HomeScreen extends ConsumerWidget {
  const HomeScreen({super.key});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final AppLocalizations l10n = AppLocalizations.of(context);

    return Scaffold(
      appBar: AppBar(
        title: Text(l10n.homeTitle),
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
          icon: Symbols.home_rounded,
          title: l10n.commonComingSoon,
          subtitle: l10n.commonComingSoonBody,
        ),
      ),
    );
  }
}
