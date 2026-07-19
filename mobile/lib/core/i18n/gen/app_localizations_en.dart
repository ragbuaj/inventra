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
  String get scanTitle => 'Scan Asset Label';

  @override
  String get scanHint => 'Point at the barcode / QR on the asset label';

  @override
  String get scanManualButton => 'Type code manually';

  @override
  String get scanCloseTooltip => 'Close scanner';

  @override
  String get scanTorchOnTooltip => 'Turn on flashlight';

  @override
  String get scanTorchOffTooltip => 'Turn off flashlight';

  @override
  String get scanCameraUnavailableTitle => 'Camera unavailable';

  @override
  String get scanCameraUnavailableBody =>
      'Allow camera access in your device settings, or use manual code entry.';

  @override
  String get scanManualSheetTitle => 'Type code manually';

  @override
  String get scanManualFieldLabel => 'Asset code';

  @override
  String get scanManualFieldHint => 'JKT01-ELK-2026-00001';

  @override
  String get scanManualFieldHelper => 'Format: OFFICE-CATEGORY-YEAR-NUMBER';

  @override
  String get scanManualSubmit => 'Search';

  @override
  String get assetDetailPhotoPlaceholder => 'No photo yet';

  @override
  String get assetDetailSectionPlacement => 'Placement';

  @override
  String get assetDetailSectionInfo => 'Information';

  @override
  String get assetDetailSectionValue => 'Value';

  @override
  String get assetDetailFieldOffice => 'Office';

  @override
  String get assetDetailFieldRoom => 'Floor / Room';

  @override
  String get assetDetailFieldHolder => 'Current holder';

  @override
  String get assetDetailFieldCategory => 'Category';

  @override
  String get assetDetailFieldBrandModel => 'Brand / Model';

  @override
  String get assetDetailFieldSerial => 'Serial no.';

  @override
  String get assetDetailFieldPurchaseDate => 'Purchase date';

  @override
  String get assetDetailFieldVendor => 'Vendor';

  @override
  String get assetDetailFieldPurchaseCost => 'Purchase cost';

  @override
  String get assetDetailFieldBookValue => 'Book value';

  @override
  String get assetDetailRestrictedBadge => 'Restricted for your role';

  @override
  String get assetDetailRestrictedTooltip =>
      'This field is restricted for your role';

  @override
  String get assetDetailStatusAvailable => 'Available';

  @override
  String get assetDetailStatusAssigned => 'Assigned';

  @override
  String get assetDetailStatusUnderMaintenance => 'Under Maintenance';

  @override
  String get assetDetailStatusInTransfer => 'In Transfer';

  @override
  String get assetDetailStatusRetired => 'Retired';

  @override
  String get assetDetailStatusDisposed => 'Disposed';

  @override
  String get assetDetailStatusLost => 'Lost';

  @override
  String get assetDetailErrorTitle => 'Failed to load asset detail';

  @override
  String get assetDetailErrorNetworkBody =>
      'No connection. Check your network and try again.';

  @override
  String get assetDetailErrorGenericBody => 'Something went wrong. Try again.';

  @override
  String get assetDetailForbiddenTitle => 'Access restricted';

  @override
  String get assetDetailForbiddenBody =>
      'Your role does not have permission to view assets.';

  @override
  String get assetDetailNotFoundTitle => 'Unknown code';

  @override
  String assetDetailNotFoundBody(String tag) {
    return 'Code $tag is not registered, or this asset is outside your authority.';
  }

  @override
  String get assetDetailScanAgain => 'Scan Again';

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

  @override
  String get approvalInboxTitle => 'Approvals';

  @override
  String get approvalInboxFilterPending => 'Pending';

  @override
  String get approvalInboxFilterApproved => 'Approved';

  @override
  String get approvalInboxFilterRejected => 'Rejected';

  @override
  String get approvalInboxFilterAll => 'All';

  @override
  String get approvalInboxPullToRefresh => 'Pull to refresh';

  @override
  String get approvalInboxEmptyPendingTitle => 'No pending requests';

  @override
  String get approvalInboxEmptyPendingBody =>
      'Every request in your scope has been decided. Nice work!';

  @override
  String get approvalInboxEmptyPendingAction => 'View history';

  @override
  String get approvalInboxEmptyFilteredTitle => 'No requests';

  @override
  String get approvalInboxEmptyFilteredBody =>
      'There are no requests with this status in your scope yet.';

  @override
  String get approvalInboxErrorTitle => 'Failed to load requests';

  @override
  String get approvalInboxErrorNetworkBody =>
      'No connection. Check your network and try again.';

  @override
  String get approvalInboxErrorGenericBody =>
      'Something went wrong. Try again.';

  @override
  String get approvalInboxForbiddenTitle => 'Access restricted';

  @override
  String get approvalInboxForbiddenBody =>
      'Your role does not have permission to view requests.';

  @override
  String get approvalInboxLoadMoreFailed => 'Failed to load the next page';

  @override
  String get approvalCardSensitive => 'sensitive';

  @override
  String get approvalTypeAssetCreate => 'Asset Registration';

  @override
  String get approvalTypeAssetDisposal => 'Disposal';

  @override
  String get approvalTypeAssetTransfer => 'Transfer';

  @override
  String get approvalTypeAssignment => 'Assignment';

  @override
  String get approvalTypeMaintenance => 'Maintenance';

  @override
  String get approvalTypeValuationExclusion => 'Valuation Exclusion';

  @override
  String get approvalStatusPending => 'Pending';

  @override
  String get approvalStatusApproved => 'Approved';

  @override
  String get approvalStatusRejected => 'Rejected';

  @override
  String get approvalStatusCancelled => 'Cancelled';

  @override
  String get approvalTimeJustNow => 'just now';

  @override
  String approvalTimeMinutesAgo(int count) {
    return '$count min ago';
  }

  @override
  String approvalTimeHoursAgo(int count) {
    return '$count hr ago';
  }

  @override
  String get approvalTimeYesterday => 'yesterday';

  @override
  String approvalTimeDaysAgo(int count) {
    return '$count days ago';
  }

  @override
  String get approvalDetailSensitiveBanner =>
      'Sensitive action — review carefully before deciding';

  @override
  String get approvalDetailSectionData => 'Submitted data';

  @override
  String get approvalDetailSectionSteps => 'Approval chain';

  @override
  String get approvalDetailFieldAsset => 'Asset';

  @override
  String get approvalDetailFieldAmount => 'Request amount';

  @override
  String get approvalDetailFieldReason => 'Reason';

  @override
  String get approvalDetailFieldName => 'Asset name';

  @override
  String get approvalDetailFieldCategory => 'Category';

  @override
  String get approvalDetailFieldOffice => 'Office';

  @override
  String get approvalDetailFieldRoom => 'Room';

  @override
  String get approvalDetailFieldOfficeChange => 'Placement office';

  @override
  String get approvalDetailFieldAssetClass => 'Asset class';

  @override
  String get approvalDetailAssetClassTangible => 'Tangible';

  @override
  String get approvalDetailAssetClassIntangible => 'Intangible';

  @override
  String get approvalDetailFieldPurchaseCost => 'Purchase cost';

  @override
  String get approvalDetailFieldPurchaseDate => 'Purchase date';

  @override
  String get approvalDetailFieldSerial => 'Serial no.';

  @override
  String get approvalDetailFieldBrandModel => 'Brand / Model';

  @override
  String get approvalDetailFieldVendor => 'Vendor';

  @override
  String get approvalDetailFieldPoNumber => 'PO no.';

  @override
  String get approvalDetailFieldFundingSource => 'Funding source';

  @override
  String get approvalDetailFieldWarrantyExpiry => 'Warranty expiry';

  @override
  String get approvalDetailFieldNotes => 'Notes';

  @override
  String get approvalDetailFieldMethod => 'Disposal method';

  @override
  String get approvalDetailMethodSale => 'Sale';

  @override
  String get approvalDetailMethodAuction => 'Auction';

  @override
  String get approvalDetailMethodDonation => 'Donation';

  @override
  String get approvalDetailMethodWriteOff => 'Write-off';

  @override
  String get approvalDetailFieldDisposalDate => 'Disposal date';

  @override
  String get approvalDetailFieldProceeds => 'Proceeds';

  @override
  String get approvalDetailFieldBookValue => 'Book value';

  @override
  String get approvalDetailFieldBastNo => 'BAST no.';

  @override
  String get approvalDetailFieldConditionSent => 'Condition when sent';

  @override
  String get approvalDetailFieldTransferDate => 'Transfer date';

  @override
  String get approvalDetailRestrictedData => 'Restricted for your role';

  @override
  String get approvalDetailStepMaker => 'Maker';

  @override
  String approvalDetailStepSubmitted(String date) {
    return 'Submitted · $date';
  }

  @override
  String approvalDetailStepApproved(String date) {
    return 'Approved · $date';
  }

  @override
  String approvalDetailStepRejected(String date) {
    return 'Rejected · $date';
  }

  @override
  String get approvalDetailStepWaiting => 'Awaiting decision';

  @override
  String get approvalDetailStepUpcoming => 'Up next';

  @override
  String get approvalDetailLevelOffice => 'Office approver';

  @override
  String get approvalDetailLevelOfficeSubtree => 'Office & subtree approver';

  @override
  String get approvalDetailLevelWilayah => 'Regional approver';

  @override
  String get approvalDetailLevelPusat => 'Head-office approver';

  @override
  String get approvalDetailNoteHint => 'Add a note (optional)';

  @override
  String get approvalDetailApprove => 'Approve';

  @override
  String get approvalDetailReject => 'Reject';

  @override
  String get approvalDetailApproveConfirmTitle => 'Approve this request?';

  @override
  String approvalDetailApproveConfirmBody(String title, String maker) {
    return '$title from $maker will be approved and move to the next step.';
  }

  @override
  String get approvalDetailApproveConfirmAction => 'Yes, Approve';

  @override
  String get approvalDetailRejectConfirmTitle => 'Reject this request?';

  @override
  String approvalDetailRejectConfirmBody(String title, String maker) {
    return '$title from $maker will be rejected and returned to the maker.';
  }

  @override
  String get approvalDetailRejectConfirmAction => 'Yes, Reject';

  @override
  String get approvalDetailYourNote => 'Your note';

  @override
  String get approvalDetailApprovedSnack => 'Request approved';

  @override
  String get approvalDetailRejectedSnack => 'Request rejected';

  @override
  String get approvalDetailDecidedApproved => 'This request has been approved';

  @override
  String get approvalDetailDecidedByYouApproved =>
      'You have approved this request';

  @override
  String get approvalDetailDecidedRejected => 'This request has been rejected';

  @override
  String get approvalDetailDecidedByYouRejected =>
      'You have rejected this request';

  @override
  String get approvalDetailDecidedCancelled =>
      'This request was cancelled by the maker';

  @override
  String get approvalDetailSodOwnRequest =>
      'This is your own request — the decision awaits another approver (makers may not decide their own requests).';

  @override
  String get approvalDetailErrorSod =>
      'You are not allowed to decide this request — makers and prior approvers may not decide their own requests.';

  @override
  String get approvalDetailErrorConflict =>
      'This request has already changed status elsewhere. Reloading…';

  @override
  String get approvalDetailErrorNetwork =>
      'No connection. Check your network and try again.';

  @override
  String get approvalDetailErrorGeneric => 'Something went wrong. Try again.';

  @override
  String get approvalDetailErrorTitle => 'Failed to load the request';

  @override
  String get approvalDetailNotFoundTitle => 'Request not found';

  @override
  String get approvalDetailNotFoundBody =>
      'The request does not exist or is outside your scope.';

  @override
  String get approvalDetailForbiddenTitle => 'Access restricted';

  @override
  String get approvalDetailForbiddenBody =>
      'Your role does not have permission to view this request.';
}
