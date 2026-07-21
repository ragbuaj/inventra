/// Profil lengkap pemanggil (`GET /auth/profile` -> `ProfileView` backend).
/// Field pegawai (kode/status/departemen/jabatan) null bila akun tak tertaut
/// pegawai. `password_hash`/`google_id` tidak pernah diserialisasi server.
class ProfileDto {
  const ProfileDto({
    required this.id,
    required this.name,
    required this.email,
    this.phone,
    this.roleName,
    this.officeName,
    this.employeeName,
    this.employeeCode,
    this.employeeStatus,
    this.departmentName,
    this.positionName,
    this.hasAvatar = false,
    this.googleLinked = false,
    this.joinedAt,
  });

  final String id;
  final String name;
  final String email;
  final String? phone;
  final String? roleName;
  final String? officeName;
  final String? employeeName;
  final String? employeeCode;
  final String? employeeStatus;
  final String? departmentName;
  final String? positionName;
  final bool hasAvatar;
  final bool googleLinked;
  final DateTime? joinedAt;

  /// True bila akun tertaut ke pegawai (ada detail pegawai untuk dirender).
  bool get hasEmployee => employeeCode != null || employeeName != null;

  ProfileDto copyWith({String? name, String? phone}) {
    return ProfileDto(
      id: id,
      name: name ?? this.name,
      email: email,
      phone: phone ?? this.phone,
      roleName: roleName,
      officeName: officeName,
      employeeName: employeeName,
      employeeCode: employeeCode,
      employeeStatus: employeeStatus,
      departmentName: departmentName,
      positionName: positionName,
      hasAvatar: hasAvatar,
      googleLinked: googleLinked,
      joinedAt: joinedAt,
    );
  }

  factory ProfileDto.fromJson(Map<String, dynamic> json) {
    String? str(String key) {
      final Object? v = json[key];
      return v is String && v.isNotEmpty ? v : null;
    }

    DateTime? joined;
    final Object? j = json['joined_at'];
    if (j is String) {
      joined = DateTime.tryParse(j);
    }

    return ProfileDto(
      id: (json['id'] as String?) ?? '',
      name: (json['name'] as String?) ?? '',
      email: (json['email'] as String?) ?? '',
      phone: str('phone'),
      roleName: str('role_name'),
      officeName: str('office_name'),
      employeeName: str('employee_name'),
      employeeCode: str('employee_code'),
      employeeStatus: str('employee_status'),
      departmentName: str('department_name'),
      positionName: str('position_name'),
      hasAvatar: json['has_avatar'] == true,
      googleLinked: json['google_linked'] == true,
      joinedAt: joined,
    );
  }
}
