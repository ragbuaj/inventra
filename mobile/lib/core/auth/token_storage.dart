import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_secure_storage/flutter_secure_storage.dart';

/// Penyimpanan token per ADR-0017: hanya refresh token yang persist, di
/// `flutter_secure_storage` (Keystore/Keychain). Access token tidak pernah
/// menyentuh disk — ia hidup di memori (SessionManager).
class TokenStorage {
  TokenStorage(this._storage);

  final FlutterSecureStorage _storage;

  static const String _refreshTokenKey = 'inventra_refresh_token';

  Future<String?> readRefreshToken() => _storage.read(key: _refreshTokenKey);

  Future<void> saveRefreshToken(String token) =>
      _storage.write(key: _refreshTokenKey, value: token);

  Future<void> clear() => _storage.delete(key: _refreshTokenKey);
}

final Provider<TokenStorage> tokenStorageProvider = Provider<TokenStorage>(
  (Ref ref) => TokenStorage(const FlutterSecureStorage()),
);
