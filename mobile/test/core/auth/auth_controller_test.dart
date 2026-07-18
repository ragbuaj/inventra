import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:inventra_mobile/core/api/app_failure.dart';
import 'package:inventra_mobile/core/auth/auth_controller.dart';
import 'package:inventra_mobile/core/auth/auth_session.dart';
import 'package:inventra_mobile/core/auth/session_manager.dart';
import 'package:inventra_mobile/core/auth/token_storage.dart';
import 'package:inventra_mobile/core/auth/data/auth_repository.dart';
import 'package:inventra_mobile/core/auth/data/token_response_dto.dart';
import 'package:inventra_mobile/core/auth/data/user_dto.dart';
import 'package:mocktail/mocktail.dart';

import '../../helpers/fakes.dart';

class _MockAuthRepository extends Mock implements AuthRepository {}

const UserDto _user = UserDto(
  id: 'user-1',
  name: 'Ragil',
  email: 'ragil@inventra.local',
  roleId: 'role-1',
  status: 'active',
  googleLinked: false,
);

const TokenResponseDto _tokens = TokenResponseDto(
  accessToken: 'access-1',
  tokenType: 'Bearer',
  expiresIn: 900,
  refreshToken: 'rt-2',
);

void main() {
  late _MockAuthRepository repository;
  late InMemoryTokenStorage storage;

  ProviderContainer makeContainer({String? storedRefreshToken}) {
    repository = _MockAuthRepository();
    storage = InMemoryTokenStorage(storedRefreshToken);
    return ProviderContainer.test(
      overrides: [
        tokenStorageProvider.overrideWithValue(storage),
        authRepositoryProvider.overrideWithValue(repository),
      ],
    );
  }

  group('cold start (build)', () {
    test('sukses: refresh + me, refresh token rotasi tersimpan', () async {
      final ProviderContainer container = makeContainer(
        storedRefreshToken: 'rt-1',
      );
      when(
        () => repository.refresh(refreshToken: 'rt-1'),
      ).thenAnswer((_) async => _tokens);
      when(() => repository.me()).thenAnswer((_) async => _user);

      final AuthSession session = await container.read(
        authControllerProvider.future,
      );

      expect(session, isA<Authenticated>());
      expect((session as Authenticated).user, _user);
      expect(storage.refreshToken, 'rt-2');
      expect(container.read(sessionManagerProvider).accessToken, 'access-1');
      verify(() => repository.refresh(refreshToken: 'rt-1')).called(1);
    });

    test(
      'refresh ditolak (401): unauthenticated dan refresh token terhapus',
      () async {
        final ProviderContainer container = makeContainer(
          storedRefreshToken: 'rt-1',
        );
        when(
          () => repository.refresh(refreshToken: 'rt-1'),
        ).thenThrow(const UnauthorizedFailure());

        final AuthSession session = await container.read(
          authControllerProvider.future,
        );

        expect(session, isA<Unauthenticated>());
        expect(storage.refreshToken, isNull);
        expect(container.read(sessionManagerProvider).accessToken, isNull);
        verifyNever(() => repository.me());
      },
    );

    test('refresh gagal jaringan (offline): unauthenticated tapi refresh token '
        'DIPERTAHANKAN untuk launch berikutnya', () async {
      final ProviderContainer container = makeContainer(
        storedRefreshToken: 'rt-1',
      );
      when(
        () => repository.refresh(refreshToken: 'rt-1'),
      ).thenThrow(const NetworkFailure());

      final AuthSession session = await container.read(
        authControllerProvider.future,
      );

      expect(session, isA<Unauthenticated>());
      expect(storage.refreshToken, 'rt-1');
      expect(storage.clearCount, 0);
      expect(container.read(sessionManagerProvider).accessToken, isNull);
      verifyNever(() => repository.me());
    });

    test(
      'tanpa refresh token: unauthenticated tanpa memanggil backend',
      () async {
        final ProviderContainer container = makeContainer();

        final AuthSession session = await container.read(
          authControllerProvider.future,
        );

        expect(session, isA<Unauthenticated>());
        verifyNever(
          () => repository.refresh(refreshToken: any(named: 'refreshToken')),
        );
        verifyNever(() => repository.me());
      },
    );

    test(
      'me gagal setelah refresh sukses: unauthenticated dan bersih',
      () async {
        final ProviderContainer container = makeContainer(
          storedRefreshToken: 'rt-1',
        );
        when(
          () => repository.refresh(refreshToken: 'rt-1'),
        ).thenAnswer((_) async => _tokens);
        when(() => repository.me()).thenThrow(const ServerFailure());

        final AuthSession session = await container.read(
          authControllerProvider.future,
        );

        expect(session, isA<Unauthenticated>());
        expect(storage.refreshToken, isNull);
        expect(container.read(sessionManagerProvider).accessToken, isNull);
      },
    );

    test(
      'me gagal jaringan setelah refresh sukses: refresh token dipertahankan',
      () async {
        final ProviderContainer container = makeContainer(
          storedRefreshToken: 'rt-1',
        );
        when(
          () => repository.refresh(refreshToken: 'rt-1'),
        ).thenAnswer((_) async => _tokens);
        when(() => repository.me()).thenThrow(const NetworkFailure());

        final AuthSession session = await container.read(
          authControllerProvider.future,
        );

        expect(session, isA<Unauthenticated>());
        // Token rotasi hasil refresh tetap tersimpan untuk percobaan lain.
        expect(storage.refreshToken, 'rt-2');
        expect(container.read(sessionManagerProvider).accessToken, isNull);
      },
    );
  });

  group('login', () {
    test('sukses: token diadopsi lalu state authenticated', () async {
      final ProviderContainer container = makeContainer();
      await container.read(authControllerProvider.future);
      when(
        () => repository.login(
          email: 'ragil@inventra.local',
          password: 'secret123',
        ),
      ).thenAnswer((_) async => _tokens);
      when(() => repository.me()).thenAnswer((_) async => _user);

      await container
          .read(authControllerProvider.notifier)
          .login(email: 'ragil@inventra.local', password: 'secret123');

      final AsyncValue<AuthSession> state = container.read(
        authControllerProvider,
      );
      expect(state.value, isA<Authenticated>());
      expect(storage.refreshToken, 'rt-2');
      expect(container.read(sessionManagerProvider).accessToken, 'access-1');
    });

    test(
      'gagal: state error berisi AppFailure, tanpa token tersimpan',
      () async {
        final ProviderContainer container = makeContainer();
        await container.read(authControllerProvider.future);
        when(
          () => repository.login(
            email: any(named: 'email'),
            password: any(named: 'password'),
          ),
        ).thenThrow(const UnauthorizedFailure());

        await container
            .read(authControllerProvider.notifier)
            .login(email: 'ragil@inventra.local', password: 'salah');

        final AsyncValue<AuthSession> state = container.read(
          authControllerProvider,
        );
        expect(state.hasError, isTrue);
        expect(state.error, isA<UnauthorizedFailure>());
        expect(storage.refreshToken, isNull);
        expect(container.read(sessionManagerProvider).accessToken, isNull);
        verifyNever(() => repository.me());
      },
    );
  });

  group('logout', () {
    Future<ProviderContainer> makeAuthenticated() async {
      final ProviderContainer container = makeContainer(
        storedRefreshToken: 'rt-1',
      );
      when(
        () => repository.refresh(refreshToken: 'rt-1'),
      ).thenAnswer((_) async => _tokens);
      when(() => repository.me()).thenAnswer((_) async => _user);
      await container.read(authControllerProvider.future);
      return container;
    }

    test(
      'memanggil endpoint dengan refresh token lalu bersih-bersih',
      () async {
        final ProviderContainer container = await makeAuthenticated();
        when(
          () => repository.logout(refreshToken: 'rt-2'),
        ).thenAnswer((_) async {});

        await container.read(authControllerProvider.notifier).logout();

        verify(() => repository.logout(refreshToken: 'rt-2')).called(1);
        expect(
          container.read(authControllerProvider).value,
          isA<Unauthenticated>(),
        );
        expect(storage.refreshToken, isNull);
        expect(container.read(sessionManagerProvider).accessToken, isNull);
      },
    );

    test('endpoint gagal: sesi lokal tetap bersih', () async {
      final ProviderContainer container = await makeAuthenticated();
      when(
        () => repository.logout(refreshToken: any(named: 'refreshToken')),
      ).thenThrow(const NetworkFailure());

      await container.read(authControllerProvider.notifier).logout();

      expect(
        container.read(authControllerProvider).value,
        isA<Unauthenticated>(),
      );
      expect(storage.refreshToken, isNull);
      expect(container.read(sessionManagerProvider).accessToken, isNull);
    });

    test(
      'tanpa refresh token tersimpan: langsung bersih tanpa endpoint',
      () async {
        final ProviderContainer container = makeContainer();
        await container.read(authControllerProvider.future);

        await container.read(authControllerProvider.notifier).logout();

        verifyNever(
          () => repository.logout(refreshToken: any(named: 'refreshToken')),
        );
        expect(
          container.read(authControllerProvider).value,
          isA<Unauthenticated>(),
        );
      },
    );
  });

  group('sesi mati dari interceptor', () {
    test(
      'notifySessionExpired membuat state unauthenticated dan bersih',
      () async {
        final ProviderContainer container = makeContainer(
          storedRefreshToken: 'rt-1',
        );
        when(
          () => repository.refresh(refreshToken: 'rt-1'),
        ).thenAnswer((_) async => _tokens);
        when(() => repository.me()).thenAnswer((_) async => _user);
        await container.read(authControllerProvider.future);

        container.read(sessionManagerProvider).notifySessionExpired();
        // clear() berjalan fire-and-forget; beri satu microtask turn.
        await Future<void>.delayed(Duration.zero);

        expect(
          container.read(authControllerProvider).value,
          isA<Unauthenticated>(),
        );
        expect(storage.refreshToken, isNull);
        expect(container.read(sessionManagerProvider).accessToken, isNull);
      },
    );
  });
}
