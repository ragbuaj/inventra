import '../i18n/gen/app_localizations.dart';
import 'app_failure.dart';

/// Maps a caught error to a user-facing message for an action submit (sheets,
/// forms). Common transport/authorization/conflict failures get a specific,
/// actionable message; anything else falls back to the action's own generic
/// message. Keeps the four asset-action sheets and the registration form from
/// collapsing every failure into one opaque "gagal" string.
String actionFailureMessage(
  Object error,
  AppLocalizations l10n, {
  required String fallback,
}) {
  if (error is NetworkFailure) {
    return l10n.commonErrorNetwork;
  }
  if (error is ForbiddenFailure) {
    return l10n.commonErrorForbidden;
  }
  if (error is ConflictFailure) {
    return l10n.commonErrorConflict;
  }
  return fallback;
}
