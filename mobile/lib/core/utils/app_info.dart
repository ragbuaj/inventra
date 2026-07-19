/// Versi aplikasi untuk tampilan (footer login, kartu Tentang Pengaturan).
///
/// Disinkronkan manual dengan `version:` pubspec.yaml; beralih ke
/// package_info_plus bila kebutuhan runtime muncul (menghindari dependensi
/// plugin hanya untuk satu label).
abstract final class AppInfo {
  static const String version = '1.0.0';
  static const String buildNumber = '1';
}
