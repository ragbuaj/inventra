import 'package:flutter/foundation.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../../core/masterdata/reference_lookup_repository.dart';
import '../data/asset_detail_repository.dart';
import '../data/asset_dto.dart';
import 'asset_by_tag_provider.dart';

/// Nama referensi hasil resolusi master data untuk satu aset. Null berarti
/// tidak ada nilainya ATAU lookup gagal — keduanya dirender em-dash; UUID
/// mentah tidak pernah ditampilkan ke pengguna.
@immutable
class AssetReferenceNames {
  const AssetReferenceNames({
    this.officeName,
    this.roomLabel,
    this.holderName,
    this.categoryName,
    this.brandName,
    this.modelName,
    this.vendorName,
  });

  final String? officeName;
  final String? roomLabel;
  final String? holderName;
  final String? categoryName;
  final String? brandName;
  final String? modelName;
  final String? vendorName;
}

/// Resolusi nama referensi aset — provider TERPISAH dari [assetByTagProvider]
/// supaya layar detail tampil segera tanpa menunggu lookup (paritas web:
/// nilai muncul "—" lalu terisi begitu resolusi selesai). Seluruh lookup
/// berjalan paralel dan non-fatal; field yang dimask field-permission bernilai
/// null di DTO sehingga tidak pernah di-lookup.
final assetReferenceNamesProvider = FutureProvider.autoDispose
    .family<AssetReferenceNames, String>((Ref ref, String tag) async {
      final AssetDetailData data = await ref.watch(
        assetByTagProvider(tag).future,
      );
      final ReferenceLookupRepository lookup = ref.watch(
        referenceLookupRepositoryProvider,
      );
      final AssetDto asset = data.asset;

      Future<String?> resolve(
        String? id,
        Future<String?> Function(String id) fn,
      ) => id == null || id.isEmpty ? Future<String?>.value() : fn(id);

      final List<String?> names = await Future.wait(<Future<String?>>[
        resolve(asset.officeId, lookup.officeName),
        resolve(asset.roomId, lookup.roomLabel),
        resolve(asset.currentHolderEmployeeId, lookup.employeeName),
        resolve(asset.categoryId, lookup.categoryName),
        resolve(asset.brandId, lookup.brandName),
        resolve(asset.modelId, lookup.modelName),
        resolve(asset.vendorId, lookup.vendorName),
      ]);
      return AssetReferenceNames(
        officeName: names[0],
        roomLabel: names[1],
        holderName: names[2],
        categoryName: names[3],
        brandName: names[4],
        modelName: names[5],
        vendorName: names[6],
      );
    }, retry: (int retryCount, Object error) => null);
