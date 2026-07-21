import 'package:flutter/foundation.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../asset_detail/data/asset_dto.dart';
import '../data/asset_list_dto.dart';
import '../data/catalog_repository.dart';
import 'catalog_query.dart';

/// State satu kueri katalog: aset termuat + status muat-berikutnya untuk
/// infinite scroll (limit/offset kontrak `GET /assets`). Sejajar pola
/// ApprovalInboxState.
@immutable
class CatalogState {
  const CatalogState({
    required this.items,
    required this.total,
    this.isLoadingMore = false,
    this.loadMoreFailed = false,
  });

  final List<AssetDto> items;

  /// Total baris di server untuk kueri ini (`AssetList.total`).
  final int total;

  final bool isLoadingMore;
  final bool loadMoreFailed;

  bool get hasMore => items.length < total;

  CatalogState copyWith({
    List<AssetDto>? items,
    int? total,
    bool? isLoadingMore,
    bool? loadMoreFailed,
  }) {
    return CatalogState(
      items: items ?? this.items,
      total: total ?? this.total,
      isLoadingMore: isLoadingMore ?? this.isLoadingMore,
      loadMoreFailed: loadMoreFailed ?? this.loadMoreFailed,
    );
  }
}

/// Daftar aset katalog per [CatalogQuery] (pencarian + filter). autoDispose:
/// state dibuang saat layar tutup; refresh lewat `ref.refresh(...(query).future)`.
/// Auto-retry Riverpod dimatikan — pengguna punya tombol "Coba lagi".
final catalogProvider = AsyncNotifierProvider.autoDispose
    .family<CatalogController, CatalogState, CatalogQuery>(
      CatalogController.new,
      retry: (int retryCount, Object error) => null,
    );

class CatalogController extends AsyncNotifier<CatalogState> {
  CatalogController(this.query);

  final CatalogQuery query;

  static const int pageSize = 20;

  @override
  Future<CatalogState> build() async {
    final AssetListDto page = await ref.watch(catalogRepositoryProvider).list(
      search: query.search,
      categoryId: query.categoryId,
      status: query.status,
      officeId: query.officeId,
      offset: 0,
      limit: pageSize,
    );
    return CatalogState(items: page.data, total: page.total);
  }

  /// Memuat halaman berikutnya (offset = jumlah item termuat). Kegagalan TIDAK
  /// menjatuhkan seluruh daftar — hanya menandai [CatalogState.loadMoreFailed]
  /// supaya baris retry tampil di kaki daftar.
  Future<void> loadMore() async {
    final CatalogState? current = state.value;
    if (current == null || current.isLoadingMore || !current.hasMore) {
      return;
    }
    state = AsyncData<CatalogState>(
      current.copyWith(isLoadingMore: true, loadMoreFailed: false),
    );
    try {
      final AssetListDto page = await ref.read(catalogRepositoryProvider).list(
        search: query.search,
        categoryId: query.categoryId,
        status: query.status,
        officeId: query.officeId,
        offset: current.items.length,
        limit: pageSize,
      );
      state = AsyncData<CatalogState>(
        current.copyWith(
          items: List<AssetDto>.unmodifiable(<AssetDto>[
            ...current.items,
            ...page.data,
          ]),
          total: page.total,
          isLoadingMore: false,
        ),
      );
    } on Object {
      state = AsyncData<CatalogState>(
        current.copyWith(isLoadingMore: false, loadMoreFailed: true),
      );
    }
  }
}
