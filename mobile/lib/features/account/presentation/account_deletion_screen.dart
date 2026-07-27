import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:material_symbols_icons/symbols.dart';
import 'package:url_launcher/url_launcher.dart';

import '../../../core/i18n/gen/app_localizations.dart';

/// Email kontak administrator/pelindung data untuk permintaan penghapusan.
/// Konfigurasi per organisasi — ganti sesuai unit yang menangani data pribadi.
const String accountDeletionContactEmail = 'admin@inventra.local';

/// Membangun URI `mailto:` prefilled (penerima + subjek + isi). Spasi di-encode
/// sebagai %20 (bukan '+') agar terbaca benar di aplikasi email. Dipisah agar
/// dapat diuji tanpa menyentuh platform url_launcher.
Uri accountDeletionMailtoUri({
  required String email,
  required String subject,
  required String body,
}) {
  return Uri.parse(
    'mailto:$email'
    '?subject=${Uri.encodeComponent(subject)}'
    '&body=${Uri.encodeComponent(body)}',
  );
}

/// Layar Penghapusan Akun & Data (FR privasi / syarat Play Store). Akun Inventra
/// dikelola administrator organisasi (tanpa pendaftaran mandiri), sehingga
/// penghapusan DIAJUKAN lewat admin — bukan hard-delete mandiri. Menjelaskan
/// data yang dihapus vs yang disimpan karena kewajiban regulasi perbankan.
class AccountDeletionScreen extends StatelessWidget {
  const AccountDeletionScreen({super.key});

  @override
  Widget build(BuildContext context) {
    final AppLocalizations l10n = AppLocalizations.of(context);
    final ThemeData theme = Theme.of(context);
    final ColorScheme scheme = theme.colorScheme;

    return Scaffold(
      appBar: AppBar(title: Text(l10n.accountDeletionTitle)),
      body: SafeArea(
        child: ListView(
          padding: const EdgeInsets.fromLTRB(20, 12, 20, 28),
          children: <Widget>[
            Text(
              l10n.accountDeletionIntro,
              style: TextStyle(
                fontSize: 13.5,
                height: 1.55,
                color: scheme.onSurfaceVariant,
              ),
            ),
            const SizedBox(height: 4),
            _Section(
              icon: Symbols.delete_sweep_rounded,
              title: l10n.accountDeletionDeletedTitle,
              body: l10n.accountDeletionDeletedBody,
            ),
            _Section(
              icon: Symbols.gavel_rounded,
              title: l10n.accountDeletionRetainedTitle,
              body: l10n.accountDeletionRetainedBody,
            ),
            _Section(
              icon: Symbols.support_agent_rounded,
              title: l10n.accountDeletionHowTitle,
              body: l10n.accountDeletionHowBody,
            ),
            const SizedBox(height: 18),
            const _ContactCard(email: accountDeletionContactEmail),
            const SizedBox(height: 12),
            Text(
              l10n.accountDeletionProcessNote,
              style: TextStyle(
                fontSize: 12,
                color: theme.textTheme.labelSmall?.color,
              ),
            ),
          ],
        ),
      ),
    );
  }
}

class _Section extends StatelessWidget {
  const _Section({
    required this.icon,
    required this.title,
    required this.body,
  });

  final IconData icon;
  final String title;
  final String body;

  @override
  Widget build(BuildContext context) {
    final ThemeData theme = Theme.of(context);
    final ColorScheme scheme = theme.colorScheme;

    return Padding(
      padding: const EdgeInsets.only(top: 18),
      child: Row(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: <Widget>[
          Icon(icon, size: 20, color: scheme.onSurfaceVariant),
          const SizedBox(width: 12),
          Expanded(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: <Widget>[
                Text(
                  title,
                  style: TextStyle(
                    fontSize: 14.5,
                    fontWeight: FontWeight.w700,
                    color: scheme.onSurface,
                  ),
                ),
                const SizedBox(height: 5),
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
          ),
        ],
      ),
    );
  }
}

/// Kartu kontak: alamat email admin + tombol "Kirim via email" (mailto prefilled
/// lewat url_launcher) dengan tombol Salin (Clipboard) sebagai fallback bila
/// tidak ada aplikasi email.
class _ContactCard extends StatelessWidget {
  const _ContactCard({required this.email});

  final String email;

  Future<void> _copy(BuildContext context) async {
    final AppLocalizations l10n = AppLocalizations.of(context);
    final ScaffoldMessengerState messenger = ScaffoldMessenger.of(context);
    await Clipboard.setData(ClipboardData(text: email));
    messenger.showSnackBar(
      SnackBar(content: Text(l10n.accountDeletionCopied)),
    );
  }

  Future<void> _sendEmail(BuildContext context) async {
    final AppLocalizations l10n = AppLocalizations.of(context);
    final ScaffoldMessengerState messenger = ScaffoldMessenger.of(context);
    final Uri uri = accountDeletionMailtoUri(
      email: email,
      subject: l10n.accountDeletionEmailSubject,
      body: l10n.accountDeletionEmailBody,
    );
    bool launched = false;
    try {
      launched = await launchUrl(uri);
    } on Object {
      launched = false;
    }
    if (!launched) {
      messenger.showSnackBar(
        SnackBar(content: Text(l10n.accountDeletionEmailFailed)),
      );
    }
  }

  @override
  Widget build(BuildContext context) {
    final AppLocalizations l10n = AppLocalizations.of(context);
    final ThemeData theme = Theme.of(context);
    final ColorScheme scheme = theme.colorScheme;

    return Container(
      padding: const EdgeInsets.fromLTRB(16, 14, 16, 14),
      decoration: BoxDecoration(
        color: theme.cardTheme.color ?? scheme.surface,
        borderRadius: BorderRadius.circular(16),
        border: Border.all(color: scheme.outlineVariant),
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: <Widget>[
          Text(
            l10n.accountDeletionContactLabel,
            style: TextStyle(fontSize: 12, color: scheme.onSurfaceVariant),
          ),
          const SizedBox(height: 2),
          Text(
            email,
            maxLines: 1,
            overflow: TextOverflow.ellipsis,
            style: const TextStyle(fontSize: 14, fontWeight: FontWeight.w600),
          ),
          const SizedBox(height: 12),
          // Tombol ditumpuk vertikal (bukan berbagi satu baris) supaya label
          // "Kirim via email" tetap satu baris di layar sempit.
          FilledButton.icon(
            key: const ValueKey<String>('account-deletion-email'),
            style: FilledButton.styleFrom(
              minimumSize: const Size.fromHeight(46),
            ),
            onPressed: () => _sendEmail(context),
            icon: const Icon(Symbols.mail_rounded, size: 18),
            label: Text(l10n.accountDeletionEmailButton),
          ),
          const SizedBox(height: 8),
          OutlinedButton.icon(
            key: const ValueKey<String>('account-deletion-copy'),
            style: OutlinedButton.styleFrom(
              minimumSize: const Size.fromHeight(46),
            ),
            onPressed: () => _copy(context),
            icon: const Icon(Symbols.content_copy_rounded, size: 18),
            label: Text(l10n.accountDeletionCopyButton),
          ),
        ],
      ),
    );
  }
}
