import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';
import 'package:material_symbols_icons/symbols.dart';

import '../../../app/theme.dart';
import '../../../core/i18n/gen/app_localizations.dart';
import '../../../core/camera/scan_camera.dart';

/// Layar Scan 1:1 mockup "Inventra Mobile - Scan": kamera full screen dengan
/// viewfinder gelap di kedua tema, bingkai target, toggle torch, tombol tutup,
/// dan input tag manual (fallback wajib — paritas web). Deteksi barcode/QR
/// maupun submit manual mengarah ke `/assets/:tag` (push di atas shell).
class ScanScreen extends ConsumerStatefulWidget {
  const ScanScreen({super.key});

  @override
  ConsumerState<ScanScreen> createState() => _ScanScreenState();
}

class _ScanScreenState extends ConsumerState<ScanScreen> {
  late final ScanCamera _camera;

  /// Debounce: kamera mengirim deteksi berulang (~4x/detik) untuk label yang
  /// sama; satu tag tidak boleh ter-push berkali-kali. Terkunci selama layar
  /// detail terbuka + jeda singkat setelah kembali (kamera masih mengarah ke
  /// label yang sama).
  bool _navigating = false;
  DateTime? _cooldownUntil;
  static const Duration _detectCooldown = Duration(seconds: 2);

  @override
  void initState() {
    super.initState();
    _camera = ref.read(scanCameraFactoryProvider)();
  }

  @override
  void dispose() {
    _camera.dispose();
    super.dispose();
  }

  Future<void> _openDetail(
    String rawTag, {
    bool fromManualEntry = false,
  }) async {
    final String tag = rawTag.trim();
    if (tag.isEmpty || _navigating) {
      return;
    }
    // Submit manual selalu diproses (niat pengguna eksplisit); deteksi kamera
    // menghormati jeda setelah kembali dari detail.
    final DateTime? cooldownUntil = _cooldownUntil;
    if (!fromManualEntry &&
        cooldownUntil != null &&
        DateTime.now().isBefore(cooldownUntil)) {
      return;
    }
    _navigating = true;
    try {
      await context.push('/assets/${Uri.encodeComponent(tag)}');
    } finally {
      _cooldownUntil = DateTime.now().add(_detectCooldown);
      _navigating = false;
    }
  }

  Future<void> _openManualEntry() async {
    final ThemeData theme = Theme.of(context);
    final String? tag = await showModalBottomSheet<String>(
      context: context,
      isScrollControlled: true,
      showDragHandle: true,
      backgroundColor: theme.cardTheme.color ?? theme.colorScheme.surface,
      shape: const RoundedRectangleBorder(
        borderRadius: BorderRadius.vertical(top: Radius.circular(28)),
      ),
      builder: (BuildContext sheetContext) => Padding(
        padding: EdgeInsets.only(
          bottom: MediaQuery.viewInsetsOf(sheetContext).bottom,
        ),
        child: const _ManualEntrySheet(),
      ),
    );
    if (tag != null && mounted) {
      await _openDetail(tag, fromManualEntry: true);
    }
  }

  @override
  Widget build(BuildContext context) {
    final AppLocalizations l10n = AppLocalizations.of(context);

    // Viewfinder gelap di kedua tema; ikon status bar dipaksa terang.
    return AnnotatedRegion<SystemUiOverlayStyle>(
      value: SystemUiOverlayStyle.light,
      child: Scaffold(
        backgroundColor: InventraScanColors.viewfinderBackground,
        body: ValueListenableBuilder<bool>(
          valueListenable: _camera.unavailable,
          builder: (BuildContext context, bool unavailable, Widget? child) {
            return Stack(
              fit: StackFit.expand,
              children: <Widget>[
                if (!unavailable) _camera.buildPreview(onDetect: _openDetail),
                // Vignette lembut supaya kontrol tetap terbaca di atas frame
                // kamera (mockup: radial transparan tengah -> gelap tepi).
                DecoratedBox(
                  decoration: BoxDecoration(
                    gradient: RadialGradient(
                      center: const Alignment(0, -0.1),
                      radius: 1.1,
                      colors: <Color>[
                        InventraScanColors.viewfinderBackground.withValues(
                          alpha: 0,
                        ),
                        InventraScanColors.viewfinderBackground.withValues(
                          alpha: 0.62,
                        ),
                      ],
                      stops: const <double>[0.42, 1],
                    ),
                  ),
                ),
                SafeArea(
                  child: Column(
                    children: <Widget>[
                      Padding(
                        padding: const EdgeInsets.fromLTRB(20, 14, 20, 0),
                        child: Row(
                          mainAxisAlignment: MainAxisAlignment.spaceBetween,
                          children: <Widget>[
                            _OverlayCircleButton(
                              icon: Symbols.close_rounded,
                              tooltip: l10n.scanCloseTooltip,
                              onPressed: () => context.go('/'),
                            ),
                            Text(
                              l10n.scanTitle,
                              style: const TextStyle(
                                fontSize: 15,
                                fontWeight: FontWeight.w700,
                                color: InventraScanColors.foreground,
                              ),
                            ),
                            _TorchButton(
                              camera: _camera,
                              enabled: !unavailable,
                              l10n: l10n,
                            ),
                          ],
                        ),
                      ),
                      Expanded(
                        child: unavailable
                            ? _CameraUnavailable(l10n: l10n)
                            : _TargetFrame(hint: l10n.scanHint),
                      ),
                      Padding(
                        padding: const EdgeInsets.only(bottom: 44),
                        child: _ManualEntryButton(
                          label: l10n.scanManualButton,
                          onPressed: _openManualEntry,
                        ),
                      ),
                    ],
                  ),
                ),
              ],
            );
          },
        ),
      ),
    );
  }
}

/// Tombol lingkaran 44 semi-transparan di atas viewfinder (tutup/torch).
class _OverlayCircleButton extends StatelessWidget {
  const _OverlayCircleButton({
    required this.icon,
    required this.tooltip,
    required this.onPressed,
    this.enabled = true,
  });

  final IconData icon;
  final String tooltip;
  final VoidCallback onPressed;
  final bool enabled;

  @override
  Widget build(BuildContext context) {
    return Tooltip(
      message: tooltip,
      child: Material(
        color: InventraScanColors.controlBackground,
        shape: const CircleBorder(),
        clipBehavior: Clip.antiAlias,
        child: InkWell(
          onTap: enabled ? onPressed : null,
          child: SizedBox(
            width: 44,
            height: 44,
            child: Icon(
              icon,
              size: 22,
              color: enabled
                  ? InventraScanColors.foreground
                  : InventraScanColors.foregroundMuted,
            ),
          ),
        ),
      ),
    );
  }
}

/// Toggle torch: ikon dan tooltip mengikuti state nyala/mati dari kamera.
class _TorchButton extends StatelessWidget {
  const _TorchButton({
    required this.camera,
    required this.enabled,
    required this.l10n,
  });

  final ScanCamera camera;
  final bool enabled;
  final AppLocalizations l10n;

  @override
  Widget build(BuildContext context) {
    return ValueListenableBuilder<bool>(
      valueListenable: camera.torchOn,
      builder: (BuildContext context, bool torchOn, Widget? child) {
        return _OverlayCircleButton(
          icon: torchOn
              ? Symbols.flashlight_off_rounded
              : Symbols.flashlight_on_rounded,
          tooltip: torchOn ? l10n.scanTorchOffTooltip : l10n.scanTorchOnTooltip,
          enabled: enabled,
          onPressed: () => camera.toggleTorch(),
        );
      },
    );
  }
}

/// Bingkai target 252x252 dengan empat sudut hijau + garis scan statis, lalu
/// pill petunjuk di bawahnya.
class _TargetFrame extends StatelessWidget {
  const _TargetFrame({required this.hint});

  final String hint;

  @override
  Widget build(BuildContext context) {
    return Column(
      mainAxisAlignment: MainAxisAlignment.center,
      children: <Widget>[
        SizedBox(
          width: 252,
          height: 252,
          child: Stack(
            children: <Widget>[
              const Positioned.fill(
                child: CustomPaint(painter: _FrameCornerPainter()),
              ),
              // Garis scan (statis — tanpa animasi; lihat catatan deviasi).
              Positioned(
                left: 16,
                right: 16,
                top: 126,
                child: Container(
                  height: 3,
                  decoration: BoxDecoration(
                    borderRadius: BorderRadius.circular(999),
                    gradient: LinearGradient(
                      colors: <Color>[
                        InventraScanColors.frameAccent.withValues(alpha: 0),
                        InventraScanColors.frameAccent,
                        InventraScanColors.frameAccent,
                        InventraScanColors.frameAccent.withValues(alpha: 0),
                      ],
                      stops: const <double>[0, 0.3, 0.7, 1],
                    ),
                    boxShadow: <BoxShadow>[
                      BoxShadow(
                        color: InventraScanColors.frameAccent.withValues(
                          alpha: 0.9,
                        ),
                        blurRadius: 14,
                      ),
                    ],
                  ),
                ),
              ),
            ],
          ),
        ),
        const SizedBox(height: 22),
        Container(
          padding: const EdgeInsets.symmetric(horizontal: 16, vertical: 8),
          decoration: const ShapeDecoration(
            color: InventraScanColors.controlBackground,
            shape: StadiumBorder(),
          ),
          child: Text(
            hint,
            style: const TextStyle(
              fontSize: 13,
              color: InventraScanColors.foregroundMuted,
            ),
          ),
        ),
      ],
    );
  }
}

/// Empat sudut bingkai target: stroke 4, panjang 52, radius sudut 26.
class _FrameCornerPainter extends CustomPainter {
  const _FrameCornerPainter();

  @override
  void paint(Canvas canvas, Size size) {
    final Paint paint = Paint()
      ..color = InventraScanColors.frameAccent
      ..style = PaintingStyle.stroke
      ..strokeWidth = 4
      ..strokeCap = StrokeCap.round;
    const double arm = 52;
    const double radius = 26;
    final double w = size.width;
    final double h = size.height;

    Path corner(Offset start, Offset cornerPoint, Offset end) {
      // Dua lengan siku dengan sudut membulat via arcToPoint.
      final Path path = Path()..moveTo(start.dx, start.dy);
      final Offset beforeCorner = Offset.lerp(
        cornerPoint,
        start,
        radius / arm,
      )!;
      final Offset afterCorner = Offset.lerp(cornerPoint, end, radius / arm)!;
      path
        ..lineTo(beforeCorner.dx, beforeCorner.dy)
        ..arcToPoint(afterCorner, radius: const Radius.circular(radius))
        ..lineTo(end.dx, end.dy);
      return path;
    }

    canvas
      ..drawPath(
        corner(const Offset(0, arm), Offset.zero, const Offset(arm, 0)),
        paint,
      )
      ..drawPath(
        corner(Offset(w - arm, 0), Offset(w, 0), Offset(w, arm)),
        paint,
      )
      ..drawPath(
        corner(Offset(w, h - arm), Offset(w, h), Offset(w - arm, h)),
        paint,
      )
      ..drawPath(
        corner(Offset(arm, h), Offset(0, h), Offset(0, h - arm)),
        paint,
      );
  }

  @override
  bool shouldRepaint(covariant CustomPainter oldDelegate) => false;
}

/// State kamera tidak tersedia (izin ditolak/emulator): pesan jelas di atas
/// viewfinder gelap; jalur manual di bawah tetap bisa dipakai.
class _CameraUnavailable extends StatelessWidget {
  const _CameraUnavailable({required this.l10n});

  final AppLocalizations l10n;

  @override
  Widget build(BuildContext context) {
    return Center(
      child: Padding(
        padding: const EdgeInsets.symmetric(horizontal: 40),
        child: Column(
          mainAxisSize: MainAxisSize.min,
          children: <Widget>[
            Container(
              width: 56,
              height: 56,
              decoration: const BoxDecoration(
                color: InventraScanColors.controlBackground,
                shape: BoxShape.circle,
              ),
              child: const Icon(
                Symbols.no_photography_rounded,
                size: 30,
                color: InventraScanColors.foregroundMuted,
              ),
            ),
            const SizedBox(height: 12),
            Text(
              l10n.scanCameraUnavailableTitle,
              textAlign: TextAlign.center,
              style: const TextStyle(
                fontSize: 14,
                fontWeight: FontWeight.w600,
                color: InventraScanColors.foreground,
              ),
            ),
            const SizedBox(height: 6),
            Text(
              l10n.scanCameraUnavailableBody,
              textAlign: TextAlign.center,
              style: const TextStyle(
                fontSize: 12,
                color: InventraScanColors.foregroundMuted,
              ),
            ),
          ],
        ),
      ),
    );
  }
}

/// Pill "Ketik kode manual" di dasar layar scan.
class _ManualEntryButton extends StatelessWidget {
  const _ManualEntryButton({required this.label, required this.onPressed});

  final String label;
  final VoidCallback onPressed;

  @override
  Widget build(BuildContext context) {
    return Material(
      color: InventraScanColors.manualButtonBackground,
      shape: const StadiumBorder(
        side: BorderSide(color: InventraScanColors.manualButtonBorder),
      ),
      clipBehavior: Clip.antiAlias,
      child: InkWell(
        onTap: onPressed,
        child: Container(
          height: 50,
          padding: const EdgeInsets.symmetric(horizontal: 22),
          child: Row(
            mainAxisSize: MainAxisSize.min,
            children: <Widget>[
              const Icon(
                Symbols.keyboard_rounded,
                size: 20,
                color: InventraScanColors.foreground,
              ),
              const SizedBox(width: 9),
              Text(
                label,
                style: const TextStyle(
                  fontSize: 14,
                  fontWeight: FontWeight.w600,
                  color: InventraScanColors.foreground,
                ),
              ),
            ],
          ),
        ),
      ),
    );
  }
}

/// Bottom sheet input tag manual 1:1 mockup: judul, field kode aset dengan
/// ikon tag, teks bantuan format, tombol Cari. Mengembalikan tag lewat
/// `Navigator.pop` — navigasi ke detail dilakukan pemanggil.
class _ManualEntrySheet extends StatefulWidget {
  const _ManualEntrySheet();

  @override
  State<_ManualEntrySheet> createState() => _ManualEntrySheetState();
}

class _ManualEntrySheetState extends State<_ManualEntrySheet> {
  final TextEditingController _controller = TextEditingController();

  @override
  void dispose() {
    _controller.dispose();
    super.dispose();
  }

  void _submit() {
    final String tag = _controller.text.trim();
    if (tag.isEmpty) {
      return;
    }
    Navigator.of(context).pop(tag);
  }

  @override
  Widget build(BuildContext context) {
    final ThemeData theme = Theme.of(context);
    final ColorScheme scheme = theme.colorScheme;
    final AppLocalizations l10n = AppLocalizations.of(context);

    return Padding(
      padding: const EdgeInsets.fromLTRB(20, 0, 20, 18),
      child: Column(
        mainAxisSize: MainAxisSize.min,
        crossAxisAlignment: CrossAxisAlignment.start,
        children: <Widget>[
          Text(
            l10n.scanManualSheetTitle,
            style: theme.textTheme.titleMedium?.copyWith(fontSize: 16),
          ),
          const SizedBox(height: 14),
          Padding(
            padding: const EdgeInsets.only(bottom: 6),
            child: Text(
              l10n.scanManualFieldLabel,
              style: TextStyle(
                fontSize: 12,
                fontWeight: FontWeight.w600,
                color: scheme.primary,
              ),
            ),
          ),
          TextField(
            controller: _controller,
            autofocus: true,
            textCapitalization: TextCapitalization.characters,
            textInputAction: TextInputAction.search,
            onSubmitted: (_) => _submit(),
            style: const TextStyle(fontSize: 15),
            decoration: InputDecoration(
              hintText: l10n.scanManualFieldHint,
              prefixIcon: Padding(
                padding: const EdgeInsets.only(left: 16, right: 10),
                child: Icon(
                  Symbols.tag_rounded,
                  size: 20,
                  color: scheme.primary,
                ),
              ),
              prefixIconConstraints: const BoxConstraints(
                minWidth: 0,
                minHeight: 0,
              ),
            ),
          ),
          const SizedBox(height: 7),
          Text(
            l10n.scanManualFieldHelper,
            style: TextStyle(
              fontSize: 11,
              color: theme.textTheme.labelSmall?.color,
            ),
          ),
          const SizedBox(height: 14),
          FilledButton.icon(
            style: FilledButton.styleFrom(
              minimumSize: const Size.fromHeight(
                InventraDimens.buttonHeightPrimary,
              ),
              textStyle: theme.textTheme.labelLarge?.copyWith(
                fontSize: 15,
                fontWeight: FontWeight.w700,
              ),
            ),
            onPressed: _submit,
            icon: const Icon(Symbols.search_rounded, size: 20),
            label: Text(l10n.scanManualSubmit),
          ),
        ],
      ),
    );
  }
}
