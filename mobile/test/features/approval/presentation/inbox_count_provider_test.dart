import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:inventra_mobile/core/auth/auth_controller.dart';
import 'package:inventra_mobile/core/auth/auth_session.dart';
import 'package:inventra_mobile/core/auth/data/user_dto.dart';
import 'package:inventra_mobile/features/approval/data/approval_repository.dart';
import 'package:inventra_mobile/features/approval/presentation/inbox_count_provider.dart';
import 'package:mocktail/mocktail.dart';

import '../../../helpers/fake_auth_controller.dart';

class _MockApprovalRepository extends Mock implements ApprovalRepository {}

/// User A yang login lebih dulu; user B memakai [fakeUser] dari helper.
const UserDto _userA = UserDto(
  id: 'user-a',
  name: 'Andi Pratama',
  email: 'andi@bank.co.id',
  roleId: 'role-1',
  status: 'active',
  googleLinked: false,
);

void main() {
  test(
    'badge di-fetch ulang saat user berganti (bukan angka user lama)',
    () async {
      // Nilai inbox count "milik" user aktif — diubah saat berganti user untuk
      // membuktikan badge menampilkan angka user baru, bukan cache user lama.
      int inboxCount = 12;
      final _MockApprovalRepository repository = _MockApprovalRepository();
      when(() => repository.inboxCount()).thenAnswer((_) async => inboxCount);

      final ProviderContainer container = ProviderContainer.test(
        overrides: [
          authControllerProvider.overrideWith(
            () =>
                FakeAuthController(initialSession: const Authenticated(_userA)),
          ),
          approvalRepositoryProvider.overrideWithValue(repository),
        ],
      );
      // Jaga provider tetap hidup melewati perubahan sesi.
      container.listen(approvalInboxCountProvider, (_, _) {});

      // User A: badge = 12.
      expect(await container.read(approvalInboxCountProvider.future), 12);

      // Berganti user di perangkat yang sama: logout lalu user B login.
      inboxCount = 3;
      await container.read(authControllerProvider.notifier).logout();
      await container
          .read(authControllerProvider.notifier)
          .login(email: 'budi@bank.co.id', password: 'secret123');
      await Future<void>.delayed(Duration.zero);

      // Badge di-fetch ulang: menampilkan angka user B (3), bukan 12 milik A.
      expect(await container.read(approvalInboxCountProvider.future), 3);
      // Minimal satu fetch untuk A dan satu untuk B setelah sesi berubah.
      verify(() => repository.inboxCount()).called(greaterThanOrEqualTo(2));
    },
  );
}
