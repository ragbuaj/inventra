import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';
import 'package:intl/intl.dart';
import 'package:material_symbols_icons/symbols.dart';

import '../../../app/theme.dart';
import '../../../core/api/app_failure.dart';
import '../../../core/i18n/gen/app_localizations.dart';
import '../../../core/widgets/app_skeleton.dart';
import '../../../core/widgets/empty_state.dart';
import '../../../core/widgets/status_chip.dart';
import '../data/asset_detail_repository.dart';
import '../data/asset_dto.dart';
import 'asset_action_bar.dart';
import 'asset_by_tag_provider.dart';
import 'asset_reference_names_provider.dart';

/// Nilai kosong: field null, dimask, atau nama referensi belum/gagal
/// ter-resolve.
const String _emDash = '—';

/// Layar Detail Aset 1:1 mockup "Inventra Mobile - Detail Aset" (read-only,
/// di atas shell tanpa bottom nav). Field yang TIDAK dikirim backend (field
/// permission) dirender em-dash dengan penanda dibatasi — klien tidak menebak.
/// Nilai referensi (kategori/kantor/dst.) di-resolve ke NAMA lewat lookup
/// master data non-fatal ([assetReferenceNamesProvider]); lookup gagal berarti
/// em-dash — UUID mentah tidak pernah ditampilkan.
class AssetDetailScreen extends ConsumerWidget {
  const AssetDetailScreen({required this.tag, super.key});

  final String tag;

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final AppLocalizations l10n = AppLocalizations.of(context);
    final AsyncValue<AssetDetailData> state = ref.watch(
      assetByTagProvider(tag),
    );
    // Resolusi nama berjalan paralel dan tidak memblokir render: null selama
    // loading/gagal -> sel merender em-dash, terisi saat resolusi selesai.
    final AssetReferenceNames? names = ref
        .watch(assetReferenceNamesProvider(tag))
        .value;

    // Bar aksi FR-M7 hanya saat detail termuat (butuh status + id aset); di
    // luar sesi opname. Read-only murni bila pengguna tak punya aksi.
    final AssetDto? loaded = state.value?.asset;

    return Scaffold(
      appBar: AppBar(title: Text(l10n.assetDetailTitle)),
      body: SafeArea(
        child: state.when(
          data: (AssetDetailData data) =>
              _AssetDetailBody(data: data, names: names),
          loading: () => const _LoadingSkeleton(),
          error: (Object error, StackTrace stackTrace) => _ErrorState(
            failure: error,
            tag: tag,
            onRetry: () => ref.invalidate(assetByTagProvider(tag)),
          ),
        ),
      ),
      bottomNavigationBar: loaded == null ? null : AssetActionBar(asset: loaded),
    );
  }
}

/// Empat cabang error: 404 (tag tak dikenal/di luar scope), 403 (tanpa izin
/// asset.view), offline, dan generik — masing-masing [EmptyState] sendiri.
class _ErrorState extends StatelessWidget {
  const _ErrorState({
    required this.failure,
    required this.tag,
    required this.onRetry,
  });

  final Object failure;
  final String tag;
  final VoidCallback onRetry;

  @override
  Widget build(BuildContext context) {
    final AppLocalizations l10n = AppLocalizations.of(context);

    return switch (failure) {
      NotFoundFailure() => EmptyState(
        icon: Symbols.question_mark_rounded,
        title: l10n.assetDetailNotFoundTitle,
        subtitle: l10n.assetDetailNotFoundBody(tag),
        actionLabel: l10n.assetDetailScanAgain,
        onAction: () {
          // Dibuka dari layar scan: kembali ke sana; deep link/cold start:
          // langsung ke tab scan.
          if (context.canPop()) {
            context.pop();
          } else {
            context.go('/scan');
          }
        },
      ),
      ForbiddenFailure() => EmptyState(
        icon: Symbols.lock_rounded,
        title: l10n.assetDetailForbiddenTitle,
        subtitle: l10n.assetDetailForbiddenBody,
      ),
      NetworkFailure() => EmptyState(
        icon: Symbols.wifi_off_rounded,
        title: l10n.assetDetailErrorTitle,
        subtitle: l10n.assetDetailErrorNetworkBody,
        actionLabel: l10n.commonRetry,
        onAction: onRetry,
      ),
      _ => EmptyState(
        icon: Symbols.error_rounded,
        title: l10n.assetDetailErrorTitle,
        subtitle: l10n.assetDetailErrorGenericBody,
        actionLabel: l10n.commonRetry,
        onAction: onRetry,
      ),
    };
  }
}

/// Skeleton loading menyusun bentuk layar: blok foto, judul, pill, tiga card.
class _LoadingSkeleton extends StatelessWidget {
  const _LoadingSkeleton();

  @override
  Widget build(BuildContext context) {
    return ListView(
      padding: const EdgeInsets.fromLTRB(20, 4, 20, 24),
      children: const <Widget>[
        AppSkeleton(height: 140, borderRadius: 20),
        SizedBox(height: 12),
        AppSkeleton(height: 22, width: 240, borderRadius: 8),
        SizedBox(height: 10),
        AppSkeleton(height: 28, width: 190, borderRadius: 999),
        SizedBox(height: 14),
        AppSkeleton(height: 150, borderRadius: 18),
        SizedBox(height: 10),
        AppSkeleton(height: 150, borderRadius: 18),
        SizedBox(height: 10),
        AppSkeleton(height: 96, borderRadius: 18),
      ],
    );
  }
}

class _AssetDetailBody extends StatelessWidget {
  const _AssetDetailBody({required this.data, required this.names});

  final AssetDetailData data;

  /// Nama referensi ter-resolve; null selama resolusi berjalan/gagal.
  final AssetReferenceNames? names;

  @override
  Widget build(BuildContext context) {
    final AppLocalizations l10n = AppLocalizations.of(context);
    final AssetDto asset = data.asset;
    final String localeName = Localizations.localeOf(context).languageCode;

    final bool valueRestricted =
        data.isMasked('purchase_cost') ||
        data.isMasked('book_value') ||
        data.isMasked('accumulated_depreciation');

    return ListView(
      padding: const EdgeInsets.fromLTRB(20, 4, 20, 24),
      children: <Widget>[
        const _PhotoPlaceholder(),
        const SizedBox(height: 10),
        _Header(data: data, l10n: l10n),
        const SizedBox(height: 10),
        _SectionCard(
          title: l10n.assetDetailSectionPlacement,
          child: Column(
            children: <Widget>[
              _PlacementRow(
                icon: Symbols.account_balance_rounded,
                label: l10n.assetDetailFieldOffice,
                value: _fieldValue(data, 'office_id', names?.officeName),
              ),
              const SizedBox(height: 11),
              _PlacementRow(
                icon: Symbols.meeting_room_rounded,
                label: l10n.assetDetailFieldRoom,
                value: _fieldValue(data, 'room_id', names?.roomLabel),
              ),
              const SizedBox(height: 11),
              _PlacementRow(
                icon: Symbols.person_rounded,
                label: l10n.assetDetailFieldHolder,
                value: _fieldValue(
                  data,
                  'current_holder_employee_id',
                  names?.holderName,
                ),
              ),
            ],
          ),
        ),
        const SizedBox(height: 10),
        _SectionCard(
          title: l10n.assetDetailSectionInfo,
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: <Widget>[
              Row(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: <Widget>[
                  Expanded(
                    child: _InfoCell(
                      label: l10n.assetDetailFieldCategory,
                      value: _fieldValue(
                        data,
                        'category_id',
                        names?.categoryName,
                      ),
                    ),
                  ),
                  const SizedBox(width: 14),
                  Expanded(
                    child: _InfoCell(
                      label: l10n.assetDetailFieldBrandModel,
                      value: _brandModelValue(data),
                    ),
                  ),
                ],
              ),
              const SizedBox(height: 11),
              Row(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: <Widget>[
                  Expanded(
                    child: _InfoCell(
                      label: l10n.assetDetailFieldSerial,
                      value: _fieldValue(
                        data,
                        'serial_number',
                        asset.serialNumber,
                      ),
                    ),
                  ),
                  const SizedBox(width: 14),
                  Expanded(
                    child: _InfoCell(
                      label: l10n.assetDetailFieldPurchaseDate,
                      value: _fieldValue(
                        data,
                        'purchase_date',
                        _formatDate(asset.purchaseDate, localeName),
                      ),
                    ),
                  ),
                ],
              ),
              const SizedBox(height: 11),
              _InfoCell(
                label: l10n.assetDetailFieldVendor,
                value: _fieldValue(data, 'vendor_id', names?.vendorName),
              ),
            ],
          ),
        ),
        const SizedBox(height: 10),
        _SectionCard(
          title: l10n.assetDetailSectionValue,
          badge: valueRestricted
              ? _RestrictedBadge(label: l10n.assetDetailRestrictedBadge)
              : null,
          child: Column(
            children: <Widget>[
              _MoneyRow(
                label: l10n.assetDetailFieldPurchaseCost,
                value: _fieldValue(
                  data,
                  'purchase_cost',
                  _formatCurrency(asset.purchaseCost, localeName),
                ),
                withDivider: true,
              ),
              _MoneyRow(
                label: l10n.assetDetailFieldBookValue,
                value: _fieldValue(
                  data,
                  'book_value',
                  _formatCurrency(asset.bookValue, localeName),
                ),
                withDivider: false,
              ),
            ],
          ),
        ),
      ],
    );
  }

  /// Gabungan Brand / Model dari nama ter-resolve; dianggap dibatasi hanya
  /// bila keduanya dimask.
  _FieldValue _brandModelValue(AssetDetailData data) {
    final bool masked = data.isMasked('brand_id') && data.isMasked('model_id');
    final List<String> parts = <String>[?names?.brandName, ?names?.modelName];
    return _FieldValue(
      text: parts.isEmpty ? null : parts.join(' · '),
      masked: masked,
    );
  }

  _FieldValue _fieldValue(AssetDetailData data, String key, String? text) {
    return _FieldValue(text: text, masked: data.isMasked(key));
  }

  String? _formatDate(String? raw, String localeName) {
    if (raw == null) {
      return null;
    }
    final DateTime? date = DateTime.tryParse(raw);
    if (date == null) {
      return raw;
    }
    return DateFormat('d MMM y', localeName).format(date);
  }

  String? _formatCurrency(String? raw, String localeName) {
    if (raw == null) {
      return null;
    }
    final double? value = double.tryParse(raw);
    if (value == null) {
      return raw;
    }
    return NumberFormat.currency(
      locale: localeName,
      symbol: 'Rp ',
      decimalDigits: 0,
    ).format(value);
  }
}

/// Nilai satu field siap render: [text] null berarti em-dash; [masked] true
/// menambah ikon gembok kecil + tooltip dibatasi.
@immutable
class _FieldValue {
  const _FieldValue({required this.text, required this.masked});

  final String? text;
  final bool masked;
}

/// Placeholder foto aset: belum ada API foto di M0 (deviasi tercatat) —
/// blok bermotif dengan ikon dan keterangan, tanpa carousel dots mockup.
class _PhotoPlaceholder extends StatelessWidget {
  const _PhotoPlaceholder();

  @override
  Widget build(BuildContext context) {
    final ColorScheme scheme = Theme.of(context).colorScheme;
    final AppLocalizations l10n = AppLocalizations.of(context);

    return Container(
      height: 140,
      decoration: BoxDecoration(
        color: scheme.secondaryContainer,
        borderRadius: BorderRadius.circular(InventraDimens.radiusCardMain),
        border: Border.all(color: scheme.outlineVariant),
      ),
      child: Stack(
        children: <Widget>[
          Center(
            child: Icon(
              Symbols.inventory_2_rounded,
              size: 44,
              color: scheme.onSurfaceVariant,
            ),
          ),
          Positioned(
            left: 12,
            bottom: 10,
            child: Text(
              l10n.assetDetailPhotoPlaceholder,
              style: TextStyle(
                fontSize: 10,
                color: Theme.of(context).textTheme.labelSmall?.color,
              ),
            ),
          ),
        ],
      ),
    );
  }
}

/// Nama aset + pill tag (ikon barcode) + chip status.
class _Header extends StatelessWidget {
  const _Header({required this.data, required this.l10n});

  final AssetDetailData data;
  final AppLocalizations l10n;

  @override
  Widget build(BuildContext context) {
    final ThemeData theme = Theme.of(context);
    final ColorScheme scheme = theme.colorScheme;
    final AssetDto asset = data.asset;
    final (String, StatusChipVariant)? status = _statusPresentation(
      asset.status,
      l10n,
    );

    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: <Widget>[
        Text(
          asset.name ?? _emDash,
          style: TextStyle(
            fontSize: 20,
            fontWeight: FontWeight.w800,
            letterSpacing: 20 * InventraDimens.titleLetterSpacingEm,
            color: scheme.onSurface,
          ),
        ),
        const SizedBox(height: 5),
        Wrap(
          spacing: 10,
          runSpacing: 6,
          crossAxisAlignment: WrapCrossAlignment.center,
          children: <Widget>[
            Container(
              padding: const EdgeInsets.symmetric(horizontal: 10, vertical: 5),
              decoration: BoxDecoration(
                color: theme.cardTheme.color ?? scheme.surface,
                borderRadius: BorderRadius.circular(8),
                border: Border.all(color: scheme.outlineVariant),
              ),
              child: Row(
                mainAxisSize: MainAxisSize.min,
                children: <Widget>[
                  Icon(
                    Symbols.barcode_rounded,
                    size: 15,
                    color: scheme.onSurfaceVariant,
                  ),
                  const SizedBox(width: 6),
                  Text(
                    asset.assetTag ?? _emDash,
                    style: TextStyle(
                      fontSize: 12,
                      color: theme.textTheme.labelMedium?.color,
                    ),
                  ),
                ],
              ),
            ),
            if (status != null)
              StatusChip(label: status.$1, variant: status.$2),
          ],
        ),
      ],
    );
  }
}

/// Peta status aset openapi -> label i18n + varian warna [StatusChip].
/// Status tak dikenal dirender apa adanya dengan varian netral (klien tidak
/// menebak makna nilai baru).
(String, StatusChipVariant)? _statusPresentation(
  String? status,
  AppLocalizations l10n,
) {
  return switch (status) {
    null => null,
    'available' => (l10n.assetDetailStatusAvailable, StatusChipVariant.success),
    'assigned' => (l10n.assetDetailStatusAssigned, StatusChipVariant.info),
    'under_maintenance' => (
      l10n.assetDetailStatusUnderMaintenance,
      StatusChipVariant.warning,
    ),
    'in_transfer' => (l10n.assetDetailStatusInTransfer, StatusChipVariant.info),
    'retired' => (l10n.assetDetailStatusRetired, StatusChipVariant.neutral),
    'disposed' => (l10n.assetDetailStatusDisposed, StatusChipVariant.neutral),
    'lost' => (l10n.assetDetailStatusLost, StatusChipVariant.danger),
    final String other => (other, StatusChipVariant.neutral),
  };
}

/// Card seksi: judul uppercase kecil (+ badge opsional) lalu isi.
class _SectionCard extends StatelessWidget {
  const _SectionCard({required this.title, required this.child, this.badge});

  final String title;
  final Widget child;
  final Widget? badge;

  @override
  Widget build(BuildContext context) {
    final ThemeData theme = Theme.of(context);
    final ColorScheme scheme = theme.colorScheme;
    final Widget? badgeWidget = badge;

    return Container(
      padding: const EdgeInsets.fromLTRB(14, 12, 14, 14),
      decoration: BoxDecoration(
        color: theme.cardTheme.color ?? scheme.surface,
        borderRadius: BorderRadius.circular(18),
        border: Border.all(color: scheme.outlineVariant),
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: <Widget>[
          Row(
            mainAxisAlignment: MainAxisAlignment.spaceBetween,
            children: <Widget>[
              Text(
                title.toUpperCase(),
                style: TextStyle(
                  fontSize: 12,
                  fontWeight: FontWeight.w700,
                  letterSpacing: 12 * 0.05,
                  color: theme.textTheme.bodySmall?.color,
                ),
              ),
              ?badgeWidget,
            ],
          ),
          const SizedBox(height: 11),
          child,
        ],
      ),
    );
  }
}

/// Badge pill "Dibatasi untuk peran Anda" pada header seksi Nilai.
class _RestrictedBadge extends StatelessWidget {
  const _RestrictedBadge({required this.label});

  final String label;

  @override
  Widget build(BuildContext context) {
    final ThemeData theme = Theme.of(context);
    final ColorScheme scheme = theme.colorScheme;

    return Container(
      padding: const EdgeInsets.symmetric(horizontal: 10, vertical: 4),
      decoration: ShapeDecoration(
        color: scheme.secondaryContainer,
        shape: const StadiumBorder(),
      ),
      child: Row(
        mainAxisSize: MainAxisSize.min,
        children: <Widget>[
          Icon(
            Symbols.lock_rounded,
            size: 13,
            color: theme.textTheme.bodySmall?.color,
          ),
          const SizedBox(width: 5),
          Text(
            label,
            style: TextStyle(
              fontSize: 11,
              fontWeight: FontWeight.w600,
              color: theme.textTheme.bodySmall?.color,
            ),
          ),
        ],
      ),
    );
  }
}

/// Baris penempatan: tile ikon 36 + label kecil + nilai.
class _PlacementRow extends StatelessWidget {
  const _PlacementRow({
    required this.icon,
    required this.label,
    required this.value,
  });

  final IconData icon;
  final String label;
  final _FieldValue value;

  @override
  Widget build(BuildContext context) {
    final ThemeData theme = Theme.of(context);
    final ColorScheme scheme = theme.colorScheme;

    return Row(
      children: <Widget>[
        Container(
          width: 36,
          height: 36,
          decoration: BoxDecoration(
            color: scheme.secondaryContainer,
            borderRadius: BorderRadius.circular(11),
          ),
          child: Icon(
            icon,
            size: 19,
            color: theme.textTheme.labelMedium?.color,
          ),
        ),
        const SizedBox(width: 11),
        Expanded(
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: <Widget>[
              Text(
                label,
                style: TextStyle(
                  fontSize: 11,
                  color: theme.textTheme.labelSmall?.color,
                ),
              ),
              _ValueText(value: value, fontSize: 13.5),
            ],
          ),
        ),
      ],
    );
  }
}

/// Sel grid seksi Informasi: label kecil + nilai.
class _InfoCell extends StatelessWidget {
  const _InfoCell({required this.label, required this.value});

  final String label;
  final _FieldValue value;

  @override
  Widget build(BuildContext context) {
    final ThemeData theme = Theme.of(context);

    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: <Widget>[
        Text(
          label,
          style: TextStyle(
            fontSize: 11,
            color: theme.textTheme.labelSmall?.color,
          ),
        ),
        _ValueText(value: value, fontSize: 13),
      ],
    );
  }
}

/// Baris seksi Nilai: label kiri, nominal kanan (dibatasi -> gembok + dash).
class _MoneyRow extends StatelessWidget {
  const _MoneyRow({
    required this.label,
    required this.value,
    required this.withDivider,
  });

  final String label;
  final _FieldValue value;
  final bool withDivider;

  @override
  Widget build(BuildContext context) {
    final ThemeData theme = Theme.of(context);

    return Column(
      children: <Widget>[
        Row(
          mainAxisAlignment: MainAxisAlignment.spaceBetween,
          children: <Widget>[
            Text(
              label,
              style: TextStyle(
                fontSize: 13,
                color: theme.textTheme.bodySmall?.color,
              ),
            ),
            _ValueText(value: value, fontSize: 14.5, bold: true),
          ],
        ),
        if (withDivider) ...<Widget>[
          const SizedBox(height: 10),
          const Divider(),
          const SizedBox(height: 10),
        ],
      ],
    );
  }
}

/// Render nilai field: teks biasa; null -> em-dash; dimask field permission ->
/// gembok kecil + em-dash + tooltip i18n (penanda "dibatasi").
class _ValueText extends StatelessWidget {
  const _ValueText({
    required this.value,
    required this.fontSize,
    this.bold = false,
  });

  final _FieldValue value;
  final double fontSize;
  final bool bold;

  @override
  Widget build(BuildContext context) {
    final ThemeData theme = Theme.of(context);
    final ColorScheme scheme = theme.colorScheme;
    final AppLocalizations l10n = AppLocalizations.of(context);
    final TextStyle style = TextStyle(
      fontSize: fontSize,
      fontWeight: bold ? FontWeight.w700 : FontWeight.w600,
      color: scheme.onSurface,
    );

    if (value.masked) {
      final Color mutedColor =
          theme.textTheme.labelSmall?.color ?? scheme.onSurfaceVariant;
      return Tooltip(
        message: l10n.assetDetailRestrictedTooltip,
        child: Row(
          mainAxisSize: MainAxisSize.min,
          children: <Widget>[
            Icon(Symbols.lock_rounded, size: 15, color: mutedColor),
            const SizedBox(width: 6),
            Text(_emDash, style: style.copyWith(color: mutedColor)),
          ],
        ),
      );
    }
    return Text(value.text ?? _emDash, style: style);
  }
}
