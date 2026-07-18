import 'dart:convert';
import 'dart:typed_data';

import 'package:dio/dio.dart';
import 'package:inventra_mobile/core/auth/token_storage.dart';

/// TokenStorage in-memory untuk tes SessionManager/AuthController — tanpa
/// plugin flutter_secure_storage (yang butuh platform channel).
class InMemoryTokenStorage implements TokenStorage {
  InMemoryTokenStorage([this.refreshToken]);

  String? refreshToken;
  int saveCount = 0;
  int clearCount = 0;

  @override
  Future<String?> readRefreshToken() async => refreshToken;

  @override
  Future<void> saveRefreshToken(String token) async {
    refreshToken = token;
    saveCount += 1;
  }

  @override
  Future<void> clear() async {
    refreshToken = null;
    clearCount += 1;
  }
}

/// Adapter HTTP palsu: setiap request diteruskan ke [handler] dan dicatat di
/// [requests] supaya tes bisa memeriksa header/urutan panggilan.
class RoutingHttpClientAdapter implements HttpClientAdapter {
  RoutingHttpClientAdapter(this.handler);

  final Future<ResponseBody> Function(RequestOptions options) handler;
  final List<RequestOptions> requests = <RequestOptions>[];

  @override
  Future<ResponseBody> fetch(
    RequestOptions options,
    Stream<Uint8List>? requestStream,
    Future<void>? cancelFuture,
  ) {
    requests.add(options);
    return handler(options);
  }

  @override
  void close({bool force = false}) {}
}

/// ResponseBody JSON untuk [RoutingHttpClientAdapter].
ResponseBody jsonResponseBody(int statusCode, Object payload) {
  return ResponseBody.fromString(
    jsonEncode(payload),
    statusCode,
    headers: <String, List<String>>{
      Headers.contentTypeHeader: <String>[Headers.jsonContentType],
    },
  );
}
