import 'package:flutter_test/flutter_test.dart';
import 'package:inventra_mobile/core/api/app_failure.dart';
import 'package:inventra_mobile/features/approval/data/approval_repository.dart'
    show ApprovalStatusFilter;
import 'package:inventra_mobile/features/approval/data/request_dto.dart';
import 'package:inventra_mobile/features/approval/data/request_list_dto.dart';
import 'package:inventra_mobile/features/my_requests/data/my_requests_repository.dart';
import 'package:inventra_mobile/features/my_requests/presentation/my_requests_controller.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:mocktail/mocktail.dart';

class _MockMyRequestsRepository extends Mock implements MyRequestsRepository {}

final DateTime _now = DateTime.utc(2026, 7, 20, 11);

RequestDto _request(String id) => RequestDto(
  id: id,
  type: 'assignment',
  status: 'pending',
  currentStep: 1,
  reason: 'Pengajuan $id',
  requestedById: 'user-me',
  createdAt: _now,
);

RequestListDto _page(List<RequestDto> items, {required int total, int offset = 0}) =>
    RequestListDto(data: items, total: total, limit: 20, offset: offset);

void main() {
  late _MockMyRequestsRepository repository;
  const ApprovalStatusFilter filter = ApprovalStatusFilter.pending;

  setUp(() {
    repository = _MockMyRequestsRepository();
  });

  ProviderContainer makeContainer() {
    final ProviderContainer container = ProviderContainer.test(
      overrides: [myRequestsRepositoryProvider.overrideWithValue(repository)],
    );
    return container;
  }

  test('build: memuat halaman pertama; hasMore true bila total > item', () async {
    when(
      () => repository.list(filter: filter, offset: 0, limit: 20),
    ).thenAnswer((_) async => _page(<RequestDto>[_request('r1')], total: 2));

    final ProviderContainer container = makeContainer();
    addTearDown(container.dispose);

    final MyRequestsState state =
        await container.read(myRequestsProvider(filter).future);
    expect(state.items, hasLength(1));
    expect(state.total, 2);
    expect(state.hasMore, isTrue);
  });

  test('loadMore: menambah halaman kedua (offset = jumlah item)', () async {
    when(
      () => repository.list(filter: filter, offset: 0, limit: 20),
    ).thenAnswer((_) async => _page(<RequestDto>[_request('r1')], total: 2));
    when(
      () => repository.list(filter: filter, offset: 1, limit: 20),
    ).thenAnswer(
      (_) async => _page(<RequestDto>[_request('r2')], total: 2, offset: 1),
    );

    final ProviderContainer container = makeContainer();
    addTearDown(container.dispose);
    await container.read(myRequestsProvider(filter).future);

    await container.read(myRequestsProvider(filter).notifier).loadMore();

    final MyRequestsState after = container.read(myRequestsProvider(filter)).value!;
    expect(after.items.map((RequestDto r) => r.id), <String>['r1', 'r2']);
    expect(after.isLoadingMore, isFalse);
    expect(after.loadMoreFailed, isFalse);
    verify(() => repository.list(filter: filter, offset: 1, limit: 20)).called(1);
  });

  test('loadMore gagal: loadMoreFailed true, item lama dipertahankan', () async {
    when(
      () => repository.list(filter: filter, offset: 0, limit: 20),
    ).thenAnswer((_) async => _page(<RequestDto>[_request('r1')], total: 2));
    when(
      () => repository.list(filter: filter, offset: 1, limit: 20),
    ).thenThrow(const NetworkFailure());

    final ProviderContainer container = makeContainer();
    addTearDown(container.dispose);
    await container.read(myRequestsProvider(filter).future);

    await container.read(myRequestsProvider(filter).notifier).loadMore();

    final MyRequestsState after = container.read(myRequestsProvider(filter)).value!;
    expect(after.items, hasLength(1)); // tidak jatuh ke error keseluruhan
    expect(after.loadMoreFailed, isTrue);
    expect(after.isLoadingMore, isFalse);
  });

  test('loadMore no-op saat hasMore false (semua sudah termuat)', () async {
    when(
      () => repository.list(filter: filter, offset: 0, limit: 20),
    ).thenAnswer((_) async => _page(<RequestDto>[_request('r1')], total: 1));

    final ProviderContainer container = makeContainer();
    addTearDown(container.dispose);
    await container.read(myRequestsProvider(filter).future);

    await container.read(myRequestsProvider(filter).notifier).loadMore();

    // Tak ada panggilan offset>0 karena hasMore sudah false.
    verifyNever(() => repository.list(filter: filter, offset: 1, limit: 20));
  });
}
