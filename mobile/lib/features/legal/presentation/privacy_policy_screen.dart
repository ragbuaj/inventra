import 'package:flutter/material.dart';

import '../../../core/i18n/gen/app_localizations.dart';

/// Halaman Kebijakan Privasi (statis, i18n id/en): pengantar + beberapa seksi
/// judul/isi. Konten murni informatif, tidak memuat data pengguna, sehingga
/// aman dibuka tanpa autentikasi tambahan.
class PrivacyPolicyScreen extends StatelessWidget {
  const PrivacyPolicyScreen({super.key});

  @override
  Widget build(BuildContext context) {
    final AppLocalizations l10n = AppLocalizations.of(context);
    final ThemeData theme = Theme.of(context);
    final ColorScheme scheme = theme.colorScheme;

    final List<(String, String)> sections = <(String, String)>[
      (l10n.privacyCollectTitle, l10n.privacyCollectBody),
      (l10n.privacyUseTitle, l10n.privacyUseBody),
      (l10n.privacyStorageTitle, l10n.privacyStorageBody),
      (l10n.privacySharingTitle, l10n.privacySharingBody),
      (l10n.privacyRightsTitle, l10n.privacyRightsBody),
      (l10n.privacyContactTitle, l10n.privacyContactBody),
    ];

    return Scaffold(
      appBar: AppBar(title: Text(l10n.privacyTitle)),
      body: SafeArea(
        child: ListView(
          padding: const EdgeInsets.fromLTRB(20, 12, 20, 28),
          children: <Widget>[
            Text(
              l10n.privacyLastUpdated,
              style: TextStyle(
                fontSize: 12,
                color: theme.textTheme.labelSmall?.color,
              ),
            ),
            const SizedBox(height: 12),
            Text(
              l10n.privacyIntro,
              style: TextStyle(
                fontSize: 13.5,
                height: 1.55,
                color: scheme.onSurfaceVariant,
              ),
            ),
            const SizedBox(height: 8),
            for (final (String, String) section in sections)
              _PolicySection(title: section.$1, body: section.$2),
          ],
        ),
      ),
    );
  }
}

/// Satu seksi kebijakan: judul tebal + paragraf isi.
class _PolicySection extends StatelessWidget {
  const _PolicySection({required this.title, required this.body});

  final String title;
  final String body;

  @override
  Widget build(BuildContext context) {
    final ThemeData theme = Theme.of(context);
    final ColorScheme scheme = theme.colorScheme;

    return Padding(
      padding: const EdgeInsets.only(top: 18),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: <Widget>[
          Text(
            title,
            style: TextStyle(
              fontSize: 15,
              fontWeight: FontWeight.w700,
              color: scheme.onSurface,
            ),
          ),
          const SizedBox(height: 6),
          Text(
            body,
            style: TextStyle(
              fontSize: 13.5,
              height: 1.55,
              color: scheme.onSurfaceVariant,
            ),
          ),
        ],
      ),
    );
  }
}
