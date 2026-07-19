import 'package:connectivity_plus/connectivity_plus.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

/// Plugin connectivity_plus sebagai provider supaya tes bisa mengganti dengan
/// stub tanpa platform channel.
final Provider<Connectivity> connectivityPluginProvider =
    Provider<Connectivity>((Ref ref) => Connectivity());

/// State global konektivitas (ARCHITECTURE bagian 2): stream online/offline
/// untuk [OfflineBanner] dan — mulai M5 — pemicu sync engine.
///
/// Nilai awal dari `checkConnectivity()` lalu mengikuti
/// `onConnectivityChanged`; keduanya `List<ConnectivityResult>` pada
/// connectivity_plus v7 (API: pub.dev/documentation/connectivity_plus/latest).
/// Perangkat dianggap online bila minimal satu antarmuka bukan
/// [ConnectivityResult.none].
final StreamProvider<bool> isOnlineProvider = StreamProvider<bool>((
  Ref ref,
) async* {
  final Connectivity connectivity = ref.watch(connectivityPluginProvider);
  bool? last;

  bool hasConnection(List<ConnectivityResult> results) => results.any(
    (ConnectivityResult result) => result != ConnectivityResult.none,
  );

  final bool initial = hasConnection(await connectivity.checkConnectivity());
  last = initial;
  yield initial;

  await for (final List<ConnectivityResult> results
      in connectivity.onConnectivityChanged) {
    final bool online = hasConnection(results);
    if (online != last) {
      last = online;
      yield online;
    }
  }
});

/// true saat offline; selama status belum diketahui (frame pertama sebelum
/// `checkConnectivity` selesai) dianggap online supaya banner tidak berkedip
/// saat cold start.
bool isOffline(AsyncValue<bool> connectivity) => connectivity.value == false;
