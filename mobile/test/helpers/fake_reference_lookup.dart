import 'package:inventra_mobile/core/masterdata/reference_lookup_repository.dart';

/// [ReferenceLookupRepository] stub untuk widget/golden test: nama diambil
/// dari peta statis berkunci `<jenis>:<id>`; id yang tidak terdaftar
/// menghasilkan null (perilaku lookup gagal non-fatal).
class FakeReferenceLookup implements ReferenceLookupRepository {
  FakeReferenceLookup([Map<String, String>? names])
    : names = names ?? <String, String>{};

  final Map<String, String> names;
  int callCount = 0;

  Future<String?> _get(String kind, String id) async {
    callCount += 1;
    return names['$kind:$id'];
  }

  @override
  Future<String?> officeName(String id) => _get('office', id);

  @override
  Future<String?> categoryName(String id) => _get('category', id);

  @override
  Future<String?> employeeName(String id) => _get('employee', id);

  @override
  Future<String?> brandName(String id) => _get('brand', id);

  @override
  Future<String?> modelName(String id) => _get('model', id);

  @override
  Future<String?> vendorName(String id) => _get('vendor', id);

  @override
  Future<String?> roomLabel(String id) => _get('room', id);
}
