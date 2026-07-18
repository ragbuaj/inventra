import 'package:flutter_secure_storage/flutter_secure_storage.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:inventra_mobile/core/auth/token_storage.dart';
import 'package:mocktail/mocktail.dart';

class _MockFlutterSecureStorage extends Mock implements FlutterSecureStorage {}

void main() {
  late _MockFlutterSecureStorage secureStorage;
  late TokenStorage tokenStorage;

  setUp(() {
    secureStorage = _MockFlutterSecureStorage();
    tokenStorage = TokenStorage(secureStorage);
  });

  test('readRefreshToken membaca kunci refresh token', () async {
    when(
      () => secureStorage.read(key: any(named: 'key')),
    ).thenAnswer((_) async => 'rt-1');

    expect(await tokenStorage.readRefreshToken(), 'rt-1');
    verify(() => secureStorage.read(key: 'inventra_refresh_token')).called(1);
  });

  test('readRefreshToken meneruskan null saat belum ada token', () async {
    when(
      () => secureStorage.read(key: any(named: 'key')),
    ).thenAnswer((_) async => null);

    expect(await tokenStorage.readRefreshToken(), isNull);
  });

  test('saveRefreshToken menulis ke kunci refresh token', () async {
    when(
      () => secureStorage.write(
        key: any(named: 'key'),
        value: any(named: 'value'),
      ),
    ).thenAnswer((_) async {});

    await tokenStorage.saveRefreshToken('rt-2');

    verify(
      () => secureStorage.write(key: 'inventra_refresh_token', value: 'rt-2'),
    ).called(1);
  });

  test('clear menghapus kunci refresh token', () async {
    when(
      () => secureStorage.delete(key: any(named: 'key')),
    ).thenAnswer((_) async {});

    await tokenStorage.clear();

    verify(() => secureStorage.delete(key: 'inventra_refresh_token')).called(1);
  });
}
