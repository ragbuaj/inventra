import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:shared_preferences/shared_preferences.dart';

/// Kunci preferensi lokal — HANYA preferensi tampilan non-sensitif.
/// Token/kredensial/PII dilarang di SharedPreferences (CONVENTIONS bagian 8);
/// refresh token tetap di flutter_secure_storage (ADR-0017).
abstract final class PrefKeys {
  /// Bahasa pilihan pengguna: 'id' / 'en'; absen berarti ikuti perangkat.
  static const String locale = 'app_locale';

  /// Tema pilihan pengguna: 'light' / 'dark' / 'system'; absen berarti system.
  static const String themeMode = 'app_theme_mode';
}

/// Penyimpanan preferensi key-value dengan baca sinkron — antarmuka tipis di
/// atas SharedPreferences supaya controller (locale/tema) bisa dites dengan
/// implementasi in-memory tanpa mocking plugin.
abstract class AppPreferences {
  String? getString(String key);

  Future<void> setString(String key, String value);
}

/// Implementasi produksi di atas [SharedPreferencesWithCache] (API yang
/// direkomendasikan paket shared_preferences: cache in-memory membuat baca
/// sinkron saat cold start). Dibuat di `main()` sebelum `runApp` lalu
/// di-inject lewat override [appPreferencesProvider].
class SharedPrefsAppPreferences implements AppPreferences {
  SharedPrefsAppPreferences(this._cache);

  final SharedPreferencesWithCache _cache;

  static Future<SharedPrefsAppPreferences> create() async {
    return SharedPrefsAppPreferences(
      await SharedPreferencesWithCache.create(
        cacheOptions: const SharedPreferencesWithCacheOptions(
          allowList: <String>{PrefKeys.locale, PrefKeys.themeMode},
        ),
      ),
    );
  }

  @override
  String? getString(String key) => _cache.getString(key);

  @override
  Future<void> setString(String key, String value) =>
      _cache.setString(key, value);
}

/// Implementasi in-memory: default provider (tes tidak menyentuh plugin) dan
/// jaring pengaman bila override produksi terlewat — preferensi tetap bekerja
/// untuk satu sesi, hanya tidak persist.
class InMemoryAppPreferences implements AppPreferences {
  final Map<String, String> _values = <String, String>{};

  @override
  String? getString(String key) => _values[key];

  @override
  Future<void> setString(String key, String value) async {
    _values[key] = value;
  }
}

/// Sumber preferensi aplikasi. `main()` meng-override dengan
/// [SharedPrefsAppPreferences]; default in-memory membuat widget test berjalan
/// tanpa setup platform channel.
final Provider<AppPreferences> appPreferencesProvider =
    Provider<AppPreferences>((Ref ref) => InMemoryAppPreferences());
