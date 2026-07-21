import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:intl/intl.dart';
import 'package:material_symbols_icons/symbols.dart';

import '../../../core/i18n/gen/app_localizations.dart';
import '../../../core/authz/permissions_provider.dart';
import '../data/asset_action_repository.dart';
import '../data/asset_dto.dart';
import 'asset_by_tag_provider.dart';
import 'asset_actions.dart';

/// Aksi FR-M7 yang SUDAH terpasang UI-nya. Bertambah per fase: M7-4 Peminjaman
/// (borrow); Check-out/Check-in (M7-5) dan Lapor Kerusakan (M7-6) menyusul —
/// [assetActionsFor] sudah menghitung semuanya, tapi hanya yang di sini yang
/// dirender agar tidak ada tombol tanpa aksi.
const Set<AssetAction> _implementedActions = <AssetAction>{AssetAction.borrow};

/// Bar aksi sticky di kaki Detail Aset (di luar sesi opname). Tombol muncul
/// sesuai permission pemanggil ([permissionsProvider]) x status aset. Tanpa
/// aksi -> tidak dirender (detail tetap read-only murni).
class AssetActionBar extends ConsumerWidget {
  const AssetActionBar({required this.asset, super.key});

  final AssetDto asset;

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final Set<String> permissions =
        ref.watch(permissionsProvider).value ?? const <String>{};
    final List<AssetAction> actions = assetActionsFor(permissions, asset.status)
        .where(_implementedActions.contains)
        .toList();
    if (actions.isEmpty) {
      return const SizedBox.shrink();
    }

    final AppLocalizations l10n = AppLocalizations.of(context);
    final ThemeData theme = Theme.of(context);

    return Material(
      color: theme.cardTheme.color ?? theme.colorScheme.surface,
      elevation: 8,
      child: SafeArea(
        top: false,
        child: Padding(
          padding: const EdgeInsets.fromLTRB(20, 10, 20, 10),
          child: Row(
            children: <Widget>[
              for (final AssetAction action in actions) ...<Widget>[
                if (action != actions.first) const SizedBox(width: 10),
                Expanded(
                  child: FilledButton.icon(
                    onPressed: () => _onAction(context, ref, action),
                    icon: Icon(_actionIcon(action), size: 18),
                    label: Text(_actionLabel(l10n, action)),
                    style: FilledButton.styleFrom(
                      minimumSize: const Size(0, 48),
                    ),
                  ),
                ),
              ],
            ],
          ),
        ),
      ),
    );
  }

  void _onAction(BuildContext context, WidgetRef ref, AssetAction action) {
    switch (action) {
      case AssetAction.borrow:
        showModalBottomSheet<void>(
          context: context,
          isScrollControlled: true,
          showDragHandle: true,
          builder: (BuildContext context) => _BorrowSheet(asset: asset),
        );
      case AssetAction.checkout:
      case AssetAction.checkin:
      case AssetAction.reportDamage:
        // Belum dirender (lihat _implementedActions).
        break;
    }
  }
}

String _actionLabel(AppLocalizations l10n, AssetAction action) {
  return switch (action) {
    AssetAction.borrow => l10n.assetActionBorrow,
    AssetAction.checkout => l10n.assetActionCheckout,
    AssetAction.checkin => l10n.assetActionCheckin,
    AssetAction.reportDamage => l10n.assetActionReportDamage,
  };
}

IconData _actionIcon(AssetAction action) {
  return switch (action) {
    AssetAction.borrow => Symbols.handshake_rounded,
    AssetAction.checkout => Symbols.logout_rounded,
    AssetAction.checkin => Symbols.login_rounded,
    AssetAction.reportDamage => Symbols.build_rounded,
  };
}

/// Sheet Ajukan Peminjaman (Staf): jatuh tempo opsional + catatan opsional lalu
/// `POST /assignments/borrow` (pengajuan via approval). Sukses -> SnackBar; error
/// -> pesan inline.
class _BorrowSheet extends ConsumerStatefulWidget {
  const _BorrowSheet({required this.asset});

  final AssetDto asset;

  @override
  ConsumerState<_BorrowSheet> createState() => _BorrowSheetState();
}

class _BorrowSheetState extends ConsumerState<_BorrowSheet> {
  final TextEditingController _notes = TextEditingController();
  DateTime? _dueDate;
  bool _submitting = false;
  String? _error;

  @override
  void dispose() {
    _notes.dispose();
    super.dispose();
  }

  Future<void> _pickDueDate() async {
    final DateTime now = DateTime.now();
    final DateTime? picked = await showDatePicker(
      context: context,
      firstDate: now,
      lastDate: DateTime(now.year + 5),
      initialDate: _dueDate ?? now,
    );
    if (picked != null) {
      setState(() => _dueDate = picked);
    }
  }

  Future<void> _submit() async {
    setState(() {
      _submitting = true;
      _error = null;
    });
    final AppLocalizations l10n = AppLocalizations.of(context);
    try {
      await ref
          .read(assetActionRepositoryProvider)
          .borrow(
            assetId: widget.asset.id ?? '',
            dueDate: _dueDate == null
                ? null
                : DateFormat('yyyy-MM-dd').format(_dueDate!),
            notes: _notes.text,
          );
      if (!mounted) {
        return;
      }
      // Muat ulang detail (nama/tag tak berubah, tapi selaras pola refresh).
      final String? tag = widget.asset.assetTag;
      if (tag != null && tag.isNotEmpty) {
        ref.invalidate(assetByTagProvider(tag));
      }
      Navigator.of(context).pop();
      ScaffoldMessenger.of(
        context,
      ).showSnackBar(SnackBar(content: Text(l10n.borrowSuccess)));
    } on Object {
      if (!mounted) {
        return;
      }
      setState(() {
        _submitting = false;
        _error = l10n.borrowError;
      });
    }
  }

  @override
  Widget build(BuildContext context) {
    final AppLocalizations l10n = AppLocalizations.of(context);
    final ThemeData theme = Theme.of(context);
    final String localeName = Localizations.localeOf(context).languageCode;
    final String dueLabel = _dueDate == null
        ? l10n.borrowPickDate
        : DateFormat('d MMM y', localeName).format(_dueDate!);

    return Padding(
      padding: EdgeInsets.fromLTRB(
        20,
        0,
        20,
        20 + MediaQuery.of(context).viewInsets.bottom,
      ),
      child: Column(
        mainAxisSize: MainAxisSize.min,
        crossAxisAlignment: CrossAxisAlignment.start,
        children: <Widget>[
          Text(
            l10n.borrowSheetTitle,
            style: theme.textTheme.titleMedium?.copyWith(
              fontWeight: FontWeight.w700,
            ),
          ),
          const SizedBox(height: 4),
          Text(
            widget.asset.name ?? l10n.catalogUnnamedAsset,
            style: TextStyle(color: theme.colorScheme.onSurfaceVariant),
          ),
          const SizedBox(height: 16),
          Text(l10n.borrowDueDateLabel, style: theme.textTheme.labelLarge),
          const SizedBox(height: 6),
          OutlinedButton.icon(
            onPressed: _submitting ? null : _pickDueDate,
            icon: const Icon(Symbols.calendar_month_rounded, size: 18),
            label: Text(dueLabel),
            style: OutlinedButton.styleFrom(
              alignment: Alignment.centerLeft,
              minimumSize: const Size.fromHeight(48),
            ),
          ),
          const SizedBox(height: 14),
          Text(l10n.borrowNotesLabel, style: theme.textTheme.labelLarge),
          const SizedBox(height: 6),
          TextField(
            controller: _notes,
            enabled: !_submitting,
            minLines: 2,
            maxLines: 4,
            decoration: InputDecoration(hintText: l10n.borrowNotesHint),
          ),
          const SizedBox(height: 8),
          Text(
            l10n.borrowPendingNote,
            style: TextStyle(
              fontSize: 12,
              color: theme.colorScheme.onSurfaceVariant,
            ),
          ),
          if (_error != null) ...<Widget>[
            const SizedBox(height: 8),
            Text(
              _error!,
              style: TextStyle(color: theme.colorScheme.error, fontSize: 13),
            ),
          ],
          const SizedBox(height: 16),
          FilledButton(
            onPressed: _submitting ? null : _submit,
            style: FilledButton.styleFrom(minimumSize: const Size.fromHeight(50)),
            child: _submitting
                ? const SizedBox(
                    width: 20,
                    height: 20,
                    child: CircularProgressIndicator(strokeWidth: 2.5),
                  )
                : Text(l10n.borrowSubmit),
          ),
        ],
      ),
    );
  }
}
