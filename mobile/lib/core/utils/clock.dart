import 'package:flutter_riverpod/flutter_riverpod.dart';

/// Sumber waktu "sekarang" untuk UI (mis. label waktu relatif kartu inbox).
/// Dipisah sebagai provider supaya widget/golden test bisa membekukan waktu.
final Provider<DateTime Function()> clockProvider =
    Provider<DateTime Function()>((Ref ref) => DateTime.now);
