import 'package:flutter/foundation.dart';
import 'package:flutter/widgets.dart';
import 'package:inventra_mobile/features/scan/presentation/scan_camera.dart';

/// [ScanCamera] stub deterministik untuk widget/golden test — kamera nyata
/// (plugin mobile_scanner) tidak pernah disentuh tes (plan M0 Task 8).
class FakeScanCamera implements ScanCamera {
  FakeScanCamera({bool unavailable = false})
    : unavailableNotifier = ValueNotifier<bool>(unavailable);

  final ValueNotifier<bool> torchOnNotifier = ValueNotifier<bool>(false);
  final ValueNotifier<bool> unavailableNotifier;

  ValueChanged<String>? _onDetect;
  int toggleTorchCalls = 0;
  int disposeCalls = 0;

  @override
  ValueListenable<bool> get torchOn => torchOnNotifier;

  @override
  ValueListenable<bool> get unavailable => unavailableNotifier;

  /// Simulasi kamera mendeteksi barcode/QR bernilai [tag].
  void detect(String tag) => _onDetect?.call(tag);

  @override
  Widget buildPreview({required ValueChanged<String> onDetect}) {
    _onDetect = onDetect;
    return const SizedBox.expand();
  }

  @override
  Future<void> toggleTorch() async {
    toggleTorchCalls += 1;
    torchOnNotifier.value = !torchOnNotifier.value;
  }

  @override
  void dispose() {
    disposeCalls += 1;
  }
}
