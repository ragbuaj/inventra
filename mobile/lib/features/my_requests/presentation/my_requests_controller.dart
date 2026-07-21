import 'package:flutter/foundation.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../approval/data/approval_repository.dart' show ApprovalStatusFilter;
import '../../approval/data/request_dto.dart';
import '../../approval/data/request_list_dto.dart';
import '../data/my_requests_repository.dart';

/// State satu tab filter "Pengajuan Saya": halaman termuat + status
/// muat-berikutnya (limit/offset). Sejajar pola ApprovalInboxState.
@immutable
class MyRequestsState {
  const MyRequestsState({
    required this.items,
    required this.total,
    this.isLoadingMore = false,
    this.loadMoreFailed = false,
  });

  final List<RequestDto> items;
  final int total;
  final bool isLoadingMore;
  final bool loadMoreFailed;

  bool get hasMore => items.length < total;

  MyRequestsState copyWith({
    List<RequestDto>? items,
    int? total,
    bool? isLoadingMore,
    bool? loadMoreFailed,
  }) {
    return MyRequestsState(
      items: items ?? this.items,
      total: total ?? this.total,
      isLoadingMore: isLoadingMore ?? this.isLoadingMore,
      loadMoreFailed: loadMoreFailed ?? this.loadMoreFailed,
    );
  }
}

/// Pengajuan milik pengguna per filter status. autoDispose; auto-retry mati.
final myRequestsProvider = AsyncNotifierProvider.autoDispose
    .family<MyRequestsController, MyRequestsState, ApprovalStatusFilter>(
      MyRequestsController.new,
      retry: (int retryCount, Object error) => null,
    );

class MyRequestsController extends AsyncNotifier<MyRequestsState> {
  MyRequestsController(this.filter);

  final ApprovalStatusFilter filter;

  static const int pageSize = 20;

  @override
  Future<MyRequestsState> build() async {
    final RequestListDto page = await ref
        .watch(myRequestsRepositoryProvider)
        .list(filter: filter, offset: 0, limit: pageSize);
    return MyRequestsState(items: page.data, total: page.total);
  }

  Future<void> loadMore() async {
    final MyRequestsState? current = state.value;
    if (current == null || current.isLoadingMore || !current.hasMore) {
      return;
    }
    state = AsyncData<MyRequestsState>(
      current.copyWith(isLoadingMore: true, loadMoreFailed: false),
    );
    try {
      final RequestListDto page = await ref
          .read(myRequestsRepositoryProvider)
          .list(filter: filter, offset: current.items.length, limit: pageSize);
      state = AsyncData<MyRequestsState>(
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
      state = AsyncData<MyRequestsState>(
        current.copyWith(isLoadingMore: false, loadMoreFailed: true),
      );
    }
  }

  /// Membatalkan pengajuan `pending` sendiri lalu memuat ulang daftar.
  /// Melempar AppFailure bila server menolak (bukan milik / bukan pending /
  /// offline) — pemanggil menampilkan SnackBar.
  Future<void> cancel(String id) async {
    await ref.read(myRequestsRepositoryProvider).cancel(id);
    ref.invalidateSelf();
    await future;
  }
}
