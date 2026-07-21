import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';
import 'package:material_symbols_icons/symbols.dart';

import '../../../core/auth/data/auth_repository.dart';
import '../../../core/i18n/gen/app_localizations.dart';

/// Lupa Password (FR-M1.5): dari Login, input email lalu `POST
/// /auth/password/forgot` (anti-enumerasi — pesan sama apa pun input).
/// Penetapan password baru diselesaikan lewat link email di halaman web.
class ForgotPasswordScreen extends ConsumerStatefulWidget {
  const ForgotPasswordScreen({super.key});

  @override
  ConsumerState<ForgotPasswordScreen> createState() =>
      _ForgotPasswordScreenState();
}

class _ForgotPasswordScreenState extends ConsumerState<ForgotPasswordScreen> {
  final TextEditingController _email = TextEditingController();
  bool _submitting = false;
  bool _sent = false;
  String? _error;

  @override
  void dispose() {
    _email.dispose();
    super.dispose();
  }

  Future<void> _submit() async {
    final AppLocalizations l10n = AppLocalizations.of(context);
    if (_email.text.trim().isEmpty) {
      setState(() => _error = l10n.forgotEmailRequired);
      return;
    }
    setState(() {
      _submitting = true;
      _error = null;
    });
    try {
      await ref.read(authRepositoryProvider).forgotPassword(_email.text);
      if (!mounted) {
        return;
      }
      setState(() {
        _submitting = false;
        _sent = true;
      });
    } on Object {
      if (!mounted) {
        return;
      }
      setState(() {
        _submitting = false;
        _error = l10n.forgotError;
      });
    }
  }

  @override
  Widget build(BuildContext context) {
    final AppLocalizations l10n = AppLocalizations.of(context);
    final ColorScheme scheme = Theme.of(context).colorScheme;

    return Scaffold(
      appBar: AppBar(title: Text(l10n.forgotTitle)),
      body: SafeArea(
        child: Padding(
          padding: const EdgeInsets.fromLTRB(20, 12, 20, 20),
          child: _sent
              ? _SentView(email: _email.text.trim())
              : Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: <Widget>[
                    Text(
                      l10n.forgotIntro,
                      style: TextStyle(color: scheme.onSurfaceVariant),
                    ),
                    const SizedBox(height: 16),
                    TextField(
                      controller: _email,
                      enabled: !_submitting,
                      keyboardType: TextInputType.emailAddress,
                      decoration: InputDecoration(labelText: l10n.forgotEmailLabel),
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
                      key: const ValueKey<String>('forgot-submit'),
                      onPressed: _submitting ? null : _submit,
                      style: FilledButton.styleFrom(
                        minimumSize: const Size.fromHeight(50),
                      ),
                      child: _submitting
                          ? const SizedBox(
                              width: 20,
                              height: 20,
                              child: CircularProgressIndicator(strokeWidth: 2.5),
                            )
                          : Text(l10n.forgotSubmit),
                    ),
                  ],
                ),
        ),
      ),
    );
  }
}

/// Konfirmasi anti-enumerasi: pesan SAMA baik email terdaftar maupun tidak.
class _SentView extends StatelessWidget {
  const _SentView({required this.email});

  final String email;

  @override
  Widget build(BuildContext context) {
    final AppLocalizations l10n = AppLocalizations.of(context);
    final ColorScheme scheme = Theme.of(context).colorScheme;

    return Center(
      child: Column(
        mainAxisSize: MainAxisSize.min,
        children: <Widget>[
          Container(
            width: 64,
            height: 64,
            decoration: BoxDecoration(
              color: scheme.primaryContainer,
              shape: BoxShape.circle,
            ),
            child: Icon(
              Symbols.mark_email_read_rounded,
              size: 30,
              color: scheme.onPrimaryContainer,
            ),
          ),
          const SizedBox(height: 16),
          Text(
            l10n.forgotSentTitle,
            style: const TextStyle(fontSize: 17, fontWeight: FontWeight.w700),
          ),
          const SizedBox(height: 8),
          Text(
            l10n.forgotSentBody,
            textAlign: TextAlign.center,
            style: TextStyle(fontSize: 13, color: scheme.onSurfaceVariant),
          ),
          const SizedBox(height: 20),
          FilledButton(
            key: const ValueKey<String>('forgot-back-to-login'),
            onPressed: () => context.go('/login'),
            style: FilledButton.styleFrom(minimumSize: const Size.fromHeight(48)),
            child: Text(l10n.forgotBackToLogin),
          ),
        ],
      ),
    );
  }
}
