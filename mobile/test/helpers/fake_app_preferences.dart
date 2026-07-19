import 'package:inventra_mobile/core/prefs/app_preferences.dart';

/// [AppPreferences] spy untuk tes persist bahasa/tema: nilai in-memory +
/// pencatatan setiap panggilan setString.
class FakeAppPreferences implements AppPreferences {
  FakeAppPreferences([Map<String, String>? initial])
    : values = <String, String>{...?initial};

  final Map<String, String> values;
  final List<(String, String)> setCalls = <(String, String)>[];

  @override
  String? getString(String key) => values[key];

  @override
  Future<void> setString(String key, String value) async {
    setCalls.add((key, value));
    values[key] = value;
  }
}
