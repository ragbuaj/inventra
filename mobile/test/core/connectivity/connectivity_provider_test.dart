import 'dart:async';

import 'package:connectivity_plus/connectivity_plus.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:inventra_mobile/core/connectivity/connectivity_provider.dart';
import 'package:mocktail/mocktail.dart';

class _MockConnectivity extends Mock implements Connectivity {}

void main() {
  late _MockConnectivity connectivity;
  late StreamController<List<ConnectivityResult>> changes;

  setUp(() {
    connectivity = _MockConnectivity();
    changes = StreamController<List<ConnectivityResult>>();
    when(
      () => connectivity.onConnectivityChanged,
    ).thenAnswer((_) => changes.stream);
  });

  tearDown(() {
    // Tidak di-await: controller tanpa listener (generator provider sudah
    // dibatalkan saat container dibuang) membuat future close menggantung.
    unawaited(changes.close());
  });

  ProviderContainer createContainer() {
    return ProviderContainer.test(
      overrides: [connectivityPluginProvider.overrideWithValue(connectivity)],
    );
  }

  Future<void> flushMicrotasks() => Future<void>.delayed(Duration.zero);

  test('nilai awal dari checkConnectivity (wifi berarti online)', () async {
    when(
      () => connectivity.checkConnectivity(),
    ).thenAnswer((_) async => <ConnectivityResult>[ConnectivityResult.wifi]);
    final ProviderContainer container = createContainer();
    final List<bool> emissions = <bool>[];
    container.listen<AsyncValue<bool>>(isOnlineProvider, (
      AsyncValue<bool>? previous,
      AsyncValue<bool> next,
    ) {
      if (next case AsyncData<bool>(:final bool value)) {
        emissions.add(value);
      }
    }, fireImmediately: true);

    await flushMicrotasks();

    expect(emissions, <bool>[true]);
    expect(isOffline(container.read(isOnlineProvider)), isFalse);
  });

  test(
    'perubahan ke none berarti offline; kembali online terpancar lagi',
    () async {
      when(() => connectivity.checkConnectivity()).thenAnswer(
        (_) async => <ConnectivityResult>[ConnectivityResult.mobile],
      );
      final ProviderContainer container = createContainer();
      final List<bool> emissions = <bool>[];
      container.listen<AsyncValue<bool>>(isOnlineProvider, (
        AsyncValue<bool>? previous,
        AsyncValue<bool> next,
      ) {
        if (next case AsyncData<bool>(:final bool value)) {
          emissions.add(value);
        }
      }, fireImmediately: true);
      await flushMicrotasks();

      changes.add(<ConnectivityResult>[ConnectivityResult.none]);
      await flushMicrotasks();
      expect(isOffline(container.read(isOnlineProvider)), isTrue);

      changes.add(<ConnectivityResult>[ConnectivityResult.wifi]);
      await flushMicrotasks();

      expect(emissions, <bool>[true, false, true]);
    },
  );

  test('nilai duplikat tidak dipancarkan ulang (distinct)', () async {
    when(
      () => connectivity.checkConnectivity(),
    ).thenAnswer((_) async => <ConnectivityResult>[ConnectivityResult.wifi]);
    final ProviderContainer container = createContainer();
    final List<bool> emissions = <bool>[];
    container.listen<AsyncValue<bool>>(isOnlineProvider, (
      AsyncValue<bool>? previous,
      AsyncValue<bool> next,
    ) {
      if (next case AsyncData<bool>(:final bool value)) {
        emissions.add(value);
      }
    }, fireImmediately: true);
    await flushMicrotasks();

    // wifi -> ethernet: keduanya online, tidak ada emisi baru.
    changes.add(<ConnectivityResult>[ConnectivityResult.ethernet]);
    await flushMicrotasks();
    changes.add(<ConnectivityResult>[ConnectivityResult.none]);
    await flushMicrotasks();
    changes.add(<ConnectivityResult>[ConnectivityResult.none]);
    await flushMicrotasks();

    expect(emissions, <bool>[true, false]);
  });

  test('sebelum status diketahui dianggap online (banner tidak berkedip)', () {
    when(
      () => connectivity.checkConnectivity(),
    ).thenAnswer((_) async => <ConnectivityResult>[ConnectivityResult.wifi]);
    final ProviderContainer container = createContainer();

    // Frame pertama: AsyncLoading — belum dianggap offline.
    expect(isOffline(container.read(isOnlineProvider)), isFalse);
  });
}
