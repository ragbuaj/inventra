import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../../core/auth/auth_controller.dart';
import '../../../core/auth/auth_session.dart';

/// State submit form login, terpisah dari [authControllerProvider] supaya
/// loading cold-start sesi tidak ikut menampilkan state "Memproses" di form.
final AsyncNotifierProvider<LoginController, void> loginControllerProvider =
    AsyncNotifierProvider<LoginController, void>(LoginController.new);

class LoginController extends AsyncNotifier<void> {
  @override
  Future<void> build() async {}

  /// Meneruskan kredensial ke [AuthController.login] lalu mencerminkan
  /// hasilnya: sukses membuat guard router pindah ke beranda; gagal menjadi
  /// `AsyncError` berisi `AppFailure` yang dipetakan layar ke pesan i18n.
  Future<void> submit({required String email, required String password}) async {
    state = const AsyncLoading<void>();
    // Tunggu build() cold-start selesai dulu: bila login berjalan saat build
    // masih pending, hasil build akan menimpa state login sesudahnya.
    try {
      await ref.read(authControllerProvider.future);
    } on Object {
      // Kontrak AuthController.build tidak melempar; jika pun terjadi, biarkan
      // login di bawah yang menentukan state akhir.
    }
    await ref
        .read(authControllerProvider.notifier)
        .login(email: email, password: password);
    final AsyncValue<AuthSession> result = ref.read(authControllerProvider);
    final Object? failure = result.error;
    state = failure == null
        ? const AsyncData<void>(null)
        : AsyncError<void>(failure, result.stackTrace ?? StackTrace.current);
  }
}
