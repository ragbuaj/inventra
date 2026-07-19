import 'package:dio/dio.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:inventra_mobile/core/api/app_failure.dart';
import 'package:inventra_mobile/features/notifications/data/notification_dto.dart';
import 'package:inventra_mobile/features/notifications/data/notification_list_dto.dart';
import 'package:inventra_mobile/features/notifications/data/notifications_repository.dart';
import 'package:mocktail/mocktail.dart';

import 'notification_dto_test.dart'
    show fullNotificationJson, readNotificationJson;

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

DioException _statusError(String path, int statusCode) {
  final RequestOptions options = RequestOptions(path: path);
  return DioException(
    requestOptions: options,
    type: DioExceptionType.badResponse,
    response: Response<dynamic>(
      requestOptions: options,
      statusCode: statusCode,
      data: <String, dynamic>{'error': 'server message'},
    ),
  );
}

Map<String, dynamic> _listJson({
  List<Map<String, dynamic>>? data,
  int total = 1,
  int limit = 20,
  int offset = 0,
}) {
  return <String, dynamic>{
    'data': data ?? <Map<String, dynamic>>[fullNotificationJson],
    'total': total,
    'limit': limit,
    'offset': offset,
  };
}

void main() {
  late _MockDio dio;
  late NotificationsRepository repository;

  setUp(() {
    dio = _MockDio();
    repository = NotificationsRepository(dio);
  });

  group('list', () {
    test('default: TANPA parameter read + limit/offset default', () async {
      when(
        () => dio.get<Map<String, dynamic>>(
          '/notifications',
          queryParameters: any(named: 'queryParameters'),
        ),
      ).thenAnswer((_) async => _jsonResponse('/notifications', _listJson()));

      final NotificationListDto page = await repository.list();

      expect(page.data.single.id, 'notif-1');
      final Map<String, dynamic> query =
          verify(
                () => dio.get<Map<String, dynamic>>(
                  '/notifications',
                  queryParameters: captureAny(named: 'queryParameters'),
                ),
              ).captured.single
              as Map<String, dynamic>;
      expect(query, <String, dynamic>{'limit': 20, 'offset': 0});
    });

    test('filter read=false diteruskan sebagai query', () async {
      when(
        () => dio.get<Map<String, dynamic>>(
          '/notifications',
          queryParameters: any(named: 'queryParameters'),
        ),
      ).thenAnswer((_) async => _jsonResponse('/notifications', _listJson()));

      await repository.list(read: false);

      final Map<String, dynamic> query =
          verify(
                () => dio.get<Map<String, dynamic>>(
                  '/notifications',
                  queryParameters: captureAny(named: 'queryParameters'),
                ),
              ).captured.single
              as Map<String, dynamic>;
      expect(query['read'], false);
    });

    test('pagination: offset halaman berikutnya diteruskan', () async {
      when(
        () => dio.get<Map<String, dynamic>>(
          '/notifications',
          queryParameters: any(named: 'queryParameters'),
        ),
      ).thenAnswer(
        (_) async => _jsonResponse(
          '/notifications',
          _listJson(
            data: <Map<String, dynamic>>[readNotificationJson],
            total: 45,
            offset: 20,
          ),
        ),
      );

      final NotificationListDto page = await repository.list(offset: 20);

      expect(page.total, 45);
      final Map<String, dynamic> query =
          verify(
                () => dio.get<Map<String, dynamic>>(
                  '/notifications',
                  queryParameters: captureAny(named: 'queryParameters'),
                ),
              ).captured.single
              as Map<String, dynamic>;
      expect(query['offset'], 20);
    });

    test('offline: NetworkFailure', () async {
      when(
        () => dio.get<Map<String, dynamic>>(
          '/notifications',
          queryParameters: any(named: 'queryParameters'),
        ),
      ).thenThrow(
        DioException(
          requestOptions: RequestOptions(path: '/notifications'),
          type: DioExceptionType.connectionError,
        ),
      );

      expect(() => repository.list(), throwsA(isA<NetworkFailure>()));
    });
  });

  group('unreadCount', () {
    test('sukses: nilai count', () async {
      when(
        () => dio.get<Map<String, dynamic>>('/notifications/unread-count'),
      ).thenAnswer(
        (_) async => _jsonResponse(
          '/notifications/unread-count',
          <String, dynamic>{'count': 3},
        ),
      );

      expect(await repository.unreadCount(), 3);
    });

    test('offline: NetworkFailure (pemanggil badge non-fatal)', () async {
      when(
        () => dio.get<Map<String, dynamic>>('/notifications/unread-count'),
      ).thenThrow(
        DioException(
          requestOptions: RequestOptions(path: '/notifications/unread-count'),
          type: DioExceptionType.connectionError,
        ),
      );

      expect(() => repository.unreadCount(), throwsA(isA<NetworkFailure>()));
    });
  });

  group('markRead', () {
    test(
      'POST /notifications/{id}/read: notifikasi terbaru dikembalikan',
      () async {
        when(
          () => dio.post<Map<String, dynamic>>('/notifications/notif-1/read'),
        ).thenAnswer(
          (_) async =>
              _jsonResponse('/notifications/notif-1/read', <String, dynamic>{
                ...fullNotificationJson,
                'read_at': '2026-07-19T03:00:00Z',
              }),
        );

        final NotificationDto updated = await repository.markRead('notif-1');

        expect(updated.readAt, DateTime.utc(2026, 7, 19, 3));
        verify(
          () => dio.post<Map<String, dynamic>>('/notifications/notif-1/read'),
        ).called(1);
      },
    );

    test('404 (termasuk id milik pengguna lain): NotFoundFailure', () async {
      when(
        () => dio.post<Map<String, dynamic>>('/notifications/notif-x/read'),
      ).thenThrow(_statusError('/notifications/notif-x/read', 404));

      expect(
        () => repository.markRead('notif-x'),
        throwsA(isA<NotFoundFailure>()),
      );
    });
  });

  group('markAllRead', () {
    test('POST /notifications/read-all (204) selesai tanpa nilai', () async {
      when(() => dio.post<void>('/notifications/read-all')).thenAnswer(
        (_) async => Response<void>(
          requestOptions: RequestOptions(path: '/notifications/read-all'),
          statusCode: 204,
        ),
      );

      await repository.markAllRead();

      verify(() => dio.post<void>('/notifications/read-all')).called(1);
    });

    test('5xx: ServerFailure', () async {
      when(
        () => dio.post<void>('/notifications/read-all'),
      ).thenThrow(_statusError('/notifications/read-all', 500));

      expect(() => repository.markAllRead(), throwsA(isA<ServerFailure>()));
    });
  });
}
