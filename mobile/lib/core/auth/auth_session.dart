import 'data/user_dto.dart';

/// Status sesi aplikasi — state global pertama dari tiga (ARCHITECTURE
/// bagian 2). Dipegang `authControllerProvider` sebagai `AsyncValue<AuthSession>`.
sealed class AuthSession {
  const AuthSession();
}

final class Unauthenticated extends AuthSession {
  const Unauthenticated();
}

final class Authenticated extends AuthSession {
  const Authenticated(this.user);

  /// Profil dari `GET /auth/me`.
  final UserDto user;
}
