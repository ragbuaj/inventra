import 'package:flutter/foundation.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../../core/masterdata/reference_lookup_repository.dart';
import '../data/approval_repository.dart';
import '../data/request_detail_dto.dart';

/// Fetch detail pengajuan per id. autoDispose; auto-retry dimatikan (403/404
/// tidak berubah bila diulang, retry lewat tombol "Coba lagi").
final approvalDetailProvider = FutureProvider.autoDispose
    .family<ApprovalDetailData, String>(
      (Ref ref, String id) => ref.watch(approvalRepositoryProvider).detail(id),
      retry: (int retryCount, Object error) => null,
    );

/// Nama referensi ter-resolve untuk payload satu pengajuan. Null berarti tidak
/// ada nilainya ATAU lookup gagal — keduanya dirender em-dash; UUID mentah
/// tidak pernah ditampilkan.
@immutable
class ApprovalReferenceNames {
  const ApprovalReferenceNames({
    this.targetLabel,
    this.payloadNames = const <String, String>{},
  });

  /// Label target pengajuan (`target_entity == 'assets'`): "Nama · TAG".
  final String? targetLabel;

  /// Nama per kunci payload openapi (mis. `to_office_id` -> nama kantor).
  final Map<String, String> payloadNames;

  String? operator [](String key) => payloadNames[key];
}

/// Kunci payload -> jenis lookup master data. Meliputi AssetCreatePayload,
/// TransferPayload, dan DisposalPayload (kontrak backend).
const Map<String, String> _payloadReferenceKinds = <String, String>{
  'office_id': 'office',
  'room_id': 'room',
  'from_office_id': 'office',
  'to_office_id': 'office',
  'to_room_id': 'room',
  'category_id': 'category',
  'brand_id': 'brand',
  'model_id': 'model',
  'vendor_id': 'vendor',
};

/// Resolusi nama referensi payload — provider TERPISAH dari
/// [approvalDetailProvider] supaya layar tampil segera tanpa menunggu lookup
/// (pola sama dengan detail aset). Seluruh lookup paralel dan non-fatal.
final approvalReferenceNamesProvider = FutureProvider.autoDispose
    .family<ApprovalReferenceNames, String>((Ref ref, String id) async {
      final ApprovalDetailData data = await ref.watch(
        approvalDetailProvider(id).future,
      );
      final ReferenceLookupRepository lookup = ref.watch(
        referenceLookupRepositoryProvider,
      );
      final RequestDetailDto request = data.request;
      final Map<String, dynamic> payload =
          request.payload ?? const <String, dynamic>{};

      final List<MapEntry<String, Future<String?>>> lookups =
          <MapEntry<String, Future<String?>>>[];
      for (final MapEntry<String, String> entry
          in _payloadReferenceKinds.entries) {
        final Object? value = payload[entry.key];
        if (value is! String || value.isEmpty) {
          continue;
        }
        lookups.add(
          MapEntry<String, Future<String?>>(entry.key, switch (entry.value) {
            'office' => lookup.officeName(value),
            'room' => lookup.roomLabel(value),
            'category' => lookup.categoryName(value),
            'brand' => lookup.brandName(value),
            'model' => lookup.modelName(value),
            'vendor' => lookup.vendorName(value),
            _ => Future<String?>.value(),
          }),
        );
      }

      final String? targetId = request.targetId;
      final Future<String?> targetFuture =
          request.targetEntity == 'assets' &&
              targetId != null &&
              targetId.isNotEmpty
          ? lookup.assetLabel(targetId)
          : Future<String?>.value();

      final List<String?> resolved = await Future.wait(<Future<String?>>[
        targetFuture,
        ...lookups.map(
          (MapEntry<String, Future<String?>> entry) => entry.value,
        ),
      ]);

      final Map<String, String> names = <String, String>{};
      for (int i = 0; i < lookups.length; i++) {
        final String? name = resolved[i + 1];
        if (name != null) {
          names[lookups[i].key] = name;
        }
      }
      return ApprovalReferenceNames(
        targetLabel: resolved[0],
        payloadNames: Map<String, String>.unmodifiable(names),
      );
    }, retry: (int retryCount, Object error) => null);
