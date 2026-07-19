import 'package:dio/dio.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:inventra_mobile/core/masterdata/reference_lookup_repository.dart';
import 'package:mocktail/mocktail.dart';

class _MockDio extends Mock implements Dio {}

Response<Map<String, dynamic>> _jsonResponse(
  String path,
  Map<String, dynamic> data,
) {
  return Response<Map<String, dynamic>>(
    requestOptions: RequestOptions(path: path),
    statusCode: 200,
    data: data,
  );
}

void main() {
  late _MockDio dio;
  late DateTime now;
  late ReferenceLookupRepository repository;

  setUp(() {
    dio = _MockDio();
    now = DateTime(2026, 7, 19, 9);
    repository = ReferenceLookupRepository(dio, now: () => now);
  });

  void stubGet(String path, Map<String, dynamic> json) {
    when(
      () => dio.get<Map<String, dynamic>>(path),
    ).thenAnswer((_) async => _jsonResponse(path, json));
  }

  group('nameById (offices/categories/employees/brands/models/vendors)', () {
    test('sukses: GET get-by-id lalu kembalikan field name', () async {
      stubGet('/offices/off-1', <String, dynamic>{
        'id': 'off-1',
        'name': 'Cabang Jakarta Selatan',
        'code': 'JKT01',
      });

      expect(await repository.officeName('off-1'), 'Cabang Jakarta Selatan');
    });

    test('nama yang sama di-cache: Dio hanya dipanggil sekali', () async {
      stubGet('/categories/cat-1', <String, dynamic>{
        'id': 'cat-1',
        'name': 'Elektronik',
      });

      expect(await repository.categoryName('cat-1'), 'Elektronik');
      expect(await repository.categoryName('cat-1'), 'Elektronik');

      verify(
        () => dio.get<Map<String, dynamic>>('/categories/cat-1'),
      ).called(1);
    });

    test('cache kedaluwarsa setelah TTL: fetch ulang', () async {
      stubGet('/brands/br-1', <String, dynamic>{'id': 'br-1', 'name': 'Dell'});

      expect(await repository.brandName('br-1'), 'Dell');
      now = now.add(
        ReferenceLookupRepository.cacheTtl + const Duration(seconds: 1),
      );
      expect(await repository.brandName('br-1'), 'Dell');

      verify(() => dio.get<Map<String, dynamic>>('/brands/br-1')).called(2);
    });

    test('gagal (offline/403/404): null tanpa melempar — non-fatal', () async {
      when(() => dio.get<Map<String, dynamic>>('/employees/emp-1')).thenThrow(
        DioException(
          requestOptions: RequestOptions(path: '/employees/emp-1'),
          type: DioExceptionType.connectionError,
        ),
      );

      expect(await repository.employeeName('emp-1'), isNull);
    });

    test('kegagalan TIDAK di-cache: percobaan berikutnya fetch lagi', () async {
      when(() => dio.get<Map<String, dynamic>>('/vendors/v-1')).thenThrow(
        DioException(requestOptions: RequestOptions(path: '/vendors/v-1')),
      );
      expect(await repository.vendorName('v-1'), isNull);

      stubGet('/vendors/v-1', <String, dynamic>{
        'id': 'v-1',
        'name': 'PT Mitra Teknologi Nusantara',
      });
      expect(
        await repository.vendorName('v-1'),
        'PT Mitra Teknologi Nusantara',
      );

      verify(() => dio.get<Map<String, dynamic>>('/vendors/v-1')).called(2);
    });

    test(
      'respons tanpa field name valid: null (klien tidak menebak)',
      () async {
        stubGet('/models/m-1', <String, dynamic>{'id': 'm-1', 'name': ''});

        expect(await repository.modelName('m-1'), isNull);
      },
    );

    test('id di-URL-encode pada path', () async {
      stubGet('/offices/a%2Fb', <String, dynamic>{'name': 'X'});

      expect(await repository.officeName('a/b'), 'X');
      verify(() => dio.get<Map<String, dynamic>>('/offices/a%2Fb')).called(1);
    });
  });

  group('roomLabel', () {
    test('gabungan "Lantai · Ruang" dari /rooms/{id} + /floors/{id}', () async {
      stubGet('/rooms/room-1', <String, dynamic>{
        'id': 'room-1',
        'floor_id': 'fl-1',
        'name': 'Ruang Operasional',
      });
      stubGet('/floors/fl-1', <String, dynamic>{
        'id': 'fl-1',
        'name': 'Lantai 2',
      });

      expect(
        await repository.roomLabel('room-1'),
        'Lantai 2 · Ruang Operasional',
      );
    });

    test(
      'lantai gagal di-resolve: nama ruangan saja (parsial tetap tampil)',
      () async {
        stubGet('/rooms/room-1', <String, dynamic>{
          'id': 'room-1',
          'floor_id': 'fl-1',
          'name': 'Ruang Operasional',
        });
        when(() => dio.get<Map<String, dynamic>>('/floors/fl-1')).thenThrow(
          DioException(requestOptions: RequestOptions(path: '/floors/fl-1')),
        );

        expect(await repository.roomLabel('room-1'), 'Ruang Operasional');
      },
    );

    test('ruangan gagal: null non-fatal', () async {
      when(() => dio.get<Map<String, dynamic>>('/rooms/room-1')).thenThrow(
        DioException(requestOptions: RequestOptions(path: '/rooms/room-1')),
      );

      expect(await repository.roomLabel('room-1'), isNull);
    });

    test('label ruangan di-cache: kunjungan kedua tanpa fetch', () async {
      stubGet('/rooms/room-1', <String, dynamic>{
        'id': 'room-1',
        'floor_id': 'fl-1',
        'name': 'Ruang Operasional',
      });
      stubGet('/floors/fl-1', <String, dynamic>{
        'id': 'fl-1',
        'name': 'Lantai 2',
      });

      await repository.roomLabel('room-1');
      await repository.roomLabel('room-1');

      verify(() => dio.get<Map<String, dynamic>>('/rooms/room-1')).called(1);
      verify(() => dio.get<Map<String, dynamic>>('/floors/fl-1')).called(1);
    });
  });
}
