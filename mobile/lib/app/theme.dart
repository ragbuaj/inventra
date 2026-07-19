import 'package:flutter/material.dart';

/// Tema Material 3 Inventra Mobile — light + dark dari token mockup
/// (docs/mobile/design). Widget tidak boleh memakai `Color(0xFF...)` literal;
/// semua warna diambil dari [ColorScheme], [InventraStatusColors], atau
/// konstanta di file ini (CONVENTIONS.md bagian 3).
abstract final class InventraTheme {
  static ThemeData get light => _build(_LightTokens());

  static ThemeData get dark => _build(_DarkTokens());

  static ThemeData _build(_Tokens t) {
    final ThemeData base = ThemeData(
      useMaterial3: true,
      brightness: t.brightness,
      fontFamily: 'Inter',
      colorScheme: t.colorScheme,
      scaffoldBackgroundColor: t.scaffoldBackground,
    );

    return base.copyWith(
      textTheme: _textTheme(base.textTheme, t),
      extensions: <ThemeExtension<dynamic>>[t.statusColors],
      appBarTheme: AppBarTheme(
        backgroundColor: t.scaffoldBackground,
        foregroundColor: t.ink,
        elevation: 0,
        scrolledUnderElevation: 0,
        centerTitle: false,
        titleTextStyle: TextStyle(
          fontFamily: 'Inter',
          fontSize: 18,
          fontWeight: FontWeight.w700,
          letterSpacing: 18 * InventraDimens.titleLetterSpacingEm,
          color: t.ink,
        ),
      ),
      cardTheme: CardThemeData(
        color: t.card,
        elevation: 0,
        shape: RoundedRectangleBorder(
          borderRadius: BorderRadius.circular(InventraDimens.radiusCardSmall),
          side: BorderSide(color: t.border),
        ),
        margin: EdgeInsets.zero,
      ),
      dividerTheme: DividerThemeData(color: t.border, thickness: 1, space: 1),
      inputDecorationTheme: InputDecorationTheme(
        filled: true,
        fillColor: t.card,
        contentPadding: const EdgeInsets.symmetric(
          horizontal: 16,
          vertical: 14,
        ),
        hintStyle: TextStyle(color: t.textMuted),
        labelStyle: TextStyle(color: t.textLabel),
        enabledBorder: OutlineInputBorder(
          borderRadius: BorderRadius.circular(InventraDimens.radiusInput),
          borderSide: BorderSide(color: t.inputBorder),
        ),
        focusedBorder: OutlineInputBorder(
          borderRadius: BorderRadius.circular(InventraDimens.radiusInput),
          borderSide: BorderSide(color: t.colorScheme.primary, width: 2),
        ),
        errorBorder: OutlineInputBorder(
          borderRadius: BorderRadius.circular(InventraDimens.radiusInput),
          borderSide: BorderSide(color: t.colorScheme.error),
        ),
        focusedErrorBorder: OutlineInputBorder(
          borderRadius: BorderRadius.circular(InventraDimens.radiusInput),
          borderSide: BorderSide(color: t.colorScheme.error, width: 2),
        ),
      ),
      filledButtonTheme: FilledButtonThemeData(
        style: _buttonStyle(height: InventraDimens.buttonHeightPrimary),
      ),
      elevatedButtonTheme: ElevatedButtonThemeData(
        style: _buttonStyle(height: InventraDimens.buttonHeightPrimary),
      ),
      outlinedButtonTheme: OutlinedButtonThemeData(
        style: _buttonStyle(height: InventraDimens.buttonHeightStandard),
      ),
      textButtonTheme: TextButtonThemeData(
        style: _buttonStyle(height: InventraDimens.buttonHeightStandard),
      ),
      chipTheme: ChipThemeData(
        shape: const StadiumBorder(),
        side: BorderSide(color: t.border),
        labelStyle: TextStyle(
          fontFamily: 'Inter',
          fontSize: 12,
          fontWeight: FontWeight.w500,
          color: t.textLabel,
        ),
      ),
    );
  }

  static ButtonStyle _buttonStyle({required double height}) {
    return ButtonStyle(
      minimumSize: WidgetStatePropertyAll<Size>(Size(64, height)),
      shape: WidgetStatePropertyAll<OutlinedBorder>(
        RoundedRectangleBorder(
          borderRadius: BorderRadius.circular(InventraDimens.radiusButton),
        ),
      ),
      textStyle: const WidgetStatePropertyAll<TextStyle>(
        TextStyle(
          fontFamily: 'Inter',
          fontSize: 16,
          fontWeight: FontWeight.w600,
        ),
      ),
    );
  }

  static TextTheme _textTheme(TextTheme base, _Tokens t) {
    TextStyle title(TextStyle? s) => (s ?? const TextStyle()).copyWith(
      fontWeight: FontWeight.w700,
      letterSpacing: (s?.fontSize ?? 14) * InventraDimens.titleLetterSpacingEm,
      color: t.ink,
    );

    return base
        .copyWith(
          headlineMedium: title(base.headlineMedium),
          headlineSmall: title(base.headlineSmall),
          titleLarge: title(base.titleLarge),
          titleMedium: title(base.titleMedium),
          titleSmall: title(base.titleSmall),
          bodyLarge: base.bodyLarge?.copyWith(color: t.ink),
          bodyMedium: base.bodyMedium?.copyWith(color: t.ink),
          bodySmall: base.bodySmall?.copyWith(color: t.textSecondary),
          labelLarge: base.labelLarge?.copyWith(
            color: t.textLabel,
            fontWeight: FontWeight.w600,
          ),
          labelMedium: base.labelMedium?.copyWith(color: t.textLabel),
          labelSmall: base.labelSmall?.copyWith(color: t.textMuted),
        )
        .apply(fontFamily: 'Inter');
  }
}

/// Dimensi bersama dari mockup: radius, tinggi tombol, letter-spacing judul.
abstract final class InventraDimens {
  /// Radius field input dan tombol.
  static const double radiusInput = 14;
  static const double radiusButton = 14;

  /// Radius card kecil (list item, tile).
  static const double radiusCardSmall = 16;

  /// Radius card utama (hero card, sheet) — mockup memakai 20-24.
  static const double radiusCardMain = 20;
  static const double radiusCardHero = 24;

  /// Chip memakai radius penuh ([StadiumBorder]).
  static const double buttonHeightPrimary = 52;
  static const double buttonHeightStandard = 48;

  /// Judul: weight 700, letter-spacing -0.02 em (dikali fontSize).
  static const double titleLetterSpacingEm = -0.02;
}

/// Warna layar Scan dari mockup "Inventra Mobile - Scan": viewfinder gelap di
/// KEDUA tema (permukaan kamera tidak mengikuti light/dark), sehingga nilainya
/// konstan di sini, bukan dari [ColorScheme].
abstract final class InventraScanColors {
  /// Latar viewfinder saat kamera belum/tidak menampilkan frame.
  static const Color viewfinderBackground = Color(0xFF090C11);

  /// Latar pill kontrol (tutup/torch) dan pill petunjuk: slate-900 ~55%.
  static const Color controlBackground = Color(0x8C0F172A);

  /// Latar pill tombol "Ketik kode manual": slate-900 ~60%.
  static const Color manualButtonBackground = Color(0x990F172A);

  /// Border tipis pill tombol manual: putih 16%.
  static const Color manualButtonBorder = Color(0x29FFFFFF);

  /// Sudut bingkai target + garis scan: green-400 (sama di kedua tema).
  static const Color frameAccent = Color(0xFF4ADE80);

  /// Teks/ikon di atas viewfinder.
  static const Color foreground = Color(0xFFFFFFFF);

  /// Teks petunjuk di atas viewfinder: putih 85%.
  static const Color foregroundMuted = Color(0xD9FFFFFF);
}

/// Warna thumbnail pemilih tema layar Pengaturan (mockup "Pemilih tema"):
/// tiap tile menggambarkan tema Terang/Gelap apa adanya dan TIDAK mengikuti
/// tema aktif, sehingga nilainya konstan di sini (pola [InventraScanColors]).
abstract final class InventraThemePreviewColors {
  static const Color lightBackground = Color(0xFFF8FAFC);
  static const Color lightSurface = Color(0xFFFFFFFF);
  static const Color lightBorder = Color(0xFFE2E8F0);
  static const Color lightAccent = Color(0xFFDCFCE7);
  static const Color lightBlock = Color(0xFFE2E8F0);

  static const Color darkBackground = Color(0xFF0F172A);
  static const Color darkSurface = Color(0xFF1E293B);
  static const Color darkBorder = Color(0xFF334155);
  static const Color darkAccent = Color(0xFF14532D);
  static const Color darkBlock = Color(0xFF334155);
}

/// Satu triplet warna chip status: titik indikator, latar, dan teks.
@immutable
class StatusColorSet {
  const StatusColorSet({
    required this.dot,
    required this.bg,
    required this.text,
  });

  final Color dot;
  final Color bg;
  final Color text;

  StatusColorSet lerpTo(StatusColorSet other, double t) => StatusColorSet(
    dot: Color.lerp(dot, other.dot, t) ?? other.dot,
    bg: Color.lerp(bg, other.bg, t) ?? other.bg,
    text: Color.lerp(text, other.text, t) ?? other.text,
  );
}

/// Warna chip status domain sebagai [ThemeExtension].
///
/// Lima keluarga semantik (success/info/warning/danger/neutral) dipetakan ke
/// status aset lewat getter; hasil opname dan status pengajuan memakai
/// keluarga semantik yang sama (mis. pengajuan pending = [warning],
/// disetujui = [success], ditolak = [danger]).
@immutable
class InventraStatusColors extends ThemeExtension<InventraStatusColors> {
  const InventraStatusColors({
    required this.success,
    required this.info,
    required this.warning,
    required this.danger,
    required this.neutral,
  });

  final StatusColorSet success;
  final StatusColorSet info;
  final StatusColorSet warning;
  final StatusColorSet danger;
  final StatusColorSet neutral;

  /// Aset tersedia — hijau.
  StatusColorSet get assetAvailable => success;

  /// Aset dipinjam — biru.
  StatusColorSet get assetBorrowed => info;

  /// Aset maintenance — amber.
  StatusColorSet get assetMaintenance => warning;

  /// Aset dilepas — slate.
  StatusColorSet get assetDisposed => neutral;

  /// Aset hilang — merah.
  StatusColorSet get assetLost => danger;

  static const InventraStatusColors light = InventraStatusColors(
    success: StatusColorSet(
      dot: Color(0xFF16A34A),
      bg: Color(0xFFDCFCE7),
      text: Color(0xFF15803D),
    ),
    info: StatusColorSet(
      dot: Color(0xFF2563EB),
      bg: Color(0xFFDBEAFE),
      text: Color(0xFF1D4ED8),
    ),
    warning: StatusColorSet(
      dot: Color(0xFFD97706),
      bg: Color(0xFFFEF3C7),
      text: Color(0xFFB45309),
    ),
    danger: StatusColorSet(
      dot: Color(0xFFDC2626),
      bg: Color(0xFFFEE2E2),
      text: Color(0xFFB91C1C),
    ),
    neutral: StatusColorSet(
      dot: Color(0xFF64748B),
      bg: Color(0xFFF1F5F9),
      text: Color(0xFF475569),
    ),
  );

  static const InventraStatusColors dark = InventraStatusColors(
    success: StatusColorSet(
      dot: Color(0xFF22C55E),
      bg: Color(0xFF14532D),
      text: Color(0xFF86EFAC),
    ),
    info: StatusColorSet(
      dot: Color(0xFF60A5FA),
      bg: Color(0xFF1E3A5F),
      text: Color(0xFF93C5FD),
    ),
    warning: StatusColorSet(
      dot: Color(0xFFF59E0B),
      bg: Color(0xFF422006),
      text: Color(0xFFFCD34D),
    ),
    danger: StatusColorSet(
      dot: Color(0xFFEF4444),
      bg: Color(0xFF450A0A),
      text: Color(0xFFFCA5A5),
    ),
    neutral: StatusColorSet(
      dot: Color(0xFF94A3B8),
      bg: Color(0xFF334155),
      text: Color(0xFFCBD5E1),
    ),
  );

  @override
  InventraStatusColors copyWith({
    StatusColorSet? success,
    StatusColorSet? info,
    StatusColorSet? warning,
    StatusColorSet? danger,
    StatusColorSet? neutral,
  }) {
    return InventraStatusColors(
      success: success ?? this.success,
      info: info ?? this.info,
      warning: warning ?? this.warning,
      danger: danger ?? this.danger,
      neutral: neutral ?? this.neutral,
    );
  }

  @override
  InventraStatusColors lerp(
    covariant ThemeExtension<InventraStatusColors>? other,
    double t,
  ) {
    if (other is! InventraStatusColors) {
      return this;
    }
    return InventraStatusColors(
      success: success.lerpTo(other.success, t),
      info: info.lerpTo(other.info, t),
      warning: warning.lerpTo(other.warning, t),
      danger: danger.lerpTo(other.danger, t),
      neutral: neutral.lerpTo(other.neutral, t),
    );
  }
}

/// Token internal per-brightness — sumber tunggal nilai warna mockup.
sealed class _Tokens {
  Brightness get brightness;
  ColorScheme get colorScheme;
  InventraStatusColors get statusColors;

  /// Latar scaffold.
  Color get scaffoldBackground;

  /// Latar card / permukaan naik.
  Color get card;

  /// Teks utama (ink).
  Color get ink;

  Color get textSecondary;
  Color get textLabel;
  Color get textMuted;

  /// Border umum (card, divider).
  Color get border;

  /// Border field input.
  Color get inputBorder;
}

final class _LightTokens extends _Tokens {
  @override
  Brightness get brightness => Brightness.light;

  @override
  Color get scaffoldBackground => const Color(0xFFF8FAFC);

  @override
  Color get card => const Color(0xFFFFFFFF);

  @override
  Color get ink => const Color(0xFF0F172A);

  @override
  Color get textSecondary => const Color(0xFF64748B);

  @override
  Color get textLabel => const Color(0xFF475569);

  @override
  Color get textMuted => const Color(0xFF94A3B8);

  @override
  Color get border => const Color(0xFFE2E8F0);

  @override
  Color get inputBorder => const Color(0xFFCBD5E1);

  @override
  InventraStatusColors get statusColors => InventraStatusColors.light;

  @override
  ColorScheme get colorScheme => ColorScheme(
    brightness: Brightness.light,
    primary: const Color(0xFF16A34A),
    onPrimary: const Color(0xFFFFFFFF),
    primaryContainer: const Color(0xFFDCFCE7),
    // Shade hover/gelap primary dari mockup.
    onPrimaryContainer: const Color(0xFF15803D),
    secondary: textLabel,
    onSecondary: const Color(0xFFFFFFFF),
    secondaryContainer: const Color(0xFFF1F5F9),
    onSecondaryContainer: textLabel,
    tertiary: const Color(0xFF2563EB),
    onTertiary: const Color(0xFFFFFFFF),
    tertiaryContainer: const Color(0xFFDBEAFE),
    onTertiaryContainer: const Color(0xFF1D4ED8),
    error: const Color(0xFFDC2626),
    onError: const Color(0xFFFFFFFF),
    errorContainer: const Color(0xFFFEE2E2),
    onErrorContainer: const Color(0xFFB91C1C),
    surface: card,
    onSurface: ink,
    onSurfaceVariant: textSecondary,
    outline: inputBorder,
    outlineVariant: border,
    surfaceContainerLowest: card,
    surfaceContainerLow: scaffoldBackground,
    surfaceContainer: scaffoldBackground,
  );
}

final class _DarkTokens extends _Tokens {
  @override
  Brightness get brightness => Brightness.dark;

  @override
  Color get scaffoldBackground => const Color(0xFF0F172A);

  @override
  Color get card => const Color(0xFF1E293B);

  @override
  Color get ink => const Color(0xFFF1F5F9);

  @override
  Color get textSecondary => const Color(0xFF94A3B8);

  @override
  Color get textLabel => const Color(0xFF94A3B8);

  @override
  Color get textMuted => const Color(0xFF64748B);

  @override
  Color get border => const Color(0xFF334155);

  @override
  Color get inputBorder => const Color(0xFF475569);

  @override
  InventraStatusColors get statusColors => InventraStatusColors.dark;

  @override
  ColorScheme get colorScheme => ColorScheme(
    brightness: Brightness.dark,
    primary: const Color(0xFF22C55E),
    onPrimary: const Color(0xFF052E16),
    primaryContainer: const Color(0xFF14532D),
    // Aksen hijau terang dari mockup dark.
    onPrimaryContainer: const Color(0xFF4ADE80),
    secondary: textSecondary,
    onSecondary: scaffoldBackground,
    secondaryContainer: border,
    onSecondaryContainer: const Color(0xFFCBD5E1),
    tertiary: const Color(0xFF60A5FA),
    onTertiary: scaffoldBackground,
    tertiaryContainer: const Color(0xFF1E3A5F),
    onTertiaryContainer: const Color(0xFF93C5FD),
    error: const Color(0xFFEF4444),
    onError: const Color(0xFF450A0A),
    errorContainer: const Color(0xFF450A0A),
    onErrorContainer: const Color(0xFFFCA5A5),
    surface: scaffoldBackground,
    onSurface: ink,
    onSurfaceVariant: textSecondary,
    outline: inputBorder,
    outlineVariant: border,
    surfaceContainerLowest: scaffoldBackground,
    surfaceContainerLow: card,
    surfaceContainer: card,
  );
}
