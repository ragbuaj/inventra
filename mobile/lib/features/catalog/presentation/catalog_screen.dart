import 'dart:async';

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';
import 'package:material_symbols_icons/symbols.dart';

import '../../../core/api/app_failure.dart';
import '../../../core/i18n/gen/app_localizations.dart';
import '../../../core/masterdata/reference_lookup_repository.dart';
import '../../../core/widgets/app_skeleton.dart';
import '../../../core/widgets/empty_state.dart';
import '../../../core/widgets/status_chip.dart';
import '../../asset_detail/data/asset_dto.dart';
import '../data/filter_options_repository.dart';
import 'catalog_controller.dart';
import 'catalog_query.dart';

/// Status aset yang bisa dipilih di filter (urut sesuai daftar backend).
const List<String> _assetStatuses = <String>[
  'available',
  'assigned',
  'under_maintenance',
  'in_transfer',
  'retired',
  'disposed',
  'lost',
];

/// Katalog Aset (1:1 mockup "Inventra Mobile - Katalog Aset"): search bar,
/// baris filter (Kategori/Status/Kantor), daftar kartu aset, pull-to-refresh,
/// infinite scroll limit/offset, empty/loading/error state. Read-only; data
/// dari `GET /assets` (data-scope + field-permission masking backend).
class CatalogScreen extends ConsumerStatefulWidget {
  const CatalogScreen({super.key});

  @override
  ConsumerState<CatalogScreen> createState() => _CatalogScreenState();
}

class _CatalogScreenState extends ConsumerState<CatalogScreen> {
  final TextEditingController _searchController = TextEditingController();
  Timer? _debounce;

  /// Kueri aktif (pencarian + filter). Menjadi argumen family provider.
  CatalogQuery _query = const CatalogQuery();

  /// Opsi terpilih disimpan untuk menampilkan NAMA pada chip (kueri hanya
  /// menyimpan id).
  FilterOption? _category;
  FilterOption? _office;

  @override
  void dispose() {
    _debounce?.cancel();
    _searchController.dispose();
    super.dispose();
  }

  void _onSearchChanged(String value) {
    _debounce?.cancel();
    _debounce = Timer(const Duration(milliseconds: 300), () {
      final String trimmed = value.trim();
      final String? next = trimmed.isEmpty ? null : trimmed;
      if (next != _query.search) {
        setState(() => _query = _query.copyWith(search: next));
      }
    });
  }

  void _resetAll() {
    _searchController.clear();
    _debounce?.cancel();
    setState(() {
      _query = const CatalogQuery();
      _category = null;
      _office = null;
    });
  }

  Future<void> _refresh() async {
    ref.invalidate(catalogProvider(_query));
    try {
      await ref.read(catalogProvider(_query).future);
    } on Object {
      // Kegagalan refresh sudah tercermin sebagai state error daftar.
    }
  }

  Future<void> _pickStatus() async {
    final AppLocalizations l10n = AppLocalizations.of(context);
    final String? picked = await showModalBottomSheet<String>(
      context: context,
      showDragHandle: true,
      builder: (BuildContext context) => _StatusPickerSheet(
        title: l10n.catalogPickerStatusTitle,
        selected: _query.status,
      ),
    );
    // Sheet mengembalikan '' untuk "Semua" (bedakan dari batal = null).
    if (picked != null) {
      setState(
        () => _query = _query.copyWith(status: picked.isEmpty ? null : picked),
      );
    }
  }

  Future<void> _pickCategory() async {
    final AppLocalizations l10n = AppLocalizations.of(context);
    final _OptionResult? result = await showModalBottomSheet<_OptionResult>(
      context: context,
      showDragHandle: true,
      builder: (BuildContext context) => _OptionPickerSheet(
        title: l10n.catalogPickerCategoryTitle,
        optionsProvider: catalogCategoryOptionsProvider,
        selectedId: _query.categoryId,
      ),
    );
    if (result != null) {
      setState(() {
        _category = result.option;
        _query = _query.copyWith(categoryId: result.option?.id);
      });
    }
  }

  Future<void> _pickOffice() async {
    final AppLocalizations l10n = AppLocalizations.of(context);
    final _OptionResult? result = await showModalBottomSheet<_OptionResult>(
      context: context,
      showDragHandle: true,
      builder: (BuildContext context) => _OptionPickerSheet(
        title: l10n.catalogPickerOfficeTitle,
        optionsProvider: catalogOfficeOptionsProvider,
        selectedId: _query.officeId,
      ),
    );
    if (result != null) {
      setState(() {
        _office = result.option;
        _query = _query.copyWith(officeId: result.option?.id);
      });
    }
  }

  @override
  Widget build(BuildContext context) {
    final AppLocalizations l10n = AppLocalizations.of(context);
    final AsyncValue<CatalogState> state = ref.watch(catalogProvider(_query));

    return Scaffold(
      appBar: AppBar(title: Text(l10n.catalogTitle)),
      body: SafeArea(
        child: Column(
          children: <Widget>[
            _SearchField(
              controller: _searchController,
              hintText: l10n.catalogSearchHint,
              onChanged: _onSearchChanged,
            ),
            _FilterRow(
              categoryLabel: _category?.name ?? l10n.catalogFilterCategory,
              categoryActive: _query.categoryId != null,
              onCategory: _pickCategory,
              statusLabel: _query.status == null
                  ? l10n.catalogFilterStatus
                  : assetStatusLabel(l10n, _query.status!),
              statusActive: _query.status != null,
              onStatus: _pickStatus,
              officeLabel: _office?.name ?? l10n.catalogFilterOffice,
              officeActive: _query.officeId != null,
              onOffice: _pickOffice,
            ),
            Expanded(
              child: state.when(
                data: (CatalogState data) => _CatalogList(
                  state: data,
                  hasFilters: _query.hasFilters,
                  onRefresh: _refresh,
                  onLoadMore: () =>
                      ref.read(catalogProvider(_query).notifier).loadMore(),
                  onReset: _resetAll,
                ),
                loading: () => const _LoadingSkeleton(),
                error: (Object error, StackTrace stackTrace) => _ErrorState(
                  failure: error,
                  onRetry: () => ref.invalidate(catalogProvider(_query)),
                ),
              ),
            ),
          ],
        ),
      ),
    );
  }
}

/// Kolom pencarian di bawah AppBar (mockup: field "Cari aset").
class _SearchField extends StatelessWidget {
  const _SearchField({
    required this.controller,
    required this.hintText,
    required this.onChanged,
  });

  final TextEditingController controller;
  final String hintText;
  final ValueChanged<String> onChanged;

  @override
  Widget build(BuildContext context) {
    return Padding(
      padding: const EdgeInsets.fromLTRB(20, 6, 20, 8),
      child: TextField(
        controller: controller,
        onChanged: onChanged,
        textInputAction: TextInputAction.search,
        decoration: InputDecoration(
          hintText: hintText,
          prefixIcon: const Icon(Symbols.search_rounded, size: 20),
          isDense: true,
        ),
      ),
    );
  }
}

/// Baris tiga chip filter dropdown: Kategori, Status, Kantor (mockup).
class _FilterRow extends StatelessWidget {
  const _FilterRow({
    required this.categoryLabel,
    required this.categoryActive,
    required this.onCategory,
    required this.statusLabel,
    required this.statusActive,
    required this.onStatus,
    required this.officeLabel,
    required this.officeActive,
    required this.onOffice,
  });

  final String categoryLabel;
  final bool categoryActive;
  final VoidCallback onCategory;
  final String statusLabel;
  final bool statusActive;
  final VoidCallback onStatus;
  final String officeLabel;
  final bool officeActive;
  final VoidCallback onOffice;

  @override
  Widget build(BuildContext context) {
    return SingleChildScrollView(
      scrollDirection: Axis.horizontal,
      padding: const EdgeInsets.fromLTRB(20, 0, 20, 12),
      child: Row(
        children: <Widget>[
          _FilterChip(
            label: categoryLabel,
            active: categoryActive,
            onTap: onCategory,
          ),
          const SizedBox(width: 8),
          _FilterChip(label: statusLabel, active: statusActive, onTap: onStatus),
          const SizedBox(width: 8),
          _FilterChip(label: officeLabel, active: officeActive, onTap: onOffice),
        ],
      ),
    );
  }
}

/// Chip filter gaya dropdown: pill dengan caret; aktif = terisi primary.
class _FilterChip extends StatelessWidget {
  const _FilterChip({
    required this.label,
    required this.active,
    required this.onTap,
  });

  final String label;
  final bool active;
  final VoidCallback onTap;

  @override
  Widget build(BuildContext context) {
    final ThemeData theme = Theme.of(context);
    final ColorScheme scheme = theme.colorScheme;

    return Semantics(
      button: true,
      selected: active,
      child: Material(
        color: active ? scheme.primary : theme.cardTheme.color ?? scheme.surface,
        shape: StadiumBorder(
          side: active
              ? BorderSide.none
              : BorderSide(color: scheme.outlineVariant),
        ),
        clipBehavior: Clip.antiAlias,
        child: InkWell(
          onTap: onTap,
          child: Padding(
            padding: const EdgeInsets.fromLTRB(14, 8, 10, 8),
            child: Row(
              mainAxisSize: MainAxisSize.min,
              children: <Widget>[
                Text(
                  label,
                  style: TextStyle(
                    fontSize: 12.5,
                    fontWeight: active ? FontWeight.w700 : FontWeight.w600,
                    color: active
                        ? scheme.onPrimary
                        : theme.textTheme.labelMedium?.color,
                  ),
                ),
                const SizedBox(width: 2),
                Icon(
                  Symbols.arrow_drop_down_rounded,
                  size: 18,
                  color: active
                      ? scheme.onPrimary
                      : theme.textTheme.labelMedium?.color,
                ),
              ],
            ),
          ),
        ),
      ),
    );
  }
}

/// Hasil picker opsi: [option] null berarti "Semua" (filter dihapus).
class _OptionResult {
  const _OptionResult(this.option);

  final FilterOption? option;
}

/// Sheet pilih Status: daftar tetap "Semua" + status aset. Mengembalikan kode
/// status ('' untuk Semua); batal (dismiss) mengembalikan null.
class _StatusPickerSheet extends StatelessWidget {
  const _StatusPickerSheet({required this.title, required this.selected});

  final String title;
  final String? selected;

  @override
  Widget build(BuildContext context) {
    final AppLocalizations l10n = AppLocalizations.of(context);

    return SafeArea(
      child: ListView(
        shrinkWrap: true,
        children: <Widget>[
          _SheetTitle(title),
          _SheetOption(
            label: l10n.catalogFilterAll,
            selected: selected == null,
            onTap: () => Navigator.of(context).pop<String>(''),
          ),
          for (final String status in _assetStatuses)
            _SheetOption(
              label: assetStatusLabel(l10n, status),
              selected: selected == status,
              onTap: () => Navigator.of(context).pop<String>(status),
            ),
        ],
      ),
    );
  }
}

/// Sheet pilih opsi async (Kategori/Kantor): loading, error, "Tidak ada data",
/// atau daftar "Semua" + opsi. Mengembalikan [_OptionResult]; batal = null.
class _OptionPickerSheet extends ConsumerWidget {
  const _OptionPickerSheet({
    required this.title,
    required this.optionsProvider,
    required this.selectedId,
  });

  final String title;
  final FutureProvider<List<FilterOption>> optionsProvider;
  final String? selectedId;

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final AppLocalizations l10n = AppLocalizations.of(context);
    final AsyncValue<List<FilterOption>> options = ref.watch(optionsProvider);

    return SafeArea(
      child: options.when(
        loading: () => const Padding(
          padding: EdgeInsets.symmetric(vertical: 40),
          child: Center(child: CircularProgressIndicator()),
        ),
        error: (Object error, StackTrace stackTrace) => Padding(
          padding: const EdgeInsets.fromLTRB(20, 12, 20, 32),
          child: Column(
            mainAxisSize: MainAxisSize.min,
            children: <Widget>[
              _SheetTitle(title),
              const SizedBox(height: 12),
              Text(
                l10n.catalogFilterOptionsError,
                style: TextStyle(color: Theme.of(context).colorScheme.error),
              ),
              TextButton(
                onPressed: () => ref.invalidate(optionsProvider),
                child: Text(l10n.commonRetry),
              ),
            ],
          ),
        ),
        data: (List<FilterOption> list) {
          if (list.isEmpty) {
            return Padding(
              padding: const EdgeInsets.fromLTRB(20, 12, 20, 40),
              child: Column(
                mainAxisSize: MainAxisSize.min,
                children: <Widget>[
                  _SheetTitle(title),
                  const SizedBox(height: 16),
                  Text(
                    l10n.catalogFilterNoOptions,
                    style: TextStyle(
                      color: Theme.of(context).colorScheme.onSurfaceVariant,
                    ),
                  ),
                ],
              ),
            );
          }
          return ListView(
            shrinkWrap: true,
            children: <Widget>[
              _SheetTitle(title),
              _SheetOption(
                label: l10n.catalogFilterAll,
                selected: selectedId == null,
                onTap: () =>
                    Navigator.of(context).pop<_OptionResult>(
                      const _OptionResult(null),
                    ),
              ),
              for (final FilterOption option in list)
                _SheetOption(
                  label: option.name,
                  selected: option.id == selectedId,
                  onTap: () => Navigator.of(
                    context,
                  ).pop<_OptionResult>(_OptionResult(option)),
                ),
            ],
          );
        },
      ),
    );
  }
}

class _SheetTitle extends StatelessWidget {
  const _SheetTitle(this.title);

  final String title;

  @override
  Widget build(BuildContext context) {
    return Padding(
      padding: const EdgeInsets.fromLTRB(20, 0, 20, 8),
      child: Text(
        title,
        style: const TextStyle(fontSize: 15, fontWeight: FontWeight.w700),
      ),
    );
  }
}

class _SheetOption extends StatelessWidget {
  const _SheetOption({
    required this.label,
    required this.selected,
    required this.onTap,
  });

  final String label;
  final bool selected;
  final VoidCallback onTap;

  @override
  Widget build(BuildContext context) {
    final ColorScheme scheme = Theme.of(context).colorScheme;
    return ListTile(
      title: Text(label),
      trailing: selected
          ? Icon(Symbols.check_rounded, color: scheme.primary)
          : null,
      onTap: onTap,
    );
  }
}

/// Daftar kartu + pull-to-refresh + infinite scroll; empty state membedakan
/// "belum ada aset" vs "filter/pencarian tak cocok" (dengan tombol Reset).
class _CatalogList extends StatelessWidget {
  const _CatalogList({
    required this.state,
    required this.hasFilters,
    required this.onRefresh,
    required this.onLoadMore,
    required this.onReset,
  });

  final CatalogState state;
  final bool hasFilters;
  final Future<void> Function() onRefresh;
  final VoidCallback onLoadMore;
  final VoidCallback onReset;

  @override
  Widget build(BuildContext context) {
    final AppLocalizations l10n = AppLocalizations.of(context);

    if (state.items.isEmpty) {
      if (hasFilters) {
        return EmptyState(
          icon: Symbols.search_off_rounded,
          title: l10n.catalogEmptySearchTitle,
          subtitle: l10n.catalogEmptySearchBody,
          actionLabel: l10n.catalogResetFilter,
          onAction: onReset,
        );
      }
      return EmptyState(
        icon: Symbols.inventory_2_rounded,
        title: l10n.catalogEmptyTitle,
        subtitle: l10n.catalogEmptyBody,
      );
    }

    final bool showFooter =
        state.isLoadingMore || state.loadMoreFailed || state.hasMore;

    return NotificationListener<ScrollNotification>(
      onNotification: (ScrollNotification notification) {
        if (notification.metrics.axis == Axis.vertical &&
            notification.metrics.pixels >=
                notification.metrics.maxScrollExtent - 320) {
          onLoadMore();
        }
        return false;
      },
      child: RefreshIndicator(
        onRefresh: onRefresh,
        child: ListView.separated(
          physics: const AlwaysScrollableScrollPhysics(),
          padding: const EdgeInsets.fromLTRB(20, 0, 20, 24),
          itemCount: state.items.length + (showFooter ? 1 : 0),
          separatorBuilder: (BuildContext context, int index) =>
              const SizedBox(height: 10),
          itemBuilder: (BuildContext context, int index) {
            if (index == state.items.length) {
              return _ListFooter(
                isLoading: state.isLoadingMore,
                failed: state.loadMoreFailed,
                onRetry: onLoadMore,
              );
            }
            return _AssetCard(asset: state.items[index]);
          },
        ),
      ),
    );
  }
}

/// Kartu aset katalog: thumbnail placeholder, nama, kode (ikon barcode), chip
/// status, dan nama kantor (di-resolve non-fatal via reference lookup). Tap
/// membuka Detail Aset lewat tag; kartu tanpa tag (dimask) tidak dapat ditekan.
class _AssetCard extends ConsumerWidget {
  const _AssetCard({required this.asset});

  final AssetDto asset;

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final ThemeData theme = Theme.of(context);
    final ColorScheme scheme = theme.colorScheme;
    final AppLocalizations l10n = AppLocalizations.of(context);
    final (String, StatusChipVariant)? status = asset.status == null
        ? null
        : (
            assetStatusLabel(l10n, asset.status!),
            assetStatusVariant(asset.status!),
          );
    final String? tag = asset.assetTag;

    return Material(
      color: theme.cardTheme.color ?? scheme.surface,
      shape: RoundedRectangleBorder(
        borderRadius: BorderRadius.circular(18),
        side: BorderSide(color: scheme.outlineVariant),
      ),
      clipBehavior: Clip.antiAlias,
      child: InkWell(
        onTap: tag == null || tag.isEmpty
            ? null
            : () => context.push('/assets/${Uri.encodeComponent(tag)}'),
        child: Padding(
          padding: const EdgeInsets.all(14),
          child: Row(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: <Widget>[
              Container(
                width: 48,
                height: 48,
                decoration: BoxDecoration(
                  color: scheme.surfaceContainerHighest,
                  borderRadius: BorderRadius.circular(12),
                ),
                child: Icon(
                  Symbols.inventory_2_rounded,
                  size: 22,
                  color: scheme.onSurfaceVariant,
                ),
              ),
              const SizedBox(width: 12),
              Expanded(
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: <Widget>[
                    Text(
                      asset.name ?? l10n.catalogUnnamedAsset,
                      maxLines: 2,
                      overflow: TextOverflow.ellipsis,
                      style: TextStyle(
                        fontSize: 14.5,
                        fontWeight: FontWeight.w700,
                        color: scheme.onSurface,
                      ),
                    ),
                    if (tag != null && tag.isNotEmpty) ...<Widget>[
                      const SizedBox(height: 4),
                      Row(
                        children: <Widget>[
                          Icon(
                            Symbols.barcode_rounded,
                            size: 14,
                            color: theme.textTheme.labelSmall?.color,
                          ),
                          const SizedBox(width: 4),
                          Flexible(
                            child: Text(
                              tag,
                              maxLines: 1,
                              overflow: TextOverflow.ellipsis,
                              style: TextStyle(
                                fontSize: 12,
                                color: theme.textTheme.bodySmall?.color,
                              ),
                            ),
                          ),
                        ],
                      ),
                    ],
                    const SizedBox(height: 8),
                    Row(
                      children: <Widget>[
                        if (status != null)
                          StatusChip(label: status.$1, variant: status.$2),
                        if (asset.officeId != null) ...<Widget>[
                          const SizedBox(width: 8),
                          Flexible(child: _OfficeLine(officeId: asset.officeId!)),
                        ],
                      ],
                    ),
                  ],
                ),
              ),
            ],
          ),
        ),
      ),
    );
  }
}

/// Nama kantor aset, di-resolve non-fatal (offline/403/404 -> em-dash).
class _OfficeLine extends ConsumerWidget {
  const _OfficeLine({required this.officeId});

  final String officeId;

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final ThemeData theme = Theme.of(context);
    return FutureBuilder<String?>(
      future: ref.read(referenceLookupRepositoryProvider).officeName(officeId),
      builder: (BuildContext context, AsyncSnapshot<String?> snapshot) {
        final String label = snapshot.data ?? '—';
        return Row(
          mainAxisSize: MainAxisSize.min,
          children: <Widget>[
            Icon(
              Symbols.business_rounded,
              size: 13,
              color: theme.textTheme.labelSmall?.color,
            ),
            const SizedBox(width: 4),
            Flexible(
              child: Text(
                label,
                maxLines: 1,
                overflow: TextOverflow.ellipsis,
                style: TextStyle(
                  fontSize: 11.5,
                  color: theme.textTheme.labelSmall?.color,
                ),
              ),
            ),
          ],
        );
      },
    );
  }
}

/// Label i18n status aset openapi (memakai kunci yang sama dengan Detail Aset).
/// Status tak dikenal dirender apa adanya.
String assetStatusLabel(AppLocalizations l10n, String status) {
  return switch (status) {
    'available' => l10n.assetDetailStatusAvailable,
    'assigned' => l10n.assetDetailStatusAssigned,
    'under_maintenance' => l10n.assetDetailStatusUnderMaintenance,
    'in_transfer' => l10n.assetDetailStatusInTransfer,
    'retired' => l10n.assetDetailStatusRetired,
    'disposed' => l10n.assetDetailStatusDisposed,
    'lost' => l10n.assetDetailStatusLost,
    final String other => other,
  };
}

/// Varian warna [StatusChip] untuk status aset (paritas Detail Aset).
StatusChipVariant assetStatusVariant(String status) {
  return switch (status) {
    'available' => StatusChipVariant.success,
    'assigned' => StatusChipVariant.info,
    'under_maintenance' => StatusChipVariant.warning,
    'in_transfer' => StatusChipVariant.info,
    'lost' => StatusChipVariant.danger,
    _ => StatusChipVariant.neutral,
  };
}

/// Kaki daftar: spinner saat memuat halaman berikutnya; baris retry bila gagal.
class _ListFooter extends StatelessWidget {
  const _ListFooter({
    required this.isLoading,
    required this.failed,
    required this.onRetry,
  });

  final bool isLoading;
  final bool failed;
  final VoidCallback onRetry;

  @override
  Widget build(BuildContext context) {
    final AppLocalizations l10n = AppLocalizations.of(context);

    if (isLoading) {
      return const Padding(
        padding: EdgeInsets.symmetric(vertical: 14),
        child: Center(
          child: SizedBox(
            width: 22,
            height: 22,
            child: CircularProgressIndicator(strokeWidth: 2.5),
          ),
        ),
      );
    }
    if (failed) {
      return Padding(
        padding: const EdgeInsets.symmetric(vertical: 4),
        child: Row(
          mainAxisAlignment: MainAxisAlignment.center,
          children: <Widget>[
            Text(
              l10n.catalogLoadMoreFailed,
              style: TextStyle(
                fontSize: 12,
                color: Theme.of(context).colorScheme.onSurfaceVariant,
              ),
            ),
            TextButton(onPressed: onRetry, child: Text(l10n.commonRetry)),
          ],
        ),
      );
    }
    return const SizedBox(height: 4);
  }
}

/// Skeleton loading: empat kerangka kartu (mockup state loading).
class _LoadingSkeleton extends StatelessWidget {
  const _LoadingSkeleton();

  @override
  Widget build(BuildContext context) {
    final ThemeData theme = Theme.of(context);
    final ColorScheme scheme = theme.colorScheme;

    Widget card() => Container(
      padding: const EdgeInsets.all(14),
      decoration: BoxDecoration(
        color: theme.cardTheme.color ?? scheme.surface,
        borderRadius: BorderRadius.circular(18),
        border: Border.all(color: scheme.outlineVariant),
      ),
      child: const Row(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: <Widget>[
          AppSkeleton(height: 48, width: 48, borderRadius: 12),
          SizedBox(width: 12),
          Expanded(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: <Widget>[
                AppSkeleton(height: 14, width: 200, borderRadius: 7),
                SizedBox(height: 9),
                AppSkeleton(height: 11, width: 130, borderRadius: 6),
                SizedBox(height: 11),
                AppSkeleton(height: 24, width: 90, borderRadius: 999),
              ],
            ),
          ),
        ],
      ),
    );

    return ListView(
      padding: const EdgeInsets.fromLTRB(20, 0, 20, 24),
      children: <Widget>[
        for (int i = 0; i < 4; i++) ...<Widget>[
          if (i > 0) const SizedBox(height: 10),
          card(),
        ],
      ],
    );
  }
}

/// Cabang error daftar: offline, 403 (tanpa akses), dan generik.
class _ErrorState extends StatelessWidget {
  const _ErrorState({required this.failure, required this.onRetry});

  final Object failure;
  final VoidCallback onRetry;

  @override
  Widget build(BuildContext context) {
    final AppLocalizations l10n = AppLocalizations.of(context);

    return switch (failure) {
      NetworkFailure() => EmptyState(
        icon: Symbols.wifi_off_rounded,
        title: l10n.catalogErrorTitle,
        subtitle: l10n.catalogErrorNetworkBody,
        actionLabel: l10n.commonRetry,
        onAction: onRetry,
      ),
      ForbiddenFailure() => EmptyState(
        icon: Symbols.lock_rounded,
        title: l10n.catalogForbiddenTitle,
        subtitle: l10n.catalogForbiddenBody,
      ),
      _ => EmptyState(
        icon: Symbols.error_rounded,
        title: l10n.catalogErrorTitle,
        subtitle: l10n.catalogErrorGenericBody,
        actionLabel: l10n.commonRetry,
        onAction: onRetry,
      ),
    };
  }
}
