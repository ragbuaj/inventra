import 'package:flutter/foundation.dart';

/// Kueri katalog: pencarian + filter kategori/status/kantor. Nilai null berarti
/// filter tidak aktif. Dipakai sebagai argumen family [CatalogController] —
/// == / hashCode menentukan kapan controller dibuat ulang (memuat halaman 0).
@immutable
class CatalogQuery {
  const CatalogQuery({
    this.search,
    this.categoryId,
    this.status,
    this.officeId,
  });

  final String? search;
  final String? categoryId;
  final String? status;
  final String? officeId;

  bool get hasFilters =>
      search != null ||
      categoryId != null ||
      status != null ||
      officeId != null;

  static const Object _unset = Object();

  /// copyWith yang membedakan "tidak diubah" dari "diset ke null" (untuk
  /// mengosongkan sebuah filter): argumen yang dibiarkan default mempertahankan
  /// nilai lama, meneruskan null secara eksplisit menghapus filter itu.
  CatalogQuery copyWith({
    Object? search = _unset,
    Object? categoryId = _unset,
    Object? status = _unset,
    Object? officeId = _unset,
  }) {
    return CatalogQuery(
      search: identical(search, _unset) ? this.search : search as String?,
      categoryId: identical(categoryId, _unset)
          ? this.categoryId
          : categoryId as String?,
      status: identical(status, _unset) ? this.status : status as String?,
      officeId: identical(officeId, _unset)
          ? this.officeId
          : officeId as String?,
    );
  }

  @override
  bool operator ==(Object other) =>
      other is CatalogQuery &&
      other.search == search &&
      other.categoryId == categoryId &&
      other.status == status &&
      other.officeId == officeId;

  @override
  int get hashCode => Object.hash(search, categoryId, status, officeId);
}
