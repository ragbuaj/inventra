import 'package:flutter/material.dart';

/// Blok skeleton loading dengan shimmer (Component Library bagian Progress ·
/// Skeleton). Bentuk disusun per layar; widget ini hanya satu blok berpendar.
///
/// Warna dari tema: dasar `outlineVariant`; kilau `secondaryContainer` (light)
/// atau `outline` (dark) — mengikuti pasangan warna shimmer mockup. Saat
/// pengguna meminta animasi dikurangi ([MediaQuery.disableAnimations]) blok
/// tampil statis.
class AppSkeleton extends StatefulWidget {
  const AppSkeleton({
    required this.height,
    this.width,
    this.borderRadius = 12,
    super.key,
  });

  final double height;

  /// Null berarti mengikuti lebar parent.
  final double? width;

  final double borderRadius;

  @override
  State<AppSkeleton> createState() => _AppSkeletonState();
}

class _AppSkeletonState extends State<AppSkeleton>
    with SingleTickerProviderStateMixin {
  late final AnimationController _controller = AnimationController(
    vsync: this,
    duration: const Duration(milliseconds: 1500),
  )..repeat();

  @override
  void dispose() {
    _controller.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    final ColorScheme scheme = Theme.of(context).colorScheme;
    final Color base = scheme.outlineVariant;
    final Color highlight = scheme.brightness == Brightness.light
        ? scheme.secondaryContainer
        : scheme.outline;
    final BorderRadius radius = BorderRadius.circular(widget.borderRadius);

    if (MediaQuery.disableAnimationsOf(context)) {
      return ExcludeSemantics(
        child: Container(
          width: widget.width,
          height: widget.height,
          decoration: BoxDecoration(color: base, borderRadius: radius),
        ),
      );
    }

    return ExcludeSemantics(
      child: AnimatedBuilder(
        animation: _controller,
        builder: (BuildContext context, Widget? child) {
          return Container(
            width: widget.width,
            height: widget.height,
            decoration: BoxDecoration(
              borderRadius: radius,
              gradient: LinearGradient(
                colors: <Color>[base, highlight, base],
                stops: const <double>[0.25, 0.5, 0.75],
                transform: _SlidingGradientTransform(_controller.value),
              ),
            ),
          );
        },
      ),
    );
  }
}

/// Menggeser gradient shimmer melintasi blok (pola cookbook resmi Flutter
/// "UI > Effects > Shimmer loading").
class _SlidingGradientTransform extends GradientTransform {
  const _SlidingGradientTransform(this.progress);

  /// 0..1 dari AnimationController; dipetakan ke -1..1 lebar blok.
  final double progress;

  @override
  Matrix4? transform(Rect bounds, {TextDirection? textDirection}) {
    return Matrix4.translationValues(bounds.width * (progress * 2 - 1), 0, 0);
  }
}
