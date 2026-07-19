import 'package:flutter/foundation.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../data/approval_repository.dart';
import '../data/request_dto.dart';
import '../data/request_list_dto.dart';

/// State satu tab filter inbox: halaman-halaman yang sudah dimuat + status
/// muat-berikutnya untuk infinite scroll sederhana (limit/offset kontrak).
@immutable
class ApprovalInboxState {
  const ApprovalInboxState({
    required this.items,
    required this.total,
    this.isLoadingMore = false,
    this.loadMoreFailed = false,
  });

  final List<RequestDto> items;

  /// Total baris di server untuk filter ini (`RequestList.total`).
  final int total;

  final bool isLoadingMore;
  final bool loadMoreFailed;

  bool get hasMore => items.length < total;

  ApprovalInboxState copyWith({
    List<RequestDto>? items,
    int? total,
    bool? isLoadingMore,
    bool? loadMoreFailed,
  }) {
    return ApprovalInboxState(
      items: items ?? this.items,
      total: total ?? this.total,
      isLoadingMore: isLoadingMore ?? this.isLoadingMore,
      loadMoreFailed: loadMoreFailed ?? this.loadMoreFailed,
    );
  }
}

/// Daftar pengajuan per filter status. autoDispose: state dibuang saat layar
/// ditutup; refresh lewat `ref.refresh(...(filter).future)` (pull-to-refresh).
/// Auto-retry Riverpod dimatikan — pengguna punya tombol "Coba lagi".
final approvalInboxProvider = AsyncNotifierProvider.autoDispose
    .family<ApprovalInboxController, ApprovalInboxState, ApprovalStatusFilter>(
      ApprovalInboxController.new,
      retry: (int retryCount, Object error) => null,
    );

class ApprovalInboxController extends AsyncNotifier<ApprovalInboxState> {
  ApprovalInboxController(this.filter);

  final ApprovalStatusFilter filter;

  static const int pageSize = 20;

  @override
  Future<ApprovalInboxState> build() async {
    final RequestListDto page = await ref
        .watch(approvalRepositoryProvider)
        .list(filter: filter, offset: 0, limit: pageSize);
    return ApprovalInboxState(items: page.data, total: page.total);
  }

  /// Memuat halaman berikutnya (offset = jumlah item termuat). Kegagalan
  /// TIDAK menjatuhkan seluruh daftar — hanya menandai [ApprovalInboxState.loadMoreFailed]
  /// supaya baris retry tampil di kaki daftar.
  Future<void> loadMore() async {
    final ApprovalInboxState? current = state.value;
    if (current == null || current.isLoadingMore || !current.hasMore) {
      return;
    }
    state = AsyncData<ApprovalInboxState>(
      current.copyWith(isLoadingMore: true, loadMoreFailed: false),
    );
    try {
      final RequestListDto page = await ref
          .read(approvalRepositoryProvider)
          .list(filter: filter, offset: current.items.length, limit: pageSize);
      state = AsyncData<ApprovalInboxState>(
        current.copyWith(
          items: List<RequestDto>.unmodifiable(<RequestDto>[
            ...current.items,
            ...page.data,
          ]),
          total: page.total,
          isLoadingMore: false,
        ),
      );
    } on Object {
      state = AsyncData<ApprovalInboxState>(
        current.copyWith(isLoadingMore: false, loadMoreFailed: true),
      );
    }
  }
}
