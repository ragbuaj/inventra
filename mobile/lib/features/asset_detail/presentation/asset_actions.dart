/// Aksi FR-M7 pada Detail Aset. Ketersediaan ditentukan oleh permission
/// pemanggil DAN status aset; server tetap menegakkan otorisasi sebenarnya.
enum AssetAction { borrow, checkout, checkin, reportDamage }

/// Aksi yang boleh tampil untuk (permissions x status aset), sesuai matriks
/// FR-M7.2/M7.3:
/// - aset `available` + `assignment.manage` -> Check-out (langsung);
///   selain itu + `request.create` -> Pinjam (ajukan peminjaman).
/// - aset `assigned` + `assignment.manage` -> Check-in.
/// - status apa pun + `request.create` -> Lapor Kerusakan.
/// Urutan hasil = urutan tampil pada bar aksi.
List<AssetAction> assetActionsFor(Set<String> permissions, String? status) {
  final bool canManage = permissions.contains('assignment.manage');
  final bool canCreate = permissions.contains('request.create');

  final List<AssetAction> actions = <AssetAction>[];
  switch (status) {
    case 'available':
      if (canManage) {
        actions.add(AssetAction.checkout);
      } else if (canCreate) {
        actions.add(AssetAction.borrow);
      }
    case 'assigned':
      if (canManage) {
        actions.add(AssetAction.checkin);
      }
  }
  if (canCreate) {
    actions.add(AssetAction.reportDamage);
  }
  return actions;
}
