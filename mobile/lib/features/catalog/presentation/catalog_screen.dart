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
import 'catalog_controller.dart';

/// Katalog Aset (1:1 mockup "Inventra Mobile - Katalog Aset"): search bar,
/// daftar kartu aset (foto, nama, kode, chip status, kantor), pull-to-refresh,
/// infinite scroll limit/offset, empty/loading/error state. Read-only; data
/// dari `GET /assets` (data-scope + field-permission masking backend).
///
/// Filter chips (Kategori/Status/Kantor) menyusul di increment berikutnya.
class CatalogScreen extends ConsumerStatefulWidget {
  const CatalogScreen({super.key});

  @override
  ConsumerState<CatalogScreen> createState() => _CatalogScreenState();
}

class _CatalogScreenState extends ConsumerState<CatalogScreen> {
  final TextEditingController _searchController = TextEditingController();
  Timer? _debounce;

  /// Istilah pencarian aktif (null/kosong = seluruh aset). Menjadi argumen
  /// family provider; berubah setelah debounce agar tidak memukul backend per
  /// ketukan tombol.
  String? _search;

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
      if (next != _search) {
        setState(() => _search = next);
      }
    });
  }

  Future<void> _refresh() async {
    ref.invalidate(catalogProvider(_search));
    try {
      await ref.read(catalogProvider(_search).future);
    } on Object {
      // Kegagalan refresh sudah tercermin sebagai state error daftar.
    }
  }

  @override
  Widget build(BuildContext context) {
    final AppLocalizations l10n = AppLocalizations.of(context);
    final AsyncValue<CatalogState> state = ref.watch(catalogProvider(_search));

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
            Expanded(
              child: state.when(
                data: (CatalogState data) => _CatalogList(
                  state: data,
                  hasSearch: _search != null,
                  onRefresh: _refresh,
                  onLoadMore: () =>
                      ref.read(catalogProvider(_search).notifier).loadMore(),
                  onClearSearch: () {
                    _searchController.clear();
                    _debounce?.cancel();
                    setState(() => _search = null);
                  },
                ),
                loading: () => const _LoadingSkeleton(),
                error: (Object error, StackTrace stackTrace) => _ErrorState(
                  failure: error,
                  onRetry: () => ref.invalidate(catalogProvider(_search)),
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
      padding: const EdgeInsets.fromLTRB(20, 6, 20, 12),
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

/// Daftar kartu + pull-to-refresh + infinite scroll; empty state membedakan
/// "belum ada aset" vs "pencarian tak cocok" (mockup punya tombol Reset).
class _CatalogList extends StatelessWidget {
  const _CatalogList({
    required this.state,
    required this.hasSearch,
    required this.onRefresh,
    required this.onLoadMore,
    required this.onClearSearch,
  });

  final CatalogState state;
  final bool hasSearch;
  final Future<void> Function() onRefresh;
  final VoidCallback onLoadMore;
  final VoidCallback onClearSearch;

  @override
  Widget build(BuildContext context) {
    final AppLocalizations l10n = AppLocalizations.of(context);

    if (state.items.isEmpty) {
      if (hasSearch) {
        return EmptyState(
          icon: Symbols.search_off_rounded,
          title: l10n.catalogEmptySearchTitle,
          subtitle: l10n.catalogEmptySearchBody,
          actionLabel: l10n.catalogResetFilter,
          onAction: onClearSearch,
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
    final (String, StatusChipVariant)? status = _assetStatusPresentation(
      asset.status,
      l10n,
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

/// Peta status aset openapi ke label i18n + varian [StatusChip] (memakai kunci
/// i18n yang sama dengan Detail Aset). Status tak dikenal dirender apa adanya,
/// varian netral.
(String, StatusChipVariant)? _assetStatusPresentation(
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
