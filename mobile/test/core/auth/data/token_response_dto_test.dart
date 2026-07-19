import 'package:flutter_test/flutter_test.dart';
import 'package:inventra_mobile/core/auth/data/token_response_dto.dart';

void main() {
  test('fromJson memakai field English snake_case persis kontrak', () {
    final TokenResponseDto dto = TokenResponseDto.fromJson(<String, dynamic>{
      'access_token': 'access-1',
      'token_type': 'Bearer',
      'expires_in': 900,
      'refresh_token': 'rt-1',
    });

    expect(dto.accessToken, 'access-1');
    expect(dto.tokenType, 'Bearer');
    expect(dto.expiresIn, 900);
    expect(dto.refreshToken, 'rt-1');
  });

  test('refresh_token absen (bentuk respons web) menjadi null', () {
    final TokenResponseDto dto = TokenResponseDto.fromJson(<String, dynamic>{
      'access_token': 'access-1',
      'token_type': 'Bearer',
      'expires_in': 900,
    });

    expect(dto.refreshToken, isNull);
  });

  test('toJson mengeluarkan kunci snake_case', () {
    const TokenResponseDto dto = TokenResponseDto(
      accessToken: 'access-1',
      tokenType: 'Bearer',
      expiresIn: 900,
      refreshToken: 'rt-1',
    );

    expect(dto.toJson(), <String, dynamic>{
      'access_token': 'access-1',
      'token_type': 'Bearer',
      'expires_in': 900,
      'refresh_token': 'rt-1',
    });
  });

  test('toString tidak memuat access/refresh token mentah', () {
    const TokenResponseDto dto = TokenResponseDto(
      accessToken: 'super-secret-access',
      tokenType: 'Bearer',
      expiresIn: 900,
      refreshToken: 'super-secret-refresh',
    );

    final String rendered = dto.toString();
    expect(rendered, isNot(contains('super-secret-access')));
    expect(rendered, isNot(contains('super-secret-refresh')));
    // Field non-sensitif tetap terlihat untuk diagnosis.
    expect(rendered, contains('Bearer'));
    expect(rendered, contains('900'));
    expect(rendered, contains('[redacted]'));
  });

  test('toString membedakan refresh token null dari terisi', () {
    const TokenResponseDto withToken = TokenResponseDto(
      accessToken: 'a',
      tokenType: 'Bearer',
      expiresIn: 900,
      refreshToken: 'rt-1',
    );
    const TokenResponseDto withoutToken = TokenResponseDto(
      accessToken: 'a',
      tokenType: 'Bearer',
      expiresIn: 900,
    );

    expect(withToken.toString(), contains('refreshToken: [redacted]'));
    expect(withoutToken.toString(), contains('refreshToken: null'));
  });
}
