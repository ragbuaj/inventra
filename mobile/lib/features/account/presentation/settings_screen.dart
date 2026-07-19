import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:material_symbols_icons/symbols.dart';

import '../../../app/locale_controller.dart';
import '../../../app/theme.dart';
import '../../../app/theme_mode_controller.dart';
import '../../../core/i18n/gen/app_localizations.dart';
import '../../../core/utils/app_info.dart';

/// Layar Pengaturan 1:1 mockup "Inventra Mobile - Pengaturan": kartu Tampilan
/// (Tema — bottom sheet Terang/Gelap/Ikuti Sistem dengan pratinjau, Bahasa —
/// bottom sheet Indonesia/English) dan kartu Tentang (versi aplikasi).
/// Preferensi persist via SharedPreferences (non-sensitif) dan langsung
/// berefek pada seluruh aplikasi.
///
/// Deviasi tercatat: kartu Notifikasi push, Penyimpanan (data opname lokal),
/// dan tautan Bantuan pada mockup tidak dirender — push, drift lokal, dan
/// runbook belum ada di M0 (menyusul bersama fase M5/notifikasi push).
class SettingsScreen extends ConsumerWidget {
  const SettingsScreen({super.key});

  String _themeLabel(AppLocalizations l10n, ThemeMode mode) {
    return switch (mode) {
      ThemeMode.light => l10n.settingsThemeLight,
      ThemeMode.dark => l10n.settingsThemeDark,
      ThemeMode.system => l10n.settingsThemeSystem,
    };
  }

  String _languageLabel(AppLocalizations l10n, String languageCode) {
    return languageCode == 'en'
        ? l10n.settingsLanguageEnglish
        : l10n.settingsLanguageIndonesian;
  }

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final AppLocalizations l10n = AppLocalizations.of(context);
    final ThemeMode themeMode = ref.watch(themeModeControllerProvider);
    final String languageCode = Localizations.localeOf(context).languageCode;

    return Scaffold(
      appBar: AppBar(title: Text(l10n.settingsTitle)),
      body: SafeArea(
        child: ListView(
          padding: const EdgeInsets.fromLTRB(20, 4, 20, 24),
          children: <Widget>[
            _SettingsCard(
              title: l10n.settingsSectionAppearance,
              children: <Widget>[
                _SettingsRow(
                  rowKey: 'settings-theme',
                  icon: Symbols.contrast_rounded,
                  title: l10n.settingsTheme,
                  subtitle: _themeLabel(l10n, themeMode),
                  onTap: () => _showThemeSheet(context, ref, themeMode),
                ),
                const Divider(height: 25),
                _SettingsRow(
                  rowKey: 'settings-language',
                  icon: Symbols.language_rounded,
                  title: l10n.settingsLanguage,
                  subtitle: _languageLabel(l10n, languageCode),
                  onTap: () => _showLanguageSheet(context, ref, languageCode),
                ),
              ],
            ),
            const SizedBox(height: 13),
            _SettingsCard(
              title: l10n.settingsSectionAbout,
              children: <Widget>[
                _SettingsRow(
                  rowKey: 'settings-about',
                  icon: Symbols.inventory_2_rounded,
                  iconVariantSuccess: true,
                  title: l10n.settingsAppName,
                  subtitle: l10n.settingsVersion(
                    AppInfo.version,
                    AppInfo.buildNumber,
                  ),
                ),
              ],
            ),
          ],
        ),
      ),
    );
  }

  Future<void> _showThemeSheet(
    BuildContext context,
    WidgetRef ref,
    ThemeMode current,
  ) async {
    final ThemeMode? applied = await showModalBottomSheet<ThemeMode>(
      context: context,
      backgroundColor:
          Theme.of(context).cardTheme.color ??
          Theme.of(context).colorScheme.surface,
      shape: const RoundedRectangleBorder(
        borderRadius: BorderRadius.vertical(top: Radius.circular(28)),
      ),
      builder: (BuildContext sheetContext) => _ThemeSheet(initial: current),
    );
    if (applied != null) {
      ref.read(themeModeControllerProvider.notifier).setMode(applied);
    }
  }

  Future<void> _showLanguageSheet(
    BuildContext context,
    WidgetRef ref,
    String currentCode,
  ) async {
    final Locale? picked = await showModalBottomSheet<Locale>(
      context: context,
      backgroundColor:
          Theme.of(context).cardTheme.color ??
          Theme.of(context).colorScheme.surface,
      shape: const RoundedRectangleBorder(
        borderRadius: BorderRadius.vertical(top: Radius.circular(28)),
      ),
      builder: (BuildContext sheetContext) =>
          _LanguageSheet(currentCode: currentCode),
    );
    if (picked != null) {
      ref.read(localeControllerProvider.notifier).setLocale(picked);
    }
  }
}

/// Kerangka kartu seksi Pengaturan: judul kecil uppercase + baris-baris.
class _SettingsCard extends StatelessWidget {
  const _SettingsCard({required this.title, required this.children});

  final String title;
  final List<Widget> children;

  @override
  Widget build(BuildContext context) {
    final ThemeData theme = Theme.of(context);

    return Container(
      width: double.infinity,
      padding: const EdgeInsets.fromLTRB(16, 15, 16, 15),
      decoration: BoxDecoration(
        color: theme.cardTheme.color ?? theme.colorScheme.surface,
        borderRadius: BorderRadius.circular(18),
        border: Border.all(color: theme.colorScheme.outlineVariant),
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: <Widget>[
          Text(
            title.toUpperCase(),
            style: TextStyle(
              fontSize: 12,
              fontWeight: FontWeight.w700,
              letterSpacing: 0.6,
              color: theme.textTheme.bodySmall?.color,
            ),
          ),
          const SizedBox(height: 12),
          ...children,
        ],
      ),
    );
  }
}

/// Satu baris pengaturan 1:1 mockup: tile ikon 38, judul + nilai saat ini,
/// chevron bila bisa di-tap.
class _SettingsRow extends StatelessWidget {
  const _SettingsRow({
    required this.rowKey,
    required this.icon,
    required this.title,
    required this.subtitle,
    this.iconVariantSuccess = false,
    this.onTap,
  });

  final String rowKey;
  final IconData icon;
  final String title;
  final String subtitle;

  /// Tile hijau (mockup baris "Inventra Mobile"); default tile netral.
  final bool iconVariantSuccess;

  final VoidCallback? onTap;

  @override
  Widget build(BuildContext context) {
    final ThemeData theme = Theme.of(context);
    final ColorScheme scheme = theme.colorScheme;
    final InventraStatusColors colors = theme
        .extension<InventraStatusColors>()!;
    final StatusColorSet tile = iconVariantSuccess
        ? colors.success
        : colors.neutral;

    final Widget row = Row(
      children: <Widget>[
        Container(
          width: 38,
          height: 38,
          decoration: BoxDecoration(
            color: tile.bg,
            borderRadius: BorderRadius.circular(11),
          ),
          child: Icon(icon, size: 19, color: tile.text),
        ),
        const SizedBox(width: 12),
        Expanded(
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: <Widget>[
              Text(
                title,
                style: TextStyle(
                  fontSize: 13.5,
                  fontWeight: FontWeight.w600,
                  color: scheme.onSurface,
                ),
              ),
              Text(
                subtitle,
                style: TextStyle(
                  fontSize: 11.5,
                  color: theme.textTheme.labelSmall?.color,
                ),
              ),
            ],
          ),
        ),
        if (onTap != null)
          Icon(Symbols.chevron_right_rounded, size: 20, color: scheme.outline),
      ],
    );

    if (onTap == null) {
      return KeyedSubtree(key: ValueKey<String>(rowKey), child: row);
    }
    return InkWell(
      key: ValueKey<String>(rowKey),
      borderRadius: BorderRadius.circular(11),
      onTap: onTap,
      child: row,
    );
  }
}

/// Bottom sheet pemilih tema 1:1 mockup: tiga tile pratinjau (Terang/Gelap/
/// Ikuti Sistem) + tombol Terapkan; pilihan dikembalikan lewat pop.
class _ThemeSheet extends StatefulWidget {
  const _ThemeSheet({required this.initial});

  final ThemeMode initial;

  @override
  State<_ThemeSheet> createState() => _ThemeSheetState();
}

class _ThemeSheetState extends State<_ThemeSheet> {
  late ThemeMode _selected = widget.initial;

  @override
  Widget build(BuildContext context) {
    final ThemeData theme = Theme.of(context);
    final AppLocalizations l10n = AppLocalizations.of(context);

    return SafeArea(
      child: Padding(
        padding: const EdgeInsets.fromLTRB(20, 8, 20, 14),
        child: Column(
          mainAxisSize: MainAxisSize.min,
          crossAxisAlignment: CrossAxisAlignment.start,
          children: <Widget>[
            Center(
              child: Container(
                width: 36,
                height: 4,
                margin: const EdgeInsets.only(top: 6, bottom: 14),
                decoration: ShapeDecoration(
                  color: theme.colorScheme.outline,
                  shape: const StadiumBorder(),
                ),
              ),
            ),
            Text(
              l10n.settingsThemeSheetTitle,
              style: TextStyle(
                fontSize: 16,
                fontWeight: FontWeight.w800,
                letterSpacing: 16 * InventraDimens.titleLetterSpacingEm,
                color: theme.colorScheme.onSurface,
              ),
            ),
            const SizedBox(height: 14),
            Row(
              children: <Widget>[
                Expanded(
                  child: _ThemeTile(
                    mode: ThemeMode.light,
                    label: l10n.settingsThemeLight,
                    selected: _selected == ThemeMode.light,
                    onTap: () => setState(() => _selected = ThemeMode.light),
                  ),
                ),
                const SizedBox(width: 10),
                Expanded(
                  child: _ThemeTile(
                    mode: ThemeMode.dark,
                    label: l10n.settingsThemeDark,
                    selected: _selected == ThemeMode.dark,
                    onTap: () => setState(() => _selected = ThemeMode.dark),
                  ),
                ),
                const SizedBox(width: 10),
                Expanded(
                  child: _ThemeTile(
                    mode: ThemeMode.system,
                    label: l10n.settingsThemeSystem,
                    selected: _selected == ThemeMode.system,
                    onTap: () => setState(() => _selected = ThemeMode.system),
                  ),
                ),
              ],
            ),
            const SizedBox(height: 14),
            FilledButton(
              key: const ValueKey<String>('settings-theme-apply'),
              style: FilledButton.styleFrom(
                minimumSize: const Size.fromHeight(50),
                textStyle: theme.textTheme.labelLarge?.copyWith(
                  fontSize: 14,
                  fontWeight: FontWeight.w700,
                ),
              ),
              onPressed: () => Navigator.of(context).pop(_selected),
              child: Text(l10n.settingsThemeApply),
            ),
          ],
        ),
      ),
    );
  }
}

/// Satu tile pratinjau tema: thumbnail mini (permukaan tema yang digambarkan,
/// konstan — [InventraThemePreviewColors]) + label; tile terpilih berbingkai
/// primary dengan badge centang.
class _ThemeTile extends StatelessWidget {
  const _ThemeTile({
    required this.mode,
    required this.label,
    required this.selected,
    required this.onTap,
  });

  final ThemeMode mode;
  final String label;
  final bool selected;
  final VoidCallback onTap;

  @override
  Widget build(BuildContext context) {
    final ThemeData theme = Theme.of(context);
    final ColorScheme scheme = theme.colorScheme;

    final Widget preview = switch (mode) {
      ThemeMode.light => const _ThemePreview(dark: false),
      ThemeMode.dark => const _ThemePreview(dark: true),
      ThemeMode.system => const Row(
        children: <Widget>[
          Expanded(child: _ThemePreview(dark: false)),
          Expanded(child: _ThemePreview(dark: true)),
        ],
      ),
    };

    return Semantics(
      button: true,
      selected: selected,
      label: label,
      child: GestureDetector(
        key: ValueKey<String>('settings-theme-tile-${mode.name}'),
        onTap: onTap,
        child: Stack(
          clipBehavior: Clip.none,
          children: <Widget>[
            Container(
              padding: const EdgeInsets.all(10),
              decoration: BoxDecoration(
                borderRadius: BorderRadius.circular(14),
                border: Border.all(
                  color: selected ? scheme.primary : scheme.outlineVariant,
                  width: selected ? 2 : 1.5,
                ),
                boxShadow: selected
                    ? <BoxShadow>[
                        BoxShadow(
                          color: scheme.primary.withValues(alpha: 0.12),
                          spreadRadius: 4,
                        ),
                      ]
                    : null,
              ),
              child: Column(
                children: <Widget>[
                  SizedBox(
                    height: 64,
                    width: double.infinity,
                    child: ClipRRect(
                      borderRadius: BorderRadius.circular(9),
                      child: preview,
                    ),
                  ),
                  const SizedBox(height: 8),
                  Text(
                    label,
                    maxLines: 1,
                    overflow: TextOverflow.ellipsis,
                    style: TextStyle(
                      fontSize: 12,
                      fontWeight: selected ? FontWeight.w700 : FontWeight.w600,
                      color: selected
                          ? scheme.onPrimaryContainer
                          : theme.textTheme.labelLarge?.color,
                    ),
                  ),
                ],
              ),
            ),
            if (selected)
              Positioned(
                top: -8,
                right: -8,
                child: Container(
                  width: 22,
                  height: 22,
                  decoration: BoxDecoration(
                    color: scheme.primary,
                    shape: BoxShape.circle,
                  ),
                  child: Icon(
                    Symbols.check_rounded,
                    size: 14,
                    color: scheme.onPrimary,
                  ),
                ),
              ),
          ],
        ),
      ),
    );
  }
}

/// Thumbnail mini satu tema (top bar + dua blok konten) — warna konstan yang
/// menggambarkan tema tersebut, bukan tema aktif.
class _ThemePreview extends StatelessWidget {
  const _ThemePreview({required this.dark});

  final bool dark;

  @override
  Widget build(BuildContext context) {
    final Color background = dark
        ? InventraThemePreviewColors.darkBackground
        : InventraThemePreviewColors.lightBackground;
    final Color surface = dark
        ? InventraThemePreviewColors.darkSurface
        : InventraThemePreviewColors.lightSurface;
    final Color border = dark
        ? InventraThemePreviewColors.darkBorder
        : InventraThemePreviewColors.lightBorder;
    final Color block = dark
        ? InventraThemePreviewColors.darkBlock
        : InventraThemePreviewColors.lightBlock;
    final Color accent = dark
        ? InventraThemePreviewColors.darkAccent
        : InventraThemePreviewColors.lightAccent;

    Widget bar(Color color) => Container(
      height: 9,
      margin: const EdgeInsets.fromLTRB(7, 5, 7, 0),
      decoration: BoxDecoration(
        color: color,
        borderRadius: BorderRadius.circular(4),
      ),
    );

    return ColoredBox(
      color: background,
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.stretch,
        children: <Widget>[
          Container(
            height: 12,
            decoration: BoxDecoration(
              color: surface,
              border: Border(bottom: BorderSide(color: border)),
            ),
          ),
          bar(block),
          bar(accent),
        ],
      ),
    );
  }
}

/// Bottom sheet pemilih bahasa (paritas pill ID/EN layar login): dua opsi,
/// pilihan langsung diterapkan lewat pop.
class _LanguageSheet extends StatelessWidget {
  const _LanguageSheet({required this.currentCode});

  final String currentCode;

  @override
  Widget build(BuildContext context) {
    final ThemeData theme = Theme.of(context);
    final AppLocalizations l10n = AppLocalizations.of(context);

    Widget option({required String code, required String label}) {
      final bool active = currentCode == code;
      return InkWell(
        key: ValueKey<String>('settings-language-$code'),
        borderRadius: BorderRadius.circular(12),
        onTap: () => Navigator.of(context).pop(Locale(code)),
        child: Padding(
          padding: const EdgeInsets.symmetric(horizontal: 4, vertical: 12),
          child: Row(
            children: <Widget>[
              Expanded(
                child: Text(
                  label,
                  style: TextStyle(
                    fontSize: 13.5,
                    fontWeight: active ? FontWeight.w700 : FontWeight.w600,
                    color: theme.colorScheme.onSurface,
                  ),
                ),
              ),
              if (active)
                Icon(
                  Symbols.check_rounded,
                  size: 20,
                  color: theme.colorScheme.primary,
                ),
            ],
          ),
        ),
      );
    }

    return SafeArea(
      child: Padding(
        padding: const EdgeInsets.fromLTRB(20, 8, 20, 14),
        child: Column(
          mainAxisSize: MainAxisSize.min,
          crossAxisAlignment: CrossAxisAlignment.start,
          children: <Widget>[
            Center(
              child: Container(
                width: 36,
                height: 4,
                margin: const EdgeInsets.only(top: 6, bottom: 14),
                decoration: ShapeDecoration(
                  color: theme.colorScheme.outline,
                  shape: const StadiumBorder(),
                ),
              ),
            ),
            Text(
              l10n.settingsLanguageSheetTitle,
              style: TextStyle(
                fontSize: 16,
                fontWeight: FontWeight.w800,
                letterSpacing: 16 * InventraDimens.titleLetterSpacingEm,
                color: theme.colorScheme.onSurface,
              ),
            ),
            const SizedBox(height: 6),
            option(code: 'id', label: l10n.settingsLanguageIndonesian),
            const Divider(height: 1),
            option(code: 'en', label: l10n.settingsLanguageEnglish),
          ],
        ),
      ),
    );
  }
}
