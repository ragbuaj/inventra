import 'package:flutter/services.dart';

/// Ekstrak digit mentah (tanpa pemisah ribuan / karakter lain) dari teks input
/// uang — dipakai saat mengirim ke backend yang menerima integer rupiah bulat.
/// "Rp 1.250.000" -> "1250000"; string tanpa digit -> "".
String currencyDigits(String text) => text.replaceAll(RegExp(r'[^0-9]'), '');

/// Formatter input uang: memformat digit dengan pemisah ribuan saat mengetik
/// (mis. "1000000" -> "1.000.000") dan menolak karakter non-digit. Pemisah
/// mengikuti locale (id memakai titik, en memakai koma). Pengelompokan
/// dilakukan manual pada string digit (tanpa `int.parse`) agar aman dari
/// overflow untuk angka sangat besar.
class ThousandsSeparatorInputFormatter extends TextInputFormatter {
  const ThousandsSeparatorInputFormatter({this.groupSeparator = '.'});

  /// Pemisah ribuan: '.' untuk id (default), ',' untuk en.
  final String groupSeparator;

  @override
  TextEditingValue formatEditUpdate(
    TextEditingValue oldValue,
    TextEditingValue newValue,
  ) {
    // Buang semua kecuali digit, lalu hilangkan nol di depan ("007" -> "7").
    String digits = newValue.text.replaceAll(RegExp(r'[^0-9]'), '');
    digits = digits.replaceFirst(RegExp(r'^0+(?=\d)'), '');
    if (digits.isEmpty) {
      return const TextEditingValue(text: '');
    }

    final StringBuffer buffer = StringBuffer();
    for (int i = 0; i < digits.length; i++) {
      if (i > 0 && (digits.length - i) % 3 == 0) {
        buffer.write(groupSeparator);
      }
      buffer.write(digits[i]);
    }
    final String formatted = buffer.toString();

    // Kursor selalu di akhir (input uang hanya bertambah dari kanan).
    return TextEditingValue(
      text: formatted,
      selection: TextSelection.collapsed(offset: formatted.length),
    );
  }
}
