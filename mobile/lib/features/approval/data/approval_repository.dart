import 'package:dio/dio.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../../core/api/dio_provider.dart';
import '../../../core/api/error_mapper.dart';
import 'request_detail_dto.dart';
import 'request_dto.dart';
import 'request_list_dto.dart';

/// Filter status inbox (tab mockup Inbox Approval). Nilai query mengikuti enum
/// `status` openapi (`pending|approved|rejected|cancelled`); [all] berarti
/// tanpa parameter status.
enum ApprovalStatusFilter {
  pending('pending'),
  approved('approved'),
  rejected('rejected'),
  all(null);

  const ApprovalStatusFilter(this.queryValue);

  /// Nilai untuk query `status`; null berarti parameter tidak dikirim.
  final String? queryValue;
}

/// Hasil `GET /requests/{id}`: DTO + himpunan field yang TIDAK dikirim
/// backend. Hanya kunci [requestMaskableKeys] (`amount`/`payload`/`reason`)
/// yang bisa dimask per kontrak; UI merender bagian itu dengan penanda
/// "dibatasi", bukan menebak nilainya.
class ApprovalDetailData {
  const ApprovalDetailData({required this.request, required this.maskedFields});

  final RequestDetailDto request;
  final Set<String> maskedFields;

  /// true bila [field] (kunci snake_case openapi) tidak dikirim backend.
  bool isMasked(String field) => maskedFields.contains(field);
}

/// Repository modul approval (kontrak backend/api/openapi.yaml):
/// `GET /requests` (filter status + limit/offset), `GET /requests/{id}`,
/// `POST /requests/{id}/approve|reject` (catatan opsional — `DecideRequest`),
/// dan `GET /requests/inbox/count` untuk badge.
///
/// Aturan SoD ditegakkan SERVER: maker/approver sebelumnya yang mencoba
/// memutus mendapat 403 ([ForbiddenFailure]); pengajuan yang sudah diputus
/// mendapat 409 ([ConflictFailure]) — klien tidak menduplikasi aturannya.
class ApprovalRepository {
  ApprovalRepository(this._dio);

  final Dio _dio;

  Future<RequestListDto> list({
    required ApprovalStatusFilter filter,
    int offset = 0,
    int limit = 20,
  }) async {
    try {
      final Response<Map<String, dynamic>> response = await _dio
          .get<Map<String, dynamic>>(
            '/requests',
            queryParameters: <String, dynamic>{
              if (filter.queryValue != null) 'status': filter.queryValue,
              'limit': limit,
              'offset': offset,
            },
          );
      return RequestListDto.fromJson(response.data!);
    } on DioException catch (err) {
      throw err.toAppFailure();
    }
  }

  Future<ApprovalDetailData> detail(String id) async {
    try {
      final Response<Map<String, dynamic>> response = await _dio
          .get<Map<String, dynamic>>('/requests/${Uri.encodeComponent(id)}');
      final Map<String, dynamic> json = response.data!;
      return ApprovalDetailData(
        request: RequestDetailDto.fromJson(json),
        maskedFields: requestMaskableKeys
            .where((String key) => !json.containsKey(key))
            .toSet(),
      );
    } on DioException catch (err) {
      throw err.toAppFailure();
    }
  }

  Future<RequestDto> approve(String id, {String? note}) =>
      _decide(id, 'approve', note);

  Future<RequestDto> reject(String id, {String? note}) =>
      _decide(id, 'reject', note);

  /// Jumlah pengajuan yang menunggu keputusan pemanggil
  /// (`GET /requests/inbox/count`, butuh permission `request.decide`).
  /// Melempar [AppFailure] — pemanggil badge memperlakukannya non-fatal.
  Future<int> inboxCount() async {
    try {
      final Response<Map<String, dynamic>> response = await _dio
          .get<Map<String, dynamic>>('/requests/inbox/count');
      return (response.data!['count'] as num).toInt();
    } on DioException catch (err) {
      throw err.toAppFailure();
    }
  }

  /// Body `DecideRequest` hanya dikirim bila ada catatan — kontrak menandai
  /// requestBody opsional untuk approve maupun reject.
  Future<RequestDto> _decide(String id, String action, String? note) async {
    final String? trimmed = note?.trim();
    try {
      final Response<Map<String, dynamic>> response = await _dio
          .post<Map<String, dynamic>>(
            '/requests/${Uri.encodeComponent(id)}/$action',
            data: trimmed == null || trimmed.isEmpty
                ? null
                : <String, dynamic>{'note': trimmed},
          );
      return RequestDto.fromJson(response.data!);
    } on DioException catch (err) {
      throw err.toAppFailure();
    }
  }
}

final Provider<ApprovalRepository> approvalRepositoryProvider =
    Provider<ApprovalRepository>(
      (Ref ref) => ApprovalRepository(ref.watch(dioProvider)),
    );
