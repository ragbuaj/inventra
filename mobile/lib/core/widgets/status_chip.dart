import 'package:flutter/material.dart';

import '../../app/theme.dart';

/// Keluarga semantik chip status — dipetakan ke [InventraStatusColors].
enum StatusChipVariant { success, info, warning, danger, neutral }

/// Chip status Component Library: titik indikator + label dalam pill.
///
/// Warna selalu dari [InventraStatusColors] (ThemeExtension) sehingga otomatis
/// mengikuti light/dark; status domain (aset/opname/pengajuan) memilih varian
/// lewat getter di extension tersebut.
class StatusChip extends StatelessWidget {
  const StatusChip({required this.label, required this.variant, super.key});

  final String label;
  final StatusChipVariant variant;

  @override
  Widget build(BuildContext context) {
    final InventraStatusColors colors = Theme.of(
      context,
    ).extension<InventraStatusColors>()!;
    final StatusColorSet set = switch (variant) {
      StatusChipVariant.success => colors.success,
      StatusChipVariant.info => colors.info,
      StatusChipVariant.warning => colors.warning,
      StatusChipVariant.danger => colors.danger,
      StatusChipVariant.neutral => colors.neutral,
    };

    return Container(
      padding: const EdgeInsets.symmetric(horizontal: 11, vertical: 4),
      decoration: ShapeDecoration(color: set.bg, shape: const StadiumBorder()),
      child: Row(
        mainAxisSize: MainAxisSize.min,
        children: <Widget>[
          Container(
            width: 7,
            height: 7,
            decoration: BoxDecoration(color: set.dot, shape: BoxShape.circle),
          ),
          const SizedBox(width: 6),
          Text(
            label,
            style: TextStyle(
              fontSize: 12,
              fontWeight: FontWeight.w600,
              color: set.text,
            ),
          ),
        ],
      ),
    );
  }
}
