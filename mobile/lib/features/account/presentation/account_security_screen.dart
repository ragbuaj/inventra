import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:material_symbols_icons/symbols.dart';

import '../../../core/api/app_failure.dart';
import '../../../core/i18n/gen/app_localizations.dart';
import '../data/account_security_repository.dart';
import 'account_providers.dart';

/// Validasi format email ringan sisi-klien (bukan pengganti validasi server).
final RegExp _emailPattern = RegExp(r'^[^@\s]+@[^@\s]+\.[^@\s]+$');

/// Keamanan Akun (FR-M6.3): ganti password & email berbasis link email. Klien
/// hanya memulai; penetapan/konfirmasi di halaman web.
class AccountSecurityScreen extends ConsumerWidget {
  const AccountSecurityScreen({super.key});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final AppLocalizations l10n = AppLocalizations.of(context);
    final String email =
        ref.watch(accountProfileProvider).value?.email ?? '—';

    return Scaffold(
      appBar: AppBar(title: Text(l10n.securityTitle)),
      body: SafeArea(
        child: ListView(
          padding: const EdgeInsets.fromLTRB(20, 12, 20, 20),
          children: <Widget>[
            _SecurityRow(
              icon: Symbols.mail_rounded,
              label: l10n.securityEmailLabel,
              value: email,
              action: l10n.securityChangeEmail,
              actionKey: 'security-change-email',
              onAction: () => showModalBottomSheet<void>(
                context: context,
                isScrollControlled: true,
                showDragHandle: true,
                builder: (BuildContext ctx) => const _EmailChangeSheet(),
              ),
            ),
            const SizedBox(height: 12),
            _SecurityRow(
              icon: Symbols.lock_rounded,
              label: l10n.securityPasswordLabel,
              value: '••••••••',
              action: l10n.securityChangePassword,
              actionKey: 'security-change-password',
              onAction: () => showModalBottomSheet<void>(
                context: context,
                isScrollControlled: true,
                showDragHandle: true,
                builder: (BuildContext ctx) => const _PasswordChangeSheet(),
              ),
            ),
          ],
        ),
      ),
    );
  }
}

class _SecurityRow extends StatelessWidget {
  const _SecurityRow({
    required this.icon,
    required this.label,
    required this.value,
    required this.action,
    required this.actionKey,
    required this.onAction,
  });

  final IconData icon;
  final String label;
  final String value;
  final String action;
  final String actionKey;
  final VoidCallback onAction;

  @override
  Widget build(BuildContext context) {
    final ThemeData theme = Theme.of(context);
    final ColorScheme scheme = theme.colorScheme;

    return Container(
      padding: const EdgeInsets.fromLTRB(16, 12, 12, 12),
      decoration: BoxDecoration(
        color: theme.cardTheme.color ?? scheme.surface,
        borderRadius: BorderRadius.circular(18),
        border: Border.all(color: scheme.outlineVariant),
      ),
      child: Row(
        children: <Widget>[
          Icon(icon, size: 22, color: scheme.onSurfaceVariant),
          const SizedBox(width: 12),
          Expanded(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: <Widget>[
                Text(
                  label,
                  style: TextStyle(
                    fontSize: 12,
                    color: scheme.onSurfaceVariant,
                  ),
                ),
                const SizedBox(height: 2),
                Text(
                  value,
                  maxLines: 1,
                  overflow: TextOverflow.ellipsis,
                  style: const TextStyle(
                    fontSize: 14,
                    fontWeight: FontWeight.w600,
                  ),
                ),
              ],
            ),
          ),
          TextButton(
            key: ValueKey<String>(actionKey),
            onPressed: onAction,
            child: Text(action),
          ),
        ],
      ),
    );
  }
}

/// Konfirmasi "Cek email Anda" — dipakai kedua sheet setelah link dikirim.
class _CheckEmailView extends StatelessWidget {
  const _CheckEmailView();

  @override
  Widget build(BuildContext context) {
    final AppLocalizations l10n = AppLocalizations.of(context);
    final ColorScheme scheme = Theme.of(context).colorScheme;

    return Column(
      mainAxisSize: MainAxisSize.min,
      children: <Widget>[
        Container(
          width: 56,
          height: 56,
          decoration: BoxDecoration(
            color: scheme.primaryContainer,
            shape: BoxShape.circle,
          ),
          child: Icon(
            Symbols.mark_email_read_rounded,
            color: scheme.onPrimaryContainer,
          ),
        ),
        const SizedBox(height: 14),
        Text(
          l10n.securityCheckEmailTitle,
          style: const TextStyle(fontSize: 16, fontWeight: FontWeight.w700),
        ),
        const SizedBox(height: 6),
        Text(
          l10n.securityCheckEmailBody,
          textAlign: TextAlign.center,
          style: TextStyle(fontSize: 13, color: scheme.onSurfaceVariant),
        ),
        const SizedBox(height: 18),
        FilledButton(
          onPressed: () => Navigator.of(context).pop(),
          style: FilledButton.styleFrom(minimumSize: const Size.fromHeight(48)),
          child: Text(l10n.securityDone),
        ),
      ],
    );
  }
}

class _PasswordChangeSheet extends ConsumerStatefulWidget {
  const _PasswordChangeSheet();

  @override
  ConsumerState<_PasswordChangeSheet> createState() =>
      _PasswordChangeSheetState();
}

class _PasswordChangeSheetState extends ConsumerState<_PasswordChangeSheet> {
  final TextEditingController _current = TextEditingController();
  bool _submitting = false;
  bool _sent = false;
  String? _error;

  @override
  void dispose() {
    _current.dispose();
    super.dispose();
  }

  Future<void> _submit() async {
    final AppLocalizations l10n = AppLocalizations.of(context);
    if (_current.text.isEmpty) {
      setState(() => _error = l10n.securityCurrentPasswordRequired);
      return;
    }
    setState(() {
      _submitting = true;
      _error = null;
    });
    try {
      await ref
          .read(accountSecurityRepositoryProvider)
          .requestPasswordChange(_current.text);
      if (!mounted) {
        return;
      }
      setState(() {
        _submitting = false;
        _sent = true;
      });
    } on ValidationFailure {
      if (!mounted) {
        return;
      }
      setState(() {
        _submitting = false;
        _error = l10n.securityWrongPassword;
      });
    } on Object {
      if (!mounted) {
        return;
      }
      setState(() {
        _submitting = false;
        _error = l10n.securityError;
      });
    }
  }

  @override
  Widget build(BuildContext context) {
    final AppLocalizations l10n = AppLocalizations.of(context);
    final ColorScheme scheme = Theme.of(context).colorScheme;

    return Padding(
      padding: EdgeInsets.fromLTRB(
        20,
        0,
        20,
        20 + MediaQuery.of(context).viewInsets.bottom,
      ),
      child: _sent
          ? const _CheckEmailView()
          : Column(
              mainAxisSize: MainAxisSize.min,
              crossAxisAlignment: CrossAxisAlignment.start,
              children: <Widget>[
                Text(
                  l10n.securityChangePassword,
                  style: const TextStyle(
                    fontSize: 16,
                    fontWeight: FontWeight.w700,
                  ),
                ),
                const SizedBox(height: 12),
                TextField(
                  controller: _current,
                  enabled: !_submitting,
                  obscureText: true,
                  decoration: InputDecoration(
                    labelText: l10n.securityCurrentPassword,
                  ),
                ),
                const SizedBox(height: 8),
                Text(
                  l10n.securityPasswordWarning,
                  style: TextStyle(fontSize: 12, color: scheme.error),
                ),
                if (_error != null) ...<Widget>[
                  const SizedBox(height: 8),
                  Text(
                    _error!,
                    style: TextStyle(color: scheme.error, fontSize: 13),
                  ),
                ],
                const SizedBox(height: 16),
                FilledButton(
                  key: const ValueKey<String>('security-password-submit'),
                  onPressed: _submitting ? null : _submit,
                  style: FilledButton.styleFrom(
                    minimumSize: const Size.fromHeight(48),
                  ),
                  child: _submitting
                      ? const SizedBox(
                          width: 18,
                          height: 18,
                          child: CircularProgressIndicator(strokeWidth: 2.5),
                        )
                      : Text(l10n.securitySendResetLink),
                ),
              ],
            ),
    );
  }
}

class _EmailChangeSheet extends ConsumerStatefulWidget {
  const _EmailChangeSheet();

  @override
  ConsumerState<_EmailChangeSheet> createState() => _EmailChangeSheetState();
}

class _EmailChangeSheetState extends ConsumerState<_EmailChangeSheet> {
  final TextEditingController _email = TextEditingController();
  final TextEditingController _current = TextEditingController();
  bool _submitting = false;
  bool _sent = false;
  String? _error;

  @override
  void dispose() {
    _email.dispose();
    _current.dispose();
    super.dispose();
  }

  Future<void> _submit() async {
    final AppLocalizations l10n = AppLocalizations.of(context);
    final String email = _email.text.trim();
    if (email.isEmpty) {
      setState(() => _error = l10n.securityNewEmailRequired);
      return;
    }
    // Validasi format di klien: tanpa ini, email salah-format ditolak backend
    // sebagai 400 -> dipetakan ke "password lama salah" yang menyesatkan.
    if (!_emailPattern.hasMatch(email)) {
      setState(() => _error = l10n.securityInvalidEmail);
      return;
    }
    if (_current.text.isEmpty) {
      setState(() => _error = l10n.securityCurrentPasswordRequired);
      return;
    }
    setState(() {
      _submitting = true;
      _error = null;
    });
    try {
      await ref
          .read(accountSecurityRepositoryProvider)
          .requestEmailChange(
            newEmail: _email.text,
            currentPassword: _current.text,
          );
      if (!mounted) {
        return;
      }
      setState(() {
        _submitting = false;
        _sent = true;
      });
    } on ConflictFailure {
      if (!mounted) {
        return;
      }
      setState(() {
        _submitting = false;
        _error = l10n.securityEmailInUse;
      });
    } on ValidationFailure {
      if (!mounted) {
        return;
      }
      setState(() {
        _submitting = false;
        _error = l10n.securityWrongPassword;
      });
    } on Object {
      if (!mounted) {
        return;
      }
      setState(() {
        _submitting = false;
        _error = l10n.securityError;
      });
    }
  }

  @override
  Widget build(BuildContext context) {
    final AppLocalizations l10n = AppLocalizations.of(context);
    final ColorScheme scheme = Theme.of(context).colorScheme;

    return Padding(
      padding: EdgeInsets.fromLTRB(
        20,
        0,
        20,
        20 + MediaQuery.of(context).viewInsets.bottom,
      ),
      child: _sent
          ? const _CheckEmailView()
          : Column(
              mainAxisSize: MainAxisSize.min,
              crossAxisAlignment: CrossAxisAlignment.start,
              children: <Widget>[
                Text(
                  l10n.securityChangeEmail,
                  style: const TextStyle(
                    fontSize: 16,
                    fontWeight: FontWeight.w700,
                  ),
                ),
                const SizedBox(height: 12),
                TextField(
                  controller: _email,
                  enabled: !_submitting,
                  keyboardType: TextInputType.emailAddress,
                  decoration: InputDecoration(labelText: l10n.securityNewEmail),
                ),
                const SizedBox(height: 10),
                TextField(
                  controller: _current,
                  enabled: !_submitting,
                  obscureText: true,
                  decoration: InputDecoration(
                    labelText: l10n.securityCurrentPassword,
                  ),
                ),
                if (_error != null) ...<Widget>[
                  const SizedBox(height: 8),
                  Text(
                    _error!,
                    style: TextStyle(color: scheme.error, fontSize: 13),
                  ),
                ],
                const SizedBox(height: 16),
                FilledButton(
                  key: const ValueKey<String>('security-email-submit'),
                  onPressed: _submitting ? null : _submit,
                  style: FilledButton.styleFrom(
                    minimumSize: const Size.fromHeight(48),
                  ),
                  child: _submitting
                      ? const SizedBox(
                          width: 18,
                          height: 18,
                          child: CircularProgressIndicator(strokeWidth: 2.5),
                        )
                      : Text(l10n.securitySendVerifyLink),
                ),
              ],
            ),
    );
  }
}
