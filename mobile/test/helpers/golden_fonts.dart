import 'dart:convert';

import 'package:flutter/services.dart';

/// Memuat seluruh font dari FontManifest.json (Inter yang di-bundle + font
/// ikon paket material_symbols_icons) supaya golden dirender dengan glyph
/// nyata, bukan font Ahem default flutter_test.
Future<void> loadAppFonts() async {
  final String manifestJson = await rootBundle.loadString('FontManifest.json');
  final List<dynamic> manifest = json.decode(manifestJson) as List<dynamic>;
  for (final dynamic entry in manifest) {
    final Map<String, dynamic> family = entry as Map<String, dynamic>;
    final FontLoader loader = FontLoader(family['family'] as String);
    for (final dynamic font in family['fonts'] as List<dynamic>) {
      loader.addFont(
        rootBundle.load((font as Map<String, dynamic>)['asset'] as String),
      );
    }
    await loader.load();
  }
}
