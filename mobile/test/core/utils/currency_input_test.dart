import 'package:flutter_test/flutter_test.dart';
import 'package:inventra_mobile/core/utils/currency_input.dart';

void main() {
  group('currencyDigits', () {
    test('mengambil digit mentah dari teks terformat', () {
      expect(currencyDigits('Rp 1.250.000'), '1250000');
      expect(currencyDigits('2,500,000'), '2500000');
      expect(currencyDigits('1000'), '1000');
    });

    test('string tanpa digit menghasilkan kosong', () {
      expect(currencyDigits(''), '');
      expect(currencyDigits('Rp'), '');
      expect(currencyDigits('abc'), '');
    });
  });

  group('ThousandsSeparatorInputFormatter', () {
    String format(String input, {String sep = '.'}) {
      final ThousandsSeparatorInputFormatter formatter =
          ThousandsSeparatorInputFormatter(groupSeparator: sep);
      return formatter
          .formatEditUpdate(
            const TextEditingValue(text: ''),
            TextEditingValue(text: input),
          )
          .text;
    }

    test('mengelompokkan ribuan dengan titik (id)', () {
      expect(format('1000000'), '1.000.000');
      expect(format('1000'), '1.000');
      expect(format('999'), '999');
      expect(format('12'), '12');
    });

    test('memakai koma untuk locale en', () {
      expect(format('1000000', sep: ','), '1,000,000');
    });

    test('membuang karakter non-digit', () {
      expect(format('1a2b3c'), '123');
      expect(format('Rp 5.000'), '5.000');
    });

    test('menghapus nol di depan (menyisakan satu nol untuk nilai nol)', () {
      expect(format('007'), '7');
      expect(format('000'), '0');
    });

    test('kosong tetap kosong', () {
      expect(format(''), '');
    });

    test('kursor diletakkan di akhir teks terformat', () {
      final ThousandsSeparatorInputFormatter formatter =
          const ThousandsSeparatorInputFormatter();
      final TextEditingValue result = formatter.formatEditUpdate(
        const TextEditingValue(text: ''),
        const TextEditingValue(text: '1000000'),
      );
      expect(result.text, '1.000.000');
      expect(result.selection.baseOffset, '1.000.000'.length);
    });
  });
}
