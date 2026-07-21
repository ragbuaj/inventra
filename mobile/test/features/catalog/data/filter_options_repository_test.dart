import 'package:dio/dio.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:inventra_mobile/core/api/app_failure.dart';
import 'package:inventra_mobile/features/catalog/data/filter_options_repository.dart';
import 'package:mocktail/mocktail.dart';

class _MockDio extends Mock implements Dio {}

class _MockRepo extends Mock implements FilterOptionsRepository {}

Response<Map<String, dynamic>> _resp(Map<String, dynamic> data) =>
    Response<Map<String, dynamic>>(
      requestOptions: RequestOptions(path: '/x'),
      statusCode: 200,
      data: data,
    );

void main() {
  late _MockDio dio;
  late FilterOptionsRepository repository;

  setUp(() {
    dio = _MockDio();
    repository = FilterOptionsRepository(dio);
  });

  group('FilterOptionsRepository', () {
    test('offices: GET /offices dengan limit 100 + parse id/name', () async {
      when(
        () => dio.get<Map<String, dynamic>>(
          '/offices',
          queryParameters: any(named: 'queryParameters'),
        ),
      ).thenAnswer(
        (_) async => _resp(<String, dynamic>{
          'data': <dynamic>[
            <String, dynamic>{'id': 'o1', 'name': 'Kantor Pusat'},
            <String, dynamic>{'id': 'o2', 'name': 'Cabang Jakarta'}
          ]
        }),
      );

      final List<FilterOption> options = await repository.offices();

      expect(options, hasLength(2));
      expect(options.first.id, 'o1');
      expect(options.first.name, 'Kantor Pusat');
      final Map<String, dynamic> query =
          verify(
                () => dio.get<Map<String, dynamic>>(
                  '/offices',
                  queryParameters: captureAny(named: 'queryParameters'),
                ),
              ).captured.single
              as Map<String, dynamic>;
      expect(query['limit'], 100);
      expect(query['offset'], 0);
    });

    test('categories: GET /categories', () async {
      when(
        () => dio.get<Map<String, dynamic>>(
          '/categories',
          queryParameters: any(named: 'queryParameters'),
        ),
      ).thenAnswer(
        (_) async => _resp(<String, dynamic>{
          'data': <dynamic>[
            <String, dynamic>{'id': 'c1', 'name': 'Elektronik'}
          ]
        }),
      );

      final List<FilterOption> options = await repository.categories();
      expect(options.single.id, 'c1');
    });

    test('entri tanpa id/name atau name kosong di-drop', () async {
      when(
        () => dio.get<Map<String, dynamic>>(
          '/offices',
          queryParameters: any(named: 'queryParameters'),
        ),
      ).thenAnswer(
        (_) async => _resp(<String, dynamic>{
          'data': <dynamic>[
            <String, dynamic>{'id': 'o1', 'name': 'Valid'},
            <String, dynamic>{'id': 'o2'}, // tanpa name
            <String, dynamic>{'name': 'TanpaId'}, // tanpa id
            <String, dynamic>{'id': 'o3', 'name': ''}, // name kosong
            'bukan-map'
          ]
        }),
      );

      final List<FilterOption> options = await repository.offices();
      expect(options, hasLength(1));
      expect(options.single.id, 'o1');
    });

    test('data bukan List (atau null): kembalikan kosong', () async {
      when(
        () => dio.get<Map<String, dynamic>>(
          '/offices',
          queryParameters: any(named: 'queryParameters'),
        ),
      ).thenAnswer(
        (_) async => _resp(<String, dynamic>{'data': <String, dynamic>{}}),
      );

      expect(await repository.offices(), isEmpty);
    });

    test('offline: DioException dipetakan ke AppFailure', () async {
      when(
        () => dio.get<Map<String, dynamic>>(
          '/offices',
          queryParameters: any(named: 'queryParameters'),
        ),
      ).thenThrow(
        DioException(
          requestOptions: RequestOptions(path: '/offices'),
          type: DioExceptionType.connectionError,
        ),
      );

      expect(() => repository.offices(), throwsA(isA<NetworkFailure>()));
    });
  });

  group('provider opsi filter autoDispose', () {
    test('catalogOfficeOptionsProvider mengambil ulang setelah tak ada '
        'listener (cegah data kantor basi lintas-user)', () async {
      final _MockRepo repo = _MockRepo();
      when(
        () => repo.offices(),
      ).thenAnswer((_) async => const <FilterOption>[FilterOption('o1', 'A')]);

      final ProviderContainer container = ProviderContainer(
        overrides: [
          filterOptionsRepositoryProvider.overrideWithValue(repo),
        ],
      );
      addTearDown(container.dispose);

      final ProviderSubscription<AsyncValue<List<FilterOption>>> sub1 =
          container.listen(
            catalogOfficeOptionsProvider,
            (_, _) {},
          );
      await container.read(catalogOfficeOptionsProvider.future);
      // Lepas listener -> autoDispose membuang state.
      sub1.close();
      await Future<void>.delayed(Duration.zero);

      // Langganan baru memicu fetch SEGAR (state lama sudah dibuang).
      container.listen(catalogOfficeOptionsProvider, (_, _) {});
      await container.read(catalogOfficeOptionsProvider.future);

      verify(() => repo.offices()).called(2);
    });
  });
}
