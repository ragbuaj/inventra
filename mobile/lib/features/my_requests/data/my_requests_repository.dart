import 'package:dio/dio.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../../core/api/dio_provider.dart';
import '../../../core/api/error_mapper.dart';
import '../../approval/data/approval_repository.dart' show ApprovalStatusFilter;
import '../../approval/data/request_dto.dart';
import '../../approval/data/request_list_dto.dart';

/// Repository "Pengajuan Saya" (lensa MAKER). Memakai `GET /requests?mine=true`
/// — server memfilter ke pengajuan milik pemanggil (user id dari JWT, bukan
/// dari request) dan melewati office scope, jadi maker selalu melihat
/// pengajuannya sendiri. Membatalkan pengajuan `pending` sendiri lewat
/// `POST /requests/:id/cancel` (server menolak bila bukan milik pemanggil atau
/// bukan pending). DTO dipakai bersama modul approval.
class MyRequestsRepository {
  MyRequestsRepository(this._dio);

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
              'mine': 'true',
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

  Future<RequestDto> cancel(String id) async {
    try {
      final Response<Map<String, dynamic>> response = await _dio
          .post<Map<String, dynamic>>(
            '/requests/${Uri.encodeComponent(id)}/cancel',
          );
      return RequestDto.fromJson(response.data!);
    } on DioException catch (err) {
      throw err.toAppFailure();
    }
  }
}

final Provider<MyRequestsRepository> myRequestsRepositoryProvider =
    Provider<MyRequestsRepository>(
      (Ref ref) => MyRequestsRepository(ref.watch(dioProvider)),
    );
