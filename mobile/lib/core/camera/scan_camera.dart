import 'dart:async';

import 'package:flutter/foundation.dart';
import 'package:flutter/widgets.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:mobile_scanner/mobile_scanner.dart';

/// Abstraksi kamera pemindai supaya widget test dapat mengganti implementasi
/// [MobileScanner] (platform channel) dengan stub deterministik — kamera nyata
/// tidak pernah disentuh tes (plan M0 Task 8).
abstract class ScanCamera {
  /// true saat torch menyala — untuk ikon toggle di overlay.
  ValueListenable<bool> get torchOn;

  /// true saat kamera gagal dipakai (izin ditolak, emulator tanpa kamera,
  /// perangkat tidak didukung) — layar menampilkan state jelas + jalur manual.
  ValueListenable<bool> get unavailable;

  /// Preview kamera full screen; [onDetect] menerima nilai mentah barcode/QR.
  /// [scanWindow] (opsional) membatasi area deteksi ke sebuah [Rect] dalam
  /// ruang koordinat widget preview (mis. bingkai target); barcode di luarnya
  /// diabaikan. Null berarti mendeteksi di seluruh frame.
  Widget buildPreview({
    required ValueChanged<String> onDetect,
    Rect? scanWindow,
  });

  Future<void> toggleTorch();

  void dispose();
}

/// Implementasi produksi di atas `mobile_scanner` v7:
/// MobileScannerController + widget MobileScanner (autoStart bawaan true,
/// lifecycle pause/resume ditangani `useAppLifecycleState` bawaan widget).
/// API: pub.dev/documentation/mobile_scanner/latest (v7.3.0).
class MobileScannerScanCamera implements ScanCamera {
  MobileScannerScanCamera() {
    _controller.addListener(_syncFromController);
  }

  final MobileScannerController _controller = MobileScannerController();
  final ValueNotifier<bool> _torchOn = ValueNotifier<bool>(false);
  final ValueNotifier<bool> _unavailable = ValueNotifier<bool>(false);

  @override
  ValueListenable<bool> get torchOn => _torchOn;

  @override
  ValueListenable<bool> get unavailable => _unavailable;

  void _syncFromController() {
    final MobileScannerState state = _controller.value;
    _torchOn.value = state.torchState == TorchState.on;
    _unavailable.value = state.error != null;
  }

  @override
  Widget buildPreview({
    required ValueChanged<String> onDetect,
    Rect? scanWindow,
  }) {
    return MobileScanner(
      controller: _controller,
      // Membatasi deteksi ke bingkai target: mobile_scanner hanya meneruskan
      // barcode yang berada di dalam scanWindow (koordinat logis widget).
      scanWindow: scanWindow,
      onDetect: (BarcodeCapture capture) {
        for (final Barcode barcode in capture.barcodes) {
          final String? raw = barcode.rawValue;
          if (raw != null && raw.isNotEmpty) {
            onDetect(raw);
            return;
          }
        }
      },
      // State error (izin ditolak/unsupported) dirender lapisan overlay layar
      // lewat [unavailable]; builder di sini cukup mengosongkan area preview.
      errorBuilder: (BuildContext context, MobileScannerException error) =>
          const SizedBox.expand(),
      placeholderBuilder: (BuildContext context) => const SizedBox.expand(),
    );
  }

  @override
  Future<void> toggleTorch() => _controller.toggleTorch();

  @override
  void dispose() {
    _controller.removeListener(_syncFromController);
    unawaited(_controller.dispose());
    _torchOn.dispose();
    _unavailable.dispose();
  }
}

/// Factory [ScanCamera] — dipisah sebagai provider supaya test/golden bisa
/// meng-override dengan stub tanpa menyentuh plugin kamera.
final Provider<ScanCamera Function()> scanCameraFactoryProvider =
    Provider<ScanCamera Function()>((Ref ref) => MobileScannerScanCamera.new);
