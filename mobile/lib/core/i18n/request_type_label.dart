import 'gen/app_localizations.dart';

/// Label i18n jenis pengajuan (enum `type` kontrak `Request` openapi.yaml).
///
/// Dinaikkan ke core karena dipakai lintas fitur: kartu/detail approval dan
/// isi notifikasi (`params.request_type` notifikasi memakai enum kawat yang
/// sama dengan `Request.type`). Nilai tak dikenal dirender apa adanya —
/// klien tidak menebak makna nilai baru dari server.
String requestTypeLabel(AppLocalizations l10n, String type) {
  return switch (type) {
    'asset_create' => l10n.approvalTypeAssetCreate,
    'asset_disposal' => l10n.approvalTypeAssetDisposal,
    'asset_transfer' => l10n.approvalTypeAssetTransfer,
    'assignment' => l10n.approvalTypeAssignment,
    'maintenance' => l10n.approvalTypeMaintenance,
    'valuation_exclusion' => l10n.approvalTypeValuationExclusion,
    _ => type,
  };
}
