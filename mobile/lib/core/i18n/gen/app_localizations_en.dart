// ignore: unused_import
import 'package:intl/intl.dart' as intl;
import 'app_localizations.dart';

// ignore_for_file: type=lint

/// The translations for English (`en`).
class AppLocalizationsEn extends AppLocalizations {
  AppLocalizationsEn([String locale = 'en']) : super(locale);

  @override
  String get appTitle => 'Inventra Mobile';

  @override
  String get commonComingSoon => 'Coming soon';

  @override
  String get commonComingSoonBody =>
      'This screen is under construction and will arrive in an upcoming update.';

  @override
  String get commonRetry => 'Retry';

  @override
  String get commonCancel => 'Cancel';

  @override
  String get commonOfflineBanner => 'Offline — scans saved on this device';

  @override
  String get commonSyncSynced => 'Synced';

  @override
  String commonSyncPending(int count) {
    return '$count not yet synced';
  }

  @override
  String get commonSyncSyncing => 'Syncing…';

  @override
  String get commonSyncFailed => 'Failed — try again';

  @override
  String get commonSyncOffline => 'Offline';

  @override
  String get shellTabHome => 'Home';

  @override
  String get shellTabOpname => 'Stocktake';

  @override
  String get shellTabScan => 'Scan';

  @override
  String get shellTabApproval => 'Approvals';

  @override
  String get shellTabNotifications => 'Alerts';

  @override
  String get notificationsTitle => 'Notifications';

  @override
  String get assetDetailTitle => 'Asset Detail';

  @override
  String get approvalDetailTitle => 'Approval Detail';

  @override
  String get opnameDetailTitle => 'Stocktake Detail';

  @override
  String get opnameVarianceTitle => 'Stocktake Variance';

  @override
  String get accountTitle => 'Profile';

  @override
  String get settingsTitle => 'Settings';

  @override
  String get homeTitle => 'Home';

  @override
  String get homeLogoutTooltip => 'Sign out';

  @override
  String get homeLogoutConfirmTitle => 'Sign out of your account?';

  @override
  String get homeLogoutConfirmMessage =>
      'Your session on this device will be ended.';

  @override
  String get homeLogoutConfirmAction => 'Sign out';

  @override
  String get loginBrandName => 'Inventra';

  @override
  String get loginBrandBadge => 'MOBILE';

  @override
  String get loginTagline => 'Field companion for asset management';

  @override
  String get loginCardTitle => 'Sign in';

  @override
  String get loginCardSubtitle => 'Use your Inventra account';

  @override
  String get loginEmailLabel => 'Email';

  @override
  String get loginEmailHint => 'name@bank.co.id';

  @override
  String get loginPasswordLabel => 'Password';

  @override
  String get loginPasswordHint => 'Enter your password';

  @override
  String get loginShowPassword => 'Show password';

  @override
  String get loginHidePassword => 'Hide password';

  @override
  String get loginSubmitButton => 'Sign in';

  @override
  String get loginSubmitLoading => 'Processing…';

  @override
  String get loginErrorInvalidCredentials =>
      'Incorrect email or password. Try again.';

  @override
  String get loginErrorNetwork =>
      'No connection. Check your network and try again.';

  @override
  String get loginErrorRateLimited =>
      'Too many attempts. Try again in a moment.';

  @override
  String get loginErrorGeneric => 'Something went wrong. Try again.';

  @override
  String get loginLanguageIndonesian => 'ID';

  @override
  String get loginLanguageEnglish => 'EN';

  @override
  String loginVersion(String version, String build) {
    return 'Inventra Mobile v$version · Build $build';
  }
}
