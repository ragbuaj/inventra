import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../data/asset_detail_repository.dart';

/// Fetch detail aset per tag. autoDispose: hasil dibuang saat layar ditutup
/// sehingga scan tag yang sama selalu mengambil data segar; retry lewat
/// `ref.invalidate(assetByTagProvider(tag))`.
///
/// Auto-retry bawaan Riverpod 3 dimatikan: 404/403 tidak akan berubah bila
/// diulang (justru menghambur request ke backend), dan pengguna sudah punya
/// tombol "Coba lagi" eksplisit untuk kegagalan jaringan.
final assetByTagProvider = FutureProvider.autoDispose
    .family<AssetDetailData, String>(
      (Ref ref, String tag) =>
          ref.watch(assetDetailRepositoryProvider).getByTag(tag),
      retry: (int retryCount, Object error) => null,
    );
