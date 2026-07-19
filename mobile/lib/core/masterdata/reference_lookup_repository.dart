import 'package:dio/dio.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../api/dio_provider.dart';

/// Lookup nama master data per id lewat endpoint get-by-id backend (semuanya
/// auth-only, tanpa permission manage): `GET /offices/{id}`,
/// `/categories/{id}`, `/employees/{id}`, `/brands/{id}`, `/models/{id}`,
/// `/vendors/{id}`, `/rooms/{id}`, `/floors/{id}` — paritas pola web
/// (useResolveCache + lookup non-fatal di halaman detail aset).
///
/// SENGAJA non-fatal: segala kegagalan (offline, 403, 404, bentuk respons tak
/// terduga) menghasilkan null sehingga pemanggil merender em-dash — panggilan
/// suplementer tidak boleh menggagalkan atau memblokir layar (pelajaran repo:
/// supplementary call yang fatal memblokir halaman saat jaringan cegukan).
/// UUID mentah tidak pernah dikembalikan sebagai fallback.
///
/// Nama yang berhasil di-cache di memori dengan TTL singkat supaya membuka
/// beberapa aset dari kantor/kategori yang sama tidak memukul backend per
/// field; kegagalan TIDAK di-cache agar pulihnya jaringan langsung terlihat.
class ReferenceLookupRepository {
  ReferenceLookupRepository(this._dio, {DateTime Function()? now})
    : _now = now ?? DateTime.now;

  final Dio _dio;
  final DateTime Function() _now;

  static const Duration cacheTtl = Duration(minutes: 5);

  final Map<String, _CachedName> _cache = <String, _CachedName>{};

  Future<String?> officeName(String id) => _nameById('offices', id);

  Future<String?> categoryName(String id) => _nameById('categories', id);

  Future<String?> employeeName(String id) => _nameById('employees', id);

  Future<String?> brandName(String id) => _nameById('brands', id);

  Future<String?> modelName(String id) => _nameById('models', id);

  Future<String?> vendorName(String id) => _nameById('vendors', id);

  /// Label aset "Nama · TAG": `GET /assets/{id}` (butuh `asset.view`; gagal
  /// berarti null, konsisten aturan non-fatal kelas ini). Tag dilewati bila
  /// tidak dikirim backend.
  Future<String?> assetLabel(String id) async {
    final String cacheKey = 'asset-label/$id';
    final String? cached = _freshCached(cacheKey);
    if (cached != null) {
      return cached;
    }
    final Map<String, dynamic>? asset = await _getJson('assets', id);
    final Object? name = asset?['name'];
    if (name is! String || name.isEmpty) {
      return null;
    }
    String label = name;
    final Object? tag = asset?['asset_tag'];
    if (tag is String && tag.isNotEmpty) {
      label = '$name · $tag';
    }
    _cache[cacheKey] = _CachedName(label, _now().add(cacheTtl));
    return label;
  }

  /// Label ruangan "Lantai · Ruang": `GET /rooms/{id}` lalu
  /// `GET /floors/{floor_id}`. Lantai gagal di-resolve berarti nama ruangan
  /// saja — kegagalan parsial tidak membatalkan bagian yang berhasil.
  Future<String?> roomLabel(String id) async {
    final String cacheKey = 'room-label/$id';
    final String? cached = _freshCached(cacheKey);
    if (cached != null) {
      return cached;
    }
    final Map<String, dynamic>? room = await _getJson('rooms', id);
    final Object? roomName = room?['name'];
    if (roomName is! String || roomName.isEmpty) {
      return null;
    }
    String label = roomName;
    final Object? floorId = room?['floor_id'];
    if (floorId is String && floorId.isNotEmpty) {
      final String? floorName = await _nameById('floors', floorId);
      if (floorName != null) {
        label = '$floorName · $roomName';
      }
    }
    _cache[cacheKey] = _CachedName(label, _now().add(cacheTtl));
    return label;
  }

  Future<String?> _nameById(String path, String id) async {
    final String cacheKey = '$path/$id';
    final String? cached = _freshCached(cacheKey);
    if (cached != null) {
      return cached;
    }
    final Map<String, dynamic>? json = await _getJson(path, id);
    final Object? name = json?['name'];
    if (name is! String || name.isEmpty) {
      return null;
    }
    _cache[cacheKey] = _CachedName(name, _now().add(cacheTtl));
    return name;
  }

  Future<Map<String, dynamic>?> _getJson(String path, String id) async {
    try {
      final Response<Map<String, dynamic>> response = await _dio
          .get<Map<String, dynamic>>('/$path/${Uri.encodeComponent(id)}');
      return response.data;
    } on DioException {
      // Non-fatal by design — lihat doc kelas.
      return null;
    }
  }

  /// Mengosongkan seluruh cache nama in-memory. Dipanggil saat sesi berakhir
  /// (logout atau sesi mati) supaya nama master data user lama tidak bocor ke
  /// user berikutnya yang login di perangkat yang sama.
  void clear() => _cache.clear();

  String? _freshCached(String key) {
    final _CachedName? entry = _cache[key];
    if (entry == null) {
      return null;
    }
    if (_now().isAfter(entry.expiresAt)) {
      _cache.remove(key);
      return null;
    }
    return entry.name;
  }
}

class _CachedName {
  const _CachedName(this.name, this.expiresAt);

  final String name;
  final DateTime expiresAt;
}

final Provider<ReferenceLookupRepository> referenceLookupRepositoryProvider =
    Provider<ReferenceLookupRepository>(
      (Ref ref) => ReferenceLookupRepository(ref.watch(dioProvider)),
    );
