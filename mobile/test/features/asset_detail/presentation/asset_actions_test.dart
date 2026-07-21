import 'package:flutter_test/flutter_test.dart';
import 'package:inventra_mobile/features/asset_detail/presentation/asset_actions.dart';

void main() {
  const Set<String> staf = <String>{'request.create'};
  const Set<String> manager = <String>{'request.create', 'assignment.manage'};
  const Set<String> viewer = <String>{'asset.view'};

  group('assetActionsFor — aset available', () {
    test('Manager: Check-out (bukan Pinjam) + Lapor Kerusakan', () {
      expect(assetActionsFor(manager, 'available'), <AssetAction>[
        AssetAction.checkout,
        AssetAction.reportDamage,
      ]);
    });

    test('Staf (request.create tanpa manage): Pinjam + Lapor Kerusakan', () {
      expect(assetActionsFor(staf, 'available'), <AssetAction>[
        AssetAction.borrow,
        AssetAction.reportDamage,
      ]);
    });

    test('tanpa izin aksi: kosong', () {
      expect(assetActionsFor(viewer, 'available'), isEmpty);
    });
  });

  group('assetActionsFor — aset assigned', () {
    test('Manager: Check-in + Lapor Kerusakan', () {
      expect(assetActionsFor(manager, 'assigned'), <AssetAction>[
        AssetAction.checkin,
        AssetAction.reportDamage,
      ]);
    });

    test('Staf: hanya Lapor Kerusakan (tak boleh check-in)', () {
      expect(assetActionsFor(staf, 'assigned'), <AssetAction>[
        AssetAction.reportDamage,
      ]);
    });
  });

  group('assetActionsFor — status lain', () {
    test('under_maintenance + Staf: hanya Lapor Kerusakan', () {
      expect(assetActionsFor(staf, 'under_maintenance'), <AssetAction>[
        AssetAction.reportDamage,
      ]);
    });

    test('status null: kosong untuk viewer', () {
      expect(assetActionsFor(viewer, null), isEmpty);
    });

    test('disposed + Manager: hanya Lapor Kerusakan (tanpa borrow/checkin)', () {
      expect(assetActionsFor(manager, 'disposed'), <AssetAction>[
        AssetAction.reportDamage,
      ]);
    });
  });
}
