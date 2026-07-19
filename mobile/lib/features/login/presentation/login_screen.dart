import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:material_symbols_icons/symbols.dart';

import '../../../app/locale_controller.dart';
import '../../../app/theme.dart';
import '../../../core/api/app_failure.dart';
import '../../../core/i18n/gen/app_localizations.dart';
import '../../../core/utils/app_info.dart';
import 'login_controller.dart';

/// Layar login 1:1 mockup "Inventra Mobile - Login" (default/error/loading,
/// light + dark). Sukses login membuat guard router memindahkan pengguna ke
/// beranda — layar ini tidak melakukan navigasi sendiri.
class LoginScreen extends ConsumerStatefulWidget {
  const LoginScreen({super.key});

  @override
  ConsumerState<LoginScreen> createState() => _LoginScreenState();
}

class _LoginScreenState extends ConsumerState<LoginScreen> {
  final TextEditingController _emailController = TextEditingController();
  final TextEditingController _passwordController = TextEditingController();
  bool _obscurePassword = true;

  @override
  void dispose() {
    _emailController.dispose();
    _passwordController.dispose();
    super.dispose();
  }

  Future<void> _submit() async {
    FocusManager.instance.primaryFocus?.unfocus();
    await ref
        .read(loginControllerProvider.notifier)
        .submit(
          email: _emailController.text.trim(),
          password: _passwordController.text,
        );
  }

  String _failureMessage(AppLocalizations l10n, Object failure) {
    return switch (failure) {
      // 400 pada login diperlakukan sama dengan 401: input kredensial salah.
      UnauthorizedFailure() ||
      ValidationFailure() => l10n.loginErrorInvalidCredentials,
      NetworkFailure() => l10n.loginErrorNetwork,
      RateLimitedFailure() => l10n.loginErrorRateLimited,
      _ => l10n.loginErrorGeneric,
    };
  }

  @override
  Widget build(BuildContext context) {
    final ThemeData theme = Theme.of(context);
    final ColorScheme scheme = theme.colorScheme;
    final AppLocalizations l10n = AppLocalizations.of(context);
    final AsyncValue<void> loginState = ref.watch(loginControllerProvider);
    final bool isLoading = loginState.isLoading;
    final Object? failure = loginState.error;
    final String? errorMessage = failure == null
        ? null
        : _failureMessage(l10n, failure);

    // Tint radial hijau di atas layar: light memakai campuran
    // primaryContainer->primary (mendekati green-200 mockup), dark memakai
    // primary alpha 0.18 persis mockup.
    final Color gradientTint = scheme.brightness == Brightness.light
        ? Color.lerp(scheme.primaryContainer, scheme.primary, 0.15)!
        : scheme.primary.withValues(alpha: 0.18);

    return Scaffold(
      body: DecoratedBox(
        decoration: BoxDecoration(
          gradient: RadialGradient(
            center: const Alignment(0, -1.1),
            radius: 1.1,
            colors: <Color>[gradientTint, gradientTint.withValues(alpha: 0)],
          ),
        ),
        child: SafeArea(
          child: LayoutBuilder(
            builder: (BuildContext context, BoxConstraints constraints) {
              return SingleChildScrollView(
                child: ConstrainedBox(
                  constraints: BoxConstraints(minHeight: constraints.maxHeight),
                  child: IntrinsicHeight(
                    child: Column(
                      children: <Widget>[
                        const SizedBox(height: 56),
                        _Branding(l10n: l10n),
                        const SizedBox(height: 36),
                        _LoginCard(
                          l10n: l10n,
                          emailController: _emailController,
                          passwordController: _passwordController,
                          isLoading: isLoading,
                          errorMessage: errorMessage,
                          obscurePassword: _obscurePassword,
                          onToggleObscure: () => setState(() {
                            _obscurePassword = !_obscurePassword;
                          }),
                          onSubmit: _submit,
                        ),
                        const Spacer(),
                        const SizedBox(height: 24),
                        _Footer(l10n: l10n),
                        const SizedBox(height: 26),
                      ],
                    ),
                  ),
                ),
              );
            },
          ),
        ),
      ),
    );
  }
}

/// Logo kotak + wordmark "Inventra" + badge MOBILE + tagline.
class _Branding extends StatelessWidget {
  const _Branding({required this.l10n});

  final AppLocalizations l10n;

  @override
  Widget build(BuildContext context) {
    final ColorScheme scheme = Theme.of(context).colorScheme;

    return Column(
      children: <Widget>[
        Container(
          width: 74,
          height: 74,
          decoration: BoxDecoration(
            color: scheme.primary,
            borderRadius: BorderRadius.circular(22),
            boxShadow: <BoxShadow>[
              BoxShadow(
                color: scheme.primary.withValues(alpha: 0.35),
                blurRadius: 28,
                offset: const Offset(0, 12),
              ),
            ],
          ),
          child: Icon(
            Symbols.inventory_2_rounded,
            size: 40,
            color: scheme.onPrimary,
          ),
        ),
        const SizedBox(height: 18),
        Row(
          mainAxisSize: MainAxisSize.min,
          children: <Widget>[
            Text(
              l10n.loginBrandName,
              style: TextStyle(
                fontSize: 30,
                fontWeight: FontWeight.w800,
                letterSpacing: 30 * InventraDimens.titleLetterSpacingEm,
                color: scheme.onSurface,
              ),
            ),
            const SizedBox(width: 8),
            Container(
              padding: const EdgeInsets.symmetric(horizontal: 8, vertical: 4),
              decoration: ShapeDecoration(
                color: scheme.primaryContainer,
                shape: const StadiumBorder(),
              ),
              child: Text(
                l10n.loginBrandBadge,
                style: TextStyle(
                  fontSize: 10,
                  fontWeight: FontWeight.w700,
                  letterSpacing: 10 * 0.08,
                  color: scheme.onPrimaryContainer,
                ),
              ),
            ),
          ],
        ),
        const SizedBox(height: 8),
        Text(
          l10n.loginTagline,
          style: TextStyle(fontSize: 13.5, color: scheme.onSurfaceVariant),
        ),
      ],
    );
  }
}

/// Card form: judul, banner error inline, field email + kata sandi, tombol.
class _LoginCard extends StatelessWidget {
  const _LoginCard({
    required this.l10n,
    required this.emailController,
    required this.passwordController,
    required this.isLoading,
    required this.errorMessage,
    required this.obscurePassword,
    required this.onToggleObscure,
    required this.onSubmit,
  });

  final AppLocalizations l10n;
  final TextEditingController emailController;
  final TextEditingController passwordController;
  final bool isLoading;
  final String? errorMessage;
  final bool obscurePassword;
  final VoidCallback onToggleObscure;
  final Future<void> Function() onSubmit;

  @override
  Widget build(BuildContext context) {
    final ThemeData theme = Theme.of(context);
    final ColorScheme scheme = theme.colorScheme;
    final bool hasError = errorMessage != null;
    final Color cardColor = theme.cardTheme.color ?? scheme.surface;

    return Container(
      margin: const EdgeInsets.symmetric(horizontal: 20),
      padding: const EdgeInsets.symmetric(horizontal: 22, vertical: 24),
      decoration: BoxDecoration(
        color: cardColor,
        borderRadius: BorderRadius.circular(InventraDimens.radiusCardHero),
        border: Border.all(color: scheme.outlineVariant),
        boxShadow: scheme.brightness == Brightness.light
            ? <BoxShadow>[
                BoxShadow(
                  color: scheme.shadow.withValues(alpha: 0.08),
                  blurRadius: 32,
                  offset: const Offset(0, 12),
                ),
              ]
            : null,
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: <Widget>[
          Text(
            l10n.loginCardTitle,
            style: theme.textTheme.titleMedium?.copyWith(fontSize: 18),
          ),
          const SizedBox(height: 2),
          Text(
            l10n.loginCardSubtitle,
            style: TextStyle(fontSize: 12.5, color: scheme.onSurfaceVariant),
          ),
          SizedBox(height: hasError ? 16 : 20),
          if (hasError) ...<Widget>[
            Semantics(
              liveRegion: true,
              child: Container(
                padding: const EdgeInsets.symmetric(
                  horizontal: 13,
                  vertical: 11,
                ),
                decoration: BoxDecoration(
                  color: scheme.errorContainer,
                  borderRadius: BorderRadius.circular(12),
                  border: Border.all(
                    color: scheme.error.withValues(alpha: 0.35),
                  ),
                ),
                child: Row(
                  children: <Widget>[
                    Icon(Symbols.error_rounded, size: 19, color: scheme.error),
                    const SizedBox(width: 9),
                    Expanded(
                      child: Text(
                        errorMessage!,
                        style: TextStyle(
                          fontSize: 12.5,
                          fontWeight: FontWeight.w600,
                          color: scheme.onErrorContainer,
                        ),
                      ),
                    ),
                  ],
                ),
              ),
            ),
            const SizedBox(height: 16),
          ],
          _FieldLabel(text: l10n.loginEmailLabel, muted: isLoading),
          TextField(
            controller: emailController,
            enabled: !isLoading,
            keyboardType: TextInputType.emailAddress,
            textInputAction: TextInputAction.next,
            autofillHints: const <String>[AutofillHints.email],
            style: const TextStyle(fontSize: 14),
            decoration: _fieldDecoration(
              theme: theme,
              hint: l10n.loginEmailHint,
              hasError: hasError,
              prefixIcon: Symbols.mail_rounded,
            ),
          ),
          const SizedBox(height: 16),
          _FieldLabel(text: l10n.loginPasswordLabel, muted: isLoading),
          TextField(
            controller: passwordController,
            enabled: !isLoading,
            obscureText: obscurePassword,
            textInputAction: TextInputAction.done,
            autofillHints: const <String>[AutofillHints.password],
            onSubmitted: (_) => onSubmit(),
            style: const TextStyle(fontSize: 14),
            decoration:
                _fieldDecoration(
                  theme: theme,
                  hint: l10n.loginPasswordHint,
                  hasError: hasError,
                  prefixIcon: Symbols.lock_rounded,
                ).copyWith(
                  suffixIcon: IconButton(
                    tooltip: obscurePassword
                        ? l10n.loginShowPassword
                        : l10n.loginHidePassword,
                    onPressed: isLoading ? null : onToggleObscure,
                    icon: Icon(
                      obscurePassword
                          ? Symbols.visibility_off_rounded
                          : Symbols.visibility_rounded,
                      size: 21,
                      color: scheme.onSurfaceVariant,
                    ),
                  ),
                ),
          ),
          const SizedBox(height: 22),
          FilledButton(
            style: FilledButton.styleFrom(
              minimumSize: const Size.fromHeight(
                InventraDimens.buttonHeightPrimary,
              ),
              // copyWith dari labelLarge mempertahankan font Inter tema.
              textStyle: theme.textTheme.labelLarge?.copyWith(
                fontSize: 15,
                fontWeight: FontWeight.w700,
              ),
              // Saat loading tombol tetap hijau (mockup opacity 0.92), bukan
              // abu-abu disabled default Material.
              disabledBackgroundColor: scheme.primary.withValues(alpha: 0.92),
              disabledForegroundColor: scheme.onPrimary,
            ),
            onPressed: isLoading ? null : onSubmit,
            child: isLoading
                ? Row(
                    mainAxisSize: MainAxisSize.min,
                    children: <Widget>[
                      SizedBox(
                        width: 19,
                        height: 19,
                        child: CircularProgressIndicator(
                          strokeWidth: 2.5,
                          color: scheme.onPrimary,
                        ),
                      ),
                      const SizedBox(width: 10),
                      Text(l10n.loginSubmitLoading),
                    ],
                  )
                : Text(l10n.loginSubmitButton),
          ),
        ],
      ),
    );
  }

  InputDecoration _fieldDecoration({
    required ThemeData theme,
    required String hint,
    required bool hasError,
    required IconData prefixIcon,
  }) {
    final ColorScheme scheme = theme.colorScheme;
    final InputDecorationThemeData inputTheme = theme.inputDecorationTheme;
    final Color iconColor = hasError
        ? scheme.error
        : (theme.textTheme.labelSmall?.color ?? scheme.onSurfaceVariant);

    return InputDecoration(
      hintText: hint,
      prefixIcon: Padding(
        padding: const EdgeInsets.only(left: 16, right: 10),
        child: Icon(prefixIcon, size: 20, color: iconColor),
      ),
      prefixIconConstraints: const BoxConstraints(minWidth: 0, minHeight: 0),
      // Border error dipasang pada state normal juga karena kegagalan datang
      // dari banner (bukan errorText per field) sesuai mockup.
      enabledBorder: hasError ? inputTheme.errorBorder : null,
      focusedBorder: hasError ? inputTheme.focusedErrorBorder : null,
    );
  }
}

class _FieldLabel extends StatelessWidget {
  const _FieldLabel({required this.text, required this.muted});

  final String text;
  final bool muted;

  @override
  Widget build(BuildContext context) {
    final ThemeData theme = Theme.of(context);
    final Color color = muted
        ? (theme.textTheme.labelSmall?.color ??
              theme.colorScheme.onSurfaceVariant)
        : (theme.textTheme.labelLarge?.color ?? theme.colorScheme.onSurface);

    return Padding(
      padding: const EdgeInsets.only(bottom: 6),
      child: Text(
        text,
        style: TextStyle(
          fontSize: 12,
          fontWeight: FontWeight.w600,
          color: color,
        ),
      ),
    );
  }
}

/// Footer: pill switch bahasa ID/EN (mengubah locale app) + versi aplikasi.
class _Footer extends ConsumerWidget {
  const _Footer({required this.l10n});

  final AppLocalizations l10n;

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final ThemeData theme = Theme.of(context);
    final ColorScheme scheme = theme.colorScheme;
    final String activeLanguage = Localizations.localeOf(context).languageCode;

    return Column(
      children: <Widget>[
        Container(
          padding: const EdgeInsets.all(3),
          decoration: ShapeDecoration(
            color: theme.cardTheme.color ?? scheme.surface,
            shape: StadiumBorder(
              side: BorderSide(color: scheme.outlineVariant),
            ),
          ),
          child: Row(
            mainAxisSize: MainAxisSize.min,
            children: <Widget>[
              _LanguageSegment(
                label: l10n.loginLanguageIndonesian,
                locale: const Locale('id'),
                active: activeLanguage == 'id',
              ),
              _LanguageSegment(
                label: l10n.loginLanguageEnglish,
                locale: const Locale('en'),
                active: activeLanguage == 'en',
              ),
            ],
          ),
        ),
        const SizedBox(height: 12),
        Text(
          l10n.loginVersion(AppInfo.version, AppInfo.buildNumber),
          style: TextStyle(
            fontSize: 11,
            color: theme.textTheme.labelSmall?.color,
          ),
        ),
      ],
    );
  }
}

class _LanguageSegment extends ConsumerWidget {
  const _LanguageSegment({
    required this.label,
    required this.locale,
    required this.active,
  });

  final String label;
  final Locale locale;
  final bool active;

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final ColorScheme scheme = Theme.of(context).colorScheme;

    return Semantics(
      button: true,
      selected: active,
      child: InkWell(
        customBorder: const StadiumBorder(),
        onTap: () =>
            ref.read(localeControllerProvider.notifier).setLocale(locale),
        child: Container(
          padding: const EdgeInsets.symmetric(horizontal: 14, vertical: 5),
          decoration: active
              ? ShapeDecoration(
                  color: scheme.primary,
                  shape: const StadiumBorder(),
                )
              : null,
          child: Text(
            label,
            style: TextStyle(
              fontSize: 11.5,
              fontWeight: active ? FontWeight.w700 : FontWeight.w600,
              color: active ? scheme.onPrimary : scheme.onSurfaceVariant,
            ),
          ),
        ),
      ),
    );
  }
}
