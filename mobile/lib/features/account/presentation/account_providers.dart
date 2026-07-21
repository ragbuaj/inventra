import 'dart:typed_data';

import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../../core/api/app_failure.dart';
import '../../../core/auth/auth_controller.dart';
import '../../../core/auth/auth_session.dart';
import '../../../core/masterdata/reference_lookup_repository.dart';
import '../data/account_repository.dart';
import '../data/profile_dto.dart';
import '../data/session_dto.dart';

/// Daftar sesi device layar Profil. autoDispose: dibuang saat layar ditutup;
/// auto-retry Riverpod dimatikan — pengguna punya tombol "Coba lagi".
final accountSessionsProvider =
    AsyncNotifierProvider.autoDispose<
      AccountSessionsController,
      List<SessionDto>
    >(
      AccountSessionsController.new,
      retry: (int retryCount, Object error) => null,
    );

class AccountSessionsController extends AsyncNotifier<List<SessionDto>> {
  @override
  Future<List<SessionDto>> build() =>
      ref.watch(accountRepositoryProvider).sessions();

  /// Mencabut satu sesi lain. Sukses (200) menghapus baris dari daftar tanpa
  /// refetch; false bila server menolak — daftar utuh, layar menampilkan
  /// pemberitahuan.
  Future<bool> revoke(String id) async {
    try {
      await ref.read(accountRepositoryProvider).revokeSession(id);
    } on AppFailure {
      return false;
    }
    final List<SessionDto>? current = state.value;
    if (current != null) {
      state = AsyncData<List<SessionDto>>(
        List<SessionDto>.unmodifiable(
          current.where((SessionDto session) => session.id != id),
        ),
      );
    }
    return true;
  }

  /// Mencabut semua sesi lain sekaligus; menyisakan sesi ini pada daftar.
  Future<bool> revokeOthers() async {
    try {
      await ref.read(accountRepositoryProvider).revokeOtherSessions();
    } on AppFailure {
      return false;
    }
    final List<SessionDto>? current = state.value;
    if (current != null) {
      state = AsyncData<List<SessionDto>>(
        List<SessionDto>.unmodifiable(
          current.where((SessionDto session) => session.current),
        ),
      );
    }
    return true;
  }
}

/// Profil lengkap pemanggil (`GET /auth/profile`) untuk kartu Data Diri +
/// Detail Pegawai + Informasi Akun. autoDispose; auto-retry dimatikan.
final accountProfileProvider = FutureProvider.autoDispose<ProfileDto>(
  (Ref ref) => ref.watch(accountRepositoryProvider).getProfile(),
  retry: (int retryCount, Object error) => null,
);

/// Bytes foto profil untuk kartu identitas — non-fatal: pengguna tanpa avatar
/// (`has_avatar` false atau 404) dan segala kegagalan lain menghasilkan null,
/// layar jatuh ke inisial nama tanpa pernah memblokir halaman.
final FutureProvider<Uint8List?> accountAvatarProvider =
    FutureProvider.autoDispose<Uint8List?>((Ref ref) async {
      final AuthSession? session = ref.watch(authControllerProvider).value;
      if (session is! Authenticated || !session.user.hasAvatar) {
        return null;
      }
      try {
        return await ref.watch(accountRepositoryProvider).avatar();
      } on AppFailure {
        return null;
      }
    }, retry: (int retryCount, Object error) => null);

/// Nama kantor pengguna untuk kartu identitas, di-resolve non-fatal via
/// [ReferenceLookupRepository] (`GET /offices/{id}`) — lookup gagal berarti
/// null dan baris kantor tidak dirender (pola header Beranda).
final FutureProvider<String?> accountOfficeNameProvider =
    FutureProvider.autoDispose<String?>((Ref ref) async {
      final AuthSession? session = ref.watch(authControllerProvider).value;
      if (session is! Authenticated) {
        return null;
      }
      final String? officeId = session.user.officeId;
      if (officeId == null || officeId.isEmpty) {
        return null;
      }
      return ref.watch(referenceLookupRepositoryProvider).officeName(officeId);
    }, retry: (int retryCount, Object error) => null);
